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
