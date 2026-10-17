-- Migration 000021: Create internal logical schemas and bounded-context views
-- Prepares the monolith database for modular extraction in Phase B

-- 1. Create Bounded Context Schemas
CREATE SCHEMA IF NOT EXISTS catalog;
CREATE SCHEMA IF NOT EXISTS inventory;
CREATE SCHEMA IF NOT EXISTS cart;
CREATE SCHEMA IF NOT EXISTS coupon;
CREATE SCHEMA IF NOT EXISTS orders;

-- 2. Schema: catalog
CREATE OR REPLACE VIEW catalog.products AS SELECT * FROM public.products;
CREATE OR REPLACE VIEW catalog.categories AS SELECT * FROM public.categories;
CREATE OR REPLACE VIEW catalog.product_reviews AS SELECT * FROM public.product_reviews;
CREATE OR REPLACE VIEW catalog.product_review_helpful_votes AS SELECT * FROM public.product_review_helpful_votes;
CREATE OR REPLACE VIEW catalog.product_review_reports AS SELECT * FROM public.product_review_reports;

-- 3. Schema: inventory
CREATE OR REPLACE VIEW inventory.stock_movements AS SELECT * FROM public.stock_movements;

-- 4. Schema: cart
CREATE OR REPLACE VIEW cart.carts AS SELECT * FROM public.carts;
CREATE OR REPLACE VIEW cart.cart_items AS SELECT * FROM public.cart_items;

-- 5. Schema: coupon
CREATE OR REPLACE VIEW coupon.coupons AS SELECT * FROM public.coupons;
CREATE OR REPLACE VIEW coupon.coupon_usages AS SELECT * FROM public.coupon_usages;

-- 6. Schema: orders
CREATE OR REPLACE VIEW orders.orders AS SELECT * FROM public.orders;
CREATE OR REPLACE VIEW orders.order_items AS SELECT * FROM public.order_items;
CREATE OR REPLACE VIEW orders.order_returns AS SELECT * FROM public.order_returns;
CREATE OR REPLACE VIEW orders.order_tracking_events AS SELECT * FROM public.order_tracking_events;
