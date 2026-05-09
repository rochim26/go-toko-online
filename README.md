# Go Toko Online

E-commerce multi-channel (B2C + B2B reseller) yang SEO-friendly, terintegrasi
Google Merchant Center, Meta Pixel + Conversions API, GA4 (client + server-side),
Xendit payment, Biteship shipping, dan PWA — semuanya dalam satu binary Go
berbasis Templ + HTMX + PostgreSQL.

Live demo: https://toko.mdt.biz.id

## Highlight Fitur

- **Dua channel dalam satu codebase**:
  - **B2C** publik dengan harga retail, checkout Xendit, ongkir Biteship.
  - **B2B Portal Reseller** dengan tier (diskon %, MOQ, Term-of-Payment / kredit),
    bulk order via SKU/CSV, generator PO PDF, statement bulanan.
- **SEO penuh**: SSR (zero-JS first paint), Schema.org `Product`/`Offer`/
  `BreadcrumbList`/`FAQPage`/`Organization`/`WebSite`, sitemap dinamis, robots
  editable, FAQ blok per produk yang dioptimalkan untuk **Google AI Overview**.
- **Tracking server-side**: Meta Pixel + **Conversions API** (deduplikasi
  via `event_id`), GA4 client + **Measurement Protocol** server-side. Tahan
  iOS 14+ dan adblock.
- **Google Merchant Center**: feed XML otomatis di `/feeds/gmc.xml` (B2C-only,
  auto-skip OOS, GTIN/MPN/brand support, kategori GMC mapping).
- **Email transaksional** via Postfix loopback + DKIM/SPF/DMARC built-in.
  Order confirmation, payment receipt, shipping notification, reseller
  approval, abandoned cart reminder — semua otomatis.
- **PWA**: manifest + service worker offline-shell.
- **Admin panel**: produk, varian, multi-foto upload, kategori, voucher, blog,
  redirect, kelola reseller (approval/tier), settings tab-tab untuk semua API
  keys, ganti password, GMC feed status, sitemap regen.
- **Image optimization**: upload otomatis di-resize ke 1600px max + kompres
  JPEG q=82 (file 20MB dari HP → ~200KB).
- **Background jobs**: abandoned cart reminder (cron), TOP overdue marker.

## Tech Stack

