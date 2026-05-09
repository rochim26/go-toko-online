package tracking

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/tokoonline/app/internal/services/security"
	"github.com/tokoonline/app/internal/services/settings"
)

type Service struct {
	settings *settings.Store
	hc       *http.Client
}

func New(s *settings.Store) *Service {
	return &Service{settings: s, hc: &http.Client{Timeout: 5 * time.Second}}
}

type Event struct {
	EventID    string
	EventName  string
	URL        string
	UserAgent  string
	IP         string
	Email      string
	Phone      string
	FBP        string
	FBC        string
	ContentIDs []string
	Currency   string
	Value      float64
	Quantity   int
}

// SendMetaCAPI sends event to Meta Conversions API.
// Returns nil silently if not configured (so callers don't have to check).
func (t *Service) SendMetaCAPI(ctx context.Context, ev Event) error {
	cfg := t.settings.Marketing()
	if cfg.MetaPixelID == "" || cfg.MetaCAPIToken == "" {
		return nil
	}
	user := map[string]any{
		"client_user_agent": ev.UserAgent,
		"client_ip_address": ev.IP,
	}
	if ev.Email != "" {
		user["em"] = []string{security.HashEmailForCAPI(ev.Email)}
	}
	if ev.Phone != "" {
		user["ph"] = []string{security.HashPhoneForCAPI(ev.Phone)}
	}
	if ev.FBP != "" {
		user["fbp"] = ev.FBP
	}
	if ev.FBC != "" {
		user["fbc"] = ev.FBC
	}
	custom := map[string]any{}
	if ev.Currency != "" {
		custom["currency"] = ev.Currency
	}
	if ev.Value > 0 {
		custom["value"] = ev.Value
	}
	if len(ev.ContentIDs) > 0 {
		custom["content_ids"] = ev.ContentIDs
		custom["content_type"] = "product"
	}
	body := map[string]any{
		"data": []map[string]any{
			{
				"event_name":     ev.EventName,
				"event_id":       ev.EventID,
				"event_time":     time.Now().Unix(),
				"action_source":  "website",
				"event_source_url": ev.URL,
				"user_data":      user,
				"custom_data":    custom,
			},
		},
		"access_token": cfg.MetaCAPIToken,
	}
	if cfg.MetaTestEventCode != "" {
		body["test_event_code"] = cfg.MetaTestEventCode
	}
	buf, _ := json.Marshal(body)
	url := fmt.Sprintf("https://graph.facebook.com/v19.0/%s/events", cfg.MetaPixelID)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	resp, err := t.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("meta CAPI: %s: %s", resp.Status, string(b))
	}
	return nil
}

// SendGA4MP sends event to GA4 Measurement Protocol.
func (t *Service) SendGA4MP(ctx context.Context, clientID string, ev Event) error {
	cfg := t.settings.Marketing()
	if cfg.GA4ID == "" || cfg.GA4APISecret == "" {
		return nil
	}
	if clientID == "" {
		clientID = ev.EventID
	}
	params := map[string]any{
		"engagement_time_msec": "100",
	}
	if ev.Currency != "" {
		params["currency"] = ev.Currency
	}
	if ev.Value > 0 {
		params["value"] = ev.Value
	}
	if len(ev.ContentIDs) > 0 {
		items := make([]map[string]any, 0, len(ev.ContentIDs))
		for _, id := range ev.ContentIDs {
			items = append(items, map[string]any{"item_id": id})
		}
		params["items"] = items
	}
	body := map[string]any{
		"client_id": clientID,
		"events": []map[string]any{
			{
				"name":   gaEventName(ev.EventName),
				"params": params,
			},
		},
	}
	buf, _ := json.Marshal(body)
	url := fmt.Sprintf("https://www.google-analytics.com/mp/collect?measurement_id=%s&api_secret=%s", cfg.GA4ID, cfg.GA4APISecret)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	resp, err := t.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ga4 MP: %s: %s", resp.Status, string(b))
	}
	return nil
}

func gaEventName(meta string) string {
	switch meta {
	case "ViewContent":
		return "view_item"
	case "AddToCart":
		return "add_to_cart"
	case "InitiateCheckout":
		return "begin_checkout"
	case "Purchase":
		return "purchase"
	case "AddPaymentInfo":
		return "add_payment_info"
	case "Lead":
		return "generate_lead"
	default:
		return meta
	}
}
