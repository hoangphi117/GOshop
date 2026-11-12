DROP TABLE IF EXISTS user_affinity;
DROP TABLE IF EXISTS product_embedding;
-- The extension is left installed: other schemas may have started using it,
-- and DROP EXTENSION CASCADE here would silently take their columns with it.
