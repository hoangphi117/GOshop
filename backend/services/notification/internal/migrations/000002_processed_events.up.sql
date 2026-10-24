-- Idempotency ledger for event consumers.
-- One row per (event, consumer) means: the event was fully processed
-- (every side effect succeeded) and must never be applied again.
CREATE TABLE IF NOT EXISTS processed_events (
    pe_event_id VARCHAR(255) NOT NULL,
    pe_consumer_name VARCHAR(100) NOT NULL,
    pe_processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_processed_events_event_consumer UNIQUE (pe_event_id, pe_consumer_name)
);

CREATE INDEX IF NOT EXISTS idx_processed_events_processed_at ON processed_events(pe_processed_at);
