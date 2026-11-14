-- Reverse of 000001_schema.up.sql, in dependency order.

DROP INDEX IF EXISTS idx_search_dismissals_subject;
DROP TABLE IF EXISTS search_history_dismissals;
DROP TABLE IF EXISTS user_events;

DROP TABLE IF EXISTS analytics_order_items_projection;
DROP TABLE IF EXISTS analytics_orders_projection;
