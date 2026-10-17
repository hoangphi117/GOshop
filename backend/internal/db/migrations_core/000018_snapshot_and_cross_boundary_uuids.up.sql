-- Migration 000018: Add cross-boundary UUIDs and snapshot columns for microservices decoupling

-- 1. order_items: add snapshot columns and UUIDs
ALTER TABLE order_items
    ADD COLUMN IF NOT EXISTS order_item_product_uuid UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000000',
    ADD COLUMN IF NOT EXISTS order_item_product_slug VARCHAR(255) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS order_item_product_image_url TEXT,
    ADD COLUMN IF NOT EXISTS order_item_order_uuid UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000000';

-- Backfill order_items from products and orders
UPDATE order_items oi
SET 
    order_item_product_uuid = p.product_uuid,
    order_item_product_slug = p.product_slug,
    order_item_product_image_url = p.product_image_url
FROM products p
WHERE oi.order_item_product_id = p.product_id
  AND (oi.order_item_product_uuid IS NULL OR oi.order_item_product_slug IS NULL);

UPDATE order_items oi
SET order_item_order_uuid = o.order_uuid
FROM orders o
WHERE oi.order_item_order_id = o.order_id
  AND oi.order_item_order_uuid IS NULL;

CREATE INDEX IF NOT EXISTS idx_order_items_product_uuid ON order_items(order_item_product_uuid);
CREATE INDEX IF NOT EXISTS idx_order_items_order_uuid ON order_items(order_item_order_uuid);

-- 2. cart_items: add product_uuid
ALTER TABLE cart_items
    ADD COLUMN IF NOT EXISTS cart_item_product_uuid UUID;

UPDATE cart_items ci
SET cart_item_product_uuid = p.product_uuid
FROM products p
WHERE ci.cart_item_product_id = p.product_id
  AND ci.cart_item_product_uuid IS NULL;

CREATE INDEX IF NOT EXISTS idx_cart_items_product_uuid ON cart_items(cart_item_product_uuid);

-- 3. stock_movements: add product_uuid
ALTER TABLE stock_movements
    ADD COLUMN IF NOT EXISTS product_uuid UUID;

UPDATE stock_movements sm
SET product_uuid = p.product_uuid
FROM products p
WHERE sm.product_id = p.product_id
  AND sm.product_uuid IS NULL;

CREATE INDEX IF NOT EXISTS idx_stock_movements_product_uuid ON stock_movements(product_uuid);

-- 4. coupon_usages: add cross-boundary UUIDs
ALTER TABLE coupon_usages
    ADD COLUMN IF NOT EXISTS coupon_usage_order_uuid UUID,
    ADD COLUMN IF NOT EXISTS coupon_usage_user_uuid UUID,
    ADD COLUMN IF NOT EXISTS coupon_usage_coupon_uuid UUID;

UPDATE coupon_usages cu
SET coupon_usage_order_uuid = o.order_uuid
FROM orders o
WHERE cu.coupon_usage_order_id = o.order_id
  AND cu.coupon_usage_order_uuid IS NULL;

UPDATE coupon_usages cu
SET coupon_usage_coupon_uuid = c.coupon_uuid
FROM coupons c
WHERE cu.coupon_usage_coupon_id = c.coupon_id
  AND cu.coupon_usage_coupon_uuid IS NULL;

CREATE INDEX IF NOT EXISTS idx_coupon_usages_order_uuid ON coupon_usages(coupon_usage_order_uuid);
CREATE INDEX IF NOT EXISTS idx_coupon_usages_coupon_uuid ON coupon_usages(coupon_usage_coupon_uuid);
CREATE INDEX IF NOT EXISTS idx_coupon_usages_user_uuid ON coupon_usages(coupon_usage_user_uuid);
