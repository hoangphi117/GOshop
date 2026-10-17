-- name: GetStockByUUID :one
SELECT * FROM inventory_stocks
WHERE product_uuid = $1 LIMIT 1;

-- name: GetStockByUUIDs :many
SELECT * FROM inventory_stocks
WHERE product_uuid = ANY($1::uuid[])
ORDER BY product_uuid ASC;

-- name: GetStockByProductID :one
SELECT * FROM inventory_stocks
WHERE product_id = $1 LIMIT 1;

-- name: GetStockByProductIDs :many
SELECT * FROM inventory_stocks
WHERE product_id = ANY($1::int[])
ORDER BY product_id ASC;

-- name: UpsertStockMetadata :one
INSERT INTO inventory_stocks (
    product_id,
    product_uuid,
    product_name,
    product_sku,
    product_image_url,
    quantity
) VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (product_uuid) DO UPDATE
SET product_id = COALESCE(EXCLUDED.product_id, inventory_stocks.product_id),
    product_name = CASE WHEN EXCLUDED.product_name <> '' THEN EXCLUDED.product_name ELSE inventory_stocks.product_name END,
    product_sku = CASE WHEN EXCLUDED.product_sku <> '' THEN EXCLUDED.product_sku ELSE inventory_stocks.product_sku END,
    product_image_url = CASE WHEN EXCLUDED.product_image_url <> '' THEN EXCLUDED.product_image_url ELSE inventory_stocks.product_image_url END,
    updated_at = now()
RETURNING *;

-- name: DeductStockByUUID :one
UPDATE inventory_stocks
SET quantity = quantity - $2,
    updated_at = now()
WHERE product_uuid = $1 AND quantity >= $2
RETURNING *;

-- name: DeductStockByID :one
UPDATE inventory_stocks
SET quantity = quantity - $2,
    updated_at = now()
WHERE product_id = $1 AND quantity >= $2
RETURNING *;

-- name: RestoreStockByUUID :one
UPDATE inventory_stocks
SET quantity = quantity + $2,
    updated_at = now()
WHERE product_uuid = $1
RETURNING *;

-- name: RestoreStockByID :one
UPDATE inventory_stocks
SET quantity = quantity + $2,
    updated_at = now()
WHERE product_id = $1
RETURNING *;

-- name: AdjustStockByUUID :one
UPDATE inventory_stocks
SET quantity = quantity + $2,
    updated_at = now()
WHERE product_uuid = $1 AND (quantity + $2) >= 0
RETURNING *;

-- name: CreateStockMovement :one
INSERT INTO stock_movements (
    product_id,
    product_uuid,
    change_type,
    quantity_change,
    stock_before,
    stock_after,
    reference_id,
    reference_type,
    actor_id,
    actor_uuid,
    actor_name,
    note
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: GetMovementByRefAndProductUUID :one
SELECT * FROM stock_movements
WHERE reference_type = $1 AND reference_id = $2 AND product_uuid = $3
LIMIT 1;

-- name: GetMovementByRefAndProductID :one
SELECT * FROM stock_movements
WHERE reference_type = $1 AND reference_id = $2 AND product_id = $3
LIMIT 1;

-- name: ListStockMovements :many
SELECT
    sm.movement_id,
    sm.movement_uuid,
    sm.product_id,
    sm.product_uuid,
    sm.change_type,
    sm.quantity_change,
    sm.stock_before,
    sm.stock_after,
    sm.reference_id,
    sm.reference_type,
    sm.actor_id,
    sm.actor_uuid,
    sm.actor_name,
    sm.note,
    sm.created_at,
    COALESCE(s.product_name, '') as product_name,
    COALESCE(s.product_sku, '') as product_sku,
    COALESCE(s.product_image_url, '') as product_image_url
FROM stock_movements sm
LEFT JOIN inventory_stocks s ON s.product_uuid = sm.product_uuid
WHERE (
    sqlc.narg(product_uuid)::uuid IS NULL
    OR sm.product_uuid = sqlc.narg(product_uuid)::uuid
) AND (
    sqlc.narg(change_type)::VARCHAR IS NULL
    OR sm.change_type = sqlc.narg(change_type)::VARCHAR
) AND (
    sqlc.narg(from_date)::TIMESTAMPTZ IS NULL
    OR sm.created_at >= sqlc.narg(from_date)::TIMESTAMPTZ
) AND (
    sqlc.narg(to_date)::TIMESTAMPTZ IS NULL
    OR sm.created_at <= sqlc.narg(to_date)::TIMESTAMPTZ
) AND (
    sqlc.narg(search)::TEXT IS NULL
    OR s.product_name ILIKE '%' || sqlc.narg(search)::TEXT || '%'
    OR s.product_sku ILIKE '%' || sqlc.narg(search)::TEXT || '%'
    OR sm.reference_id ILIKE '%' || sqlc.narg(search)::TEXT || '%'
)
ORDER BY sm.created_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountStockMovements :one
SELECT count(*)
FROM stock_movements sm
LEFT JOIN inventory_stocks s ON s.product_uuid = sm.product_uuid
WHERE (
    sqlc.narg(product_uuid)::uuid IS NULL
    OR sm.product_uuid = sqlc.narg(product_uuid)::uuid
) AND (
    sqlc.narg(change_type)::VARCHAR IS NULL
    OR sm.change_type = sqlc.narg(change_type)::VARCHAR
) AND (
    sqlc.narg(from_date)::TIMESTAMPTZ IS NULL
    OR sm.created_at >= sqlc.narg(from_date)::TIMESTAMPTZ
) AND (
    sqlc.narg(to_date)::TIMESTAMPTZ IS NULL
    OR sm.created_at <= sqlc.narg(to_date)::TIMESTAMPTZ
) AND (
    sqlc.narg(search)::TEXT IS NULL
    OR s.product_name ILIKE '%' || sqlc.narg(search)::TEXT || '%'
    OR s.product_sku ILIKE '%' || sqlc.narg(search)::TEXT || '%'
    OR sm.reference_id ILIKE '%' || sqlc.narg(search)::TEXT || '%'
);
