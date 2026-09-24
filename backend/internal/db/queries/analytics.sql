-- name: GetOverviewStats :one
WITH paid_orders AS (
    SELECT
        o.order_id,
        (o.order_total_amount - o.order_refunded_amount)::numeric(14,2) AS net_revenue,
        COALESCE(o.order_delivered_at, o.order_updated_at, o.order_created_at) AS paid_time
    FROM orders o
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
    s.revenue,
    s.order_count,
    s.prev_revenue,
    s.prev_order_count,
    po.pending_count,
    c.new_customers,
    c.prev_new_customers
FROM order_stats s
CROSS JOIN pending_orders po
CROSS JOIN customer_stats c;

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
LEFT JOIN orders o
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
LEFT JOIN orders o
    ON o.order_payment_status IN (2, 3)
   AND date_trunc('month', COALESCE(o.order_delivered_at, o.order_updated_at, o.order_created_at)) = gs.month
GROUP BY gs.month
ORDER BY gs.month ASC;

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
WHERE o.order_payment_status IN (2, 3)
  AND COALESCE(o.order_delivered_at, o.order_updated_at, o.order_created_at) >= NOW() - (sqlc.arg('days')::int * INTERVAL '1 day')
  AND p.product_deleted_at IS NULL
GROUP BY p.product_id, p.product_uuid, p.product_name, p.product_slug, p.product_price, p.product_image_url, c.category_name
ORDER BY total_units_sold DESC, total_sales_amount DESC
LIMIT sqlc.arg('limit_count')::int;


-- name: GetRecentOrders :many
-- Operational list view, not revenue-related; unchanged.
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

-- name: GetOrderStatusDistribution :many
SELECT 
    order_status,
    COUNT(*)::bigint AS count,
    COALESCE(SUM(order_total_amount), 0)::numeric(14,2) AS total_amount
FROM orders
WHERE order_created_at >= sqlc.arg('from_date')
  AND order_created_at <= sqlc.arg('to_date')
GROUP BY order_status
ORDER BY order_status ASC;

-- name: GetLowStockProducts :many
SELECT 
    p.product_id,
    p.product_uuid,
    p.product_name,
    p.product_slug,
    p.product_image_url,
    p.product_stock,
    p.product_price,
    c.category_name
FROM products p
LEFT JOIN categories c ON p.product_category_id = c.category_id
WHERE p.product_stock <= sqlc.arg('threshold')::int
  AND p.product_status = 1
ORDER BY p.product_stock ASC
LIMIT sqlc.arg('limit')::int;

-- name: GetPaymentMethodBreakdown :many
SELECT 
    order_payment_method,
    COUNT(*)::bigint AS order_count,
    COALESCE(SUM(order_total_amount - order_refunded_amount), 0)::numeric(14,2) AS net_revenue
FROM orders
WHERE order_payment_status IN (2, 3)
  AND order_created_at >= sqlc.arg('from_date')
  AND order_created_at <= sqlc.arg('to_date')
GROUP BY order_payment_method
ORDER BY net_revenue DESC;

-- name: GetOverviewStatsCustom :one
WITH paid_orders AS (
    SELECT
        o.order_id,
        (o.order_total_amount - o.order_refunded_amount)::numeric(14,2) AS net_revenue,
        COALESCE(o.order_delivered_at, o.order_updated_at, o.order_created_at)::timestamptz AS paid_time
    FROM orders o
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
    FROM orders
    WHERE order_status = 1
),
customer_stats AS (
    SELECT
        COUNT(user_id) FILTER (
            WHERE user_created_at >= sqlc.arg('from_date')::timestamptz AND user_created_at <= sqlc.arg('to_date')::timestamptz
        )::bigint AS new_customers,
        COUNT(user_id) FILTER (
            WHERE user_created_at >= sqlc.arg('prev_from_date')::timestamptz AND user_created_at < sqlc.arg('from_date')::timestamptz
        )::bigint AS prev_new_customers
    FROM users
    WHERE user_level = 3
      AND user_deleted_at IS NULL
      AND user_created_at >= sqlc.arg('prev_from_date')::timestamptz
      AND user_created_at <= sqlc.arg('to_date')::timestamptz
)
SELECT
    s.revenue,
    s.order_count,
    s.prev_revenue,
    s.prev_order_count,
    po.pending_count,
    c.new_customers,
    c.prev_new_customers
FROM order_stats s
CROSS JOIN pending_orders po
CROSS JOIN customer_stats c;

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
LEFT JOIN orders o
    ON o.order_payment_status IN (2, 3)
   AND date_trunc('day', COALESCE(o.order_delivered_at, o.order_updated_at, o.order_created_at)) = gs.day
GROUP BY gs.day
ORDER BY gs.day ASC;

-- name: GetTopSellingProductsCustom :many
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
WHERE o.order_payment_status IN (2, 3)
  AND COALESCE(o.order_delivered_at, o.order_updated_at, o.order_created_at) >= sqlc.arg('from_date')
  AND COALESCE(o.order_delivered_at, o.order_updated_at, o.order_created_at) <= sqlc.arg('to_date')
  AND p.product_deleted_at IS NULL
GROUP BY p.product_id, p.product_uuid, p.product_name, p.product_slug, p.product_price, p.product_image_url, c.category_name
ORDER BY total_units_sold DESC, total_sales_amount DESC
LIMIT sqlc.arg('limit_count')::int;

-- name: GetCartAbandonmentAndFunnelStats :one
WITH active_carts AS ( 
    SELECT 
        c.cart_id,
        c.cart_user_id,
        c.cart_updated_at,
        COALESCE(SUM(ci.cart_item_quantity * p.product_price), 0)::numeric(14,2) AS cart_value
    FROM carts c
    JOIN cart_items ci ON ci.cart_item_cart_id = c.cart_id
    JOIN products p ON p.product_id = ci.cart_item_product_id
    WHERE c.cart_updated_at >= sqlc.arg('from_date')::timestamptz 
      AND c.cart_updated_at <= sqlc.arg('to_date')::timestamptz
    GROUP BY c.cart_id, c.cart_user_id, c.cart_updated_at
),
abandoned AS (
    SELECT 
        ac.cart_id,
        ac.cart_value
    FROM active_carts ac
    WHERE ac.cart_updated_at < NOW() - INTERVAL '24 hours'
      AND NOT EXISTS (
          SELECT 1 FROM orders o
          WHERE o.order_user_id = ac.cart_user_id
            AND o.order_created_at >= ac.cart_updated_at
      )
),
order_funnel AS (
    SELECT
        COUNT(order_id)::bigint AS orders_created,
        COUNT(order_id) FILTER (WHERE order_payment_status IN (2, 3))::bigint AS orders_paid
    FROM orders
    WHERE order_created_at >= sqlc.arg('from_date')::timestamptz 
      AND order_created_at <= sqlc.arg('to_date')::timestamptz
)
SELECT
    COALESCE((SELECT COUNT(*) FROM active_carts), 0)::bigint AS total_carts,
    COALESCE(of.orders_created, 0)::bigint AS orders_created,
    COALESCE(of.orders_paid, 0)::bigint AS orders_paid,
    COALESCE((SELECT COUNT(*) FROM abandoned), 0)::bigint AS abandoned_carts,
    COALESCE((SELECT SUM(cart_value) FROM abandoned), 0)::numeric(14,2) AS potential_lost_revenue
FROM order_funnel of;

-- name: GetReturnAnalyticsSummary :one
WITH range_returns AS (
    SELECT 
        r.return_id,
        r.return_order_id
    FROM order_returns r
    WHERE r.return_created_at >= sqlc.arg('from_date')::timestamptz
      AND r.return_created_at <= sqlc.arg('to_date')::timestamptz
),
range_orders AS (
    SELECT
        COUNT(o.order_id)::bigint AS total_paid_orders,
        COALESCE(SUM(o.order_refunded_amount), 0)::numeric(14,2) AS total_refunded_amount
    FROM orders o
    WHERE o.order_payment_status IN (2, 3, 4)
      AND o.order_created_at >= sqlc.arg('from_date')::timestamptz
      AND o.order_created_at <= sqlc.arg('to_date')::timestamptz
)
SELECT
    COALESCE((SELECT COUNT(*) FROM range_returns), 0)::bigint AS total_returns,
    COALESCE((SELECT COUNT(DISTINCT return_order_id) FROM range_returns), 0)::bigint AS returned_orders_count,
    ro.total_paid_orders,
    ro.total_refunded_amount
FROM range_orders ro;

-- name: GetReturnReasonsBreakdown :many
SELECT
    r.return_reason,
    COUNT(r.return_id)::bigint AS reason_count
FROM order_returns r
WHERE r.return_created_at >= sqlc.arg('from_date')::timestamptz
  AND r.return_created_at <= sqlc.arg('to_date')::timestamptz
GROUP BY r.return_reason
ORDER BY reason_count DESC
LIMIT sqlc.arg('limit_count')::int;

-- name: GetTopReturnedProducts :many
SELECT
    p.product_id,
    p.product_uuid,
    p.product_name,
    p.product_slug,
    p.product_price,
    p.product_image_url,
    COUNT(DISTINCT r.return_id)::bigint AS return_case_count,
    COALESCE(SUM(oi.order_item_quantity), 0)::bigint AS returned_units_count,
    COALESCE(SUM(oi.order_item_subtotal), 0)::numeric(14,2) AS returned_value
FROM order_returns r
JOIN order_items oi ON oi.order_item_order_id = r.return_order_id
JOIN products p ON p.product_id = oi.order_item_product_id
WHERE r.return_created_at >= sqlc.arg('from_date')::timestamptz
  AND r.return_created_at <= sqlc.arg('to_date')::timestamptz
  AND p.product_deleted_at IS NULL
GROUP BY p.product_id, p.product_uuid, p.product_name, p.product_slug, p.product_price, p.product_image_url
ORDER BY return_case_count DESC, returned_units_count DESC
LIMIT sqlc.arg('limit_count')::int;

