// Package db — query helpers for every table in the schema.
// All functions take a *sql.Tx or *sql.DB (via the Querier interface) so they
// work inside and outside transactions without duplication.
package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// ─── Models ──────────────────────────────────────────────────────────────────

type Team struct {
	ID        int64
	Name      string
	CreatedAt time.Time
}

type GithubInstallation struct {
	ID             int64
	TeamID         int64
	InstallationID int64
	AccountLogin   string
	CreatedAt      time.Time
}

type Repo struct {
	ID             int64
	InstallationID int64
	Owner          string
	Name           string
	DefaultBranch  string
	CreatedAt      time.Time
}

type RepoSettings struct {
	RepoID                   int64
	Locales                  []string // stored as JSON in DB
	TonePreset               string
	Provider                 string
	Model                    string
	SafetyMode               bool
	ChunkWordBudget          int
	ChunkKeyCeiling          int
	CustomInstallCmd         string
	CustomBuildCmd           string
	RootDir                  string // Relative subdirectory inside repository (e.g. "apps/web", "frontend", "mobile")
	ExistingTranslationsMode string // "skip" (default), "replace" (regenerate all), "prompt"
	EncryptedAPIKeyOverride  []byte // Optional per-repo API key override (AES-256-GCM encrypted); falls back to global team credential if empty
	UserDirective            string // UI Integration Directive / Custom instruction
	WebhookPushEnabled       bool   // Autopilot trigger on git push webhook
	WebhookBranchFilter      string // "default_branch" (default), "all", "custom"
	WebhookCustomBranches    string // Comma-separated branch patterns e.g. "main, dev, release/*"
	WebhookAction            string // "auto_pr" (default), "direct_commit", "draft_pr"
	WebhookPRCommentsEnabled bool   // Whether @langpeanut PR comment commands are active
	WebhookCustomBranchPrefix string // Custom PR branch prefix e.g. "langpeanut/i18n-"
	WebhookPathFilter        string // Optional file path filter/glob e.g. "src/**, app/**"
	UpdatedAt                time.Time
}

type Job struct {
	ID                     int64
	RepoID                 int64
	TriggerType            string
	Status                 string
	Branch                 string
	HeadCommitSHA          string
	RepoSettingsHash       string
	PRURL                  string
	ErrorMessage           string
	ExecutionLogsJSON      string
	TranslationsMatrixJSON string
	CreatedAt              time.Time
	StartedAt              *time.Time
	FinishedAt             *time.Time
}

type APICredential struct {
	ID           int64
	TeamID       int64
	Provider     string
	EncryptedKey []byte
	CreatedAt    time.Time
}

type User struct {
	ID          int64     `json:"id"`
	TeamID      int64     `json:"team_id"`
	Email       string    `json:"email"`
	Name        string    `json:"name"`
	GithubLogin string    `json:"github_login"`
	GithubID    int64     `json:"github_id"`
	AvatarURL   string    `json:"avatar_url"`
	CreatedAt   time.Time `json:"created_at"`
}

type JobTokenUsage struct {
	ID           int64
	JobID        int64
	Provider     string
	Model        string
	InputTokens  int64
	OutputTokens int64
	CostUSD      float64
}

// ─── Teams ───────────────────────────────────────────────────────────────────

func (db *DB) CreateTeam(name string) (*Team, error) {
	res, err := db.Exec("INSERT INTO teams(name) VALUES(?)", name)
	if err != nil {
		return nil, fmt.Errorf("create team: %w", err)
	}
	id, _ := res.LastInsertId()
	return &Team{ID: id, Name: name, CreatedAt: time.Now().UTC()}, nil
}

func (db *DB) GetTeamByID(id int64) (*Team, error) {
	t := &Team{}
	err := db.QueryRow("SELECT id, name, created_at FROM teams WHERE id=?", id).
		Scan(&t.ID, &t.Name, &t.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return t, err
}

// ─── Users ───────────────────────────────────────────────────────────────────

// UpsertUserByGithubID is the identity path for real OAuth logins: GitHub's
// numeric account ID is the stable key (logins can be renamed, emails can be
// private/empty). Creates the user's row on first login, refreshes profile
// fields on every subsequent one.
func (db *DB) UpsertUserByGithubID(teamID, githubID int64, email, name, githubLogin, avatarURL string) (*User, error) {
	_, err := db.Exec(`
		INSERT INTO users(team_id, email, name, github_login, github_id, avatar_url)
		VALUES(?,?,?,?,?,?)
		ON CONFLICT(github_id) WHERE github_id != 0 DO UPDATE SET
			email=excluded.email,
			name=excluded.name,
			github_login=excluded.github_login,
			avatar_url=excluded.avatar_url`,
		teamID, email, name, githubLogin, githubID, avatarURL)
	if err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
	}
	return db.GetUserByGithubID(githubID)
}

