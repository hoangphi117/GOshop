-- Migration 000019 down: Restore cross-boundary Foreign Keys

-- 1. order_items -> products
ALTER TABLE order_items
    ADD CONSTRAINT order_items_order_item_product_id_fkey
    FOREIGN KEY (order_item_product_id) REFERENCES products(product_id);

-- 2. cart_items -> products
ALTER TABLE cart_items
    ADD CONSTRAINT cart_items_cart_item_product_id_fkey
    FOREIGN KEY (cart_item_product_id) REFERENCES products(product_id) ON DELETE CASCADE;

-- 3. stock_movements -> products
ALTER TABLE stock_movements
    ADD CONSTRAINT stock_movements_product_id_fkey
    FOREIGN KEY (product_id) REFERENCES products(product_id) ON DELETE CASCADE;

-- 4. coupon_usages -> orders
ALTER TABLE coupon_usages
    ADD CONSTRAINT coupon_usages_coupon_usage_order_id_fkey
    FOREIGN KEY (coupon_usage_order_id) REFERENCES orders(order_id) ON DELETE SET NULL;

-- 5. orders -> coupons
ALTER TABLE orders
    ADD CONSTRAINT orders_order_coupon_id_fkey
    FOREIGN KEY (order_coupon_id) REFERENCES coupons(coupon_id);

-- 6. payment_transactions -> orders
ALTER TABLE payment_transactions
    ADD CONSTRAINT payment_transactions_pt_order_id_fkey
    FOREIGN KEY (pt_order_id) REFERENCES orders(order_id);
