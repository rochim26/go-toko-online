package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tokoonline/app/internal/httpx"
	"github.com/tokoonline/app/internal/middleware"
	"github.com/tokoonline/app/internal/models"
	"github.com/tokoonline/app/internal/services/auth"
	"github.com/tokoonline/app/internal/services/catalog"
	"github.com/tokoonline/app/internal/services/imageopt"
	"github.com/tokoonline/app/internal/services/integrations"
	"github.com/tokoonline/app/internal/services/mailer"
	"github.com/tokoonline/app/internal/services/order"
	"github.com/tokoonline/app/internal/services/pdf"
	"github.com/tokoonline/app/internal/services/reseller"
	"github.com/tokoonline/app/internal/services/settings"
	"github.com/tokoonline/app/internal/views/admin"
	"github.com/tokoonline/app/internal/views/layouts"
)

type Handler struct {
	Pool      *pgxpool.Pool
	Auth      *auth.Service
	Catalog   *catalog.Service
	Order     *order.Service
	Reseller  *reseller.Service
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
		Marketing:  settings.Marketing{}, // hide tracking pixel on admin
		BodyClass:  "admin",
		Integrations: integrations.All(h.Settings),
	}
	if uid := middleware.UserID(r); uid != nil {
		var done bool
		_ = h.Pool.QueryRow(r.Context(), `SELECT onboarding_completed FROM users WHERE id=$1`, *uid).Scan(&done)
		d.OnboardingDone = done
	}
	return d
}

// ---- Auth ----

func (h *Handler) ShowLogin(w http.ResponseWriter, r *http.Request) {
	d := h.PageData(r)
	d.Title = "Login Admin"
	d.NoIndex = true
	httpx.Render(w, r, admin.Login(d, ""))
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	user, err := h.Auth.Authenticate(r.Context(), r.FormValue("email"), r.FormValue("password"))
	if err != nil || (user.Role != "admin" && user.Role != "staff") {
		d := h.PageData(r)
		d.Title = "Login Admin"
		d.NoIndex = true
		httpx.Render(w, r, admin.Login(d, "Email atau password salah"))
		return
	}
	h.Sessions.Put(r.Context(), "user_id", user.ID.String())
	h.Sessions.Put(r.Context(), "user_role", user.Role)
	h.Sessions.Put(r.Context(), "user_email", user.Email)
	h.Sessions.RenewToken(r.Context())
	http.Redirect(w, r, "/admin", http.StatusFound)
}

// ---- Dashboard ----

func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	st := admin.DashStat{}
	_ = h.Pool.QueryRow(r.Context(), `SELECT COALESCE(count(*),0), COALESCE(sum(grand_total),0) FROM orders WHERE created_at >= current_date`).Scan(&st.OrdersToday, &st.RevenueToday)
	_ = h.Pool.QueryRow(r.Context(), `SELECT COALESCE(count(*),0), COALESCE(sum(grand_total),0) FROM orders WHERE payment_status='paid'`).Scan(&st.OrdersTotal, &st.RevenueTotal)
	_ = h.Pool.QueryRow(r.Context(), `SELECT count(*) FROM orders WHERE status IN ('paid','packed')`).Scan(&st.PendingOrders)
	_ = h.Pool.QueryRow(r.Context(), `SELECT count(*) FROM reseller_profiles WHERE status='pending'`).Scan(&st.PendingResellers)
	_ = h.Pool.QueryRow(r.Context(), `SELECT count(*) FROM inventory_levels WHERE on_hand <= low_stock_at`).Scan(&st.LowStock)
	d := h.PageData(r)
	d.Title = "Dashboard"
	d.NoIndex = true
	httpx.Render(w, r, admin.Dashboard(d, st))
}

// ---- Products ----

func (h *Handler) Products(w http.ResponseWriter, r *http.Request) {
	products, _, _ := h.Catalog.List(r.Context(), catalog.ListOpts{Limit: 100, Audience: "b2c"})
	d := h.PageData(r)
	d.Title = "Produk"
	d.NoIndex = true
	httpx.Render(w, r, admin.Products(d, products))
}

