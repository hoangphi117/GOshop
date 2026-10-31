-- name: GetCouponByCode :one
SELECT * FROM coupons
WHERE coupon_code = $1 AND coupon_deleted_at IS NULL;

-- name: GetCouponByCodeForUpdate :one
SELECT * FROM coupons
WHERE coupon_code = $1 AND coupon_deleted_at IS NULL
FOR UPDATE;

-- name: GetCouponByIDForUpdate :one
SELECT * FROM coupons
WHERE coupon_id = $1 AND coupon_deleted_at IS NULL
FOR UPDATE;

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

-- name: UpsertCouponUsage :one
-- Reservation identity: (coupon, order). A retry of an already-reserved pair
-- returns the existing row with inserted=false so quota is not consumed again.
-- A released (tombstoned) row comes back with status=2 so the service can
-- reject a reservation for a cancelled order.
-- The two CTE branches are complementary: a successful insert is invisible to
-- the `existing` snapshot, and on conflict the insert branch is empty.
WITH ins AS (
    INSERT INTO coupon_usages (
        coupon_usage_coupon_id,
        coupon_usage_coupon_uuid,
        coupon_usage_user_id,
        coupon_usage_user_uuid,
        coupon_usage_order_id,
        coupon_usage_order_uuid
    ) VALUES (
        sqlc.arg(coupon_id), sqlc.arg(coupon_uuid), sqlc.arg(user_id), sqlc.arg(user_uuid), sqlc.arg(order_id), sqlc.arg(order_uuid)
    )
    ON CONFLICT (coupon_usage_coupon_id, coupon_usage_order_uuid) WHERE coupon_usage_order_uuid IS NOT NULL
    DO NOTHING
    RETURNING coupon_usage_id, coupon_usage_uuid, coupon_usage_coupon_id, coupon_usage_coupon_uuid,
              coupon_usage_user_id, coupon_usage_user_uuid, coupon_usage_order_id, coupon_usage_order_uuid,
              coupon_usage_used_at, coupon_usage_status, coupon_usage_released_at, coupon_usage_updated_at,
              TRUE AS inserted
), existing AS (
    SELECT coupon_usage_id, coupon_usage_uuid, coupon_usage_coupon_id, coupon_usage_coupon_uuid,
           coupon_usage_user_id, coupon_usage_user_uuid, coupon_usage_order_id, coupon_usage_order_uuid,
           coupon_usage_used_at, coupon_usage_status, coupon_usage_released_at, coupon_usage_updated_at,
           FALSE AS inserted
    FROM coupon_usages
    WHERE coupon_usage_coupon_id = sqlc.arg(coupon_id)
      AND coupon_usage_order_uuid = sqlc.arg(order_uuid)
)
SELECT * FROM ins
UNION ALL
SELECT * FROM existing
LIMIT 1;

-- name: ReleaseCouponUsage :one
-- Tombstone transition: only a currently-reserved usage flips to released.
-- A redelivered Release (or a Release with no reservation yet) matches zero
-- rows so quota is never refunded twice or refunded for nothing.
UPDATE coupon_usages
SET coupon_usage_status     = 2,
    coupon_usage_released_at = now(),
    coupon_usage_updated_at  = now()
WHERE coupon_usage_status = 1
  AND (
        (sqlc.narg(order_id)::int IS NOT NULL AND coupon_usage_order_id = sqlc.narg(order_id)::int)
     OR (sqlc.narg(order_uuid)::uuid IS NOT NULL AND coupon_usage_order_uuid = sqlc.narg(order_uuid)::uuid)
  )
RETURNING *;

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

