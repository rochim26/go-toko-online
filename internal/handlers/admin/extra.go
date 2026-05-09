package admin

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/tokoonline/app/internal/httpx"
	"github.com/tokoonline/app/internal/middleware"
	"github.com/tokoonline/app/internal/models"
	"github.com/tokoonline/app/internal/services/security"
	"github.com/tokoonline/app/internal/views/admin"
)

// ─── Vouchers ──────────────────────────

func (h *Handler) Vouchers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.Pool.Query(r.Context(), `SELECT id,code,name,type,value,min_subtotal,max_discount,audience,usage_limit,used_count,valid_from,valid_to,is_active FROM vouchers ORDER BY created_at DESC`)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()
	var items []*models.Voucher
	for rows.Next() {
		v := &models.Voucher{}
		if err := rows.Scan(&v.ID, &v.Code, &v.Name, &v.Type, &v.Value, &v.MinSubtotal, &v.MaxDiscount, &v.Audience, &v.UsageLimit, &v.UsedCount, &v.ValidFrom, &v.ValidTo, &v.IsActive); err == nil {
			items = append(items, v)
		}
	}
	d := h.PageData(r)
	d.Title = "Voucher"
	d.NoIndex = true
	httpx.Render(w, r, admin.Vouchers(d, items))
}

func (h *Handler) VoucherUpsert(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	val, _ := strconv.ParseFloat(r.FormValue("value"), 64)
	min, _ := strconv.ParseFloat(r.FormValue("min_subtotal"), 64)
	var maxD *float64
	if s := r.FormValue("max_discount"); s != "" {
		if v, err := strconv.ParseFloat(s, 64); err == nil {
			maxD = &v
		}
	}
	var limit *int
	if s := r.FormValue("usage_limit"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			limit = &v
		}
	}
	var validFrom, validTo *time.Time
	if s := r.FormValue("valid_from"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			validFrom = &t
		}
	}
	if s := r.FormValue("valid_to"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			validTo = &t
		}
	}
	_, err := h.Pool.Exec(r.Context(), `
		INSERT INTO vouchers(code,name,type,value,min_subtotal,max_discount,audience,usage_limit,valid_from,valid_to,is_active)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,TRUE)
		ON CONFLICT(code) DO UPDATE SET name=EXCLUDED.name, type=EXCLUDED.type, value=EXCLUDED.value,
			min_subtotal=EXCLUDED.min_subtotal, max_discount=EXCLUDED.max_discount, audience=EXCLUDED.audience,
			usage_limit=EXCLUDED.usage_limit, valid_from=EXCLUDED.valid_from, valid_to=EXCLUDED.valid_to`,
		strings.ToUpper(strings.TrimSpace(r.FormValue("code"))),
		nullStrA(r.FormValue("name")),
		r.FormValue("type"),
		val, min, maxD, r.FormValue("audience"), limit, validFrom, validTo)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/admin/vouchers", http.StatusFound)
}

func (h *Handler) VoucherToggle(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_, _ = h.Pool.Exec(r.Context(), `UPDATE vouchers SET is_active=NOT is_active WHERE id=$1`, id)
	http.Redirect(w, r, "/admin/vouchers", http.StatusFound)
}

// ─── Categories ────────────────────────

func (h *Handler) Categories(w http.ResponseWriter, r *http.Request) {
	cats, _ := h.Catalog.ListCategories(r.Context())
	d := h.PageData(r)
	d.Title = "Kategori"
	d.NoIndex = true
	httpx.Render(w, r, admin.Categories(d, cats))
}

func (h *Handler) CategoryCreate(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "name required", 400)
		return
	}
	slug := strings.TrimSpace(r.FormValue("slug"))
	if slug == "" {
		slug = httpx.Slugify(name)
	}
	sortOrder, _ := strconv.Atoi(r.FormValue("sort_order"))
	var parentID *uuid.UUID
	if v := r.FormValue("parent_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			parentID = &id
		}
	}
	_, err := h.Pool.Exec(r.Context(), `INSERT INTO categories(name,slug,parent_id,description,sort_order,seo_title,seo_desc,gmc_category) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`,
		name, slug, parentID,
		nullStrA(r.FormValue("description")),
		sortOrder,
		nullStrA(r.FormValue("seo_title")),
		nullStrA(r.FormValue("seo_desc")),
		nullStrA(r.FormValue("gmc_category")))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/admin/categories", http.StatusFound)
}

func (h *Handler) CategoryToggle(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_, _ = h.Pool.Exec(r.Context(), `UPDATE categories SET is_active=NOT is_active WHERE id=$1`, id)
	http.Redirect(w, r, "/admin/categories", http.StatusFound)
}

func (h *Handler) CategoryDelete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_, _ = h.Pool.Exec(r.Context(), `DELETE FROM categories WHERE id=$1`, id)
	http.Redirect(w, r, "/admin/categories", http.StatusFound)
}

// ─── Blog ─────────────────────────────

func (h *Handler) BlogList(w http.ResponseWriter, r *http.Request) {
	rows, err := h.Pool.Query(r.Context(), `SELECT id,slug,title,excerpt,body_html,cover_url,seo_title,seo_desc,author,is_published,published_at,updated_at FROM blog_posts ORDER BY created_at DESC`)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()
	var posts []*models.BlogPost
	for rows.Next() {
		p := &models.BlogPost{}
		if err := rows.Scan(&p.ID, &p.Slug, &p.Title, &p.Excerpt, &p.BodyHTML, &p.CoverURL, &p.SeoTitle, &p.SeoDesc, &p.Author, &p.IsPublished, &p.PublishedAt, &p.UpdatedAt); err == nil {
			posts = append(posts, p)
		}
	}
	d := h.PageData(r)
	d.Title = "Blog"
	d.NoIndex = true
	httpx.Render(w, r, admin.BlogList(d, posts))
}

