DROP TABLE IF EXISTS orders.webhook_events CASCADE;
DROP TABLE IF EXISTS orders.outbox_events CASCADE;
DROP TABLE IF EXISTS orders.order_sagas CASCADE;
DROP TABLE IF EXISTS orders.order_returns CASCADE;
DROP TABLE IF EXISTS orders.order_tracking_events CASCADE;
DROP TABLE IF EXISTS orders.order_items CASCADE;
DROP TABLE IF EXISTS orders.orders CASCADE;

DROP FUNCTION IF EXISTS update_return_updated_at CASCADE;
DROP FUNCTION IF EXISTS update_order_updated_at CASCADE;
