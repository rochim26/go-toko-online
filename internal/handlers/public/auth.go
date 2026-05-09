package public

import (
	"net/http"
	"strings"

	"github.com/alexedwards/scs/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tokoonline/app/internal/httpx"
	"github.com/tokoonline/app/internal/middleware"
	"github.com/tokoonline/app/internal/services/auth"
	"github.com/tokoonline/app/internal/services/cart"
	"github.com/tokoonline/app/internal/services/mailer"
	"github.com/tokoonline/app/internal/services/security"
	views "github.com/tokoonline/app/internal/views/public"
)

type AuthHandler struct {
	Pool     *pgxpool.Pool
	Auth     *auth.Service
	Cart     *cart.Service
	Mailer   *mailer.Mailer
	Sessions *scs.SessionManager
	Public   *Handler
}

func (h *AuthHandler) ShowLogin(w http.ResponseWriter, r *http.Request) {
	d := h.Public.PageData(r)
	d.Title = "Masuk"
	d.NoIndex = true
	httpx.Render(w, r, views.Login(d, "", "", r.URL.Query().Get("next")))
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	email := r.FormValue("email")
	password := r.FormValue("password")
	next := r.FormValue("next")
	user, err := h.Auth.Authenticate(r.Context(), email, password)
	if err != nil {
		d := h.Public.PageData(r)
		d.Title = "Masuk"
		d.NoIndex = true
		httpx.Render(w, r, views.Login(d, err.Error(), email, next))
		return
	}
	h.Sessions.Put(r.Context(), "user_id", user.ID.String())
	h.Sessions.Put(r.Context(), "user_role", user.Role)
	h.Sessions.Put(r.Context(), "user_email", user.Email)
	h.Sessions.RenewToken(r.Context())
	// Merge anonymous cart
	if tok := middleware.SessionToken(r); tok != "" {
		_ = h.Cart.MergeOnLogin(r.Context(), tok, user.ID)
	}
	dest := redirectFor(user.Role, next)
	http.Redirect(w, r, dest, http.StatusFound)
}

func redirectFor(role, next string) string {
	if next != "" && strings.HasPrefix(next, "/") && !strings.HasPrefix(next, "//") {
		return next
	}
	switch role {
	case "admin", "staff":
		return "/admin"
	case "reseller":
		return "/reseller"
	default:
		return "/account"
	}
}

func (h *AuthHandler) ShowRegister(w http.ResponseWriter, r *http.Request) {
	d := h.Public.PageData(r)
	d.Title = "Daftar"
	d.NoIndex = true
	httpx.Render(w, r, views.Register(d, "", "", "", ""))
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	email := r.FormValue("email")
	password := r.FormValue("password")
	name := r.FormValue("name")
	phone := r.FormValue("phone")
	if len(password) < 8 {
		d := h.Public.PageData(r)
		httpx.Render(w, r, views.Register(d, "Password minimal 8 karakter", email, name, phone))
		return
	}
	id, err := h.Auth.RegisterCustomer(r.Context(), email, password, name, phone)
	if err != nil {
		d := h.Public.PageData(r)
		httpx.Render(w, r, views.Register(d, "Gagal: "+err.Error(), email, name, phone))
		return
	}
	h.Sessions.Put(r.Context(), "user_id", id.String())
	h.Sessions.Put(r.Context(), "user_role", "customer")
	h.Sessions.Put(r.Context(), "user_email", email)
	h.Sessions.RenewToken(r.Context())
	if tok := middleware.SessionToken(r); tok != "" {
		_ = h.Cart.MergeOnLogin(r.Context(), tok, id)
	}
	http.Redirect(w, r, "/account", http.StatusFound)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	_ = h.Sessions.Destroy(r.Context())
	http.Redirect(w, r, "/", http.StatusFound)
}

// Change password (customer/reseller self-service)

func (h *AuthHandler) ShowChangePassword(w http.ResponseWriter, r *http.Request) {
	d := h.Public.PageData(r)
	d.Title = "Ganti Password"
	d.NoIndex = true
	httpx.Render(w, r, views.ChangePassword(d, "", ""))
}

func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserID(r)
	if uid == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	r.ParseForm()
	cur := r.FormValue("current")
	nw := r.FormValue("new")
	cf := r.FormValue("confirm")
	d := h.Public.PageData(r)
	d.Title = "Ganti Password"
	d.NoIndex = true
	if nw != cf {
		httpx.Render(w, r, views.ChangePassword(d, "Password baru tidak sama", ""))
		return
	}
	if len(nw) < 8 {
		httpx.Render(w, r, views.ChangePassword(d, "Minimal 8 karakter", ""))
		return
	}
	var hash string
	if err := h.Pool.QueryRow(r.Context(), `SELECT password_hash FROM users WHERE id=$1`, *uid).Scan(&hash); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	ok, _ := security.VerifyPassword(cur, hash)
	if !ok {
		httpx.Render(w, r, views.ChangePassword(d, "Password lama salah", ""))
		return
	}
	newHash, err := security.HashPassword(nw)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	_, _ = h.Pool.Exec(r.Context(), `UPDATE users SET password_hash=$2, updated_at=now() WHERE id=$1`, *uid, newHash)
	// Notify via email
	email := middleware.UserEmail(r)
	if email != "" {
		store := ""
		if d.Store.Name != "" {
			store = d.Store.Name
		}
		subj, body := mailer.PasswordChanged(store, "")
		h.Mailer.SendAsync(r.Context(), email, subj, body)
	}
	httpx.Render(w, r, views.ChangePassword(d, "", "Password berhasil diubah."))
}

// Account is now handled by AccountHandler.Home (see account.go).
// Function kept here removed; route updated in app.go.

func (h *AuthHandler) OrderHistory(w http.ResponseWriter, r *http.Request) {
	uid, _ := r.Context().Value(middleware.UserIDKey).(uuid.UUID)
	rows, err := h.Pool.Query(r.Context(), `SELECT id, code, channel, status, payment_status, grand_total, created_at FROM orders WHERE user_id=$1 ORDER BY created_at DESC LIMIT 100`, uid)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()
	type Row struct {
		ID            uuid.UUID
		Code          string
		Channel       string
		Status        string
		PaymentStatus string
		GrandTotal    float64
	}
	d := h.Public.PageData(r)
	d.Title = "Pesanan Saya"
	d.NoIndex = true
	// reuse models.Order
	orders := []*orderRow{}
	for rows.Next() {
		var o orderRow
		if err := rows.Scan(&o.ID, &o.Code, &o.Channel, &o.Status, &o.PaymentStatus, &o.GrandTotal, &o.CreatedAt); err == nil {
			orders = append(orders, &o)
		}
	}
	httpx.Render(w, r, views.OrderHistory(d, toModelOrders(orders)))
}
