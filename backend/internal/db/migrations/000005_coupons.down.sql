-- Delete the columns that were added to the orders table (in reverse order)
ALTER TABLE orders
    DROP COLUMN IF EXISTS order_coupon_id,
    DROP COLUMN IF EXISTS order_coupon_code,
    DROP COLUMN IF EXISTS order_subtotal_amount,
    DROP COLUMN IF EXISTS order_discount_amount;

DROP INDEX IF EXISTS uq_coupon_usages_coupon_order;

DROP TRIGGER IF EXISTS set_coupon_updated_at ON coupons;
DROP FUNCTION IF EXISTS update_coupon_updated_at();

-- Delete tables in the order of dependencies (child table coupon_usages first, parent table coupons second).
DROP TABLE IF EXISTS coupon_usages;
DROP TABLE IF EXISTS coupons;
