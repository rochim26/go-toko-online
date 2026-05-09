package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tokoonline/app/internal/models"
)

type Service struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

type ListOpts struct {
	CategorySlug string
	Search       string
	Audience     string // "b2c" or tier id
	Limit        int
	Offset       int
	OnlyB2B      bool
	OnlyB2C      bool
}

const productSelect = `
SELECT p.id, p.slug, p.name, p.brand_id, p.category_id, p.short_desc, p.description,
       p.status, p.is_b2c, p.is_b2b, p.weight_grams, p.length_cm, p.width_cm, p.height_cm,
       p.seo_title, p.seo_desc, p.focus_keyword, p.og_image_url,
       p.gmc_enabled, p.gmc_brand, p.gmc_gtin, p.gmc_mpn, p.gmc_condition, p.gmc_age_group, p.gmc_gender,
       p.faqs, p.published_at, p.created_at, p.updated_at,
       b.name AS brand_name, b.slug AS brand_slug,
       c.name AS category_name, c.slug AS category_slug,
       (SELECT url FROM product_images WHERE product_id=p.id ORDER BY is_primary DESC, sort_order ASC LIMIT 1) AS primary_image,
       COALESCE((SELECT MIN(price) FROM product_prices pp JOIN product_variants v ON v.id=pp.variant_id WHERE v.product_id=p.id AND pp.audience=$1),
                COALESCE((SELECT MIN(base_price) FROM product_variants WHERE product_id=p.id),0)) AS min_price,
       COALESCE((SELECT MAX(price) FROM product_prices pp JOIN product_variants v ON v.id=pp.variant_id WHERE v.product_id=p.id AND pp.audience=$1),
                COALESCE((SELECT MAX(base_price) FROM product_variants WHERE product_id=p.id),0)) AS max_price,
       COALESCE((SELECT SUM(il.on_hand-il.reserved) FROM inventory_levels il JOIN product_variants v ON v.id=il.variant_id WHERE v.product_id=p.id),0) AS on_hand
FROM products p
LEFT JOIN brands b ON b.id = p.brand_id
LEFT JOIN categories c ON c.id = p.category_id
`

func scanProduct(row pgx.Row) (*models.Product, error) {
	var p models.Product
	var faqs []byte
	err := row.Scan(&p.ID, &p.Slug, &p.Name, &p.BrandID, &p.CategoryID, &p.ShortDesc, &p.Description,
		&p.Status, &p.IsB2C, &p.IsB2B, &p.WeightGrams, &p.LengthCm, &p.WidthCm, &p.HeightCm,
		&p.SeoTitle, &p.SeoDesc, &p.FocusKeyword, &p.OGImageURL,
		&p.GMCEnabled, &p.GMCBrand, &p.GMCGtin, &p.GMCMpn, &p.GMCCondition, &p.GMCAgeGroup, &p.GMCGender,
		&faqs, &p.PublishedAt, &p.CreatedAt, &p.UpdatedAt,
		&p.BrandName, &p.BrandSlug, &p.CategoryName, &p.CategorySlug,
		&p.PrimaryImage, &p.MinPrice, &p.MaxPrice, &p.OnHand)
	if err != nil {
		return nil, err
	}
	if len(faqs) > 0 {
		_ = json.Unmarshal(faqs, &p.FAQs)
	}
	return &p, nil
}

