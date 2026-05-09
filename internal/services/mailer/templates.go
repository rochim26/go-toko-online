package mailer

import (
	"fmt"
	"strings"

	"github.com/tokoonline/app/internal/httpx"
	"github.com/tokoonline/app/internal/models"
)

const baseStyle = `<style>
body{margin:0;padding:0;background:#f8fafc;font-family:system-ui,-apple-system,Segoe UI,Roboto,sans-serif;color:#0f172a;line-height:1.5}
.wrap{max-width:560px;margin:0 auto;padding:24px}
.card{background:#fff;border:1px solid #e2e8f0;border-radius:12px;padding:24px}
.btn{display:inline-block;padding:12px 20px;background:#0f172a;color:#fff !important;border-radius:8px;text-decoration:none;font-weight:500}
.btn-accent{background:#dc2626}
h1{margin:0 0 12px;font-size:1.5rem}
h2{font-size:1.1rem;margin:1.25rem 0 .5rem}
table{width:100%;border-collapse:collapse;margin:1rem 0}
td,th{padding:6px 8px;border-bottom:1px solid #e2e8f0;text-align:left;font-size:.9rem}
th{background:#f8fafc;color:#475569;font-weight:600;font-size:.75rem;text-transform:uppercase}
.muted{color:#64748b;font-size:.85rem}
.total{font-weight:700;font-size:1.05rem;border-top:1px solid #e2e8f0;padding-top:8px;margin-top:8px}
.footer{text-align:center;color:#94a3b8;font-size:.8rem;padding-top:16px}
</style>`

func wrap(storeName, title, inner string) string {
	return fmt.Sprintf(`<!DOCTYPE html><html lang="id"><head><meta charset="utf-8"/>%s</head><body><div class="wrap">
<div style="text-align:center;padding:12px 0;font-weight:700;font-size:1.1rem">%s</div>
<div class="card">%s</div>
<div class="footer">Email otomatis dari %s. Mohon tidak membalas email ini.</div>
</div></body></html>`, baseStyle, storeName, inner, storeName)
}

// OrderConfirmation - sent right after order creation (awaiting payment or TOP)
func OrderConfirmation(storeName, baseURL string, o *models.Order) (subject, body string) {
	subject = fmt.Sprintf("[%s] Pesanan %s diterima", storeName, o.Code)
	var rows strings.Builder
	for _, it := range o.Items {
		rows.WriteString(fmt.Sprintf(`<tr><td>%s<br/><span class="muted">%s</span></td><td>%d</td><td>%s</td></tr>`,
			esc(it.Name), esc(it.SKU), it.Qty, httpx.IDR(it.LineTotal)))
	}
	cta := fmt.Sprintf(`<a class="btn btn-accent" href="%s/account/orders/%s">Lihat & Bayar</a>`, baseURL, o.Code)
	if o.PaymentTerm == "top" {
		cta = fmt.Sprintf(`<a class="btn" href="%s/account/orders/%s/po.pdf">Download PO PDF</a> &nbsp; <a class="btn btn-accent" href="%s/account/orders/%s">Detail Pesanan</a>`, baseURL, o.Code, baseURL, o.Code)
	}
	inner := fmt.Sprintf(`<h1>Terima kasih, pesanan Anda diterima</h1>
<p class="muted">No. Pesanan: <strong>%s</strong></p>
<p>Hi <strong>%s</strong>,<br/>Pesanan Anda di %s sudah kami terima. Berikut detailnya:</p>
<table><thead><tr><th>Produk</th><th>Qty</th><th>Total</th></tr></thead><tbody>%s</tbody></table>
<table>
<tr><td>Subtotal</td><td colspan="2" style="text-align:right">%s</td></tr>
<tr><td>Ongkos Kirim</td><td colspan="2" style="text-align:right">%s</td></tr>
<tr><td class="total">Grand Total</td><td colspan="2" class="total" style="text-align:right">%s</td></tr>
</table>
<p>%s</p>
<p style="margin-top:16px">%s</p>`,
		o.Code, esc(deref(o.CustomerName)), storeName, rows.String(),
		httpx.IDR(o.Subtotal), httpx.IDR(o.ShippingTotal), httpx.IDR(o.GrandTotal),
		paymentInstructions(o, baseURL), cta)
	body = wrap(storeName, subject, inner)
	return
}

// PaymentReceived - sent after Xendit webhook confirms payment
func PaymentReceived(storeName, baseURL string, o *models.Order) (subject, body string) {
	subject = fmt.Sprintf("[%s] Pembayaran %s diterima ✓", storeName, o.Code)
	inner := fmt.Sprintf(`<h1>Pembayaran Diterima ✓</h1>
<p>Hi <strong>%s</strong>,</p>
<p>Pembayaran untuk pesanan <strong>%s</strong> sudah kami terima. Pesanan Anda akan segera kami proses dan kirim.</p>
<p><strong>Total dibayar:</strong> %s<br/><strong>Metode:</strong> %s</p>
<p style="margin-top:16px"><a class="btn btn-accent" href="%s/account/orders/%s">Lacak Pesanan</a></p>`,
		esc(deref(o.CustomerName)), o.Code, httpx.IDR(o.PaidTotal), esc(deref(o.PaymentMethod)),
		baseURL, o.Code)
	body = wrap(storeName, subject, inner)
	return
}

