package reseller

import (
	"encoding/csv"
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tokoonline/app/internal/httpx"
	"github.com/tokoonline/app/internal/middleware"
	"github.com/tokoonline/app/internal/models"
	"github.com/tokoonline/app/internal/services/auth"
	"github.com/tokoonline/app/internal/services/cart"
	"github.com/tokoonline/app/internal/services/catalog"
	"github.com/tokoonline/app/internal/services/imageopt"
	"github.com/tokoonline/app/internal/services/mailer"
	"github.com/tokoonline/app/internal/services/pricing"
	resellerSvc "github.com/tokoonline/app/internal/services/reseller"
	"github.com/tokoonline/app/internal/services/settings"
	"github.com/tokoonline/app/internal/views/layouts"
	views "github.com/tokoonline/app/internal/views/reseller"
)

type Handler struct {
	Pool      *pgxpool.Pool
	Auth      *auth.Service
	Reseller  *resellerSvc.Service
	Catalog   *catalog.Service
	Cart      *cart.Service
	Pricing   *pricing.Service
	Settings  *settings.Store
	Sessions  *scs.SessionManager
	Mailer    *mailer.Mailer
	UploadDir string
	BaseURL   string
}

func (h *Handler) PageData(r *http.Request) layouts.PageData {
	d := layouts.PageData{
		BaseURL:    h.BaseURL,
		CSRFToken:  httpx.CSRFToken(r),
		IsLoggedIn: middleware.UserID(r) != nil,
		UserRole:   middleware.UserRole(r),
		UserEmail:  middleware.UserEmail(r),
		Store:      h.Settings.Store(),
		SEO:        h.Settings.SEO(),
		Marketing:  h.Settings.Marketing(),
	}
	uid := middleware.UserID(r)
	if uid != nil {
		aud, _ := h.Pricing.AudienceForUser(r.Context(), uid)
		c, err := h.Cart.GetOrCreate(r.Context(), middleware.SessionToken(r), uid, aud.Code)
		if err == nil {
			d.CartCount = cart.TotalQty(c)
		}
	}
	return d
}

func (h *Handler) ShowLogin(w http.ResponseWriter, r *http.Request) {
	d := h.PageData(r)
	d.Title = "Login Reseller"
	d.NoIndex = true
	httpx.Render(w, r, views.Login(d, ""))
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	user, err := h.Auth.Authenticate(r.Context(), r.FormValue("email"), r.FormValue("password"))
	if err != nil || user.Role != "reseller" {
		d := h.PageData(r)
		d.Title = "Login Reseller"
		d.NoIndex = true
		msg := "Email atau password salah"
		if err == auth.ErrInactive {
			msg = "Akun Anda belum diapprove admin."
		}
		httpx.Render(w, r, views.Login(d, msg))
		return
	}
	h.Sessions.Put(r.Context(), "user_id", user.ID.String())
	h.Sessions.Put(r.Context(), "user_role", user.Role)
	h.Sessions.Put(r.Context(), "user_email", user.Email)
	h.Sessions.RenewToken(r.Context())
	if tok := middleware.SessionToken(r); tok != "" {
		_ = h.Cart.MergeOnLogin(r.Context(), tok, user.ID)
	}
	http.Redirect(w, r, "/reseller", http.StatusFound)
}

