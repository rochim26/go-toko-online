package viewdata

import "github.com/tokoonline/app/internal/services/settings"

type PageData struct {
	Title       string
	Description string
	URL         string
	Canonical   string
	OGImage     string
	NoIndex     bool
	BaseURL     string
	CSRFToken   string
	IsLoggedIn  bool
	UserRole    string
	UserEmail   string
	UserName    string
	CartCount   int
	JSONLD      []string
	Flash       string
	FlashKind   string
	BodyClass   string
	Store       settings.StoreInfo
	SEO         settings.SEOGlobal
	Marketing   settings.Marketing
}
