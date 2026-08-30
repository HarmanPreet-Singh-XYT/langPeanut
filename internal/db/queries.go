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
	existingMode := s.ExistingTranslationsMode
	if existingMode == "" {
		existingMode = "skip"
	}
	_, err = db.Exec(`
		INSERT INTO repo_settings(repo_id, locales_json, tone_preset, provider, model, safety_mode, chunk_word_budget, chunk_key_ceiling, custom_install_cmd, custom_build_cmd, root_dir, existing_translations_mode, encrypted_api_key_override, user_directive, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?, strftime('%Y-%m-%dT%H:%M:%SZ','now'))
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
			updated_at=excluded.updated_at`,
		s.RepoID, string(localesJSON), s.TonePreset, s.Provider, s.Model,
		safetyInt, s.ChunkWordBudget, s.ChunkKeyCeiling, s.CustomInstallCmd, s.CustomBuildCmd, s.RootDir, existingMode, s.EncryptedAPIKeyOverride, s.UserDirective)
	return err
}

func (db *DB) GetRepoSettings(repoID int64) (*RepoSettings, error) {
	var localesJSON string
	var safetyInt int
	var overrideBlob []byte
	s := &RepoSettings{RepoID: repoID}
	err := db.QueryRow(`SELECT locales_json, tone_preset, provider, model, safety_mode, chunk_word_budget, chunk_key_ceiling, COALESCE(custom_install_cmd, ''), COALESCE(custom_build_cmd, ''), COALESCE(root_dir, ''), COALESCE(existing_translations_mode, 'skip'), COALESCE(encrypted_api_key_override, X''), COALESCE(user_directive, ''), updated_at
		FROM repo_settings WHERE repo_id=?`, repoID).
		Scan(&localesJSON, &s.TonePreset, &s.Provider, &s.Model, &safetyInt, &s.ChunkWordBudget, &s.ChunkKeyCeiling, &s.CustomInstallCmd, &s.CustomBuildCmd, &s.RootDir, &s.ExistingTranslationsMode, &overrideBlob, &s.UserDirective, &s.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s.SafetyMode = safetyInt == 1
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
