DROP TABLE IF EXISTS product_review_reports;
DROP TABLE IF EXISTS product_review_helpful_votes;
ALTER TABLE product_reviews DROP COLUMN IF EXISTS review_helpful_count;
