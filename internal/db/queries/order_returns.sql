-- name: CreateOrderReturn :one
INSERT INTO order_returns (
    return_order_id,
    return_user_id,
    return_reason,
    return_images,
    return_video_url,
    return_status
) VALUES (
    $1, $2, $3, $4, $5, 1 -- 1: Pending
)
RETURNING *;

-- name: GetActiveReturnByOrderID :one
SELECT * FROM order_returns
WHERE return_order_id = $1 
  AND return_status IN (1, 2) -- 1: Pending, 2: Approved
LIMIT 1;

-- name: GetLatestReturnByOrderID :one
SELECT * FROM order_returns
WHERE return_order_id = $1
ORDER BY return_created_at DESC
LIMIT 1;

-- name: GetReturnByUUID :one
SELECT * FROM order_returns
WHERE return_uuid = $1;

-- name: AdminGetReturns :many
SELECT 
    r.return_id, r.return_uuid, r.return_order_id, r.return_user_id,
    r.return_reason, r.return_images, r.return_video_url, r.return_status,
    r.return_admin_note, r.return_created_at, r.return_updated_at,
    o.order_uuid, o.order_number, o.order_shipping_name, o.order_shipping_phone,
    o.order_total_amount, o.order_payment_method, o.order_payment_status,
    u.user_uuid, u.user_fullname, u.user_email
FROM order_returns r
JOIN orders o ON o.order_id = r.return_order_id
JOIN users u ON u.user_id = r.return_user_id
WHERE (sqlc.narg('status')::smallint IS NULL OR r.return_status = sqlc.narg('status'))
  AND (sqlc.narg('search')::text IS NULL OR (
      o.order_number ILIKE '%' || sqlc.narg('search') || '%'
      OR o.order_shipping_name ILIKE '%' || sqlc.narg('search') || '%'
      OR o.order_shipping_phone ILIKE '%' || sqlc.narg('search') || '%'
      OR u.user_email ILIKE '%' || sqlc.narg('search') || '%'
  ))
ORDER BY r.return_created_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: AdminCountReturns :one
SELECT COUNT(*)
FROM order_returns r
JOIN orders o ON o.order_id = r.return_order_id
JOIN users u ON u.user_id = r.return_user_id
WHERE (sqlc.narg('status')::smallint IS NULL OR r.return_status = sqlc.narg('status'))
  AND (sqlc.narg('search')::text IS NULL OR (
      o.order_number ILIKE '%' || sqlc.narg('search') || '%'
      OR o.order_shipping_name ILIKE '%' || sqlc.narg('search') || '%'
      OR o.order_shipping_phone ILIKE '%' || sqlc.narg('search') || '%'
      OR u.user_email ILIKE '%' || sqlc.narg('search') || '%'
  ));

-- name: UpdateReturnStatus :one
UPDATE order_returns SET
    return_status     = sqlc.arg('status'),
    return_admin_note = sqlc.narg('admin_note'),
    return_updated_at = now()
WHERE return_uuid = sqlc.arg('return_uuid')
RETURNING *;

-- name: UpdateReturnShippingInfo :one
UPDATE order_returns SET
    return_shipping_provider = $1,
    return_shipping_code     = $2,
    return_shipping_note     = $3,
    return_updated_at        = now()
WHERE return_uuid = $4
RETURNING *;

-- name: GetReturnsByUserID :many
SELECT 
    r.return_id, r.return_uuid, r.return_order_id, r.return_user_id,
    r.return_reason, r.return_images, r.return_video_url, r.return_status,
    r.return_admin_note, r.return_shipping_provider, r.return_shipping_code,
    r.return_shipping_note, r.return_created_at, r.return_updated_at,
    o.order_uuid, o.order_number, o.order_total_amount,
    o.order_payment_method, o.order_payment_status,
    o.order_shipping_name, o.order_shipping_phone
FROM order_returns r
JOIN orders o ON o.order_id = r.return_order_id
WHERE r.return_user_id = sqlc.arg('user_id')
  AND (sqlc.narg('status')::smallint IS NULL OR r.return_status = sqlc.narg('status'))
  AND (sqlc.narg('search')::text IS NULL OR o.order_number ILIKE '%' || sqlc.narg('search') || '%')
ORDER BY r.return_created_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountReturnsByUserID :one
SELECT COUNT(*)
FROM order_returns r
JOIN orders o ON o.order_id = r.return_order_id
WHERE r.return_user_id = sqlc.arg('user_id')
  AND (sqlc.narg('status')::smallint IS NULL OR r.return_status = sqlc.narg('status'))
  AND (sqlc.narg('search')::text IS NULL OR o.order_number ILIKE '%' || sqlc.narg('search') || '%');

-- name: AdminGetReturnByUUID :one
SELECT 
    r.return_id, r.return_uuid, r.return_order_id, r.return_user_id,
    r.return_reason, r.return_images, r.return_video_url, r.return_status,
    r.return_admin_note, r.return_shipping_provider, r.return_shipping_code,
    r.return_shipping_note, r.return_bank_name, r.return_bank_account,
    r.return_bank_account_name, r.return_created_at, r.return_updated_at,
    o.order_id, o.order_uuid, o.order_number, o.order_total_amount, o.order_refunded_amount,
    o.order_payment_method, o.order_payment_status, o.order_status,
    o.order_shipping_name, o.order_shipping_phone, o.order_shipping_address,
    o.order_created_at,
    u.user_uuid, u.user_fullname, u.user_email
FROM order_returns r
JOIN orders o ON o.order_id = r.return_order_id
JOIN users u ON u.user_id = r.return_user_id
WHERE r.return_uuid = $1;

-- name: AdminGetLatestReturnByOrderID :one
SELECT 
    r.return_id, r.return_uuid, r.return_order_id, r.return_user_id,
    r.return_reason, r.return_images, r.return_video_url, r.return_status,
    r.return_admin_note, r.return_shipping_provider, r.return_shipping_code,
    r.return_shipping_note, r.return_bank_name, r.return_bank_account,
    r.return_bank_account_name, r.return_created_at, r.return_updated_at,
    o.order_id, o.order_uuid, o.order_number, o.order_total_amount, o.order_refunded_amount,
    o.order_payment_method, o.order_payment_status, o.order_status,
    o.order_shipping_name, o.order_shipping_phone, o.order_shipping_address,
    o.order_created_at,
    u.user_uuid, u.user_fullname, u.user_email
FROM order_returns r
JOIN orders o ON o.order_id = r.return_order_id
JOIN users u ON u.user_id = r.return_user_id
WHERE r.return_order_id = $1
ORDER BY r.return_created_at DESC
LIMIT 1;

-- name: UpdateReturnBankAccount :one
UPDATE order_returns SET
    return_bank_name         = $1,
    return_bank_account      = $2,
    return_bank_account_name = $3,
    return_updated_at        = now()
WHERE return_uuid = $4
RETURNING *;


