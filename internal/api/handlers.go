// Package api contains all HTTP handlers for langpeanut-cloud.
// Routes are registered in RegisterRoutes(); the caller (cmd/server/main.go)
// owns the http.Server lifecycle.
//
// All handlers return JSON. Error responses follow the shape:
//   {"error": "human-readable message"}
package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/langPeanut/langpeanut-cloud/internal/auth"
	"github.com/langPeanut/langpeanut-cloud/internal/db"
	ghpkg "github.com/langPeanut/langPeanut/pkg/github"
	"github.com/langPeanut/langPeanut/pkg/logger"
)

// Handler groups dependencies needed by all route handlers.
type Handler struct {
	DB            *db.DB
	MasterKey     string // hex AES-256 key for encrypting/decrypting api_credentials
	AppID         string // GitHub App ID (numeric string)
	PrivateKeyPEM []byte // raw PEM bytes of the GitHub App's RSA private key
}

// RegisterRoutes wires all API routes onto mux.
func RegisterRoutes(mux *http.ServeMux, h *Handler) {
	// Health check — no auth needed, used by Docker HEALTHCHECK + Caddy probe.
	mux.HandleFunc("GET /health", h.handleHealth)

	// ── GitHub App Discovery & Repos ──────────────────────────────────────────
	// List available repos across GitHub App installations that can be imported.
	mux.HandleFunc("GET /api/github/available-repos", h.requireTeam(h.handleListAvailableGitHubRepos))

	// ── Repos ─────────────────────────────────────────────────────────────────
	// List repos the team has enabled.
	mux.HandleFunc("GET /api/repos", h.requireTeam(h.handleListRepos))
	// Enable (upsert) a repo for localization.
	mux.HandleFunc("POST /api/repos", h.requireTeam(h.handleUpsertRepo))

	// ── Repo Settings & Translation Matrix ───────────────────────────────────
	mux.HandleFunc("GET /api/repos/{repoID}/settings", h.requireTeam(h.handleGetSettings))
	mux.HandleFunc("PUT /api/repos/{repoID}/settings", h.requireTeam(h.handleUpsertSettings))
	mux.HandleFunc("PUT /api/repos/{repoID}/matrix", h.requireTeam(h.handleUpdateMatrixCell))

	// ── Jobs ──────────────────────────────────────────────────────────────────
	// List recent jobs for a repo.
	mux.HandleFunc("GET /api/repos/{repoID}/jobs", h.requireTeam(h.handleListJobs))
	// Manually trigger a new localization job.
	mux.HandleFunc("POST /api/repos/{repoID}/jobs", h.requireTeam(h.handleTriggerJob))
	// Get a specific job's status.
	mux.HandleFunc("GET /api/jobs/{jobID}", h.requireTeam(h.handleGetJob))

	// ── API Credentials (BYO LLM key) ─────────────────────────────────────────
	mux.HandleFunc("GET /api/credentials", h.requireTeam(h.handleListCredentials))
	mux.HandleFunc("PUT /api/credentials/{provider}", h.requireTeam(h.handleUpsertCredential))

	// ── Auth & User Profile ───────────────────────────────────────────────────
	mux.HandleFunc("POST /api/auth/login", h.handleLogin)
	mux.HandleFunc("GET /api/auth/me", h.requireTeam(h.handleGetMe))
	mux.HandleFunc("POST /api/auth/logout", h.handleLogout)

	// ── GitHub Webhook ────────────────────────────────────────────────────────
	mux.HandleFunc("POST /api/webhook", h.handleWebhook)
}

// ─── Health ──────────────────────────────────────────────────────────────────

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ─── GitHub App Available Repositories ────────────────────────────────────────

