DROP TRIGGER IF EXISTS set_ua_updated_at ON user_addresses;
DROP FUNCTION IF EXISTS set_ua_updated_at_fn();
DROP INDEX IF EXISTS idx_user_addresses_one_default;
DROP INDEX IF EXISTS idx_user_addresses_user_id;
DROP TABLE IF EXISTS user_addresses;
