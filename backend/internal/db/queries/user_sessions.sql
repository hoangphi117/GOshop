-- name: CreateSession :one
INSERT INTO user_sessions (user_id, family_id, ip_address, user_agent, device_name)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListSessionsByUserID :many
SELECT * FROM user_sessions
WHERE user_id = $1
ORDER BY last_active_at DESC;

-- name: GetSessionByUUID :one
SELECT * FROM user_sessions
WHERE session_uuid = $1
LIMIT 1;

-- name: GetSessionByFamilyID :one
SELECT * FROM user_sessions
WHERE family_id = $1
LIMIT 1;

-- name: DeleteSessionByUUID :exec
DELETE FROM user_sessions
WHERE session_uuid = $1 AND user_id = $2;

-- name: DeleteAllOtherSessions :exec
DELETE FROM user_sessions
WHERE user_id = $1 AND family_id != $2;

-- name: DeleteSessionByFamilyID :exec
DELETE FROM user_sessions
WHERE family_id = $1;

-- name: UpdateSessionActivity :exec
UPDATE user_sessions
SET last_active_at = now()
WHERE family_id = $1;