func (h *Handler) handleListAvailableGitHubRepos(w http.ResponseWriter, r *http.Request) {
	if h.AppID == "" || len(h.PrivateKeyPEM) == 0 {
		writeError(w, http.StatusServiceUnavailable, "GitHub App credentials not configured on server")
		return
	}

	pk, err := ghpkg.ParsePrivateKeyPEM(h.PrivateKeyPEM)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "parse private key: "+err.Error())
		return
	}

	appCfg := ghpkg.AppConfig{AppID: h.AppID, PrivateKey: pk}
	installs, err := ghpkg.ListInstallations(r.Context(), appCfg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list installations: "+err.Error())
		return
	}

	teamID := teamIDFromCtx(r)

	type availableRepo struct {
		InstallationID int64  `json:"installation_id"`
		AccountLogin   string `json:"account_login"`
		RepoID         int64  `json:"repo_id"`
		Owner          string `json:"owner"`
		Name           string `json:"name"`
		DefaultBranch  string `json:"default_branch"`
		Private        bool   `json:"private"`
		IsImported     bool   `json:"is_imported"`
	}

	var results []availableRepo
	for _, inst := range installs {
		// Auto-upsert installation in DB for this team if not exists
		dbInst, _ := h.DB.UpsertInstallation(teamID, inst.ID, inst.Account.Login)

		tok, err := ghpkg.CreateInstallationToken(r.Context(), appCfg, inst.ID)
		if err != nil {
			continue
		}
		repos, err := ghpkg.ListInstallationRepos(r.Context(), tok.Token)
		if err != nil {
			continue
		}

		// Get already imported repos for this installation
		existingRepos, _ := h.DB.ListReposByInstallation(dbInst.ID)
		importedMap := make(map[string]bool)
		for _, er := range existingRepos {
			importedMap[strings.ToLower(er.Owner+"/"+er.Name)] = true
		}

		for _, rp := range repos {
			owner := inst.Account.Login
			if parts := strings.Split(rp.FullName, "/"); len(parts) == 2 {
				owner = parts[0]
			}
			fullKey := strings.ToLower(owner + "/" + rp.Name)
			results = append(results, availableRepo{
				InstallationID: dbInst.ID,
				AccountLogin:   inst.Account.Login,
				RepoID:         rp.ID,
				Owner:          owner,
				Name:           rp.Name,
				DefaultBranch:  rp.DefaultBranch,
				Private:        rp.Private,
				IsImported:     importedMap[fullKey],
			})
		}
	}

	writeJSON(w, http.StatusOK, results)
}

// ─── Repos ───────────────────────────────────────────────────────────────────

func (h *Handler) handleListRepos(w http.ResponseWriter, r *http.Request) {
	teamID := teamIDFromCtx(r)
	installs, err := h.DB.ListInstallationsByTeam(teamID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list installations: "+err.Error())
		return
	}
	type repoWithSettings struct {
		*db.Repo
		HasSettings bool             `json:"has_settings"`
		Settings    *db.RepoSettings `json:"settings,omitempty"`
	}
	var out []repoWithSettings
	for _, inst := range installs {
		repos, err := h.DB.ListReposByInstallation(inst.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "list repos: "+err.Error())
			return
		}
		for _, rp := range repos {
			s, _ := h.DB.GetRepoSettings(rp.ID)
			out = append(out, repoWithSettings{
				Repo:        rp,
				HasSettings: s != nil,
				Settings:    s,
			})
		}
	}
	writeJSON(w, http.StatusOK, out)
}

type upsertRepoReq struct {
	InstallationID int64  `json:"installation_id"`
	Owner          string `json:"owner"`
	Name           string `json:"name"`
	DefaultBranch  string `json:"default_branch"`
}

func (h *Handler) handleUpsertRepo(w http.ResponseWriter, r *http.Request) {
	var req upsertRepoReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decode body: "+err.Error())
		return
	}
	if req.Owner == "" || req.Name == "" {
		writeError(w, http.StatusBadRequest, "owner and name are required")
		return
	}
	if req.DefaultBranch == "" {
		req.DefaultBranch = "main"
	}
	repo, err := h.DB.UpsertRepo(req.InstallationID, req.Owner, req.Name, req.DefaultBranch)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "upsert repo: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, repo)
}

// ─── Settings ────────────────────────────────────────────────────────────────

func (h *Handler) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	repoID, ok := parseRepoID(w, r)
	if !ok {
		return
	}
	s, err := h.DB.GetRepoSettings(repoID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if s == nil {
		writeError(w, http.StatusNotFound, "no settings configured for this repo")
		return
	}
	writeJSON(w, http.StatusOK, s)
}

type upsertSettingsReq struct {
	Locales                  []string `json:"locales"`
	TonePreset               string   `json:"tone_preset"`
	Provider                 string   `json:"provider"`
	Model                    string   `json:"model"`
	SafetyMode               bool     `json:"safety_mode"`
	ChunkWordBudget          int      `json:"chunk_word_budget"`
	ChunkKeyCeiling          int      `json:"chunk_key_ceiling"`
	CustomInstallCmd         string   `json:"custom_install_cmd"`
	CustomBuildCmd           string   `json:"custom_build_cmd"`
	RootDir                  string   `json:"root_dir"`
	ExistingTranslationsMode string   `json:"existing_translations_mode"`
}

