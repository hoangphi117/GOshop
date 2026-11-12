-- Recreate the foreign table without the search columns. Dropping a column on a
-- foreign table is just a local catalog change; the analytics table is owned by
-- another service and is untouched.
DROP FOREIGN TABLE IF EXISTS analytics_mirror.user_events;

CREATE FOREIGN TABLE analytics_mirror.user_events (
    event_type   TEXT        NOT NULL,
    anon_id      UUID,
    user_id      INT,
    product_uuid UUID,
    props        JSONB       NOT NULL,
    occurred_at  TIMESTAMPTZ NOT NULL
) SERVER analytics_db OPTIONS (schema_name 'public', table_name 'user_events');
