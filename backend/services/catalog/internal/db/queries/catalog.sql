-- =============================================================================
-- CATEGORIES
-- =============================================================================

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
    OR category_name ILIKE '%' || sqlc.narg(search)::TEXT || '%'
    OR category_slug ILIKE '%' || sqlc.narg(search)::TEXT || '%'
);

-- name: GetCategoryBySlug :one
SELECT * FROM categories
WHERE category_slug = $1
AND category_deleted_at IS NULL;

-- name: GetCategoryByUUID :one
SELECT * FROM categories
WHERE category_uuid = $1
AND category_deleted_at IS NULL;

-- name: GetCategoryByID :one
SELECT * FROM categories
WHERE category_id = $1
AND category_deleted_at IS NULL;

-- name: ListCategories :many
SELECT * FROM categories
WHERE (
    sqlc.narg(active_only)::bool IS NULL 
    OR sqlc.narg(active_only)::bool = FALSE 
    OR (sqlc.narg(active_only)::bool = TRUE AND category_status = 1)
)
AND category_deleted_at IS NULL
ORDER BY category_name ASC;

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

-- =============================================================================
-- PRODUCTS
-- =============================================================================

-- name: CreateProduct :one
WITH inserted AS (
    INSERT INTO products (
        product_name, product_slug, product_description,
        product_price, product_sku,
        product_category_id, product_image_url, product_status
    )
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
    RETURNING *
)
SELECT
    inserted.*,
    categories.category_uuid AS category_uuid,
    categories.category_name AS category_name
FROM inserted
JOIN categories ON categories.category_id = inserted.product_category_id;

-- name: CountProducts :one
SELECT count(*) FROM products
JOIN categories ON categories.category_id = products.product_category_id
WHERE (
    sqlc.narg(deleted)::bool IS NULL
    OR sqlc.narg(deleted)::bool = TRUE AND product_deleted_at IS NOT NULL
    OR sqlc.narg(deleted)::bool = FALSE AND product_deleted_at IS NULL
) AND (
    sqlc.narg(search)::TEXT IS NULL
    OR product_name ILIKE '%' || sqlc.narg(search)::TEXT || '%'
    OR product_slug ILIKE '%' || sqlc.narg(search)::TEXT || '%'
) AND (
    sqlc.narg(category_uuid)::uuid IS NULL
    OR categories.category_uuid = sqlc.narg(category_uuid)::uuid
) AND (
    sqlc.narg(min_price)::numeric IS NULL
    OR product_price >= sqlc.narg(min_price)::numeric
) AND (
    sqlc.narg(max_price)::numeric IS NULL
    OR product_price <= sqlc.narg(max_price)::numeric
) AND (
    sqlc.narg(min_rating)::numeric IS NULL
    OR product_avg_rating >= sqlc.narg(min_rating)::numeric
) AND (
    sqlc.narg(status)::int IS NULL
    OR product_status = sqlc.narg(status)::int
);

-- name: GetProductBySlug :one
SELECT
    products.*,
    categories.category_uuid AS category_uuid,
    categories.category_name AS category_name
FROM products
JOIN categories ON categories.category_id = products.product_category_id
WHERE products.product_slug = $1
AND products.product_deleted_at IS NULL;

-- name: GetProductByUUID :one
SELECT
    products.*,
    categories.category_uuid AS category_uuid,
    categories.category_name AS category_name
FROM products
JOIN categories ON categories.category_id = products.product_category_id
WHERE products.product_uuid = $1
AND products.product_deleted_at IS NULL;

-- name: GetProductByID :one
SELECT
    products.*,
    categories.category_uuid AS category_uuid,
    categories.category_name AS category_name
FROM products
JOIN categories ON categories.category_id = products.product_category_id
WHERE products.product_id = $1
AND products.product_deleted_at IS NULL;

-- name: GetProductsByUUIDs :many
SELECT
    products.*,
    categories.category_uuid AS category_uuid,
    categories.category_name AS category_name
FROM products
JOIN categories ON categories.category_id = products.product_category_id
WHERE products.product_uuid = ANY(sqlc.arg(uuids)::uuid[])
AND products.product_deleted_at IS NULL;

-- name: GetProductsByIDs :many
SELECT
    products.*,
    categories.category_uuid AS category_uuid,
    categories.category_name AS category_name
FROM products
JOIN categories ON categories.category_id = products.product_category_id
WHERE products.product_id = ANY(sqlc.arg(ids)::int[])
AND products.product_deleted_at IS NULL;

-- name: GetByUUIDsForUpdate :many
SELECT
    products.*,
    categories.category_uuid AS category_uuid,
    categories.category_name AS category_name
FROM products
JOIN categories ON categories.category_id = products.product_category_id
WHERE products.product_uuid = ANY(sqlc.arg(uuids)::uuid[])
AND products.product_deleted_at IS NULL
ORDER BY product_uuid
FOR UPDATE;

-- name: UpdateProduct :one
WITH updated AS (
    UPDATE products
    SET
        product_name         = COALESCE(sqlc.narg('product_name')::varchar,        products.product_name),
        product_description  = COALESCE(sqlc.narg('product_description')::text,    products.product_description),
        product_slug         = COALESCE(sqlc.narg('product_slug')::varchar,        products.product_slug),
        product_price         = COALESCE(sqlc.narg('product_price')::numeric,       products.product_price),
        product_sku           = COALESCE(sqlc.narg('product_sku')::varchar,         products.product_sku),
        product_category_id   = COALESCE(sqlc.narg('product_category_id')::int,     products.product_category_id),
        product_image_url     = COALESCE(sqlc.narg('product_image_url')::text,      products.product_image_url),
        product_status = COALESCE(sqlc.narg('product_status')::int, products.product_status)
    WHERE product_uuid = sqlc.arg(product_uuid)
    AND product_deleted_at IS NULL
    RETURNING *
)
SELECT
    updated.*,
    categories.category_uuid AS category_uuid,
    categories.category_name AS category_name
FROM updated
JOIN categories ON categories.category_id = updated.product_category_id;

-- name: SoftDeleteProduct :one
UPDATE products
SET product_deleted_at = NOW()
WHERE product_uuid = sqlc.arg(product_uuid)::uuid
AND product_deleted_at IS NULL
RETURNING *;

-- name: RestoreProduct :one
UPDATE products
SET product_deleted_at = NULL
WHERE product_uuid = sqlc.arg(product_uuid)::uuid
AND product_deleted_at IS NOT NULL
RETURNING *;

-- name: TrashProduct :one
DELETE FROM products
WHERE product_uuid = sqlc.arg(product_uuid)::uuid
AND product_deleted_at IS NOT NULL
RETURNING *;

-- =============================================================================
-- OUTBOX EVENTS
-- =============================================================================

-- name: InsertOutboxEvent :one
INSERT INTO outbox_events (
    aggregate_type,
    aggregate_id,
    event_type,
    payload,
    status
) VALUES ($1, $2, $3, $4, 'PENDING')
RETURNING *;

-- name: GetPendingOutboxEvents :many
SELECT * FROM outbox_events
WHERE status = 'PENDING'
ORDER BY created_at ASC
LIMIT $1;

-- name: MarkOutboxEventProcessed :exec
UPDATE outbox_events
SET status = 'PROCESSED', processed_at = NOW()
WHERE event_id = $1;