func (h *Handler) handleUpsertSettings(w http.ResponseWriter, r *http.Request) {
	repoID, ok := parseRepoID(w, r)
	if !ok {
		return
	}
	var req upsertSettingsReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decode body: "+err.Error())
		return
	}
	if len(req.Locales) == 0 {
		writeError(w, http.StatusBadRequest, "at least one target locale is required")
		return
	}
	if req.Provider == "" {
		req.Provider = "openai"
	}
	if req.Model == "" {
		req.Model = "gpt-4o-mini"
	}
	if req.TonePreset == "" {
		req.TonePreset = "neutral"
	}
	if req.ChunkWordBudget == 0 {
		switch strings.ToLower(req.Provider) {
		case "openai", "claude", "anthropic", "gemini":
			req.ChunkWordBudget = 50000
		case "ollama", "custom", "local":
			req.ChunkWordBudget = 4000
		default:
			req.ChunkWordBudget = 50000
		}
	}
	if req.ChunkKeyCeiling == 0 {
		switch strings.ToLower(req.Provider) {
		case "openai", "claude", "anthropic", "gemini":
			req.ChunkKeyCeiling = 1500
		case "ollama", "custom", "local":
			req.ChunkKeyCeiling = 100
		default:
			req.ChunkKeyCeiling = 1500
		}
	}
	if req.ExistingTranslationsMode == "" {
		req.ExistingTranslationsMode = "skip"
	}
	s := &db.RepoSettings{
		RepoID:                   repoID,
		Locales:                  req.Locales,
		TonePreset:               req.TonePreset,
		Provider:                 req.Provider,
		Model:                    req.Model,
		SafetyMode:               req.SafetyMode,
		ChunkWordBudget:          req.ChunkWordBudget,
		ChunkKeyCeiling:          req.ChunkKeyCeiling,
		CustomInstallCmd:         req.CustomInstallCmd,
		CustomBuildCmd:           req.CustomBuildCmd,
		RootDir:                  req.RootDir,
		ExistingTranslationsMode: req.ExistingTranslationsMode,
	}
	if err := h.DB.UpsertRepoSettings(s); err != nil {
		writeError(w, http.StatusInternalServerError, "upsert settings: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type updateMatrixCellReq struct {
	Locale string `json:"locale"`
	Key    string `json:"key"`
	Value  string `json:"value"`
}

func (h *Handler) handleUpdateMatrixCell(w http.ResponseWriter, r *http.Request) {
	repoID, ok := parseRepoID(w, r)
	if !ok {
		return
	}
	var req updateMatrixCellReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Locale == "" || req.Key == "" {
		writeError(w, http.StatusBadRequest, "invalid payload: locale and key required")
		return
	}

	repo, err := h.DB.GetRepoByID(repoID)
	if err != nil || repo == nil {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}

	slog.Info("matrix: updated translation cell", "repo", repo.Owner+"/"+repo.Name, "locale", req.Locale, "key", req.Key)
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "saved",
		"locale":  req.Locale,
		"key":     req.Key,
		"message": fmt.Sprintf("Updated '%s' for [%s]", req.Key, req.Locale),
	})
}

// ─── Jobs ─────────────────────────────────────────────────────────────────────

func (h *Handler) handleListJobs(w http.ResponseWriter, r *http.Request) {
	repoID, ok := parseRepoID(w, r)
	if !ok {
		return
	}
	jobs, err := h.DB.ListJobsByRepo(repoID, 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, jobs)
}

func (h *Handler) handleTriggerJob(w http.ResponseWriter, r *http.Request) {
	repoID, ok := parseRepoID(w, r)
	if !ok {
		return
	}
	// Ensure settings exist before queueing.
	s, err := h.DB.GetRepoSettings(repoID)
	if err != nil || s == nil {
		writeError(w, http.StatusBadRequest, "configure repo settings before triggering a job")
		return
	}

	repo, err := h.DB.GetRepoByID(repoID)
	if err != nil || repo == nil {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}

	// Ensure API key credential exists for the selected provider
	inst, err := h.DB.GetInstallationByID(repo.InstallationID)
	if err == nil && inst != nil {
		cred, _ := h.DB.GetAPICredential(inst.TeamID, s.Provider)
		if cred == nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("no API key configured for provider '%s'. Please add your API key in Settings.", s.Provider))
			return
		}
	}

	job, err := h.DB.CreateJob(repoID, "manual")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create job: "+err.Error())
		return
	}
	slog.Info("api: job queued", "job_id", job.ID, "repo_id", repoID)
	writeJSON(w, http.StatusAccepted, job)
}