func (h *Handler) BlogNew(w http.ResponseWriter, r *http.Request) {
	d := h.PageData(r)
	d.Title = "Tulis Artikel"
	d.NoIndex = true
	httpx.Render(w, r, admin.BlogForm(d, admin.BlogFormData{IsNew: true, IsPublished: false}))
}

func (h *Handler) BlogCreate(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		http.Error(w, "title required", 400)
		return
	}
	slug := strings.TrimSpace(r.FormValue("slug"))
	if slug == "" {
		slug = httpx.Slugify(title)
	}
	pub := r.FormValue("is_published") != ""
	var pubAt *time.Time
	if pub {
		t := time.Now()
		pubAt = &t
	}
	var id uuid.UUID
	err := h.Pool.QueryRow(r.Context(), `INSERT INTO blog_posts(slug,title,excerpt,body_html,cover_url,seo_title,seo_desc,author,is_published,published_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`,
		slug, title,
		nullStrA(r.FormValue("excerpt")),
		r.FormValue("body_html"),
		nullStrA(r.FormValue("cover_url")),
		nullStrA(r.FormValue("seo_title")),
		nullStrA(r.FormValue("seo_desc")),
		nullStrA(r.FormValue("author")),
		pub, pubAt,
	).Scan(&id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/admin/blog/"+id.String(), http.StatusFound)
}

func (h *Handler) BlogEdit(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	p := &models.BlogPost{}
	err = h.Pool.QueryRow(r.Context(), `SELECT id,slug,title,excerpt,body_html,cover_url,seo_title,seo_desc,author,is_published,published_at,updated_at FROM blog_posts WHERE id=$1`, id).
		Scan(&p.ID, &p.Slug, &p.Title, &p.Excerpt, &p.BodyHTML, &p.CoverURL, &p.SeoTitle, &p.SeoDesc, &p.Author, &p.IsPublished, &p.PublishedAt, &p.UpdatedAt)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	d := h.PageData(r)
	d.Title = "Edit " + p.Title
	d.NoIndex = true
	httpx.Render(w, r, admin.BlogForm(d, admin.BlogFormData{
		IsNew: false, ID: p.ID.String(),
		Slug: p.Slug, Title: p.Title,
		Excerpt: derefS(p.Excerpt), BodyHTML: p.BodyHTML,
		CoverURL: derefS(p.CoverURL),
		SeoTitle: derefS(p.SeoTitle), SeoDesc: derefS(p.SeoDesc),
		Author: derefS(p.Author), IsPublished: p.IsPublished,
	}))
}

func (h *Handler) BlogUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	r.ParseForm()
	pub := r.FormValue("is_published") != ""
	_, err = h.Pool.Exec(r.Context(), `
		UPDATE blog_posts SET slug=$2, title=$3, excerpt=$4, body_html=$5, cover_url=$6, seo_title=$7, seo_desc=$8, author=$9, is_published=$10,
			published_at = CASE WHEN $10 AND published_at IS NULL THEN now() ELSE published_at END,
			updated_at = now()
		WHERE id=$1`,
		id,
		strings.TrimSpace(r.FormValue("slug")),
		strings.TrimSpace(r.FormValue("title")),
		nullStrA(r.FormValue("excerpt")),
		r.FormValue("body_html"),
		nullStrA(r.FormValue("cover_url")),
		nullStrA(r.FormValue("seo_title")),
		nullStrA(r.FormValue("seo_desc")),
		nullStrA(r.FormValue("author")),
		pub,
	)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/admin/blog/"+id.String(), http.StatusFound)
}

// ─── Change Password ──────────────────

func (h *Handler) ShowChangePassword(w http.ResponseWriter, r *http.Request) {
	d := h.PageData(r)
	d.Title = "Ganti Password"
	d.NoIndex = true
	httpx.Render(w, r, admin.ChangePassword(d, "", ""))
}

func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserID(r)
	if uid == nil {
		http.Redirect(w, r, "/admin/login", http.StatusFound)
		return
	}
	r.ParseForm()
	cur := r.FormValue("current")
	nw := r.FormValue("new")
	cf := r.FormValue("confirm")
	d := h.PageData(r)
	d.Title = "Ganti Password"
	d.NoIndex = true
	if nw != cf {
		httpx.Render(w, r, admin.ChangePassword(d, "Password baru tidak sama", ""))
		return
	}
	if len(nw) < 8 {
		httpx.Render(w, r, admin.ChangePassword(d, "Minimal 8 karakter", ""))
		return
	}
	var hash string
	if err := h.Pool.QueryRow(r.Context(), `SELECT password_hash FROM users WHERE id=$1`, *uid).Scan(&hash); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	ok, _ := security.VerifyPassword(cur, hash)
	if !ok {
		httpx.Render(w, r, admin.ChangePassword(d, "Password lama salah", ""))
		return
	}
	newHash, err := security.HashPassword(nw)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	_, _ = h.Pool.Exec(r.Context(), `UPDATE users SET password_hash=$2, updated_at=now() WHERE id=$1`, *uid, newHash)
	httpx.Render(w, r, admin.ChangePassword(d, "", "Password berhasil diubah."))
}

func nullStrA(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func derefS(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