func (h *Handler) NewProductForm(w http.ResponseWriter, r *http.Request) {
	brands, cats := h.brandsCats(r.Context())
	d := h.PageData(r)
	d.Title = "Tambah Produk"
	d.NoIndex = true
	httpx.Render(w, r, admin.ProductForm(d, admin.ProductFormData{
		IsNew: true, Brands: brands, Categories: cats,
		Status: "draft", IsB2C: true, IsB2B: true, GMCEnabled: true, WeightGrams: 500,
	}))
}

func (h *Handler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	in := h.parseProductInput(r)
	id, err := h.Catalog.CreateProduct(r.Context(), in)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/admin/products/"+id.String(), http.StatusFound)
}

func (h *Handler) EditProduct(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	p, _, _ := h.fetchProductForForm(r.Context(), id)
	if p == nil {
		http.NotFound(w, r)
		return
	}
	d := h.PageData(r)
	d.Title = "Edit " + p.Name
	d.NoIndex = true
	f := h.buildProductForm(r.Context(), p)
	httpx.Render(w, r, admin.ProductForm(d, f))
}

func (h *Handler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	r.ParseForm()
	if err := h.Catalog.UpdateProduct(r.Context(), id, h.parseProductInput(r)); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/admin/products/"+id.String(), http.StatusFound)
}

func (h *Handler) AddVariant(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	r.ParseForm()
	bp, _ := strconv.ParseFloat(r.FormValue("base_price"), 64)
	wg, _ := strconv.Atoi(r.FormValue("weight_grams"))
	_, err = h.Catalog.AddVariant(r.Context(), id, r.FormValue("sku"), r.FormValue("name"), bp, wg, false)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/admin/products/"+id.String(), http.StatusFound)
}

