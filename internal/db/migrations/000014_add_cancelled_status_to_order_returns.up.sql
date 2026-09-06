ALTER TABLE order_returns DROP CONSTRAINT IF EXISTS order_returns_return_status_check;
ALTER TABLE order_returns ADD CONSTRAINT order_returns_return_status_check 
    CHECK (return_status IN (1, 2, 3, 4, 5));

COMMENT ON COLUMN order_returns.return_status IS '1: Pending, 2: Approved, 3: Rejected, 4: Completed, 5: Cancelled';
