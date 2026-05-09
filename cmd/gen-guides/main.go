// gen-guides generates 3 PDF user guides into static/guides/.
// Run: go run ./cmd/gen-guides
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/jung-kurt/gofpdf"
)

const (
	brandName    = "GlowMart Beauty"
	brandTagline = "Skincare & makeup pilihan, original 100%"
	colorAccent  = "#FF3A44"
	primaryDark  = "#0B1220"
)

func main() {
	outDir := "static/guides"
	if len(os.Args) > 1 {
		outDir = os.Args[1]
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		log.Fatalf("mkdir %s: %v", outDir, err)
	}

	guides := []struct {
		filename string
		title    string
		audience string
		build    func(*gofpdf.Fpdf)
	}{
		{"panduan-pembeli.pdf", "Panduan Pembeli", "Pembeli / Customer", buildBuyer},
		{"panduan-reseller.pdf", "Panduan Reseller", "Reseller / B2B", buildReseller},
		{"panduan-admin.pdf", "Panduan Admin", "Admin / Staff", buildAdmin},
	}

	for _, g := range guides {
		pdf := gofpdf.New("P", "mm", "A4", "")
		pdf.SetMargins(18, 22, 18)
		pdf.SetAutoPageBreak(true, 22)
		pdf.SetTitle(g.title+" - "+brandName, true)
		pdf.SetAuthor(brandName, true)
		pdf.AliasNbPages("")
		pdf.SetFooterFunc(func() {
			pdf.SetY(-15)
			pdf.SetFont("Helvetica", "I", 8)
			pdf.SetTextColor(120, 120, 120)
			pdf.CellFormat(0, 8, brandName+" • "+g.title+" • Halaman "+itoa(pdf.PageNo())+" / {nb}", "", 0, "C", false, 0, "")
		})

		drawCover(pdf, g.title, g.audience)
		pdf.AddPage()
		g.build(pdf)

		path := filepath.Join(outDir, g.filename)
		if err := pdf.OutputFileAndClose(path); err != nil {
			log.Fatalf("write %s: %v", path, err)
		}
		fmt.Printf("✓ %s (%d page)\n", path, pdf.PageNo())
	}
}

// ─────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}

func setFillHex(pdf *gofpdf.Fpdf, hex string) {
	r, g, b := hexRGB(hex)
	pdf.SetFillColor(r, g, b)
}

func setTextHex(pdf *gofpdf.Fpdf, hex string) {
	r, g, b := hexRGB(hex)
	pdf.SetTextColor(r, g, b)
}

func setDrawHex(pdf *gofpdf.Fpdf, hex string) {
	r, g, b := hexRGB(hex)
	pdf.SetDrawColor(r, g, b)
}

func hexRGB(s string) (int, int, int) {
	if len(s) == 7 && s[0] == '#' {
		s = s[1:]
	}
	parseHex := func(h string) int {
		v := 0
		for i := 0; i < len(h); i++ {
			c := h[i]
			d := 0
			switch {
			case c >= '0' && c <= '9':
				d = int(c - '0')
			case c >= 'a' && c <= 'f':
				d = int(c-'a') + 10
			case c >= 'A' && c <= 'F':
				d = int(c-'A') + 10
			}
			v = v*16 + d
		}
		return v
	}
	if len(s) == 6 {
		return parseHex(s[0:2]), parseHex(s[2:4]), parseHex(s[4:6])
	}
	return 0, 0, 0
}

