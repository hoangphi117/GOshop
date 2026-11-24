-- Add variant_uuid and variant_title to cart_items
-- Strategy: variant_uuid is nullable.
-- Unique constraint: (cart_id, product_uuid) if no variant; (cart_id, product_uuid, variant_uuid) if variant.

ALTER TABLE cart_items
    ADD COLUMN IF NOT EXISTS variant_uuid  UUID,
    ADD COLUMN IF NOT EXISTS variant_title VARCHAR(255);

-- Drop the old unique constraint (cart_id, product_uuid)
ALTER TABLE cart_items DROP CONSTRAINT IF EXISTS uk_cart_product;

-- New partial unique indexes replacing the old constraint:

-- For products without variants (variant_uuid IS NULL)
CREATE UNIQUE INDEX IF NOT EXISTS uq_cart_product_no_variant
    ON cart_items (cart_id, product_uuid)
    WHERE variant_uuid IS NULL;

-- For products with variants (one item per variant in a cart)
CREATE UNIQUE INDEX IF NOT EXISTS uq_cart_product_variant
    ON cart_items (cart_id, product_uuid, variant_uuid)
    WHERE variant_uuid IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_cart_items_variant_uuid ON cart_items(variant_uuid);
