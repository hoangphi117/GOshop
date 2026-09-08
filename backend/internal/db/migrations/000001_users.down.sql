-- Drop trigger if exists

DROP TRIGGER IF EXISTS set_user_updated_at on users;

-- Drop trigger if exists
DROP FUNCTION IF EXISTS update_user_updated_at;

-- Drop index
DROP INDEX IF EXISTS idx_users_level;
DROP INDEX IF EXISTS idx_users_status;
DROP INDEX IF EXISTS idx_users_created_at;
DROP INDEX IF EXISTS idx_users_deleted_at;
DROP INDEX IF EXISTS idx_users_email_status;

-- Drop table
DROP TABLE IF EXISTS users;