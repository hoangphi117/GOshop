-- Phase A: the visitor whose pass becomes "the account's history".
--   dddd0001 = the anon id the harness calls use.
\set ON_ERROR_STOP on
\pset pager off
\connect shopping_analytics

\echo '--- BEFORE ---'
SELECT count(*) AS events_for_visitor
  FROM user_events
 WHERE anon_id = 'dddd0001-dddd-4ddd-8ddd-ddddddddddd1';

-- ONE pass, three products. Three brands on purpose: diversify() round-robins
-- by brand and can only fan out as wide as the brands it was given, so a
-- one-brand pass returns barely the items it names and the shelf comes back too
-- short to rank (the tier correctly falls through to best sellers).
--
-- Three products is also what clears sessionThreshold, so this visitor is a
-- profile-tier visitor as soon as the shelf is computed with an account.
INSERT INTO user_events (event_id, event_type, anon_id, session_id, product_uuid, occurred_at) VALUES
  (gen_random_uuid(), 'view_item', 'dddd0001-dddd-4ddd-8ddd-ddddddddddd1', '55550000-0000-4000-8000-000000000001', 'b9a496f9-88ac-4e9b-bb20-fa4fab736d24', now() - interval '5 minutes'),
  (gen_random_uuid(), 'view_item', 'dddd0001-dddd-4ddd-8ddd-ddddddddddd1', '55550000-0000-4000-8000-000000000001', '4dec425f-b0ea-466a-aa3b-641d7bfff67d', now() - interval '4 minutes'),
  (gen_random_uuid(), 'view_item', 'dddd0001-dddd-4ddd-8ddd-ddddddddddd1', '55550000-0000-4000-8000-000000000001', '0123ba94-fae1-407f-a5d7-7936dc836191', now() - interval '3 minutes'),
  -- A search too, so search affinity has a seed.
  (gen_random_uuid(), 'search',    'dddd0001-dddd-4ddd-8ddd-ddddddddddd1', '55550000-0000-4000-8000-000000000001', NULL, now() - interval '2 minutes');

\echo '--- AFTER ---'
SELECT count(*) AS events_for_visitor
  FROM user_events
 WHERE anon_id = 'dddd0001-dddd-4ddd-8ddd-ddddddddddd1';

SELECT event_type, count(*)
  FROM user_events
 WHERE anon_id = 'dddd0001-dddd-4ddd-8ddd-ddddddddddd1'
 GROUP BY 1 ORDER BY 1;
