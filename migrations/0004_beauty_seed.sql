-- 0004_beauty_seed: dummy data brand kecantikan agar storefront terlihat aktif.
-- Idempotent: pakai ON CONFLICT DO NOTHING di mana memungkinkan.
-- Jalankan setelah 0001-0003. Aman dijalankan ulang.

-- ─────────────────────────────────────────────────────────────
-- Branding store
-- ─────────────────────────────────────────────────────────────
UPDATE settings
SET value = jsonb_set(
        jsonb_set(
            jsonb_set(value, '{name}', '"GlowMart Beauty"'::jsonb),
            '{tagline}', '"Skincare & makeup pilihan, original 100% — kirim seluruh Indonesia."'::jsonb
        ),
        '{address}', '"Jl. Pemuda Raya No. 12, Rawamangun, Jakarta Timur 13220"'::jsonb
    ),
    updated_at = now()
WHERE key = 'store';

-- ─────────────────────────────────────────────────────────────
-- Brands
-- ─────────────────────────────────────────────────────────────
INSERT INTO brands(name, slug, description) VALUES
  ('GlowMe',         'glowme',         'Skincare ringan untuk kulit sensitif, formulasi gentle dengan bahan aktif teruji.'),
  ('DermaLab',       'dermalab',       'Perawatan kulit dermatologically-tested untuk masalah jerawat & penuaan.'),
  ('PureBeauty',     'purebeauty',     'Natural & vegan beauty — bebas paraben, bebas sulfat.'),
  ('AuraBalm',       'aurabalm',       'Body care & fragrance dengan minyak esensial pilihan.'),
  ('LumiCosmetics',  'lumi',           'Makeup berdaya tahan tinggi, pigmentasi pekat untuk look profesional.')
ON CONFLICT (slug) DO NOTHING;

-- ─────────────────────────────────────────────────────────────
-- Categories
-- ─────────────────────────────────────────────────────────────
INSERT INTO categories(name, slug, description, sort_order, is_active, seo_title, seo_desc) VALUES
  ('Skincare',            'skincare',   'Perawatan wajah harian: cleanser, toner, serum, moisturizer, sunscreen.', 2, true, 'Skincare', 'Produk skincare original di GlowMart Beauty'),
  ('Makeup',              'makeup',     'Make up & color cosmetics: foundation, lipstick, mascara, palette.',     3, true, 'Makeup',   'Makeup berkualitas, pigmentasi pekat'),
  ('Body Care',           'body-care',  'Body lotion, scrub, lulur, sabun mandi.',                                  4, true, 'Body Care','Body care natural & menutrisi kulit'),
  ('Hair Care',           'hair-care',  'Shampoo, conditioner, serum & masker rambut.',                             5, true, 'Hair Care','Perawatan rambut sehat & berkilau'),
  ('Fragrance',           'fragrance',  'Parfum, body mist & cologne ringan.',                                      6, true, 'Fragrance','Wewangian pilihan untuk setiap mood'),
  ('Tools & Accessories', 'tools',      'Brush, sponge, applicator & accessories.',                                 7, true, 'Tools',    'Beauty tools & accessories')
ON CONFLICT (slug) DO NOTHING;

-- ─────────────────────────────────────────────────────────────
-- Products  (slug, name, brand_slug, cat_slug, short_desc, description, weight_grams, gtin)
-- ─────────────────────────────────────────────────────────────
INSERT INTO products(slug, name, brand_id, category_id, short_desc, description, status, published_at, weight_grams, seo_title, seo_desc, focus_keyword, gmc_brand, gmc_gtin, faqs, is_b2c, is_b2b)
SELECT
  v.slug, v.name, b.id, c.id, v.short_desc, v.description, 'active', now() - (random()*30 || ' days')::interval,
  v.weight, v.name || ' - GlowMart Beauty', v.short_desc, v.focus_kw, b.name, v.gtin,
  v.faqs::jsonb, true, true
