package public

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tokoonline/app/internal/httpx"
	"github.com/tokoonline/app/internal/middleware"
	"github.com/tokoonline/app/internal/models"
	"github.com/tokoonline/app/internal/services/buildinfo"
	"github.com/tokoonline/app/internal/services/cart"
	"github.com/tokoonline/app/internal/services/catalog"
	"github.com/tokoonline/app/internal/services/order"
	"github.com/tokoonline/app/internal/services/pricing"
	"github.com/tokoonline/app/internal/services/seo"
	"github.com/tokoonline/app/internal/services/settings"
	"github.com/tokoonline/app/internal/views/layouts"
	views "github.com/tokoonline/app/internal/views/public"
)

type Handler struct {
	Pool     *pgxpool.Pool
	Settings *settings.Store
	Catalog  *catalog.Service
	Cart     *cart.Service
	Pricing  *pricing.Service
	Order    *order.Service
	BaseURL  string
}

func (h *Handler) PageData(r *http.Request) layouts.PageData {
	uid := middleware.UserID(r)
	cartCount := 0
	audience := "b2c"
	if uid != nil {
		aud, _ := h.Pricing.AudienceForUser(r.Context(), uid)
		audience = aud.Code
	}
	c, err := h.Cart.GetOrCreate(r.Context(), middleware.SessionToken(r), uid, audience)
	if err == nil {
		cartCount = cart.TotalQty(c)
	}
	return layouts.PageData{
		BaseURL:    h.BaseURL,
		AssetVer:   buildinfo.Version(),
		URL:        r.URL.Path,
		CSRFToken:  httpx.CSRFToken(r),
		IsLoggedIn: uid != nil,
		UserRole:   middleware.UserRole(r),
		UserEmail:  middleware.UserEmail(r),
		CartCount:  cartCount,
		Store:      h.Settings.Store(),
		SEO:        h.Settings.SEO(),
		Marketing:  h.Settings.Marketing(),
	}
}

func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	products, _, _ := h.Catalog.List(r.Context(), catalog.ListOpts{Limit: 12, OnlyB2C: true})
	cats, _ := h.Catalog.ListCategories(r.Context())
	d := h.PageData(r)
	d.URL = "/"
	d.JSONLD = []string{
		seo.OrgLD(d.Store.Name, h.BaseURL, d.Store.LogoURL),
		seo.WebSiteLD(d.Store.Name, h.BaseURL),
	}
	httpx.Render(w, r, views.Home(d, products, cats))
}

func (h *Handler) CatalogPage(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	cat, _ := h.Catalog.GetCategoryBySlug(r.Context(), slug)
	products, total, err := h.Catalog.List(r.Context(), catalog.ListOpts{
		CategorySlug: slug, Limit: 24, OnlyB2C: true,
	})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	d := h.PageData(r)
	d.URL = "/c/" + slug
	if cat != nil {
		d.Title = derefStr(cat.SeoTitle, cat.Name)
		d.Description = derefStr(cat.SeoDesc, "")
		d.JSONLD = []string{seo.BreadcrumbLD(h.BaseURL, []seo.Crumb{{Name: "Beranda", URL: "/"}, {Name: cat.Name, URL: "/c/" + cat.Slug}})}
	}
	httpx.Render(w, r, views.Catalog(d, cat, products, total, ""))
}

func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	products, total, _ := h.Catalog.List(r.Context(), catalog.ListOpts{Search: q, Limit: 48, OnlyB2C: true})
	d := h.PageData(r)
	d.URL = "/search?q=" + q
	d.Title = "Pencarian: " + q
	d.NoIndex = true
	httpx.Render(w, r, views.Catalog(d, nil, products, total, q))
}

