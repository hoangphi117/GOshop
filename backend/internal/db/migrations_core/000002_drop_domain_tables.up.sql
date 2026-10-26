-- User data lives in the identity service database; carts/messages/stock and
-- similar domain data live in their owning service databases. The monolith DB
-- keeps only the interaction projections below.

DROP TABLE IF EXISTS users CASCADE;

DROP TABLE IF EXISTS user_addresses CASCADE;

DROP TABLE IF EXISTS carts CASCADE;

DROP TABLE IF EXISTS cart_items CASCADE;

DROP TABLE IF EXISTS categories CASCADE;

DROP TABLE IF EXISTS products CASCADE;

DROP TABLE IF EXISTS product_reviews CASCADE;

DROP TABLE IF EXISTS product_review_helpful_votes CASCADE;

DROP TABLE IF EXISTS product_review_reports CASCADE;

DROP TABLE IF EXISTS notifications CASCADE;

DROP TABLE IF EXISTS coupons CASCADE;

DROP TABLE IF EXISTS coupon_usages CASCADE;

DROP TABLE IF EXISTS stock_movements CASCADE;

DROP TABLE IF EXISTS banners CASCADE;

DROP TABLE IF EXISTS chat_conversations CASCADE;

DROP TABLE IF EXISTS chat_messages CASCADE;
