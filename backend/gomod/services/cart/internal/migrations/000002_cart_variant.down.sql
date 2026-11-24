DROP INDEX IF EXISTS uq_cart_product_variant;
DROP INDEX IF EXISTS uq_cart_product_no_variant;
DROP INDEX IF EXISTS idx_cart_items_variant_uuid;

ALTER TABLE cart_items DROP COLUMN IF EXISTS variant_uuid;
ALTER TABLE cart_items DROP COLUMN IF EXISTS variant_title;

ALTER TABLE cart_items ADD CONSTRAINT uk_cart_product UNIQUE (cart_id, product_uuid);
