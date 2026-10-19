-- Migration 000023: Add review_order_uuid and drop cross-domain foreign key from product_reviews to orders

-- 1. Add review_order_uuid column
ALTER TABLE product_reviews
    ADD COLUMN IF NOT EXISTS review_order_uuid UUID;

-- 2. Backfill review_order_uuid from orders table
UPDATE product_reviews pr
SET review_order_uuid = o.order_uuid
FROM orders o
WHERE pr.review_order_id = o.order_id
  AND pr.review_order_uuid IS NULL;

-- In case of orphaned or test review records, populate with a generated UUID
UPDATE product_reviews
SET review_order_uuid = gen_random_uuid()
WHERE review_order_uuid IS NULL;

-- 3. Set NOT NULL on review_order_uuid and allow review_order_id to be nullable
ALTER TABLE product_reviews
    ALTER COLUMN review_order_uuid SET NOT NULL;

ALTER TABLE product_reviews
    ALTER COLUMN review_order_id DROP NOT NULL;

-- 4. Drop Foreign Key constraint to orders table
ALTER TABLE product_reviews
    DROP CONSTRAINT IF EXISTS product_reviews_review_order_id_fkey;

-- 5. Drop old unique index on review_order_id and create new unique index on review_order_uuid
DROP INDEX IF EXISTS idx_reviews_unique_per_order;

CREATE UNIQUE INDEX IF NOT EXISTS idx_reviews_unique_per_order_uuid
    ON product_reviews(review_user_id, review_product_id, review_order_uuid);

CREATE INDEX IF NOT EXISTS idx_product_reviews_order_uuid
    ON product_reviews(review_order_uuid);
