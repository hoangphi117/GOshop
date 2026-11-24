-- Add variant fields to order_items
SET search_path TO orders, public;

ALTER TABLE orders.order_items
    ADD COLUMN IF NOT EXISTS order_item_variant_uuid  UUID,
    ADD COLUMN IF NOT EXISTS order_item_variant_title VARCHAR(255);

CREATE INDEX IF NOT EXISTS idx_order_items_variant_uuid
    ON orders.order_items (order_item_variant_uuid)
    WHERE order_item_variant_uuid IS NOT NULL;
