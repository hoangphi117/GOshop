-- Migration 000021 Down: Drop internal logical schemas and views

DROP VIEW IF EXISTS orders.order_tracking_events;
DROP VIEW IF EXISTS orders.order_returns;
DROP VIEW IF EXISTS orders.order_items;
DROP VIEW IF EXISTS orders.orders;

DROP VIEW IF EXISTS coupon.coupon_usages;
DROP VIEW IF EXISTS coupon.coupons;

DROP VIEW IF EXISTS cart.cart_items;
DROP VIEW IF EXISTS cart.carts;

DROP VIEW IF EXISTS inventory.stock_movements;

DROP VIEW IF EXISTS catalog.product_review_reports;
DROP VIEW IF EXISTS catalog.product_review_helpful_votes;
DROP VIEW IF EXISTS catalog.product_reviews;
DROP VIEW IF EXISTS catalog.categories;
DROP VIEW IF EXISTS catalog.products;

DROP SCHEMA IF EXISTS orders CASCADE;
DROP SCHEMA IF EXISTS coupon CASCADE;
DROP SCHEMA IF EXISTS cart CASCADE;
DROP SCHEMA IF EXISTS inventory CASCADE;
DROP SCHEMA IF EXISTS catalog CASCADE;
