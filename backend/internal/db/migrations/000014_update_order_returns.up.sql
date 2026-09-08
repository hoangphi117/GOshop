ALTER TABLE order_returns DROP CONSTRAINT IF EXISTS order_returns_return_status_check;

ALTER TABLE order_returns ADD CONSTRAINT order_returns_return_status_check 
    CHECK (return_status IN (1, 2, 3, 4, 5));

ALTER TABLE order_returns
    ADD COLUMN IF NOT EXISTS return_shipping_provider VARCHAR(50),
    ADD COLUMN IF NOT EXISTS return_shipping_code     VARCHAR(100),
    ADD COLUMN IF NOT EXISTS return_shipping_note     TEXT,
    ADD COLUMN IF NOT EXISTS return_bank_name         VARCHAR(100),
    ADD COLUMN IF NOT EXISTS return_bank_account      VARCHAR(50),
    ADD COLUMN IF NOT EXISTS return_bank_account_name VARCHAR(100);

CREATE INDEX IF NOT EXISTS idx_returns_shipping_code ON order_returns(return_shipping_code);

COMMENT ON COLUMN order_returns.return_status IS '1: Pending, 2: Approved, 3: Rejected, 4: Completed, 5: Cancelled';
