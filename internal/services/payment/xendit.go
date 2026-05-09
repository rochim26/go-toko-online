package payment

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/tokoonline/app/internal/services/settings"
)

type Xendit struct {
	settings *settings.Store
	hc       *http.Client
	baseURL  string
}

func New(s *settings.Store) *Xendit {
	return &Xendit{
		settings: s,
		hc:       &http.Client{Timeout: 15 * time.Second},
		baseURL:  "https://api.xendit.co",
	}
}

type InvoiceRequest struct {
	ExternalID         string   `json:"external_id"`
	Amount             float64  `json:"amount"`
	PayerEmail         string   `json:"payer_email,omitempty"`
	Description        string   `json:"description"`
	InvoiceDuration    int      `json:"invoice_duration,omitempty"`
	SuccessRedirectURL string   `json:"success_redirect_url,omitempty"`
	FailureRedirectURL string   `json:"failure_redirect_url,omitempty"`
	Currency           string   `json:"currency,omitempty"`
	Customer           Customer `json:"customer,omitempty"`
	Items              []Item   `json:"items,omitempty"`
	PaymentMethods     []string `json:"payment_methods,omitempty"`
}

type Customer struct {
	GivenNames   string `json:"given_names,omitempty"`
	Email        string `json:"email,omitempty"`
	MobileNumber string `json:"mobile_number,omitempty"`
}

type Item struct {
	Name     string  `json:"name"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
}

type Invoice struct {
	ID         string  `json:"id"`
	ExternalID string  `json:"external_id"`
	Status     string  `json:"status"`
	InvoiceURL string  `json:"invoice_url"`
	Amount     float64 `json:"amount"`
	ExpiryDate string  `json:"expiry_date"`
}

func (x *Xendit) CreateInvoice(ctx context.Context, in InvoiceRequest) (*Invoice, error) {
	cfg := x.settings.Xendit()
	if cfg.SecretKey == "" {
		return nil, errors.New("xendit secret key not configured")
	}
	if in.Currency == "" {
		in.Currency = "IDR"
	}
	if in.InvoiceDuration == 0 {
		in.InvoiceDuration = 86400 // 24 hours
	}
	if len(in.PaymentMethods) == 0 && len(cfg.MethodsEnabled) > 0 {
		// Map our friendly names to Xendit channel codes if needed
		// Empty = all enabled in dashboard
	}
	body, _ := json.Marshal(in)
	req, _ := http.NewRequestWithContext(ctx, "POST", x.baseURL+"/v2/invoices", bytes.NewReader(body))
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(cfg.SecretKey+":")))
	req.Header.Set("Content-Type", "application/json")
	resp, err := x.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("xendit invoice: %s: %s", resp.Status, string(rb))
	}
	var inv Invoice
	if err := json.Unmarshal(rb, &inv); err != nil {
		return nil, err
	}
	return &inv, nil
}

// VerifyWebhook checks the X-CALLBACK-TOKEN header against the configured token.
func (x *Xendit) VerifyWebhook(token string) bool {
	cfg := x.settings.Xendit()
	return cfg.WebhookToken != "" && cfg.WebhookToken == token
}

type WebhookInvoice struct {
	ID                 string  `json:"id"`
	ExternalID         string  `json:"external_id"`
	Status             string  `json:"status"`
	PaymentMethod      string  `json:"payment_method"`
	PaymentChannel     string  `json:"payment_channel"`
	Amount             float64 `json:"amount"`
	PaidAmount         float64 `json:"paid_amount"`
	PaidAt             string  `json:"paid_at"`
	PayerEmail         string  `json:"payer_email"`
	BankCode           string  `json:"bank_code"`
}