func (h *Handler) Product(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	uid := middleware.UserID(r)
	aud, _ := h.Pricing.AudienceForUser(r.Context(), uid)
	p, err := h.Catalog.GetBySlug(r.Context(), slug, aud.Code)
	if err != nil {
		// fallback: maybe it's a content page (FAQ, About, etc.)
		h.Page(w, r)
		return
	}
	variants, _ := h.Catalog.GetVariants(r.Context(), p.ID)
	images, _ := h.Catalog.GetImages(r.Context(), p.ID)
	defaultPrice := p.MinPrice
	if len(variants) > 0 {
		defaultV := variants[0]
		for _, v := range variants {
			if v.IsDefault {
				defaultV = v
				break
			}
		}
		price, _ := h.Pricing.PriceFor(r.Context(), defaultV.ID, aud.Code, aud.DiscountPct)
		defaultPrice = price
	}
	d := h.PageData(r)
	d.URL = "/p/" + p.Slug
	d.Title = derefStr(p.SeoTitle, p.Name)
	d.Description = derefStr(p.SeoDesc, derefStr(p.ShortDesc, ""))
	d.OGImage = ""
	if len(images) > 0 {
		d.OGImage = absURL(h.BaseURL, images[0].URL)
	}
	imageURL := d.OGImage
	availability := "https://schema.org/InStock"
	if p.OnHand <= 0 {
		availability = "https://schema.org/OutOfStock"
	}
	d.JSONLD = []string{
		seo.ProductLD(p, h.BaseURL, imageURL, "IDR", defaultPrice, availability),
		seo.BreadcrumbLD(h.BaseURL, []seo.Crumb{
			{Name: "Beranda", URL: "/"},
			{Name: derefStr(p.CategoryName, "Katalog"), URL: "/c/" + derefStr(p.CategorySlug, "semua")},
			{Name: p.Name, URL: "/p/" + p.Slug},
		}),
	}
	if faq := seo.FAQLD(p.FAQs); faq != "" {
		d.JSONLD = append(d.JSONLD, faq)
	}
	httpx.Render(w, r, views.Product(d, p, variants, images, defaultPrice))
}

func (h *Handler) CartPage(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserID(r)
	aud, _ := h.Pricing.AudienceForUser(r.Context(), uid)
	c, err := h.Cart.GetOrCreate(r.Context(), middleware.SessionToken(r), uid, aud.Code)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	d := h.PageData(r)
	d.URL = "/cart"
	d.Title = "Keranjang"
	d.NoIndex = true
	httpx.Render(w, r, views.Cart(d, c, cart.Subtotal(c), cart.TotalWeightGrams(c)))
}

func (h *Handler) APICartAdd(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	vid, err := uuid.Parse(r.FormValue("variant_id"))
	if err != nil {
		http.Error(w, "invalid variant", 400)
		return
	}
	qty, _ := strconv.Atoi(r.FormValue("qty"))
	if qty <= 0 {
		qty = 1
	}
	uid := middleware.UserID(r)
	aud, _ := h.Pricing.AudienceForUser(r.Context(), uid)
	c, err := h.Cart.GetOrCreate(r.Context(), middleware.SessionToken(r), uid, aud.Code)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if err := h.Cart.Add(r.Context(), c, vid, qty, aud.Code, aud.DiscountPct); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	c, _ = h.Cart.GetOrCreate(r.Context(), middleware.SessionToken(r), uid, aud.Code)

	// "Buy now" flow: client requested immediate checkout — return HX-Redirect AND
	// JSON with redirect so both htmx-aware and bare fetch clients work.
	resp := map[string]any{"ok": true, "count": cart.TotalQty(c)}
	if r.FormValue("action") == "buy" {
		resp["redirect"] = "/checkout"
		w.Header().Set("HX-Redirect", "/checkout")
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) APICartUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	id, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, "invalid id", 400)
		return
	}
	qty, _ := strconv.Atoi(r.FormValue("qty"))
	uid := middleware.UserID(r)
	aud, _ := h.Pricing.AudienceForUser(r.Context(), uid)
	c, err := h.Cart.GetOrCreate(r.Context(), middleware.SessionToken(r), uid, aud.Code)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if err := h.Cart.UpdateQty(r.Context(), c.ID, id, qty); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	c, _ = h.Cart.GetOrCreate(r.Context(), middleware.SessionToken(r), uid, aud.Code)
	if httpx.IsHTMX(r) {
		w.Header().Set("HX-Refresh", "true")
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "count": cart.TotalQty(c)})
}