FROM (VALUES
  -- Skincare — GlowMe
  ('hydra-vit-c-serum',     'Hydra Boost Vitamin C Serum 30ml',                'glowme',     'skincare',  'Serum vitamin C 10% mencerahkan, melembabkan, & meratakan warna kulit.', '<p>Serum dengan 10% Vitamin C stabil (Sodium Ascorbyl Phosphate) yang membantu mencerahkan, meratakan warna kulit, dan menyamarkan flek hitam.</p><p>Cocok untuk semua jenis kulit. Pakai pagi sebelum sunscreen, atau malam sebelum moisturizer.</p><ul><li>10% Vitamin C stabil</li><li>Hyaluronic Acid 1%</li><li>Niacinamide 2%</li><li>Bebas alkohol & parfum</li></ul>', 80,  'serum vitamin c',           '8991234500001', '[{"q":"Cocok untuk kulit sensitif?","a":"Ya, formulasi gentle tanpa alkohol dan parfum, aman untuk kulit sensitif."},{"q":"Berapa kali sehari pemakaian?","a":"1-2 kali sehari, pagi dan/atau malam setelah toner dan sebelum moisturizer."}]'),
  ('niacinamide-toner',      'Niacinamide Brightening Toner 100ml',             'glowme',     'skincare',  'Toner 5% niacinamide — mengontrol minyak, meratakan warna kulit, & mengecilkan pori.',  '<p>Toner ringan dengan 5% niacinamide & zinc PCA untuk membantu mengontrol produksi minyak, meratakan warna kulit, dan mengecilkan tampilan pori.</p>', 130, 'toner niacinamide',          '8991234500002', '[{"q":"Apakah bisa digunakan dengan vitamin C?","a":"Bisa, gunakan toner ini lebih dulu, lalu serum vitamin C."}]'),
  ('daily-sunscreen-spf50',  'Daily Sunscreen SPF 50+ PA++++ 50ml',             'glowme',     'skincare',  'Sunscreen ringan, no white cast, cocok untuk daily makeup base.',                       '<p>Perlindungan UVA + UVB dengan SPF 50+ PA++++. Tekstur ringan, cepat menyerap, no white cast — sempurna sebagai base makeup.</p>', 80,  'sunscreen wajah',            '8991234500003', '[{"q":"Apakah membuat kulit putih kebiruan (white cast)?","a":"Tidak, formula no white cast aman untuk semua warna kulit."}]'),
  ('gentle-cleanser',        'Gentle Cleanser pH 5.5 100ml',                    'glowme',     'skincare',  'Sabun cuci muka pH balanced, lembut tanpa membuat kulit kering.',                        '<p>Pembersih wajah dengan pH 5.5 yang ramah kulit. Membersihkan minyak & kotoran tanpa membuat kulit terasa ketarik.</p>',  130, 'sabun cuci muka',            '8991234500004', '[]'),
  ('sheet-mask-hydrating',   'Sheet Mask Hydrating (5pcs) 25ml',                'glowme',     'skincare',  'Masker lembar 5x untuk pelembab intensif harian.',                                       '<p>Masker lembar dengan kandungan hyaluronic acid, ceramide, dan ekstrak bunga lavender. Pakai 1x seminggu untuk hidrasi optimal.</p>',                       150, 'sheet mask',                '8991234500005', '[]'),

  -- Skincare — DermaLab
  ('retinol-night-serum',    'Retinol 0.3% Night Serum 15ml',                   'dermalab',   'skincare',  'Anti-aging serum retinol untuk meremajakan kulit & menghaluskan tekstur.',                '<p>Serum malam dengan 0.3% encapsulated retinol untuk membantu mengurangi tampilan garis halus, kerutan, dan tekstur kulit kasar.</p>', 60,  'retinol serum',             '8991234500006', '[{"q":"Apakah aman untuk pemula retinol?","a":"Mulai dengan pemakaian 2x seminggu di malam hari. Tingkatkan secara bertahap. Wajib pakai sunscreen di pagi hari."}]'),
  ('ceramide-repair-cream',  'Ceramide Repair Cream 50g',                       'dermalab',   'skincare',  'Krim malam pelembab dengan ceramide untuk skin barrier sehat.',                          '<p>Krim pelembab kaya nutrisi dengan 5 jenis ceramide untuk memperbaiki dan memperkuat skin barrier yang rusak akibat over-exfoliation atau kulit kering.</p>', 100, 'ceramide cream',           '8991234500007', '[]'),
  ('salicylic-toner',        'Salicylic Acid 2% Toner 100ml',                   'dermalab',   'skincare',  'BHA toner 2% untuk membersihkan pori dari dalam, kulit acne-prone.',                     '<p>BHA exfoliating toner dengan 2% salicylic acid yang mampu menembus pori dan membantu membersihkan komedo, jerawat, serta minyak berlebih.</p>',          130, 'bha toner salicylic',       '8991234500008', '[]'),
  ('aha-bha-serum',          'AHA BHA Exfoliating Serum 30ml',                  'dermalab',   'skincare',  'Eksfoliasi kimia ringan untuk kulit cerah & halus.',                                    '<p>Kombinasi 5% AHA (glycolic) + 2% BHA (salicylic) untuk mempercepat regenerasi sel kulit. Hasilkan kulit lebih halus, cerah, dan merata.</p>',                  80,  'aha bha serum',             '8991234500009', '[]'),

  -- Skincare — PureBeauty
  ('aloe-soothing-gel',      'Aloe Vera Soothing Gel 200ml',                    'purebeauty', 'skincare',  'Gel aloe vera 99% — multifungsi untuk wajah, tubuh, & rambut.',                          '<p>Gel aloe vera 99% murni tanpa pewangi tambahan. Cocok untuk after-sun, masker tidur, atau pelembab tubuh.</p>',                                          230, 'aloe vera gel',            '8991234500010', '[]'),
  ('green-tea-foam',         'Green Tea Cleansing Foam 150ml',                  'purebeauty', 'skincare',  'Foam pembersih dengan ekstrak green tea — antioksidan & menyegarkan.',                  '<p>Pembersih wajah berbusa lembut dengan ekstrak green tea organik untuk membersihkan kotoran sambil memberi efek menenangkan.</p>',                            180, 'cleansing foam',           '8991234500011', '[]'),
  ('vit-e-lip-mask',         'Vitamin E Lip Mask 10g',                          'purebeauty', 'skincare',  'Masker bibir overnight untuk bibir lembut & merah alami.',                              '<p>Sleeping mask khusus bibir dengan Vitamin E, shea butter, dan minyak almond. Aplikasikan sebelum tidur untuk bibir lembab dan kenyal di pagi hari.</p>',     30,  'lip mask',                 '8991234500012', '[]'),

  -- Body Care — AuraBalm
  ('shea-body-lotion',       'Shea Butter Body Lotion 250ml',                   'aurabalm',   'body-care', 'Body lotion shea butter — melembabkan tahan lama, aroma lembut.',                       '<p>Lotion tubuh dengan shea butter Afrika asli, vitamin E, dan minyak jojoba. Memberikan kelembapan tahan lama hingga 24 jam.</p>',                              280, 'body lotion',              '8991234500013', '[]'),
  ('coconut-body-scrub',     'Coconut Body Scrub 200g',                         'aurabalm',   'body-care', 'Scrub kelapa & gula laut untuk kulit halus bercahaya.',                                  '<p>Scrub eksfoliasi tubuh dengan butiran gula laut halus, minyak kelapa, dan ekstrak vanilla. Aman untuk pemakaian 2-3x seminggu.</p>',                          230, 'body scrub kelapa',        '8991234500014', '[]'),
  ('vanilla-honey-balm',     'Vanilla Honey Lip Balm 8g',                       'aurabalm',   'body-care', 'Lip balm vanilla honey, bibir kenyal & pink alami.',                                     '<p>Balm bibir dengan beeswax alami, madu murni, dan extract vanilla. Tekstur lembut, tidak lengket.</p>',                                                       30,  'lip balm vanilla',         '8991234500015', '[]'),

  -- Fragrance — AuraBalm
  ('rose-body-mist',         'Rose Body Mist 100ml',                            'aurabalm',   'fragrance', 'Body mist mawar dengan note manis lembut, cocok harian.',                                '<p>Body mist segar dengan top notes rose petals, middle notes peony, dan base notes white musk. Tahan 4-6 jam di kulit.</p>',                                  130, 'body mist mawar',          '8991234500016', '[]'),
  ('eau-bloom-50ml',         'Eau de Parfum "Bloom" 50ml',                      'aurabalm',   'fragrance', 'Parfum floral fruity — peach, jasmine, sandalwood. Tahan 8 jam.',                       '<p>Eau de Parfum floral-fruity dengan top notes pink pepper & peach, heart of jasmine & rose, base sandalwood & cashmeran.</p>',                              160, 'parfum wanita',            '8991234500017', '[]'),
  ('eau-citrus-50ml',        'Eau de Parfum "Citrus Sunrise" 50ml',             'aurabalm',   'fragrance', 'Parfum citrus segar — bergamot, lemon, vetiver. Cocok unisex.',                          '<p>Parfum segar unisex dengan note citrus dominan: bergamot Italia, lemon Sicilia, dan vetiver Haiti. Tahan 6-8 jam.</p>',                                     160, 'parfum citrus unisex',     '8991234500018', '[]'),

  -- Hair Care — PureBeauty
  ('argan-hair-serum',       'Argan Oil Hair Serum 50ml',                       'purebeauty', 'hair-care', 'Serum rambut argan oil — anti-frizz, kilau alami.',                                      '<p>Serum rambut leave-on dengan minyak argan Maroko murni dan vitamin E. Atasi rambut kering, kusut, dan ujung bercabang.</p>',                                80,  'serum rambut argan',       '8991234500019', '[]'),
  ('anti-dandruff-shampoo',  'Anti-Dandruff Shampoo 250ml',                     'purebeauty', 'hair-care', 'Sampo anti-ketombe dengan tea tree oil & zinc.',                                         '<p>Shampoo anti-ketombe dengan tea tree oil, peppermint, dan zinc pyrithione. Membersihkan kulit kepala dan mengurangi gatal.</p>',                              280, 'shampoo ketombe',          '8991234500020', '[]'),
  ('keratin-conditioner',    'Keratin Repair Conditioner 250ml',                'purebeauty', 'hair-care', 'Conditioner keratin untuk rambut rusak & sering di-styling.',                            '<p>Hair conditioner dengan hydrolyzed keratin & argan oil yang membantu memperbaiki rambut rusak akibat pewarnaan, smoothing, atau penataan dengan panas.</p>', 280, 'conditioner keratin',      '8991234500021', '[]'),

  -- Makeup — LumiCosmetics
  ('lipstick-velvet-red',    'Matte Liquid Lipstick "Velvet Red" 5ml',          'lumi',       'makeup',    'Lipstick matte tahan 8 jam, warna merah klasik.',                                        '<p>Liquid lipstick formula matte yang nyaman, tidak mengeringkan bibir. Pigmentasi tinggi, tahan 8 jam dan transfer-proof.</p>',                                40,  'lipstick matte merah',     '8991234500022', '[]'),
  ('lipstick-nude-pink',     'Matte Liquid Lipstick "Nude Pink" 5ml',           'lumi',       'makeup',    'Lipstick matte nude pink, daily wear.',                                                 '<p>Liquid lipstick warna nude pink universal — cocok untuk semua undertone kulit Indonesia. Matte, tidak transfer.</p>',                                       40,  'lipstick nude',            '8991234500023', '[]'),
  ('foundation-light',       'Long-Wear Foundation Light Beige 30ml',           'lumi',       'makeup',    'Foundation tahan 12 jam, coverage medium-full, finish satin.',                          '<p>Foundation cair tahan lama dengan coverage medium-to-full yang bisa di-build. Finish natural satin yang flattering.</p>',                                   80,  'foundation tahan lama',    '8991234500024', '[]'),
  ('foundation-medium',      'Long-Wear Foundation Medium Tan 30ml',            'lumi',       'makeup',    'Foundation tahan 12 jam, coverage medium-full, finish satin.',                          '<p>Versi shade Medium Tan — cocok untuk kulit sawo matang Indonesia.</p>',                                                                                       80,  'foundation tahan lama',    '8991234500025', '[]'),
  ('eyeshadow-sunset',       'Eyeshadow Palette "Sunset" 12 colors',            'lumi',       'makeup',    'Palette 12 warna sunset — neutral hangat, daily-glam.',                                  '<p>12 shades warna sunset hangat dari champagne, coral, copper, hingga deep plum. Mix matte & shimmer. Bebas paraben.</p>',                                    160, 'eyeshadow palette',        '8991234500026', '[]'),
  ('volumizing-mascara',     'Volumizing Mascara Black 10ml',                   'lumi',       'makeup',    'Maskara volumizing tahan air, mata mempesona.',                                          '<p>Maskara hitam pekat dengan brush khusus untuk volume maksimal. Tahan air dan tahan luntur. Mudah dihapus dengan oil-based cleanser.</p>',                    50,  'mascara waterproof',       '8991234500027', '[]'),
  ('brow-pencil-brown',      'Brow Pencil Dark Brown',                          'lumi',       'makeup',    'Pensil alis ujung sisir, hasil natural seperti rambut alis.',                            '<p>Pensil alis dengan ujung halus seperti helai rambut + sisir spoolie di ujung. Tahan air.</p>',                                                                30,  'pensil alis',              '8991234500028', '[]'),
  ('highlighter-gold',       'Highlighter Stick Gold Glow 8g',                  'lumi',       'makeup',    'Highlighter stick warna gold, glowing instant.',                                         '<p>Highlighter stick yang mudah dibaur. Aplikasikan di tulang pipi, tulang hidung, dan cupid bow untuk efek glow alami.</p>',                                  40,  'highlighter stick',        '8991234500029', '[]'),

  -- Tools — LumiCosmetics
  ('beauty-sponge-set',      'Beauty Sponge Set 4pcs',                          'lumi',       'tools',     'Set 4 sponge berbentuk berbeda untuk foundation, concealer, & contour.',                 '<p>Set 4 beauty sponge: teardrop, flat-edge, mini, dan classic. Latex-free, dapat dipakai basah atau kering.</p>',                                              60,  'beauty sponge',            '8991234500030', '[]'),
  ('makeup-brush-set',       'Makeup Brush Set 7pcs',                           'lumi',       'tools',     'Set 7 brush profesional + pouch — face & eye brushes.',                                  '<p>Set 7 brush makeup berkualitas salon: 4 face brush (powder, foundation, blush, contour) + 3 eye brush (blending, flat, smudge). Termasuk pouch.</p>',         260, 'kuas makeup',              '8991234500031', '[]')
) AS v(slug, name, brand_slug, cat_slug, short_desc, description, weight, focus_kw, gtin, faqs)
JOIN brands b      ON b.slug = v.brand_slug
JOIN categories c  ON c.slug = v.cat_slug
ON CONFLICT (slug) DO NOTHING;

