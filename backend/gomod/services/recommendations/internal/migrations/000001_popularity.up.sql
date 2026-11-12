-- Best sellers read model.
--
-- Popularity is derived from order history, not from telemetry: a purchase is
-- the only interaction whose meaning is unambiguous (views and carts are cheap
-- to fake, a click is not a purchase).
--
-- The order tables live in another service's database (shopping_order, schema
-- `orders`). Two databases cannot be queried in one statement, so the order
-- tables are mirrored here as foreign tables over postgres_fdw: one link, one
-- user mapping, no Kafka consumer, no Redis window, no cron. The link can be
-- dropped for an `order.purchased` event later without changing the API.
--
-- The foreign server and user mapping are created before this migration runs
-- (ensureOrderServer in app.go): their host and credentials come from the
-- environment, which a migration cannot read. These tables are plain DDL and
-- never test connectivity, so a missing or broken link still lets the schema
-- land; every read then errors and the service falls back to ratings.

CREATE TABLE IF NOT EXISTS product_popularity (
    product_uuid   UUID PRIMARY KEY,
    units_sold     BIGINT           NOT NULL DEFAULT 0,
    revenue        NUMERIC(14,2)    NOT NULL DEFAULT 0,
    window_days    INT              NOT NULL DEFAULT 30,
    refreshed_at   TIMESTAMPTZ      NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_product_popularity_rank
    ON product_popularity (units_sold DESC, revenue DESC);

CREATE EXTENSION IF NOT EXISTS postgres_fdw;

CREATE SCHEMA IF NOT EXISTS order_mirror;

-- Columns and types mirror orders.order_items / orders.orders exactly; a
-- mismatch fails the rebuild, which then degrades to ratings.
CREATE FOREIGN TABLE IF NOT EXISTS order_mirror.order_items (
    order_item_order_id     INT           NOT NULL,
    order_item_product_uuid UUID          NOT NULL,
    order_item_quantity     INT           NOT NULL,
    order_item_subtotal     NUMERIC(14,2) NOT NULL
) SERVER order_db OPTIONS (schema_name 'orders', table_name 'order_items');

CREATE FOREIGN TABLE IF NOT EXISTS order_mirror.orders (
    order_id         INT         NOT NULL,
    order_status     INT         NOT NULL,
    order_created_at TIMESTAMPTZ NOT NULL
) SERVER order_db OPTIONS (schema_name 'orders', table_name 'orders');
