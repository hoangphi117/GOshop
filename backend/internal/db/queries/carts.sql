-- name: GetOrCreateCartByUserID :one
-- Upsert "no-op": if cart already exists for user, return it; otherwise create a new cart and return it.
-- Atomic, safe for concurrent requests, and avoids race conditions by using INSERT ... ON CONFLICT.
INSERT INTO carts (cart_user_id)
VALUES (sqlc.arg(user_id))
ON CONFLICT (cart_user_id) DO UPDATE
SET cart_user_id = carts.cart_user_id -- no-op, chỉ để RETURNING hoạt động khi đã tồn tại
RETURNING *;

-- name: GetCartItemsWithProductDetails :many
-- JOIN directly with products to get the LATEST price/inventory/status at the time of reading.
-- product_deleted_at and product_status are returned so the service layer can automatically calculate is_available.
SELECT
    cart_items.cart_item_uuid,
    cart_items.cart_item_quantity,
    cart_items.cart_item_created_at,
    products.product_uuid,
    products.product_name,
    products.product_slug,
    products.product_image_url,
    products.product_price,
    products.product_stock,
    products.product_status,
    products.product_deleted_at
FROM cart_items
JOIN products ON products.product_id = cart_items.cart_item_product_id
WHERE cart_items.cart_item_cart_id = sqlc.arg(cart_id)
ORDER BY cart_items.cart_item_created_at ASC;

-- name: GetCartByUserID :one
-- Khác GetOrCreateCartByUserID: KHÔNG tự tạo cart nếu chưa có. Dùng cho
-- CleanupCartAfterOrder — nếu user chưa từng có giỏ hàng (mua thẳng, không qua
-- cart), không cần thiết phải tạo mới 1 cart rỗng chỉ để phục vụ bước dọn dẹp.
SELECT * FROM carts
WHERE cart_user_id = sqlc.arg(user_id);

-- name: GetCartItemQuantity :one
-- Dùng cho soft-check tồn kho trước khi AddOrUpdateCartItem (Key Design Decision #2).
SELECT cart_item_quantity FROM cart_items
WHERE cart_item_cart_id = sqlc.arg(cart_id)
AND cart_item_product_id = sqlc.arg(product_id);

-- name: AddOrUpdateCartItem :one
-- Atomic increment, giới hạn trần 99 ngay trong SQL — không cần đọc trước rồi
-- ghi lại (tránh check-then-act race hoàn toàn cho riêng bước tăng số lượng).
-- Stock check thực hiện ở tầng service TRƯỚC khi gọi query này — không block cứng ở đây vì Order mới là nơi enforce
-- stock nghiêm ngặt.
INSERT INTO cart_items (cart_item_cart_id, cart_item_product_id, cart_item_quantity)
VALUES (sqlc.arg(cart_id), sqlc.arg(product_id), sqlc.arg(quantity))
ON CONFLICT (cart_item_cart_id, cart_item_product_id)
DO UPDATE SET cart_item_quantity = LEAST(cart_items.cart_item_quantity + excluded.cart_item_quantity, 99)
RETURNING *;

-- name: SetCartItemQuantity :one
-- Dùng cho UpdateQuantity (ghi đè tuyệt đối, không cộng dồn) — khác AddOrUpdateCartItem.
UPDATE cart_items
SET cart_item_quantity = sqlc.arg(quantity)
WHERE cart_item_uuid = sqlc.arg(cart_item_uuid)
AND cart_item_cart_id = sqlc.arg(cart_id)
RETURNING *;

-- name: DeleteCartItemByUUID :one
DELETE FROM cart_items
WHERE cart_item_uuid = sqlc.arg(cart_item_uuid)
AND cart_item_cart_id = sqlc.arg(cart_id)
RETURNING *;

-- name: DeleteCartItemsByProductIDs :exec
-- Dùng cho bước "dọn giỏ hàng sau khi đặt" — xóa CHỌN
-- LỌC theo danh sách product_id có trong đơn vừa tạo, không xóa sạch toàn bộ giỏ.
DELETE FROM cart_items
WHERE cart_item_cart_id = sqlc.arg(cart_id)
AND cart_item_product_id = ANY(sqlc.arg(product_ids)::int[]);

-- name: ClearCartByCartID :exec
DELETE FROM cart_items
WHERE cart_item_cart_id = sqlc.arg(cart_id);

-- name: CountCartItems :one
SELECT count(*) FROM cart_items
WHERE cart_item_cart_id = sqlc.arg(cart_id);