func (h *Handler) DeleteVariant(w http.ResponseWriter, r *http.Request) {
	pid := chi.URLParam(r, "id")
	vid, err := uuid.Parse(chi.URLParam(r, "vid"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = h.Catalog.DeleteVariant(r.Context(), vid)
	http.Redirect(w, r, "/admin/products/"+pid, http.StatusFound)
}

func (h *Handler) SetVariantPrice(w http.ResponseWriter, r *http.Request) {
	vid, err := uuid.Parse(chi.URLParam(r, "vid"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	r.ParseForm()
	price, _ := strconv.ParseFloat(r.FormValue("price"), 64)
	_ = h.Catalog.UpdateVariantPrice(r.Context(), vid, r.FormValue("audience"), price)
	if httpx.IsHTMX(r) {
		w.WriteHeader(204)
		return
	}
	http.Redirect(w, r, "/admin/products/"+chi.URLParam(r, "id"), http.StatusFound)
}

func (h *Handler) SetInventory(w http.ResponseWriter, r *http.Request) {
	vid, err := uuid.Parse(chi.URLParam(r, "vid"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	r.ParseForm()
	q, _ := strconv.Atoi(r.FormValue("qty"))
	_ = h.Catalog.SetInventory(r.Context(), vid, q)
	if httpx.IsHTMX(r) {
		w.WriteHeader(204)
		return
	}
	http.Redirect(w, r, "/admin/products/"+chi.URLParam(r, "id"), http.StatusFound)
}

func (h *Handler) UploadImage(w http.ResponseWriter, r *http.Request) {
	pid, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	// Allow up to 50MB total per submit (multiple files combined)
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		http.Error(w, "Upload terlalu besar atau format salah: "+err.Error(), 400)
		return
	}
	if err := os.MkdirAll(h.UploadDir, 0o755); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		// fallback to single-file field name "file" for backward compat
		if f := r.MultipartForm.File["file"]; len(f) > 0 {
			files = f
		}
	}
	if len(files) == 0 {
		http.Error(w, "Tidak ada file yang dipilih", 400)
		return
	}
	primary := r.FormValue("is_primary") == "1"
	altText := r.FormValue("alt_text")
	saved := 0
	for i, fh := range files {
		f, err := fh.Open()
		if err != nil {
			continue
		}
		stem := fmt.Sprintf("%d-%s", time.Now().UnixNano(), uuid.NewString()[:8])
		srcExt := strings.ToLower(filepath.Ext(fh.Filename))
		if srcExt == "" {
			srcExt = ".jpg"
		}
		dstPath := filepath.Join(h.UploadDir, stem+srcExt)
		finalExt, oerr := imageopt.Optimize(f, dstPath)
		f.Close()
		if oerr != nil {
			continue
		}
		url := "/uploads/" + stem + finalExt
		// Only the FIRST uploaded image of this batch becomes "primary" (if requested)
		isPrimary := primary && i == 0
		if err := h.Catalog.AddImage(r.Context(), pid, url, altText, isPrimary); err == nil {
			saved++
		}
	}
	http.Redirect(w, r, "/admin/products/"+pid.String(), http.StatusFound)
}

var _ = io.Discard

func (h *Handler) DeleteImage(w http.ResponseWriter, r *http.Request) {
	pid := chi.URLParam(r, "id")
	iid, err := uuid.Parse(chi.URLParam(r, "iid"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = h.Catalog.DeleteImage(r.Context(), iid)
	http.Redirect(w, r, "/admin/products/"+pid, http.StatusFound)
}

func (h *Handler) parseProductInput(r *http.Request) catalog.ProductInput {
	in := catalog.ProductInput{
		Slug:         strings.TrimSpace(r.FormValue("slug")),
		Name:         strings.TrimSpace(r.FormValue("name")),
		ShortDesc:    r.FormValue("short_desc"),
		Description:  r.FormValue("description"),
		Status:       r.FormValue("status"),
		IsB2C:        r.FormValue("is_b2c") != "",
		IsB2B:        r.FormValue("is_b2b") != "",
		SeoTitle:     r.FormValue("seo_title"),
		SeoDesc:      r.FormValue("seo_desc"),
		FocusKeyword: r.FormValue("focus_keyword"),
		OGImageURL:   r.FormValue("og_image_url"),
		GMCEnabled:   r.FormValue("gmc_enabled") != "",
		GMCBrand:     r.FormValue("gmc_brand"),
		GMCGtin:      r.FormValue("gmc_gtin"),
		GMCMpn:       r.FormValue("gmc_mpn"),
		GMCCondition: "new",
	}
	if w, _ := strconv.Atoi(r.FormValue("weight_grams")); w > 0 {
		in.WeightGrams = w
	}
	if v := r.FormValue("brand_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			in.BrandID = &id
		}
	}
	if v := r.FormValue("category_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			in.CategoryID = &id
		}
	}
	if raw := r.FormValue("faqs"); raw != "" {
		var faqs []models.FAQ
		_ = json.Unmarshal([]byte(raw), &faqs)
		in.FAQs = faqs
	}
	return in
}

func (h *Handler) brandsCats(ctx context.Context) ([]*models.Brand, []*models.Category) {
	var brands []*models.Brand
	rows, err := h.Pool.Query(ctx, `SELECT id,name,slug,logo_url,description FROM brands ORDER BY name`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			b := &models.Brand{}
			if err := rows.Scan(&b.ID, &b.Name, &b.Slug, &b.LogoURL, &b.Description); err == nil {
				brands = append(brands, b)
			}
		}
	}
	cats, _ := h.Catalog.ListCategories(ctx)
	return brands, cats
}

func (h *Handler) fetchProductForForm(ctx context.Context, id uuid.UUID) (*models.Product, []*models.Variant, []*models.ProductImage) {
	p := &models.Product{}
	var faqs []byte
	err := h.Pool.QueryRow(ctx, `SELECT id,slug,name,brand_id,category_id,short_desc,description,status,is_b2c,is_b2b,weight_grams,seo_title,seo_desc,focus_keyword,og_image_url,gmc_enabled,gmc_brand,gmc_gtin,gmc_mpn,gmc_condition,faqs FROM products WHERE id=$1`, id).
		Scan(&p.ID, &p.Slug, &p.Name, &p.BrandID, &p.CategoryID, &p.ShortDesc, &p.Description, &p.Status, &p.IsB2C, &p.IsB2B, &p.WeightGrams, &p.SeoTitle, &p.SeoDesc, &p.FocusKeyword, &p.OGImageURL, &p.GMCEnabled, &p.GMCBrand, &p.GMCGtin, &p.GMCMpn, &p.GMCCondition, &faqs)
	if err != nil {
		return nil, nil, nil
	}
	if len(faqs) > 0 {
		_ = json.Unmarshal(faqs, &p.FAQs)
	}
	vs, _ := h.Catalog.GetVariants(ctx, p.ID)
	imgs, _ := h.Catalog.GetImages(ctx, p.ID)
	return p, vs, imgs
}

func (h *Handler) buildProductForm(ctx context.Context, p *models.Product) admin.ProductFormData {
	brands, cats := h.brandsCats(ctx)
	vs, _ := h.Catalog.GetVariants(ctx, p.ID)
	imgs, _ := h.Catalog.GetImages(ctx, p.ID)
	tiers, _ := h.Reseller.ListTiers(ctx)

	prices := map[string]map[string]float64{}
	rows, err := h.Pool.Query(ctx, `SELECT pp.variant_id::text, pp.audience, pp.price FROM product_prices pp JOIN product_variants v ON v.id=pp.variant_id WHERE v.product_id=$1`, p.ID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var vid, aud string
			var pr float64
			if err := rows.Scan(&vid, &aud, &pr); err == nil {
				if prices[vid] == nil {
					prices[vid] = map[string]float64{}
				}
				prices[vid][aud] = pr
			}
		}
	}
	inv := map[string]int{}
	rows2, err := h.Pool.Query(ctx, `SELECT il.variant_id::text, il.on_hand FROM inventory_levels il JOIN product_variants v ON v.id=il.variant_id WHERE v.product_id=$1`, p.ID)
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var vid string
			var oh int
			if err := rows2.Scan(&vid, &oh); err == nil {
				inv[vid] = oh
			}
		}
	}
	faqsRaw, _ := json.Marshal(p.FAQs)
	bid := ""
	if p.BrandID != nil {
		bid = p.BrandID.String()
	}
	cid := ""
	if p.CategoryID != nil {
		cid = p.CategoryID.String()
	}
	return admin.ProductFormData{
		IsNew: false, ID: p.ID.String(), Slug: p.Slug, Name: p.Name,
		Brands: brands, BrandID: bid,
		Categories: cats, CategoryID: cid,
		ShortDesc: derefStr(p.ShortDesc), Description: derefStr(p.Description),
		Status: p.Status, IsB2C: p.IsB2C, IsB2B: p.IsB2B,
		WeightGrams:  p.WeightGrams,
		SeoTitle:     derefStr(p.SeoTitle),
		SeoDesc:      derefStr(p.SeoDesc),
		FocusKeyword: derefStr(p.FocusKeyword),
		OGImageURL:   derefStr(p.OGImageURL),
		GMCEnabled:   p.GMCEnabled,
		GMCBrand:     derefStr(p.GMCBrand),
		GMCGtin:      derefStr(p.GMCGtin),
		GMCMpn:       derefStr(p.GMCMpn),
		FAQs:         string(faqsRaw),
		Variants:     vs,
		Images:       imgs,
		Tiers:        tiers,
		PricesByVariant: prices,
		Inventory:    inv,
	}
}

