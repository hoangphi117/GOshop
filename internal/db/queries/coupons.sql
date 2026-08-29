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

-- name: GetCouponByUUID :one
SELECT * FROM coupons
WHERE coupon_uuid = $1
AND coupon_deleted_at IS NULL;

-- name: UpdateCoupon :one
UPDATE coupons
SET
    coupon_name                = COALESCE(sqlc.narg('coupon_name'), coupon_name),
    coupon_description         = COALESCE(sqlc.narg('coupon_description'), coupon_description),
    coupon_discount_value      = COALESCE(sqlc.narg('coupon_discount_value'), coupon_discount_value),
    coupon_max_discount_amount = CASE 
                                    WHEN sqlc.narg('clear_max_discount')::boolean = true THEN NULL 
                                    ELSE COALESCE(sqlc.narg('coupon_max_discount_amount'), coupon_max_discount_amount)
                                 END,
    coupon_min_order_amount    = COALESCE(sqlc.narg('coupon_min_order_amount'), coupon_min_order_amount),
    coupon_usage_limit         = CASE 
                                    WHEN sqlc.narg('clear_usage_limit')::boolean = true THEN NULL 
                                    ELSE COALESCE(sqlc.narg('coupon_usage_limit'), coupon_usage_limit)
                                 END,
    coupon_user_usage_limit    = COALESCE(sqlc.narg('coupon_user_usage_limit'), coupon_user_usage_limit),
    coupon_end_date            = COALESCE(sqlc.narg('coupon_end_date'), coupon_end_date),
    coupon_is_active           = COALESCE(sqlc.narg('coupon_is_active'), coupon_is_active)
WHERE coupon_uuid = sqlc.arg(coupon_uuid)
AND coupon_deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteCouponByUUID :one
UPDATE coupons
SET coupon_deleted_at = now(), coupon_is_active = false
WHERE coupon_uuid = sqlc.arg(coupon_uuid)
AND coupon_deleted_at IS NULL
RETURNING *;

-- name: CountCoupons :one
SELECT count(*)
FROM coupons
WHERE (
    sqlc.narg(deleted)::bool IS NULL
    OR sqlc.narg(deleted)::bool = TRUE AND coupon_deleted_at IS NOT NULL
    OR sqlc.narg(deleted)::bool = FALSE AND coupon_deleted_at IS NULL
) AND (
    sqlc.narg(search)::TEXT IS NULL
    OR sqlc.narg(search)::TEXT = ''
    OR unaccent(coupon_code) ILIKE '%' || unaccent(sqlc.narg(search)::TEXT) || '%'
    OR unaccent(coupon_name) ILIKE '%' || unaccent(sqlc.narg(search)::TEXT) || '%'
) AND (
    sqlc.narg(type)::int IS NULL
    OR coupon_type = sqlc.narg(type)::int
) AND (
    sqlc.narg(is_active)::bool IS NULL
    OR coupon_is_active = sqlc.narg(is_active)::bool
);
