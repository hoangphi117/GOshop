-- =============================================================================
-- Migration 000016 Down: Restore Physical Foreign Key Constraints referencing users
-- =============================================================================

-- 1. Orders Domain
ALTER TABLE orders ADD CONSTRAINT orders_order_user_id_fkey FOREIGN KEY (order_user_id) REFERENCES users(user_id);
ALTER TABLE order_returns ADD CONSTRAINT order_returns_return_user_id_fkey FOREIGN KEY (return_user_id) REFERENCES users(user_id);

-- 2. Cart Domain
ALTER TABLE carts ADD CONSTRAINT carts_cart_user_id_fkey FOREIGN KEY (cart_user_id) REFERENCES users(user_id) ON DELETE CASCADE;

-- 3. Promotion / Coupon Domain
ALTER TABLE coupon_usages ADD CONSTRAINT coupon_usages_coupon_usage_user_id_fkey FOREIGN KEY (coupon_usage_user_id) REFERENCES users(user_id) ON DELETE CASCADE;

-- 4. Review Domain
ALTER TABLE product_reviews ADD CONSTRAINT product_reviews_review_user_id_fkey FOREIGN KEY (review_user_id) REFERENCES users(user_id) ON DELETE CASCADE;
ALTER TABLE product_review_helpful_votes ADD CONSTRAINT product_review_helpful_votes_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE;
ALTER TABLE product_review_reports ADD CONSTRAINT product_review_reports_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE;

-- 5. Notification Domain
ALTER TABLE notifications ADD CONSTRAINT notifications_notification_user_id_fkey FOREIGN KEY (notification_user_id) REFERENCES users(user_id) ON DELETE CASCADE;

-- 6. Inventory / Stock Movement Domain
ALTER TABLE stock_movements ADD CONSTRAINT stock_movements_actor_id_fkey FOREIGN KEY (actor_id) REFERENCES users(user_id) ON DELETE SET NULL;

-- 7. Chat Domain
ALTER TABLE chat_conversations ADD CONSTRAINT chat_conversations_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE;
ALTER TABLE chat_conversations ADD CONSTRAINT chat_conversations_admin_id_fkey FOREIGN KEY (admin_id) REFERENCES users(user_id) ON DELETE SET NULL;
ALTER TABLE chat_messages ADD CONSTRAINT chat_messages_sender_id_fkey FOREIGN KEY (sender_id) REFERENCES users(user_id) ON DELETE CASCADE;
