-- 006_repo_api_key_override.sql
-- Add optional per-repo encrypted API key override to repo_settings.
-- If set, the job worker uses this repo-specific key; otherwise it falls back to the team's global API credentials.
ALTER TABLE repo_settings ADD COLUMN encrypted_api_key_override BLOB DEFAULT NULL;
