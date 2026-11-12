-- Widen the catalog mirror with the columns the search facets read.
--
-- Hybrid search filters by category, price and rating, and it does so in SQL so
-- the vector pool cannot shrink under a fixed-size page. The original mirror
-- only declared what the embedder needed (name, description, category, status),
-- so the facet predicate had nothing to compare against.
--
-- category_uuid is the category's public identifier, the value the storefront
-- actually sends. Resolving it through catalog_mirror.products worked only for
-- products a shopper had already viewed: the uuid belongs to the category, and
-- the category is welcome in the index without such a product ever existing.
--
-- A foreign table is a local declaration: adding a column here does not touch
-- the catalog database, and the catalog offers no ALTER on a foreign table, so
-- the declaration has to be replaced rather than extended. The types match the
-- catalog columns exactly; postgres_fdw rejects a remote value it cannot fit.
DROP FOREIGN TABLE IF EXISTS catalog_mirror.products;
DROP FOREIGN TABLE IF EXISTS catalog_mirror.categories;

CREATE FOREIGN TABLE catalog_mirror.products (
    product_uuid        UUID          NOT NULL,
    product_name        VARCHAR(200)  NOT NULL,
    product_description TEXT,
    product_category_id INT           NOT NULL,
    product_status      INT           NOT NULL,
    product_deleted_at  TIMESTAMPTZ,
    product_price       NUMERIC(12,2) NOT NULL,
    product_avg_rating  NUMERIC(2,1)  NOT NULL
) SERVER catalog_db OPTIONS (schema_name 'public', table_name 'products');

CREATE FOREIGN TABLE catalog_mirror.categories (
    category_id   INT          NOT NULL,
    category_uuid UUID         NOT NULL,
    category_name VARCHAR(100) NOT NULL
) SERVER catalog_db OPTIONS (schema_name 'public', table_name 'categories');
