-- Migration 000018 Down: Rollback snapshot columns and cross-boundary UUIDs

ALTER TABLE coupon_usages
    DROP COLUMN IF EXISTS coupon_usage_order_uuid,
    DROP COLUMN IF EXISTS coupon_usage_user_uuid,
    DROP COLUMN IF EXISTS coupon_usage_coupon_uuid;

ALTER TABLE stock_movements
    DROP COLUMN IF EXISTS product_uuid;

ALTER TABLE cart_items
    DROP COLUMN IF EXISTS cart_item_product_uuid;

ALTER TABLE order_items
    DROP COLUMN IF EXISTS order_item_product_uuid,
    DROP COLUMN IF EXISTS order_item_product_slug,
    DROP COLUMN IF EXISTS order_item_product_image_url,
    DROP COLUMN IF EXISTS order_item_order_uuid;
