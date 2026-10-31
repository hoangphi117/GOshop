-- Analytics service projection queries.
--
-- The analytics database (shopping_analytics) owns only the projection tables:
-- analytics_orders_projection and analytics_order_items_projection. External
-- dimension tables (users, products) live in other services and are read over
-- gRPC at composition time in the service layer.

-- name: GetOverviewStats :one
-- Orders/revenue half of the overview KPI. New customers are counted in the
-- identity service and composed in Go.
WITH paid_orders AS (
    SELECT
        o.order_id,
        (o.order_total_amount - o.order_refunded_amount)::numeric(14,2) AS net_revenue,
        COALESCE(o.order_delivered_at, o.order_updated_at, o.order_created_at) AS paid_time
    FROM analytics_orders_projection o
    WHERE o.order_payment_status IN (2, 3) -- Paid or Partially Refunded
      AND COALESCE(o.order_delivered_at, o.order_updated_at, o.order_created_at) >= NOW() - INTERVAL '60 days'
),
order_stats AS (
    SELECT
        COALESCE(SUM(net_revenue) FILTER (
            WHERE paid_time >= NOW() - INTERVAL '30 days'
        ), 0)::numeric(14,2) AS revenue,
        COUNT(order_id) FILTER (
            WHERE paid_time >= NOW() - INTERVAL '30 days'
        )::bigint AS order_count,
        COALESCE(SUM(net_revenue) FILTER (
            WHERE paid_time < NOW() - INTERVAL '30 days'
        ), 0)::numeric(14,2) AS prev_revenue,
        COUNT(order_id) FILTER (
            WHERE paid_time < NOW() - INTERVAL '30 days'
        )::bigint AS prev_order_count
    FROM paid_orders
),
pending_orders AS (
    SELECT COUNT(order_id)::bigint AS pending_count
    FROM analytics_orders_projection
    WHERE order_status = 1
)
SELECT
    s.revenue,
    s.order_count,
    s.prev_revenue,
    s.prev_order_count,
    po.pending_count
FROM order_stats s
CROSS JOIN pending_orders po;

-- name: GetOverviewStatsCustom :one
WITH paid_orders AS (
    SELECT
        o.order_id,
        (o.order_total_amount - o.order_refunded_amount)::numeric(14,2) AS net_revenue,
        COALESCE(o.order_delivered_at, o.order_updated_at, o.order_created_at)::timestamptz AS paid_time
    FROM analytics_orders_projection o
    WHERE o.order_payment_status IN (2, 3)
      AND COALESCE(o.order_delivered_at, o.order_updated_at, o.order_created_at) >= sqlc.arg('prev_from_date')::timestamptz
      AND COALESCE(o.order_delivered_at, o.order_updated_at, o.order_created_at) <= sqlc.arg('to_date')::timestamptz
),
order_stats AS (
    SELECT
        COALESCE(SUM(net_revenue) FILTER (
            WHERE paid_time >= sqlc.arg('from_date')::timestamptz AND paid_time <= sqlc.arg('to_date')::timestamptz
        ), 0)::numeric(14,2) AS revenue,
        COUNT(order_id) FILTER (
            WHERE paid_time >= sqlc.arg('from_date')::timestamptz AND paid_time <= sqlc.arg('to_date')::timestamptz
        )::bigint AS order_count,
        COALESCE(SUM(net_revenue) FILTER (
            WHERE paid_time >= sqlc.arg('prev_from_date')::timestamptz AND paid_time < sqlc.arg('from_date')::timestamptz
        ), 0)::numeric(14,2) AS prev_revenue,
        COUNT(order_id) FILTER (
            WHERE paid_time >= sqlc.arg('prev_from_date')::timestamptz AND paid_time < sqlc.arg('from_date')::timestamptz
        )::bigint AS prev_order_count
    FROM paid_orders
),
pending_orders AS (
    SELECT COUNT(order_id)::bigint AS pending_count
    FROM analytics_orders_projection
    WHERE order_status = 1
)
SELECT
    s.revenue,
    s.order_count,
    s.prev_revenue,
    s.prev_order_count,
    po.pending_count
FROM order_stats s
CROSS JOIN pending_orders po;

-- name: GetRevenueDailyChart :many
SELECT
    gs.day::timestamptz AS date,
    COALESCE(SUM(o.order_total_amount - o.order_refunded_amount), 0)::numeric(14,2) AS total_revenue,
    COUNT(o.order_id)::bigint AS total_orders
FROM generate_series(
    date_trunc('day', NOW() - ((sqlc.arg('days')::int - 1) * INTERVAL '1 day')),
    date_trunc('day', NOW()),
    INTERVAL '1 day'
) AS gs(day)
LEFT JOIN analytics_orders_projection o
    ON o.order_payment_status IN (2, 3)
   AND date_trunc('day', COALESCE(o.order_delivered_at, o.order_updated_at, o.order_created_at)) = gs.day
GROUP BY gs.day
ORDER BY gs.day ASC;

