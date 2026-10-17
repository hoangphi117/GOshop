-- Migration 000019: Drop cross-boundary Foreign Keys for Order, Cart, Coupon, Inventory, and Product decoupling

-- 1. order_items -> products (Catalog boundary)
ALTER TABLE order_items
    DROP CONSTRAINT IF EXISTS order_items_order_item_product_id_fkey;

CREATE INDEX IF NOT EXISTS idx_order_items_product_id ON order_items(order_item_product_id);

-- 2. cart_items -> products (Catalog boundary)
ALTER TABLE cart_items
    DROP CONSTRAINT IF EXISTS cart_items_cart_item_product_id_fkey;

CREATE INDEX IF NOT EXISTS idx_cart_items_product_id ON cart_items(cart_item_product_id);

-- 3. stock_movements -> products (Catalog boundary)
ALTER TABLE stock_movements
    DROP CONSTRAINT IF EXISTS stock_movements_product_id_fkey;

CREATE INDEX IF NOT EXISTS idx_stock_movements_product_id ON stock_movements(product_id);

-- 4. coupon_usages -> orders (Order boundary)
ALTER TABLE coupon_usages
    DROP CONSTRAINT IF EXISTS coupon_usages_coupon_usage_order_id_fkey;

CREATE INDEX IF NOT EXISTS idx_coupon_usages_order_id ON coupon_usages(coupon_usage_order_id);

-- 5. orders -> coupons (Coupon boundary)
ALTER TABLE orders
    DROP CONSTRAINT IF EXISTS orders_order_coupon_id_fkey;

CREATE INDEX IF NOT EXISTS idx_orders_coupon_id ON orders(order_coupon_id);

-- 6. payment_transactions -> orders (Payment boundary)
ALTER TABLE payment_transactions
    DROP CONSTRAINT IF EXISTS payment_transactions_pt_order_id_fkey;

CREATE INDEX IF NOT EXISTS idx_payment_transactions_order_id ON payment_transactions(pt_order_id);
