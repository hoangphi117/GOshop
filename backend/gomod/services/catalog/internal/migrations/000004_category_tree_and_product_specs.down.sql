DROP INDEX IF EXISTS idx_products_specifications;
ALTER TABLE products DROP COLUMN IF EXISTS product_specifications;

DROP INDEX IF EXISTS idx_categories_parent_id;
ALTER TABLE categories DROP COLUMN IF EXISTS category_level;
ALTER TABLE categories DROP COLUMN IF EXISTS parent_id;
