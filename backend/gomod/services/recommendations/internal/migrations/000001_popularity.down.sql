DROP FOREIGN TABLE IF EXISTS order_mirror.order_items;
DROP FOREIGN TABLE IF EXISTS order_mirror.orders;
DROP SCHEMA IF EXISTS order_mirror;
DROP USER MAPPING IF EXISTS FOR CURRENT_USER SERVER order_db;
DROP SERVER IF EXISTS order_db;
DROP INDEX IF EXISTS idx_product_popularity_rank;
DROP TABLE IF EXISTS product_popularity;
