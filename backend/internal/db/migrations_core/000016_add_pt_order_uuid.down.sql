DROP INDEX IF EXISTS idx_pt_order_uuid;
ALTER TABLE payment_transactions DROP COLUMN IF EXISTS pt_order_uuid;
