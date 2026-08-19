-- name: CreateCategory :one
INSERT INTO categories (
    category_name,
    category_slug,
    category_description,
    category_status
) VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: CountCategories :one
SELECT count(*) FROM categories
WHERE (
    sqlc.narg(deleted)::bool IS NULL
    OR sqlc.narg(deleted)::bool = TRUE AND category_deleted_at IS NOT NULL
    OR sqlc.narg(deleted)::bool = FALSE AND category_deleted_at IS NULL
) AND (
    sqlc.narg(search)::TEXT IS NULL
    OR unaccent(category_name) ILIKE '%' || unaccent(sqlc.narg(search)::TEXT) || '%'
    OR unaccent(category_slug) ILIKE '%' || unaccent(sqlc.narg(search)::TEXT) || '%'
);

-- name: GetCategoryBySlug :one
SELECT * FROM categories
WHERE category_slug = $1
AND category_deleted_at IS NULL;

-- name: GetCategoryByUuid :one
SELECT * FROM categories
WHERE category_uuid = $1
AND category_deleted_at IS NULL;

-- name: UpdateCategory :one
UPDATE categories
SET
    category_name = COALESCE(sqlc.narg(category_name), category_name),
    category_description = COALESCE(sqlc.narg(category_description), category_description),
    category_status = COALESCE(sqlc.narg(category_status), category_status)
WHERE category_uuid = sqlc.arg(category_uuid)
AND category_deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteCategory :one
UPDATE categories
SET category_deleted_at = NOW()
WHERE category_uuid = sqlc.arg(category_uuid)::uuid
AND category_deleted_at IS NULL
RETURNING *;

-- name: RestoreCategory :one
UPDATE categories
SET category_deleted_at = NULL
WHERE category_uuid = sqlc.arg(category_uuid)::uuid
AND category_deleted_at IS NOT NULL
RETURNING *;

-- name: TrashCategory :one
DELETE FROM categories
WHERE category_uuid = sqlc.arg(category_uuid)::uuid
AND category_deleted_at IS NOT NULL
RETURNING *;