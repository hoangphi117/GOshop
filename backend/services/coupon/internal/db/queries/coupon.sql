-- name: GetCouponByCode :one
SELECT * FROM coupons
WHERE coupon_code = $1 AND coupon_deleted_at IS NULL;

-- name: GetCouponByCodeForUpdate :one
SELECT * FROM coupons
WHERE coupon_code = $1 AND coupon_deleted_at IS NULL
FOR UPDATE;

-- name: GetCouponByID :one
SELECT * FROM coupons
WHERE coupon_id = $1 AND coupon_deleted_at IS NULL;

-- name: GetCouponByUUID :one
SELECT * FROM coupons
WHERE coupon_uuid = $1 AND coupon_deleted_at IS NULL;

-- name: ListCoupons :many
SELECT * FROM coupons
WHERE coupon_deleted_at IS NULL
  AND (sqlc.narg(search)::text IS NULL OR coupon_code ILIKE '%' || sqlc.narg(search)::text || '%' OR coupon_name ILIKE '%' || sqlc.narg(search)::text || '%')
  AND (sqlc.narg(type)::int IS NULL OR coupon_type = sqlc.narg(type)::int)
  AND (sqlc.narg(is_active)::bool IS NULL OR coupon_is_active = sqlc.narg(is_active)::bool)
ORDER BY coupon_created_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountCoupons :one
SELECT count(*) FROM coupons
WHERE coupon_deleted_at IS NULL
  AND (sqlc.narg(search)::text IS NULL OR coupon_code ILIKE '%' || sqlc.narg(search)::text || '%' OR coupon_name ILIKE '%' || sqlc.narg(search)::text || '%')
  AND (sqlc.narg(type)::int IS NULL OR coupon_type = sqlc.narg(type)::int)
  AND (sqlc.narg(is_active)::bool IS NULL OR coupon_is_active = sqlc.narg(is_active)::bool);

-- name: CreateCoupon :one
INSERT INTO coupons (
    coupon_code,
    coupon_name,
    coupon_description,
    coupon_type,
    coupon_discount_value,
    coupon_max_discount_amount,
    coupon_min_order_amount,
    coupon_usage_limit,
    coupon_user_usage_limit,
    coupon_start_date,
    coupon_end_date,
    coupon_is_active
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
)
RETURNING *;

-- name: UpdateCoupon :one
UPDATE coupons
SET
    coupon_name = COALESCE(sqlc.narg(coupon_name), coupon_name),
    coupon_description = COALESCE(sqlc.narg(coupon_description), coupon_description),
    coupon_type = COALESCE(sqlc.narg(coupon_type), coupon_type),
    coupon_discount_value = COALESCE(sqlc.narg(coupon_discount_value), coupon_discount_value),
    coupon_max_discount_amount = COALESCE(sqlc.narg(coupon_max_discount_amount), coupon_max_discount_amount),
    coupon_min_order_amount = COALESCE(sqlc.narg(coupon_min_order_amount), coupon_min_order_amount),
    coupon_usage_limit = COALESCE(sqlc.narg(coupon_usage_limit), coupon_usage_limit),
    coupon_user_usage_limit = COALESCE(sqlc.narg(coupon_user_usage_limit), coupon_user_usage_limit),
    coupon_start_date = COALESCE(sqlc.narg(coupon_start_date), coupon_start_date),
    coupon_end_date = COALESCE(sqlc.narg(coupon_end_date), coupon_end_date),
    coupon_is_active = COALESCE(sqlc.narg(coupon_is_active), coupon_is_active),
    coupon_updated_at = now()
WHERE coupon_uuid = sqlc.arg(coupon_uuid) AND coupon_deleted_at IS NULL
RETURNING *;

-- name: DeleteCoupon :exec
UPDATE coupons
SET coupon_deleted_at = now(), coupon_updated_at = now()
WHERE coupon_uuid = $1 AND coupon_deleted_at IS NULL;

-- name: CountUserCouponUsages :one
SELECT count(*) FROM coupon_usages
WHERE coupon_usage_coupon_id = $1 AND (coupon_usage_user_id = $2 OR (sqlc.narg(user_uuid)::uuid IS NOT NULL AND coupon_usage_user_uuid = sqlc.narg(user_uuid)::uuid));

-- name: RecordCouponUsage :one
INSERT INTO coupon_usages (
    coupon_usage_coupon_id,
    coupon_usage_coupon_uuid,
    coupon_usage_user_id,
    coupon_usage_user_uuid,
    coupon_usage_order_id,
    coupon_usage_order_uuid
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: DeleteCouponUsageByOrder :exec
DELETE FROM coupon_usages
WHERE (sqlc.narg(order_id)::int IS NOT NULL AND coupon_usage_order_id = sqlc.narg(order_id)::int)
   OR (sqlc.narg(order_uuid)::uuid IS NOT NULL AND coupon_usage_order_uuid = sqlc.narg(order_uuid)::uuid);

-- name: IncrementCouponUsedCount :one
UPDATE coupons
SET coupon_used_count = coupon_used_count + 1, coupon_updated_at = now()
WHERE coupon_id = sqlc.arg(coupon_id)
  AND (coupon_usage_limit IS NULL OR coupon_used_count < coupon_usage_limit)
RETURNING *;

-- name: DecrementCouponUsedCount :one
UPDATE coupons
SET coupon_used_count = GREATEST(0, coupon_used_count - 1), coupon_updated_at = now()
WHERE coupon_id = sqlc.arg(coupon_id)
RETURNING *;
