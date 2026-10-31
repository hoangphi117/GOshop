-- name: InsertOutboxEvent :one
INSERT INTO outbox_events (
    aggregate_type, aggregate_id, event_type, payload
) VALUES (
    $1, $2, $3, $4
) RETURNING *;

-- name: GetPendingOutboxEvents :many
SELECT * FROM outbox_events
WHERE status = 'PENDING' AND retry_count < 5
ORDER BY created_at ASC
LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: GetPendingOutboxEventsByAggregateTypes :many
SELECT * FROM outbox_events
WHERE status = 'PENDING' AND retry_count < 5 AND aggregate_type = ANY(@aggregate_types::varchar[])
ORDER BY created_at ASC
LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: MarkOutboxEventsProcessed :exec
UPDATE outbox_events
SET status = 'PROCESSED', processed_at = NOW()
WHERE event_id = ANY($1::bigint[]);

-- name: MarkOutboxEventFailed :exec
UPDATE outbox_events
SET status = CASE WHEN retry_count + 1 >= 5 THEN 'FAILED' ELSE 'PENDING' END,
    retry_count = retry_count + 1,
    last_error = $2
WHERE event_id = $1;
