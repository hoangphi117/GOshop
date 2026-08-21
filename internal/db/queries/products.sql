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