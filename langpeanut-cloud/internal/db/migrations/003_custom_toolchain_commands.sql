-- 003_custom_toolchain_commands.sql
-- Add custom_install_cmd, custom_build_cmd, and root_dir to repo_settings
ALTER TABLE repo_settings ADD COLUMN custom_install_cmd TEXT NOT NULL DEFAULT '';
ALTER TABLE repo_settings ADD COLUMN custom_build_cmd TEXT NOT NULL DEFAULT '';
ALTER TABLE repo_settings ADD COLUMN root_dir TEXT NOT NULL DEFAULT '';
