DROP TRIGGER IF EXISTS set_pt_updated_at ON payment_transactions;
DROP FUNCTION IF EXISTS update_pt_updated_at();
DROP INDEX IF EXISTS idx_we_unprocessed;
DROP INDEX IF EXISTS idx_pt_refund_of;
DROP INDEX IF EXISTS idx_pt_provider_txn;
DROP INDEX IF EXISTS idx_pt_order;
DROP INDEX IF EXISTS idx_pt_pending_online;
DROP TABLE IF EXISTS webhook_events;
DROP TABLE IF EXISTS payment_transactions;
