package seo

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tokoonline/app/internal/models"
)

type Page struct {
	Title        string
	Description  string
	URL          string
	OGImage      string
	Canonical    string
	NoIndex      bool
	StructuredJSON []string // raw JSON-LD strings
}

func TitleWith(pattern, title, fallback string) string {
	if pattern == "" {
		pattern = "%s"
	}
	if title == "" {
		title = fallback
	}
	if !strings.Contains(pattern, "%s") {
		return title
	}
	return strings.ReplaceAll(pattern, "%s", title)
}

func ProductLD(p *models.Product, baseURL string, image string, currency string, price float64, availability string) string {
	if availability == "" {
		availability = "https://schema.org/InStock"
	}
	if currency == "" {
		currency = "IDR"
	}
	url := baseURL + "/p/" + p.Slug
	d := map[string]any{
		"@context": "https://schema.org/",
		"@type":    "Product",
		"name":     p.Name,
		"sku":      p.Slug,
		"url":      url,
	}
	if p.ShortDesc != nil && *p.ShortDesc != "" {
		d["description"] = *p.ShortDesc
	} else if p.SeoDesc != nil {
		d["description"] = *p.SeoDesc
	}
	if image != "" {
		d["image"] = []string{image}
	}
	if p.GMCBrand != nil && *p.GMCBrand != "" {
		d["brand"] = map[string]string{"@type": "Brand", "name": *p.GMCBrand}
	} else if p.BrandName != nil {
		d["brand"] = map[string]string{"@type": "Brand", "name": *p.BrandName}
	}
	if p.GMCGtin != nil && *p.GMCGtin != "" {
		d["gtin"] = *p.GMCGtin
	}
	if p.GMCMpn != nil && *p.GMCMpn != "" {
		d["mpn"] = *p.GMCMpn
	}
	d["offers"] = map[string]any{
		"@type":         "Offer",
		"priceCurrency": currency,
		"price":         fmt.Sprintf("%.0f", price),
		"availability":  availability,
		"url":           url,
	}
	b, _ := json.Marshal(d)
	return string(b)
}

func BreadcrumbLD(baseURL string, crumbs []Crumb) string {
	items := make([]map[string]any, 0, len(crumbs))
	for i, c := range crumbs {
		items = append(items, map[string]any{
			"@type":    "ListItem",
			"position": i + 1,
			"name":     c.Name,
			"item":     baseURL + c.URL,
		})
	}
	d := map[string]any{
		"@context":        "https://schema.org",
		"@type":           "BreadcrumbList",
		"itemListElement": items,
	}
	b, _ := json.Marshal(d)
	return string(b)
}

type Crumb struct {
	Name string
	URL  string
}

func FAQLD(faqs []models.FAQ) string {
	if len(faqs) == 0 {
		return ""
	}
	items := make([]map[string]any, 0, len(faqs))
	for _, f := range faqs {
		items = append(items, map[string]any{
			"@type": "Question",
			"name":  f.Q,
			"acceptedAnswer": map[string]any{
				"@type": "Answer",
				"text":  f.A,
			},
		})
	}
	d := map[string]any{
		"@context":   "https://schema.org",
		"@type":      "FAQPage",
		"mainEntity": items,
	}
	b, _ := json.Marshal(d)
	return string(b)
}

func OrgLD(name, url, logo string) string {
	d := map[string]any{
		"@context": "https://schema.org",
		"@type":    "Organization",
		"name":     name,
		"url":      url,
	}
	if logo != "" {
		d["logo"] = logo
	}
	b, _ := json.Marshal(d)
	return string(b)
}

func WebSiteLD(name, url string) string {
	d := map[string]any{
		"@context": "https://schema.org",
		"@type":    "WebSite",
		"name":     name,
		"url":      url,
		"potentialAction": map[string]any{
			"@type":       "SearchAction",
			"target":      url + "/search?q={search_term_string}",
			"query-input": "required name=search_term_string",
		},
	}
	b, _ := json.Marshal(d)
	return string(b)
}
