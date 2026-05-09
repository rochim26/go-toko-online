// Package integrations exposes a status snapshot of every external service
// configured via the admin settings store. It is consumed by the admin sidebar
// (badge count of unconfigured items) and the settings page (per-tab badge).
package integrations

import "github.com/tokoonline/app/internal/services/settings"

type Status struct {
	Key          string // settings tab key (xendit, biteship, marketing, etc.)
	Name         string // human label
	Configured   bool
	Required     bool   // true for "must be set before going live"
	Missing      []string // names of fields still empty
	HelpURL      string   // public docs URL
	DashboardURL string   // provider's own dashboard
}

func All(s *settings.Store) []Status {
	out := []Status{
		store(s),
		xendit(s),
		biteship(s),
		metaPixel(s),
		metaCAPI(s),
		ga4Client(s),
		ga4MP(s),
		gtm(s),
		tiktok(s),
		googleAds(s),
		gmc(s),
		mailer(s),
		seoVerification(s),
	}
	return out
}

// AnyMissing returns the count of integrations that are required but not yet configured.
func MissingCount(all []Status) int {
	n := 0
	for _, st := range all {
		if !st.Configured {
			n++
		}
	}
	return n
}

func RequiredMissingCount(all []Status) int {
	n := 0
	for _, st := range all {
		if st.Required && !st.Configured {
			n++
		}
	}
	return n
}

func ByKey(all []Status, key string) *Status {
	for i := range all {
		if all[i].Key == key {
			return &all[i]
		}
	}
	return nil
}

// ────────────────────────────────────────────────────────
// Per-integration checks

func store(s *settings.Store) Status {
	v := s.Store()
	st := Status{Key: "store", Name: "Identitas Toko", Required: true}
	if v.Name == "" {
		st.Missing = append(st.Missing, "Nama toko")
	}
	if v.WANumber == "" || v.WANumber == "+62" {
		st.Missing = append(st.Missing, "Nomor WhatsApp")
	}
	if v.OriginAreaID == "" {
		st.Missing = append(st.Missing, "Origin Area ID (Biteship)")
	}
	if v.OriginPostalCode == "" {
		st.Missing = append(st.Missing, "Origin Postal Code")
	}
	st.Configured = len(st.Missing) == 0
	return st
}

func xendit(s *settings.Store) Status {
	v := s.Xendit()
	st := Status{
		Key: "xendit", Name: "Xendit (Pembayaran)", Required: true,
		HelpURL:      "https://docs.xendit.co/",
		DashboardURL: "https://dashboard.xendit.co/settings/developers#api-keys",
	}
	if v.SecretKey == "" {
		st.Missing = append(st.Missing, "Secret Key")
	}
	if v.WebhookToken == "" {
		st.Missing = append(st.Missing, "Webhook Verification Token")
	}
	st.Configured = len(st.Missing) == 0
	return st
}

func biteship(s *settings.Store) Status {
	v := s.Biteship()
	st := Status{
		Key: "biteship", Name: "Biteship (Ongkir)", Required: true,
		HelpURL:      "https://biteship.com/id/docs/api",
		DashboardURL: "https://biteship.com/id/dashboard/api/keys",
	}
	if v.APIKey == "" {
		st.Missing = append(st.Missing, "API Key")
	}
	if len(v.Couriers) == 0 {
		st.Missing = append(st.Missing, "Daftar kurir")
	}
	st.Configured = len(st.Missing) == 0
	return st
}

func metaPixel(s *settings.Store) Status {
	v := s.Marketing()
	st := Status{
		Key: "meta_pixel", Name: "Meta Pixel (browser)",
		HelpURL:      "https://www.facebook.com/business/help/952192354843755",
		DashboardURL: "https://business.facebook.com/events_manager2",
	}
	if v.MetaPixelID == "" {
		st.Missing = append(st.Missing, "Pixel ID")
	}
	st.Configured = len(st.Missing) == 0
	return st
}

func metaCAPI(s *settings.Store) Status {
	v := s.Marketing()
	st := Status{
		Key: "meta_capi", Name: "Meta Conversions API (server)",
		HelpURL:      "https://developers.facebook.com/docs/marketing-api/conversions-api/",
		DashboardURL: "https://business.facebook.com/events_manager2",
	}
	if v.MetaPixelID == "" {
		st.Missing = append(st.Missing, "Pixel ID")
	}
	if v.MetaCAPIToken == "" {
		st.Missing = append(st.Missing, "Access Token (CAPI)")
	}
	st.Configured = len(st.Missing) == 0
	return st
}

