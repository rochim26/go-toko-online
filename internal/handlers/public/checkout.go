package public

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tokoonline/app/internal/httpx"
	"github.com/tokoonline/app/internal/middleware"
	"github.com/tokoonline/app/internal/services/cart"
	"github.com/tokoonline/app/internal/services/mailer"
	"github.com/tokoonline/app/internal/services/order"
	"github.com/tokoonline/app/internal/services/payment"
	"github.com/tokoonline/app/internal/services/pricing"
	"github.com/tokoonline/app/internal/services/security"
	"github.com/tokoonline/app/internal/services/settings"
	"github.com/tokoonline/app/internal/services/shipping"
	"github.com/tokoonline/app/internal/services/tracking"
	views "github.com/tokoonline/app/internal/views/public"
)

type CheckoutHandler struct {
	Pool     *pgxpool.Pool
	Settings *settings.Store
	Cart     *cart.Service
	Pricing  *pricing.Service
	Order    *order.Service
	Xendit   *payment.Xendit
	Biteship *shipping.Biteship
	Tracking *tracking.Service
	Mailer   *mailer.Mailer
	BaseURL  string
	Sessions *scs.SessionManager
	Public   *Handler
}

func (h *CheckoutHandler) Show(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserID(r)
	aud, _ := h.Pricing.AudienceForUser(r.Context(), uid)
	c, _ := h.Cart.GetOrCreate(r.Context(), middleware.SessionToken(r), uid, aud.Code)
	d := h.Public.PageData(r)
	d.Title = "Checkout"
	d.NoIndex = true

	email := middleware.UserEmail(r)
	var name, phone string
	channel := "b2c"
	isTOP := false
	topDays := 0
	if aud.Code != "b2c" {
		channel = "b2b"
		if aud.TopDays > 0 {
			isTOP = true
			topDays = aud.TopDays
		}
	}

	// Pre-fill identity + saved addresses from logged-in user
	var saved []views.SavedAddress
	if uid != nil {
		var nm, ph *string
		_ = h.Pool.QueryRow(r.Context(), `SELECT full_name, phone FROM users WHERE id=$1`, *uid).Scan(&nm, &ph)
		if nm != nil {
			name = *nm
		}
		if ph != nil {
			phone = *ph
		}
		rows, err := h.Pool.Query(r.Context(), `
			SELECT id::text, label, recipient, phone, address, province, city,
			       COALESCE(district,''), postal_code, COALESCE(area_id,''), is_default
			FROM customer_addresses WHERE user_id=$1 ORDER BY is_default DESC, created_at DESC LIMIT 6`, *uid)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var a views.SavedAddress
				if err := rows.Scan(&a.ID, &a.Label, &a.Recipient, &a.Phone, &a.Address, &a.Province, &a.City, &a.District, &a.PostalCode, &a.AreaID, &a.IsDefault); err == nil {
					saved = append(saved, a)
				}
			}
		}
	}

	httpx.Render(w, r, views.Checkout(d, views.CheckoutData{
		Cart:           c,
		Subtotal:       cart.Subtotal(c),
		Weight:         cart.TotalWeightGrams(c),
		Email:          email,
		Name:           name,
		Phone:          phone,
		Channel:        channel,
		IsTOP:          isTOP,
		TOPDays:        topDays,
		SavedAddresses: saved,
	}))
}

