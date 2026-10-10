-- =============================================================================
-- Migration 000016: Decouple Identity Foreign Keys for Microservices Architecture
-- Goal: Remove physical PostgreSQL Foreign Key constraints referencing users.user_id
-- to allow splitting the Identity Service into an independent database without FK errors.
-- Logical integrity is maintained via indexed IDs and user_uuid.
-- =============================================================================

-- 1. Orders Domain
ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_order_user_id_fkey;
ALTER TABLE order_returns DROP CONSTRAINT IF EXISTS order_returns_return_user_id_fkey;
CREATE INDEX IF NOT EXISTS idx_orders_order_user_id ON orders(order_user_id);
CREATE INDEX IF NOT EXISTS idx_order_returns_user_id ON order_returns(return_user_id);

-- 2. Cart Domain
ALTER TABLE carts DROP CONSTRAINT IF EXISTS carts_cart_user_id_fkey;
CREATE INDEX IF NOT EXISTS idx_carts_cart_user_id ON carts(cart_user_id);

-- 3. Promotion / Coupon Domain
ALTER TABLE coupon_usages DROP CONSTRAINT IF EXISTS coupon_usages_coupon_usage_user_id_fkey;
CREATE INDEX IF NOT EXISTS idx_coupon_usages_user_id ON coupon_usages(coupon_usage_user_id);

-- 4. Review Domain
ALTER TABLE product_reviews DROP CONSTRAINT IF EXISTS product_reviews_review_user_id_fkey;
ALTER TABLE product_review_helpful_votes DROP CONSTRAINT IF EXISTS product_review_helpful_votes_user_id_fkey;
ALTER TABLE product_review_reports DROP CONSTRAINT IF EXISTS product_review_reports_user_id_fkey;
CREATE INDEX IF NOT EXISTS idx_product_reviews_user_id ON product_reviews(review_user_id);
CREATE INDEX IF NOT EXISTS idx_product_review_helpful_votes_user_id ON product_review_helpful_votes(user_id);
CREATE INDEX IF NOT EXISTS idx_product_review_reports_user_id ON product_review_reports(user_id);

-- 5. Notification Domain
ALTER TABLE notifications DROP CONSTRAINT IF EXISTS notifications_notification_user_id_fkey;
CREATE INDEX IF NOT EXISTS idx_notifications_user_id ON notifications(notification_user_id);

-- 6. Inventory / Stock Movement Domain
ALTER TABLE stock_movements DROP CONSTRAINT IF EXISTS stock_movements_actor_id_fkey;
CREATE INDEX IF NOT EXISTS idx_stock_movements_actor_id ON stock_movements(actor_id);

-- 7. Chat Domain
ALTER TABLE chat_conversations DROP CONSTRAINT IF EXISTS chat_conversations_user_id_fkey;
ALTER TABLE chat_conversations DROP CONSTRAINT IF EXISTS chat_conversations_admin_id_fkey;
ALTER TABLE chat_messages DROP CONSTRAINT IF EXISTS chat_messages_sender_id_fkey;
CREATE INDEX IF NOT EXISTS idx_chat_conversations_user_id ON chat_conversations(user_id);
CREATE INDEX IF NOT EXISTS idx_chat_conversations_admin_id ON chat_conversations(admin_id);
CREATE INDEX IF NOT EXISTS idx_chat_messages_sender_id ON chat_messages(sender_id);
