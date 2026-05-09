package admin

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/tokoonline/app/internal/httpx"
	"github.com/tokoonline/app/internal/models"
	"github.com/tokoonline/app/internal/services/integrations"
	"github.com/tokoonline/app/internal/views/admin"
)

// ─── Customers ──────────────────────────

func (h *Handler) Customers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.Pool.Query(r.Context(), `
		SELECT u.id::text, u.email, COALESCE(u.full_name,''), COALESCE(u.phone,''), u.role,
			COALESCE((SELECT count(*) FROM orders o WHERE o.user_id = u.id),0) AS order_count,
			COALESCE((SELECT sum(grand_total) FROM orders o WHERE o.user_id = u.id AND payment_status='paid'),0) AS total_spent,
			u.created_at
		FROM users u
		WHERE u.role IN ('customer','reseller')
		ORDER BY u.created_at DESC
		LIMIT 200`)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()
	items := []admin.CustomerListItem{}
	for rows.Next() {
		var it admin.CustomerListItem
		if err := rows.Scan(&it.ID, &it.Email, &it.FullName, &it.Phone, &it.Role,
			&it.OrderCount, &it.TotalSpent, &it.CreatedAt); err == nil {
			items = append(items, it)
		}
	}
	d := h.PageData(r)
	d.Title = "Pelanggan"
	d.NoIndex = true
	httpx.Render(w, r, admin.Customers(d, items))
}

// ─── Pages CMS ──────────────────────────

func (h *Handler) Pages(w http.ResponseWriter, r *http.Request) {
	rows, err := h.Pool.Query(r.Context(), `SELECT id,slug,title,body_html,seo_title,seo_desc,is_published,updated_at FROM pages ORDER BY slug`)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()
	pages := []*models.Page{}
	for rows.Next() {
		p := &models.Page{}
		if err := rows.Scan(&p.ID, &p.Slug, &p.Title, &p.BodyHTML, &p.SeoTitle, &p.SeoDesc, &p.IsPublished, &p.UpdatedAt); err == nil {
			pages = append(pages, p)
		}
	}
	d := h.PageData(r)
	d.Title = "Halaman"
	d.NoIndex = true
	httpx.Render(w, r, admin.Pages(d, pages))
}

func (h *Handler) PageUpsert(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	slug := strings.TrimSpace(r.FormValue("slug"))
	title := strings.TrimSpace(r.FormValue("title"))
	if slug == "" || title == "" {
		http.Error(w, "slug & title wajib", 400)
		return
	}
	pub := r.FormValue("is_published") != ""
	_, err := h.Pool.Exec(r.Context(), `
		INSERT INTO pages(slug,title,body_html,seo_title,seo_desc,is_published) VALUES($1,$2,$3,$4,$5,$6)
		ON CONFLICT(slug) DO UPDATE SET title=EXCLUDED.title, body_html=EXCLUDED.body_html,
			seo_title=EXCLUDED.seo_title, seo_desc=EXCLUDED.seo_desc, is_published=EXCLUDED.is_published, updated_at=now()`,
		slug, title, r.FormValue("body_html"), nullStrA(r.FormValue("seo_title")), nullStrA(r.FormValue("seo_desc")), pub)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/admin/pages", http.StatusFound)
}

func (h *Handler) PageDelete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_, _ = h.Pool.Exec(r.Context(), `DELETE FROM pages WHERE id=$1`, id)
	http.Redirect(w, r, "/admin/pages", http.StatusFound)
}

// ─── Redirects ─────────────────────────

func (h *Handler) Redirects(w http.ResponseWriter, r *http.Request) {
	rows, err := h.Pool.Query(r.Context(), `SELECT id::text,from_path,to_path,code,created_at FROM redirects ORDER BY created_at DESC`)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()
	items := []admin.RedirectItem{}
	for rows.Next() {
		var it admin.RedirectItem
		if err := rows.Scan(&it.ID, &it.From, &it.To, &it.Code, &it.CreatedAt); err == nil {
			items = append(items, it)
		}
	}
	d := h.PageData(r)
	d.Title = "Redirect"
	d.NoIndex = true
	httpx.Render(w, r, admin.Redirects(d, items))
}

