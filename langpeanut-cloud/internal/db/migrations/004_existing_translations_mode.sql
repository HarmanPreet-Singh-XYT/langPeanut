-- 004_existing_translations_mode.sql
-- Add existing_translations_mode to repo_settings (skip | replace | prompt)
ALTER TABLE repo_settings ADD COLUMN existing_translations_mode TEXT NOT NULL DEFAULT 'skip';
