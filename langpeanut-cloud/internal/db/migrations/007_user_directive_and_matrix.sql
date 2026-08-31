-- 007_user_directive_and_matrix.sql
-- 1. Add user_directive (UI Integration Switcher Directive) to repo_settings
ALTER TABLE repo_settings ADD COLUMN user_directive TEXT NOT NULL DEFAULT '';

-- 2. Add execution_logs_json and translations_matrix_json to jobs
ALTER TABLE jobs ADD COLUMN execution_logs_json TEXT NOT NULL DEFAULT '[]';
ALTER TABLE jobs ADD COLUMN translations_matrix_json TEXT NOT NULL DEFAULT '{}';

-- 3. Dedicated table for persistent real translation key-value matrix per repo
CREATE TABLE IF NOT EXISTS repo_translation_matrix (
    repo_id           INTEGER NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    locale            TEXT    NOT NULL,
    translation_key   TEXT    NOT NULL,
    translation_value TEXT    NOT NULL,
    updated_at        DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    PRIMARY KEY(repo_id, locale, translation_key)
);
