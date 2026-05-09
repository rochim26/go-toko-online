package settings

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	mu        sync.RWMutex
	pool      *pgxpool.Pool
	cache     map[string]json.RawMessage
	loadedAt  time.Time
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool, cache: map[string]json.RawMessage{}}
}

func (s *Store) Reload(ctx context.Context) error {
	rows, err := s.pool.Query(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return err
	}
	defer rows.Close()
	next := map[string]json.RawMessage{}
	for rows.Next() {
		var k string
		var v json.RawMessage
		if err := rows.Scan(&k, &v); err != nil {
			return err
		}
		next[k] = v
	}
	s.mu.Lock()
	s.cache = next
	s.loadedAt = time.Now()
	s.mu.Unlock()
	return rows.Err()
}

func (s *Store) Get(key string, dst any) error {
	s.mu.RLock()
	raw, ok := s.cache[key]
	s.mu.RUnlock()
	if !ok {
		return nil
	}
	return json.Unmarshal(raw, dst)
}

func (s *Store) GetRaw(key string) json.RawMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cache[key]
}

func (s *Store) Set(ctx context.Context, key string, val any) error {
	b, err := json.Marshal(val)
	if err != nil {
		return err
	}
	if _, err := s.pool.Exec(ctx, `INSERT INTO settings(key,value,updated_at) VALUES($1,$2,now())
		ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value, updated_at=now()`, key, b); err != nil {
		return err
	}
	return s.Reload(ctx)
}

// ─────────────────────────────────────────────
// Typed accessors

type Store_ = Store

type StoreInfo struct {
	Name             string `json:"name"`
	Tagline          string `json:"tagline"`
	LogoURL          string `json:"logo_url"`
	FaviconURL       string `json:"favicon_url"`
	Email            string `json:"email"`
	Phone            string `json:"phone"`
	WANumber         string `json:"wa_number"`
	Address          string `json:"address"`
	OriginAreaID     string `json:"origin_area_id"`
	OriginPostalCode string `json:"origin_postal_code"`
}

type SEOGlobal struct {
	TitlePattern        string `json:"title_pattern"`
	DefaultTitle        string `json:"default_title"`
	DefaultDesc         string `json:"default_desc"`
	DefaultOGImage      string `json:"default_og_image"`
	RobotsExtra         string `json:"robots_extra"`
	GSCVerification     string `json:"gsc_verification"`
	BingVerification    string `json:"bing_verification"`
	AIOverviewOptimized bool   `json:"ai_overview_optimized"`
}

type Marketing struct {
	MetaPixelID       string `json:"meta_pixel_id"`
	MetaCAPIToken     string `json:"meta_capi_token"`
	MetaTestEventCode string `json:"meta_test_event_code"`
	GA4ID             string `json:"ga4_id"`
	GA4APISecret      string `json:"ga4_api_secret"`
	GTMID             string `json:"gtm_id"`
	TikTokPixelID     string `json:"tiktok_pixel_id"`
	GoogleAdsID       string `json:"google_ads_id"`
	GoogleAdsLabel    string `json:"google_ads_label"`
}

type GMC struct {
	MerchantID       string `json:"merchant_id"`
	FeedEnabled      bool   `json:"feed_enabled"`
	FeedFormat       string `json:"feed_format"`
	AutoDisableOOS   bool   `json:"auto_disable_oos"`
	ShippingCountry  string `json:"shipping_country"`
	ContentLanguage  string `json:"content_language"`
	TargetCountry    string `json:"target_country"`
}

type Xendit struct {
	SecretKey       string   `json:"secret_key"`
	WebhookToken    string   `json:"webhook_token"`
	PublicKey       string   `json:"public_key"`
	MethodsEnabled  []string `json:"methods_enabled"`
	SuccessRedirect string   `json:"success_redirect"`
	FailureRedirect string   `json:"failure_redirect"`
}

type Biteship struct {
	APIKey           string   `json:"api_key"`
	OriginAreaID     string   `json:"origin_area_id"`
	OriginPostalCode string   `json:"origin_postal_code"`
	Couriers         []string `json:"couriers"`
}

type Shipping struct {
	FreeShippingThreshold float64 `json:"free_shipping_threshold"`
	FlatRateFallback      float64 `json:"flat_rate_fallback"`
}

type Mailer struct {
	SMTPHost  string `json:"smtp_host"`
	SMTPPort  int    `json:"smtp_port"`
	SMTPUser  string `json:"smtp_user"`
	SMTPPass  string `json:"smtp_pass"`
	FromEmail string `json:"from_email"`
	FromName  string `json:"from_name"`
}

type Tax struct {
	PPNPct        float64 `json:"ppn_pct"`
	InvoicePrefix string  `json:"invoice_prefix"`
	InvoiceNPWP   string  `json:"invoice_npwp"`
}

type Reseller struct {
	RegistrationOpen bool    `json:"registration_open"`
	AutoApprove      bool    `json:"auto_approve"`
	RequireNPWP      bool    `json:"require_npwp"`
	RequireKTP       bool    `json:"require_ktp"`
	MinFirstOrder    float64 `json:"min_first_order"`
}

func (s *Store) Store() StoreInfo   { var v StoreInfo; _ = s.Get("store", &v); return v }
func (s *Store) SEO() SEOGlobal     { var v SEOGlobal; _ = s.Get("seo_global", &v); return v }
func (s *Store) Marketing() Marketing { var v Marketing; _ = s.Get("marketing", &v); return v }
func (s *Store) GMC() GMC           { var v GMC; _ = s.Get("gmc", &v); return v }
func (s *Store) Xendit() Xendit     { var v Xendit; _ = s.Get("xendit", &v); return v }
func (s *Store) Biteship() Biteship { var v Biteship; _ = s.Get("biteship", &v); return v }
func (s *Store) Shipping() Shipping { var v Shipping; _ = s.Get("shipping", &v); return v }
func (s *Store) Mailer() Mailer     { var v Mailer; _ = s.Get("mailer", &v); return v }
func (s *Store) Tax() Tax           { var v Tax; _ = s.Get("tax", &v); return v }
func (s *Store) Reseller() Reseller { var v Reseller; _ = s.Get("reseller", &v); return v }
