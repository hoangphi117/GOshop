-- Extend the analytics mirror with the two columns affinity reads.
--
-- The original mirror only declared the product-side event columns. Search
-- affinity needs `query` and `result_count`: `search` turns a query into a seed
-- ("this shopper was looking for X"), `search_click` links query -> product
-- ("...and this result answered it").
--
-- MUST run after 000004, which created the table. CREATE FOREIGN TABLE IF NOT
-- EXISTS is a no-op on an existing table, so the columns have to be added
-- explicitly. The options carry over from the original declaration.
ALTER FOREIGN TABLE analytics_mirror.user_events
    ADD COLUMN IF NOT EXISTS query        TEXT,
    ADD COLUMN IF NOT EXISTS result_count INT;
