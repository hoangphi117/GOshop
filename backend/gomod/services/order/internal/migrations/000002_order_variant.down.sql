SET search_path TO orders, public;

DROP INDEX IF EXISTS orders.idx_order_items_variant_uuid;
ALTER TABLE orders.order_items DROP COLUMN IF EXISTS order_item_variant_uuid;
ALTER TABLE orders.order_items DROP COLUMN IF EXISTS order_item_variant_title;
