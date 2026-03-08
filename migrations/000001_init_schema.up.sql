-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Users
CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email         VARCHAR(255) UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    name          VARCHAR(255) NOT NULL,
    role          VARCHAR(20) NOT NULL DEFAULT 'staff' CHECK (role IN ('admin', 'staff')),
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    last_login_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Categories
CREATE TABLE categories (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        VARCHAR(255) UNIQUE NOT NULL,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Products
CREATE TABLE products (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name           VARCHAR(255) NOT NULL,
    description    TEXT,
    category_id    UUID REFERENCES categories(id) ON DELETE SET NULL,
    sku            VARCHAR(100) UNIQUE NOT NULL,
    barcode        VARCHAR(100) UNIQUE NOT NULL,
    buying_price   NUMERIC(12,2) NOT NULL DEFAULT 0,
    selling_price  NUMERIC(12,2) NOT NULL DEFAULT 0,
    quantity       INTEGER NOT NULL DEFAULT 0,
    reorder_level  INTEGER NOT NULL DEFAULT 5,
    supplier_name  VARCHAR(255),
    supplier_phone VARCHAR(50),
    image_url      TEXT,
    is_active      BOOLEAN NOT NULL DEFAULT TRUE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_products_sku ON products(sku);
CREATE INDEX idx_products_barcode ON products(barcode);
CREATE INDEX idx_products_category ON products(category_id);
CREATE INDEX idx_products_name ON products USING gin(to_tsvector('english', name));

-- Stock Movements
CREATE TABLE stock_movements (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    product_id   UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    type         VARCHAR(20) NOT NULL CHECK (type IN ('in', 'out', 'adjustment')),
    quantity     INTEGER NOT NULL,
    reference    VARCHAR(255),
    notes        TEXT,
    created_by   UUID NOT NULL REFERENCES users(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_stock_movements_product ON stock_movements(product_id);
CREATE INDEX idx_stock_movements_created_at ON stock_movements(created_at);

-- Sales
CREATE TABLE sales (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    total_amount      NUMERIC(12,2) NOT NULL,
    discount_type     VARCHAR(20) DEFAULT '' CHECK (discount_type IN ('', 'percentage', 'fixed')),
    discount_value    NUMERIC(12,2) NOT NULL DEFAULT 0,
    discount_amount   NUMERIC(12,2) NOT NULL DEFAULT 0,
    net_amount        NUMERIC(12,2) NOT NULL,
    payment_method    VARCHAR(20) NOT NULL CHECK (payment_method IN ('cash', 'mpesa', 'card', 'credit')),
    payment_reference VARCHAR(255),
    customer_name     VARCHAR(255),
    customer_phone    VARCHAR(50),
    notes             TEXT,
    created_by        UUID NOT NULL REFERENCES users(id),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sales_created_at ON sales(created_at);
CREATE INDEX idx_sales_payment_method ON sales(payment_method);
CREATE INDEX idx_sales_created_by ON sales(created_by);

-- Sale Items
CREATE TABLE sale_items (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    sale_id      UUID NOT NULL REFERENCES sales(id) ON DELETE CASCADE,
    product_id   UUID NOT NULL REFERENCES products(id),
    quantity     INTEGER NOT NULL,
    unit_price   NUMERIC(12,2) NOT NULL,
    buying_price NUMERIC(12,2) NOT NULL,
    subtotal     NUMERIC(12,2) NOT NULL
);

CREATE INDEX idx_sale_items_sale ON sale_items(sale_id);
CREATE INDEX idx_sale_items_product ON sale_items(product_id);

-- Payments (M-Pesa)
CREATE TABLE payments (
    id                   UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    sale_id              UUID REFERENCES sales(id) ON DELETE SET NULL,
    mpesa_receipt        VARCHAR(255),
    phone_number         VARCHAR(50),
    amount               NUMERIC(12,2) NOT NULL,
    status               VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'completed', 'failed', 'cancelled')),
    result_code          VARCHAR(10),
    result_desc          TEXT,
    checkout_request_id  VARCHAR(255),
    merchant_request_id  VARCHAR(255),
    transaction_date     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payments_sale ON payments(sale_id);
CREATE INDEX idx_payments_status ON payments(status);
CREATE INDEX idx_payments_phone ON payments(phone_number);
CREATE INDEX idx_payments_checkout_id ON payments(checkout_request_id);

-- Alert Preferences
CREATE TABLE alert_preferences (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    alert_type VARCHAR(50) NOT NULL,
    channel    VARCHAR(20) NOT NULL CHECK (channel IN ('email', 'whatsapp', 'sms')),
    enabled    BOOLEAN NOT NULL DEFAULT TRUE,
    UNIQUE(user_id, alert_type, channel)
);

-- Alerts Log
CREATE TABLE alerts (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type       VARCHAR(50) NOT NULL,
    channel    VARCHAR(20) NOT NULL CHECK (channel IN ('email', 'whatsapp', 'sms')),
    message    TEXT NOT NULL,
    status     VARCHAR(20) NOT NULL DEFAULT 'pending',
    sent_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_alerts_user ON alerts(user_id);
CREATE INDEX idx_alerts_created_at ON alerts(created_at);

-- Auto-update updated_at
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER products_updated_at BEFORE UPDATE ON products
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
