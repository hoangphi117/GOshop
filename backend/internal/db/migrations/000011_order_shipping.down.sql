DROP TRIGGER IF EXISTS set_return_updated_at ON order_returns;
DROP FUNCTION IF EXISTS update_return_updated_at();
DROP INDEX IF EXISTS idx_returns_active_per_order;
DROP INDEX IF EXISTS idx_returns_status;
DROP INDEX IF EXISTS idx_returns_user;
DROP INDEX IF EXISTS idx_returns_order;
DROP TABLE IF EXISTS order_returns;
DROP INDEX IF EXISTS idx_tracking_order;
DROP TABLE IF EXISTS order_tracking_events;
ALTER TABLE orders
    DROP COLUMN IF EXISTS order_shipping_provider,
    DROP COLUMN IF EXISTS order_shipping_code,
    DROP COLUMN IF EXISTS order_shipping_fee,
    DROP COLUMN IF EXISTS order_expected_delivery,
    DROP COLUMN IF EXISTS order_delivered_at,
    DROP COLUMN IF EXISTS order_returned_at;
