-- Migration 000020: Add idempotency constraints for stock movements and coupon usages

-- 1. Backfill any missing UUIDs in stock_movements
UPDATE stock_movements sm
SET product_uuid = p.product_uuid
FROM products p
WHERE sm.product_id = p.product_id
  AND sm.product_uuid IS NULL;

-- 2. Backfill any missing UUIDs in coupon_usages
UPDATE coupon_usages cu
SET 
    coupon_usage_order_uuid = o.order_uuid,
    coupon_usage_user_uuid = u.user_uuid,
    coupon_usage_coupon_uuid = c.coupon_uuid
FROM orders o, users u, coupons c
WHERE cu.coupon_usage_order_id = o.order_id
  AND cu.coupon_usage_user_id = u.user_id
  AND cu.coupon_usage_coupon_id = c.coupon_id
  AND (cu.coupon_usage_order_uuid IS NULL OR cu.coupon_usage_coupon_uuid IS NULL OR cu.coupon_usage_user_uuid IS NULL);

-- 3. Idempotency unique indexes for stock_movements
CREATE UNIQUE INDEX IF NOT EXISTS uq_stock_movements_idempotency
    ON stock_movements (reference_type, reference_id, product_id)
    WHERE reference_type IS NOT NULL AND reference_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_stock_movements_idempotency_uuid
    ON stock_movements (reference_type, reference_id, product_uuid)
    WHERE reference_type IS NOT NULL AND reference_id IS NOT NULL AND product_uuid IS NOT NULL;

-- 4. Idempotency unique indexes for coupon_usages
CREATE UNIQUE INDEX IF NOT EXISTS uq_coupon_usages_order_uuid
    ON coupon_usages (coupon_usage_order_uuid)
    WHERE coupon_usage_order_uuid IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_coupon_usages_order_id
    ON coupon_usages (coupon_usage_order_id)
    WHERE coupon_usage_order_id IS NOT NULL;
