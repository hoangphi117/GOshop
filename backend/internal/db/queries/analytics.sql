-- name: GetOverviewStats :one
WITH order_stats AS (
    SELECT
        COALESCE(SUM(order_total_amount) FILTER (
            WHERE order_created_at >= NOW() - INTERVAL '30 days'
        ), 0)::numeric(14,2) AS revenue,
        COUNT(order_id) FILTER (
            WHERE order_created_at >= NOW() - INTERVAL '30 days'
        )::bigint AS order_count,
        COALESCE(SUM(order_total_amount) FILTER (
            WHERE order_created_at >= NOW() - INTERVAL '60 days'
              AND order_created_at < NOW() - INTERVAL '30 days'
        ), 0)::numeric(14,2) AS prev_revenue,
        COUNT(order_id) FILTER (
            WHERE order_created_at >= NOW() - INTERVAL '60 days'
              AND order_created_at < NOW() - INTERVAL '30 days'
        )::bigint AS prev_order_count
    FROM orders
    WHERE order_status IN (2, 3, 4)
      AND order_created_at >= NOW() - INTERVAL '60 days'
),
pending_orders AS (
    SELECT COUNT(order_id)::bigint AS pending_count
    FROM orders
    WHERE order_status = 1
),
customer_stats AS (
    SELECT
        COUNT(user_id) FILTER (
            WHERE user_created_at >= NOW() - INTERVAL '30 days'
        )::bigint AS new_customers,
        COUNT(user_id) FILTER (
            WHERE user_created_at >= NOW() - INTERVAL '60 days'
              AND user_created_at < NOW() - INTERVAL '30 days'
        )::bigint AS prev_new_customers
    FROM users
    WHERE user_level = 3
      AND user_deleted_at IS NULL
      AND user_created_at >= NOW() - INTERVAL '60 days'
)
SELECT
    o.revenue,
    o.order_count,
    o.prev_revenue,
    o.prev_order_count,
    po.pending_count,
    c.new_customers,
    c.prev_new_customers
FROM order_stats o
CROSS JOIN pending_orders po
CROSS JOIN customer_stats c;

-- name: GetRevenueDailyChart :many
SELECT
    gs.day::timestamptz AS date,
    COALESCE(SUM(o.order_total_amount), 0)::numeric(14,2) AS total_revenue,
    COUNT(o.order_id)::bigint AS total_orders
FROM generate_series(
    date_trunc('day', NOW() - ((sqlc.arg('days')::int - 1) * INTERVAL '1 day')),
    date_trunc('day', NOW()),
    INTERVAL '1 day'
) AS gs(day)
LEFT JOIN orders o
    ON date_trunc('day', o.order_created_at) = gs.day
   AND o.order_status IN (2, 3, 4)
GROUP BY gs.day
ORDER BY gs.day ASC;

-- name: GetRevenueMonthlyChart :many
SELECT
    gs.month::timestamptz AS month,
    COALESCE(SUM(o.order_total_amount), 0)::numeric(14,2) AS total_revenue,
    COUNT(o.order_id)::bigint AS total_orders
FROM generate_series(
    date_trunc('month', NOW() - ((sqlc.arg('months')::int - 1) * INTERVAL '1 month')),
    date_trunc('month', NOW()),
    INTERVAL '1 month'
) AS gs(month)
LEFT JOIN orders o
    ON date_trunc('month', o.order_created_at) = gs.month
   AND o.order_status IN (2, 3, 4)
GROUP BY gs.month
ORDER BY gs.month ASC;

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