func ga4Client(s *settings.Store) Status {
	v := s.Marketing()
	st := Status{
		Key: "ga4_client", Name: "Google Analytics 4 (browser)",
		HelpURL:      "https://support.google.com/analytics/answer/9304153",
		DashboardURL: "https://analytics.google.com/",
	}
	if v.GA4ID == "" {
		st.Missing = append(st.Missing, "Measurement ID (G-XXXX)")
	}
	st.Configured = len(st.Missing) == 0
	return st
}

func ga4MP(s *settings.Store) Status {
	v := s.Marketing()
	st := Status{
		Key: "ga4_mp", Name: "Google Analytics 4 Measurement Protocol (server)",
		HelpURL:      "https://developers.google.com/analytics/devguides/collection/protocol/ga4",
		DashboardURL: "https://analytics.google.com/analytics/web/#/admin",
	}
	if v.GA4ID == "" {
		st.Missing = append(st.Missing, "Measurement ID")
	}
	if v.GA4APISecret == "" {
		st.Missing = append(st.Missing, "API Secret")
	}
	st.Configured = len(st.Missing) == 0
	return st
}

func gtm(s *settings.Store) Status {
	v := s.Marketing()
	st := Status{
		Key: "gtm", Name: "Google Tag Manager",
		HelpURL:      "https://support.google.com/tagmanager",
		DashboardURL: "https://tagmanager.google.com/",
	}
	if v.GTMID == "" {
		st.Missing = append(st.Missing, "Container ID (GTM-XXXX)")
	}
	st.Configured = len(st.Missing) == 0
	return st
}

func tiktok(s *settings.Store) Status {
	v := s.Marketing()
	st := Status{
		Key: "tiktok", Name: "TikTok Pixel",
		HelpURL:      "https://ads.tiktok.com/help/article/get-started-pixel",
		DashboardURL: "https://ads.tiktok.com/i18n/events_manager",
	}
	if v.TikTokPixelID == "" {
		st.Missing = append(st.Missing, "TikTok Pixel ID")
	}
	st.Configured = len(st.Missing) == 0
	return st
}

func googleAds(s *settings.Store) Status {
	v := s.Marketing()
	st := Status{
		Key: "google_ads", Name: "Google Ads Conversion",
		HelpURL:      "https://support.google.com/google-ads/answer/6095821",
		DashboardURL: "https://ads.google.com/",
	}
	if v.GoogleAdsID == "" {
		st.Missing = append(st.Missing, "Google Ads ID (AW-XXXX)")
	}
	st.Configured = len(st.Missing) == 0
	return st
}

func gmc(s *settings.Store) Status {
	v := s.GMC()
	st := Status{
		Key: "gmc", Name: "Google Merchant Center",
		HelpURL:      "https://support.google.com/merchants/answer/188478",
		DashboardURL: "https://merchants.google.com/",
	}
	if v.MerchantID == "" {
		st.Missing = append(st.Missing, "Merchant ID")
	}
	st.Configured = len(st.Missing) == 0
	return st
}

func mailer(s *settings.Store) Status {
	v := s.Mailer()
	// Mailer has 2 modes: external SMTP or local postfix loopback.
	// We treat it as configured if SMTP host filled OR FromEmail configured (loopback ok).
	st := Status{
		Key: "mailer", Name: "Email / SMTP",
		HelpURL: "",
	}
	if v.FromEmail == "" {
		st.Missing = append(st.Missing, "From Email")
	}
	if v.FromName == "" {
		st.Missing = append(st.Missing, "From Name")
	}
	st.Configured = len(st.Missing) == 0
	return st
}

func seoVerification(s *settings.Store) Status {
	v := s.SEO()
	st := Status{
		Key: "seo", Name: "SEO Verification",
		HelpURL:      "https://support.google.com/webmasters/answer/9008080",
		DashboardURL: "https://search.google.com/search-console",
	}
	if v.GSCVerification == "" {
		st.Missing = append(st.Missing, "Google Search Console verification code")
	}
	st.Configured = len(st.Missing) == 0
	return st
}
