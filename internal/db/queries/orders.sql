-- name: CreateOrder :one
INSERT INTO orders (
    order_number, order_user_id,
    order_shipping_name, order_shipping_phone, order_shipping_address,
    order_total_amount,
    order_payment_method, order_payment_status,
    order_note, order_status
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetOrderByUUID :one
SELECT o.*, u.user_uuid, u.user_fullname, u.user_email
FROM orders o
JOIN users u ON u.user_id = o.order_user_id
WHERE o.order_uuid = $1;


-- name: GetLatestOrderNumber :one
SELECT order_number FROM orders
WHERE order_number LIKE $1
ORDER BY order_id DESC
LIMIT 1;

-- name: UpdateOrderStatus :one
UPDATE orders
SET order_status = sqlc.arg(order_status)
WHERE order_uuid = sqlc.arg(order_uuid)
RETURNING *;

-- name: UpdateOrderShippingInfo :one
UPDATE orders
SET
    order_shipping_name    = COALESCE(sqlc.narg('order_shipping_name')::varchar,    order_shipping_name),
    order_shipping_phone   = COALESCE(sqlc.narg('order_shipping_phone')::varchar,   order_shipping_phone),
    order_shipping_address = COALESCE(sqlc.narg('order_shipping_address')::text,      order_shipping_address),
    order_note             = COALESCE(sqlc.narg('order_note')::text,                  order_note)
WHERE order_uuid = sqlc.arg(order_uuid)
  AND order_status = ANY(sqlc.arg('allowed_statuses')::int[]) -- Check atomic in DB
RETURNING *;



-- name: CancelOrder :one
UPDATE orders
SET
    order_status = 5,
    order_cancelled_at = NOW()
WHERE order_uuid = sqlc.arg(order_uuid)
AND order_status IN (1, 2)
RETURNING *;