DROP TRIGGER IF EXISTS touch_cart_after_item_change ON cart_items;
DROP FUNCTION IF EXISTS touch_cart_on_item_change();
DROP TRIGGER IF EXISTS set_cart_item_updated_at ON cart_items;
DROP FUNCTION IF EXISTS update_cart_item_updated_at();
DROP TRIGGER IF EXISTS set_cart_updated_at ON carts;
DROP FUNCTION IF EXISTS update_cart_updated_at();

DROP TABLE IF EXISTS cart_items;
DROP TABLE IF EXISTS carts;
