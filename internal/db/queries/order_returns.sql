-- name: CreateOrderReturn :one
INSERT INTO order_returns (
    return_order_id,
    return_user_id,
    return_reason,
    return_images,
    return_video_url,
    return_status
) VALUES (
    $1, $2, $3, $4, $5, 1 -- 1: Pending
)
RETURNING *;

-- name: GetActiveReturnByOrderID :one
SELECT * FROM order_returns
WHERE return_order_id = $1 
  AND return_status IN (1, 2) -- 1: Pending, 2: Approved
LIMIT 1;

-- name: GetReturnByUUID :one
SELECT * FROM order_returns
WHERE return_uuid = $1;
