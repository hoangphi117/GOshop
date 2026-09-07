-- name: CreateNotification :one
INSERT INTO notifications (
    notification_user_id,
    notification_type,
    notification_title,
    notification_body,
    notification_data
) VALUES (
    $1, $2, $3, $4, $5
) RETURNING *;
