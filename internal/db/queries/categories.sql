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