// drawCover renders the front cover page.
func drawCover(pdf *gofpdf.Fpdf, title, audience string) {
	pdf.AddPage()
	w, h := pdf.GetPageSize()

	// Top accent bar
	setFillHex(pdf, primaryDark)
	pdf.Rect(0, 0, w, 70, "F")

	// Brand
	setTextHex(pdf, "#ffffff")
	pdf.SetFont("Helvetica", "B", 22)
	pdf.SetXY(18, 28)
	pdf.Cell(0, 10, brandName)
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetXY(18, 42)
	pdf.Cell(0, 6, brandTagline)

	// Accent decorative dot
	setFillHex(pdf, colorAccent)
	pdf.Circle(w-30, 35, 7, "F")
	pdf.Circle(w-22, 50, 4, "F")

	// Title
	setTextHex(pdf, primaryDark)
	pdf.SetFont("Helvetica", "B", 32)
	pdf.SetXY(18, 90)
	pdf.MultiCell(w-36, 14, title, "", "L", false)

	// Audience tag
	pdf.Ln(2)
	setFillHex(pdf, colorAccent)
	setTextHex(pdf, "#ffffff")
	pdf.SetFont("Helvetica", "B", 10)
	pdf.SetX(18)
	pdf.CellFormat(60, 8, "  UNTUK: "+audience, "", 0, "L", true, 0, "")
	pdf.Ln(20)

	// Body intro
	setTextHex(pdf, "#475569")
	pdf.SetFont("Helvetica", "", 11)
	pdf.SetX(18)
	pdf.MultiCell(w-36, 6, "Panduan lengkap untuk membantu Anda menggunakan platform "+brandName+" dengan mudah dan efektif. Ikuti langkah-langkah di dalamnya.", "", "L", false)

	// Footer of cover
	pdf.SetY(h - 35)
	setDrawHex(pdf, "#e5e7eb")
	pdf.SetLineWidth(0.3)
	pdf.Line(18, h-30, w-18, h-30)

	setTextHex(pdf, "#94a3b8")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetXY(18, h-25)
	pdf.Cell(0, 5, "Versi 1.0 - "+time.Now().Format("January 2006"))
	pdf.SetXY(18, h-19)
	pdf.Cell(0, 5, "Web: https://toko.mdt.biz.id")
}

// h1 renders a section heading with accent bar.
func h1(pdf *gofpdf.Fpdf, num, text string) {
	pdf.Ln(4)
	setDrawHex(pdf, colorAccent)
	pdf.SetLineWidth(1.2)
	x := pdf.GetX()
	y := pdf.GetY()
	pdf.Line(x, y+1, x, y+10)
	pdf.SetX(x + 2.5)
	setTextHex(pdf, primaryDark)
	pdf.SetFont("Helvetica", "B", 15)
	pdf.CellFormat(0, 10, num+"  "+text, "", 1, "L", false, 0, "")
	pdf.Ln(1)
}

// h2 — sub-heading
func h2(pdf *gofpdf.Fpdf, text string) {
	pdf.Ln(2)
	setTextHex(pdf, primaryDark)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(0, 7, text, "", 1, "L", false, 0, "")
}

// p — paragraph (justified, regular)
func p(pdf *gofpdf.Fpdf, text string) {
	setTextHex(pdf, "#1f2937")
	pdf.SetFont("Helvetica", "", 10)
	w, _ := pdf.GetPageSize()
	l, _, r, _ := pdf.GetMargins()
	pdf.MultiCell(w-l-r, 5.5, text, "", "L", false)
	pdf.Ln(2)
}

// li — bullet list item
func li(pdf *gofpdf.Fpdf, text string) {
	setTextHex(pdf, "#1f2937")
	pdf.SetFont("Helvetica", "", 10)
	w, _ := pdf.GetPageSize()
	l, _, r, _ := pdf.GetMargins()
	startX := pdf.GetX()
	startY := pdf.GetY()
	setFillHex(pdf, colorAccent)
	pdf.Circle(startX+2, startY+2.7, 0.9, "F")
	pdf.SetX(startX + 6)
	pdf.MultiCell(w-l-r-6, 5.5, text, "", "L", false)
}

// step renders a numbered step.
func step(pdf *gofpdf.Fpdf, n int, title, body string) {
	pdf.Ln(1)
	startX := pdf.GetX()
	startY := pdf.GetY()
	// Number circle
	setFillHex(pdf, primaryDark)
	pdf.Circle(startX+3.5, startY+3.5, 3.5, "F")
	setTextHex(pdf, "#ffffff")
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetXY(startX+1.4, startY+1.1)
	pdf.CellFormat(4, 5, fmt.Sprintf("%d", n), "", 0, "C", false, 0, "")

	// Title
	pdf.SetXY(startX+9, startY)
	setTextHex(pdf, primaryDark)
	pdf.SetFont("Helvetica", "B", 11)
	w, _ := pdf.GetPageSize()
	l, _, r, _ := pdf.GetMargins()
	pdf.CellFormat(w-l-r-9, 6, title, "", 1, "L", false, 0, "")
	pdf.SetX(startX + 9)
	setTextHex(pdf, "#1f2937")
	pdf.SetFont("Helvetica", "", 10)
	pdf.MultiCell(w-l-r-9, 5.3, body, "", "L", false)
	pdf.Ln(2)
}