func (h *Handler) ShowRegister(w http.ResponseWriter, r *http.Request) {
	cfg := h.Settings.Reseller()
	if !cfg.RegistrationOpen {
		http.Error(w, "Pendaftaran reseller sedang ditutup.", 403)
		return
	}
	d := h.PageData(r)
	d.Title = "Daftar Reseller"
	d.NoIndex = true
	httpx.Render(w, r, views.Register(d, "", cfg.RequireKTP, cfg.RequireNPWP))
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	// Parse multipart up to 25MB total (KTP + NPWP + a few store photos)
	if err := r.ParseMultipartForm(25 << 20); err != nil {
		// fall back to simple form parsing if not multipart
		_ = r.ParseForm()
	}
	cfg := h.Settings.Reseller()
	if !cfg.RegistrationOpen {
		http.Error(w, "registrasi tutup", 403)
		return
	}
	// Save any uploaded verification documents
	docURLs := h.saveResellerDocs(r)

	in := resellerSvc.RegisterInput{
		Email:       strings.TrimSpace(strings.ToLower(r.FormValue("email"))),
		Password:    r.FormValue("password"),
		FullName:    r.FormValue("name"),
		StoreName:   r.FormValue("store_name"),
		Phone:       r.FormValue("phone"),
		NPWP:        r.FormValue("npwp"),
		KTPNumber:   r.FormValue("ktp_number"),
		Address:     r.FormValue("address"),
		Province:    r.FormValue("province"),
		City:        r.FormValue("city"),
		District:    r.FormValue("district"),
		PostalCode:  r.FormValue("postal_code"),
		Docs:        docURLs,
		AutoApprove: cfg.AutoApprove,
	}
	if cfg.RequireKTP && in.KTPNumber == "" {
		d := h.PageData(r)
		httpx.Render(w, r, views.Register(d, "KTP wajib diisi", cfg.RequireKTP, cfg.RequireNPWP))
		return
	}
	if cfg.RequireNPWP && in.NPWP == "" {
		d := h.PageData(r)
		httpx.Render(w, r, views.Register(d, "NPWP wajib diisi", cfg.RequireKTP, cfg.RequireNPWP))
		return
	}
	if len(in.Password) < 8 {
		d := h.PageData(r)
		httpx.Render(w, r, views.Register(d, "Password minimal 8 karakter", cfg.RequireKTP, cfg.RequireNPWP))
		return
	}
	_, err := h.Reseller.Register(r.Context(), in)
	if err != nil {
		d := h.PageData(r)
		httpx.Render(w, r, views.Register(d, "Gagal: "+err.Error(), cfg.RequireKTP, cfg.RequireNPWP))
		return
	}
	// Notify the new reseller and the admin
	store := h.Settings.Store()
	subj, body := mailer.ResellerPending(store.Name, in.FullName)
	h.Mailer.SendAsync(r.Context(), in.Email, subj, body)
	if store.Email != "" {
		adminSubj := "[" + store.Name + "] Reseller baru menunggu approval: " + in.StoreName
		adminBody := "Email: " + in.Email + "<br/>Nama Toko: " + in.StoreName + "<br/>HP: " + in.Phone + "<br/>Cek di /admin/resellers?status=pending"
		h.Mailer.SendAsync(r.Context(), store.Email, adminSubj, adminBody)
	}
	d := h.PageData(r)
	d.Title = "Pendaftaran Diterima"
	d.NoIndex = true
	httpx.Render(w, r, views.Pending(d))
}

func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserID(r)
	if uid == nil {
		http.Redirect(w, r, "/reseller/login", http.StatusFound)
		return
	}
	profile, tier, err := h.Reseller.Profile(r.Context(), *uid)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	outstanding, _ := h.Reseller.CreditUsage(r.Context(), *uid)
	stmts, _ := h.Reseller.MonthlyStatements(r.Context(), *uid)
	d := h.PageData(r)
	d.Title = "Dashboard Reseller"
	d.NoIndex = true
	httpx.Render(w, r, views.Dashboard(d, views.DashData{
		Profile: profile, Tier: tier, OutstandTOP: outstanding, Statements: stmts,
	}))
}

func (h *Handler) CatalogPage(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserID(r)
	aud, _ := h.Pricing.AudienceForUser(r.Context(), uid)
	products, _, _ := h.Catalog.List(r.Context(), catalog.ListOpts{Limit: 100, Audience: aud.Code, OnlyB2B: true})
	var tier *models.ResellerTier
	if uid != nil && aud.TierID != nil {
		_, t, _ := h.Reseller.Profile(r.Context(), *uid)
		tier = t
	}
	d := h.PageData(r)
	d.Title = "Katalog Reseller"
	d.NoIndex = true
	httpx.Render(w, r, views.Catalog(d, products, tier))
}

func (h *Handler) BulkPage(w http.ResponseWriter, r *http.Request) {
	d := h.PageData(r)
	d.Title = "Bulk Order"
	d.NoIndex = true
	httpx.Render(w, r, views.BulkOrder(d))
}

