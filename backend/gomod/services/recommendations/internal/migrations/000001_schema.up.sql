-- Recommendations service schema, in one migration.
--
-- The service's database is disposable: everything here is a read model built
-- from events or from foreign tables over other services' databases, so a
-- rebuild is a truncate plus a resync, never a data migration. That is why this
-- is one file rather than eight numbered steps — there is no installed database
-- to migrate, and the numbered files had accumulated three successive
-- declarations of the same foreign tables (000003 -> 000006, 000004 -> 000005
-- -> 000008) that each had to be read in order to learn the final shape.
-- The 000001_* naming is kept so the tracked migration version starts where
-- every existing checkout already is.
--
-- Written as a final shape: every table and foreign table is declared once,
-- with every column the read paths use.
--
-- The foreign servers (order_db, catalog_db, analytics_db) and their user
-- mappings are NOT created here. Their host and credentials come from the
-- environment, which a migration cannot read; app.go creates them before
-- migrating (ensureOrderServer / ensureCatalogServer / ensureAnalyticsServer).
-- Plain DDL never tests connectivity, so a missing link still lets the schema
-- land and every read then errors into the caller's fallback.


-- ---------------------------------------------------------------------------
-- Best sellers: the order mirror and its rollup.
--
-- Popularity is derived from order history, not from telemetry: a purchase is
-- the only interaction whose meaning is unambiguous (views and carts are cheap
-- to fake, a click is not a purchase).
--
-- The order tables live in another service's database (shopping_order, schema
-- `orders`). Two databases cannot be queried in one statement, so the order
-- tables are mirrored as foreign tables over postgres_fdw: one link, one user
-- mapping, no Kafka consumer, no Redis window, no cron. The link can be dropped
-- for an `order.purchased` event later without changing the API.
-- ---------------------------------------------------------------------------

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


-- ---------------------------------------------------------------------------
-- Embeddings and taste profiles.
--
-- Both tables are read models fed by an out-of-band worker (catalog outbox
-- events -> embedder sidecar -> product_embedding; analytics user_events ->
-- user_affinity). Nothing here is written by the HTTP handlers, so a stale
-- table degrades a shelf but never fails a request.
-- ---------------------------------------------------------------------------