// note — a callout box (info/tip)
func note(pdf *gofpdf.Fpdf, kind, text string) {
	pdf.Ln(2)
	w, _ := pdf.GetPageSize()
	l, _, r, _ := pdf.GetMargins()
	startX := pdf.GetX()
	startY := pdf.GetY()
	bgHex := "#eff6ff"
	barHex := "#3b82f6"
	label := "INFO"
	switch kind {
	case "tip":
		bgHex = "#fff7ed"
		barHex = "#f59e0b"
		label = "TIPS"
	case "warn":
		bgHex = "#fef2f2"
		barHex = "#dc2626"
		label = "PENTING"
	}
	setFillHex(pdf, bgHex)
	pdf.Rect(startX, startY, w-l-r, 18, "F")
	setFillHex(pdf, barHex)
	pdf.Rect(startX, startY, 1.5, 18, "F")

	setTextHex(pdf, barHex)
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetXY(startX+4, startY+2)
	pdf.Cell(0, 5, label)

	setTextHex(pdf, "#374151")
	pdf.SetFont("Helvetica", "", 9.5)
	pdf.SetXY(startX+4, startY+7)
	pdf.MultiCell(w-l-r-8, 4.5, text, "", "L", false)
	pdf.Ln(3)
}

// faqQ renders a Q/A pair.
func faqQ(pdf *gofpdf.Fpdf, q, a string) {
	setTextHex(pdf, primaryDark)
	pdf.SetFont("Helvetica", "B", 10.5)
	w, _ := pdf.GetPageSize()
	l, _, r, _ := pdf.GetMargins()
	pdf.MultiCell(w-l-r, 5.5, "Q: "+q, "", "L", false)
	setTextHex(pdf, "#475569")
	pdf.SetFont("Helvetica", "", 10)
	pdf.MultiCell(w-l-r, 5.3, "A: "+a, "", "L", false)
	pdf.Ln(2)
}

