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

-- name: CreateNotificationForAllUsers :exec
INSERT INTO notifications (
    notification_user_id,
    notification_type,
    notification_title,
    notification_body,
    notification_data
)
SELECT
    users.user_id, $1, $2, $3, $4
FROM users
WHERE user_deleted_at IS NULL;


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

-- name: MarkNotificationRead :one
UPDATE notifications SET
    notification_is_read = TRUE,
    notification_read_at  = now()
WHERE notification_uuid    = sqlc.arg(notification_uuid)
  AND notification_user_id = sqlc.arg(user_id)
  AND notification_is_read = FALSE
RETURNING *;

-- name: MarkAllNotificationsRead :exec
UPDATE notifications SET
    notification_is_read = TRUE,
    notification_read_at  = now()
WHERE notification_user_id = $1
  AND notification_is_read = FALSE;

-- name: DeleteNotification :exec
DELETE FROM notifications
WHERE notification_uuid    = sqlc.arg(notification_uuid)
  AND notification_user_id = sqlc.arg(user_id);