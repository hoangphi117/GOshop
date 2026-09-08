ALTER TABLE payment_transactions ADD COLUMN IF NOT EXISTS pt_txn_ref VARCHAR(100) DEFAULT NULL;

CREATE INDEX IF NOT EXISTS idx_pt_txn_ref ON payment_transactions(pt_provider, pt_txn_ref) 
    WHERE pt_txn_ref IS NOT NULL;

-- Backfill existing rows: nếu pt_provider_txn_id chứa dấu "_" thì đó là txn_ref
UPDATE payment_transactions 
SET pt_txn_ref = pt_provider_txn_id 
WHERE pt_txn_ref IS NULL AND pt_provider_txn_id LIKE '%_%';
