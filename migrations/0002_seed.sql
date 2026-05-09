-- Seed: default settings, default tiers, sample brand/category, sample product

INSERT INTO settings(key, value, description) VALUES
  ('store', '{"name":"Toko MDT","tagline":"Belanja online cepat & terpercaya","logo_url":"","favicon_url":"","email":"hello@toko.mdt.biz.id","phone":"+62","wa_number":"+62","address":"","origin_area_id":"","origin_postal_code":""}', 'Identitas toko'),
  ('seo_global', '{"title_pattern":"%s | Toko MDT","default_title":"Toko MDT - Belanja Online","default_desc":"Belanja online produk berkualitas dengan pengiriman cepat ke seluruh Indonesia.","default_og_image":"","robots_extra":"","gsc_verification":"","bing_verification":"","ai_overview_optimized":true}', 'SEO Global'),
  ('marketing', '{"meta_pixel_id":"","meta_capi_token":"","meta_test_event_code":"","ga4_id":"","ga4_api_secret":"","gtm_id":"","tiktok_pixel_id":"","google_ads_id":"","google_ads_label":""}', 'Pixel & GA4'),
  ('gmc', '{"merchant_id":"","feed_enabled":true,"feed_format":"xml","auto_disable_oos":true,"shipping_country":"ID","content_language":"id","target_country":"ID"}', 'Google Merchant Center'),
  ('xendit', '{"secret_key":"","webhook_token":"","public_key":"","methods_enabled":["VA","EWALLET","QRIS","CARD","RETAIL"],"success_redirect":"/order/success","failure_redirect":"/order/failed"}', 'Xendit'),
  ('biteship', '{"api_key":"","origin_area_id":"","origin_postal_code":"","couriers":["jne","jnt","sicepat","anteraja","gojek","grab"]}', 'Biteship'),
  ('shipping', '{"free_shipping_threshold":0,"flat_rate_fallback":15000}', 'Pengiriman'),
  ('mailer', '{"smtp_host":"","smtp_port":587,"smtp_user":"","smtp_pass":"","from_email":"","from_name":"Toko MDT"}', 'Email/SMTP'),
  ('tax', '{"ppn_pct":0,"invoice_prefix":"INV","invoice_npwp":""}', 'Pajak & Faktur'),
  ('reseller', '{"registration_open":true,"auto_approve":false,"require_npwp":false,"require_ktp":true,"min_first_order":0}', 'Reseller setting')
ON CONFLICT (key) DO NOTHING;

INSERT INTO reseller_tiers(code, name, discount_pct, moq_qty, moq_value, top_days, sort_order) VALUES
  ('BRONZE','Bronze',15.00, 10,  500000, 0, 1),
  ('SILVER','Silver',25.00, 50, 2500000, 7, 2),
  ('GOLD',  'Gold',  35.00,200,10000000,14, 3)
ON CONFLICT (code) DO NOTHING;

INSERT INTO brands(name, slug, description) VALUES
  ('MDT','mdt','Brand utama Toko MDT')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO categories(name, slug, description, sort_order, is_active, seo_title, seo_desc) VALUES
  ('Semua Produk','semua','Daftar semua produk',1,true,'Semua Produk','Daftar semua produk yang tersedia di Toko MDT')
ON CONFLICT (slug) DO NOTHING;

-- A demo product so the storefront has something to show on first deploy
WITH b AS (SELECT id FROM brands WHERE slug='mdt' LIMIT 1),
     c AS (SELECT id FROM categories WHERE slug='semua' LIMIT 1),
     p AS (
       INSERT INTO products(slug,name,brand_id,category_id,short_desc,description,status,published_at,seo_title,seo_desc,focus_keyword,gmc_brand,faqs)
       VALUES('produk-demo','Produk Demo',(SELECT id FROM b),(SELECT id FROM c),
              'Produk contoh untuk menguji storefront.',
              '<p>Ini adalah produk demo. Anda dapat mengubah atau menghapusnya dari halaman admin.</p>',
              'active', now(),
              'Produk Demo - Toko MDT','Produk contoh untuk menguji storefront Toko MDT.','produk demo',
              'MDT',
              '[{"q":"Apakah produk ini original?","a":"Ya, semua produk yang dijual di Toko MDT adalah original."},{"q":"Berapa lama pengiriman?","a":"Pengiriman umumnya 1-3 hari kerja untuk wilayah Jabodetabek."}]'::jsonb)
       ON CONFLICT (slug) DO NOTHING
       RETURNING id
     ),
     v AS (
       INSERT INTO product_variants(product_id, sku, name, base_price, weight_grams, is_default)
       SELECT id, 'DEMO-001','Default', 100000, 500, true FROM p
       ON CONFLICT (sku) DO NOTHING
       RETURNING id
     )
INSERT INTO inventory_levels(variant_id, on_hand)
SELECT id, 100 FROM v ON CONFLICT (variant_id) DO NOTHING;

-- Default sale prices for tiers
INSERT INTO product_prices(variant_id, audience, price)
SELECT v.id, 'b2c', v.base_price
FROM product_variants v
WHERE NOT EXISTS (SELECT 1 FROM product_prices p WHERE p.variant_id = v.id AND p.audience='b2c');

-- About & Privacy pages
INSERT INTO pages(slug,title,body_html,seo_title,seo_desc) VALUES
  ('tentang-kami','Tentang Kami','<p>Selamat datang di Toko MDT. Kami menjual produk berkualitas dengan pengiriman ke seluruh Indonesia.</p>','Tentang Kami','Tentang Toko MDT'),
  ('kebijakan-privasi','Kebijakan Privasi','<p>Kami menghormati privasi Anda. Data pribadi disimpan dengan aman dan tidak dibagikan ke pihak ketiga tanpa izin.</p>','Kebijakan Privasi','Kebijakan privasi Toko MDT'),
  ('syarat-ketentuan','Syarat & Ketentuan','<p>Dengan menggunakan layanan kami, Anda menyetujui syarat & ketentuan yang berlaku.</p>','Syarat & Ketentuan','Syarat dan ketentuan Toko MDT'),
  ('faq','FAQ','<h2>Pertanyaan Umum</h2><p>Bagaimana cara pesan? Pilih produk, klik beli, isi alamat, lalu bayar.</p>','FAQ','Pertanyaan umum Toko MDT')
ON CONFLICT (slug) DO NOTHING;