func (db *DB) GetUserByGithubID(githubID int64) (*User, error) {
	u := &User{}
	err := db.QueryRow(`SELECT id, team_id, email, name, github_login, github_id, avatar_url, created_at
		FROM users WHERE github_id=?`, githubID).
		Scan(&u.ID, &u.TeamID, &u.Email, &u.Name, &u.GithubLogin, &u.GithubID, &u.AvatarURL, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func (db *DB) GetUserByEmail(email string) (*User, error) {
	u := &User{}
	err := db.QueryRow(`SELECT id, team_id, email, name, github_login, github_id, avatar_url, created_at
		FROM users WHERE email=?`, email).
		Scan(&u.ID, &u.TeamID, &u.Email, &u.Name, &u.GithubLogin, &u.GithubID, &u.AvatarURL, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func (db *DB) GetUserByID(id int64) (*User, error) {
	u := &User{}
	err := db.QueryRow(`SELECT id, team_id, email, name, github_login, github_id, avatar_url, created_at
		FROM users WHERE id=?`, id).
		Scan(&u.ID, &u.TeamID, &u.Email, &u.Name, &u.GithubLogin, &u.GithubID, &u.AvatarURL, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

// ─── GitHub Installations ────────────────────────────────────────────────────

func (db *DB) UpsertInstallation(teamID, installationID int64, accountLogin string) (*GithubInstallation, error) {
	_, err := db.Exec(`
		INSERT INTO github_installations(team_id, installation_id, account_login)
		VALUES(?,?,?)
		ON CONFLICT(installation_id) DO UPDATE SET account_login=excluded.account_login`,
		teamID, installationID, accountLogin)
	if err != nil {
		return nil, fmt.Errorf("upsert installation: %w", err)
	}
	gi := &GithubInstallation{}
	err = db.QueryRow(`SELECT id, team_id, installation_id, account_login, created_at
		FROM github_installations WHERE installation_id=?`, installationID).
		Scan(&gi.ID, &gi.TeamID, &gi.InstallationID, &gi.AccountLogin, &gi.CreatedAt)
	return gi, err
}

func (db *DB) ListInstallationsByTeam(teamID int64) ([]*GithubInstallation, error) {
	rows, err := db.Query(`SELECT id, team_id, installation_id, account_login, created_at
		FROM github_installations WHERE team_id=? ORDER BY created_at`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*GithubInstallation
	for rows.Next() {
		gi := &GithubInstallation{}
		if err := rows.Scan(&gi.ID, &gi.TeamID, &gi.InstallationID, &gi.AccountLogin, &gi.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, gi)
	}
	return out, rows.Err()
}

func (db *DB) GetInstallationByID(id int64) (*GithubInstallation, error) {
	gi := &GithubInstallation{}
	err := db.QueryRow(`SELECT id, team_id, installation_id, account_login, created_at
		FROM github_installations WHERE id=?`, id).
		Scan(&gi.ID, &gi.TeamID, &gi.InstallationID, &gi.AccountLogin, &gi.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return gi, err
}

// ─── Repos ───────────────────────────────────────────────────────────────────

func (db *DB) UpsertRepo(installationID int64, owner, name, defaultBranch string) (*Repo, error) {
	_, err := db.Exec(`
		INSERT INTO repos(installation_id, owner, name, default_branch)
		VALUES(?,?,?,?)
		ON CONFLICT(owner, name) DO UPDATE SET
			installation_id=excluded.installation_id,
			default_branch=excluded.default_branch`,
		installationID, owner, name, defaultBranch)
	if err != nil {
		return nil, fmt.Errorf("upsert repo: %w", err)
	}
	r := &Repo{}
	err = db.QueryRow(`SELECT id, installation_id, owner, name, default_branch, created_at
		FROM repos WHERE owner=? AND name=?`, owner, name).
		Scan(&r.ID, &r.InstallationID, &r.Owner, &r.Name, &r.DefaultBranch, &r.CreatedAt)
	return r, err
}

func (db *DB) GetRepoByID(id int64) (*Repo, error) {
	r := &Repo{}
	err := db.QueryRow(`SELECT id, installation_id, owner, name, default_branch, created_at
		FROM repos WHERE id=?`, id).
		Scan(&r.ID, &r.InstallationID, &r.Owner, &r.Name, &r.DefaultBranch, &r.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return r, err
}

func (db *DB) GetRepoByOwnerAndName(owner, name string) (*Repo, error) {
	r := &Repo{}
	err := db.QueryRow(`SELECT id, installation_id, owner, name, default_branch, created_at
		FROM repos WHERE owner=? AND name=?`, owner, name).
		Scan(&r.ID, &r.InstallationID, &r.Owner, &r.Name, &r.DefaultBranch, &r.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return r, err
}

func (db *DB) ListReposByInstallation(installationID int64) ([]*Repo, error) {
	rows, err := db.Query(`SELECT id, installation_id, owner, name, default_branch, created_at
		FROM repos WHERE installation_id=? ORDER BY owner, name`, installationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Repo
	for rows.Next() {
		r := &Repo{}
		if err := rows.Scan(&r.ID, &r.InstallationID, &r.Owner, &r.Name, &r.DefaultBranch, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ─── Repo Settings ────────────────────────────────────────────────────────────

func (db *DB) UpsertRepoSettings(s *RepoSettings) error {
	localesJSON, err := json.Marshal(s.Locales)
	if err != nil {
		return err
	}
	safetyInt := 0
	if s.SafetyMode {
		safetyInt = 1
	}
	pushEnabledInt := 0
	if s.WebhookPushEnabled {
		pushEnabledInt = 1
	}
	prCommentsInt := 0
	if s.WebhookPRCommentsEnabled {
		prCommentsInt = 1
	}
	branchFilter := s.WebhookBranchFilter
	if branchFilter == "" {
		branchFilter = "default_branch"
	}
	action := s.WebhookAction
	if action == "" {
		action = "auto_pr"
	}
	prefix := s.WebhookCustomBranchPrefix
	if prefix == "" {
		prefix = "langpeanut/i18n-"
	}
	existingMode := s.ExistingTranslationsMode
	if existingMode == "" {
		existingMode = "skip"
	}
	_, err = db.Exec(`
		INSERT INTO repo_settings(
			repo_id, locales_json, tone_preset, provider, model, safety_mode,
			chunk_word_budget, chunk_key_ceiling, custom_install_cmd, custom_build_cmd,
			root_dir, existing_translations_mode, encrypted_api_key_override, user_directive,
			webhook_push_enabled, webhook_branch_filter, webhook_custom_branches, webhook_action,
			webhook_pr_comments_enabled, webhook_custom_branch_prefix, webhook_path_filter,
			updated_at
		)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?, strftime('%Y-%m-%dT%H:%M:%SZ','now'))
		ON CONFLICT(repo_id) DO UPDATE SET
			locales_json=excluded.locales_json,
			tone_preset=excluded.tone_preset,
			provider=excluded.provider,
			model=excluded.model,
			safety_mode=excluded.safety_mode,
			chunk_word_budget=excluded.chunk_word_budget,
			chunk_key_ceiling=excluded.chunk_key_ceiling,
			custom_install_cmd=excluded.custom_install_cmd,
			custom_build_cmd=excluded.custom_build_cmd,
			root_dir=excluded.root_dir,
			existing_translations_mode=excluded.existing_translations_mode,
			encrypted_api_key_override=excluded.encrypted_api_key_override,
			user_directive=excluded.user_directive,
			webhook_push_enabled=excluded.webhook_push_enabled,
			webhook_branch_filter=excluded.webhook_branch_filter,
			webhook_custom_branches=excluded.webhook_custom_branches,
			webhook_action=excluded.webhook_action,
			webhook_pr_comments_enabled=excluded.webhook_pr_comments_enabled,
			webhook_custom_branch_prefix=excluded.webhook_custom_branch_prefix,
			webhook_path_filter=excluded.webhook_path_filter,
			updated_at=excluded.updated_at`,
		s.RepoID, string(localesJSON), s.TonePreset, s.Provider, s.Model,
		safetyInt, s.ChunkWordBudget, s.ChunkKeyCeiling, s.CustomInstallCmd, s.CustomBuildCmd,
		s.RootDir, existingMode, s.EncryptedAPIKeyOverride, s.UserDirective,
		pushEnabledInt, branchFilter, s.WebhookCustomBranches, action,
		prCommentsInt, prefix, s.WebhookPathFilter)
	return err
}

func (db *DB) GetRepoSettings(repoID int64) (*RepoSettings, error) {
	var localesJSON string
	var safetyInt, pushEnabledInt, prCommentsInt int
	var overrideBlob []byte
	s := &RepoSettings{RepoID: repoID}
	err := db.QueryRow(`SELECT locales_json, tone_preset, provider, model, safety_mode,
		chunk_word_budget, chunk_key_ceiling, COALESCE(custom_install_cmd, ''),
		COALESCE(custom_build_cmd, ''), COALESCE(root_dir, ''), COALESCE(existing_translations_mode, 'skip'),
		COALESCE(encrypted_api_key_override, X''), COALESCE(user_directive, ''),
		COALESCE(webhook_push_enabled, 1), COALESCE(webhook_branch_filter, 'default_branch'),
		COALESCE(webhook_custom_branches, ''), COALESCE(webhook_action, 'auto_pr'),
		COALESCE(webhook_pr_comments_enabled, 1), COALESCE(webhook_custom_branch_prefix, 'langpeanut/i18n-'),
		COALESCE(webhook_path_filter, ''), updated_at
		FROM repo_settings WHERE repo_id=?`, repoID).
		Scan(
			&localesJSON, &s.TonePreset, &s.Provider, &s.Model, &safetyInt,
			&s.ChunkWordBudget, &s.ChunkKeyCeiling, &s.CustomInstallCmd,
			&s.CustomBuildCmd, &s.RootDir, &s.ExistingTranslationsMode,
			&overrideBlob, &s.UserDirective,
			&pushEnabledInt, &s.WebhookBranchFilter,
			&s.WebhookCustomBranches, &s.WebhookAction,
			&prCommentsInt, &s.WebhookCustomBranchPrefix,
			&s.WebhookPathFilter, &s.UpdatedAt,
		)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s.SafetyMode = safetyInt == 1
	s.WebhookPushEnabled = pushEnabledInt == 1
	s.WebhookPRCommentsEnabled = prCommentsInt == 1
	if len(overrideBlob) > 0 {
		s.EncryptedAPIKeyOverride = overrideBlob
	}
	if err := json.Unmarshal([]byte(localesJSON), &s.Locales); err != nil {
		return nil, fmt.Errorf("parse locales_json: %w", err)
	}
	return s, nil
}

// ─── API Credentials ──────────────────────────────────────────────────────────

func (db *DB) UpsertAPICredential(teamID int64, provider string, encryptedKey []byte) error {
	_, err := db.Exec(`
		INSERT INTO api_credentials(team_id, provider, encrypted_key)
		VALUES(?,?,?)
		ON CONFLICT(team_id, provider) DO UPDATE SET encrypted_key=excluded.encrypted_key`,
		teamID, provider, encryptedKey)
	return err
}

func (db *DB) GetAPICredential(teamID int64, provider string) (*APICredential, error) {
	c := &APICredential{}
	err := db.QueryRow(`SELECT id, team_id, provider, encrypted_key, created_at
		FROM api_credentials WHERE team_id=? AND provider=?`, teamID, provider).
		Scan(&c.ID, &c.TeamID, &c.Provider, &c.EncryptedKey, &c.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return c, err
}

// ─── Jobs ─────────────────────────────────────────────────────────────────────

// CreateJob inserts a new pending job and returns its ID.
func (db *DB) CreateJob(repoID int64, triggerType string) (*Job, error) {
	return db.CreateJobWithBranch(repoID, triggerType, "")
}

// CreateJobWithBranch inserts a new pending job with an explicit target branch.
func (db *DB) CreateJobWithBranch(repoID int64, triggerType, branch string) (*Job, error) {
	res, err := db.Exec(`INSERT INTO jobs(repo_id, trigger_type, branch, status) VALUES(?,?,?,'pending')`,
		repoID, triggerType, branch)
	if err != nil {
		return nil, fmt.Errorf("create job: %w", err)
	}
	id, _ := res.LastInsertId()
	return db.GetJobByID(id)
}

func (db *DB) GetJobByID(id int64) (*Job, error) {
	return scanJob(db.QueryRow(`
		SELECT id, repo_id, trigger_type, status, branch, head_commit_sha, repo_settings_hash,
		       pr_url, error_message, created_at, started_at, finished_at
		FROM jobs WHERE id=?`, id))
}

// ClaimNextPendingJob atomically claims the oldest pending job for the worker.
// Returns (nil, nil) if no pending job exists.
func (db *DB) ClaimNextPendingJob() (*Job, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck

	var id int64
	err = tx.QueryRow(`SELECT id FROM jobs WHERE status='pending' ORDER BY created_at LIMIT 1`).Scan(&id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(`UPDATE jobs SET status='running', started_at=strftime('%Y-%m-%dT%H:%M:%SZ','now') WHERE id=? AND status='pending'`, id)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return db.GetJobByID(id)
}

// HasDuplicateSuccessfulJob checks if a job for the same repo+commit+settings already succeeded.
// Used for the §6.2 dedupe check.
func (db *DB) HasDuplicateSuccessfulJob(repoID int64, headCommitSHA, repoSettingsHash string) (bool, error) {
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM jobs
		WHERE repo_id=? AND head_commit_sha=? AND repo_settings_hash=?
		  AND status IN ('succeeded','needs_review')`,
		repoID, headCommitSHA, repoSettingsHash).Scan(&count)
	return count > 0, err
}

// UpdateJobStatus updates mutable fields on a job after work is done.
func (db *DB) UpdateJobStatus(id int64, status, branch, headSHA, settingsHash, prURL, errMsg string) error {
	_, err := db.Exec(`
		UPDATE jobs SET
			status=?, branch=?, head_commit_sha=?, repo_settings_hash=?,
			pr_url=?, error_message=?,
			finished_at=strftime('%Y-%m-%dT%H:%M:%SZ','now')
		WHERE id=?`,
		status, branch, headSHA, settingsHash, prURL, errMsg, id)
	return err
}

func (db *DB) ListJobsByRepo(repoID int64, limit int) ([]*Job, error) {
	rows, err := db.Query(`
		SELECT id, repo_id, trigger_type, status, branch, head_commit_sha, repo_settings_hash,
		       pr_url, error_message, created_at, started_at, finished_at
		FROM jobs WHERE repo_id=? ORDER BY created_at DESC LIMIT ?`, repoID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// ─── Job Token Usage ─────────────────────────────────────────────────────────

func (db *DB) RecordJobTokenUsage(u *JobTokenUsage) error {
	_, err := db.Exec(`
		INSERT INTO job_token_usage(job_id, provider, model, input_tokens, output_tokens, cost_usd)
		VALUES(?,?,?,?,?,?)`,
		u.JobID, u.Provider, u.Model, u.InputTokens, u.OutputTokens, u.CostUSD)
	return err
}

func (db *DB) ListTokenUsageByJob(jobID int64) ([]*JobTokenUsage, error) {
	rows, err := db.Query(`SELECT id, job_id, provider, model, input_tokens, output_tokens, cost_usd
		FROM job_token_usage WHERE job_id=?`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*JobTokenUsage
	for rows.Next() {
		u := &JobTokenUsage{}
		if err := rows.Scan(&u.ID, &u.JobID, &u.Provider, &u.Model, &u.InputTokens, &u.OutputTokens, &u.CostUSD); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// UpdateJobStatusWithDetails updates mutable fields and attaches real execution logs and matrix JSON.
func (db *DB) UpdateJobStatusWithDetails(id int64, status, branch, headSHA, settingsHash, prURL, errMsg, execLogsJSON, matrixJSON string) error {
	_, err := db.Exec(`
		UPDATE jobs SET
			status=?, branch=?, head_commit_sha=?, repo_settings_hash=?,
			pr_url=?, error_message=?,
			execution_logs_json=?, translations_matrix_json=?,
			finished_at=strftime('%Y-%m-%dT%H:%M:%SZ','now')
		WHERE id=?`,
		status, branch, headSHA, settingsHash, prURL, errMsg, execLogsJSON, matrixJSON, id)
	return err
}

func (db *DB) GetJobLogs(jobID int64) (string, error) {
	var logsJSON string
	err := db.QueryRow(`SELECT COALESCE(execution_logs_json, '[]') FROM jobs WHERE id=?`, jobID).Scan(&logsJSON)
	if err == sql.ErrNoRows {
		return "[]", nil
	}
	return logsJSON, err
}

// ─── Translation Matrix ──────────────────────────────────────────────────────

func (db *DB) UpsertTranslationMatrix(repoID int64, translations map[string]map[string]string) error {
	if len(translations) == 0 {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	stmt, err := tx.Prepare(`
		INSERT INTO repo_translation_matrix(repo_id, locale, translation_key, translation_value, updated_at)
		VALUES(?,?,?,?, strftime('%Y-%m-%dT%H:%M:%SZ','now'))
		ON CONFLICT(repo_id, locale, translation_key) DO UPDATE SET
			translation_value=excluded.translation_value,
			updated_at=excluded.updated_at`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for loc, keys := range translations {
		for k, v := range keys {
			if _, err := stmt.Exec(repoID, loc, k, v); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func (db *DB) UpdateTranslationCell(repoID int64, locale, key, value string) error {
	_, err := db.Exec(`
		INSERT INTO repo_translation_matrix(repo_id, locale, translation_key, translation_value, updated_at)
		VALUES(?,?,?,?, strftime('%Y-%m-%dT%H:%M:%SZ','now'))
		ON CONFLICT(repo_id, locale, translation_key) DO UPDATE SET
			translation_value=excluded.translation_value,
			updated_at=excluded.updated_at`,
		repoID, locale, key, value)
	return err
}

func (db *DB) GetTranslationMatrix(repoID int64) (map[string]map[string]string, error) {
	rows, err := db.Query(`SELECT locale, translation_key, translation_value FROM repo_translation_matrix WHERE repo_id=? ORDER BY translation_key ASC`, repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	matrix := make(map[string]map[string]string)
	for rows.Next() {
		var loc, key, val string
		if err := rows.Scan(&loc, &key, &val); err != nil {
			return nil, err
		}
		if matrix[loc] == nil {
			matrix[loc] = make(map[string]string)
		}
		matrix[loc][key] = val
	}
	return matrix, rows.Err()
}

// ─── SEO & Market Growth Studio ──────────────────────────────────────────────

type RepoSEOStrategy struct {
	RepoID             int64     `json:"repo_id"`
	ProjectName        string    `json:"project_name"`
	Category           string    `json:"category"`
	ProductDescription string    `json:"product_description"`
	TargetLocales      []string  `json:"target_locales"`
	Goal               string    `json:"goal"`
	ScopeTier          string    `json:"scope_tier"`
	CompetitorURLs     []string  `json:"competitor_urls"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type RepoSEOCompetitor struct {
	ID              int64     `json:"id"`
	RepoID          int64     `json:"repo_id"`
	Locale          string    `json:"locale"`
	Domain          string    `json:"domain"`
	Rank            int       `json:"rank"`
	URL             string    `json:"url"`
	Title           string    `json:"title"`
	MetaDescription string    `json:"meta_description"`
	H1s             []string  `json:"h1s"`
	H2s             []string  `json:"h2s"`
	Keywords        []string  `json:"keywords"`
	ValueProps      []string  `json:"value_props"`
	IsDiscovered    bool      `json:"is_discovered"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type RepoSEOKeyword struct {
	ID               int64     `json:"id"`
	RepoID           int64     `json:"repo_id"`
	Locale           string    `json:"locale"`
	Keyword          string    `json:"keyword"`
	Intent           string    `json:"intent"`
	VolumeTier       string    `json:"volume_tier"`
	EstMonthlyVolume int       `json:"est_monthly_volume"`
	Difficulty       int       `json:"difficulty"`
	Relevance        int       `json:"relevance"`
	IsPrimary        bool      `json:"is_primary"`
	IsLocked         bool      `json:"is_locked"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type RepoSEOOptimization struct {
	RepoID               int64     `json:"repo_id"`
	Locale               string    `json:"locale"`
	TranslationKey       string    `json:"translation_key"`
	SourceEn             string    `json:"source_en"`
	BaselineTranslation  string    `json:"baseline_translation"`
	OptimizedTranslation string    `json:"optimized_translation"`
	InjectedKeywords     []string  `json:"injected_keywords"`
	Rationale            string    `json:"rationale"`
	ImpactTier           string    `json:"impact_tier"`
	CharacterLength      int       `json:"character_length"`
	PixelWidthDesktop    int       `json:"pixel_width_desktop"`
	IsTitleTruncated     bool      `json:"is_title_truncated"`
	ICUVariablesMatched  bool      `json:"icu_variables_matched"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type RepoSEOMetrics struct {
	RepoID                int64     `json:"repo_id"`
	Locale                string    `json:"locale"`
	SearchVolumeBaseline  int       `json:"search_volume_baseline"`
	SearchVolumeOptimized int       `json:"search_volume_optimized"`
	SearchVolumeUpliftPct float64   `json:"search_volume_uplift_pct"`
	ProjectedCTRBaseline  float64   `json:"projected_ctr_baseline"`
	ProjectedCTROptimized float64   `json:"projected_ctr_optimized"`
	ProjectedCTRUpliftPct float64   `json:"projected_ctr_uplift_pct"`
	AvgKeywordDifficulty  int       `json:"avg_keyword_difficulty"`
	ReadabilityScore      int       `json:"readability_score"`
	LocalTrustScore       int       `json:"local_trust_score"`
	KeywordDensityPct     float64   `json:"keyword_density_pct"`
	DensitySafe           bool      `json:"density_safe"`
	EstimatedRankingDays  int       `json:"estimated_ranking_days"`
	UpdatedAt             time.Time `json:"updated_at"`
}

func (db *DB) GetSEOStrategy(repoID int64) (*RepoSEOStrategy, error) {
	row := db.QueryRow(`
		SELECT repo_id, project_name, category, product_description, target_locales_json, goal, scope_tier, competitor_urls_json, updated_at
		FROM repo_seo_strategies WHERE repo_id=?`, repoID)

	s := &RepoSEOStrategy{}
	var locsJSON, compsJSON string
	err := row.Scan(&s.RepoID, &s.ProjectName, &s.Category, &s.ProductDescription, &locsJSON, &s.Goal, &s.ScopeTier, &compsJSON, &s.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(locsJSON), &s.TargetLocales)
	_ = json.Unmarshal([]byte(compsJSON), &s.CompetitorURLs)
	return s, nil
}

func (db *DB) UpsertSEOStrategy(s *RepoSEOStrategy) error {
	locsJSON, _ := json.Marshal(s.TargetLocales)
	compsJSON, _ := json.Marshal(s.CompetitorURLs)

	_, err := db.Exec(`
		INSERT INTO repo_seo_strategies(repo_id, project_name, category, product_description, target_locales_json, goal, scope_tier, competitor_urls_json, updated_at)
		VALUES(?,?,?,?,?,?,?,?, strftime('%Y-%m-%dT%H:%M:%SZ','now'))
		ON CONFLICT(repo_id) DO UPDATE SET
			project_name=excluded.project_name,
			category=excluded.category,
			product_description=excluded.product_description,
			target_locales_json=excluded.target_locales_json,
			goal=excluded.goal,
			scope_tier=excluded.scope_tier,
			competitor_urls_json=excluded.competitor_urls_json,
			updated_at=excluded.updated_at`,
		s.RepoID, s.ProjectName, s.Category, s.ProductDescription, string(locsJSON), s.Goal, s.ScopeTier, string(compsJSON))
	return err
}

func (db *DB) GetSEOCompetitors(repoID int64, locale string) ([]RepoSEOCompetitor, error) {
	query := `SELECT id, repo_id, locale, domain, rank, url, title, meta_description, h1s_json, h2s_json, keywords_json, value_props_json, is_discovered, updated_at
		FROM repo_seo_competitors WHERE repo_id=?`
	args := []any{repoID}
	if locale != "" {
		query += ` AND locale=?`
		args = append(args, locale)
	}
	query += ` ORDER BY rank ASC`

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []RepoSEOCompetitor
	for rows.Next() {
		var c RepoSEOCompetitor
		var h1s, h2s, kws, vps string
		var isDisc int
		if err := rows.Scan(&c.ID, &c.RepoID, &c.Locale, &c.Domain, &c.Rank, &c.URL, &c.Title, &c.MetaDescription, &h1s, &h2s, &kws, &vps, &isDisc, &c.UpdatedAt); err != nil {
			return nil, err
		}
		c.IsDiscovered = isDisc == 1
		_ = json.Unmarshal([]byte(h1s), &c.H1s)
		_ = json.Unmarshal([]byte(h2s), &c.H2s)
		_ = json.Unmarshal([]byte(kws), &c.Keywords)
		_ = json.Unmarshal([]byte(vps), &c.ValueProps)
		list = append(list, c)
	}
	return list, rows.Err()
}

func (db *DB) UpsertSEOCompetitors(repoID int64, locale string, comps []RepoSEOCompetitor) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	stmt, err := tx.Prepare(`
		INSERT INTO repo_seo_competitors(repo_id, locale, domain, rank, url, title, meta_description, h1s_json, h2s_json, keywords_json, value_props_json, is_discovered, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?, strftime('%Y-%m-%dT%H:%M:%SZ','now'))
		ON CONFLICT(repo_id, locale, domain) DO UPDATE SET
			rank=excluded.rank,
			url=excluded.url,
			title=excluded.title,
			meta_description=excluded.meta_description,
			h1s_json=excluded.h1s_json,
			h2s_json=excluded.h2s_json,
			keywords_json=excluded.keywords_json,
			value_props_json=excluded.value_props_json,
			is_discovered=excluded.is_discovered,
			updated_at=excluded.updated_at`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, c := range comps {
		h1s, _ := json.Marshal(c.H1s)
		h2s, _ := json.Marshal(c.H2s)
		kws, _ := json.Marshal(c.Keywords)
		vps, _ := json.Marshal(c.ValueProps)
		isDisc := 0
		if c.IsDiscovered {
			isDisc = 1
		}
		if _, err := stmt.Exec(repoID, locale, c.Domain, c.Rank, c.URL, c.Title, c.MetaDescription, string(h1s), string(h2s), string(kws), string(vps), isDisc); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (db *DB) GetSEOKeywords(repoID int64, locale string) ([]RepoSEOKeyword, error) {
	query := `SELECT id, repo_id, locale, keyword, intent, volume_tier, est_monthly_volume, difficulty, relevance, is_primary, is_locked, updated_at
		FROM repo_seo_keywords WHERE repo_id=?`
	args := []any{repoID}
	if locale != "" {
		query += ` AND locale=?`
		args = append(args, locale)
	}
	query += ` ORDER BY is_primary DESC, est_monthly_volume DESC`

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []RepoSEOKeyword
	for rows.Next() {
		var k RepoSEOKeyword
		var isPrim, isLock int
		if err := rows.Scan(&k.ID, &k.RepoID, &k.Locale, &k.Keyword, &k.Intent, &k.VolumeTier, &k.EstMonthlyVolume, &k.Difficulty, &k.Relevance, &isPrim, &isLock, &k.UpdatedAt); err != nil {
			return nil, err
		}
		k.IsPrimary = isPrim == 1
		k.IsLocked = isLock == 1
		list = append(list, k)
	}
	return list, rows.Err()
}

func (db *DB) UpsertSEOKeywords(repoID int64, locale string, kws []RepoSEOKeyword) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	stmt, err := tx.Prepare(`
		INSERT INTO repo_seo_keywords(repo_id, locale, keyword, intent, volume_tier, est_monthly_volume, difficulty, relevance, is_primary, is_locked, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?, strftime('%Y-%m-%dT%H:%M:%SZ','now'))
		ON CONFLICT(repo_id, locale, keyword) DO UPDATE SET
			intent=excluded.intent,
			volume_tier=excluded.volume_tier,
			est_monthly_volume=excluded.est_monthly_volume,
			difficulty=excluded.difficulty,
			relevance=excluded.relevance,
			is_primary=excluded.is_primary,
			is_locked=excluded.is_locked,
			updated_at=excluded.updated_at`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, k := range kws {
		isPrim := 0
		if k.IsPrimary {
			isPrim = 1
		}
		isLock := 0
		if k.IsLocked {
			isLock = 1
		}
		if _, err := stmt.Exec(repoID, locale, k.Keyword, k.Intent, k.VolumeTier, k.EstMonthlyVolume, k.Difficulty, k.Relevance, isPrim, isLock); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (db *DB) GetSEOOptimizations(repoID int64, locale string) ([]RepoSEOOptimization, error) {
	query := `SELECT repo_id, locale, translation_key, source_en, baseline_translation, optimized_translation, injected_keywords_json, rationale, impact_tier, character_length, pixel_width_desktop, is_title_truncated, icu_variables_matched, updated_at
		FROM repo_seo_optimizations WHERE repo_id=?`
	args := []any{repoID}
	if locale != "" {
		query += ` AND locale=?`
		args = append(args, locale)
	}
	query += ` ORDER BY impact_tier ASC, translation_key ASC`

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []RepoSEOOptimization
	for rows.Next() {
		var o RepoSEOOptimization
		var kwsJSON string
		var isTrunc, icuOK int
		if err := rows.Scan(&o.RepoID, &o.Locale, &o.TranslationKey, &o.SourceEn, &o.BaselineTranslation, &o.OptimizedTranslation, &kwsJSON, &o.Rationale, &o.ImpactTier, &o.CharacterLength, &o.PixelWidthDesktop, &isTrunc, &icuOK, &o.UpdatedAt); err != nil {
			return nil, err
		}
		o.IsTitleTruncated = isTrunc == 1
		o.ICUVariablesMatched = icuOK == 1
		_ = json.Unmarshal([]byte(kwsJSON), &o.InjectedKeywords)
		list = append(list, o)
	}
	return list, rows.Err()
}

func (db *DB) UpsertSEOOptimizations(repoID int64, locale string, opts []RepoSEOOptimization) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	stmt, err := tx.Prepare(`
		INSERT INTO repo_seo_optimizations(repo_id, locale, translation_key, source_en, baseline_translation, optimized_translation, injected_keywords_json, rationale, impact_tier, character_length, pixel_width_desktop, is_title_truncated, icu_variables_matched, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?, strftime('%Y-%m-%dT%H:%M:%SZ','now'))
		ON CONFLICT(repo_id, locale, translation_key) DO UPDATE SET
			source_en=excluded.source_en,
			baseline_translation=excluded.baseline_translation,
			optimized_translation=excluded.optimized_translation,
			injected_keywords_json=excluded.injected_keywords_json,
			rationale=excluded.rationale,
			impact_tier=excluded.impact_tier,
			character_length=excluded.character_length,
			pixel_width_desktop=excluded.pixel_width_desktop,
			is_title_truncated=excluded.is_title_truncated,
			icu_variables_matched=excluded.icu_variables_matched,
			updated_at=excluded.updated_at`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, o := range opts {
		kwsJSON, _ := json.Marshal(o.InjectedKeywords)
		isTrunc := 0
		if o.IsTitleTruncated {
			isTrunc = 1
		}
		icuOK := 0
		if o.ICUVariablesMatched {
			icuOK = 1
		}
		if _, err := stmt.Exec(repoID, locale, o.TranslationKey, o.SourceEn, o.BaselineTranslation, o.OptimizedTranslation, string(kwsJSON), o.Rationale, o.ImpactTier, o.CharacterLength, o.PixelWidthDesktop, isTrunc, icuOK); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (db *DB) GetSEOMetrics(repoID int64, locale string) (*RepoSEOMetrics, error) {
	row := db.QueryRow(`
		SELECT repo_id, locale, search_volume_baseline, search_volume_optimized, search_volume_uplift_pct, projected_ctr_baseline, projected_ctr_optimized, projected_ctr_uplift_pct, avg_keyword_difficulty, readability_score, local_trust_score, keyword_density_pct, density_safe, estimated_ranking_days, updated_at
		FROM repo_seo_metrics WHERE repo_id=? AND locale=?`, repoID, locale)

	m := &RepoSEOMetrics{}
	var isSafe int
	err := row.Scan(&m.RepoID, &m.Locale, &m.SearchVolumeBaseline, &m.SearchVolumeOptimized, &m.SearchVolumeUpliftPct, &m.ProjectedCTRBaseline, &m.ProjectedCTROptimized, &m.ProjectedCTRUpliftPct, &m.AvgKeywordDifficulty, &m.ReadabilityScore, &m.LocalTrustScore, &m.KeywordDensityPct, &isSafe, &m.EstimatedRankingDays, &m.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m.DensitySafe = isSafe == 1
	return m, nil
}

func (db *DB) UpsertSEOMetrics(m *RepoSEOMetrics) error {
	isSafe := 0
	if m.DensitySafe {
		isSafe = 1
	}
	_, err := db.Exec(`
		INSERT INTO repo_seo_metrics(repo_id, locale, search_volume_baseline, search_volume_optimized, search_volume_uplift_pct, projected_ctr_baseline, projected_ctr_optimized, projected_ctr_uplift_pct, avg_keyword_difficulty, readability_score, local_trust_score, keyword_density_pct, density_safe, estimated_ranking_days, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?, strftime('%Y-%m-%dT%H:%M:%SZ','now'))
		ON CONFLICT(repo_id, locale) DO UPDATE SET
			search_volume_baseline=excluded.search_volume_baseline,
			search_volume_optimized=excluded.search_volume_optimized,
			search_volume_uplift_pct=excluded.search_volume_uplift_pct,
			projected_ctr_baseline=excluded.projected_ctr_baseline,
			projected_ctr_optimized=excluded.projected_ctr_optimized,
			projected_ctr_uplift_pct=excluded.projected_ctr_uplift_pct,
			avg_keyword_difficulty=excluded.avg_keyword_difficulty,
			readability_score=excluded.readability_score,
			local_trust_score=excluded.local_trust_score,
			keyword_density_pct=excluded.keyword_density_pct,
			density_safe=excluded.density_safe,
			estimated_ranking_days=excluded.estimated_ranking_days,
			updated_at=excluded.updated_at`,
		m.RepoID, m.Locale, m.SearchVolumeBaseline, m.SearchVolumeOptimized, m.SearchVolumeUpliftPct, m.ProjectedCTRBaseline, m.ProjectedCTROptimized, m.ProjectedCTRUpliftPct, m.AvgKeywordDifficulty, m.ReadabilityScore, m.LocalTrustScore, m.KeywordDensityPct, isSafe, m.EstimatedRankingDays)
	return err
}

// ResetRepoData purges all localization output, job histories, matrix values, and SEO data for a repo
// allowing the user to start localization and analysis from a clean baseline.
func (db *DB) ResetRepoData(repoID int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Purge Translation Matrix
	if _, err := tx.Exec(`DELETE FROM repo_translation_matrix WHERE repo_id = ?`, repoID); err != nil {
		return fmt.Errorf("delete translation matrix: %w", err)
	}

	// 2. Purge SEO Studio data
	if _, err := tx.Exec(`DELETE FROM repo_seo_optimizations WHERE repo_id = ?`, repoID); err != nil {
		return fmt.Errorf("delete seo optimizations: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM repo_seo_metrics WHERE repo_id = ?`, repoID); err != nil {
		return fmt.Errorf("delete seo metrics: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM repo_seo_keywords WHERE repo_id = ?`, repoID); err != nil {
		return fmt.Errorf("delete seo keywords: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM repo_seo_competitors WHERE repo_id = ?`, repoID); err != nil {
		return fmt.Errorf("delete seo competitors: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM repo_seo_strategies WHERE repo_id = ?`, repoID); err != nil {
		return fmt.Errorf("delete seo strategies: %w", err)
	}

	// 3. Purge Jobs and Logs
	if _, err := tx.Exec(`DELETE FROM job_token_usage WHERE job_id IN (SELECT id FROM jobs WHERE repo_id = ?)`, repoID); err != nil {
		return fmt.Errorf("delete job token usage: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM jobs WHERE repo_id = ?`, repoID); err != nil {
		return fmt.Errorf("delete jobs: %w", err)
	}

	return tx.Commit()
}

// DeleteRepo completely purges the repository row and cascades to all child settings and records.
func (db *DB) DeleteRepo(repoID int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Explicit cleanup in case foreign keys cascade is not configured on older migrations
	_, _ = tx.Exec(`DELETE FROM repo_translation_matrix WHERE repo_id = ?`, repoID)
	_, _ = tx.Exec(`DELETE FROM repo_seo_optimizations WHERE repo_id = ?`, repoID)
	_, _ = tx.Exec(`DELETE FROM repo_seo_metrics WHERE repo_id = ?`, repoID)
	_, _ = tx.Exec(`DELETE FROM repo_seo_keywords WHERE repo_id = ?`, repoID)
	_, _ = tx.Exec(`DELETE FROM repo_seo_competitors WHERE repo_id = ?`, repoID)
	_, _ = tx.Exec(`DELETE FROM repo_seo_strategies WHERE repo_id = ?`, repoID)
	_, _ = tx.Exec(`DELETE FROM job_token_usage WHERE job_id IN (SELECT id FROM jobs WHERE repo_id = ?)`, repoID)
	_, _ = tx.Exec(`DELETE FROM jobs WHERE repo_id = ?`, repoID)
	_, _ = tx.Exec(`DELETE FROM repo_settings WHERE repo_id = ?`, repoID)

	if _, err := tx.Exec(`DELETE FROM repos WHERE id = ?`, repoID); err != nil {
		return fmt.Errorf("delete repo: %w", err)
	}

	return tx.Commit()
}

// ─── scanner helpers ──────────────────────────────────────────────────────────

type scanner interface {
	Scan(dest ...any) error
}

func scanJob(row scanner) (*Job, error) {
	j := &Job{}
	err := row.Scan(
		&j.ID, &j.RepoID, &j.TriggerType, &j.Status,
		&j.Branch, &j.HeadCommitSHA, &j.RepoSettingsHash,
		&j.PRURL, &j.ErrorMessage,
		&j.CreatedAt, &j.StartedAt, &j.FinishedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return j, err
}