| Komponen | Pilihan |
|---|---|
| Bahasa | Go 1.24+ |
| Template | [Templ](https://github.com/a-h/templ) (type-safe SSR) |
| Interaksi | HTMX — semua halaman tetap server-rendered untuk SEO |
| Database | PostgreSQL 16 (FTS, JSONB, citext, unaccent) |
| Router | go-chi |
| Session | scs/v2 + postgresstore |
| CSRF | nosurf |
| Password | argon2id |
| Email | Postfix loopback (DKIM via opendkim) |
| Payment | Xendit Invoice API |
| Shipping | Biteship API |

## Struktur

```
.
├── cmd/
│   ├── server/      # HTTP server (main binary)
│   ├── migrate/     # SQL migration runner
│   └── gen-icons/   # PWA icon generator (run sekali sebelum deploy)
├── internal/
│   ├── app/         # routes wiring + DI
│   ├── config/      # env loader
│   ├── db/          # pgx pool
│   ├── models/      # plain types
│   ├── middleware/  # auth + security headers
│   ├── handlers/    # HTTP handlers (public/admin/reseller/api)
│   ├── services/    # domain (catalog, cart, order, pricing, payment,
│   │                #         shipping, mailer, gmc, seo, tracking,
│   │                #         reseller, security, settings, cron, pdf,
│   │                #         imageopt, auth)
│   ├── views/       # templ files per area
│   └── httpx/       # render helpers + format (IDR, dates)
├── migrations/      # 0001_init.sql + 0002_seed.sql
├── static/          # css, js, img, manifest, sw
├── deploy/          # nginx.conf, systemd unit
└── README.md
```

## Quickstart Lokal

```bash
# 1. dep
go mod download
go install github.com/a-h/templ/cmd/templ@latest

# 2. Postgres lokal
createdb tokoonline
createuser -s tokoonline

# 3. .env
cp .env.example .env
# isi APP_SECRET, DATABASE_URL, ADMIN_BOOTSTRAP_*

# 4. migrate + seed
go run ./cmd/migrate

# 5. generate templ + run
templ generate
go run ./cmd/gen-icons static/img       # sekali saja, generate favicon dll.
go run ./cmd/server
```

Buka http://localhost:8100. Login admin pakai `ADMIN_BOOTSTRAP_*` dari `.env`.

## Production Deploy

Lihat `deploy/`:

- `deploy/nginx.conf` — vhost dengan TLS Let's Encrypt + Cloudflare real-IP
- `deploy/tokoonline.service` — systemd unit

Build linux binary:
```bash
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/server ./cmd/server
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/migrate ./cmd/migrate
```

SCP `bin/`, `static/`, `migrations/`, `.env` ke `/opt/tokoonline/`. Lalu:
```bash
systemctl enable --now tokoonline.service
```

Buat database & user di Postgres:
```sql
CREATE ROLE tokoonline LOGIN PASSWORD 'xxx';
CREATE DATABASE tokoonline OWNER tokoonline;
```

Jalankan migrations:
```bash
DATABASE_URL=... ./bin/migrate
```

### Email deliverability (DKIM/SPF/DMARC)

Untuk email transaksional sampai inbox Gmail/Outlook:

1. Install `opendkim` + `opendkim-tools`.
2. Generate key: `opendkim-genkey -b 2048 -d yourdomain.com -s mail`.
3. Tambah DNS records:
   - `mail._domainkey.yourdomain.com` TXT — public DKIM key dari `mail.txt`
   - `yourdomain.com` TXT — `v=spf1 ip4:YOUR_IP ~all`
   - `_dmarc.yourdomain.com` TXT — `v=DMARC1; p=quarantine; rua=mailto:you@email.com`
4. Hook ke postfix: `smtpd_milters=inet:127.0.0.1:8891`.
5. Force IPv4 outbound: `smtp_address_preference=ipv4` (kalau SPF ip4-only).

App mengirim via loopback `127.0.0.1:25` tanpa SASL. Lihat `internal/services/mailer/mailer.go`.

## Settings yang harus diisi sebelum live

Login `/admin/login` → `/admin/settings`:

- **Toko**: nama, WA number, alamat, **Origin Area ID & Postal Code (Biteship)**
- **Xendit**: secret key + webhook token. Webhook URL: `/webhooks/xendit`
- **Biteship**: API key, daftar kurir
- **Marketing**: Meta Pixel ID + CAPI token, GA4 ID + API secret, GTM ID,
  TikTok Pixel
- **GMC**: merchant ID. Feed otomatis di `/feeds/gmc.xml` siap submit
- **Email**: kosongkan SMTP host untuk pakai postfix loopback (default)

## Memo Arsitektur

- **Single-tenant**: satu instalasi = satu brand. Skema mendukung multi-brand
  via tabel `brands`, tapi store identity (logo, dll.) tunggal di `settings`.
- **Pricing audience**: tabel `product_prices(audience, price)` di mana
  audience = `"b2c"` atau UUID tier reseller. Resolusi harga di
  `internal/services/pricing` cek tabel dulu, fallback ke
  `base_price * (1 - tier.discount_pct/100)`.
- **Cart persistent**: di-DB untuk anonim (via session token) dan user.
  Auto-merge saat login.
- **Tracking dedup**: `event_id` sama dipakai client (Pixel/GA4) dan server
  (CAPI/MP) sehingga Meta & Google deduplikasi otomatis.
- **CSRF**: nosurf — request POST butuh `csrf_token` form field + Referer
  header (browser otomatis).

## Lisensi

MIT — silakan dipakai dan dimodifikasi.

## Kontribusi

PR welcome. Silakan run:
```bash
templ generate
go build ./...
go vet ./...
```
sebelum push.
