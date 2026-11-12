-- Analytics tables mirrored for affinity.
--
-- Same pattern as the order mirror. user_events is owned by the analytics
-- service and written by the storefront's event batch; this service only reads
-- it, to turn interactions into a per-visitor taste profile.

CREATE SCHEMA IF NOT EXISTS analytics_mirror;

CREATE FOREIGN TABLE IF NOT EXISTS analytics_mirror.user_events (
    event_type   TEXT        NOT NULL,
    anon_id      UUID,
    user_id      INT,
    product_uuid UUID,
    props        JSONB       NOT NULL,
    occurred_at  TIMESTAMPTZ NOT NULL
) SERVER analytics_db OPTIONS (schema_name 'public', table_name 'user_events');