func (h *Handler) BulkAdd(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	uid := middleware.UserID(r)
	aud, _ := h.Pricing.AudienceForUser(r.Context(), uid)
	c, err := h.Cart.GetOrCreate(r.Context(), middleware.SessionToken(r), uid, aud.Code)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	sku := strings.TrimSpace(r.FormValue("sku"))
	qty, _ := strconv.Atoi(r.FormValue("qty"))
	if qty <= 0 {
		qty = 1
	}
	var vid uuid.UUID
	if err := h.Pool.QueryRow(r.Context(), `SELECT id FROM product_variants WHERE sku=$1 AND is_active=TRUE`, sku).Scan(&vid); err != nil {
		http.Error(w, "SKU tidak ditemukan: "+sku, 404)
		return
	}
	if err := h.Cart.Add(r.Context(), c, vid, qty, aud.Code, aud.DiscountPct); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if httpx.IsHTMX(r) {
		w.Header().Set("HX-Refresh", "true")
	}
	w.Write([]byte(fmt.Sprintf("OK: SKU %s qty %d", sku, qty)))
}

func (h *Handler) BulkCSV(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	f, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	defer f.Close()
	uid := middleware.UserID(r)
	aud, _ := h.Pricing.AudienceForUser(r.Context(), uid)
	c, err := h.Cart.GetOrCreate(r.Context(), middleware.SessionToken(r), uid, aud.Code)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	rd := csv.NewReader(f)
	added, skipped := 0, 0
	for {
		row, err := rd.Read()
		if err != nil {
			break
		}
		if len(row) < 2 {
			continue
		}
		sku := strings.TrimSpace(row[0])
		qty, _ := strconv.Atoi(strings.TrimSpace(row[1]))
		if sku == "" || qty <= 0 {
			continue
		}
		var vid uuid.UUID
		if err := h.Pool.QueryRow(r.Context(), `SELECT id FROM product_variants WHERE sku=$1 AND is_active=TRUE`, sku).Scan(&vid); err != nil {
			skipped++
			continue
		}
		if err := h.Cart.Add(r.Context(), c, vid, qty, aud.Code, aud.DiscountPct); err == nil {
			added++
		}
	}
	http.Redirect(w, r, fmt.Sprintf("/cart?added=%d&skipped=%d", added, skipped), http.StatusFound)
}

func (h *Handler) Profile(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserID(r)
	if uid == nil {
		http.Redirect(w, r, "/reseller/login", http.StatusFound)
		return
	}
	p, _, err := h.Reseller.Profile(r.Context(), *uid)
	if err != nil {
		if err == pgx.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), 500)
		return
	}
	d := h.PageData(r)
	d.Title = "Profil"
	d.NoIndex = true
	httpx.Render(w, r, views.Profile(d, p))
}

// saveResellerDocs persists uploaded KTP/NPWP/store photos and returns their URLs.
// Errors during single-file save are silently skipped (the user will still be created).
func (h *Handler) saveResellerDocs(r *http.Request) []string {
	if r.MultipartForm == nil {
		return nil
	}
	dir := h.UploadDir
	if dir == "" {
		dir = "static/uploads"
	}
	subdir := filepath.Join(dir, "reseller-docs")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		return nil
	}
	urls := []string{}
	saveOne := func(field string, fh *multipart.FileHeader) {
		if fh == nil {
			return
		}
		f, err := fh.Open()
		if err != nil {
			return
		}
		defer f.Close()
		stem := fmt.Sprintf("%s-%d-%s", field, time.Now().UnixNano(), uuid.NewString()[:8])
		srcExt := strings.ToLower(filepath.Ext(fh.Filename))
		if srcExt == "" {
			srcExt = ".jpg"
		}
		dstPath := filepath.Join(subdir, stem+srcExt)
		finalExt, oerr := imageopt.Optimize(f, dstPath)
		if oerr != nil {
			return
		}
		urls = append(urls, "/uploads/reseller-docs/"+stem+finalExt)
	}
	for _, field := range []string{"ktp_file", "npwp_file"} {
		if files := r.MultipartForm.File[field]; len(files) > 0 {
			saveOne(field, files[0])
		}
	}
	if files := r.MultipartForm.File["store_photos"]; len(files) > 0 {
		for i, fh := range files {
			if i >= 3 {
				break
			}
			saveOne("store", fh)
		}
	}
	return urls
}

func (h *Handler) Statements(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserID(r)
	if uid == nil {
		http.Redirect(w, r, "/reseller/login", http.StatusFound)
		return
	}
	stmts, _ := h.Reseller.MonthlyStatements(r.Context(), *uid)
	d := h.PageData(r)
	d.Title = "Tagihan & Rekap"
	d.NoIndex = true
	httpx.Render(w, r, views.Statements(d, stmts))
}
