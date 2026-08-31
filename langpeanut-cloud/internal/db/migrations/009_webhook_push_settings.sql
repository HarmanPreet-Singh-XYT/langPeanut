-- 009_webhook_push_settings.sql
-- Add webhook autopilot push and PR bot customization settings to repo_settings

ALTER TABLE repo_settings ADD COLUMN webhook_push_enabled INTEGER NOT NULL DEFAULT 1;
ALTER TABLE repo_settings ADD COLUMN webhook_branch_filter TEXT NOT NULL DEFAULT 'default_branch';
ALTER TABLE repo_settings ADD COLUMN webhook_custom_branches TEXT NOT NULL DEFAULT '';
ALTER TABLE repo_settings ADD COLUMN webhook_action TEXT NOT NULL DEFAULT 'auto_pr';
ALTER TABLE repo_settings ADD COLUMN webhook_pr_comments_enabled INTEGER NOT NULL DEFAULT 1;
ALTER TABLE repo_settings ADD COLUMN webhook_custom_branch_prefix TEXT NOT NULL DEFAULT 'langpeanut/i18n-';
ALTER TABLE repo_settings ADD COLUMN webhook_path_filter TEXT NOT NULL DEFAULT '';
