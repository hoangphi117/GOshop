-- =============================================================================
-- GOSHOP MULTI-DATABASE INITIALIZATION SCRIPT
-- Runs on PostgreSQL container initialization (/docker-entrypoint-initdb.d/)
-- =============================================================================

SELECT 'CREATE DATABASE shopping_identity'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'shopping_identity')\gexec

SELECT 'CREATE DATABASE shopping_coupon'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'shopping_coupon')\gexec

SELECT 'CREATE DATABASE shopping_inventory'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'shopping_inventory')\gexec

SELECT 'CREATE DATABASE shopping_catalog'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'shopping_catalog')\gexec

SELECT 'CREATE DATABASE shopping_cart'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'shopping_cart')\gexec

SELECT 'CREATE DATABASE shopping_order'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'shopping_order')\gexec

SELECT 'CREATE DATABASE shopping_payment'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'shopping_payment')\gexec

SELECT 'CREATE DATABASE shopping_banner'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'shopping_banner')\gexec

SELECT 'CREATE DATABASE shopping_review'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'shopping_review')\gexec




SELECT 'CREATE DATABASE shopping_analytics'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'shopping_analytics')\gexec

SELECT 'CREATE DATABASE shopping_chat'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'shopping_chat')\gexec

SELECT 'CREATE DATABASE shopping_notification'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'shopping_notification')\gexec

-- The assistant keeps its transcript in its own database (cmd/assistant stamps
-- DBName=shopping_assistant), so the other services' migration sets cannot
-- collide with schema_migrations_assistant.
SELECT 'CREATE DATABASE shopping_assistant'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'shopping_assistant')\gexec

-- Personalization store: affinity, popularity windows and product embeddings.
-- Owned by services/recommendations (phase 2); created here so phase 1 can wire
-- the consumer against a database that already exists.
SELECT 'CREATE DATABASE shopping_recs'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'shopping_recs')\gexec