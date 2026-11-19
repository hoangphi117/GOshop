-- Backfill the Meilisearch index for products that predate it.
--
-- Run once in the Supabase SQL editor after MEILISEARCH_HOST/KEY are set on
-- Render. The catalog outbox worker polls every 2s and pushes any PENDING
-- PRODUCT_UPSERT event to Meilisearch, then marks it PROCESSED — so inserting
-- rows here is the whole job; no restart or deploy is needed.
--
-- Safe to re-run: it only queues events for products that have none pending,
-- and the worker's MarkOutboxEventProcessed makes a duplicate index a no-op.
--
-- The payload must match search.ProductDocument's JSON tags exactly, because
-- the worker unmarshals it straight into that struct. If it does not parse,
-- the worker falls back to GetProductByUUID by aggregate_id, so a malformed
-- payload degrades to a slower correct path rather than dropping the product.

INSERT INTO outbox_events (aggregate_type, aggregate_id, event_type, payload)
SELECT
    'product',
    p.product_uuid::text,
    'PRODUCT_UPSERT',
    jsonb_build_object(
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
    )
FROM products p
JOIN categories c ON c.category_id = p.product_category_id
WHERE p.product_deleted_at IS NULL
  AND NOT EXISTS (
      SELECT 1 FROM outbox_events o
      WHERE o.aggregate_id = p.product_uuid::text
        AND o.event_type = 'PRODUCT_UPSERT'
        AND o.status = 'PENDING'
  );

-- Expect one row per live product (currently ~5000). The worker drains them at
-- GetPendingOutboxEvents' batch size per 2s tick; a few hundred rows takes
-- under a minute, several thousand takes a few.
SELECT status, count(*) FROM outbox_events
WHERE event_type = 'PRODUCT_UPSERT'
GROUP BY status;