// ---- Orders ----

func (h *Handler) Orders(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	channel := r.URL.Query().Get("channel")
	orders, _, _ := h.Order.AdminList(r.Context(), status, channel, 100, 0)
	d := h.PageData(r)
	d.Title = "Pesanan"
	d.NoIndex = true
	httpx.Render(w, r, admin.Orders(d, orders, status, channel))
}

func (h *Handler) OrderDetail(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	o, err := h.Order.GetByCode(r.Context(), code)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	d := h.PageData(r)
	d.Title = "Order " + o.Code
	d.NoIndex = true
	httpx.Render(w, r, admin.OrderDetail(d, o))
}

func (h *Handler) UpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	o, err := h.Order.GetByCode(r.Context(), code)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	r.ParseForm()
	newStatus := r.FormValue("status")
	awb := r.FormValue("awb")
	cc := r.FormValue("courier_code")
	cs := r.FormValue("courier_service")
	_ = h.Order.UpdateStatus(r.Context(), o.ID, newStatus, awb, cc, cs)
	// If transitioned to 'shipped' and there's an email + AWB, notify
	if newStatus == "shipped" && o.CustomerEmail != nil && *o.CustomerEmail != "" && awb != "" {
		o2, _ := h.Order.GetByCode(r.Context(), code)
		if o2 != nil {
			subj, body := mailer.OrderShipped(h.Settings.Store().Name, h.BaseURL, o2)
			h.Mailer.SendAsync(r.Context(), *o2.CustomerEmail, subj, body)
		}
	}
	http.Redirect(w, r, "/admin/orders/"+code, http.StatusFound)
}