func (h *Handler) APICartRemove(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, "invalid id", 400)
		return
	}
	uid := middleware.UserID(r)
	aud, _ := h.Pricing.AudienceForUser(r.Context(), uid)
	c, err := h.Cart.GetOrCreate(r.Context(), middleware.SessionToken(r), uid, aud.Code)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if err := h.Cart.Remove(r.Context(), c.ID, id); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if httpx.IsHTMX(r) {
		w.Header().Set("HX-Refresh", "true")
	}
	w.Header().Set("Content-Type", "application/json")
	c, _ = h.Cart.GetOrCreate(r.Context(), middleware.SessionToken(r), uid, aud.Code)
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "count": cart.TotalQty(c)})
}

// --- Pages / Blog ---

func (h *Handler) Guides(w http.ResponseWriter, r *http.Request) {
	d := h.PageData(r)
	d.URL = "/panduan"
	d.Title = "Panduan Penggunaan"
	d.Description = "Panduan PDF untuk pembeli, reseller, dan admin"
	httpx.Render(w, r, views.Guides(d))
}

func (h *Handler) Page(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	p := &models.Page{}
	err := h.Pool.QueryRow(r.Context(), `SELECT id,slug,title,body_html,seo_title,seo_desc,is_published,updated_at FROM pages WHERE slug=$1 AND is_published=TRUE`, slug).
		Scan(&p.ID, &p.Slug, &p.Title, &p.BodyHTML, &p.SeoTitle, &p.SeoDesc, &p.IsPublished, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		// also try product slug
		if pp, e2 := h.Catalog.GetBySlug(r.Context(), slug, "b2c"); e2 == nil && pp != nil {
			http.Redirect(w, r, "/p/"+pp.Slug, http.StatusFound)
			return
		}
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	d := h.PageData(r)
	d.URL = "/p/" + slug
	d.Title = derefStr(p.SeoTitle, p.Title)
	d.Description = derefStr(p.SeoDesc, "")
	httpx.Render(w, r, views.ContentPage(d, p))
}

func (h *Handler) BlogIndex(w http.ResponseWriter, r *http.Request) {
	rows, err := h.Pool.Query(r.Context(), `SELECT id,slug,title,excerpt,body_html,cover_url,seo_title,seo_desc,author,is_published,published_at,updated_at FROM blog_posts WHERE is_published=TRUE ORDER BY published_at DESC NULLS LAST LIMIT 50`)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()
	posts := []*models.BlogPost{}
	for rows.Next() {
		p := &models.BlogPost{}
		if err := rows.Scan(&p.ID, &p.Slug, &p.Title, &p.Excerpt, &p.BodyHTML, &p.CoverURL, &p.SeoTitle, &p.SeoDesc, &p.Author, &p.IsPublished, &p.PublishedAt, &p.UpdatedAt); err == nil {
			posts = append(posts, p)
		}
	}
	d := h.PageData(r)
	d.URL = "/blog"
	d.Title = "Blog"
	httpx.Render(w, r, views.BlogIndex(d, posts))
}

func (h *Handler) BlogPost(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	p := &models.BlogPost{}
	err := h.Pool.QueryRow(r.Context(), `SELECT id,slug,title,excerpt,body_html,cover_url,seo_title,seo_desc,author,is_published,published_at,updated_at FROM blog_posts WHERE slug=$1 AND is_published=TRUE`, slug).
		Scan(&p.ID, &p.Slug, &p.Title, &p.Excerpt, &p.BodyHTML, &p.CoverURL, &p.SeoTitle, &p.SeoDesc, &p.Author, &p.IsPublished, &p.PublishedAt, &p.UpdatedAt)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	d := h.PageData(r)
	d.URL = "/blog/" + slug
	d.Title = derefStr(p.SeoTitle, p.Title)
	if p.Excerpt != nil {
		d.Description = *p.Excerpt
	}
	if p.CoverURL != nil {
		d.OGImage = absURL(h.BaseURL, *p.CoverURL)
	}
	httpx.Render(w, r, views.BlogPost(d, p))
}

func absURL(base, u string) string {
	if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		return u
	}
	if !strings.HasPrefix(u, "/") {
		u = "/" + u
	}
	return strings.TrimRight(base, "/") + u
}