// ─────────────────────────────────────────────────────────────
// PEMBELI
// ─────────────────────────────────────────────────────────────
func buildBuyer(pdf *gofpdf.Fpdf) {
	h1(pdf, "1.", "Mengenal "+brandName)
	p(pdf, brandName+" adalah toko online produk kecantikan original — skincare, makeup, body care, hair care, fragrance, dan tools — dengan pengiriman ke seluruh Indonesia. Anda dapat berbelanja sebagai tamu (guest) atau dengan akun terdaftar.")
	li(pdf, "Pengiriman cepat via JNE, J&T, SiCepat, AnterAja, dan POS Indonesia.")
	li(pdf, "Pembayaran aman: Transfer Bank, Virtual Account, QRIS, e-Wallet (OVO/Gopay/Dana), dan COD untuk wilayah tertentu.")
	li(pdf, "Garansi 100% original — uang kembali jika produk tidak sesuai.")
	li(pdf, "Customer Service responsif via WhatsApp.")

	h1(pdf, "2.", "Membuat Akun")
	step(pdf, 1, "Klik Daftar", "Di pojok kanan atas situs, klik Daftar. Isi nama, email, nomor HP/WhatsApp, dan password (minimal 8 karakter).")
	step(pdf, 2, "Verifikasi", "Cek inbox email Anda untuk link verifikasi (jika diminta). Klik link untuk mengaktifkan akun.")
	step(pdf, 3, "Login", "Setelah aktif, login dengan email dan password Anda. Anda akan diarahkan ke halaman akun.")
	note(pdf, "tip", "Anda juga bisa belanja TANPA daftar (guest checkout). Tapi dengan daftar, alamat Anda akan tersimpan untuk pembelian berikutnya — lebih cepat.")

	h1(pdf, "3.", "Mencari Produk")
	li(pdf, "Cari di kotak pencarian di header (atas) — bisa berdasarkan nama produk, brand, atau kategori.")
	li(pdf, "Atau jelajahi via menu Kategori (Skincare, Makeup, Body Care, Hair Care, Fragrance, Tools).")
	li(pdf, "Di halaman katalog, gunakan dropdown Urutkan untuk sort harga termurah/termahal/terbaru.")
	li(pdf, "Klik ikon hati di kartu produk untuk simpan ke wishlist (tersimpan di browser Anda).")

	h1(pdf, "4.", "Menambahkan ke Keranjang & Beli Sekarang")
	step(pdf, 1, "Buka halaman produk", "Klik produk yang Anda mau. Anda akan melihat foto, harga, deskripsi, varian (jika ada), dan stok.")
	step(pdf, 2, "Pilih kuantitas & varian", "Atur jumlah dengan tombol +/-. Pilih varian (warna/ukuran) jika tersedia.")
	step(pdf, 3, "Klik tombol", "+ Keranjang untuk simpan ke keranjang & lanjut belanja, atau Beli Sekarang untuk langsung checkout.")
	note(pdf, "info", "Notifikasi 'Ditambahkan ke keranjang' akan muncul di pojok bawah. Klik ikon keranjang di header untuk melihat isi keranjang.")

	h1(pdf, "5.", "Checkout")
	step(pdf, 1, "Lengkapi data kontak", "Isi email, nama lengkap, dan nomor HP yang aktif. Email akan menerima konfirmasi pesanan.")
	step(pdf, 2, "Cari alamat pengiriman", "Ketik nama kecamatan/kota di kolom Cari Kecamatan/Kota/Kode Pos. Pilih dari dropdown — kode pos otomatis terisi.")
	step(pdf, 3, "Lengkapi alamat lengkap", "Tulis jalan, nomor rumah, RT/RW, dan patokan untuk memudahkan kurir.")
	step(pdf, 4, "Pilih kurir", "Setelah alamat dipilih, daftar kurir & ongkir muncul otomatis. Klik salah satu kurir.")
	step(pdf, 5, "Bayar", "Klik Bayar Sekarang. Anda akan dialihkan ke halaman pembayaran. Selesaikan pembayaran sesuai instruksi.")
	note(pdf, "warn", "Pesanan akan otomatis dibatalkan jika pembayaran tidak diselesaikan dalam 24 jam.")

	h1(pdf, "6.", "Cek Status Pesanan")
	p(pdf, "Setelah bayar, Anda dapat memantau status pesanan dari halaman akun:")
	li(pdf, "Login -> menu Akun -> Pesanan, atau langsung buka /account/orders.")
	li(pdf, "Status pesanan: Pending Bayar -> Dibayar -> Diproses -> Dikirim -> Selesai.")
	li(pdf, "Saat status Dikirim, Anda akan menerima nomor resi via email.")
	li(pdf, "Klik Reorder pada pesanan lama untuk membeli ulang produk yang sama.")

	h1(pdf, "7.", "Mengelola Alamat & Profil")
	li(pdf, "Akun -> Alamat: tambah, edit, hapus alamat. Tandai satu sebagai Default.")
	li(pdf, "Akun -> Profil: ubah nama, nomor HP. Email tidak bisa diubah dari sini.")
	li(pdf, "Akun -> Password: ganti password berkala untuk keamanan.")
	note(pdf, "tip", "Saat checkout, alamat default Anda akan langsung dipilih — checkout 1 klik!")

	h1(pdf, "8.", "Pertanyaan Umum (FAQ)")
	faqQ(pdf, "Apakah produk yang dijual original?", "Ya, semua produk 100% original dari distributor resmi. Kami berikan garansi tukar produk bila tidak sesuai.")
	faqQ(pdf, "Berapa lama pengiriman?", "Jabodetabek 1-2 hari kerja, Pulau Jawa 2-3 hari, luar Jawa 3-9 hari tergantung lokasi & kurir.")
	faqQ(pdf, "Saya bisa retur produk?", "Bisa dalam 7 hari setelah barang diterima dengan syarat: kemasan masih segel, foto/video unboxing tersedia, dan alasan jelas (rusak, salah barang).")
	faqQ(pdf, "Pembayaran apa saja yang diterima?", "Transfer Bank (BCA, Mandiri, BRI, BNI), Virtual Account, QRIS, e-Wallet (OVO/Gopay/Dana/ShopeePay), dan COD untuk wilayah tertentu.")
	faqQ(pdf, "Bagaimana hubungi CS?", "Klik tombol WhatsApp melayang di pojok kanan bawah situs, atau lihat nomor WA di footer.")

	h1(pdf, "9.", "Butuh Bantuan?")
	p(pdf, "Tim Customer Service kami siap membantu Anda Senin-Sabtu, jam 09:00-18:00 WIB.")
	li(pdf, "WhatsApp: tombol WA melayang di sudut kanan bawah situs.")
	li(pdf, "Email: cs@toko.mdt.biz.id")
	li(pdf, "Atau buka halaman Bantuan -> FAQ di footer situs.")
}

