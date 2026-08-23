-- name: CreateProduct :one
WITH inserted AS (
    INSERT INTO products (
        product_name, product_slug, product_description,
        product_price, product_stock, product_sku,
        product_category_id, product_image_url, product_status
    )
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
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
    OR unaccent(product_name) ILIKE '%' || unaccent(sqlc.narg(search)::TEXT) || '%'
    OR unaccent(product_slug) ILIKE '%' || unaccent(sqlc.narg(search)::TEXT) || '%'
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

-- name: UpdateProduct :one
WITH updated AS (
    UPDATE products
    SET
        product_name        = COALESCE(sqlc.narg('product_name')::varchar,        products.product_name),
        product_description = COALESCE(sqlc.narg('product_description')::text,    products.product_description),
        product_slug        = COALESCE(sqlc.narg('product_slug')::varchar,        products.product_slug),
        product_price       = COALESCE(sqlc.narg('product_price')::numeric,       products.product_price),
        product_stock       = COALESCE(sqlc.narg('product_stock')::int,           products.product_stock),
        product_sku         = COALESCE(sqlc.narg('product_sku')::varchar,         products.product_sku),
        product_category_id = COALESCE(sqlc.narg('product_category_id')::int,     products.product_category_id),
        product_image_url   = COALESCE(sqlc.narg('product_image_url')::text,      products.product_image_url),
        product_status      = COALESCE(sqlc.narg('product_status')::int,          products.product_status)
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

-- name: DeductStock :execrows
UPDATE products
SET 
    product_stock = product_stock - sqlc.arg(quantity)::int,
    product_status = CASE 
        WHEN (product_stock - sqlc.arg(quantity)::int) = 0 THEN 3 
        ELSE product_status 
    END
WHERE product_id = sqlc.arg(product_id)
  AND product_stock >= sqlc.arg(quantity)::int
  AND product_deleted_at IS NULL;

-- name: RestoreStock :exec
UPDATE products
SET 
    product_stock = product_stock + sqlc.arg(quantity)::int,
    product_status = CASE 
        WHEN product_status = 3 THEN 1 
        ELSE product_status 
    END
WHERE product_id = sqlc.arg(product_id)
  AND product_deleted_at IS NULL;