func (h *Handler) handleGetJob(w http.ResponseWriter, r *http.Request) {
	jobID, err := strconv.ParseInt(r.PathValue("jobID"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job id")
		return
	}
	job, err := h.DB.GetJobByID(jobID)
	if err != nil || job == nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	usage, _ := h.DB.ListTokenUsageByJob(jobID)
	writeJSON(w, http.StatusOK, map[string]any{
		"job":         job,
		"token_usage": usage,
	})
}

// ─── Credentials (BYO LLM Key) ───────────────────────────────────────────────

func (h *Handler) handleListCredentials(w http.ResponseWriter, r *http.Request) {
	teamID := teamIDFromCtx(r)
	providers := []string{"openai", "claude", "gemini", "deepl", "custom"}
	type providerStatus struct {
		Provider   string `json:"provider"`
		Configured bool   `json:"configured"`
	}
	var res []providerStatus
	for _, p := range providers {
		cred, _ := h.DB.GetAPICredential(teamID, p)
		res = append(res, providerStatus{Provider: p, Configured: cred != nil})
	}
	writeJSON(w, http.StatusOK, res)
}

type upsertCredentialReq struct {
	APIKey string `json:"api_key"`
}

func (h *Handler) handleUpsertCredential(w http.ResponseWriter, r *http.Request) {
	teamID := teamIDFromCtx(r)
	provider := r.PathValue("provider")
	if provider == "" {
		writeError(w, http.StatusBadRequest, "provider is required")
		return
	}
	var req upsertCredentialReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decode body: "+err.Error())
		return
	}
	if req.APIKey == "" {
		writeError(w, http.StatusBadRequest, "api_key is required")
		return
	}
	encrypted, err := auth.Encrypt(h.MasterKey, req.APIKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "encrypt key: "+err.Error())
		return
	}
	if err := h.DB.UpsertAPICredential(teamID, provider, encrypted); err != nil {
		writeError(w, http.StatusInternalServerError, "upsert credential: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "provider": provider})
}

// ─── Auth & User Profiles ───────────────────────────────────────────────────

type loginReq struct {
	Email       string `json:"email"`
	Name        string `json:"name"`
	GithubLogin string `json:"github_login"`
	AvatarURL   string `json:"avatar_url"`
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	_ = json.NewDecoder(r.Body).Decode(&req)

	if req.Email == "" {
		req.Email = "developer@langpeanut.ai"
	}
	if req.GithubLogin == "" {
		req.GithubLogin = "langpeanut-dev"
	}
	if req.Name == "" {
		req.Name = req.GithubLogin
	}
	if req.AvatarURL == "" {
		req.AvatarURL = fmt.Sprintf("https://github.com/%s.png", req.GithubLogin)
	}

	// Find or create default team
	team, err := h.DB.GetTeamByID(1)
	if err != nil || team == nil {
		team, err = h.DB.CreateTeam("Engineering Core")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "create team: "+err.Error())
			return
		}
	}

	user, err := h.DB.UpsertUser(team.ID, req.Email, req.Name, req.GithubLogin, req.AvatarURL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "upsert user: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user":        user,
		"team":        team,
		"token":       fmt.Sprintf("team_%d_user_%d", team.ID, user.ID),
		"permissions": []string{"repo:read", "repo:write", "pull_request:write"},
	})
}

func (h *Handler) handleGetMe(w http.ResponseWriter, r *http.Request) {
	teamID := teamIDFromCtx(r)
	team, err := h.DB.GetTeamByID(teamID)
	if err != nil || team == nil {
		team = &db.Team{ID: teamID, Name: "Engineering Core"}
	}

	var user *db.User
	if userEmail := r.Header.Get("X-User-Email"); userEmail != "" {
		user, _ = h.DB.GetUserByEmail(userEmail)
	}
	if user == nil {
		if userLogin := r.Header.Get("X-User-Login"); userLogin != "" {
			user, _ = h.DB.GetUserByGithubLogin(userLogin)
		}
	}
	if user == nil {
		user, _ = h.DB.GetLatestUserByTeam(teamID)
	}

	installs, _ := h.DB.ListInstallationsByTeam(teamID)
	var installLogins []string
	for _, inst := range installs {
		installLogins = append(installLogins, inst.AccountLogin)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user":          user,
		"team":          team,
		"installations": installLogins,
		"permissions": []string{
			"contents:read",
			"contents:write",
			"pull_requests:write",
			"metadata:read",
		},
	})
}

func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

// ─── Webhook (GitHub → langpeanut-cloud) ─────────────────────────────────────

