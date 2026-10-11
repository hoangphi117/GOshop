-- Index for joining order_items to products in top selling products analytics
CREATE INDEX IF NOT EXISTS idx_order_items_product_id ON order_items(order_item_product_id);

-- Composite index for fast user cohort analytics
CREATE INDEX IF NOT EXISTS idx_users_level_created_at ON users(user_level, user_created_at) WHERE user_deleted_at IS NULL;
