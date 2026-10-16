-- Align webhook_events and outbox_events columns for payment service compatibility
ALTER TABLE webhook_events ADD COLUMN IF NOT EXISTS we_created_at TIMESTAMPTZ DEFAULT now() NOT NULL;

ALTER TABLE outbox_events ADD COLUMN IF NOT EXISTS oe_id BIGINT;
ALTER TABLE outbox_events ADD COLUMN IF NOT EXISTS oe_aggregate_type VARCHAR(100);
ALTER TABLE outbox_events ADD COLUMN IF NOT EXISTS oe_aggregate_id VARCHAR(100);
ALTER TABLE outbox_events ADD COLUMN IF NOT EXISTS oe_event_type VARCHAR(50);
ALTER TABLE outbox_events ADD COLUMN IF NOT EXISTS oe_payload BYTEA;
ALTER TABLE outbox_events ADD COLUMN IF NOT EXISTS oe_status VARCHAR(20) DEFAULT 'PENDING';
ALTER TABLE outbox_events ADD COLUMN IF NOT EXISTS oe_created_at TIMESTAMPTZ DEFAULT now();
ALTER TABLE outbox_events ADD COLUMN IF NOT EXISTS oe_processed_at TIMESTAMPTZ;

ALTER TABLE outbox_events ALTER COLUMN aggregate_type SET DEFAULT '';
ALTER TABLE outbox_events ALTER COLUMN aggregate_id SET DEFAULT '';
ALTER TABLE outbox_events ALTER COLUMN event_type SET DEFAULT '';
ALTER TABLE outbox_events ALTER COLUMN payload SET DEFAULT '{}'::jsonb;