func (h *Handler) RedirectCreate(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	from := strings.TrimSpace(r.FormValue("from_path"))
	to := strings.TrimSpace(r.FormValue("to_path"))
	code, _ := strconv.Atoi(r.FormValue("code"))
	if code == 0 {
		code = 301
	}
	if from == "" || to == "" {
		http.Error(w, "from & to wajib", 400)
		return
	}
	_, err := h.Pool.Exec(r.Context(), `INSERT INTO redirects(from_path,to_path,code) VALUES($1,$2,$3)
		ON CONFLICT(from_path) DO UPDATE SET to_path=EXCLUDED.to_path, code=EXCLUDED.code`, from, to, code)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/admin/redirects", http.StatusFound)
}

func (h *Handler) RedirectDelete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_, _ = h.Pool.Exec(r.Context(), `DELETE FROM redirects WHERE id=$1`, id)
	http.Redirect(w, r, "/admin/redirects", http.StatusFound)
}

// ─── GMC Feed Status ───────────────────

func (h *Handler) FeedStatus(w http.ResponseWriter, r *http.Request) {
	st := admin.FeedStats{}
	_ = h.Pool.QueryRow(r.Context(), `SELECT count(*) FROM products`).Scan(&st.TotalProducts)
	_ = h.Pool.QueryRow(r.Context(), `SELECT count(*) FROM products WHERE status='active' AND is_b2c=TRUE`).Scan(&st.ActiveProducts)
	_ = h.Pool.QueryRow(r.Context(), `SELECT count(*) FROM products WHERE status='active' AND is_b2c=TRUE AND gmc_enabled=TRUE
		AND COALESCE((SELECT SUM(on_hand-reserved) FROM inventory_levels il JOIN product_variants v ON v.id=il.variant_id WHERE v.product_id=products.id),0) > 0`).Scan(&st.GMCEnabled)
	_ = h.Pool.QueryRow(r.Context(), `SELECT count(*) FROM products WHERE status='active' AND
		COALESCE((SELECT SUM(on_hand-reserved) FROM inventory_levels il JOIN product_variants v ON v.id=il.variant_id WHERE v.product_id=products.id),0) <= 0`).Scan(&st.OutOfStock)
	_ = h.Pool.QueryRow(r.Context(), `SELECT count(*) FROM products WHERE status='active' AND is_b2c=TRUE AND COALESCE(gmc_gtin,'')='' AND COALESCE(gmc_mpn,'')=''`).Scan(&st.MissingGTIN)
	_ = h.Pool.QueryRow(r.Context(), `SELECT count(*) FROM products p WHERE p.status='active' AND is_b2c=TRUE AND NOT EXISTS (SELECT 1 FROM product_images i WHERE i.product_id=p.id)`).Scan(&st.MissingImage)
	d := h.PageData(r)
	d.Title = "GMC Feed Status"
	d.NoIndex = true
	gmcStat := integrations.Status{}
	if s := integrations.ByKey(d.Integrations, "gmc"); s != nil {
		gmcStat = *s
	}
	httpx.Render(w, r, admin.FeedStatus(d, h.BaseURL, st, gmcStat))
}

// ─── Sitemap Status ────────────────────

func (h *Handler) SitemapStatus(w http.ResponseWriter, r *http.Request) {
	st := admin.SitemapStats{BaseURL: h.BaseURL}
	_ = h.Pool.QueryRow(r.Context(), `SELECT count(*) FROM products WHERE status='active'`).Scan(&st.Products)
	_ = h.Pool.QueryRow(r.Context(), `SELECT count(*) FROM categories WHERE is_active=TRUE`).Scan(&st.Categories)
	_ = h.Pool.QueryRow(r.Context(), `SELECT count(*) FROM pages WHERE is_published=TRUE`).Scan(&st.Pages)
	_ = h.Pool.QueryRow(r.Context(), `SELECT count(*) FROM blog_posts WHERE is_published=TRUE`).Scan(&st.Posts)
	st.URLs = st.Products + st.Categories + st.Pages + st.Posts + 3 // home + catalog + blog
	d := h.PageData(r)
	d.Title = "Sitemap"
	d.NoIndex = true
	httpx.Render(w, r, admin.SitemapStatus(d, st))
}

// ─── Tour replay ───────────────────────

func (h *Handler) TourPage(w http.ResponseWriter, r *http.Request) {
	d := h.PageData(r)
	d.Title = "Tutorial"
	d.NoIndex = true
	httpx.Render(w, r, admin.TourReplay(d))
}
