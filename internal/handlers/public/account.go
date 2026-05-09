package public

import (
	"net/http"
	"strings"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tokoonline/app/internal/httpx"
	"github.com/tokoonline/app/internal/middleware"
	"github.com/tokoonline/app/internal/models"
	"github.com/tokoonline/app/internal/services/cart"
	"github.com/tokoonline/app/internal/services/order"
	"github.com/tokoonline/app/internal/services/pricing"
	views "github.com/tokoonline/app/internal/views/public"
)

type AccountHandler struct {
	Pool     *pgxpool.Pool
	Cart     *cart.Service
	Pricing  *pricing.Service
	Order    *order.Service
	Sessions *scs.SessionManager
	Public   *Handler
}

// Home: enhanced dashboard with stats + recent orders + default address.
func (h *AccountHandler) Home(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserID(r)
	if uid == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	st := views.AccountStats{
		UserEmail: middleware.UserEmail(r),
	}
	var name, phone *string
	_ = h.Pool.QueryRow(r.Context(), `SELECT full_name, phone FROM users WHERE id=$1`, *uid).Scan(&name, &phone)
	if name != nil {
		st.UserName = *name
	}
	if phone != nil {
		st.UserPhone = *phone
	}
	_ = h.Pool.QueryRow(r.Context(), `SELECT count(*) FROM orders WHERE user_id=$1`, *uid).Scan(&st.OrderCount)
	_ = h.Pool.QueryRow(r.Context(), `SELECT count(*) FROM orders WHERE user_id=$1 AND payment_status<>'paid' AND status NOT IN ('cancelled','refunded')`, *uid).Scan(&st.UnpaidCount)
	_ = h.Pool.QueryRow(r.Context(), `SELECT COALESCE(sum(grand_total),0) FROM orders WHERE user_id=$1 AND payment_status='paid'`, *uid).Scan(&st.TotalSpent)
	_ = h.Pool.QueryRow(r.Context(), `SELECT count(*) FROM customer_addresses WHERE user_id=$1`, *uid).Scan(&st.AddressCount)
	recent := h.recentOrders(r, *uid, 5)
	addrs := h.listAddresses(r, *uid)
	d := h.Public.PageData(r)
	d.Title = "Akun Saya"
	d.NoIndex = true
	httpx.Render(w, r, views.AccountHome(d, st, recent, addrs))
}

