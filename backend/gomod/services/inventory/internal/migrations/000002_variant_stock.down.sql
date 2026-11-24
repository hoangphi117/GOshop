-- Revert variant_uuid columns from inventory
DROP INDEX IF EXISTS uq_inventory_product_variant;
DROP INDEX IF EXISTS uq_inventory_product_no_variant;
DROP INDEX IF EXISTS idx_inventory_stocks_variant_uuid;
DROP INDEX IF EXISTS uq_stock_movements_idempotency_variant;
DROP INDEX IF EXISTS idx_stock_movements_variant_uuid;

ALTER TABLE inventory_stocks DROP COLUMN IF EXISTS variant_uuid;
ALTER TABLE inventory_stocks DROP COLUMN IF EXISTS variant_id;
ALTER TABLE stock_movements  DROP COLUMN IF EXISTS variant_uuid;
ALTER TABLE stock_movements  DROP COLUMN IF EXISTS variant_id;

-- Restore original unique index
CREATE UNIQUE INDEX IF NOT EXISTS inventory_stocks_product_uuid_key ON inventory_stocks(product_uuid);
