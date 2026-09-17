-- name: GetOverviewStats :one
WITH current_orders AS (
    SELECT
        COALESCE(SUM(order_total_amount), 0)::numeric(14,2) AS revenue,
        COUNT(order_id)::bigint AS order_count
    FROM orders
    WHERE order_status IN (2, 3, 4) -- Confirmed, Shipped, Delivered
      AND order_created_at >= NOW() - INTERVAL '30 days'
),
previous_orders AS (
    SELECT
        COALESCE(SUM(order_total_amount), 0)::numeric(14,2) AS prev_revenue,
        COUNT(order_id)::bigint AS prev_order_count
    FROM orders
    WHERE order_status IN (2, 3, 4)
      AND order_created_at >= NOW() - INTERVAL '60 days'
      AND order_created_at < NOW() - INTERVAL '30 days'
),
pending_orders AS (
    SELECT COUNT(order_id)::bigint AS pending_count
    FROM orders
    WHERE order_status = 1 -- Pending
),
current_customers AS (
    SELECT COUNT(user_id)::bigint AS new_customers
    FROM users
    WHERE user_level = 3 -- Member
      AND user_deleted_at IS NULL
      AND user_created_at >= NOW() - INTERVAL '30 days'
),
previous_customers AS (
    SELECT COUNT(user_id)::bigint AS prev_new_customers
    FROM users
    WHERE user_level = 3
      AND user_deleted_at IS NULL
      AND user_created_at >= NOW() - INTERVAL '60 days'
      AND user_created_at < NOW() - INTERVAL '30 days'
)
SELECT
    c.revenue,
    c.order_count,
    p.prev_revenue,
    p.prev_order_count,
    po.pending_count,
    cc.new_customers,
    pc.prev_new_customers
FROM current_orders c
CROSS JOIN previous_orders p
CROSS JOIN pending_orders po
CROSS JOIN current_customers cc
CROSS JOIN previous_customers pc;

-- name: GetRevenueDailyChart :many
SELECT
    date_trunc('day', order_created_at)::timestamptz AS date,
    COALESCE(SUM(order_total_amount), 0)::numeric(14,2) AS total_revenue,
    COUNT(order_id)::bigint AS total_orders
FROM orders
WHERE order_status IN (2, 3, 4)
  AND order_created_at >= NOW() - (sqlc.arg('days')::int * INTERVAL '1 day')
GROUP BY date_trunc('day', order_created_at)
ORDER BY date ASC;

-- name: GetRevenueMonthlyChart :many
SELECT
    date_trunc('month', order_created_at)::timestamptz AS month,
    COALESCE(SUM(order_total_amount), 0)::numeric(14,2) AS total_revenue,
    COUNT(order_id)::bigint AS total_orders
FROM orders
WHERE order_status IN (2, 3, 4)
  AND order_created_at >= NOW() - (sqlc.arg('months')::int * INTERVAL '1 month')
GROUP BY date_trunc('month', order_created_at)
ORDER BY month ASC;

-- name: GetRecentOrders :many
SELECT
    o.order_id,
    o.order_uuid,
    o.order_number,
    o.order_shipping_name,
    o.order_total_amount,
    o.order_payment_method,
    o.order_payment_status,
    o.order_status,
    o.order_created_at,
    u.user_uuid,
    u.user_fullname,
    u.user_email
FROM orders o
JOIN users u ON u.user_id = o.order_user_id
ORDER BY o.order_created_at DESC
LIMIT sqlc.arg('limit_count')::int;

-- name: GetTopSellingProducts :many
SELECT
    p.product_id,
    p.product_uuid,
    p.product_name,
    p.product_slug,
    p.product_price,
    p.product_image_url,
    c.category_name,
    COALESCE(SUM(oi.order_item_quantity), 0)::bigint AS total_units_sold,
    COALESCE(SUM(oi.order_item_subtotal), 0)::numeric(14,2) AS total_sales_amount
FROM order_items oi
JOIN orders o ON o.order_id = oi.order_item_order_id
JOIN products p ON p.product_id = oi.order_item_product_id
JOIN categories c ON c.category_id = p.product_category_id
WHERE o.order_status IN (2, 3, 4)
  AND o.order_created_at >= NOW() - (sqlc.arg('days')::int * INTERVAL '1 day')
  AND p.product_deleted_at IS NULL
GROUP BY p.product_id, p.product_uuid, p.product_name, p.product_slug, p.product_price, p.product_image_url, c.category_name
ORDER BY total_units_sold DESC, total_sales_amount DESC
LIMIT sqlc.arg('limit_count')::int;