func (h *AccountHandler) recentOrders(r *http.Request, uid uuid.UUID, limit int) []*models.Order {
	rows, err := h.Pool.Query(r.Context(), `SELECT id, code, channel, status, payment_status, grand_total, created_at FROM orders WHERE user_id=$1 ORDER BY created_at DESC LIMIT $2`, uid, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []*models.Order{}
	for rows.Next() {
		o := &models.Order{}
		if err := rows.Scan(&o.ID, &o.Code, &o.Channel, &o.Status, &o.PaymentStatus, &o.GrandTotal, &o.CreatedAt); err == nil {
			out = append(out, o)
		}
	}
	return out
}

func (h *AccountHandler) listAddresses(r *http.Request, uid uuid.UUID) []*models.CustomerAddress {
	rows, err := h.Pool.Query(r.Context(), `SELECT id,user_id,label,recipient,phone,address,province,city,district,postal_code,area_id,is_default,created_at FROM customer_addresses WHERE user_id=$1 ORDER BY is_default DESC, created_at DESC`, uid)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []*models.CustomerAddress{}
	for rows.Next() {
		a := &models.CustomerAddress{}
		if err := rows.Scan(&a.ID, &a.UserID, &a.Label, &a.Recipient, &a.Phone, &a.Address, &a.Province, &a.City, &a.District, &a.PostalCode, &a.AreaID, &a.IsDefault, &a.CreatedAt); err == nil {
			out = append(out, a)
		}
	}
	return out
}

// ─── Profile edit ────────────────────────

func (h *AccountHandler) ShowProfile(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserID(r)
	if uid == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	var name, phone *string
	_ = h.Pool.QueryRow(r.Context(), `SELECT full_name, phone FROM users WHERE id=$1`, *uid).Scan(&name, &phone)
	d := h.Public.PageData(r)
	d.Title = "Profil"
	d.NoIndex = true
	httpx.Render(w, r, views.Profile(d, derefStrAcc(name), derefStrAcc(phone), middleware.UserEmail(r), "", ""))
}

func (h *AccountHandler) SaveProfile(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserID(r)
	if uid == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	r.ParseForm()
	name := strings.TrimSpace(r.FormValue("name"))
	phone := strings.TrimSpace(r.FormValue("phone"))
	if name == "" {
		d := h.Public.PageData(r)
		httpx.Render(w, r, views.Profile(d, name, phone, middleware.UserEmail(r), "Nama tidak boleh kosong", ""))
		return
	}
	_, err := h.Pool.Exec(r.Context(), `UPDATE users SET full_name=$2, phone=NULLIF($3,''), updated_at=now() WHERE id=$1`, *uid, name, phone)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	d := h.Public.PageData(r)
	d.Title = "Profil"
	d.NoIndex = true
	httpx.Render(w, r, views.Profile(d, name, phone, middleware.UserEmail(r), "", "Profil berhasil diperbarui."))
}

// ─── Address book CRUD ───────────────────

func (h *AccountHandler) Addresses(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserID(r)
	if uid == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	addrs := h.listAddresses(r, *uid)
	d := h.Public.PageData(r)
	d.Title = "Alamat Saya"
	d.NoIndex = true
	httpx.Render(w, r, views.Addresses(d, addrs, views.AddressFormData{}, ""))
}

func (h *AccountHandler) AddressEdit(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserID(r)
	if uid == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	a := &models.CustomerAddress{}
	err = h.Pool.QueryRow(r.Context(), `SELECT id,user_id,label,recipient,phone,address,province,city,district,postal_code,area_id,is_default,created_at FROM customer_addresses WHERE id=$1 AND user_id=$2`, id, *uid).
		Scan(&a.ID, &a.UserID, &a.Label, &a.Recipient, &a.Phone, &a.Address, &a.Province, &a.City, &a.District, &a.PostalCode, &a.AreaID, &a.IsDefault, &a.CreatedAt)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	addrs := h.listAddresses(r, *uid)
	form := views.AddressFormData{
		IsEdit: true, ID: a.ID.String(),
		Label: a.Label, Recipient: a.Recipient, Phone: a.Phone, Address: a.Address,
		Province: a.Province, City: a.City, PostalCode: a.PostalCode,
		District:  derefStrAcc(a.District),
		AreaID:    derefStrAcc(a.AreaID),
		IsDefault: a.IsDefault,
	}
	d := h.Public.PageData(r)
	d.Title = "Edit Alamat"
	d.NoIndex = true
	httpx.Render(w, r, views.Addresses(d, addrs, form, ""))
}

func (h *AccountHandler) AddressCreate(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserID(r)
	if uid == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	r.ParseForm()
	isDefault := r.FormValue("is_default") == "1"
	if isDefault {
		_, _ = h.Pool.Exec(r.Context(), `UPDATE customer_addresses SET is_default=FALSE WHERE user_id=$1`, *uid)
	}
	_, err := h.Pool.Exec(r.Context(), `INSERT INTO customer_addresses(user_id,label,recipient,phone,address,province,city,district,postal_code,area_id,is_default) VALUES($1,$2,$3,$4,$5,$6,$7,NULLIF($8,''),$9,NULLIF($10,''),$11)`,
		*uid,
		strings.TrimSpace(r.FormValue("label")),
		strings.TrimSpace(r.FormValue("recipient")),
		strings.TrimSpace(r.FormValue("phone")),
		strings.TrimSpace(r.FormValue("address")),
		strings.TrimSpace(r.FormValue("province")),
		strings.TrimSpace(r.FormValue("city")),
		strings.TrimSpace(r.FormValue("district")),
		strings.TrimSpace(r.FormValue("postal_code")),
		strings.TrimSpace(r.FormValue("area_id")),
		isDefault)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/account/addresses", http.StatusFound)
}

func (h *AccountHandler) AddressUpdate(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserID(r)
	if uid == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	r.ParseForm()
	isDefault := r.FormValue("is_default") == "1"
	if isDefault {
		_, _ = h.Pool.Exec(r.Context(), `UPDATE customer_addresses SET is_default=FALSE WHERE user_id=$1 AND id<>$2`, *uid, id)
	}
	_, err = h.Pool.Exec(r.Context(), `UPDATE customer_addresses SET label=$3,recipient=$4,phone=$5,address=$6,province=$7,city=$8,district=NULLIF($9,''),postal_code=$10,area_id=NULLIF($11,''),is_default=$12,updated_at=now() WHERE id=$1 AND user_id=$2`,
		id, *uid,
		strings.TrimSpace(r.FormValue("label")),
		strings.TrimSpace(r.FormValue("recipient")),
		strings.TrimSpace(r.FormValue("phone")),
		strings.TrimSpace(r.FormValue("address")),
		strings.TrimSpace(r.FormValue("province")),
		strings.TrimSpace(r.FormValue("city")),
		strings.TrimSpace(r.FormValue("district")),
		strings.TrimSpace(r.FormValue("postal_code")),
		strings.TrimSpace(r.FormValue("area_id")),
		isDefault)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/account/addresses", http.StatusFound)
}

func (h *AccountHandler) AddressDelete(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserID(r)
	if uid == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_, _ = h.Pool.Exec(r.Context(), `DELETE FROM customer_addresses WHERE id=$1 AND user_id=$2`, id, *uid)
	http.Redirect(w, r, "/account/addresses", http.StatusFound)
}

func (h *AccountHandler) AddressMakeDefault(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserID(r)
	if uid == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_, _ = h.Pool.Exec(r.Context(), `UPDATE customer_addresses SET is_default=FALSE WHERE user_id=$1`, *uid)
	_, _ = h.Pool.Exec(r.Context(), `UPDATE customer_addresses SET is_default=TRUE, updated_at=now() WHERE id=$1 AND user_id=$2`, id, *uid)
	http.Redirect(w, r, "/account/addresses", http.StatusFound)
}

// ─── Reorder ─────────────────────────────

func (h *AccountHandler) Reorder(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserID(r)
	if uid == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	code := chi.URLParam(r, "code")
	o, err := h.Order.GetByCode(r.Context(), code)
	if err != nil || o.UserID == nil || *o.UserID != *uid {
		http.NotFound(w, r)
		return
	}
	aud, _ := h.Pricing.AudienceForUser(r.Context(), uid)
	c, err := h.Cart.GetOrCreate(r.Context(), middleware.SessionToken(r), uid, aud.Code)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	added := 0
	for _, it := range o.Items {
		if it.VariantID == nil {
			continue
		}
		if err := h.Cart.Add(r.Context(), c, *it.VariantID, it.Qty, aud.Code, aud.DiscountPct); err == nil {
			added++
		}
	}
	if added == 0 {
		http.Redirect(w, r, "/cart?reorder=empty", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/checkout?reorder="+code, http.StatusFound)
}

func derefStrAcc(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
