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
WHERE (
    sqlc.narg(deleted)::bool IS NULL
    OR sqlc.narg(deleted)::bool = TRUE AND user_deleted_at IS NOT NULL
    OR sqlc.narg(deleted)::bool = FALSE AND user_deleted_at IS NULL
) AND (
    sqlc.narg(search)::TEXT IS NULL
    OR unaccent(user_email) ILIKE '%' || unaccent(sqlc.narg(search)::TEXT) || '%'
    OR unaccent(user_fullname) ILIKE '%' || unaccent(sqlc.narg(search)::TEXT) || '%'
);

-- name: ListUserUserIdAsc :many
SELECT *
FROM users
WHERE user_deleted_at IS NULL
AND (
    sqlc.narg(search)::TEXT IS NULL
    OR unaccent(user_email) ILIKE '%' || unaccent(sqlc.narg(search)::TEXT) || '%'
    OR unaccent(user_fullname) ILIKE '%' || unaccent(sqlc.narg(search)::TEXT) || '%'
)
ORDER BY user_id ASC
LIMIT $1 OFFSET $2;

-- name: ListUserUserIdDesc :many
SELECT *
FROM users
WHERE user_deleted_at IS NULL
AND (
    sqlc.narg(search)::TEXT IS NULL
    OR unaccent(user_email) ILIKE '%' || unaccent(sqlc.narg(search)::TEXT) || '%'
    OR unaccent(user_fullname) ILIKE '%' || unaccent(sqlc.narg(search)::TEXT) || '%'
)
ORDER BY user_id DESC
LIMIT $1 OFFSET $2;

-- name: ListUserUserCreatedAtAsc :many
SELECT *
FROM users
WHERE user_deleted_at IS NULL
AND (
    sqlc.narg(search)::TEXT IS NULL
    OR unaccent(user_email) ILIKE '%' || unaccent(sqlc.narg(search)::TEXT) || '%'
    OR unaccent(user_fullname) ILIKE '%' || unaccent(sqlc.narg(search)::TEXT) || '%'
)
ORDER BY user_created_at ASC, user_id ASC
LIMIT $1 OFFSET $2;

-- name: ListUserUserCreatedAtDesc :many
SELECT *
FROM users
WHERE user_deleted_at IS NULL
AND (
    sqlc.narg(search)::TEXT IS NULL
    OR unaccent(user_email) ILIKE '%' || unaccent(sqlc.narg(search)::TEXT) || '%'
    OR unaccent(user_fullname) ILIKE '%' || unaccent(sqlc.narg(search)::TEXT) || '%'
)
ORDER BY user_created_at DESC, user_id DESC
LIMIT $1 OFFSET $2;

-- name: GetUserByUUID :one
SELECT *
FROM users
WHERE user_uuid = $1
AND user_deleted_at IS NULL;

-- name: GetUserByEmail :one
SELECT *
FROM users
WHERE user_email = $1
AND user_deleted_at IS NULL;

-- name: GetActiveAdminUserIDs :many
SELECT user_id FROM users
WHERE user_deleted_at IS NULL
    AND user_status = 1
    AND user_level IN (1, 2);

-- name: UpdateUser :one
UPDATE users
SET
    user_fullname = COALESCE(sqlc.narg(user_fullname), user_fullname),
    user_password = COALESCE(sqlc.narg(user_password), user_password),
    user_age = COALESCE(sqlc.narg(user_age), user_age),
    user_status = COALESCE(sqlc.narg(user_status), user_status),
    user_level = COALESCE(sqlc.narg(user_level), user_level)
WHERE 
    user_uuid = sqlc.arg(user_uuid)
    AND user_deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteUser :one
UPDATE users
SET 
    user_deleted_at = NOW()
WHERE 
    user_uuid = sqlc.arg(user_uuid)::uuid
    AND user_deleted_at IS NULL
RETURNING *;

-- name: RestoreUser :one
UPDATE users
SET 
    user_deleted_at = NULL
WHERE 
    user_uuid = sqlc.arg(user_uuid)::uuid
    AND user_deleted_at IS NOT NULL
RETURNING *;

-- name: TrashUser :one
DELETE FROM users
WHERE 
    user_uuid = sqlc.arg(user_uuid)::uuid
    AND user_deleted_at IS NOT NULL 
RETURNING *;

-- name: UpdatePassword :one
UPDATE users
SET user_password = sqlc.arg(new_password)
WHERE 
    user_uuid = sqlc.arg(user_uuid)::uuid
    AND user_deleted_at IS NULL
RETURNING *;