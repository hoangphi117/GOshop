-- name: CreateCategory :one
INSERT INTO categories (
    category_name,
    category_slug,
    category_description,
    category_status
) VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetCategoryBySlug :one
SELECT * FROM categories
WHERE category_slug = $1
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