func derefStr(s *string, def string) string {
	if s == nil || *s == "" {
		return def
	}
	return *s
}

// SEO files

func (h *Handler) Robots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte("User-agent: *\nAllow: /\nDisallow: /admin\nDisallow: /reseller/admin\nDisallow: /api/\nDisallow: /cart\nDisallow: /checkout\nDisallow: /account\nSitemap: " + h.BaseURL + "/sitemap.xml\n"))
	if extra := h.Settings.SEO().RobotsExtra; extra != "" {
		w.Write([]byte(extra + "\n"))
	}
}

func (h *Handler) Sitemap(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>` + "\n"))
	w.Write([]byte(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n"))
	urls := []string{"/", "/c/semua", "/blog"}
	// pages
	rows, _ := h.Pool.Query(r.Context(), `SELECT slug FROM pages WHERE is_published=TRUE`)
	for rows.Next() {
		var s string
		rows.Scan(&s)
		urls = append(urls, "/p/"+s)
	}
	rows.Close()
	rows, _ = h.Pool.Query(r.Context(), `SELECT slug FROM products WHERE status='active'`)
	for rows.Next() {
		var s string
		rows.Scan(&s)
		urls = append(urls, "/p/"+s)
	}
	rows.Close()
	rows, _ = h.Pool.Query(r.Context(), `SELECT slug FROM categories WHERE is_active=TRUE`)
	for rows.Next() {
		var s string
		rows.Scan(&s)
		urls = append(urls, "/c/"+s)
	}
	rows.Close()
	rows, _ = h.Pool.Query(r.Context(), `SELECT slug FROM blog_posts WHERE is_published=TRUE`)
	for rows.Next() {
		var s string
		rows.Scan(&s)
		urls = append(urls, "/blog/"+s)
	}
	rows.Close()
	for _, u := range urls {
		w.Write([]byte("<url><loc>" + h.BaseURL + u + "</loc></url>\n"))
	}
	w.Write([]byte("</urlset>\n"))
}

// API attribution
func (h *Handler) APIAttribution(w http.ResponseWriter, r *http.Request) {
	var v struct {
		UTMSource   string `json:"utm_source"`
		UTMMedium   string `json:"utm_medium"`
		UTMCampaign string `json:"utm_campaign"`
		UTMTerm     string `json:"utm_term"`
		UTMContent  string `json:"utm_content"`
		Landing     string `json:"landing"`
		Referer     string `json:"referer"`
	}
	json.NewDecoder(r.Body).Decode(&v)
	first := false
	var n int
	_ = h.Pool.QueryRow(r.Context(), `SELECT count(*) FROM utm_attributions WHERE session_token=$1`, middleware.SessionToken(r)).Scan(&n)
	if n == 0 {
		first = true
	}
	_, _ = h.Pool.Exec(r.Context(), `INSERT INTO utm_attributions(session_token,user_id,utm_source,utm_medium,utm_campaign,utm_term,utm_content,landing_url,referer,is_first_touch)
		VALUES($1,$2,NULLIF($3,''),NULLIF($4,''),NULLIF($5,''),NULLIF($6,''),NULLIF($7,''),NULLIF($8,''),NULLIF($9,''),$10)`,
		middleware.SessionToken(r), middleware.UserID(r), v.UTMSource, v.UTMMedium, v.UTMCampaign, v.UTMTerm, v.UTMContent, v.Landing, v.Referer, first)
	w.WriteHeader(http.StatusNoContent)
}

// Helpers used for redirects table
func (h *Handler) RedirectMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		var to string
		var code int
		err := h.Pool.QueryRow(context.Background(), `SELECT to_path, code FROM redirects WHERE from_path=$1`, path).Scan(&to, &code)
		if err == nil && to != "" {
			http.Redirect(w, r, to, code)
			return
		}
		next.ServeHTTP(w, r)
	})
}