func (h *Handler) OrderPOPDF(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	o, err := h.Order.GetByCode(r.Context(), code)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	store := h.Settings.Store()
	tax := h.Settings.Tax()
	d := pdf.POData{
		StoreName:    store.Name,
		StoreAddress: store.Address,
		StoreNPWP:    tax.InvoiceNPWP,
		BuyerName:    derefStr(o.CustomerName),
		BuyerAddress: derefStr(o.ShipAddress),
		PONumber:     "PO-" + o.Code,
		IssuedAt:     httpx.DateID(o.CreatedAt),
		Order:        o,
	}
	if o.TopDueAt != nil {
		d.DueAt = httpx.DateID(*o.TopDueAt)
	}
	buf, err := pdf.GeneratePO(d)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `inline; filename="`+d.PONumber+`.pdf"`)
	w.Write(buf)
}

// ---- Reseller mgmt ----

func (h *Handler) ResellersList(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	rs, _ := h.Reseller.AdminList(r.Context(), status)
	tiers, _ := h.Reseller.ListTiers(r.Context())
	items := make([]admin.ResellerListItem, 0, len(rs))
	for _, x := range rs {
		tn := ""
		if x.TierName != nil {
			tn = *x.TierName
		}
		items = append(items, admin.ResellerListItem{
			UserID: x.UserID.String(), Email: x.Email, StoreName: x.StoreName, Status: x.Status, TierName: tn, CreatedAt: x.CreatedAt,
		})
	}
	d := h.PageData(r)
	d.Title = "Reseller"
	d.NoIndex = true
	httpx.Render(w, r, admin.Resellers(d, items, tiers, status))
}

func (h *Handler) ResellerApprove(w http.ResponseWriter, r *http.Request) {
	uid, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	r.ParseForm()
	tid, err := uuid.Parse(r.FormValue("tier_id"))
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	approver := middleware.UserID(r)
	if approver == nil {
		http.Error(w, "no admin", 401)
		return
	}
	if err := h.Reseller.Approve(r.Context(), uid, tid, *approver); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	// notify reseller via email
	var email, name string
	_ = h.Pool.QueryRow(r.Context(), `SELECT u.email, COALESCE(u.full_name,'') FROM users u WHERE u.id=$1`, uid).Scan(&email, &name)
	tiers, _ := h.Reseller.ListTiers(r.Context())
	var tierName string
	var disc float64
	var moq int
	for _, t := range tiers {
		if t.ID == tid {
			tierName = t.Name
			disc = t.DiscountPct
			moq = t.MoqQty
		}
	}
	if email != "" {
		subj, body := mailer.ResellerApproved(h.Settings.Store().Name, h.BaseURL, name, tierName, disc, moq)
		h.Mailer.SendAsync(r.Context(), email, subj, body)
	}
	http.Redirect(w, r, "/admin/resellers?status=pending", http.StatusFound)
}