-- all-MiniLM-L6-v2 emits 384 floats. Keeping the dimension in the column type
-- is what lets an HNSW index exist at all: vector without a dimension cannot
-- be indexed, and the model is pinned in the embedder sidecar.
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS product_embedding (
    product_uuid  UUID PRIMARY KEY,
    embedding     vector(384) NOT NULL,
    model         TEXT        NOT NULL DEFAULT 'all-MiniLM-L6-v2',
    -- Text that produced the vector. Comparing it with the catalog's current
    -- text is how the backfill notices a product whose copy was edited.
    source_hash   TEXT        NOT NULL,
    embedded_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Cosine distance matches the normalised output of the model; inner product
-- would rank longer vectors higher for no reason here.
CREATE INDEX IF NOT EXISTS idx_product_embedding_hnsw
    ON product_embedding USING hnsw (embedding vector_cosine_ops);

-- Affinity is computed on read from analytics_mirror.user_events, so this table
-- is no longer written by anything. It is kept (empty) only because rolling a
-- migration back on a live database is riskier than leaving a dead table; drop
-- it in a follow-up migration once the read path has run in production.
--
-- One row per (visitor, product). visitor_key is 'u:<user_id>' for a signed-in
-- shopper or 'a:<anon_id>' for a guest. Scores would be decayed weights, not
-- counters: the same product viewed twice in a minute must not outrank a
-- purchase that happened last week.
CREATE TABLE IF NOT EXISTS user_affinity (
    visitor_key   TEXT             NOT NULL,
    product_uuid  UUID             NOT NULL,
    score         DOUBLE PRECISION NOT NULL DEFAULT 0,
    last_event_at TIMESTAMPTZ      NOT NULL DEFAULT now(),
    PRIMARY KEY (visitor_key, product_uuid)
);

CREATE INDEX IF NOT EXISTS idx_user_affinity_visitor
    ON user_affinity (visitor_key, score DESC);


-- ---------------------------------------------------------------------------
-- Catalog mirror.
--
-- Same reason as the order mirror: the catalog lives in its own database
-- (shopping_catalog) and PostgreSQL cannot join across databases. Read-only
-- here; writes stay in the catalog service.
--
-- The declaration carries the facet columns too (price, rating, category_uuid)
-- on purpose. A foreign table is a local declaration and has no ALTER, so a
-- later widening means drop-and-recreate; declaring the final column set once
-- avoids that. category_uuid is the category's public identifier, the value the
-- storefront actually sends — resolving it through products only worked for
-- products a shopper had already viewed.
--
-- The types match the catalog columns exactly; postgres_fdw rejects a remote
-- value it cannot fit.
-- ---------------------------------------------------------------------------

CREATE SCHEMA IF NOT EXISTS catalog_mirror;

CREATE FOREIGN TABLE IF NOT EXISTS catalog_mirror.products (
    product_uuid        UUID          NOT NULL,
    product_name        VARCHAR(200)  NOT NULL,
    product_description TEXT,
    product_category_id INT           NOT NULL,
    product_status      INT           NOT NULL,
    product_deleted_at  TIMESTAMPTZ,
    product_price       NUMERIC(12,2) NOT NULL,
    product_avg_rating  NUMERIC(2,1)  NOT NULL
) SERVER catalog_db OPTIONS (schema_name 'public', table_name 'products');

CREATE FOREIGN TABLE IF NOT EXISTS catalog_mirror.categories (
    category_id   INT          NOT NULL,
    category_uuid UUID         NOT NULL,
    category_name VARCHAR(100) NOT NULL
) SERVER catalog_db OPTIONS (schema_name 'public', table_name 'categories');


-- ---------------------------------------------------------------------------
-- Analytics mirror.
--
-- user_events is owned by the analytics service and written by the storefront's
-- event batch; this service only reads it, to turn interactions into a
-- per-visitor taste profile.
--
-- The column list must stay a subset of analytics' user_events. A column
-- declared here but absent there fails at parse time — a 500 on the homepage
-- shelf rather than a degraded one. query/result_count feed search affinity,
-- session_id feeds the session tier.
-- ---------------------------------------------------------------------------

CREATE SCHEMA IF NOT EXISTS analytics_mirror;

CREATE FOREIGN TABLE IF NOT EXISTS analytics_mirror.user_events (
    event_type   TEXT        NOT NULL,
    anon_id      UUID,
    user_id      INT,
    session_id   UUID,
    query        TEXT,
    result_count INT,
    product_uuid UUID,
    props        JSONB       NOT NULL,
    occurred_at  TIMESTAMPTZ NOT NULL
) SERVER analytics_db OPTIONS (schema_name 'public', table_name 'user_events');


-- ---------------------------------------------------------------------------
-- Co-visitation: which products people look at together.
--
-- Vector closeness answers "this looks like what you browsed". It cannot answer
-- "people who looked at this also looked at that", which is the strongest single
-- signal in e-commerce and the one thing a shared-template description destroys:
-- every product reads as the same sentence, so a camera and a memory card are
-- as far apart as a camera and a phone.
--
-- The table is a materialised aggregate, not a live view: it is written by
-- recommendations.RefreshCovisitation and read on the shelf path, so a request
-- never joins the event log.
--
-- Its source is analytics events rather than order_items because a demo store
-- has traffic long before it has orders. The purchase signal is folded in at
-- the same weight class when orders exist.
--
-- Undirected by construction: every pair is stored once with the smaller uuid
-- first, so a lookup is one indexed union rather than two scans.
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS product_covisitation (
    source_uuid UUID    NOT NULL,
    target_uuid UUID    NOT NULL,
    visitors    INT     NOT NULL,
    last_seen   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (source_uuid, target_uuid),
    CHECK (source_uuid <> target_uuid)
);

-- The shelf path looks up one side only, and it looks it up in bulk.
CREATE INDEX IF NOT EXISTS product_covisitation_target_idx ON product_covisitation (target_uuid, visitors DESC);
CREATE INDEX IF NOT EXISTS product_covisitation_visitors_idx ON product_covisitation (visitors DESC);
