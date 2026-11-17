-- Phase B: who the session-pass visitor signs in as.
--
-- The analytics handler takes user_id from the JWT, never from the body, so an
-- honest login bridge writes a row carrying BOTH ids: the anon one and the
-- account's. That single row is what lets `visitorPredicate` join the two
-- trails. Migration 000002 of the analytics service is deliberately NOT used
-- here — it copies the anon trail onto the user at migration time, which is the
-- thing under test.
\set ON_ERROR_STOP on
\pset pager off
\connect shopping_analytics

\echo '--- row carrying both ids (the bridge) ---'
INSERT INTO user_events (event_id, event_type, anon_id, user_id, session_id, product_uuid, occurred_at) VALUES
  (gen_random_uuid(), 'view_item', 'dddd0001-dddd-4ddd-8ddd-ddddddddddd1', 1, '55550000-0000-4000-8000-000000000002', 'b9a496f9-88ac-4e9b-bb20-fa4fab736d24', now() - interval '30 seconds');

SELECT user_id, count(*) AS bridged_rows
  FROM user_events
 WHERE anon_id = 'dddd0001-dddd-4ddd-8ddd-ddddddddddd1'
   AND user_id IS NOT NULL
 GROUP BY 1;

\echo '--- what the identity bridge makes visible to the account ---'
\connect shopping_recs
SELECT count(*) AS bridged_anon_ids
  FROM analytics_mirror.user_events
 WHERE user_id = 1 AND anon_id IS NOT NULL;
