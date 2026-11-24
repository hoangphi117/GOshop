-- Add variant_uuid support to inventory_stocks and stock_movements
-- Strategy: add nullable variant_uuid column; existing rows track base product stock (NULL = no variant).
-- Unique key changes from (product_uuid UNIQUE) to (product_uuid, variant_uuid) composite.

-- inventory_stocks: add variant columns
ALTER TABLE inventory_stocks
    ADD COLUMN IF NOT EXISTS variant_uuid UUID,
    ADD COLUMN IF NOT EXISTS variant_id   INT;

-- Drop the old unique constraint and replace with composite
DROP INDEX IF EXISTS inventory_stocks_product_uuid_key;

-- Partial unique index: when variant_uuid IS NULL → product-level stock (one per product, no variant)
CREATE UNIQUE INDEX IF NOT EXISTS uq_inventory_product_no_variant
    ON inventory_stocks (product_uuid)
    WHERE variant_uuid IS NULL;

-- When variant_uuid IS NOT NULL → variant-level stock (one per variant per product)
CREATE UNIQUE INDEX IF NOT EXISTS uq_inventory_product_variant
    ON inventory_stocks (product_uuid, variant_uuid)
    WHERE variant_uuid IS NOT NULL;

-- Also add to stock_movements
ALTER TABLE stock_movements
    ADD COLUMN IF NOT EXISTS variant_uuid UUID,
    ADD COLUMN IF NOT EXISTS variant_id   INT;

-- New idempotency index for variant-level movements
CREATE UNIQUE INDEX IF NOT EXISTS uq_stock_movements_idempotency_variant
    ON stock_movements (reference_type, reference_id, variant_uuid)
    WHERE reference_id IS NOT NULL
    AND reference_type IS NOT NULL
    AND variant_uuid IS NOT NULL;

-- Indexes for new column
CREATE INDEX IF NOT EXISTS idx_inventory_stocks_variant_uuid ON inventory_stocks(variant_uuid);
CREATE INDEX IF NOT EXISTS idx_stock_movements_variant_uuid  ON stock_movements(variant_uuid, created_at DESC);
