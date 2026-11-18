-- Enables every extension GOshop needs, in every database that needs it.
--
-- Run this with psql, NOT the Supabase SQL editor: the editor wraps the script in
-- a single transaction, and CREATE DATABASE cannot run inside one. psql runs
-- statements one at a time, so this works where the editor does not.
--
--   psql "postgresql://postgres:PASS@db.<ref>.supabase.co:5432/postgres?sslmode=require" `
--        -v ON_ERROR_STOP=1 -f scripts/init_supabase.psql.sql
--
-- Idempotent. Safe to re-run after adding a service.
--
-- Why psql and not the Go services: each service runs its own migrations at
-- boot, so a missing extension surfaces there as a fatal, not here. This file
-- exists to turn that fatal into a no-op before the first boot.

\set ON_ERROR_STOP on

-- 1. The 13 databases. CREATE DATABASE is a statement, not a function, so it
--    has to be generated and executed by the client — that is \gexec's job.
SELECT format('CREATE DATABASE %I', d)
FROM unnest(ARRAY[
  'shopping_identity', 'shopping_coupon', 'shopping_inventory',
  'shopping_catalog', 'shopping_cart', 'shopping_order',
  'shopping_payment', 'shopping_banner', 'shopping_review',
  'shopping_analytics', 'shopping_chat', 'shopping_notification',
  'shopping_recs'
]) AS d
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = d)
\gexec

-- 2. Extensions, one \connect per database that needs them.
\connect shopping_identity
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS unaccent;

\connect shopping_order
CREATE EXTENSION IF NOT EXISTS unaccent;

\connect shopping_review
CREATE EXTENSION IF NOT EXISTS unaccent;

\connect shopping_recs
CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS postgres_fdw;

-- 3. Report. Expect 13 rows; the extension column shows what landed where.
SELECT datname FROM pg_database WHERE datname LIKE 'shopping%' ORDER BY datname;

\connect shopping_recs
SELECT extname, extversion FROM pg_extension
WHERE extname IN ('vector', 'postgres_fdw') ORDER BY extname;