func (h *CheckoutHandler) Submit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	uid := middleware.UserID(r)
	aud, _ := h.Pricing.AudienceForUser(r.Context(), uid)
	c, err := h.Cart.GetOrCreate(r.Context(), middleware.SessionToken(r), uid, aud.Code)
	if err != nil || len(c.Items) == 0 {
		writeJSON(w, map[string]any{"ok": false, "error": "Keranjang kosong"})
		return
	}

	// Validate B2B MOQ
	if aud.TierID != nil {
		if cart.TotalQty(c) < aud.MoqQty {
			writeJSON(w, map[string]any{"ok": false, "error": fmt.Sprintf("MOQ tier Anda minimal %d item", aud.MoqQty)})
			return
		}
		if cart.Subtotal(c) < aud.MoqValue {
			writeJSON(w, map[string]any{"ok": false, "error": fmt.Sprintf("Minimum nilai pesanan tier Anda Rp %.0f", aud.MoqValue)})
			return
		}
	}

	address := order.Address{
		Recipient:  r.FormValue("name"),
		Phone:      r.FormValue("phone"),
		Address:    r.FormValue("ship_address"),
		Province:   r.FormValue("ship_province"),
		City:       r.FormValue("ship_city"),
		District:   r.FormValue("ship_district"),
		PostalCode: r.FormValue("ship_postal_code"),
		AreaID:     r.FormValue("ship_area_id"),
	}
	shipping_total, _ := strconv.ParseFloat(r.FormValue("shipping_total"), 64)
	paymentTerm := r.FormValue("payment_term")
	if paymentTerm == "" {
		paymentTerm = "prepaid"
	}
	if paymentTerm == "top" && (aud.TierID == nil || aud.TopDays <= 0) {
		paymentTerm = "prepaid"
	}

	tax := h.Settings.Tax()
	taxTotal := 0.0
	if tax.PPNPct > 0 {
		taxTotal = (cart.Subtotal(c) * tax.PPNPct) / 100
	}

	channel := "b2c"
	if aud.Code != "b2c" {
		channel = "b2b"
	}

	utm := map[string]string{}
	for _, k := range []string{"utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content"} {
		if v := r.FormValue(k); v != "" {
			utm[k] = v
		}
	}

	items := make([]order.OrderItemInput, 0, len(c.Items))
	for _, it := range c.Items {
		items = append(items, order.OrderItemInput{
			VariantID: it.VariantID, SKU: it.VariantSKU, Name: it.ProductName,
			Qty: it.Qty, UnitPrice: it.UnitPrice,
			ImageURL: derefPtr(it.ImageURL),
		})
	}

	// Save the address as a "default" if user is logged in and has none yet
	if uid != nil && address.Address != "" {
		var n int
		_ = h.Pool.QueryRow(r.Context(), `SELECT count(*) FROM customer_addresses WHERE user_id=$1`, *uid).Scan(&n)
		if n == 0 {
			_, _ = h.Pool.Exec(r.Context(), `INSERT INTO customer_addresses(user_id,label,recipient,phone,address,province,city,district,postal_code,area_id,is_default) VALUES($1,'Utama',$2,$3,$4,$5,$6,NULLIF($7,''),$8,NULLIF($9,''),TRUE)`,
				*uid, address.Recipient, address.Phone, address.Address, address.Province, address.City, address.District, address.PostalCode, address.AreaID)
		}
	}

	o, err := h.Order.Create(r.Context(), order.CreateInput{
		UserID:         uid,
		Channel:        channel,
		PaymentTerm:    paymentTerm,
		TopDays:        aud.TopDays,
		CustomerEmail:  r.FormValue("email"),
		CustomerName:   r.FormValue("name"),
		CustomerPhone:  r.FormValue("phone"),
		Address:        address,
		CourierCode:    r.FormValue("courier_code"),
		CourierService: r.FormValue("courier_service"),
		ShippingTotal:  shipping_total,
		TaxTotal:       taxTotal,
		VoucherCode:    r.FormValue("voucher_code"),
		Notes:          r.FormValue("notes"),
		UTM:            utm,
		FBP:            httpx.Cookie(r, "_fbp"),
		FBC:            httpx.Cookie(r, "_fbc"),
		ClientIP:       httpx.ClientIP(r),
		UserAgent:      r.UserAgent(),
		Items:          items,
		InvoicePrefix:  tax.InvoicePrefix,
	})
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	// clear cart
	_ = h.Cart.Clear(r.Context(), c.ID)

	// fire CAPI Purchase event (best-effort) - only if prepaid+xendit-flow we usually fire after webhook,
	// but InitiateCheckout is appropriate here.
	go func() {
		ev := tracking.Event{
			EventID:   "ic-" + o.Code,
			EventName: "InitiateCheckout",
			URL:       h.BaseURL + "/checkout",
			UserAgent: r.UserAgent(),
			IP:        httpx.ClientIP(r),
			Email:     r.FormValue("email"),
			Phone:     r.FormValue("phone"),
			FBP:       httpx.Cookie(r, "_fbp"),
			FBC:       httpx.Cookie(r, "_fbc"),
			Currency:  "IDR",
			Value:     o.GrandTotal,
		}
		_ = h.Tracking.SendMetaCAPI(context.Background(), ev)
		_ = h.Tracking.SendGA4MP(context.Background(), middleware.SessionToken(r), ev)
	}()

	if paymentTerm == "top" {
		poNum := "PO-" + o.Code
		_, _ = h.Pool.Exec(r.Context(), `INSERT INTO po_documents(order_id,po_number,due_at) VALUES($1,$2,$3)`, o.ID, poNum, o.TopDueAt)
		// Email confirmation
		if email := r.FormValue("email"); email != "" {
			o.CustomerEmail = &email
			o.CustomerName = strPtr(r.FormValue("name"))
			subj, body := mailer.OrderConfirmation(h.Settings.Store().Name, h.BaseURL, o)
			h.Mailer.SendAsync(r.Context(), email, subj, body)
		}
		writeJSON(w, map[string]any{"ok": true, "redirect": "/order/success?code=" + o.Code})
		return
	}

	cfg := h.Settings.Xendit()
	successURL := h.BaseURL + "/order/success?code=" + o.Code
	failURL := h.BaseURL + "/order/failed?code=" + o.Code
	inv, err := h.Xendit.CreateInvoice(r.Context(), payment.InvoiceRequest{
		ExternalID:         o.Code,
		Amount:             o.GrandTotal,
		PayerEmail:         r.FormValue("email"),
		Description:        h.Settings.Store().Name + " · " + o.Code,
		Customer:           payment.Customer{GivenNames: r.FormValue("name"), Email: r.FormValue("email"), MobileNumber: r.FormValue("phone")},
		SuccessRedirectURL: successURL,
		FailureRedirectURL: failURL,
		PaymentMethods:     cfg.MethodsEnabled,
	})
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "Gagal membuat invoice: " + err.Error()})
		return
	}
	_ = h.Order.AttachXenditInvoice(r.Context(), o.ID, inv.ID, inv.InvoiceURL)
	// Email order confirmation with invoice URL
	if email := r.FormValue("email"); email != "" {
		o.CustomerEmail = &email
		o.CustomerName = strPtr(r.FormValue("name"))
		o.XenditInvoiceURL = &inv.InvoiceURL
		subj, body := mailer.OrderConfirmation(h.Settings.Store().Name, h.BaseURL, o)
		h.Mailer.SendAsync(r.Context(), email, subj, body)
	}
	writeJSON(w, map[string]any{"ok": true, "redirect": "/order/success?code=" + o.Code})
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ---- Shipping APIs ----

