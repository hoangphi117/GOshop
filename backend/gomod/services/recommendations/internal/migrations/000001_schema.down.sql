-- Reverse of 000001_schema.up.sql, in dependency order.
--
-- No CASCADE: a leftover object would make this fail loudly instead of taking
-- unrelated things with it. Servers (order_db, catalog_db, analytics_db) are
-- owned by app.go, which recreates them on every boot, so they are not dropped
-- here.

DROP TABLE IF EXISTS product_covisitation;

DROP FOREIGN TABLE IF EXISTS analytics_mirror.user_events;
DROP SCHEMA IF EXISTS analytics_mirror;

DROP FOREIGN TABLE IF EXISTS catalog_mirror.categories;
DROP FOREIGN TABLE IF EXISTS catalog_mirror.products;
DROP SCHEMA IF EXISTS catalog_mirror;

DROP TABLE IF EXISTS user_affinity;
DROP TABLE IF EXISTS product_embedding;
-- Extensions are left installed: other schemas may have started using them, and
-- DROP EXTENSION CASCADE here would silently take their columns with it.

DROP FOREIGN TABLE IF EXISTS order_mirror.order_items;
DROP FOREIGN TABLE IF EXISTS order_mirror.orders;
DROP SCHEMA IF EXISTS order_mirror;

DROP TABLE IF EXISTS product_popularity;
