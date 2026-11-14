-- Phase C: other shoppers, so a trend exists.
--
-- Each anon id is a different device. The trending tier sums purchased units
-- (props.quantity) over the last day and needs at least TrendingMinimum = 2
-- rows, otherwise the caller falls through to the all-time list.
\set ON_ERROR_STOP on
\pset pager off
\connect shopping_analytics

INSERT INTO user_events (event_id, event_type, anon_id, product_uuid, props, occurred_at) VALUES
  (gen_random_uuid(), 'purchase', 'dddd0002-dddd-4ddd-8ddd-ddddddddddd2', 'b9a496f9-88ac-4e9b-bb20-fa4fab736d24', '{"quantity": 5}'::jsonb, now() - interval '2 hours'),
  (gen_random_uuid(), 'purchase', 'dddd0003-dddd-4ddd-8ddd-ddddddddddd3', '4dec425f-b0ea-466a-aa3b-641d7bfff67d', '{"quantity": 3}'::jsonb, now() - interval '1 hour'),
  (gen_random_uuid(), 'purchase', 'dddd0004-dddd-4ddd-8ddd-ddddddddddd4', '0123ba94-fae1-407f-a5d7-7936dc836191', '{"quantity": 1}'::jsonb, now() - interval '30 minutes');

\echo '--- what the trending query sees ---'
\connect shopping_recs
SELECT product_uuid,
       SUM(COALESCE((props->>'quantity')::int, 1))::bigint AS units
  FROM analytics_mirror.user_events
 WHERE event_type = 'purchase'
   AND product_uuid IS NOT NULL
   AND occurred_at > now() - make_interval(days => 1)
 GROUP BY product_uuid
 ORDER BY units DESC;
