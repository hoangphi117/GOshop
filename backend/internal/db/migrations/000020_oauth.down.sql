DROP INDEX IF EXISTS idx_users_oauth;
ALTER TABLE users DROP COLUMN IF EXISTS user_oauth_provider;
ALTER TABLE users DROP COLUMN IF EXISTS user_oauth_id;
ALTER TABLE users DROP COLUMN IF EXISTS user_avatar_url;
ALTER TABLE users ALTER COLUMN user_password SET NOT NULL;