-- ─────────────────────────────────────────────────────────────
-- Variants (1 default per product)  with compare_at_price (~15% markup) so storefront tampil "DISKON"
-- ─────────────────────────────────────────────────────────────
INSERT INTO product_variants(product_id, sku, name, base_price, compare_at_price, weight_grams, is_default, is_active)
SELECT
  p.id,
  upper(replace(replace(p.slug, '-', ''), '_', '')) || '-001' AS sku,
  'Default',
  v.price,
  CASE WHEN v.price > 0 THEN ROUND(v.price * 1.15, -2) ELSE NULL END AS compare_at_price,
  COALESCE(p.weight_grams, v.weight, 200),
  true,
  true
FROM products p
JOIN (VALUES
  ('hydra-vit-c-serum',     199000, 80),
  ('niacinamide-toner',      89000, 130),
  ('daily-sunscreen-spf50', 145000, 80),
  ('gentle-cleanser',        79000, 130),
  ('sheet-mask-hydrating',   75000, 150),
  ('retinol-night-serum',   285000, 60),
  ('ceramide-repair-cream', 215000, 100),
  ('salicylic-toner',       125000, 130),
  ('aha-bha-serum',         185000, 80),
  ('aloe-soothing-gel',      65000, 230),
  ('green-tea-foam',         75000, 180),
  ('vit-e-lip-mask',         49000, 30),
  ('shea-body-lotion',       95000, 280),
  ('coconut-body-scrub',     79000, 230),
  ('vanilla-honey-balm',     39000, 30),
  ('rose-body-mist',        119000, 130),
  ('eau-bloom-50ml',        299000, 160),
  ('eau-citrus-50ml',       299000, 160),
  ('argan-hair-serum',      129000, 80),
  ('anti-dandruff-shampoo',  89000, 280),
  ('keratin-conditioner',    95000, 280),
  ('lipstick-velvet-red',    89000, 40),
  ('lipstick-nude-pink',     89000, 40),
  ('foundation-light',      169000, 80),
  ('foundation-medium',     169000, 80),
  ('eyeshadow-sunset',      249000, 160),
  ('volumizing-mascara',     99000, 50),
  ('brow-pencil-brown',      65000, 30),
  ('highlighter-gold',      119000, 40),
  ('beauty-sponge-set',      49000, 60),
  ('makeup-brush-set',      169000, 260)
) AS v(slug, price, weight) ON v.slug = p.slug
ON CONFLICT (sku) DO NOTHING;

