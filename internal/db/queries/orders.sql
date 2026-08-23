-- name: CreateOrder :one
INSERT INTO orders (
    order_number, order_user_id,
    order_shipping_name, order_shipping_phone, order_shipping_address,
    order_total_amount,
    order_payment_method, order_payment_status,
    order_note, order_status
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetLatestOrderNumber :one
SELECT order_number FROM orders
WHERE order_number LIKE $1
ORDER BY order_id DESC
LIMIT 1;