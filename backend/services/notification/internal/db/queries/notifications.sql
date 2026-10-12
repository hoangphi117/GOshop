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
  AND notification_type NOT IN ('order', 'stock', 'inventory', 'return', 'order_return', 'refund', 'review', 'system', 'admin_order', 'admin_stock', 'admin_inventory', 'admin_return', 'admin_refund', 'admin_review', 'admin_system')
ORDER BY notification_created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountNotificationsByUserID :one
SELECT COUNT(*) FROM notifications
WHERE notification_user_id = $1
  AND notification_type NOT IN ('order', 'stock', 'inventory', 'return', 'order_return', 'refund', 'review', 'system', 'admin_order', 'admin_stock', 'admin_inventory', 'admin_return', 'admin_refund', 'admin_review', 'admin_system');

-- name: CountUnreadNotificationsByUserID :one
SELECT COUNT(*) FROM notifications
WHERE notification_user_id = $1
  AND notification_is_read = FALSE
  AND notification_type NOT IN ('order', 'stock', 'inventory', 'return', 'order_return', 'refund', 'review', 'system', 'admin_order', 'admin_stock', 'admin_inventory', 'admin_return', 'admin_refund', 'admin_review', 'admin_system');

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
  AND notification_type NOT IN ('order', 'stock', 'inventory', 'return', 'order_return', 'refund', 'review', 'system', 'admin_order', 'admin_stock', 'admin_inventory', 'admin_return', 'admin_refund', 'admin_review', 'admin_system')
  AND notification_is_read = FALSE;

-- name: DeleteNotification :exec
DELETE FROM notifications
WHERE notification_uuid    = sqlc.arg(notification_uuid)
  AND notification_user_id = sqlc.arg(user_id);

-- name: GetAdminNotifications :many
SELECT * FROM notifications
WHERE notification_user_id = sqlc.arg(user_id)
  AND notification_type IN ('order', 'stock', 'inventory', 'return', 'order_return', 'refund', 'review', 'system', 'admin_order', 'admin_stock', 'admin_inventory', 'admin_return', 'admin_refund', 'admin_review', 'admin_system')
  AND (sqlc.narg(type)::TEXT IS NULL OR notification_type = sqlc.narg(type)::TEXT OR (sqlc.narg(type)::TEXT = 'return' AND notification_type = 'order_return') OR (sqlc.narg(type)::TEXT = 'stock' AND notification_type = 'inventory'))
  AND (sqlc.narg(is_read)::BOOLEAN IS NULL OR notification_is_read = sqlc.narg(is_read)::BOOLEAN)
ORDER BY notification_created_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountAdminNotifications :one
SELECT COUNT(*) FROM notifications
WHERE notification_user_id = sqlc.arg(user_id)
  AND notification_type IN ('order', 'stock', 'inventory', 'return', 'order_return', 'refund', 'review', 'system', 'admin_order', 'admin_stock', 'admin_inventory', 'admin_return', 'admin_refund', 'admin_review', 'admin_system')
  AND (sqlc.narg(type)::TEXT IS NULL OR notification_type = sqlc.narg(type)::TEXT OR (sqlc.narg(type)::TEXT = 'return' AND notification_type = 'order_return') OR (sqlc.narg(type)::TEXT = 'stock' AND notification_type = 'inventory'))
  AND (sqlc.narg(is_read)::BOOLEAN IS NULL OR notification_is_read = sqlc.narg(is_read)::BOOLEAN);

-- name: MarkAllAdminNotificationsRead :exec
UPDATE notifications SET
    notification_is_read = TRUE,
    notification_read_at  = now()
WHERE notification_user_id = $1
  AND notification_type IN ('order', 'stock', 'inventory', 'return', 'order_return', 'refund', 'review', 'system', 'admin_order', 'admin_stock', 'admin_inventory', 'admin_return', 'admin_refund', 'admin_review', 'admin_system')
  AND notification_is_read = FALSE;

-- name: DeleteOldNotifications :execrows
DELETE FROM notifications
WHERE notification_is_read = TRUE
  AND notification_created_at < $1;
