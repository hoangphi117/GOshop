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
    note
)
VALUES ($1, (SELECT product_uuid FROM products WHERE product_id = $1), $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListStockMovements :many
SELECT
    sm.movement_id,
    sm.movement_uuid,
    sm.product_id,
    sm.change_type,
    sm.quantity_change,
    sm.stock_before,
    sm.stock_after,
    sm.reference_id,
    sm.reference_type,
    sm.actor_id,
    sm.note,
    sm.created_at,
    p.product_uuid,
    p.product_name,
    p.product_sku,
    p.product_image_url,
    sm.actor_uuid,
    sm.actor_name
FROM stock_movements sm
JOIN products p ON p.product_id = sm.product_id
WHERE (
    sqlc.narg(product_uuid)::uuid IS NULL
    OR p.product_uuid = sqlc.narg(product_uuid)::uuid
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
    OR p.product_name ILIKE '%' || sqlc.narg(search)::TEXT || '%'
    OR p.product_sku ILIKE '%' || sqlc.narg(search)::TEXT || '%'
    OR sm.reference_id ILIKE '%' || sqlc.narg(search)::TEXT || '%'
)
ORDER BY sm.created_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountStockMovements :one
SELECT count(*)
FROM stock_movements sm
JOIN products p ON p.product_id = sm.product_id
WHERE (
    sqlc.narg(product_uuid)::uuid IS NULL
    OR p.product_uuid = sqlc.narg(product_uuid)::uuid
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
    OR p.product_name ILIKE '%' || sqlc.narg(search)::TEXT || '%'
    OR p.product_sku ILIKE '%' || sqlc.narg(search)::TEXT || '%'
    OR sm.reference_id ILIKE '%' || sqlc.narg(search)::TEXT || '%'
);
