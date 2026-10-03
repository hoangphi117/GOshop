-- Allow NULL password for OAuth users
ALTER TABLE users ALTER COLUMN user_password DROP NOT NULL;
ALTER TABLE users ALTER COLUMN user_password SET DEFAULT NULL;

-- Add OAuth fields 
ALTER TABLE users ADD COLUMN IF NOT EXISTS user_oauth_provider VARCHAR(30) DEFAULT NULL;
ALTER TABLE users ADD COLUMN IF NOT EXISTS user_oauth_id       VARCHAR(200) DEFAULT NULL;
ALTER TABLE users ADD COLUMN IF NOT EXISTS user_avatar_url     TEXT DEFAULT NULL;

-- Index for lookup by OAuth provider and ID
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_oauth
  ON users(user_oauth_provider, user_oauth_id)
  WHERE user_oauth_provider IS NOT NULL;