func (h *CheckoutHandler) AreasSearch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) < 3 {
		// Empty -> hide dropdown
		w.Write([]byte(""))
		return
	}
	areas, err := h.Biteship.SearchArea(r.Context(), q)
	if err != nil {
		fmt.Fprintf(w, `<div class="area-item" style="color:#991b1b">Gagal mencari: %s</div>`, htmlEscape(err.Error()))
		return
	}
	if len(areas) == 0 {
		w.Write([]byte(`<div class="area-item muted">Tidak ada hasil. Coba kata kunci lain.</div>`))
		return
	}
	for _, a := range areas {
		label := a.AdminLevel3
		if a.AdminLevel2 != "" {
			label += ", " + a.AdminLevel2
		}
		if a.AdminLevel1 != "" {
			label += ", " + a.AdminLevel1
		}
		meta := a.PostalCode
		if a.PostalCode == "" {
			meta = "—"
		}
		fmt.Fprintf(w, `<button type="button" class="area-item" data-area="%s" data-province="%s" data-city="%s" data-district="%s" data-postal="%s" data-label="%s">
			<div class="area-item-name">%s</div>
			<div class="area-item-meta">Kode pos: %s</div>
		</button>`,
			htmlEscape(a.ID), htmlEscape(a.AdminLevel1), htmlEscape(a.AdminLevel2), htmlEscape(a.AdminLevel3),
			htmlEscape(a.PostalCode), htmlEscape(label),
			htmlEscape(label), htmlEscape(meta))
	}
}

func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")
	return r.Replace(s)
}

func (h *CheckoutHandler) Rates(w http.ResponseWriter, r *http.Request) {
	areaID := r.URL.Query().Get("ship_area_id")
	postal := r.URL.Query().Get("ship_postal_code")
	weight, _ := strconv.Atoi(r.URL.Query().Get("weight"))
	if areaID == "" && postal == "" {
		fmt.Fprint(w, `<div class="muted">Isi alamat dulu.</div>`)
		return
	}
	uid := middleware.UserID(r)
	aud, _ := h.Pricing.AudienceForUser(r.Context(), uid)
	c, _ := h.Cart.GetOrCreate(r.Context(), middleware.SessionToken(r), uid, aud.Code)
	if weight <= 0 {
		weight = cart.TotalWeightGrams(c)
	}
	items := []shipping.RateItem{}
	for _, it := range c.Items {
		items = append(items, shipping.RateItem{Name: it.ProductName, Value: it.UnitPrice, Quantity: it.Qty, Weight: it.WeightGrams})
	}
	rates, err := h.Biteship.Rates(r.Context(), shipping.RateRequest{
		DestAreaID:     areaID,
		DestPostalCode: postal,
		Items:          items,
	})
	if err != nil {
		fmt.Fprintf(w, `<div class="flash flash-error">Gagal: %s</div>`, err.Error())
		return
	}
	if len(rates) == 0 {
		fmt.Fprint(w, `<div class="muted">Tidak ada layanan untuk alamat ini.</div>`)
		return
	}
	for _, r2 := range rates {
		fmt.Fprintf(w, `<button type="button" data-rate="1" data-code="%s" data-service="%s" data-price="%.0f" class="card" style="display:block;width:100%%;text-align:left;cursor:pointer;margin-bottom:.5rem">
			<div style="display:flex;justify-content:space-between;align-items:center">
				<div><strong>%s</strong> · %s<br/><span class="muted" style="font-size:.85rem">%s</span></div>
				<div style="font-weight:700">Rp %s</div>
			</div></button>`,
			r2.CourierCode, r2.CourierService, r2.Price, strings.ToUpper(r2.CourierName), r2.ServiceName, r2.ETD, formatThousand(r2.Price))
	}
}

