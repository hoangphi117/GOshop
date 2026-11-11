-- Interaction telemetry owned by the analytics service.
--
-- The storefront batches events here; a dedicated personalization consumer
-- reads them through Kafka and keeps the recommendation tables (`user_affinity`,
-- `product_popularity`) on the recommendations service's database.
--
-- Cross-database foreign keys are intentionally absent: products and users live
-- in other services.

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
