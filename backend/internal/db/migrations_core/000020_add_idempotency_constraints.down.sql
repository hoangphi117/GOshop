-- Migration 000020 Down
DROP INDEX IF EXISTS uq_stock_movements_idempotency;
DROP INDEX IF EXISTS uq_stock_movements_idempotency_uuid;
DROP INDEX IF EXISTS uq_coupon_usages_order_uuid;
DROP INDEX IF EXISTS uq_coupon_usages_order_id;