func (h *Handler) ResellerReject(w http.ResponseWriter, r *http.Request) {
	uid, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	r.ParseForm()
	reason := r.FormValue("reason")
	if err := h.Reseller.Reject(r.Context(), uid, reason); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	var email, name string
	_ = h.Pool.QueryRow(r.Context(), `SELECT u.email, COALESCE(u.full_name,'') FROM users u WHERE u.id=$1`, uid).Scan(&email, &name)
	if email != "" {
		subj, body := mailer.ResellerRejected(h.Settings.Store().Name, name, reason)
		h.Mailer.SendAsync(r.Context(), email, subj, body)
	}
	http.Redirect(w, r, "/admin/resellers?status=pending", http.StatusFound)
}

func (h *Handler) Tiers(w http.ResponseWriter, r *http.Request) {
	tiers, _ := h.Reseller.ListTiers(r.Context())
	d := h.PageData(r)
	d.Title = "Tier Reseller"
	d.NoIndex = true
	httpx.Render(w, r, admin.Tiers(d, tiers))
}

func (h *Handler) TierUpsert(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	disc, _ := strconv.ParseFloat(r.FormValue("discount_pct"), 64)
	moqQty, _ := strconv.Atoi(r.FormValue("moq_qty"))
	moqVal, _ := strconv.ParseFloat(r.FormValue("moq_value"), 64)
	topDays, _ := strconv.Atoi(r.FormValue("top_days"))
	limit, _ := strconv.ParseFloat(r.FormValue("credit_limit"), 64)
	_, err := h.Pool.Exec(r.Context(), `INSERT INTO reseller_tiers(code,name,discount_pct,moq_qty,moq_value,top_days,credit_limit) VALUES($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT(code) DO UPDATE SET name=EXCLUDED.name, discount_pct=EXCLUDED.discount_pct, moq_qty=EXCLUDED.moq_qty, moq_value=EXCLUDED.moq_value, top_days=EXCLUDED.top_days, credit_limit=EXCLUDED.credit_limit, updated_at=now()`,
		r.FormValue("code"), r.FormValue("name"), disc, moqQty, moqVal, topDays, limit)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/admin/tiers", http.StatusFound)
}

// ---- Settings ----

func (h *Handler) SettingsPage(w http.ResponseWriter, r *http.Request) {
	tab := r.URL.Query().Get("tab")
	if tab == "" {
		tab = "store"
	}
	sd := admin.SettingsData{ActiveTab: tab}
	jsonToMap(h.Settings.GetRaw("store"), &sd.StoreInfo)
	jsonToMap(h.Settings.GetRaw("seo_global"), &sd.SEO)
	jsonToMap(h.Settings.GetRaw("marketing"), &sd.Marketing)
	jsonToMap(h.Settings.GetRaw("gmc"), &sd.GMC)
	jsonToMap(h.Settings.GetRaw("xendit"), &sd.Xendit)
	jsonToMap(h.Settings.GetRaw("biteship"), &sd.Biteship)
	jsonToMap(h.Settings.GetRaw("shipping"), &sd.Shipping)
	jsonToMap(h.Settings.GetRaw("mailer"), &sd.Mailer)
	jsonToMap(h.Settings.GetRaw("tax"), &sd.Tax)
	jsonToMap(h.Settings.GetRaw("reseller"), &sd.Reseller)
	d := h.PageData(r)
	d.Title = "Pengaturan"
	d.NoIndex = true
	httpx.Render(w, r, admin.Settings(d, sd))
}

