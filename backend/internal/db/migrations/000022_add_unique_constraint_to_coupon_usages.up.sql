-- Prevent applying the same coupon to the same order multiple times
CREATE UNIQUE INDEX IF NOT EXISTS uq_coupon_usages_coupon_order 
ON coupon_usages (coupon_usage_coupon_id, coupon_usage_order_id) 
WHERE coupon_usage_order_id IS NOT NULL;
