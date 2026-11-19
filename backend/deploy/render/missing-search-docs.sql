-- Emit the ProductDocument JSON for products missing from Meilisearch.
--
-- Run from the directory holding missing_ids.txt (one product_uuid per line):
--   docker run --rm -v "${PWD}:/w" -w /w -i postgres:16-alpine \
--     psql "$CATALOG_DB_URL" -At -f /dev/stdin < missing-search-docs.sql > missing.json
--   then POST missing.json to Meilisearch's documents endpoint.
--
-- Field names must match search.ProductDocument's JSON tags: a wrong tag
-- indexes a field nothing queries, and a wrong type makes Meilisearch reject
-- the whole batch.

CREATE TEMP TABLE missing_ids (id text PRIMARY KEY);

\copy missing_ids FROM 'missing_ids.txt'

SELECT json_agg(doc)
FROM (
    SELECT jsonb_build_object(
        'id',                   p.product_uuid::text,
        'product_id',           p.product_id,
        'product_name',         p.product_name,
        'product_slug',         p.product_slug,
        'product_description',  COALESCE(p.product_description, ''),
        'product_price',        p.product_price::float8,
        'product_stock',        0,
        'product_sku',          COALESCE(p.product_sku, ''),
        'category_uuid',        c.category_uuid::text,
        'category_name',        c.category_name,
        'product_image_url',    COALESCE(p.product_image_url, ''),
        'product_status',       p.product_status,
        'product_created_at',   EXTRACT(EPOCH FROM p.product_created_at)::bigint
    ) AS doc
    FROM products p
    JOIN categories c ON c.category_id = p.product_category_id
    JOIN missing_ids m ON m.id = p.product_uuid::text
    WHERE p.product_deleted_at IS NULL
    ORDER BY p.product_id
) s;
