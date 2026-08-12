-- name: CreateUser :one
INSERT INTO users (
    user_fullname,
    user_email,
    user_password,
    user_age,
    user_status,
    user_level
)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: CountUsers :one
SELECT count(*)
FROM users
WHERE user_deleted_at IS NULL
AND (
    sqlc.narg(search)::TEXT IS NULL
    OR user_email ILIKE '%' || sqlc.narg(search)::TEXT || '%'
    OR user_fullname ILIKE '%' || sqlc.narg(search)::TEXT || '%'
);

-- name: ListUserUserIdAsc :many
SELECT *
FROM users
WHERE user_deleted_at IS NULL
AND (
    sqlc.narg(search)::TEXT IS NULL
    OR user_email ILIKE '%' || sqlc.narg(search)::TEXT || '%'
    OR user_fullname ILIKE '%' || sqlc.narg(search)::TEXT || '%'
)
ORDER BY user_id ASC
LIMIT $1 OFFSET $2;


-- name: ListUserUserIdDesc :many
SELECT *
FROM users
WHERE user_deleted_at IS NULL
AND (
    sqlc.narg(search)::TEXT IS NULL
    OR user_email ILIKE '%' || sqlc.narg(search)::TEXT || '%'
    OR user_fullname ILIKE '%' || sqlc.narg(search)::TEXT || '%'
)
ORDER BY user_id DESC
LIMIT $1 OFFSET $2;


-- name: ListUserUserCreatedAtAsc :many
SELECT *
FROM users
WHERE user_deleted_at IS NULL
AND (
    sqlc.narg(search)::TEXT IS NULL
    OR user_email ILIKE '%' || sqlc.narg(search)::TEXT || '%'
    OR user_fullname ILIKE '%' || sqlc.narg(search)::TEXT || '%'
)
ORDER BY user_created_at ASC, user_id ASC
LIMIT $1 OFFSET $2;


-- name: ListUserUserCreatedAtDesc :many
SELECT *
FROM users
WHERE user_deleted_at IS NULL
AND (
    sqlc.narg(search)::TEXT IS NULL
    OR user_email ILIKE '%' || sqlc.narg(search)::TEXT || '%'
    OR user_fullname ILIKE '%' || sqlc.narg(search)::TEXT || '%'
)
ORDER BY user_created_at DESC, user_id DESC
LIMIT $1 OFFSET $2;