// OrderShipped - sent when admin updates status to shipped with AWB
func OrderShipped(storeName, baseURL string, o *models.Order) (subject, body string) {
	subject = fmt.Sprintf("[%s] Pesanan %s dikirim 📦", storeName, o.Code)
	inner := fmt.Sprintf(`<h1>Pesanan Dalam Perjalanan</h1>
<p>Hi <strong>%s</strong>,</p>
<p>Pesanan <strong>%s</strong> sudah kami kirim via <strong>%s</strong>.</p>
<p><strong>Nomor Resi:</strong> %s</p>
<p style="margin-top:16px"><a class="btn btn-accent" href="%s/account/orders/%s">Detail Pesanan</a></p>`,
		esc(deref(o.CustomerName)), o.Code, esc(strings.ToUpper(deref(o.CourierCode))),
		esc(deref(o.AWB)), baseURL, o.Code)
	body = wrap(storeName, subject, inner)
	return
}

// AbandonedCart - sent N hours after cart sat untouched with email captured
func AbandonedCart(storeName, baseURL, customerName string, items []models.CartItem) (subject, body string) {
	subject = fmt.Sprintf("[%s] Masih ada barang di keranjang Anda 🛒", storeName)
	var rows strings.Builder
	var subtotal float64
	for _, it := range items {
		subtotal += it.UnitPrice * float64(it.Qty)
		rows.WriteString(fmt.Sprintf(`<tr><td>%s</td><td>%d</td><td>%s</td></tr>`,
			esc(it.ProductName), it.Qty, httpx.IDR(it.UnitPrice*float64(it.Qty))))
	}
	inner := fmt.Sprintf(`<h1>Lupa sesuatu? 🛒</h1>
<p>Hi%s,</p>
<p>Anda meninggalkan beberapa barang di keranjang. Kami simpankan dulu, klik di bawah untuk melanjutkan checkout.</p>
<table><thead><tr><th>Produk</th><th>Qty</th><th>Subtotal</th></tr></thead><tbody>%s</tbody>
<tfoot><tr><td colspan="2" class="total" style="text-align:right">Total</td><td class="total">%s</td></tr></tfoot></table>
<p style="text-align:center;margin-top:16px"><a class="btn btn-accent" href="%s/cart">Lanjutkan Checkout</a></p>`,
		ifNonempty(", "+customerName), rows.String(), httpx.IDR(subtotal), baseURL)
	body = wrap(storeName, subject, inner)
	return
}

// ResellerPending - sent on registration
func ResellerPending(storeName, customerName string) (subject, body string) {
	subject = fmt.Sprintf("[%s] Pendaftaran reseller kami terima", storeName)
	inner := fmt.Sprintf(`<h1>Terima kasih sudah mendaftar</h1>
<p>Hi%s,</p>
<p>Pendaftaran Anda sebagai reseller %s sudah kami terima dan sedang direview oleh admin. Kami akan kirim email lanjutan setelah akun Anda diaktifkan.</p>
<p>Estimasi: 1×24 jam pada hari kerja.</p>`, ifNonempty(", "+customerName), storeName)
	body = wrap(storeName, subject, inner)
	return
}

// ResellerApproved
func ResellerApproved(storeName, baseURL, customerName, tierName string, discount float64, moq int) (subject, body string) {
	subject = fmt.Sprintf("[%s] Akun reseller Anda sudah aktif ✓", storeName)
	inner := fmt.Sprintf(`<h1>Akun Reseller Aktif ✓</h1>
<p>Hi%s,</p>
<p>Selamat! Akun reseller Anda sudah disetujui dengan tier <strong>%s</strong>.</p>
<ul><li>Diskon: <strong>%.0f%%</strong> dari harga retail</li><li>Minimum order: <strong>%d pcs</strong></li></ul>
<p style="margin-top:16px"><a class="btn btn-accent" href="%s/reseller">Mulai Belanja</a></p>`,
		ifNonempty(", "+customerName), esc(tierName), discount, moq, baseURL)
	body = wrap(storeName, subject, inner)
	return
}

// ResellerRejected
func ResellerRejected(storeName, customerName, reason string) (subject, body string) {
	subject = fmt.Sprintf("[%s] Status pendaftaran reseller", storeName)
	r := "Mohon maaf, pendaftaran reseller Anda belum bisa kami setujui saat ini."
	if reason != "" {
		r += " Alasan: " + reason + "."
	}
	inner := fmt.Sprintf(`<h1>Pendaftaran Belum Disetujui</h1>
<p>Hi%s,</p><p>%s</p><p>Anda dapat menghubungi customer service kami untuk informasi lebih lanjut.</p>`,
		ifNonempty(", "+customerName), r)
	body = wrap(storeName, subject, inner)
	return
}

// PasswordChanged - security notice
func PasswordChanged(storeName, customerName string) (subject, body string) {
	subject = fmt.Sprintf("[%s] Password akun Anda baru saja diubah", storeName)
	inner := fmt.Sprintf(`<h1>Password Diubah</h1>
<p>Hi%s,</p><p>Password akun Anda baru saja diubah. Jika ini bukan Anda, segera hubungi kami.</p>`, ifNonempty(", "+customerName))
	body = wrap(storeName, subject, inner)
	return
}

// helpers

func paymentInstructions(o *models.Order, baseURL string) string {
	if o.PaymentTerm == "top" && o.TopDueAt != nil {
		return fmt.Sprintf(`<strong>Tagihan Tempo:</strong> Pesanan ini menggunakan TOP. Mohon lakukan pembayaran sebelum <strong>%s</strong>. Detail rekening tujuan akan dikirim terpisah.`,
			httpx.DateID(*o.TopDueAt))
	}
	if o.XenditInvoiceURL != nil && *o.XenditInvoiceURL != "" {
		return fmt.Sprintf(`<strong>Pembayaran:</strong> Klik tombol di bawah untuk membayar via Xendit. Invoice akan kadaluarsa 24 jam.`)
	}
	return "Pesanan akan diproses setelah pembayaran kami terima."
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
func ifNonempty(s string) string {
	if strings.TrimSpace(s) == "," {
		return ""
	}
	return s
}
func esc(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")
	return r.Replace(s)
}
