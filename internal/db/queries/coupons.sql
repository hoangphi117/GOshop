-- name: CreateCoupon :one
INSERT INTO coupons (
    coupon_code, coupon_name, coupon_description, coupon_type,
    coupon_discount_value, coupon_max_discount_amount, coupon_min_order_amount,
    coupon_usage_limit, coupon_user_usage_limit, coupon_start_date, coupon_end_date, coupon_is_active
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
) RETURNING *;

-- name: GetCouponByCode :one
SELECT * FROM coupons
WHERE coupon_code = $1
AND coupon_deleted_at IS NULL;