// ─────────────────────────────────────────────────────────────
// RESELLER
// ─────────────────────────────────────────────────────────────
func buildReseller(pdf *gofpdf.Fpdf) {
	h1(pdf, "1.", "Tentang Program Reseller")
	p(pdf, "Program "+brandName+" Reseller adalah kemitraan B2B yang memberikan akses harga grosir, MOQ rendah, dan opsi Term of Payment (TOP) untuk usaha Anda — toko offline, online seller, atau agen.")
	h2(pdf, "Keuntungan Reseller")
	li(pdf, "Diskon hingga 35% dari harga retail sesuai tier.")
	li(pdf, "Akses katalog reseller dengan harga grosir.")
	li(pdf, "Bulk order via input SKU atau upload CSV.")
	li(pdf, "Generate Purchase Order (PO) PDF otomatis untuk pembayaran tempo.")
	li(pdf, "Statement bulanan untuk rekap transaksi Anda.")
	li(pdf, "Channel CS prioritas untuk reseller.")

	h1(pdf, "2.", "Tier & Benefit")
	h2(pdf, "Bronze (entry level)")
	li(pdf, "Diskon 15% dari harga retail.")
	li(pdf, "MOQ 10 pcs / Rp 500.000 per order.")
	li(pdf, "Pembayaran prepaid (bayar di muka).")
	h2(pdf, "Silver")
	li(pdf, "Diskon 25% dari harga retail.")
	li(pdf, "MOQ 50 pcs / Rp 2.500.000 per order.")
	li(pdf, "Term of Payment 7 hari.")
	h2(pdf, "Gold (top tier)")
	li(pdf, "Diskon 35% dari harga retail.")
	li(pdf, "MOQ 200 pcs / Rp 10.000.000 per order.")
	li(pdf, "Term of Payment 14 hari.")
	li(pdf, "Akses awal produk baru sebelum dirilis publik.")
	note(pdf, "info", "Tier ditetapkan oleh tim kami berdasarkan track record order Anda. Mulai dari Bronze, Anda bisa naik ke Silver/Gold setelah memenuhi minimum volume order 3 bulan.")

	h1(pdf, "3.", "Cara Mendaftar")
	step(pdf, 1, "Buka halaman pendaftaran", "Footer -> Login Reseller, atau langsung /reseller/register.")
	step(pdf, 2, "Isi formulir", "Nama lengkap, email, nomor HP/WA, nama toko/usaha, alamat usaha, dan upload KTP. Jika berbadan usaha, upload juga NPWP.")
	step(pdf, 3, "Tunggu approval", "Tim kami akan verifikasi dalam 1x24 jam kerja. Setelah disetujui, Anda akan menerima email aktivasi.")
	step(pdf, 4, "Login portal", "Login di /reseller/login dengan email & password yang sudah dibuat.")
	note(pdf, "warn", "Pendaftaran ulang menggunakan email yang sama tidak diperbolehkan. Pastikan data benar saat mendaftar.")

	h1(pdf, "4.", "Login Portal Reseller")
	li(pdf, "URL: https://toko.mdt.biz.id/reseller/login")
	li(pdf, "Setelah login, Anda akan masuk ke Dashboard Reseller.")
	li(pdf, "Dashboard menampilkan: total pesanan, total spend bulan ini, tier aktif, pesanan terakhir, dan saldo TOP yang masih jalan.")

	h1(pdf, "5.", "Cara Order — Single SKU")
	step(pdf, 1, "Browse katalog reseller", "Menu Katalog -> Anda akan melihat harga reseller (sudah didiskon sesuai tier).")
	step(pdf, 2, "Tambah ke keranjang", "Pilih varian, atur kuantitas (perhatikan MOQ tier Anda), klik + Keranjang.")
	step(pdf, 3, "Checkout", "Sama seperti checkout B2C, tapi Anda dapat memilih metode pembayaran: Bayar Sekarang (prepaid) atau Term of Payment (jika tier Anda mendukung).")

	h1(pdf, "6.", "Bulk Order via SKU / CSV")
	p(pdf, "Untuk order banyak SKU sekaligus (rekomendasi untuk Silver+), gunakan menu Bulk Order:")
	li(pdf, "Input SKU manual: ketik daftar SKU + jumlah, satu baris per SKU.")
	li(pdf, "Upload CSV: download template CSV, isi (kolom SKU, qty), upload kembali.")
	li(pdf, "Sistem akan validasi SKU dan harga otomatis sebelum Anda checkout.")
	note(pdf, "tip", "CSV format: header 'sku,qty'. Contoh baris: HYDRAVITCSERUM-001,12.")

	h1(pdf, "7.", "Term of Payment (TOP) & PO")
	step(pdf, 1, "Pilih TOP saat checkout", "Untuk tier Silver/Gold: di langkah Pembayaran pilih Bayar Tempo (TOP X hari).")
	step(pdf, 2, "Generate Purchase Order PDF", "Setelah submit, sistem akan generate file PO PDF otomatis. Anda dapat download dari halaman pesanan.")
	step(pdf, 3, "Pengiriman", "Pesanan langsung diproses & dikirim. Bukan menunggu pembayaran.")
	step(pdf, 4, "Bayar dalam jatuh tempo", "Lakukan transfer ke rekening yang tertera di PO sebelum tanggal jatuh tempo.")
	note(pdf, "warn", "Keterlambatan pembayaran TOP akan menonaktifkan akses tier untuk order berikutnya. Penagihan otomatis dilakukan H+1 jatuh tempo.")

	h1(pdf, "8.", "Statement Bulanan")
	li(pdf, "Menu Akun -> Statement: lihat rekap order, pembayaran, dan saldo TOP.")
	li(pdf, "Statement bulanan dikirim otomatis ke email Anda setiap tanggal 5.")
	li(pdf, "Bisa download dalam format PDF & CSV untuk akuntansi internal.")

	h1(pdf, "9.", "FAQ Reseller")
	faqQ(pdf, "Apakah harga reseller bisa dilihat tanpa login?", "Tidak. Harga reseller hanya muncul setelah Anda login dengan akun reseller yang sudah aktif.")
	faqQ(pdf, "Bisakah saya naik tier?", "Bisa. Setelah 3 bulan dengan total order memenuhi syarat tier yang lebih tinggi, tim kami akan upgrade tier Anda otomatis.")
	faqQ(pdf, "Apa yang terjadi jika saya telat bayar TOP?", "Akses tier TOP Anda dinonaktifkan sementara. Order berikutnya harus prepaid sampai tunggakan dilunasi + denda 1% per minggu.")
	faqQ(pdf, "Apakah ada minimum first order?", "Tidak ada minimum first order untuk Bronze. Untuk Silver/Gold, ditetapkan sesuai MOQ tier.")
	faqQ(pdf, "Bagaimana jika produk yang saya order kosong?", "Sistem akan menahan order, atau Anda bisa pilih substitusi. Tim CS reseller akan kontak Anda jika perlu konfirmasi.")

	h1(pdf, "10.", "Kontak Reseller")
	p(pdf, "Tim Reseller "+brandName+" siap membantu Senin-Sabtu, 09:00-17:00 WIB.")
	li(pdf, "WhatsApp Reseller (prioritas): nomor terpisah, akan dikirim setelah approval.")
	li(pdf, "Email: reseller@toko.mdt.biz.id")
	li(pdf, "Lihat dashboard reseller untuk pengumuman & promo eksklusif.")
}