-- name: GetRevenueMonthlyChart :many
SELECT
    gs.month::timestamptz AS month,
    COALESCE(SUM(o.order_total_amount - o.order_refunded_amount), 0)::numeric(14,2) AS total_revenue,
    COUNT(o.order_id)::bigint AS total_orders
FROM generate_series(
    date_trunc('month', NOW() - ((sqlc.arg('months')::int - 1) * INTERVAL '1 month')),
    date_trunc('month', NOW()),
    INTERVAL '1 month'
) AS gs(month)
LEFT JOIN analytics_orders_projection o
    ON o.order_payment_status IN (2, 3)
   AND date_trunc('month', COALESCE(o.order_delivered_at, o.order_updated_at, o.order_created_at)) = gs.month
GROUP BY gs.month
ORDER BY gs.month ASC;

-- name: GetRevenueDailyCustom :many
SELECT
    gs.day::timestamptz AS date,
    COALESCE(SUM(o.order_total_amount - o.order_refunded_amount), 0)::numeric(14,2) AS total_revenue,
    COUNT(o.order_id)::bigint AS total_orders
FROM generate_series(
    date_trunc('day', sqlc.arg('from_date')::timestamptz),
    date_trunc('day', sqlc.arg('to_date')::timestamptz),
    INTERVAL '1 day'
) AS gs(day)
LEFT JOIN analytics_orders_projection o
    ON o.order_payment_status IN (2, 3)
   AND date_trunc('day', COALESCE(o.order_delivered_at, o.order_updated_at, o.order_created_at)) = gs.day
GROUP BY gs.day
ORDER BY gs.day ASC;

-- name: GetTopSellingProducts :many
-- Product dimension (uuid/name/slug/price/image/category) is filled in Go via
-- the catalog GetProductsBatch RPC; only ids and sales aggregates live here.
SELECT
    oi.order_item_product_id AS product_id,
    COALESCE(SUM(oi.order_item_quantity), 0)::bigint AS total_units_sold,
    COALESCE(SUM(oi.order_item_subtotal), 0)::numeric(14,2) AS total_sales_amount
FROM analytics_order_items_projection oi
JOIN analytics_orders_projection o ON o.order_id = oi.order_item_order_id
WHERE o.order_payment_status IN (2, 3)
  AND COALESCE(o.order_delivered_at, o.order_updated_at, o.order_created_at) >= NOW() - (sqlc.arg('days')::int * INTERVAL '1 day')
GROUP BY oi.order_item_product_id
ORDER BY total_units_sold DESC, total_sales_amount DESC
LIMIT sqlc.arg('limit_count')::int;

-- name: GetTopSellingProductsCustom :many
SELECT
    oi.order_item_product_id AS product_id,
    COALESCE(SUM(oi.order_item_quantity), 0)::bigint AS total_units_sold,
    COALESCE(SUM(oi.order_item_subtotal), 0)::numeric(14,2) AS total_sales_amount
FROM analytics_order_items_projection oi
JOIN analytics_orders_projection o ON o.order_id = oi.order_item_order_id
WHERE o.order_payment_status IN (2, 3)
  AND COALESCE(o.order_delivered_at, o.order_updated_at, o.order_created_at) >= sqlc.arg('from_date')::timestamptz
  AND COALESCE(o.order_delivered_at, o.order_updated_at, o.order_created_at) <= sqlc.arg('to_date')::timestamptz
GROUP BY oi.order_item_product_id
ORDER BY total_units_sold DESC, total_sales_amount DESC
LIMIT sqlc.arg('limit_count')::int;

-- name: GetRecentOrders :many
-- Operational list view, not revenue-related; projection-only, unchanged.
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
    o.order_user_uuid AS user_uuid,
    o.order_customer_name AS user_fullname,
    o.order_customer_email AS user_email
FROM analytics_orders_projection o
ORDER BY o.order_created_at DESC
LIMIT sqlc.arg('limit_count')::int;

-- name: GetOrderStatusDistribution :many
SELECT
    order_status,
    COUNT(*)::bigint AS count,
    COALESCE(SUM(order_total_amount), 0)::numeric(14,2) AS total_amount
FROM analytics_orders_projection
WHERE order_created_at >= sqlc.arg('from_date')::timestamptz
  AND order_created_at <= sqlc.arg('to_date')::timestamptz
GROUP BY order_status
ORDER BY order_status ASC;

-- name: GetPaymentMethodBreakdown :many
SELECT
    order_payment_method,
    COUNT(*)::bigint AS order_count,
    COALESCE(SUM(order_total_amount - order_refunded_amount), 0)::numeric(14,2) AS net_revenue
FROM analytics_orders_projection
WHERE order_payment_status IN (2, 3)
  AND order_created_at >= sqlc.arg('from_date')::timestamptz
  AND order_created_at <= sqlc.arg('to_date')::timestamptz
GROUP BY order_payment_method
ORDER BY net_revenue DESC;