-- ─────────────────────────────────────────────────────────────
-- Inventory (random 8..200, biasakan stok beragam)
-- ─────────────────────────────────────────────────────────────
INSERT INTO inventory_levels(variant_id, on_hand)
SELECT v.id, (8 + floor(random()*180))::int
FROM product_variants v
JOIN products p ON p.id = v.product_id
WHERE p.slug IN (
  'hydra-vit-c-serum','niacinamide-toner','daily-sunscreen-spf50','gentle-cleanser','sheet-mask-hydrating',
  'retinol-night-serum','ceramide-repair-cream','salicylic-toner','aha-bha-serum',
  'aloe-soothing-gel','green-tea-foam','vit-e-lip-mask',
  'shea-body-lotion','coconut-body-scrub','vanilla-honey-balm',
  'rose-body-mist','eau-bloom-50ml','eau-citrus-50ml',
  'argan-hair-serum','anti-dandruff-shampoo','keratin-conditioner',
  'lipstick-velvet-red','lipstick-nude-pink','foundation-light','foundation-medium',
  'eyeshadow-sunset','volumizing-mascara','brow-pencil-brown','highlighter-gold',
  'beauty-sponge-set','makeup-brush-set'
)
ON CONFLICT (variant_id) DO NOTHING;

-- Knock a few SKUs to "stok tipis" (≤ 5) untuk demo badge "Stok Tipis"
UPDATE inventory_levels SET on_hand = 3 WHERE variant_id IN (SELECT id FROM product_variants WHERE sku IN ('SHEETMASKHYDRATING-001','VITELIPMASK-001','BROWPENCILBROWN-001'));
-- Set 1 SKU jadi habis stok untuk demo "Habis"
UPDATE inventory_levels SET on_hand = 0 WHERE variant_id IN (SELECT id FROM product_variants WHERE sku = 'EAUCITRUS50ML-001');

