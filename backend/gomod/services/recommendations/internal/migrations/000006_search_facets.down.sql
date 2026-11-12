-- Drop the facet columns by restoring the 000003 declaration.
DROP FOREIGN TABLE IF EXISTS catalog_mirror.products;
DROP FOREIGN TABLE IF EXISTS catalog_mirror.categories;

CREATE FOREIGN TABLE catalog_mirror.products (
    product_uuid        UUID         NOT NULL,
    product_name        VARCHAR(200) NOT NULL,
    product_description TEXT,
    product_category_id INT          NOT NULL,
    product_status      INT          NOT NULL,
    product_deleted_at  TIMESTAMPTZ
) SERVER catalog_db OPTIONS (schema_name 'public', table_name 'products');

CREATE FOREIGN TABLE catalog_mirror.categories (
    category_id   INT          NOT NULL,
    category_name VARCHAR(100) NOT NULL
) SERVER catalog_db OPTIONS (schema_name 'public', table_name 'categories');
