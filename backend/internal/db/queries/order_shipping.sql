-- name: UpdateOrderShippingAndStatus :one
UPDATE orders SET
    order_shipping_provider = sqlc.arg(shipping_provider),
    order_shipping_code     = sqlc.arg(shipping_code),
    order_shipping_fee      = sqlc.arg(shipping_fee),
    order_expected_delivery = sqlc.narg(expected_delivery),
    order_status            = sqlc.arg(order_status), -- 3: Shipped
    order_updated_at        = now()
WHERE order_uuid = sqlc.arg(order_uuid)
  AND order_status = 2  -- Optimistic lock: chỉ cập nhật nếu đơn vẫn đang là Confirmed
RETURNING *;

-- name: GetOrderByShippingCode :one
SELECT order_id, order_uuid, order_number, order_status, order_payment_method, order_payment_status, order_shipping_code
FROM orders
WHERE order_shipping_code = $1
LIMIT 1;

-- name: MarkOrderDeliveredByShippingCode :one
UPDATE orders SET
    order_status = 4,
    order_delivered_at = now(),
    order_payment_status = CASE 
        WHEN order_payment_method = 1 THEN 2 -- COD -> Paid (Đã thu tiền)
        ELSE order_payment_status 
    END,
    order_updated_at = now()
WHERE order_shipping_code = sqlc.arg(shipping_code)
  AND order_status = 3 -- Chỉ update nếu đơn đang ở trạng thái Shipped (Idempotent)
RETURNING *;

-- name: CancelOrderShipping :one
UPDATE orders SET
    order_shipping_provider = NULL,
    order_shipping_code     = NULL,
    order_status            = 2, -- Quay về Confirmed (Chờ giao hàng)
    order_updated_at        = now()
WHERE order_uuid = sqlc.arg(order_uuid)
  AND order_status = 3        -- Optimistic lock: Chỉ cho phép hủy khi đơn đang ở trạng thái Shipped
RETURNING *;

-- name: MarkOrderDeliveredByUUID :one
UPDATE orders SET
    order_status = 4,
    order_delivered_at = now(),
    order_payment_status = CASE 
        WHEN order_payment_method = 1 THEN 2 -- COD -> Paid (Đã thu tiền)
        ELSE order_payment_status 
    END,
    order_updated_at = now()
WHERE order_uuid = sqlc.arg(order_uuid)
  AND order_status = 3 -- Chỉ update nếu đơn đang ở trạng thái Shipped
RETURNING *;

-- name: GetActiveShippedOrders :many
SELECT * FROM orders
WHERE order_status = 3
  AND order_shipping_code IS NOT NULL
  AND order_shipping_code != ''
  AND order_cancelled_at IS NULL
ORDER BY order_updated_at ASC
LIMIT $1;