-- ─────────────────────────────────────────────────────────────
-- B2C prices = base_price (untuk audience b2c)
-- ─────────────────────────────────────────────────────────────
INSERT INTO product_prices(variant_id, audience, price)
SELECT v.id, 'b2c', v.base_price
FROM product_variants v
WHERE NOT EXISTS (SELECT 1 FROM product_prices p WHERE p.variant_id = v.id AND p.audience = 'b2c');

-- ─────────────────────────────────────────────────────────────
-- Product images: pakai placehold.co dengan tema warna per-kategori (untuk demo, bisa di-replace via /admin)
-- ─────────────────────────────────────────────────────────────
INSERT INTO product_images(product_id, url, alt_text, is_primary, sort_order)
SELECT
  p.id,
  'https://placehold.co/600x600/' ||
    CASE c.slug
      WHEN 'skincare'  THEN 'fce7f3/be185d'  -- pink → rose
      WHEN 'makeup'    THEN 'fef3c7/b45309'  -- nude → amber
      WHEN 'body-care' THEN 'dcfce7/166534'  -- mint → green
      WHEN 'hair-care' THEN 'e0e7ff/4338ca'  -- lavender → indigo
      WHEN 'fragrance' THEN 'fed7aa/9a3412'  -- peach → orange
      WHEN 'tools'     THEN 'e5e7eb/374151'  -- gray → slate
      ELSE 'fafafa/0b1220'
    END ||
    '/png?text=' || replace(replace(replace(replace(p.name, ' ', '+'), '"', ''), '%', ''), '&', '%26'),
  p.name,
  true, 0
