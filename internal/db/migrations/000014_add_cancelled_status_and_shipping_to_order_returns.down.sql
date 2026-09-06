ALTER TABLE order_returns DROP CONSTRAINT IF EXISTS order_returns_return_status_check;

DROP INDEX IF EXISTS idx_returns_shipping_code;

ALTER TABLE order_returns
    DROP COLUMN IF EXISTS return_shipping_provider,
    DROP COLUMN IF EXISTS return_shipping_code,
    DROP COLUMN IF EXISTS return_shipping_note;