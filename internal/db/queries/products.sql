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