FROM products p
JOIN categories c ON c.id = p.category_id
WHERE p.slug IN (
  'hydra-vit-c-serum','niacinamide-toner','daily-sunscreen-spf50','gentle-cleanser','sheet-mask-hydrating',
  'retinol-night-serum','ceramide-repair-cream','salicylic-toner','aha-bha-serum',
  'aloe-soothing-gel','green-tea-foam','vit-e-lip-mask',
  'shea-body-lotion','coconut-body-scrub','vanilla-honey-balm',
  'rose-body-mist','eau-bloom-50ml','eau-citrus-50ml',
  'argan-hair-serum','anti-dandruff-shampoo','keratin-conditioner',
  'lipstick-velvet-red','lipstick-nude-pink','foundation-light','foundation-medium',
  'eyeshadow-sunset','volumizing-mascara','brow-pencil-brown','highlighter-gold',
  'beauty-sponge-set','makeup-brush-set'
)
AND NOT EXISTS (SELECT 1 FROM product_images i WHERE i.product_id = p.id);

-- ─────────────────────────────────────────────────────────────
-- Beberapa halaman/blog post sample untuk variasi konten
-- ─────────────────────────────────────────────────────────────
INSERT INTO pages(slug, title, body_html, seo_title, seo_desc) VALUES
  ('cara-pesan',     'Cara Pesan',          '<h2>Cara Belanja di GlowMart Beauty</h2><ol><li>Pilih produk yang Anda inginkan dari katalog.</li><li>Klik <strong>Beli Sekarang</strong> atau tambah ke keranjang.</li><li>Isi alamat pengiriman & pilih kurir.</li><li>Bayar via transfer/QRIS/e-wallet.</li><li>Pesanan diproses & dikirim H+1 hari kerja.</li></ol>', 'Cara Pesan',         'Panduan belanja di GlowMart Beauty'),
  ('cara-pengiriman','Pengiriman & Ongkir', '<h2>Pengiriman</h2><p>Kami kirim dari Jakarta Timur menggunakan kurir JNE, J&T, SiCepat, AnterAja, dan POS Indonesia. Estimasi sampai:</p><ul><li>Jabodetabek: 1-2 hari kerja</li><li>Pulau Jawa: 2-3 hari</li><li>Sumatera/Bali: 3-5 hari</li><li>Kalimantan/Sulawesi: 4-6 hari</li><li>NTT/Maluku/Papua: 5-9 hari</li></ul><p>Setiap pesanan dikemas dengan bubble wrap & free safety packaging.</p>', 'Pengiriman & Ongkir', 'Info pengiriman & estimasi')
ON CONFLICT (slug) DO NOTHING;
