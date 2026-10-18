-- Migration 000022 rollback
DROP VIEW IF EXISTS orders.order_sagas;
DROP INDEX IF EXISTS idx_order_sagas_active;
DROP TABLE IF EXISTS order_sagas;
