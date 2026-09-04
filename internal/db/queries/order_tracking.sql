-- name: InsertTrackingEvent :one
INSERT INTO order_tracking_events (
    event_order_id,
    event_status,
    event_note,
    event_source
) VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetTrackingEventsByOrderID :many
SELECT * FROM order_tracking_events
WHERE event_order_id = $1
ORDER BY event_created_at ASC;