func (s *Service) List(ctx context.Context, o ListOpts) ([]*models.Product, int, error) {
	if o.Limit == 0 || o.Limit > 100 {
		o.Limit = 24
	}
	if o.Audience == "" {
		o.Audience = "b2c"
	}
	listArgs := []any{o.Audience}
	countArgs := []any{}
	listWhere := []string{"p.status='active'"}
	countWhere := []string{"p.status='active'"}
	if o.OnlyB2C {
		listWhere = append(listWhere, "p.is_b2c=TRUE")
		countWhere = append(countWhere, "p.is_b2c=TRUE")
	}
	if o.OnlyB2B {
		listWhere = append(listWhere, "p.is_b2b=TRUE")
		countWhere = append(countWhere, "p.is_b2b=TRUE")
	}
	if o.CategorySlug != "" {
		listArgs = append(listArgs, o.CategorySlug)
		countArgs = append(countArgs, o.CategorySlug)
		listWhere = append(listWhere, fmt.Sprintf("c.slug=$%d", len(listArgs)))
		countWhere = append(countWhere, fmt.Sprintf("c.slug=$%d", len(countArgs)))
	}
	if q := strings.TrimSpace(o.Search); q != "" {
		listArgs = append(listArgs, q)
		countArgs = append(countArgs, q)
		listWhere = append(listWhere, fmt.Sprintf("(p.search_vec @@ plainto_tsquery('simple', unaccent($%d)) OR p.name ILIKE '%%' || $%d || '%%')", len(listArgs), len(listArgs)))
		countWhere = append(countWhere, fmt.Sprintf("(p.search_vec @@ plainto_tsquery('simple', unaccent($%d)) OR p.name ILIKE '%%' || $%d || '%%')", len(countArgs), len(countArgs)))
	}
	listArgs = append(listArgs, o.Limit, o.Offset)
	sql := productSelect + " WHERE " + strings.Join(listWhere, " AND ") +
		" ORDER BY p.published_at DESC NULLS LAST" +
		fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(listArgs)-1, len(listArgs))

	rows, err := s.pool.Query(ctx, sql, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []*models.Product{}
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}

	csql := "SELECT count(*) FROM products p LEFT JOIN brands b ON b.id=p.brand_id LEFT JOIN categories c ON c.id=p.category_id WHERE " + strings.Join(countWhere, " AND ")
	var total int
	if err := s.pool.QueryRow(ctx, csql, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (s *Service) GetBySlug(ctx context.Context, slug, audience string) (*models.Product, error) {
	if audience == "" {
		audience = "b2c"
	}
	row := s.pool.QueryRow(ctx, productSelect+" WHERE p.slug=$2 AND p.status='active'", audience, slug)
	return scanProduct(row)
}

func (s *Service) GetVariants(ctx context.Context, productID uuid.UUID) ([]*models.Variant, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, product_id, sku, name, attributes, base_price, compare_at_price, cost_price, weight_grams, barcode, is_default, is_active
		FROM product_variants WHERE product_id=$1 AND is_active=TRUE ORDER BY is_default DESC, created_at`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*models.Variant{}
	for rows.Next() {
		var v models.Variant
		var attrs []byte
		if err := rows.Scan(&v.ID, &v.ProductID, &v.SKU, &v.Name, &attrs, &v.BasePrice, &v.CompareAtPrice, &v.CostPrice, &v.WeightGrams, &v.Barcode, &v.IsDefault, &v.IsActive); err != nil {
			return nil, err
		}
		v.Attributes = map[string]any{}
		_ = json.Unmarshal(attrs, &v.Attributes)
		out = append(out, &v)
	}
	return out, rows.Err()
}

func (s *Service) GetImages(ctx context.Context, productID uuid.UUID) ([]*models.ProductImage, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, product_id, variant_id, url, alt_text, sort_order, is_primary FROM product_images WHERE product_id=$1 ORDER BY is_primary DESC, sort_order`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*models.ProductImage{}
	for rows.Next() {
		var im models.ProductImage
		if err := rows.Scan(&im.ID, &im.ProductID, &im.VariantID, &im.URL, &im.AltText, &im.SortOrder, &im.IsPrimary); err != nil {
			return nil, err
		}
		out = append(out, &im)
	}
	return out, rows.Err()
}

func (s *Service) ListCategories(ctx context.Context) ([]*models.Category, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,parent_id,name,slug,description,image_url,sort_order,is_active,seo_title,seo_desc,gmc_category FROM categories WHERE is_active=TRUE ORDER BY sort_order, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*models.Category{}
	for rows.Next() {
		var c models.Category
		if err := rows.Scan(&c.ID, &c.ParentID, &c.Name, &c.Slug, &c.Description, &c.ImageURL, &c.SortOrder, &c.IsActive, &c.SeoTitle, &c.SeoDesc, &c.GMCCategory); err != nil {
			return nil, err
		}
		out = append(out, &c)
	}
	return out, rows.Err()
}

func (s *Service) GetCategoryBySlug(ctx context.Context, slug string) (*models.Category, error) {
	var c models.Category
	err := s.pool.QueryRow(ctx, `SELECT id,parent_id,name,slug,description,image_url,sort_order,is_active,seo_title,seo_desc,gmc_category FROM categories WHERE slug=$1`, slug).
		Scan(&c.ID, &c.ParentID, &c.Name, &c.Slug, &c.Description, &c.ImageURL, &c.SortOrder, &c.IsActive, &c.SeoTitle, &c.SeoDesc, &c.GMCCategory)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Service) GetVariantWithProduct(ctx context.Context, variantID uuid.UUID) (variant *models.Variant, product *models.Product, image *string, err error) {
	v := &models.Variant{}
	var attrs []byte
	err = s.pool.QueryRow(ctx, `SELECT id, product_id, sku, name, attributes, base_price, compare_at_price, cost_price, weight_grams, barcode, is_default, is_active FROM product_variants WHERE id=$1`, variantID).
		Scan(&v.ID, &v.ProductID, &v.SKU, &v.Name, &attrs, &v.BasePrice, &v.CompareAtPrice, &v.CostPrice, &v.WeightGrams, &v.Barcode, &v.IsDefault, &v.IsActive)
	if err != nil {
		return nil, nil, nil, err
	}
	v.Attributes = map[string]any{}
	_ = json.Unmarshal(attrs, &v.Attributes)
	p, err := s.getByID(ctx, v.ProductID, "b2c")
	if err != nil {
		return v, nil, nil, err
	}
	var img *string
	_ = s.pool.QueryRow(ctx, `SELECT url FROM product_images WHERE product_id=$1 ORDER BY is_primary DESC, sort_order LIMIT 1`, v.ProductID).Scan(&img)
	return v, p, img, nil
}

func (s *Service) getByID(ctx context.Context, id uuid.UUID, audience string) (*models.Product, error) {
	if audience == "" {
		audience = "b2c"
	}
	row := s.pool.QueryRow(ctx, productSelect+" WHERE p.id=$2", audience, id)
	return scanProduct(row)
}

// ─────────────────────────────────────────────
// Admin operations

type ProductInput struct {
	Slug         string
	Name         string
	BrandID      *uuid.UUID
	CategoryID   *uuid.UUID
	ShortDesc    string
	Description  string
	Status       string
	IsB2C        bool
	IsB2B        bool
	WeightGrams  int
	SeoTitle     string
	SeoDesc      string
	FocusKeyword string
	OGImageURL   string
	GMCEnabled   bool
	GMCBrand     string
	GMCGtin      string
	GMCMpn       string
	GMCCondition string
	FAQs         []models.FAQ
}

func (s *Service) CreateProduct(ctx context.Context, in ProductInput) (uuid.UUID, error) {
	faqs, _ := json.Marshal(in.FAQs)
	if in.GMCCondition == "" {
		in.GMCCondition = "new"
	}
	if in.WeightGrams == 0 {
		in.WeightGrams = 500
	}
	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `
		INSERT INTO products(slug,name,brand_id,category_id,short_desc,description,status,is_b2c,is_b2b,weight_grams,
			seo_title,seo_desc,focus_keyword,og_image_url,gmc_enabled,gmc_brand,gmc_gtin,gmc_mpn,gmc_condition,faqs,
			published_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,
			CASE WHEN $7='active' THEN now() ELSE NULL END)
		RETURNING id`,
		in.Slug, in.Name, in.BrandID, in.CategoryID, in.ShortDesc, in.Description, in.Status,
		in.IsB2C, in.IsB2B, in.WeightGrams,
		nullStr(in.SeoTitle), nullStr(in.SeoDesc), nullStr(in.FocusKeyword), nullStr(in.OGImageURL),
		in.GMCEnabled, nullStr(in.GMCBrand), nullStr(in.GMCGtin), nullStr(in.GMCMpn), in.GMCCondition, faqs,
	).Scan(&id)
	return id, err
}

