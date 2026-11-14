-- Analytics service schema, in one migration.
--
-- Analytics owns this database and nothing else writes to it. It is a read
-- model: the projections are fed by the order event consumer and can be rebuilt
-- from events, and the event log is telemetry. That is why this is one file
-- rather than three numbered steps — there is no installed database to migrate,
-- and shipping a single declaration keeps the table definitions readable beside
-- each other. The 000001_* naming is kept so the tracked migration version
-- starts where every existing checkout already is.
--
-- Cross-database foreign keys are intentionally absent: products and users live
-- in other services. Product dimensions are resolved over gRPC at query time.


-- ---------------------------------------------------------------------------
-- Order projections.
--
-- Previously maintained inside the monolith database (analytics_*_projection);
-- copied here so analytics owns its storage. The order projection consumer
-- feeds them, upserting by idempotency key.
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS analytics_orders_projection (
    order_id                INT PRIMARY KEY,
    order_uuid              UUID NOT NULL UNIQUE,
    order_number            VARCHAR(20) NOT NULL UNIQUE,
    order_user_id           INT NOT NULL,
    order_user_uuid         UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000000',
    order_customer_name     VARCHAR(100) NOT NULL DEFAULT '',
    order_customer_email    VARCHAR(255) NOT NULL DEFAULT '',
    order_shipping_name     VARCHAR(100) NOT NULL DEFAULT '',
    order_shipping_phone    VARCHAR(20) NOT NULL DEFAULT '',
    order_shipping_address  TEXT NOT NULL DEFAULT '',
    order_total_amount      NUMERIC(14,2) NOT NULL CHECK (order_total_amount >= 0),
    order_refunded_amount   NUMERIC(14,2) NOT NULL DEFAULT 0.00 CHECK (order_refunded_amount >= 0),
    order_discount_amount   NUMERIC(12,2) NOT NULL DEFAULT 0.00,
    order_payment_method    INT NOT NULL DEFAULT 1 CHECK (order_payment_method IN (1, 2, 3)),
    order_payment_status    INT NOT NULL DEFAULT 1 CHECK (order_payment_status IN (1, 2, 3, 4)),
    order_note              TEXT,
    order_status            INT NOT NULL DEFAULT 1 CHECK (order_status IN (1, 2, 3, 4, 5, 6)),
    order_shipping_provider VARCHAR(20),
    order_shipping_code     VARCHAR(50),
    order_shipping_fee      NUMERIC(12,2) NOT NULL DEFAULT 0.00,
    order_delivered_at      TIMESTAMPTZ DEFAULT NULL,
    order_cancelled_at      TIMESTAMPTZ DEFAULT NULL,
    order_created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    order_updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_analytics_orders_created_at ON analytics_orders_projection (order_created_at);
CREATE INDEX IF NOT EXISTS idx_analytics_orders_paid ON analytics_orders_projection (order_payment_status, order_created_at);

CREATE TABLE IF NOT EXISTS analytics_order_items_projection (
    order_item_uuid          UUID PRIMARY KEY,
    order_item_order_id      INT NOT NULL REFERENCES analytics_orders_projection(order_id) ON DELETE CASCADE,
    -- No FK: products live in the catalog service.
    order_item_product_id    INT NOT NULL,
    order_item_product_uuid  UUID,
    order_item_product_name  VARCHAR(200) NOT NULL,
    order_item_price         NUMERIC(12,2) NOT NULL CHECK (order_item_price > 0),
    order_item_quantity      INT NOT NULL CHECK (order_item_quantity > 0),
    order_item_subtotal      NUMERIC(14,2) NOT NULL CHECK (order_item_subtotal > 0),
    order_item_created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_analytics_order_items_order_id ON analytics_order_items_projection (order_item_order_id);
CREATE INDEX IF NOT EXISTS idx_analytics_order_items_product_id ON analytics_order_items_projection (order_item_product_id);
CREATE INDEX IF NOT EXISTS idx_analytics_order_items_product_uuid ON analytics_order_items_projection (order_item_product_uuid);


-- ---------------------------------------------------------------------------
-- Interaction telemetry.
--
-- The storefront batches events here; a dedicated personalization consumer
-- reads them through Kafka and keeps the recommendation tables on the
-- recommendations service's database. The recommendations service also reads
-- this table directly over postgres_fdw.
--
-- session_id groups a browsing pass (sessionStorage on the storefront) so the
-- session tier can answer "what is this visitor doing right now". NULL is
-- allowed and common: privacy-mode browsers and events queued before the
-- storefront stamped sessions.
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS user_events (
    event_id     UUID PRIMARY KEY,
    event_type   TEXT NOT NULL,
    anon_id      UUID,
    user_id      INT,
    session_id   UUID,
    query        TEXT,
    product_uuid UUID,
    position     SMALLINT,
    result_count INT,
    props        JSONB NOT NULL DEFAULT '{}'::jsonb,
    occurred_at  TIMESTAMPTZ NOT NULL,
    ingested_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Every event must be attributable to someone, or it is unusable downstream.
    CONSTRAINT user_events_identity CHECK (anon_id IS NOT NULL OR user_id IS NOT NULL)
);

CREATE INDEX IF NOT EXISTS idx_user_events_occurred_at ON user_events (occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_user_events_anon ON user_events (anon_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_user_events_user ON user_events (user_id, occurred_at DESC);
-- Search history reads group by query for one subject; keep it index-only-ish.
CREATE INDEX IF NOT EXISTS idx_user_events_search ON user_events (anon_id, user_id, query, occurred_at DESC)
    WHERE event_type = 'search';
-- Session affinity. The recommendations service's session tier filters one
-- session and orders by time, which neither (anon_id, ...) nor (user_id, ...)
-- can serve: a session is a narrower slice than either key. The partial
-- predicate keeps a NULL-heavy table from paying for an index that never
-- returns those rows.
CREATE INDEX IF NOT EXISTS idx_user_events_session ON user_events (session_id, occurred_at DESC)
    WHERE session_id IS NOT NULL;

-- Dismissals live apart from the event log so the log stays immutable while a
-- user can still delete a keyword. An event is hidden when a dismissal was made
-- at or after the event's timestamp.
CREATE TABLE IF NOT EXISTS search_history_dismissals (
    subject_kind TEXT NOT NULL CHECK (subject_kind IN ('anon', 'user')),
    subject_id   TEXT NOT NULL,
    query        TEXT NOT NULL,
    dismissed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (subject_kind, subject_id, query)
);

CREATE INDEX IF NOT EXISTS idx_search_dismissals_subject ON search_history_dismissals (subject_kind, subject_id);