// ─────────────────────────────────────────────────────────────
// ADMIN
// ─────────────────────────────────────────────────────────────
func buildAdmin(pdf *gofpdf.Fpdf) {
	h1(pdf, "1.", "Login Admin & Dashboard")
	p(pdf, "Akses panel admin di /admin/login. Hanya user dengan role admin atau staff yang dapat login. Setelah login, Anda akan masuk ke Dashboard yang menampilkan ringkasan: total pesanan hari ini, revenue, produk dengan stok rendah, dan order yang menunggu konfirmasi.")
	note(pdf, "warn", "Jaga kerahasiaan password admin. Gunakan password kuat (12+ karakter, kombinasi huruf besar/kecil/angka/simbol).")

	h1(pdf, "2.", "Mengelola Produk")
	h2(pdf, "Tambah Produk Baru")
	step(pdf, 1, "Buka /admin/products -> Tambah Produk", "Isi: Nama, Slug (URL), Kategori, Brand, Deskripsi singkat, Deskripsi (HTML), berat (gram), dimensi.")
	step(pdf, 2, "Tambah varian", "Setiap produk minimal 1 varian. Isi: SKU (unik), nama varian, harga base, harga compare-at (untuk diskon), stok, dan upload foto.")
	step(pdf, 3, "Set status", "Active = tampil di storefront. Draft = tersimpan, tidak tampil. Archived = arsip.")
	step(pdf, 4, "Optimasi SEO", "Isi SEO Title, Meta Description, Focus Keyword, dan upload OG Image. Tambahkan FAQ untuk Schema.org & AI Overview.")

	h2(pdf, "Mengedit Produk")
	li(pdf, "Buka /admin/products -> klik produk -> Edit. Semua field dapat di-update kapan saja.")
	li(pdf, "Tab Gambar: drag-drop untuk reorder. Tandai 1 sebagai Primary.")
	li(pdf, "Tab Varian: edit harga, stok, status aktif. Hapus varian akan menghapus inventory & price-nya.")
	li(pdf, "Tab Inventory: lihat & ubah stok manual. Riwayat perubahan stok ada di audit log.")

	h2(pdf, "Kategori & Brand")
	li(pdf, "/admin/categories: kelola hierarchy kategori (max 2 level), urutan tampil, deskripsi, SEO.")
	li(pdf, "/admin/brands: kelola brand, logo, deskripsi.")

	h1(pdf, "3.", "Mengelola Pesanan")
	step(pdf, 1, "Buka /admin/orders", "Lihat semua pesanan. Filter berdasarkan status, tanggal, kanal (B2C/Reseller).")
	step(pdf, 2, "Update status pesanan", "Klik pesanan -> ubah status: Pending Bayar -> Paid -> Processing -> Shipped (input no resi) -> Delivered.")
	step(pdf, 3, "Cetak label & invoice", "Tombol Print Label (4x6 inci untuk thermal printer) & Print Invoice (A4).")
	step(pdf, 4, "Refund / cancel", "Untuk order yang sudah dibayar, klik Refund (akan trigger reverse di Xendit). Untuk pending, klik Cancel.")
	note(pdf, "tip", "Status update otomatis kirim email ke pembeli (jika SMTP dikonfigurasi). Status Shipped + nomor resi otomatis kirim notifikasi.")

	h1(pdf, "4.", "Mengelola Pelanggan")
	li(pdf, "/admin/customers: lihat semua pelanggan B2C, riwayat order, total spend, alamat tersimpan.")
	li(pdf, "Bisa nonaktifkan akun yang spam/fraud.")
	li(pdf, "Reset password manual jika pelanggan minta bantuan.")

	h1(pdf, "5.", "Mengelola Reseller")
	step(pdf, 1, "Approval reseller baru", "/admin/resellers -> tab Pending. Klik aplikasi -> review KTP & data -> Approve / Reject.")
	step(pdf, 2, "Set tier reseller", "Klik reseller -> Edit -> set tier (Bronze/Silver/Gold). Tier menentukan diskon, MOQ, dan TOP.")
	step(pdf, 3, "Kelola TOP & statement", "Tab Statement: lihat saldo terutang, ngerek pembayaran, kirim reminder.")
	note(pdf, "info", "Reseller yang telat bayar TOP otomatis ditandai oleh sistem. Cron job background mark reseller overdue setiap H+1 jatuh tempo.")

	h1(pdf, "6.", "Voucher & Diskon")
	li(pdf, "/admin/vouchers: buat kode diskon (% atau nominal), set min spend, max use, tanggal aktif.")
	li(pdf, "Tipe: GENERAL (semua user) / B2C-only / RESELLER-only / FIRST-ORDER.")
	li(pdf, "Voucher digunakan saat checkout di kolom Kode Voucher.")

	h1(pdf, "7.", "Blog & Halaman Statis")
	li(pdf, "/admin/blog: buat artikel (judul, isi, kategori, tag, thumbnail, SEO).")
	li(pdf, "/admin/pages: edit halaman statis (Tentang Kami, Privacy, Syarat & Ketentuan, FAQ).")
	li(pdf, "Editor mendukung HTML + image upload (otomatis di-resize ke 1600px max).")

	h1(pdf, "8.", "Settings")
	h2(pdf, "Tab Toko")
	li(pdf, "Nama toko, tagline, logo, favicon, alamat, nomor WhatsApp, email kontak.")
	li(pdf, "Origin Area ID & Postal Code: titik pengiriman gudang Anda (untuk hitung ongkir).")
	h2(pdf, "Tab Pembayaran (Xendit)")
	li(pdf, "Secret Key & Webhook Token dari dashboard Xendit.")
	li(pdf, "Webhook URL: https://toko.mdt.biz.id/webhooks/xendit (set di Xendit dashboard).")
	li(pdf, "Method enabled: VA, EWALLET, QRIS, CARD, RETAIL.")
	h2(pdf, "Tab Pengiriman (Biteship)")
	li(pdf, "API Key dari Biteship. Jika kosong, sistem fallback ke tabel ongkir static (gratis).")
	li(pdf, "Pilih kurir: jne, jnt, sicepat, anteraja, gojek, grab.")
	h2(pdf, "Tab Email/SMTP")
	li(pdf, "SMTP host/port/user/pass. Untuk produksi: gunakan local Postfix dengan DKIM/SPF/DMARC dikonfigurasi (lihat README).")
	h2(pdf, "Tab Marketing")
	li(pdf, "Meta Pixel ID + CAPI Token. GA4 ID + API Secret. GTM ID. TikTok Pixel ID. Google Ads conversion.")
	h2(pdf, "Tab GMC")
	li(pdf, "Merchant ID Google Merchant Center. Feed otomatis di /feeds/gmc.xml — submit URL ini ke GMC.")
	li(pdf, "Auto-disable produk OOS untuk feed GMC (compliance).")

	h1(pdf, "9.", "SEO & Sitemap")
	li(pdf, "Sitemap dinamis di /sitemap.xml — auto-update saat ada produk baru.")
	li(pdf, "Robots.txt editable di /admin/seo.")
	li(pdf, "Schema.org markup otomatis: Product, Offer, BreadcrumbList, FAQPage, Organization, WebSite.")
	li(pdf, "Optimasi AI Overview: aktifkan toggle di Settings -> SEO Global.")
	li(pdf, "Check sitemap submitted di Google Search Console & Bing Webmaster Tools.")

	h1(pdf, "10.", "Backup & Maintenance")
	li(pdf, "Backup database harian: gunakan pg_dump terjadwal via cron Linux.")
	li(pdf, "Backup folder uploads/: rsync ke object storage (S3/R2) atau VPS lain.")
	li(pdf, "Update aplikasi: pull dari git repo, regen templ, build binary, restart systemd service.")
	li(pdf, "Monitor log: journalctl -u tokoonline.service -f.")
	li(pdf, "Cronjob abandoned cart reminder & TOP overdue marker sudah otomatis berjalan.")
	note(pdf, "warn", "Sebelum update produksi, lakukan deploy ke staging server dulu jika tersedia. Test fitur kritis (checkout, payment webhook).")

	h1(pdf, "11.", "Troubleshooting Singkat")
	faqQ(pdf, "Order masuk tapi status masih pending bayar?", "Cek webhook Xendit. Pastikan webhook token cocok & URL benar. Tail log untuk error 4xx.")
	faqQ(pdf, "Email transaksional tidak terkirim?", "Cek SMTP setting. Untuk Postfix loopback, pastikan Postfix running & DKIM tersedia. Lihat /var/log/mail.log.")
	faqQ(pdf, "Ongkir tidak muncul?", "Cek Biteship API key & balance, atau pastikan origin Area ID terisi. Tabel static fallback otomatis aktif jika API key kosong.")
	faqQ(pdf, "GMC feed tidak update?", "Feed di-cache 1 jam. Force regen via /admin/gmc/regen. Pastikan produk active & GMC enabled.")
	faqQ(pdf, "Reseller tidak bisa login?", "Cek status: pending (belum approve), suspended (manual), atau active. Reset password jika perlu.")
}
