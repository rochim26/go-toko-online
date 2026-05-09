package viewdata

import (
	"github.com/tokoonline/app/internal/services/integrations"
	"github.com/tokoonline/app/internal/services/settings"
)

type PageData struct {
	Title       string
	Description string
	URL         string
	Canonical   string
	OGImage     string
	NoIndex     bool
	BaseURL     string
	AssetVer    string // build version for cache-busting static assets
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
	// Active key used by mobile bottom nav (one of: home, catalog, cart, account, search).
	// If empty, the bottom nav infers from URL.
	ActiveNav   string
	// Hide bottom nav (e.g. checkout flow, full-screen forms).
	HideBottomNav bool
	Store     settings.StoreInfo
	SEO       settings.SEOGlobal
	Marketing settings.Marketing
	// Admin-only data
	OnboardingDone bool
	Integrations   []integrations.Status
}
