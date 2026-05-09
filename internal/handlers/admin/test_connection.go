package admin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/tokoonline/app/internal/middleware"
	"github.com/tokoonline/app/internal/services/mailer"
	"github.com/tokoonline/app/internal/services/security"
)

// TestConnection runs a quick health check against the configured external service
// and returns an HTML snippet (HTMX-targeted) showing OK/error.
func (h *Handler) TestConnection(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	provider := chi.URLParam(r, "provider")
	switch provider {
	case "xendit":
		h.testXendit(w, r)
	case "biteship":
		h.testBiteship(w, r)
	case "mailer":
		h.testMailer(w, r)
	default:
		fmt.Fprint(w, `<div class="flash flash-error">Provider tidak dikenal.</div>`)
	}
}

func (h *Handler) testXendit(w http.ResponseWriter, r *http.Request) {
	cfg := h.Settings.Xendit()
	if cfg.SecretKey == "" {
		fmt.Fprint(w, `<div class="flash flash-error">⚠ Secret Key belum diisi.</div>`)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	// Hit /v2/invoices/ list as a cheap auth check (returns 200 even if list empty)
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.xendit.co/v2/invoices?limit=1", nil)
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(cfg.SecretKey+":")))
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		fmt.Fprintf(w, `<div class="flash flash-error">⚠ Gagal koneksi: %s</div>`, err.Error())
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		fmt.Fprintf(w, `<div class="flash flash-error">⚠ Secret Key ditolak (HTTP %d). Pastikan key benar dan punya permission "Money-in / Invoice".</div>`, resp.StatusCode)
		return
	}
	if resp.StatusCode >= 400 {
		fmt.Fprintf(w, `<div class="flash flash-error">⚠ HTTP %d dari Xendit: %s</div>`, resp.StatusCode, escapeHTML(truncate(string(body), 300)))
		return
	}
	fmt.Fprintf(w, `<div class="flash flash-success">✓ Xendit terhubung (HTTP %d). Webhook URL: <code>%s/webhooks/xendit</code> — pastikan terdaftar di Xendit dashboard.</div>`, resp.StatusCode, h.BaseURL)
}

func (h *Handler) testBiteship(w http.ResponseWriter, r *http.Request) {
	cfg := h.Settings.Biteship()
	if cfg.APIKey == "" {
		fmt.Fprint(w, `<div class="flash flash-error">⚠ API Key belum diisi.</div>`)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.biteship.com/v1/maps/areas?countries=ID&input=jakarta&type=single", nil)
	req.Header.Set("Authorization", cfg.APIKey)
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		fmt.Fprintf(w, `<div class="flash flash-error">⚠ Gagal koneksi: %s</div>`, err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		fmt.Fprintf(w, `<div class="flash flash-error">⚠ API Key ditolak (HTTP %d).</div>`, resp.StatusCode)
		return
	}
	body, _ := io.ReadAll(resp.Body)
	var out struct {
		Success bool          `json:"success"`
		Areas   []interface{} `json:"areas"`
	}
	_ = json.Unmarshal(body, &out)
	if resp.StatusCode >= 400 {
		fmt.Fprintf(w, `<div class="flash flash-error">⚠ HTTP %d: %s</div>`, resp.StatusCode, escapeHTML(truncate(string(body), 300)))
		return
	}
	fmt.Fprintf(w, `<div class="flash flash-success">✓ Biteship terhubung. Test query "jakarta" mengembalikan %d area. Pastikan Origin Area ID dan Postal Code di tab Toko sudah benar agar ongkir bisa terhitung.</div>`, len(out.Areas))
}

func (h *Handler) testMailer(w http.ResponseWriter, r *http.Request) {
	// Send to currently logged-in admin so they can verify it actually landed
	to := middlewareUserEmail(r)
	if to == "" {
		to = h.Settings.Mailer().FromEmail
	}
	if to == "" {
		fmt.Fprint(w, `<div class="flash flash-error">⚠ Tidak bisa menentukan email tujuan test.</div>`)
		return
	}
	store := h.Settings.Store().Name
	if store == "" {
		store = "Toko"
	}
	subj := fmt.Sprintf("[%s] Test email %s", store, time.Now().Format("15:04:05"))
	body := fmt.Sprintf(`<p>Halo,</p><p>Ini adalah test email dari panel admin Anda. Jika email ini sampai inbox (bukan spam), berarti pipeline DKIM/SPF/DMARC sudah berjalan baik.</p><p>Toko: <strong>%s</strong><br/>Waktu: %s</p>`, store, time.Now().Format("2 January 2006 15:04:05"))
	if err := h.Mailer.Send(r.Context(), to, subj, body); err != nil {
		fmt.Fprintf(w, `<div class="flash flash-error">⚠ Gagal kirim: %s</div>`, escapeHTML(err.Error()))
		return
	}
	fmt.Fprintf(w, `<div class="flash flash-success">✓ Test email dikirim ke <strong>%s</strong>. Cek inbox (dan folder Spam) — biasanya sampai &lt; 30 detik.</div>`, escapeHTML(to))
}

func escapeHTML(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")
	return r.Replace(s)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

// keep mailer & security imports referenced
var _ = mailer.New
var _ = security.RandomToken

func middlewareUserEmail(r *http.Request) string {
	return middleware.UserEmail(r)
}
