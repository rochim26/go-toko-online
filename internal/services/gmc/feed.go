package gmc

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tokoonline/app/internal/services/settings"
)

type Service struct {
	pool     *pgxpool.Pool
	settings *settings.Store
}

func New(pool *pgxpool.Pool, s *settings.Store) *Service { return &Service{pool: pool, settings: s} }

type rss struct {
	XMLName xml.Name `xml:"rss"`
	Version string   `xml:"version,attr"`
	NSG     string   `xml:"xmlns:g,attr"`
	Channel channel  `xml:"channel"`
}

type channel struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	Items       []item `xml:"item"`
}

type item struct {
	GID          string `xml:"g:id"`
	Title        string `xml:"g:title"`
	Description  string `xml:"g:description"`
	Link         string `xml:"g:link"`
	ImageLink    string `xml:"g:image_link"`
	Availability string `xml:"g:availability"`
	Price        string `xml:"g:price"`
	Brand        string `xml:"g:brand,omitempty"`
	Condition    string `xml:"g:condition"`
	GTIN         string `xml:"g:gtin,omitempty"`
	MPN          string `xml:"g:mpn,omitempty"`
	GoogleCat    string `xml:"g:google_product_category,omitempty"`
	ProductType  string `xml:"g:product_type,omitempty"`
	Identifier   string `xml:"g:identifier_exists,omitempty"`
	Shipping     string `xml:"g:shipping_weight,omitempty"`
	AgeGroup     string `xml:"g:age_group,omitempty"`
	Gender       string `xml:"g:gender,omitempty"`
}

func (s *Service) Write(ctx context.Context, w io.Writer, baseURL string) error {
	store := s.settings.Store()
	cfg := s.settings.GMC()
	if !cfg.FeedEnabled {
		_, err := io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?><rss version="2.0"><channel><title>disabled</title></channel></rss>`)
		return err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT p.id::text, p.slug, p.name, COALESCE(p.short_desc, p.description, ''), p.gmc_brand, p.gmc_gtin, p.gmc_mpn, p.gmc_condition,
		       p.gmc_age_group, p.gmc_gender, p.weight_grams,
		       (SELECT url FROM product_images WHERE product_id=p.id ORDER BY is_primary DESC, sort_order LIMIT 1) AS img,
		       COALESCE((SELECT MIN(price) FROM product_prices pp JOIN product_variants v ON v.id=pp.variant_id WHERE v.product_id=p.id AND pp.audience='b2c'),
		                COALESCE((SELECT MIN(base_price) FROM product_variants WHERE product_id=p.id),0)) AS price,
		       COALESCE((SELECT SUM(on_hand-reserved) FROM inventory_levels il JOIN product_variants v ON v.id=il.variant_id WHERE v.product_id=p.id),0) AS on_hand,
		       c.gmc_category, c.name
		FROM products p
		LEFT JOIN categories c ON c.id=p.category_id
		WHERE p.status='active' AND p.is_b2c=TRUE AND p.gmc_enabled=TRUE`)
	if err != nil {
		return err
	}
	defer rows.Close()

	feed := rss{
		Version: "2.0",
		NSG:     "http://base.google.com/ns/1.0",
		Channel: channel{
			Title:       store.Name,
			Link:        baseURL,
			Description: store.Tagline,
		},
	}
	for rows.Next() {
		var id, slug, name, desc, condition string
		var brand, gtin, mpn, ageGroup, gender, gmcCat, catName, img *string
		var weight int
		var price float64
		var onHand int
		if err := rows.Scan(&id, &slug, &name, &desc, &brand, &gtin, &mpn, &condition, &ageGroup, &gender, &weight, &img, &price, &onHand, &gmcCat, &catName); err != nil {
			return err
		}
		if cfg.AutoDisableOOS && onHand <= 0 {
			continue
		}
		availability := "in_stock"
		if onHand <= 0 {
			availability = "out_of_stock"
		}
		imageLink := ""
		if img != nil {
			imageLink = absURL(baseURL, *img)
		}
		desc = strings.TrimSpace(stripHTML(desc))
		if len(desc) > 5000 {
			desc = desc[:5000]
		}
		ident := "no"
		if (gtin != nil && *gtin != "") || (mpn != nil && *mpn != "") || (brand != nil && *brand != "") {
			ident = "yes"
		}
		it := item{
			GID:          slug,
			Title:        name,
			Description:  desc,
			Link:         baseURL + "/p/" + slug,
			ImageLink:    imageLink,
			Availability: availability,
			Price:        fmt.Sprintf("%.0f IDR", price),
			Condition:    condition,
			Identifier:   ident,
			Shipping:     fmt.Sprintf("%d g", weight),
		}
		if brand != nil {
			it.Brand = *brand
		}
		if gtin != nil {
			it.GTIN = *gtin
		}
		if mpn != nil {
			it.MPN = *mpn
		}
		if gmcCat != nil {
			it.GoogleCat = *gmcCat
		}
		if catName != nil {
			it.ProductType = *catName
		}
		if ageGroup != nil {
			it.AgeGroup = *ageGroup
		}
		if gender != nil {
			it.Gender = *gender
		}
		feed.Channel.Items = append(feed.Channel.Items, it)
	}
	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	return enc.Encode(feed)
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

func stripHTML(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		if r == '<' {
			in = true
			continue
		}
		if r == '>' {
			in = false
			continue
		}
		if !in {
			b.WriteRune(r)
		}
	}
	return b.String()
}