func (s *Service) UpdateProduct(ctx context.Context, id uuid.UUID, in ProductInput) error {
	faqs, _ := json.Marshal(in.FAQs)
	_, err := s.pool.Exec(ctx, `
		UPDATE products SET slug=$2,name=$3,brand_id=$4,category_id=$5,short_desc=$6,description=$7,status=$8,
			is_b2c=$9,is_b2b=$10,weight_grams=$11,
			seo_title=$12,seo_desc=$13,focus_keyword=$14,og_image_url=$15,
			gmc_enabled=$16,gmc_brand=$17,gmc_gtin=$18,gmc_mpn=$19,gmc_condition=$20,faqs=$21,
			published_at = CASE WHEN $8='active' AND published_at IS NULL THEN now() ELSE published_at END
		WHERE id=$1`,
		id, in.Slug, in.Name, in.BrandID, in.CategoryID, in.ShortDesc, in.Description, in.Status,
		in.IsB2C, in.IsB2B, in.WeightGrams,
		nullStr(in.SeoTitle), nullStr(in.SeoDesc), nullStr(in.FocusKeyword), nullStr(in.OGImageURL),
		in.GMCEnabled, nullStr(in.GMCBrand), nullStr(in.GMCGtin), nullStr(in.GMCMpn), in.GMCCondition, faqs,
	)
	return err
}

