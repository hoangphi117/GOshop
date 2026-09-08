-- name: CreateOrderItem :one
INSERT INTO order_items (
    order_item_order_id, order_item_product_id,
    order_item_product_name, order_item_price,
    order_item_quantity, order_item_subtotal
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetOrderItemsByOrderID :many
SELECT
    oi.*,
    p.product_uuid AS product_uuid,
    p.product_slug AS product_slug,
    p.product_image_url AS product_image_url
FROM order_items oi
JOIN products p ON p.product_id = oi.order_item_product_id
WHERE oi.order_item_order_id = $1
ORDER BY oi.order_item_id ASC;
