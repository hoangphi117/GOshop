-- P0: Functional and partial indexes for dashboard analytics query optimization.
--
-- 1. idx_analytics_orders_paid_time: Speeds up net revenue and top selling products
-- filtering by matching the COALESCE() expression on paid orders.
CREATE INDEX IF NOT EXISTS idx_analytics_orders_paid_time
    ON analytics_orders_projection (
        (COALESCE(order_delivered_at, order_updated_at, order_created_at))
    )
    INCLUDE (order_id, order_total_amount, order_refunded_amount)
    WHERE order_payment_status IN (2, 3);

-- 2. idx_analytics_orders_paid_day: Functional index for daily revenue aggregation charts.
CREATE INDEX IF NOT EXISTS idx_analytics_orders_paid_day
    ON analytics_orders_projection (
        (date_trunc('day', COALESCE(order_delivered_at, order_updated_at, order_created_at) AT TIME ZONE 'UTC'))
    )
    WHERE order_payment_status IN (2, 3);

-- 3. idx_analytics_orders_pending: Partial index for pending orders count CTE.
CREATE INDEX IF NOT EXISTS idx_analytics_orders_pending
    ON analytics_orders_projection (order_id)
    WHERE order_status = 1;

-- 4. idx_analytics_order_items_order_product: Covering index on order items for top product aggregation.
CREATE INDEX IF NOT EXISTS idx_analytics_order_items_order_product
    ON analytics_order_items_projection (order_item_order_id)
    INCLUDE (order_item_product_id, order_item_quantity, order_item_subtotal);
