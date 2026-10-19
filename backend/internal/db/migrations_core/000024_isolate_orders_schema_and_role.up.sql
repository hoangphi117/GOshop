-- Migration 000024: Isolate order tables into orders schema and setup order_svc role

-- 1. Ensure orders schema exists
CREATE SCHEMA IF NOT EXISTS orders;

-- 2. Drop existing compatibility views inside orders schema so we can move real tables
DROP VIEW IF EXISTS orders.order_sagas CASCADE;
DROP VIEW IF EXISTS orders.order_tracking_events CASCADE;
DROP VIEW IF EXISTS orders.order_returns CASCADE;
DROP VIEW IF EXISTS orders.order_items CASCADE;
DROP VIEW IF EXISTS orders.orders CASCADE;

-- 3. Move Order domain tables to schema orders
ALTER TABLE IF EXISTS public.orders SET SCHEMA orders;
ALTER TABLE IF EXISTS public.order_items SET SCHEMA orders;
ALTER TABLE IF EXISTS public.order_returns SET SCHEMA orders;
ALTER TABLE IF EXISTS public.order_tracking_events SET SCHEMA orders;
ALTER TABLE IF EXISTS public.order_sagas SET SCHEMA orders;

-- 4. Create compatibility views in public schema for backward compatibility (analytics, read models)
CREATE OR REPLACE VIEW public.orders AS SELECT * FROM orders.orders;
CREATE OR REPLACE VIEW public.order_items AS SELECT * FROM orders.order_items;
CREATE OR REPLACE VIEW public.order_returns AS SELECT * FROM orders.order_returns;
CREATE OR REPLACE VIEW public.order_tracking_events AS SELECT * FROM orders.order_tracking_events;
CREATE OR REPLACE VIEW public.order_sagas AS SELECT * FROM orders.order_sagas;

-- 5. Create or configure order_svc role with isolated permissions on orders schema
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'order_svc') THEN
        CREATE ROLE order_svc WITH LOGIN PASSWORD 'order_svc_secret';
    END IF;
END $$;

ALTER ROLE order_svc SET search_path = orders, public;

GRANT USAGE ON SCHEMA orders TO order_svc;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA orders TO order_svc;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA orders TO order_svc;
ALTER DEFAULT PRIVILEGES IN SCHEMA orders GRANT ALL ON TABLES TO order_svc;
ALTER DEFAULT PRIVILEGES IN SCHEMA orders GRANT ALL ON SEQUENCES TO order_svc;

-- Deny access to other tables in public (catalog, users, etc.)
REVOKE ALL ON ALL TABLES IN SCHEMA public FROM order_svc;
GRANT USAGE ON SCHEMA public TO order_svc;
