-- Catalog tables mirrored for embedding.
--
-- Same reason as the order mirror: the catalog lives in its own database
-- (shopping_catalog) and PostgreSQL cannot join across databases. The server
-- and user mapping (catalog_db) are created by ensureCatalogServer in app.go
-- because they carry credentials a migration cannot read.
--
-- These foreign tables are read-only here; writes stay in the catalog service.
-- Only the columns the embedder needs are mirrored, so a catalog schema change
-- outside them cannot break this migration.

CREATE SCHEMA IF NOT EXISTS catalog_mirror;

CREATE FOREIGN TABLE IF NOT EXISTS catalog_mirror.products (
    product_uuid        UUID         NOT NULL,
    product_name        VARCHAR(200) NOT NULL,
    product_description TEXT,
    product_category_id INT          NOT NULL,
    product_status      INT          NOT NULL,
    product_deleted_at  TIMESTAMPTZ
) SERVER catalog_db OPTIONS (schema_name 'public', table_name 'products');

CREATE FOREIGN TABLE IF NOT EXISTS catalog_mirror.categories (
    category_id   INT          NOT NULL,
    category_name VARCHAR(100) NOT NULL
) SERVER catalog_db OPTIONS (schema_name 'public', table_name 'categories');
