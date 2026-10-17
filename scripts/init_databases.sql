-- =============================================================================
-- GOSHOP MULTI-DATABASE INITIALIZATION SCRIPT
-- Runs on PostgreSQL container initialization (/docker-entrypoint-initdb.d/)
-- =============================================================================

SELECT 'CREATE DATABASE shopping_identity'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'shopping_identity')\gexec

SELECT 'CREATE DATABASE shopping_coupon'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'shopping_coupon')\gexec
