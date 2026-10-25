-- Analytics read model owned by the analytics service.
--
-- These were previously maintained inside the monolith database
-- (analytics_*_projection). They are copied here so analytics owns its
-- storage; the order projection consumer feeds them.
--
-- Cross-database foreign keys are intentionally absent: products and users
-- live in other services now. Product dimensions are resolved over gRPC at
-- query time.

CREATE TABLE IF NOT EXISTS analytics_orders_projection (
    order_id                INT PRIMARY KEY,
    order_uuid              UUID NOT NULL UNIQUE,
    order_number            VARCHAR(20) NOT NULL UNIQUE,
    order_user_id           INT NOT NULL,
    order_user_uuid         UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000000',
    order_customer_name     VARCHAR(100) NOT NULL DEFAULT '',
    order_customer_email    VARCHAR(255) NOT NULL DEFAULT '',
    order_shipping_name     VARCHAR(100) NOT NULL DEFAULT '',
    order_shipping_phone    VARCHAR(20) NOT NULL DEFAULT '',
    order_shipping_address  TEXT NOT NULL DEFAULT '',
    order_total_amount      NUMERIC(14,2) NOT NULL CHECK (order_total_amount >= 0),
    order_refunded_amount   NUMERIC(14,2) NOT NULL DEFAULT 0.00 CHECK (order_refunded_amount >= 0),
    order_discount_amount   NUMERIC(12,2) NOT NULL DEFAULT 0.00,
    order_payment_method    INT NOT NULL DEFAULT 1 CHECK (order_payment_method IN (1, 2, 3)),
    order_payment_status    INT NOT NULL DEFAULT 1 CHECK (order_payment_status IN (1, 2, 3, 4)),
    order_note              TEXT,
    order_status            INT NOT NULL DEFAULT 1 CHECK (order_status IN (1, 2, 3, 4, 5, 6)),
    order_shipping_provider VARCHAR(20),
    order_shipping_code     VARCHAR(50),
    order_shipping_fee      NUMERIC(12,2) NOT NULL DEFAULT 0.00,
    order_delivered_at      TIMESTAMPTZ DEFAULT NULL,
    order_cancelled_at      TIMESTAMPTZ DEFAULT NULL,
    order_created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    order_updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_analytics_orders_created_at ON analytics_orders_projection (order_created_at);
CREATE INDEX IF NOT EXISTS idx_analytics_orders_paid ON analytics_orders_projection (order_payment_status, order_created_at);

CREATE TABLE IF NOT EXISTS analytics_order_items_projection (
    order_item_uuid          UUID PRIMARY KEY,
    order_item_order_id      INT NOT NULL REFERENCES analytics_orders_projection(order_id) ON DELETE CASCADE,
    -- No FK: products live in the catalog service.
    order_item_product_id    INT NOT NULL,
    order_item_product_uuid  UUID,
    order_item_product_name  VARCHAR(200) NOT NULL,
    order_item_price         NUMERIC(12,2) NOT NULL CHECK (order_item_price > 0),
    order_item_quantity      INT NOT NULL CHECK (order_item_quantity > 0),
    order_item_subtotal      NUMERIC(14,2) NOT NULL CHECK (order_item_subtotal > 0),
    order_item_created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_analytics_order_items_order_id ON analytics_order_items_projection (order_item_order_id);
CREATE INDEX IF NOT EXISTS idx_analytics_order_items_product_id ON analytics_order_items_projection (order_item_product_id);
CREATE INDEX IF NOT EXISTS idx_analytics_order_items_product_uuid ON analytics_order_items_projection (order_item_product_uuid);

