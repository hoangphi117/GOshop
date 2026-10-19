-- Revert Migration 000023: Restore review_order_id FK and indexes

DROP INDEX IF EXISTS idx_product_reviews_order_uuid;
DROP INDEX IF EXISTS idx_reviews_unique_per_order_uuid;

CREATE UNIQUE INDEX IF NOT EXISTS idx_reviews_unique_per_order 
    ON product_reviews(review_user_id, review_product_id, review_order_id);

ALTER TABLE product_reviews
    ADD CONSTRAINT product_reviews_review_order_id_fkey
    FOREIGN KEY (review_order_id) REFERENCES orders(order_id) ON DELETE CASCADE;

ALTER TABLE product_reviews
    ALTER COLUMN review_order_id SET NOT NULL;

ALTER TABLE product_reviews
    DROP COLUMN IF EXISTS review_order_uuid;
