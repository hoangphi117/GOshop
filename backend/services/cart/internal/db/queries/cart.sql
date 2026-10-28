-- name: GetOrCreateCartByUserUUID :one
INSERT INTO carts (user_uuid, user_id)
VALUES (sqlc.arg(user_uuid), sqlc.narg(user_id))
ON CONFLICT (user_uuid) DO UPDATE
SET user_id = COALESCE(sqlc.narg(user_id), carts.user_id),
    cart_updated_at = now()
RETURNING *;

-- name: GetCartByUserUUID :one
SELECT * FROM carts
WHERE user_uuid = sqlc.arg(user_uuid);

-- name: GetCartByUserID :one
SELECT * FROM carts
WHERE user_id = sqlc.arg(user_id);

-- name: GetCartByCartID :one
SELECT * FROM carts
WHERE cart_id = sqlc.arg(cart_id);

-- name: GetCartByCartUUID :one
SELECT * FROM carts
WHERE cart_uuid = sqlc.arg(cart_uuid);

-- name: GetCartItems :many
SELECT * FROM cart_items
WHERE cart_id = sqlc.arg(cart_id)
ORDER BY cart_item_created_at ASC;

-- name: GetCartItemByUUID :one
SELECT * FROM cart_items
WHERE cart_item_uuid = sqlc.arg(cart_item_uuid)
  AND cart_id = sqlc.arg(cart_id);

-- name: GetCartItemQuantity :one
SELECT quantity FROM cart_items
WHERE cart_id = sqlc.arg(cart_id)
  AND product_uuid = sqlc.arg(product_uuid);

-- name: AddOrUpdateCartItem :one
INSERT INTO cart_items (cart_id, product_uuid, product_id, quantity)
VALUES (sqlc.arg(cart_id), sqlc.arg(product_uuid), sqlc.narg(product_id), sqlc.arg(quantity))
ON CONFLICT (cart_id, product_uuid)
DO UPDATE SET
    quantity = LEAST(cart_items.quantity + excluded.quantity, 99),
    product_id = COALESCE(excluded.product_id, cart_items.product_id),
    cart_item_updated_at = now()
RETURNING *;

-- name: SetCartItemQuantity :one
UPDATE cart_items
SET quantity = sqlc.arg(quantity),
    cart_item_updated_at = now()
WHERE cart_item_uuid = sqlc.arg(cart_item_uuid)
  AND cart_id = sqlc.arg(cart_id)
RETURNING *;

-- name: DeleteCartItemByUUID :one
DELETE FROM cart_items
WHERE cart_item_uuid = sqlc.arg(cart_item_uuid)
  AND cart_id = sqlc.arg(cart_id)
RETURNING *;

-- name: DeleteCartItemsByProductUUIDs :exec
DELETE FROM cart_items
WHERE cart_id = sqlc.arg(cart_id)
  AND product_uuid = ANY(sqlc.arg(product_uuids)::uuid[]);

-- name: DeleteCartItemsByProductIDs :exec
DELETE FROM cart_items
WHERE cart_id = sqlc.arg(cart_id)
  AND product_id = ANY(sqlc.arg(product_ids)::int[]);

-- name: ClearCartByCartID :exec
DELETE FROM cart_items
WHERE cart_id = sqlc.arg(cart_id);

-- name: CountCartItems :one
SELECT count(*) FROM cart_items
WHERE cart_id = sqlc.arg(cart_id);