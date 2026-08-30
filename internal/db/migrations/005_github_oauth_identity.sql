-- 005_github_oauth_identity.sql
-- Real GitHub OAuth login: key users by GitHub's stable numeric account ID
-- instead of email, since a logged-in user's email may be empty/private and
-- their login can be renamed. github_id=0 means "not yet linked via OAuth"
-- (e.g. rows created before this migration).

ALTER TABLE users ADD COLUMN github_id INTEGER NOT NULL DEFAULT 0;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_github_id
    ON users(github_id)
    WHERE github_id != 0;
