DROP TRIGGER IF EXISTS set_review_updated_at ON product_reviews;
DROP FUNCTION IF EXISTS set_review_updated_at_fn();
DROP INDEX IF EXISTS idx_reviews_unique_per_order;
DROP INDEX IF EXISTS idx_reviews_user;
DROP INDEX IF EXISTS idx_reviews_product;
DROP INDEX IF EXISTS idx_reviews_product_filter;
DROP TABLE IF EXISTS product_review_reports;
DROP TABLE IF EXISTS product_review_helpful_votes;
DROP TABLE IF EXISTS product_reviews;

ALTER TABLE products
    DROP COLUMN IF EXISTS product_avg_rating,
    DROP COLUMN IF EXISTS product_review_count;
