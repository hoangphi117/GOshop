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

-- name: GetNotificationsByUserId :many
SELECT * FROM notifications
WHERE notification_user_id = $1
ORDER BY notification_created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountNotificationsByUserID :one
SELECT COUNT(*) FROM notifications
WHERE notification_user_id = $1;

-- name: CountUnreadNotificationsByUserID :one
SELECT COUNT(*) FROM notifications
WHERE notification_user_id = $1
  AND notification_is_read = FALSE;