func (s *Service) AddVariant(ctx context.Context, productID uuid.UUID, sku, name string, basePrice float64, weight int, isDefault bool) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `INSERT INTO product_variants(product_id,sku,name,base_price,weight_grams,is_default) VALUES($1,$2,$3,$4,$5,$6) RETURNING id`,
		productID, sku, nullStr(name), basePrice, nullInt(weight), isDefault).Scan(&id)
	if err != nil {
		return id, err
	}
	_, _ = s.pool.Exec(ctx, `INSERT INTO product_prices(variant_id,audience,price) VALUES($1,'b2c',$2) ON CONFLICT DO NOTHING`, id, basePrice)
	_, _ = s.pool.Exec(ctx, `INSERT INTO inventory_levels(variant_id,on_hand) VALUES($1,0) ON CONFLICT DO NOTHING`, id)
	return id, nil
}

func (s *Service) UpdateVariantPrice(ctx context.Context, variantID uuid.UUID, audience string, price float64) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO product_prices(variant_id,audience,price) VALUES($1,$2,$3)
		ON CONFLICT DO NOTHING`, variantID, audience, price)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `UPDATE product_prices SET price=$3 WHERE variant_id=$1 AND audience=$2 AND valid_from IS NULL AND valid_to IS NULL`, variantID, audience, price)
	return err
}

func (s *Service) SetInventory(ctx context.Context, variantID uuid.UUID, onHand int) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO inventory_levels(variant_id, on_hand) VALUES($1,$2)
		ON CONFLICT(variant_id) DO UPDATE SET on_hand=EXCLUDED.on_hand, updated_at=now()`, variantID, onHand)
	return err
}

func (s *Service) AddImage(ctx context.Context, productID uuid.UUID, url, alt string, primary bool) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO product_images(product_id,url,alt_text,is_primary) VALUES($1,$2,$3,$4)`,
		productID, url, nullStr(alt), primary)
	return err
}

func (s *Service) DeleteImage(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM product_images WHERE id=$1`, id)
	return err
}

func (s *Service) DeleteVariant(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM product_variants WHERE id=$1`, id)
	return err
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func nullInt(i int) any {
	if i == 0 {
		return nil
	}
	return i
}
