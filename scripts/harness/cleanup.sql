-- Removes everything the harness wrote. Safe to run repeatedly.
--
-- Every harness anon id shares the prefix 0000-4ddd-8ddd-, so this cannot reach
-- a real visitor's trail.
\set ON_ERROR_STOP on
\connect shopping_analytics

SELECT count(*) AS rows_to_remove
  FROM user_events
 WHERE anon_id::text LIKE 'dddd000%';

DELETE FROM user_events
 WHERE anon_id::text LIKE 'dddd000%';
