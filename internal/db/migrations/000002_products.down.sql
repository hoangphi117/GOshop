DROP TRIGGER IF EXISTS set_products_updated_at ON products;
DROP FUNCTION IF EXISTS update_products_updated_at();
DROP INDEX IF EXISTS idx_products_list;
DROP INDEX IF EXISTS idx_products_slug;
DROP TABLE IF EXISTS products;

DROP TRIGGER IF EXISTS set_categories_updated_at ON categories;
DROP FUNCTION IF EXISTS update_categories_updated_at();
DROP INDEX IF EXISTS idx_categories_list;
DROP INDEX IF EXISTS idx_categories_slug;
DROP INDEX IF EXISTS idx_categories_name_unique;
DROP TABLE IF EXISTS categories;


