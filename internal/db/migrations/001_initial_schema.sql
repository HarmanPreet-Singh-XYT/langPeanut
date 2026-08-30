-- 001_initial_schema.sql
-- langPeanut Cloud — initial SQLite schema (WAL mode is enabled at runtime, not here)
-- Run at startup via internal/db.Migrate() in version order.

CREATE TABLE IF NOT EXISTS teams (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT    NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- One row per GitHub App installation (an installation can be an org or a personal account).
CREATE TABLE IF NOT EXISTS github_installations (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    team_id         INTEGER NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    installation_id INTEGER NOT NULL UNIQUE,  -- GitHub's installation ID
    account_login   TEXT    NOT NULL,         -- org or user login
    created_at      DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- One row per repo the team has enabled localization for.
CREATE TABLE IF NOT EXISTS repos (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    installation_id INTEGER NOT NULL REFERENCES github_installations(id) ON DELETE CASCADE,
    owner           TEXT    NOT NULL,  -- GitHub org or user login
    name            TEXT    NOT NULL,  -- repo name (without owner/)
    default_branch  TEXT    NOT NULL DEFAULT 'main',
    created_at      DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    UNIQUE(owner, name)
);

-- Per-repo localization settings (mirrors the CLI wizard's 4 steps + Session 37/38 tunables).
-- A repo must have a settings row before a job can be created for it.
CREATE TABLE IF NOT EXISTS repo_settings (
    repo_id            INTEGER PRIMARY KEY REFERENCES repos(id) ON DELETE CASCADE,
    locales_json       TEXT    NOT NULL DEFAULT '[]',  -- JSON array of locale codes e.g. ["fr","es","de"]
    tone_preset        TEXT    NOT NULL DEFAULT 'neutral',
    provider           TEXT    NOT NULL DEFAULT 'gemini',
    model              TEXT    NOT NULL DEFAULT 'gemini-3.5-flash',
    safety_mode        INTEGER NOT NULL DEFAULT 1,      -- 1=on, 0=off (maps to CLI --safety-mode)
    chunk_word_budget  INTEGER NOT NULL DEFAULT 10000,
    chunk_key_ceiling  INTEGER NOT NULL DEFAULT 300,
    updated_at         DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- BYO API keys, AES-GCM encrypted at rest.  The master key lives in an env var, never here.
-- provider is one of: openai, claude, gemini, deepl, custom
CREATE TABLE IF NOT EXISTS api_credentials (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    team_id      INTEGER NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    provider     TEXT    NOT NULL,
    encrypted_key BLOB   NOT NULL,  -- AES-256-GCM ciphertext (nonce prepended)
    created_at   DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    UNIQUE(team_id, provider)
);

-- Jobs table doubles as the job queue.  Worker polls for status='pending' rows.
-- trigger_type: manual | webhook
-- status:       pending | running | succeeded | needs_review | failed | skipped_no_changes
CREATE TABLE IF NOT EXISTS jobs (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_id            INTEGER NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    trigger_type       TEXT    NOT NULL DEFAULT 'manual',
    status             TEXT    NOT NULL DEFAULT 'pending',
    branch             TEXT    NOT NULL DEFAULT '',          -- target branch name (langpeanut/i18n-*)
    head_commit_sha    TEXT    NOT NULL DEFAULT '',          -- HEAD SHA at job-claim time (dedupe key)
    repo_settings_hash TEXT    NOT NULL DEFAULT '',          -- SHA-256 of repo_settings JSON (dedupe key)
    pr_url             TEXT    NOT NULL DEFAULT '',          -- set after PR is opened
    error_message      TEXT    NOT NULL DEFAULT '',          -- set on status='failed'
    created_at         DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    started_at         DATETIME,
    finished_at        DATETIME
);

CREATE INDEX IF NOT EXISTS idx_jobs_status     ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_repo_id    ON jobs(repo_id);
CREATE INDEX IF NOT EXISTS idx_jobs_dedupe     ON jobs(repo_id, head_commit_sha, repo_settings_hash, status);

-- Per-job token usage — same shape as pkg/llm/tracker.go's ModelUsage, just per-job.
CREATE TABLE IF NOT EXISTS job_token_usage (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id        INTEGER NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    provider      TEXT    NOT NULL,
    model         TEXT    NOT NULL,
    input_tokens  INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    cost_usd      REAL    NOT NULL DEFAULT 0.0
);