func (h *Handler) handleWebhook(w http.ResponseWriter, r *http.Request) {
	event := r.Header.Get("X-GitHub-Event")
	slog.Info("webhook: received", "event", event)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "read body: "+err.Error())
		return
	}

	switch event {
	case "push":
		var pushEv ghpkg.PushEvent
		if err := json.Unmarshal(body, &pushEv); err != nil {
			writeError(w, http.StatusBadRequest, "decode push: "+err.Error())
			return
		}
		// Ignore commits not on default branch
		branch := strings.TrimPrefix(pushEv.Ref, "refs/heads/")
		if branch != pushEv.Repository.DefaultBranch && pushEv.Repository.DefaultBranch != "" {
			writeJSON(w, http.StatusOK, map[string]string{"status": "ignored_non_default_branch"})
			return
		}

		repo, err := h.DB.GetRepoByOwnerAndName(pushEv.Repository.Owner.Login, pushEv.Repository.Name)
		if err != nil || repo == nil {
			writeJSON(w, http.StatusOK, map[string]string{"status": "repo_not_enabled"})
			return
		}

		// Check settings
		settings, err := h.DB.GetRepoSettings(repo.ID)
		if err != nil || settings == nil || len(settings.Locales) == 0 {
			writeJSON(w, http.StatusOK, map[string]string{"status": "no_locales_configured"})
			return
		}

		// Queue job for continuous autopilot workflow
		job, err := h.DB.CreateJob(repo.ID, "webhook_push")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "queue job: "+err.Error())
			return
		}
		slog.Info("webhook: queued push job", "job_id", job.ID, "repo", repo.Owner+"/"+repo.Name)
		writeJSON(w, http.StatusOK, map[string]any{"status": "job_queued", "job_id": job.ID})
		return

	case "issue_comment":
		var commentEv ghpkg.IssueCommentEvent
		if err := json.Unmarshal(body, &commentEv); err != nil {
			writeError(w, http.StatusBadRequest, "decode issue_comment: "+err.Error())
			return
		}

		if !commentEv.IsPullRequestComment() {
			writeJSON(w, http.StatusOK, map[string]string{"status": "ignored_non_pr_comment"})
			return
		}

		botCmd, isBotCmd := ghpkg.ParseBotCommand(commentEv.Comment.Body)
		if !isBotCmd || botCmd == nil {
			writeJSON(w, http.StatusOK, map[string]string{"status": "no_bot_command"})
			return
		}

		repo, err := h.DB.GetRepoByOwnerAndName(commentEv.Repository.Owner.Login, commentEv.Repository.Name)
		if err != nil || repo == nil {
			writeJSON(w, http.StatusOK, map[string]string{"status": "repo_not_enabled"})
			return
		}

		// Queue PR bot on-demand localization job
		job, err := h.DB.CreateJob(repo.ID, "webhook_pr_comment")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "queue bot job: "+err.Error())
			return
		}
		slog.Info("webhook: queued @langpeanut PR bot job", "job_id", job.ID, "action", botCmd.Action, "pr", commentEv.Issue.Number)
		writeJSON(w, http.StatusOK, map[string]any{
			"status":     "bot_job_queued",
			"job_id":     job.ID,
			"bot_action": botCmd.Action,
			"pr_number":  commentEv.Issue.Number,
		})
		return

	case "installation_repositories", "installation":
		slog.Info("webhook: sync installation repositories event")
		writeJSON(w, http.StatusOK, map[string]string{"status": "installations_synced"})
		return

	default:
		writeJSON(w, http.StatusOK, map[string]string{"status": "unhandled_event", "event": event})
	}
}

// ─── Middleware ───────────────────────────────────────────────────────────────

func (h *Handler) requireTeam(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw := r.Header.Get("X-Team-ID")
		if raw == "" {
			// Default to team 1 for single-tenant / local development
			r = r.WithContext(contextWithTeamID(r.Context(), 1))
			next(w, r)
			return
		}
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid X-Team-ID")
			return
		}
		r = r.WithContext(contextWithTeamID(r.Context(), id))
		next(w, r)
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("api: encode response", "err", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	resp := map[string]any{"error": msg}
	advice := logger.ExplainError(fmt.Errorf("%s", msg))
	if advice != nil {
		resp["diagnostic"] = map[string]any{
			"title":          advice.Title,
			"subsystem":      advice.Subsystem,
			"root_cause":     advice.RootCause,
			"action_steps":   advice.ActionSteps,
			"auto_heal_note": advice.AutoHealNote,
		}
	}
	writeJSON(w, status, resp)
}

func parseRepoID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("repoID"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid repo id: %v", err))
		return 0, false
	}
	return id, true
}
