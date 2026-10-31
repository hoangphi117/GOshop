-- name: CreateOrderItem :one
INSERT INTO order_items (
    order_item_order_id, order_item_product_id,
    order_item_product_name, order_item_price,
    order_item_quantity, order_item_subtotal,
    order_item_product_uuid, order_item_product_slug,
    order_item_product_image_url, order_item_order_uuid
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetOrderItemsByOrderID :many
SELECT
    oi.order_item_id,
    oi.order_item_uuid,
    oi.order_item_order_id,
    oi.order_item_product_id,
    oi.order_item_product_name,
    oi.order_item_price,
    oi.order_item_quantity,
    oi.order_item_subtotal,
    oi.order_item_created_at,
    oi.order_item_product_uuid,
    oi.order_item_product_slug,
    oi.order_item_product_image_url,
    oi.order_item_order_uuid,
    oi.order_item_product_uuid AS product_uuid,
    oi.order_item_product_slug AS product_slug,
    oi.order_item_product_image_url AS product_image_url
FROM order_items oi
WHERE oi.order_item_order_id = $1
ORDER BY oi.order_item_id ASC;