func (h *CheckoutHandler) OrderSuccessPage(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	o, err := h.Order.GetByCode(r.Context(), code)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	d := h.Public.PageData(r)
	d.Title = "Pesanan Berhasil"
	d.NoIndex = true
	httpx.Render(w, r, views.OrderSuccess(d, o))
}

func (h *CheckoutHandler) OrderFailedPage(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	o, err := h.Order.GetByCode(r.Context(), code)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	d := h.Public.PageData(r)
	d.Title = "Pembayaran Gagal"
	d.NoIndex = true
	httpx.Render(w, r, views.OrderFailedPage(d, o))
}

// Order detail (public if logged in OR via code lookup token-less - since we redirect after checkout,
// for security we require user_id matches OR the request is from the same session that just placed the order)
func (h *CheckoutHandler) OrderShow(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	o, err := h.Order.GetByCode(r.Context(), code)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	uid := middleware.UserID(r)
	if o.UserID != nil && (uid == nil || *uid != *o.UserID) {
		http.Error(w, "forbidden", 403)
		return
	}
	d := h.Public.PageData(r)
	d.Title = "Pesanan " + o.Code
	d.NoIndex = true
	httpx.Render(w, r, views.OrderStatus(d, o))
}

// Xendit webhook
func (h *CheckoutHandler) XenditWebhook(w http.ResponseWriter, r *http.Request) {
	if !h.Xendit.VerifyWebhook(r.Header.Get("X-CALLBACK-TOKEN")) {
		http.Error(w, "unauthorized", 401)
		return
	}
	var body payment.WebhookInvoice
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad body", 400)
		return
	}
	if body.Status != "PAID" {
		w.WriteHeader(http.StatusOK)
		return
	}
	o, err := h.Order.MarkPaid(r.Context(), body.ExternalID, body.PaymentChannel, body.PaidAmount)
	if err != nil {
		w.WriteHeader(http.StatusOK)
		return
	}
	raw, _ := json.Marshal(body)
	_, _ = h.Pool.Exec(r.Context(), `INSERT INTO payments(order_id,provider,provider_ref,amount,method,status,raw) VALUES($1,'xendit',$2,$3,$4,'paid',$5)`,
		o.ID, body.ID, body.PaidAmount, body.PaymentChannel, raw)
	go func() {
		event := tracking.Event{
			EventID:   "p-" + o.Code,
			EventName: "Purchase",
			URL:       h.BaseURL + "/account/orders/" + o.Code,
			IP:        httpx.ClientIP(r),
			Email:     derefPtr(o.CustomerEmail),
			Phone:     derefPtr(o.CustomerPhone),
			Currency:  "IDR",
			Value:     o.GrandTotal,
		}
		_ = h.Tracking.SendMetaCAPI(context.Background(), event)
		_ = h.Tracking.SendGA4MP(context.Background(), o.Code, event)
	}()
	// Email payment receipt
	if o.CustomerEmail != nil && *o.CustomerEmail != "" {
		full, _ := h.Order.GetByCode(context.Background(), o.Code)
		if full != nil {
			subj, mbody := mailer.PaymentReceived(h.Settings.Store().Name, h.BaseURL, full)
			h.Mailer.SendAsync(context.Background(), *o.CustomerEmail, subj, mbody)
		}
	}
	w.WriteHeader(http.StatusOK)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func derefPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func formatThousand(v float64) string {
	intPart := int64(v + 0.5)
	s := fmt.Sprintf("%d", intPart)
	out := []byte{}
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, byte(c))
	}
	return string(out)
}

// helper exported for crypto-secure tokens (for future)
var _ = security.RandomToken
