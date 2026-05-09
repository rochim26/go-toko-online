-- TokoOnline init schema
-- Idempotent: uses IF NOT EXISTS where possible.

CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS unaccent;
CREATE EXTENSION IF NOT EXISTS citext;

-- ─────────────────────────────────────────────────────────────────────────────
-- Identity
-- ─────────────────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           CITEXT UNIQUE NOT NULL,
    phone           TEXT,
    password_hash   TEXT NOT NULL,
    role            TEXT NOT NULL CHECK (role IN ('admin','staff','customer','reseller')),
    full_name       TEXT,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    email_verified  BOOLEAN NOT NULL DEFAULT FALSE,
    last_login_at   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS users_role_idx ON users(role);

CREATE TABLE IF NOT EXISTS reseller_tiers (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code            TEXT UNIQUE NOT NULL,
    name            TEXT NOT NULL,
    discount_pct    NUMERIC(5,2) NOT NULL DEFAULT 0 CHECK (discount_pct BETWEEN 0 AND 100),
    moq_qty         INTEGER NOT NULL DEFAULT 1 CHECK (moq_qty >= 1),
    moq_value       NUMERIC(14,2) NOT NULL DEFAULT 0,
    credit_limit    NUMERIC(14,2) NOT NULL DEFAULT 0,
    top_days        INTEGER NOT NULL DEFAULT 0 CHECK (top_days >= 0),
    sort_order      INTEGER NOT NULL DEFAULT 0,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS reseller_profiles (
    user_id         UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    tier_id         UUID REFERENCES reseller_tiers(id),
    store_name      TEXT NOT NULL,
    npwp            TEXT,
    ktp_number      TEXT,
    address         TEXT,
    province        TEXT,
    city            TEXT,
    district        TEXT,
    postal_code     TEXT,
    contact_phone   TEXT,
    docs            JSONB NOT NULL DEFAULT '[]'::jsonb,
    status          TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending','approved','rejected','suspended')),
    rejection_reason TEXT,
    credit_limit_override NUMERIC(14,2),
    notes           TEXT,
    approved_at     TIMESTAMPTZ,
    approved_by     UUID REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS reseller_profiles_status_idx ON reseller_profiles(status);

CREATE TABLE IF NOT EXISTS customer_addresses (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    label           TEXT NOT NULL DEFAULT 'Rumah',
    recipient       TEXT NOT NULL,
    phone           TEXT NOT NULL,
    address         TEXT NOT NULL,
    province        TEXT NOT NULL,
    city            TEXT NOT NULL,
    district        TEXT,
    postal_code     TEXT NOT NULL,
    area_id         TEXT,        -- biteship area id
    is_default      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS customer_addresses_user_idx ON customer_addresses(user_id);

-- ─────────────────────────────────────────────────────────────────────────────
-- Catalog
-- ─────────────────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS brands (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    slug        TEXT UNIQUE NOT NULL,
    logo_url    TEXT,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS categories (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parent_id   UUID REFERENCES categories(id) ON DELETE SET NULL,
    name        TEXT NOT NULL,
    slug        TEXT UNIQUE NOT NULL,
    description TEXT,
    image_url   TEXT,
    sort_order  INTEGER NOT NULL DEFAULT 0,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    seo_title   TEXT,
    seo_desc    TEXT,
    gmc_category TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS categories_parent_idx ON categories(parent_id);

CREATE TABLE IF NOT EXISTS products (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug            TEXT UNIQUE NOT NULL,
    name            TEXT NOT NULL,
    brand_id        UUID REFERENCES brands(id) ON DELETE SET NULL,
    category_id     UUID REFERENCES categories(id) ON DELETE SET NULL,
    short_desc      TEXT,
    description     TEXT,
    status          TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','active','archived')),
    is_b2c          BOOLEAN NOT NULL DEFAULT TRUE,
    is_b2b          BOOLEAN NOT NULL DEFAULT TRUE,
    weight_grams    INTEGER NOT NULL DEFAULT 500,
    length_cm       INTEGER,
    width_cm        INTEGER,
    height_cm       INTEGER,
    -- SEO
    seo_title       TEXT,
    seo_desc        TEXT,
    focus_keyword   TEXT,
    og_image_url    TEXT,
    schema_override JSONB,
    -- Marketing
    gmc_enabled     BOOLEAN NOT NULL DEFAULT TRUE,
    gmc_brand       TEXT,
    gmc_gtin        TEXT,
    gmc_mpn         TEXT,
    gmc_condition   TEXT NOT NULL DEFAULT 'new',
    gmc_age_group   TEXT,
    gmc_gender      TEXT,
    -- FAQ for AI Overview
    faqs            JSONB NOT NULL DEFAULT '[]'::jsonb,
    -- search
    search_vec      tsvector,
    published_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS products_status_idx ON products(status);
CREATE INDEX IF NOT EXISTS products_category_idx ON products(category_id);
CREATE INDEX IF NOT EXISTS products_search_idx ON products USING GIN(search_vec);
CREATE INDEX IF NOT EXISTS products_name_trgm_idx ON products USING GIN(name gin_trgm_ops);

CREATE OR REPLACE FUNCTION products_search_trigger() RETURNS trigger AS $$
BEGIN
    NEW.search_vec :=
        setweight(to_tsvector('simple', unaccent(coalesce(NEW.name,''))), 'A') ||
        setweight(to_tsvector('simple', unaccent(coalesce(NEW.short_desc,''))), 'B') ||
        setweight(to_tsvector('simple', unaccent(coalesce(NEW.description,''))), 'C');
    NEW.updated_at := now();
    RETURN NEW;
END
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS products_search_trg ON products;
CREATE TRIGGER products_search_trg BEFORE INSERT OR UPDATE ON products
    FOR EACH ROW EXECUTE FUNCTION products_search_trigger();

CREATE TABLE IF NOT EXISTS product_variants (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id      UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    sku             TEXT UNIQUE NOT NULL,
    name            TEXT,
    attributes      JSONB NOT NULL DEFAULT '{}'::jsonb,
    base_price      NUMERIC(14,2) NOT NULL DEFAULT 0,
    compare_at_price NUMERIC(14,2),
    cost_price      NUMERIC(14,2),
    weight_grams    INTEGER,
    barcode         TEXT,
    is_default      BOOLEAN NOT NULL DEFAULT FALSE,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS variants_product_idx ON product_variants(product_id);

CREATE TABLE IF NOT EXISTS product_images (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id      UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    variant_id      UUID REFERENCES product_variants(id) ON DELETE SET NULL,
    url             TEXT NOT NULL,
    alt_text        TEXT,
    sort_order      INTEGER NOT NULL DEFAULT 0,
    is_primary      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS images_product_idx ON product_images(product_id);

CREATE TABLE IF NOT EXISTS product_prices (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    variant_id      UUID NOT NULL REFERENCES product_variants(id) ON DELETE CASCADE,
    audience        TEXT NOT NULL,           -- 'b2c' or tier_id (uuid as text)
    price           NUMERIC(14,2) NOT NULL,
    valid_from      TIMESTAMPTZ,
    valid_to        TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS prices_variant_audience_idx ON product_prices(variant_id, audience);

CREATE TABLE IF NOT EXISTS inventory_levels (
    variant_id      UUID PRIMARY KEY REFERENCES product_variants(id) ON DELETE CASCADE,
    on_hand         INTEGER NOT NULL DEFAULT 0,
    reserved        INTEGER NOT NULL DEFAULT 0,
    low_stock_at    INTEGER NOT NULL DEFAULT 5,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ─────────────────────────────────────────────────────────────────────────────
-- Cart & Orders
-- ─────────────────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS carts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID REFERENCES users(id) ON DELETE SET NULL,
    session_token   TEXT UNIQUE,
    channel         TEXT NOT NULL DEFAULT 'b2c' CHECK (channel IN ('b2c','b2b')),
    audience        TEXT NOT NULL DEFAULT 'b2c',
    notes           TEXT,
    abandoned_notified BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS carts_user_idx ON carts(user_id);

CREATE TABLE IF NOT EXISTS cart_items (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cart_id         UUID NOT NULL REFERENCES carts(id) ON DELETE CASCADE,
    variant_id      UUID NOT NULL REFERENCES product_variants(id),
    qty             INTEGER NOT NULL CHECK (qty > 0),
    unit_price      NUMERIC(14,2) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(cart_id, variant_id)
);

CREATE TABLE IF NOT EXISTS orders (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code                TEXT UNIQUE NOT NULL,
    user_id             UUID REFERENCES users(id) ON DELETE SET NULL,
    channel             TEXT NOT NULL CHECK (channel IN ('b2c','b2b')),
    status              TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending','awaiting_payment','paid','packed','shipped','delivered','cancelled','refunded','on_credit','overdue')),
    payment_status      TEXT NOT NULL DEFAULT 'unpaid'
        CHECK (payment_status IN ('unpaid','paid','failed','expired','refunded','top')),
    payment_method      TEXT,
    payment_term        TEXT NOT NULL DEFAULT 'prepaid' CHECK (payment_term IN ('prepaid','top')),
    top_due_at          TIMESTAMPTZ,
    -- amounts
    subtotal            NUMERIC(14,2) NOT NULL DEFAULT 0,
    discount_total      NUMERIC(14,2) NOT NULL DEFAULT 0,
    shipping_total      NUMERIC(14,2) NOT NULL DEFAULT 0,
    tax_total           NUMERIC(14,2) NOT NULL DEFAULT 0,
    grand_total         NUMERIC(14,2) NOT NULL DEFAULT 0,
    paid_total          NUMERIC(14,2) NOT NULL DEFAULT 0,
    -- shipping
    ship_recipient      TEXT,
    ship_phone          TEXT,
    ship_address        TEXT,
    ship_province       TEXT,
    ship_city           TEXT,
    ship_district       TEXT,
    ship_postal_code    TEXT,
    ship_area_id        TEXT,
    courier_code        TEXT,
    courier_service     TEXT,
    awb                 TEXT,
    -- billing/customer snapshot
    customer_email      TEXT,
    customer_phone      TEXT,
    customer_name       TEXT,
    -- attribution
    utm                 JSONB,
    fbp                 TEXT,
    fbc                 TEXT,
    client_ip           INET,
    user_agent          TEXT,
    -- xendit
    xendit_invoice_id   TEXT,
    xendit_invoice_url  TEXT,
    -- meta
    notes               TEXT,
    voucher_code        TEXT,
    paid_at             TIMESTAMPTZ,
    cancelled_at        TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS orders_user_idx ON orders(user_id);
CREATE INDEX IF NOT EXISTS orders_status_idx ON orders(status);
CREATE INDEX IF NOT EXISTS orders_created_idx ON orders(created_at DESC);

CREATE TABLE IF NOT EXISTS order_items (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id        UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    variant_id      UUID REFERENCES product_variants(id) ON DELETE SET NULL,
    sku             TEXT NOT NULL,
    name            TEXT NOT NULL,
    qty             INTEGER NOT NULL,
    unit_price      NUMERIC(14,2) NOT NULL,
    line_total      NUMERIC(14,2) NOT NULL,
    image_url       TEXT,
    attributes      JSONB
);
CREATE INDEX IF NOT EXISTS order_items_order_idx ON order_items(order_id);

CREATE TABLE IF NOT EXISTS payments (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id            UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    provider            TEXT NOT NULL DEFAULT 'xendit',
    provider_ref        TEXT,
    amount              NUMERIC(14,2) NOT NULL,
    method              TEXT,
    status              TEXT NOT NULL,
    raw                 JSONB,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS payments_order_idx ON payments(order_id);

-- B2B PO documents
CREATE TABLE IF NOT EXISTS po_documents (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id        UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    po_number       TEXT NOT NULL,
    file_path       TEXT,
    issued_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    due_at          TIMESTAMPTZ,
    notes           TEXT
);

-- ─────────────────────────────────────────────────────────────────────────────
-- Marketing
-- ─────────────────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS vouchers (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code            TEXT UNIQUE NOT NULL,
    name            TEXT,
    type            TEXT NOT NULL CHECK (type IN ('percent','fixed','shipping')),
    value           NUMERIC(14,2) NOT NULL DEFAULT 0,
    min_subtotal    NUMERIC(14,2) NOT NULL DEFAULT 0,
    max_discount    NUMERIC(14,2),
    audience        TEXT NOT NULL DEFAULT 'all' CHECK (audience IN ('all','b2c','b2b')),
    usage_limit     INTEGER,
    used_count      INTEGER NOT NULL DEFAULT 0,
    valid_from      TIMESTAMPTZ,
    valid_to        TIMESTAMPTZ,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS events_log (
    id              BIGSERIAL PRIMARY KEY,
    event_id        TEXT UNIQUE,
    event_name      TEXT NOT NULL,
    user_id         UUID REFERENCES users(id) ON DELETE SET NULL,
    session_token   TEXT,
    url             TEXT,
    referer         TEXT,
    payload         JSONB,
    sent_meta       BOOLEAN NOT NULL DEFAULT FALSE,
    sent_ga4        BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS events_log_created_idx ON events_log(created_at DESC);

CREATE TABLE IF NOT EXISTS utm_attributions (
    id              BIGSERIAL PRIMARY KEY,
    session_token   TEXT,
    user_id         UUID REFERENCES users(id) ON DELETE SET NULL,
    utm_source      TEXT,
    utm_medium      TEXT,
    utm_campaign    TEXT,
    utm_term        TEXT,
    utm_content     TEXT,
    landing_url     TEXT,
    referer         TEXT,
    is_first_touch  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS utm_session_idx ON utm_attributions(session_token);

-- ─────────────────────────────────────────────────────────────────────────────
-- Content & SEO
-- ─────────────────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS pages (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug            TEXT UNIQUE NOT NULL,
    title           TEXT NOT NULL,
    body_html       TEXT NOT NULL DEFAULT '',
    seo_title       TEXT,
    seo_desc        TEXT,
    is_published    BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS blog_posts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug            TEXT UNIQUE NOT NULL,
    title           TEXT NOT NULL,
    excerpt         TEXT,
    body_html       TEXT NOT NULL DEFAULT '',
    cover_url       TEXT,
    seo_title       TEXT,
    seo_desc        TEXT,
    author          TEXT,
    is_published    BOOLEAN NOT NULL DEFAULT FALSE,
    published_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS redirects (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    from_path       TEXT UNIQUE NOT NULL,
    to_path         TEXT NOT NULL,
    code            INTEGER NOT NULL DEFAULT 301 CHECK (code IN (301,302,307,308)),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ─────────────────────────────────────────────────────────────────────────────
-- Settings & Audit
-- ─────────────────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS settings (
    key             TEXT PRIMARY KEY,
    value           JSONB NOT NULL DEFAULT '{}'::jsonb,
    description     TEXT,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by      UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS audit_log (
    id              BIGSERIAL PRIMARY KEY,
    user_id         UUID REFERENCES users(id) ON DELETE SET NULL,
    action          TEXT NOT NULL,
    entity          TEXT,
    entity_id       TEXT,
    diff            JSONB,
    ip              INET,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS audit_log_created_idx ON audit_log(created_at DESC);

-- ─────────────────────────────────────────────────────────────────────────────
-- Sessions table for SCS postgres store
-- ─────────────────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS sessions (
    token   TEXT PRIMARY KEY,
    data    BYTEA NOT NULL,
    expiry  TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS sessions_expiry_idx ON sessions(expiry);

-- ─────────────────────────────────────────────────────────────────────────────
-- Migration tracking
-- ─────────────────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS schema_migrations (
    version     TEXT PRIMARY KEY,
    applied_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
