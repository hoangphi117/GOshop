-- name: InsertWebhookEvent :one
INSERT INTO webhook_events (we_provider, we_external_event_id, we_event_type, we_raw_body)
VALUES ($1, $2, $3, $4)
ON CONFLICT (we_provider, we_external_event_id) DO NOTHING
RETURNING *;

-- name: MarkWebhookEventProcessed :exec
UPDATE webhook_events SET we_processed = TRUE WHERE we_id = $1;