func (h *Handler) SaveSettings(w http.ResponseWriter, r *http.Request) {
	tab := chi.URLParam(r, "tab")
	r.ParseForm()
	val := map[string]any{}
	switch tab {
	case "store":
		for _, k := range []string{"name", "tagline", "logo_url", "favicon_url", "email", "phone", "wa_number", "address", "origin_area_id", "origin_postal_code"} {
			val[k] = r.FormValue(k)
		}
		_ = h.Settings.Set(r.Context(), "store", val)
	case "seo":
		for _, k := range []string{"title_pattern", "default_title", "default_desc", "default_og_image", "robots_extra", "gsc_verification", "bing_verification"} {
			val[k] = r.FormValue(k)
		}
		val["ai_overview_optimized"] = r.FormValue("ai_overview_optimized") != ""
		_ = h.Settings.Set(r.Context(), "seo_global", val)
	case "marketing":
		for _, k := range []string{"meta_pixel_id", "meta_capi_token", "meta_test_event_code", "ga4_id", "ga4_api_secret", "gtm_id", "tiktok_pixel_id", "google_ads_id", "google_ads_label"} {
			val[k] = r.FormValue(k)
		}
		_ = h.Settings.Set(r.Context(), "marketing", val)
	case "gmc":
		for _, k := range []string{"merchant_id", "feed_format", "shipping_country", "content_language", "target_country"} {
			val[k] = r.FormValue(k)
		}
		val["feed_enabled"] = r.FormValue("feed_enabled") != ""
		val["auto_disable_oos"] = r.FormValue("auto_disable_oos") != ""
		_ = h.Settings.Set(r.Context(), "gmc", val)
	case "xendit":
		for _, k := range []string{"secret_key", "webhook_token", "public_key"} {
			val[k] = r.FormValue(k)
		}
		val["methods_enabled"] = []string{}
		val["success_redirect"] = "/order/success"
		val["failure_redirect"] = "/order/failed"
		_ = h.Settings.Set(r.Context(), "xendit", val)
	case "biteship":
		val["api_key"] = r.FormValue("api_key")
		val["origin_area_id"] = r.FormValue("origin_area_id")
		val["origin_postal_code"] = r.FormValue("origin_postal_code")
		var couriers []string
		for _, c := range strings.Split(r.FormValue("couriers"), ",") {
			c = strings.TrimSpace(c)
			if c != "" {
				couriers = append(couriers, c)
			}
		}
		val["couriers"] = couriers
		_ = h.Settings.Set(r.Context(), "biteship", val)
	case "shipping":
		v1, _ := strconv.ParseFloat(r.FormValue("free_shipping_threshold"), 64)
		v2, _ := strconv.ParseFloat(r.FormValue("flat_rate_fallback"), 64)
		val["free_shipping_threshold"] = v1
		val["flat_rate_fallback"] = v2
		_ = h.Settings.Set(r.Context(), "shipping", val)
	case "mailer":
		port, _ := strconv.Atoi(r.FormValue("smtp_port"))
		val["smtp_host"] = r.FormValue("smtp_host")
		val["smtp_port"] = port
		val["smtp_user"] = r.FormValue("smtp_user")
		val["smtp_pass"] = r.FormValue("smtp_pass")
		val["from_email"] = r.FormValue("from_email")
		val["from_name"] = r.FormValue("from_name")
		_ = h.Settings.Set(r.Context(), "mailer", val)
	case "tax":
		ppn, _ := strconv.ParseFloat(r.FormValue("ppn_pct"), 64)
		val["ppn_pct"] = ppn
		val["invoice_prefix"] = r.FormValue("invoice_prefix")
		val["invoice_npwp"] = r.FormValue("invoice_npwp")
		_ = h.Settings.Set(r.Context(), "tax", val)
	case "reseller":
		val["registration_open"] = r.FormValue("registration_open") != ""
		val["auto_approve"] = r.FormValue("auto_approve") != ""
		val["require_npwp"] = r.FormValue("require_npwp") != ""
		val["require_ktp"] = r.FormValue("require_ktp") != ""
		mfo, _ := strconv.ParseFloat(r.FormValue("min_first_order"), 64)
		val["min_first_order"] = mfo
		_ = h.Settings.Set(r.Context(), "reseller", val)
	default:
		http.Error(w, "unknown tab", 400)
		return
	}
	http.Redirect(w, r, "/admin/settings?tab="+tab, http.StatusFound)
}

func jsonToMap(raw []byte, dst *map[string]any) {
	*dst = map[string]any{}
	if len(raw) == 0 {
		return
	}
	_ = json.Unmarshal(raw, dst)
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
