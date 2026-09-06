ALTER TABLE order_returns DROP CONSTRAINT IF EXISTS order_returns_return_status_check;

ALTER TABLE order_returns ADD CONSTRAINT order_returns_return_status_check 
    CHECK (return_status IN (1, 2, 3, 4));

DROP INDEX IF EXISTS idx_returns_shipping_code;

ALTER TABLE order_returns
    DROP COLUMN IF EXISTS return_shipping_provider,
    DROP COLUMN IF EXISTS return_shipping_code,
    DROP COLUMN IF EXISTS return_shipping_note,
    DROP COLUMN IF EXISTS return_bank_name,
    DROP COLUMN IF EXISTS return_bank_account,
    DROP COLUMN IF EXISTS return_bank_account_name;
