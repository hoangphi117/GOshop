ALTER TABLE payment_transactions ADD COLUMN IF NOT EXISTS pt_order_uuid UUID;
CREATE INDEX IF NOT EXISTS idx_pt_order_uuid ON payment_transactions(pt_order_uuid);

UPDATE payment_transactions pt
SET pt_order_uuid = o.order_uuid
FROM orders o
WHERE pt.pt_order_id = o.order_id AND pt.pt_order_uuid IS NULL;
