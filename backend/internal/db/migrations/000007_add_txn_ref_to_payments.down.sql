DROP INDEX IF EXISTS idx_pt_txn_ref;
ALTER TABLE payment_transactions DROP COLUMN IF EXISTS pt_txn_ref;
