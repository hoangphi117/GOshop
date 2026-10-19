-- Revert Migration 000024: Move order tables back to public schema

-- 1. Drop compatibility views in public
DROP VIEW IF EXISTS public.order_sagas CASCADE;
DROP VIEW IF EXISTS public.order_tracking_events CASCADE;
DROP VIEW IF EXISTS public.order_returns CASCADE;
DROP VIEW IF EXISTS public.order_items CASCADE;
DROP VIEW IF EXISTS public.orders CASCADE;

-- 2. Move tables back to public schema
ALTER TABLE IF EXISTS orders.order_sagas SET SCHEMA public;
ALTER TABLE IF EXISTS orders.order_tracking_events SET SCHEMA public;
ALTER TABLE IF EXISTS orders.order_returns SET SCHEMA public;
ALTER TABLE IF EXISTS orders.order_items SET SCHEMA public;
ALTER TABLE IF EXISTS orders.orders SET SCHEMA public;

-- 3. Recreate compatibility views in orders schema
CREATE OR REPLACE VIEW orders.orders AS SELECT * FROM public.orders;
CREATE OR REPLACE VIEW orders.order_items AS SELECT * FROM public.order_items;
CREATE OR REPLACE VIEW orders.order_returns AS SELECT * FROM public.order_returns;
CREATE OR REPLACE VIEW orders.order_tracking_events AS SELECT * FROM public.order_tracking_events;
