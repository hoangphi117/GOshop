ALTER TABLE webhook_events DROP COLUMN IF EXISTS we_created_at;
ALTER TABLE outbox_events DROP COLUMN IF EXISTS oe_aggregate_type;
ALTER TABLE outbox_events DROP COLUMN IF EXISTS oe_aggregate_id;
ALTER TABLE outbox_events DROP COLUMN IF EXISTS oe_event_type;
ALTER TABLE outbox_events DROP COLUMN IF EXISTS oe_payload;
ALTER TABLE outbox_events DROP COLUMN IF EXISTS oe_status;
ALTER TABLE outbox_events DROP COLUMN IF EXISTS oe_created_at;
ALTER TABLE outbox_events DROP COLUMN IF EXISTS oe_processed_at;
