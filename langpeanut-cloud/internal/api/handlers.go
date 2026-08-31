// Package api contains all HTTP handlers for langpeanut-cloud.
// Routes are registered in RegisterRoutes(); the caller (cmd/server/main.go)
// owns the http.Server lifecycle.
//
// All handlers return JSON. Error responses follow the shape:
//   {"error": "human-readable message"}
package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/langPeanut/langpeanut-cloud/internal/auth"
	"github.com/langPeanut/langpeanut-cloud/internal/db"
	"github.com/langPeanut/langPeanut/pkg/agents"
	"github.com/langPeanut/langPeanut/pkg/chat"
	"github.com/langPeanut/langPeanut/pkg/genkit"
	ghpkg "github.com/langPeanut/langPeanut/pkg/github"
	"github.com/langPeanut/langPeanut/pkg/llm"
	"github.com/langPeanut/langPeanut/pkg/logger"
	"github.com/langPeanut/langPeanut/pkg/platforms"
	"github.com/langPeanut/langPeanut/pkg/seo"
	"github.com/langPeanut/langPeanut/pkg/types"
)

// Handler groups dependencies needed by all route handlers.
type Handler struct {
	DB            *db.DB
	MasterKey     string // hex AES-256 key for encrypting/decrypting api_credentials
	SessionSecret string // hex key for signing session cookies (falls back to MasterKey)
	AppID         string // GitHub App ID (numeric string)
	PrivateKeyPEM []byte // raw PEM bytes of the GitHub App's RSA private key

	// OAuth (user login) — the GitHub App's "user-to-server" OAuth credentials,
	// distinct from AppID/PrivateKeyPEM above.
	OAuthClientID     string
	OAuthClientSecret string
	WebhookSecret     string // GITHUB_WEBHOOK_SECRET; verifies X-Hub-Signature-256
	PublicBaseURL     string // e.g. https://app.langpeanut.ai — used to build the OAuth callback URL
}

// RegisterRoutes wires all API routes onto mux.
func RegisterRoutes(mux *http.ServeMux, h *Handler) {
	// Health check — no auth needed, used by Docker HEALTHCHECK + Caddy probe.
	mux.HandleFunc("GET /health", h.handleHealth)

	// ── GitHub App Discovery & Repos ──────────────────────────────────────────
	// List available repos across GitHub App installations that can be imported.
	mux.HandleFunc("GET /api/github/available-repos", h.requireSession(h.handleListAvailableGitHubRepos))

	// ── Repos ─────────────────────────────────────────────────────────────────
	// List repos the team has enabled.
	mux.HandleFunc("GET /api/repos", h.requireSession(h.handleListRepos))
	// Enable (upsert) a repo for localization.
	mux.HandleFunc("POST /api/repos", h.requireSession(h.handleUpsertRepo))
	// Reset all translations, jobs, and SEO data for a repo (fresh start).
	mux.HandleFunc("POST /api/repos/{repoID}/reset", h.requireSession(h.handleResetRepoData))
	// Delete a repo entirely.
	mux.HandleFunc("DELETE /api/repos/{repoID}", h.requireSession(h.handleDeleteRepo))

	// ── Repo Settings & Translation Matrix ───────────────────────────────────
	mux.HandleFunc("GET /api/repos/{repoID}/settings", h.requireSession(h.handleGetSettings))
	mux.HandleFunc("PUT /api/repos/{repoID}/settings", h.requireSession(h.handleUpsertSettings))
	mux.HandleFunc("POST /api/repos/{repoID}/model", h.requireSession(h.handleQuickSwitchModel))
	mux.HandleFunc("GET /api/repos/{repoID}/matrix", h.requireSession(h.handleGetMatrix))
	mux.HandleFunc("PUT /api/repos/{repoID}/matrix", h.requireSession(h.handleUpdateMatrixCell))
	mux.HandleFunc("POST /api/repos/{repoID}/matrix/copilot", h.requireSession(h.handleMatrixCopilot))
	mux.HandleFunc("GET /api/repos/{repoID}/branches", h.requireSession(h.handleListBranches))

	// ── SEO & Market Growth Studio ────────────────────────────────────────────
	mux.HandleFunc("GET /api/repos/{repoID}/seo", h.requireSession(h.handleGetSEOOverview))
	mux.HandleFunc("POST /api/repos/{repoID}/seo/strategy", h.requireSession(h.handleSaveSEOStrategy))
	mux.HandleFunc("POST /api/repos/{repoID}/seo/analyze-domain", h.requireSession(h.handleAnalyzeSEODomain))
	mux.HandleFunc("POST /api/repos/{repoID}/seo/scout", h.requireSession(h.handleRunSEOScout))
	mux.HandleFunc("POST /api/repos/{repoID}/seo/optimize", h.requireSession(h.handleRunSEOOptimize))
	mux.HandleFunc("POST /api/repos/{repoID}/seo/apply", h.requireSession(h.handleApplySEOToMatrix))

	// ── Google Genkit Workflows & Autonomous Copilot ───────────────────────
	mux.HandleFunc("GET /api/repos/{repoID}/genkit/runtime", h.requireSession(h.handleGenkitRuntime))
	mux.HandleFunc("GET /api/repos/{repoID}/genkit/flows", h.requireSession(h.handleGenkitFlows))
	mux.HandleFunc("POST /api/repos/{repoID}/genkit/flow/{flowName}", h.requireSession(h.handleGenkitFlowRun))
	mux.HandleFunc("GET /api/repos/{repoID}/doctor", h.requireSession(h.handleRepoDoctor))
	mux.HandleFunc("POST /api/repos/{repoID}/discover-persona", h.requireSession(h.handleDiscoverPersona))
	mux.HandleFunc("GET /api/repos/{repoID}/dead-keys", h.requireSession(h.handleGetDeadKeys))
	mux.HandleFunc("POST /api/repos/{repoID}/prune-keys", h.requireSession(h.handlePruneDeadKeys))
	mux.HandleFunc("POST /api/repos/{repoID}/chat", h.requireSession(h.handleRepoChat))

	// ── Jobs ──────────────────────────────────────────────────────────────────
	// List recent jobs for a repo.
	mux.HandleFunc("GET /api/repos/{repoID}/jobs", h.requireSession(h.handleListJobs))
	// Manually trigger a new localization job.
	mux.HandleFunc("POST /api/repos/{repoID}/jobs", h.requireSession(h.handleTriggerJob))
	// Get execution logs for a job.
	mux.HandleFunc("GET /api/repos/{repoID}/jobs/{jobID}/logs", h.requireSession(h.handleGetJobLogs))
	// Get a specific job's status.
	mux.HandleFunc("GET /api/jobs/{jobID}", h.requireSession(h.handleGetJob))

	// ── API Credentials (BYO LLM key) ─────────────────────────────────────────
	mux.HandleFunc("GET /api/credentials", h.requireSession(h.handleListCredentials))
	mux.HandleFunc("PUT /api/credentials/{provider}", h.requireSession(h.handleUpsertCredential))

	// ── Auth & User Profile ───────────────────────────────────────────────────
	// GitHub OAuth login: browser is redirected through these two, not called via fetch().
	mux.HandleFunc("GET /api/auth/github/start", h.handleGitHubOAuthStart)
	mux.HandleFunc("GET /api/auth/github/callback", h.handleGitHubOAuthCallback)
	mux.HandleFunc("GET /api/auth/me", h.requireSession(h.handleGetMe))
	mux.HandleFunc("POST /api/auth/logout", h.handleLogout)
	mux.HandleFunc("GET /api/app/info", h.handleAppInfo)

	// ── GitHub Webhook & Automation Testing ──────────────────────────────────
	mux.HandleFunc("POST /api/webhook", h.handleWebhook)
	mux.HandleFunc("POST /api/repos/{repoID}/webhook/test-push", h.requireSession(h.handleTestWebhookPush))
	mux.HandleFunc("POST /api/repos/{repoID}/webhook/test-bot", h.requireSession(h.handleTestWebhookBot))
}

// ─── App Info & Health ───────────────────────────────────────────────────────

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleAppInfo(w http.ResponseWriter, r *http.Request) {
	appSlug := os.Getenv("GITHUB_APP_SLUG")
	if appSlug == "" {
		appSlug = "langpeanut-localization-bot"
	}
	installURL := "https://github.com/apps/" + appSlug + "/installations/new"
	if custom := os.Getenv("GITHUB_APP_INSTALL_URL"); custom != "" {
		installURL = custom
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"app_id":      h.AppID,
		"client_id":   h.OAuthClientID,
		"app_slug":    appSlug,
		"install_url": installURL,
	})
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

	sess := sessionFromCtx(r)
	teamID := sess.TeamID
	user, _ := h.DB.GetUserByID(sess.UserID)

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

	teamInstalls, _ := h.DB.ListInstallationsByTeam(teamID)
	teamInstMap := make(map[int64]bool)
	for _, ti := range teamInstalls {
		teamInstMap[ti.InstallationID] = true
	}

	var results []availableRepo
	for _, inst := range installs {
		// Only display installations that match the authenticated user's GitHub username or team
		if user != nil && user.GithubLogin != "" {
			if !strings.EqualFold(inst.Account.Login, user.GithubLogin) && !teamInstMap[inst.ID] {
				continue
			}
		}

		// Auto-upsert installation in DB for this team if authorized
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
	teamID := sessionFromCtx(r).TeamID
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
	teamID := sessionFromCtx(r).TeamID
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
	inst, err := h.DB.GetInstallationByID(req.InstallationID)
	if err != nil || inst == nil || inst.TeamID != teamID {
		writeError(w, http.StatusForbidden, "installation not found for your team")
		return
	}
	repo, err := h.DB.UpsertRepo(req.InstallationID, req.Owner, req.Name, req.DefaultBranch)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "upsert repo: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, repo)
}

func (h *Handler) handleResetRepoData(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepo(w, r)
	if !ok {
		return
	}

	if err := h.DB.ResetRepoData(repo.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "reset repo data: "+err.Error())
		return
	}

	// Remove mirror cache and working checkout if exists
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "data"
	}
	mirrorPath := filepath.Join(dataDir, "mirrors", fmt.Sprintf("%d.git", repo.ID))
	_ = os.RemoveAll(mirrorPath)
	repoPath := filepath.Join(dataDir, "repos", fmt.Sprintf("%d", repo.ID))
	_ = os.RemoveAll(repoPath)

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"message": fmt.Sprintf("Successfully reset all translation matrix keys, jobs, and SEO data for %s/%s. You can now start fresh from the beginning.", repo.Owner, repo.Name),
		"repo_id": repo.ID,
	})
}

func (h *Handler) handleDeleteRepo(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepo(w, r)
	if !ok {
		return
	}

	if err := h.DB.DeleteRepo(repo.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "delete repo: "+err.Error())
		return
	}

	// Clean up mirror bare git repo and working checkout
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "data"
	}
	mirrorPath := filepath.Join(dataDir, "mirrors", fmt.Sprintf("%d.git", repo.ID))
	_ = os.RemoveAll(mirrorPath)
	repoPath := filepath.Join(dataDir, "repos", fmt.Sprintf("%d", repo.ID))
	_ = os.RemoveAll(repoPath)

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"message": fmt.Sprintf("Repository %s/%s deleted successfully.", repo.Owner, repo.Name),
		"repo_id": repo.ID,
	})
}

// authorizeRepo loads a repo by path-parameter ID and verifies it belongs to
// the caller's team (via its installation), writing a 403/404 response and
// returning ok=false if not. Every {repoID}-scoped handler must call this
// before touching the repo — sessions authenticate *who* you are, this
// authorizes *which repos* you're allowed to act on.
func (h *Handler) authorizeRepo(w http.ResponseWriter, r *http.Request) (*db.Repo, bool) {
	repoID, ok := parseRepoID(w, r)
	if !ok {
		return nil, false
	}
	repo, err := h.DB.GetRepoByID(repoID)
	if err != nil || repo == nil {
		writeError(w, http.StatusNotFound, "repo not found")
		return nil, false
	}
	inst, err := h.DB.GetInstallationByID(repo.InstallationID)
	if err != nil || inst == nil || inst.TeamID != sessionFromCtx(r).TeamID {
		writeError(w, http.StatusForbidden, "repo not found")
		return nil, false
	}
	return repo, true
}

// ─── Settings ────────────────────────────────────────────────────────────────

func (h *Handler) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepo(w, r)
	if !ok {
		return
	}
	s, err := h.DB.GetRepoSettings(repo.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if s == nil {
		writeError(w, http.StatusNotFound, "no settings configured for this repo")
		return
	}
	resp := map[string]any{
		"repo_id":                      s.RepoID,
		"locales":                      s.Locales,
		"tone_preset":                  s.TonePreset,
		"provider":                     s.Provider,
		"model":                        s.Model,
		"safety_mode":                  s.SafetyMode,
		"chunk_word_budget":            s.ChunkWordBudget,
		"chunk_key_ceiling":            s.ChunkKeyCeiling,
		"custom_install_cmd":           s.CustomInstallCmd,
		"custom_build_cmd":             s.CustomBuildCmd,
		"root_dir":                     s.RootDir,
		"existing_translations_mode":   s.ExistingTranslationsMode,
		"user_directive":               s.UserDirective,
		"webhook_push_enabled":         s.WebhookPushEnabled,
		"webhook_branch_filter":        s.WebhookBranchFilter,
		"webhook_custom_branches":      s.WebhookCustomBranches,
		"webhook_action":               s.WebhookAction,
		"webhook_pr_comments_enabled":   s.WebhookPRCommentsEnabled,
		"webhook_custom_branch_prefix": s.WebhookCustomBranchPrefix,
		"webhook_path_filter":          s.WebhookPathFilter,
		"has_api_key_override":         len(s.EncryptedAPIKeyOverride) > 0,
		"updated_at":                   s.UpdatedAt,
	}
	writeJSON(w, http.StatusOK, resp)
}

type upsertSettingsReq struct {
	Locales                    []string `json:"locales"`
	TonePreset                 string   `json:"tone_preset"`
	Provider                   string   `json:"provider"`
	Model                      string   `json:"model"`
	SafetyMode                 bool     `json:"safety_mode"`
	ChunkWordBudget            int      `json:"chunk_word_budget"`
	ChunkKeyCeiling            int      `json:"chunk_key_ceiling"`
	CustomInstallCmd           string   `json:"custom_install_cmd"`
	CustomBuildCmd             string   `json:"custom_build_cmd"`
	RootDir                    string   `json:"root_dir"`
	ExistingTranslationsMode   string   `json:"existing_translations_mode"`
	UserDirective              string   `json:"user_directive,omitempty"`
	APIKeyOverride             string   `json:"api_key_override,omitempty"`
	WebhookPushEnabled         *bool    `json:"webhook_push_enabled,omitempty"`
	WebhookBranchFilter        string   `json:"webhook_branch_filter,omitempty"`
	WebhookCustomBranches      string   `json:"webhook_custom_branches,omitempty"`
	WebhookAction              string   `json:"webhook_action,omitempty"`
	WebhookPRCommentsEnabled   *bool    `json:"webhook_pr_comments_enabled,omitempty"`
	WebhookCustomBranchPrefix  string   `json:"webhook_custom_branch_prefix,omitempty"`
	WebhookPathFilter          string   `json:"webhook_path_filter,omitempty"`
}

func (h *Handler) handleUpsertSettings(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepo(w, r)
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
		req.Provider = "gemini"
	}
	if req.Model == "" {
		req.Model = "gemini-3.7-flash"
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

	existingSettings, _ := h.DB.GetRepoSettings(repo.ID)
	var encryptedOverride []byte
	if existingSettings != nil {
		encryptedOverride = existingSettings.EncryptedAPIKeyOverride
	}

	if req.APIKeyOverride == "__CLEAR__" {
		encryptedOverride = nil
	} else if req.APIKeyOverride != "" {
		enc, err := auth.Encrypt(h.MasterKey, req.APIKeyOverride)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "encrypt api key override: "+err.Error())
			return
		}
		encryptedOverride = enc
	}

	pushEnabled := true
	if req.WebhookPushEnabled != nil {
		pushEnabled = *req.WebhookPushEnabled
	} else if existingSettings != nil {
		pushEnabled = existingSettings.WebhookPushEnabled
	}

	prCommentsEnabled := true
	if req.WebhookPRCommentsEnabled != nil {
		prCommentsEnabled = *req.WebhookPRCommentsEnabled
	} else if existingSettings != nil {
		prCommentsEnabled = existingSettings.WebhookPRCommentsEnabled
	}

	branchFilter := req.WebhookBranchFilter
	if branchFilter == "" {
		if existingSettings != nil && existingSettings.WebhookBranchFilter != "" {
			branchFilter = existingSettings.WebhookBranchFilter
		} else {
			branchFilter = "default_branch"
		}
	}

	action := req.WebhookAction
	if action == "" {
		if existingSettings != nil && existingSettings.WebhookAction != "" {
			action = existingSettings.WebhookAction
		} else {
			action = "auto_pr"
		}
	}

	branchPrefix := req.WebhookCustomBranchPrefix
	if branchPrefix == "" {
		if existingSettings != nil && existingSettings.WebhookCustomBranchPrefix != "" {
			branchPrefix = existingSettings.WebhookCustomBranchPrefix
		} else {
			branchPrefix = "langpeanut/i18n-"
		}
	}

	s := &db.RepoSettings{
		RepoID:                    repo.ID,
		Locales:                   req.Locales,
		TonePreset:                req.TonePreset,
		Provider:                  req.Provider,
		Model:                     req.Model,
		SafetyMode:                req.SafetyMode,
		ChunkWordBudget:           req.ChunkWordBudget,
		ChunkKeyCeiling:           req.ChunkKeyCeiling,
		CustomInstallCmd:          req.CustomInstallCmd,
		CustomBuildCmd:            req.CustomBuildCmd,
		RootDir:                   req.RootDir,
		ExistingTranslationsMode:  req.ExistingTranslationsMode,
		EncryptedAPIKeyOverride:   encryptedOverride,
		UserDirective:             req.UserDirective,
		WebhookPushEnabled:        pushEnabled,
		WebhookBranchFilter:       branchFilter,
		WebhookCustomBranches:     req.WebhookCustomBranches,
		WebhookAction:             action,
		WebhookPRCommentsEnabled:  prCommentsEnabled,
		WebhookCustomBranchPrefix: branchPrefix,
		WebhookPathFilter:         req.WebhookPathFilter,
	}
	if err := h.DB.UpsertRepoSettings(s); err != nil {
		writeError(w, http.StatusInternalServerError, "upsert settings: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleQuickSwitchModel(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepo(w, r)
	if !ok {
		return
	}
	var req struct {
		Provider string `json:"provider"`
		Model    string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Provider == "" || req.Model == "" {
		writeError(w, http.StatusBadRequest, "provider and model are required")
		return
	}

	settings, err := h.DB.GetRepoSettings(repo.ID)
	if err != nil || settings == nil {
		settings = &db.RepoSettings{
			RepoID:                   repo.ID,
			Locales:                  []string{"es", "fr", "de"},
			TonePreset:               "neutral",
			Provider:                 req.Provider,
			Model:                    req.Model,
			SafetyMode:               true,
			ChunkWordBudget:          50000,
			ChunkKeyCeiling:          1500,
			ExistingTranslationsMode: "skip",
		}
	} else {
		settings.Provider = req.Provider
		settings.Model = req.Model
	}

	if err := h.DB.UpsertRepoSettings(settings); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save model switch: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message":  "model updated successfully",
		"provider": req.Provider,
		"model":    req.Model,
	})
}

func (h *Handler) handleGetMatrix(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepo(w, r)
	if !ok {
		return
	}
	matrix, err := h.DB.GetTranslationMatrix(repo.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get matrix: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, matrix)
}

type updateMatrixCellReq struct {
	Locale string `json:"locale"`
	Key    string `json:"key"`
	Value  string `json:"value"`
}

func (h *Handler) handleUpdateMatrixCell(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepo(w, r)
	if !ok {
		return
	}
	var req updateMatrixCellReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Locale == "" || req.Key == "" {
		writeError(w, http.StatusBadRequest, "invalid payload: locale and key required")
		return
	}

	if err := h.DB.UpdateTranslationCell(repo.ID, req.Locale, req.Key, req.Value); err != nil {
		writeError(w, http.StatusInternalServerError, "update cell: "+err.Error())
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

type matrixCopilotReq struct {
	Key                string `json:"key"`
	SourceLocale       string `json:"source_locale"`
	SourceText         string `json:"source_text"`
	TargetLocale       string `json:"target_locale"`
	CurrentTranslation string `json:"current_translation"`
	Instruction        string `json:"instruction"`
	ApplyDirectly      bool   `json:"apply_directly"`
}

type matrixCopilotResp struct {
	Key             string `json:"key"`
	TargetLocale    string `json:"target_locale"`
	TranslatedText  string `json:"translated_text"`
	Explanation     string `json:"explanation"`
	ICUVariablesOk  bool   `json:"icu_variables_ok"`
	LengthReduction string `json:"length_reduction,omitempty"`
}

func (h *Handler) handleMatrixCopilot(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepo(w, r)
	if !ok {
		return
	}
	var req matrixCopilotReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Key == "" || req.TargetLocale == "" {
		writeError(w, http.StatusBadRequest, "invalid payload: key and target_locale required")
		return
	}
	if req.SourceLocale == "" {
		req.SourceLocale = "en"
	}
	if req.SourceText == "" {
		req.SourceText = req.CurrentTranslation
	}
	if req.SourceText == "" {
		req.SourceText = req.Key
	}

	settings, err := h.DB.GetRepoSettings(repo.ID)
	if err != nil || settings == nil {
		writeError(w, http.StatusBadRequest, "please configure repository settings and AI provider first")
		return
	}

	// Resolve API Key: check repo override first, then team's global credential
	var encKey []byte
	if len(settings.EncryptedAPIKeyOverride) > 0 {
		encKey = settings.EncryptedAPIKeyOverride
	} else {
		inst, err := h.DB.GetInstallationByID(repo.InstallationID)
		if err == nil && inst != nil {
			cred, _ := h.DB.GetAPICredential(inst.TeamID, settings.Provider)
			if cred != nil {
				encKey = cred.EncryptedKey
			}
		}
	}

	if len(encKey) == 0 {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("no API key configured for provider '%s'. Set an API key in Global AI Keys Vault or Repo Settings.", settings.Provider))
		return
	}

	apiKey, err := auth.Decrypt(h.MasterKey, encKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "decrypt api key: "+err.Error())
		return
	}

	client := llm.NewClientWithAPIKey(llm.ProviderType(settings.Provider), settings.Model, apiKey)

	systemPrompt := `You are langPeanut Translation Copilot, an expert localization AI agent.
Your task is to translate or re-synthesize a single UI string according to the user's specific directive.
CRITICAL INVARIANTS:
1. Preserve all ICU variables, placeholders, and tokens exactly (e.g. {userName}, {count}, (${total}), %s, @value).
2. Do NOT wrap output in markdown code fences or markdown blocks. Return ONLY valid JSON in this exact shape:
{"translated_text": "...", "explanation": "Brief 1-sentence note explaining the translation decision"}`

	directive := req.Instruction
	if directive == "" || directive == "shorter" {
		directive = "Make the translation more concise and shorter (reduce length by ~20-35% without losing core meaning so it fits compact mobile buttons)."
	} else if directive == "casual" {
		directive = "Use a friendly, warm, colloquial, engaging tone suitable for modern apps."
	} else if directive == "formal" {
		directive = "Use an authoritative, precise, professional enterprise-grade B2B tone."
	} else if directive == "brand_safe" {
		directive = "Ensure all brand names and technical terms remain completely untranslated in English."
	}

	userPrompt := fmt.Sprintf("Source Key: %s\nSource Language: %s\nSource Text: \"%s\"\nTarget Language: %s\nCurrent Translation: \"%s\"\nUser Directive: %s\n\nGenerate the perfected translation for locale [%s].",
		req.Key, req.SourceLocale, req.SourceText, req.TargetLocale, req.CurrentTranslation, directive, req.TargetLocale)

	ctx := r.Context()
	llmOut, err := client.Complete(ctx, systemPrompt, userPrompt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "AI Copilot completion failed: "+err.Error())
		return
	}

	type aiResult struct {
		TranslatedText string `json:"translated_text"`
		Explanation    string `json:"explanation"`
	}
	var res aiResult
	cleanJSON := strings.TrimSpace(llmOut)
	if strings.HasPrefix(cleanJSON, "```") {
		lines := strings.Split(cleanJSON, "\n")
		if len(lines) > 2 {
			cleanJSON = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}
	if err := json.Unmarshal([]byte(cleanJSON), &res); err != nil || res.TranslatedText == "" {
		res.TranslatedText = strings.Trim(strings.TrimSpace(llmOut), "\"`'")
		res.Explanation = fmt.Sprintf("Synthesized via %s (%s)", settings.Provider, settings.Model)
	}

	// ICU variable validation
	icuOk := true
	reICU := regexp.MustCompile(`\{[a-zA-Z0-9_]+\}`)
	srcTokens := reICU.FindAllString(req.SourceText, -1)
	for _, tok := range srcTokens {
		if !strings.Contains(res.TranslatedText, tok) {
			icuOk = false
			break
		}
	}

	var lenRed string
	if len(req.CurrentTranslation) > 0 && len(res.TranslatedText) < len(req.CurrentTranslation) {
		pct := int(float64(len(req.CurrentTranslation)-len(res.TranslatedText)) / float64(len(req.CurrentTranslation)) * 100)
		lenRed = fmt.Sprintf("-%d%% shorter", pct)
	}

	if req.ApplyDirectly {
		_ = h.DB.UpdateTranslationCell(repo.ID, req.TargetLocale, req.Key, res.TranslatedText)
	}

	writeJSON(w, http.StatusOK, matrixCopilotResp{
		Key:             req.Key,
		TargetLocale:    req.TargetLocale,
		TranslatedText:  res.TranslatedText,
		Explanation:     res.Explanation,
		ICUVariablesOk:  icuOk,
		LengthReduction: lenRed,
	})
}

// ─── Autonomous Agentic Endpoints: Doctor, Persona Scout & Pruner ─────────────

func (h *Handler) getRepoScanDir(repoID int64) string {
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		if mDir := os.Getenv("MIRRORS_DIR"); mDir != "" {
			dataDir = filepath.Dir(mDir)
		} else {
			dataDir = "data"
		}
	}
	repoDir := filepath.Join(dataDir, "repos", fmt.Sprintf("%d", repoID))
	if fi, err := os.Stat(repoDir); err == nil && fi.IsDir() {
		// Check if directory is non-empty
		entries, _ := os.ReadDir(repoDir)
		if len(entries) > 0 {
			return repoDir
		}
	}

	// 1. If bare mirror exists, clone working copy for this repo
	mirrorPath := filepath.Join(dataDir, "mirrors", fmt.Sprintf("%d.git", repoID))
	if _, err := os.Stat(mirrorPath); err == nil {
		_ = os.MkdirAll(filepath.Dir(repoDir), 0750)
		cmd := exec.Command("git", "clone", mirrorPath, repoDir)
		if err := cmd.Run(); err == nil {
			return repoDir
		}
	}

	// 2. Check if this specific repo has an active job directory
	jobs, err := h.DB.ListJobsByRepo(repoID, 1)
	if err == nil && len(jobs) > 0 {
		targetRepoDir := filepath.Join(dataDir, "jobs", fmt.Sprintf("%d", jobs[0].ID), "repo")
		if _, err := os.Stat(targetRepoDir); err == nil {
			return targetRepoDir
		}
	}

	// 3. Cold-start on-demand shallow clone from GitHub if App credentials available
	repo, rErr := h.DB.GetRepoByID(repoID)
	if rErr == nil && repo != nil && len(h.PrivateKeyPEM) > 0 && h.AppID != "" {
		inst, iErr := h.DB.GetInstallationByID(repo.InstallationID)
		if iErr == nil && inst != nil {
			pk, pErr := ghpkg.ParsePrivateKeyPEM(h.PrivateKeyPEM)
			if pErr == nil && pk != nil {
				appCfg := ghpkg.AppConfig{AppID: h.AppID, PrivateKey: pk}
				tok, tErr := ghpkg.CreateInstallationToken(context.Background(), appCfg, inst.InstallationID)
				if tErr == nil && tok != nil && tok.Token != "" {
					authURL := fmt.Sprintf("https://x-access-token:%s@github.com/%s/%s.git", tok.Token, repo.Owner, repo.Name)
					_ = os.RemoveAll(repoDir)
					_ = os.MkdirAll(filepath.Dir(repoDir), 0750)
					cloneCmd := exec.Command("git", "clone", "--depth", "1", authURL, repoDir)
					if err := cloneCmd.Run(); err == nil {
						return repoDir
					}
				}
			}
		}
	}

	// Return isolated repoDir path (ensure directory exists so scanner doesn't inspect server root)
	_ = os.MkdirAll(repoDir, 0750)
	return repoDir
}

func (h *Handler) handleRepoDoctor(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepo(w, r)
	if !ok {
		return
	}
	scanDir := h.getRepoScanDir(repo.ID)
	registry := platforms.NewRegistry()
	platform, _ := registry.AutoDetect(scanDir)

	doctor := agents.NewDoctorAgent(platform)
	report, err := doctor.DiagnoseProject(scanDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "doctor audit failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (h *Handler) handleDiscoverPersona(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepo(w, r)
	if !ok {
		return
	}
	settings, err := h.DB.GetRepoSettings(repo.ID)
	if err != nil || settings == nil {
		writeError(w, http.StatusBadRequest, "please configure repo settings first")
		return
	}

	var apiKey string
	var encKey []byte
	if len(settings.EncryptedAPIKeyOverride) > 0 {
		encKey = settings.EncryptedAPIKeyOverride
	} else {
		inst, err := h.DB.GetInstallationByID(repo.InstallationID)
		if err == nil && inst != nil {
			cred, _ := h.DB.GetAPICredential(inst.TeamID, settings.Provider)
			if cred != nil {
				encKey = cred.EncryptedKey
			}
		}
	}
	if len(encKey) > 0 {
		apiKey, _ = auth.Decrypt(h.MasterKey, encKey)
	}

	var client llm.Client
	if apiKey != "" {
		client = llm.NewClientWithAPIKey(llm.ProviderType(settings.Provider), settings.Model, apiKey)
	}

	scanDir := h.getRepoScanDir(repo.ID)
	scout := agents.NewPersonaScoutAgent(client)
	report, err := scout.DiscoverPersona(scanDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "persona discovery failed: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, report)
}

func (h *Handler) handleGetDeadKeys(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepo(w, r)
	if !ok {
		return
	}
	scanDir := h.getRepoScanDir(repo.ID)
	registry := platforms.NewRegistry()
	platform, _ := registry.AutoDetect(scanDir)

	pruner := agents.NewPrunerAgent(platform)
	report, err := pruner.AnalyzeDeadKeys(scanDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "dead key analysis failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (h *Handler) handlePruneDeadKeys(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepo(w, r)
	if !ok {
		return
	}
	scanDir := h.getRepoScanDir(repo.ID)
	registry := platforms.NewRegistry()
	platform, _ := registry.AutoDetect(scanDir)

	pruner := agents.NewPrunerAgent(platform)
	report, err := pruner.PruneDeadKeys(scanDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "prune keys failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, report)
}

// ─── Jobs ─────────────────────────────────────────────────────────────────────

func (h *Handler) handleListJobs(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepo(w, r)
	if !ok {
		return
	}
	jobs, err := h.DB.ListJobsByRepo(repo.ID, 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, jobs)
}

type triggerJobReq struct {
	Branch        string `json:"branch,omitempty"`
	UserDirective string `json:"user_directive,omitempty"`
}

type branchInfo struct {
	Name      string `json:"name"`
	IsDefault bool   `json:"is_default"`
	Protected bool   `json:"protected"`
}

func (h *Handler) handleListBranches(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepo(w, r)
	if !ok {
		return
	}

	branches := []branchInfo{}
	defaultBranch := repo.DefaultBranch
	if defaultBranch == "" {
		defaultBranch = "main"
	}

	// Try querying GitHub API with installation token
	inst, err := h.DB.GetInstallationByID(repo.InstallationID)
	if err == nil && inst != nil && len(h.PrivateKeyPEM) > 0 {
		pk, err := ghpkg.ParsePrivateKeyPEM(h.PrivateKeyPEM)
		if err == nil && pk != nil {
			appCfg := ghpkg.AppConfig{
				AppID:      h.AppID,
				PrivateKey: pk,
			}
			tok, err := ghpkg.CreateInstallationToken(r.Context(), appCfg, inst.InstallationID)
			if err == nil && tok != nil && tok.Token != "" {
				req, reqErr := http.NewRequestWithContext(r.Context(), "GET",
					fmt.Sprintf("https://api.github.com/repos/%s/%s/branches?per_page=100", repo.Owner, repo.Name), nil)
				if reqErr == nil {
				req.Header.Set("Authorization", "Bearer "+tok.Token)
				req.Header.Set("Accept", "application/vnd.github+json")
				resp, httpErr := http.DefaultClient.Do(req)
				if httpErr == nil && resp.StatusCode == http.StatusOK {
					var ghBranches []struct {
						Name      string `json:"name"`
						Protected bool   `json:"protected"`
					}
					if json.NewDecoder(resp.Body).Decode(&ghBranches) == nil {
						for _, b := range ghBranches {
							branches = append(branches, branchInfo{
								Name:      b.Name,
								IsDefault: b.Name == defaultBranch,
								Protected: b.Protected,
							})
						}
					}
					resp.Body.Close()
				}
			}
		}
	}
}

	// Fallback if no remote branches returned
	if len(branches) == 0 {
		branches = append(branches, branchInfo{
			Name:      defaultBranch,
			IsDefault: true,
			Protected: false,
		})
	}

	writeJSON(w, http.StatusOK, branches)
}

func (h *Handler) handleTriggerJob(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepo(w, r)
	if !ok {
		return
	}
	// Ensure settings exist before queueing.
	s, err := h.DB.GetRepoSettings(repo.ID)
	if err != nil || s == nil {
		writeError(w, http.StatusBadRequest, "configure repo settings before triggering a job")
		return
	}

	var req triggerJobReq
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.UserDirective != "" && req.UserDirective != s.UserDirective {
		s.UserDirective = req.UserDirective
		_ = h.DB.UpsertRepoSettings(s)
	}

	// Ensure API key credential exists (check repo-specific override first, then global team key)
	if len(s.EncryptedAPIKeyOverride) == 0 {
		inst, err := h.DB.GetInstallationByID(repo.InstallationID)
		if err == nil && inst != nil {
			cred, _ := h.DB.GetAPICredential(inst.TeamID, s.Provider)
			if cred == nil {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("no API key configured for provider '%s'. Please add your API key in Global AI Keys Vault or Repo Settings.", s.Provider))
				return
			}
		}
	}

	targetBranch := strings.TrimSpace(req.Branch)
	if targetBranch == "" {
		targetBranch = repo.DefaultBranch
	}
	if targetBranch == "" {
		targetBranch = "main"
	}

	job, err := h.DB.CreateJobWithBranch(repo.ID, "manual", targetBranch)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create job: "+err.Error())
		return
	}
	slog.Info("api: job queued", "job_id", job.ID, "repo_id", repo.ID, "branch", targetBranch)
	writeJSON(w, http.StatusAccepted, job)
}

func (h *Handler) handleGetJobLogs(w http.ResponseWriter, r *http.Request) {
	_, ok := h.authorizeRepo(w, r)
	if !ok {
		return
	}
	jobID, err := strconv.ParseInt(r.PathValue("jobID"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job id")
		return
	}
	logsJSON, err := h.DB.GetJobLogs(jobID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get logs: "+err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(logsJSON))
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
	repo, err := h.DB.GetRepoByID(job.RepoID)
	if err != nil || repo == nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	inst, err := h.DB.GetInstallationByID(repo.InstallationID)
	if err != nil || inst == nil || inst.TeamID != sessionFromCtx(r).TeamID {
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
	teamID := sessionFromCtx(r).TeamID
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
	teamID := sessionFromCtx(r).TeamID
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

// ─── Auth & User Profiles (GitHub OAuth) ─────────────────────────────────────
//
// Login is a full-page redirect flow, not a fetch() call: the browser is sent
// to GitHub, GitHub redirects back to our callback with a `code`, we exchange
// it server-side (client secret never touches the browser), then set a signed
// httpOnly session cookie and redirect into the app.

const oauthStateCookie = "langpeanut_oauth_state"

func (h *Handler) oauthRedirectURI() string {
	return strings.TrimRight(h.PublicBaseURL, "/") + "/api/auth/github/callback"
}

func (h *Handler) handleGitHubOAuthStart(w http.ResponseWriter, r *http.Request) {
	if h.OAuthClientID == "" {
		writeError(w, http.StatusServiceUnavailable, "GitHub OAuth not configured on server")
		return
	}
	state, err := auth.GenerateState(h.SessionSecret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "generate state: "+err.Error())
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    state,
		Path:     "/",
		MaxAge:   600, // 10 minutes to complete the OAuth round trip
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, auth.AuthorizeURL(h.OAuthClientID, h.oauthRedirectURI(), state), http.StatusFound)
}

func (h *Handler) handleGitHubOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if h.OAuthClientID == "" || h.OAuthClientSecret == "" {
		writeError(w, http.StatusServiceUnavailable, "GitHub OAuth not configured on server")
		return
	}

	stateParam := r.URL.Query().Get("state")
	stateCookie, err := r.Cookie(oauthStateCookie)
	if err != nil || stateParam == "" || stateCookie.Value != stateParam || !auth.VerifyState(h.SessionSecret, stateParam) {
		writeError(w, http.StatusBadRequest, "invalid or expired oauth state")
		return
	}
	// One-time use: clear the state cookie now that it's been consumed.
	http.SetCookie(w, &http.Cookie{Name: oauthStateCookie, Value: "", Path: "/", MaxAge: -1})

	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "missing code")
		return
	}

	userToken, err := auth.ExchangeCode(r.Context(), h.OAuthClientID, h.OAuthClientSecret, code, h.oauthRedirectURI())
	if err != nil {
		writeError(w, http.StatusBadGateway, "github oauth exchange: "+err.Error())
		return
	}
	ghUser, err := auth.FetchGitHubUser(r.Context(), userToken)
	if err != nil {
		writeError(w, http.StatusBadGateway, "github oauth profile: "+err.Error())
		return
	}

	// Each GitHub account gets its own team on first login — repos and
	// credentials are scoped per-team, not shared across unrelated users.
	existing, err := h.DB.GetUserByGithubID(ghUser.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup user: "+err.Error())
		return
	}
	var teamID int64
	if existing != nil {
		teamID = existing.TeamID
	} else {
		team, err := h.DB.CreateTeam(ghUser.Login)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "create team: "+err.Error())
			return
		}
		teamID = team.ID
	}

	user, err := h.DB.UpsertUserByGithubID(teamID, ghUser.ID, ghUser.Email, ghUser.Name, ghUser.Login, ghUser.AvatarURL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "upsert user: "+err.Error())
		return
	}

	sessionToken, err := auth.NewSessionToken(h.SessionSecret, user.ID, user.TeamID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create session: "+err.Error())
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    sessionToken,
		Path:     "/",
		MaxAge:   int(auth.SessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

func (h *Handler) handleGetMe(w http.ResponseWriter, r *http.Request) {
	sess := sessionFromCtx(r)
	user, err := h.DB.GetUserByID(sess.UserID)
	if err != nil || user == nil {
		writeError(w, http.StatusUnauthorized, "session user not found")
		return
	}
	team, err := h.DB.GetTeamByID(sess.TeamID)
	if err != nil || team == nil {
		writeError(w, http.StatusInternalServerError, "team not found")
		return
	}

	installs, _ := h.DB.ListInstallationsByTeam(sess.TeamID)
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
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

func isSecureRequest(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

// ─── Webhook (GitHub → langpeanut-cloud) ─────────────────────────────────────

// verifyWebhookSignature checks the X-Hub-Signature-256 header GitHub sends
// with every webhook delivery: HMAC-SHA256 of the raw request body, keyed by
// GITHUB_WEBHOOK_SECRET. Without this, anyone who discovers the webhook URL
// can forge push/PR-comment events and trigger jobs against any repo we
// track. Uses hmac.Equal for a constant-time comparison to avoid leaking the
// correct signature one byte at a time via response-time side channels.
func (h *Handler) verifyWebhookSignature(r *http.Request, body []byte) bool {
	if h.WebhookSecret == "" {
		// Refuse to run wide open if the operator forgot to configure a secret.
		return false
	}
	sigHeader := r.Header.Get("X-Hub-Signature-256")
	const prefix = "sha256="
	if !strings.HasPrefix(sigHeader, prefix) {
		return false
	}
	sig, err := hex.DecodeString(strings.TrimPrefix(sigHeader, prefix))
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(h.WebhookSecret))
	mac.Write(body)
	expected := mac.Sum(nil)
	return hmac.Equal(sig, expected)
}

func (h *Handler) handleWebhook(w http.ResponseWriter, r *http.Request) {
	event := r.Header.Get("X-GitHub-Event")
	slog.Info("webhook: received", "event", event)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "read body: "+err.Error())
		return
	}

	if !h.verifyWebhookSignature(r, body) {
		slog.Warn("webhook: signature verification failed", "event", event)
		writeError(w, http.StatusUnauthorized, "invalid webhook signature")
		return
	}

	switch event {
	case "push":
		var pushEv ghpkg.PushEvent
		if err := json.Unmarshal(body, &pushEv); err != nil {
			writeError(w, http.StatusBadRequest, "decode push: "+err.Error())
			return
		}

		branch := strings.TrimPrefix(pushEv.Ref, "refs/heads/")
		// Ignore deleted branches, tags, and internal langpeanut PR branches to prevent recursion loops
		if pushEv.Deleted || !strings.HasPrefix(pushEv.Ref, "refs/heads/") || strings.HasPrefix(branch, "langpeanut/") {
			writeJSON(w, http.StatusOK, map[string]string{"status": "ignored_internal_or_tag"})
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

		// Check if push autopilot is enabled
		if !settings.WebhookPushEnabled {
			slog.Info("webhook: push ignored because push autopilot is disabled in repo settings", "repo", repo.Owner+"/"+repo.Name, "branch", branch)
			writeJSON(w, http.StatusOK, map[string]string{
				"status": "push_trigger_disabled",
				"reason": "Push webhook autopilot is disabled in repository settings.",
			})
			return
		}

		// Check branch against settings.WebhookBranchFilter
		branchMatches := false
		switch settings.WebhookBranchFilter {
		case "all":
			branchMatches = true
		case "custom":
			if settings.WebhookCustomBranches != "" {
				patterns := strings.Split(settings.WebhookCustomBranches, ",")
				for _, pat := range patterns {
					pat = strings.TrimSpace(pat)
					if pat == "" {
						continue
					}
					if matched, _ := filepath.Match(pat, branch); matched || pat == branch {
						branchMatches = true
						break
					}
				}
			} else {
				branchMatches = (branch == repo.DefaultBranch || repo.DefaultBranch == "")
			}
		case "default_branch", "":
			fallthrough
		default:
			branchMatches = (branch == repo.DefaultBranch || repo.DefaultBranch == "")
		}

		if !branchMatches {
			slog.Info("webhook: push ignored due to branch filter", "repo", repo.Owner+"/"+repo.Name, "branch", branch, "filter", settings.WebhookBranchFilter, "custom", settings.WebhookCustomBranches)
			writeJSON(w, http.StatusOK, map[string]string{
				"status": "ignored_branch_filter",
				"branch": branch,
				"reason": fmt.Sprintf("Branch '%s' does not match repository branch filter '%s'", branch, settings.WebhookBranchFilter),
			})
			return
		}

		// Queue job with target branch for continuous autopilot workflow
		job, err := h.DB.CreateJobWithBranch(repo.ID, "webhook_push", branch)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "queue job: "+err.Error())
			return
		}
		slog.Info("webhook: queued push job for branch", "job_id", job.ID, "repo", repo.Owner+"/"+repo.Name, "branch", branch)
		writeJSON(w, http.StatusOK, map[string]any{"status": "job_queued", "job_id": job.ID, "branch": branch})
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

		settings, _ := h.DB.GetRepoSettings(repo.ID)
		if settings != nil && !settings.WebhookPRCommentsEnabled {
			slog.Info("webhook: PR comment ignored because PR comments are disabled in repo settings", "repo", repo.Owner+"/"+repo.Name)
			writeJSON(w, http.StatusOK, map[string]string{
				"status": "pr_comments_disabled",
				"reason": "PR bot comment commands (@langpeanut) are disabled in repository settings.",
			})
			return
		}

		targetBranch := repo.DefaultBranch
		inst, iErr := h.DB.GetInstallationByID(repo.InstallationID)
		if iErr == nil && inst != nil && len(h.PrivateKeyPEM) > 0 && h.AppID != "" {
			pk, pErr := ghpkg.ParsePrivateKeyPEM(h.PrivateKeyPEM)
			if pErr == nil && pk != nil {
				appCfg := ghpkg.AppConfig{AppID: h.AppID, PrivateKey: pk}
				tok, tErr := ghpkg.CreateInstallationToken(r.Context(), appCfg, inst.InstallationID)
				if tErr == nil && tok != nil && tok.Token != "" {
					prReq, _ := http.NewRequestWithContext(r.Context(), "GET",
						fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", repo.Owner, repo.Name, commentEv.Issue.Number), nil)
					if prReq != nil {
						prReq.Header.Set("Authorization", "Bearer "+tok.Token)
						prReq.Header.Set("Accept", "application/vnd.github+json")
						if prResp, err := http.DefaultClient.Do(prReq); err == nil && prResp.StatusCode == http.StatusOK {
							var prData struct {
								Head struct {
									Ref string `json:"ref"`
								} `json:"head"`
							}
							if json.NewDecoder(prResp.Body).Decode(&prData) == nil && prData.Head.Ref != "" {
								targetBranch = prData.Head.Ref
							}
							prResp.Body.Close()
						}
					}
				}
			}
		}

		if botCmd.Directive != "" {
			if s, err := h.DB.GetRepoSettings(repo.ID); err == nil && s != nil {
				s.UserDirective = botCmd.Directive
				_ = h.DB.UpsertRepoSettings(s)
			}
		}

		// Queue PR bot on-demand localization job
		job, err := h.DB.CreateJobWithBranch(repo.ID, "webhook_pr_comment", targetBranch)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "queue bot job: "+err.Error())
			return
		}
		slog.Info("webhook: queued @langpeanut PR bot job", "job_id", job.ID, "action", botCmd.Action, "pr", commentEv.Issue.Number, "branch", targetBranch)
		writeJSON(w, http.StatusOK, map[string]any{
			"status":     "bot_job_queued",
			"job_id":     job.ID,
			"bot_action": botCmd.Action,
			"pr_number":  commentEv.Issue.Number,
			"branch":     targetBranch,
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

func (h *Handler) handleTestWebhookPush(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepo(w, r)
	if !ok {
		return
	}

	var req struct {
		Branch string `json:"branch"`
		Commit string `json:"commit"`
		DryRun bool   `json:"dry_run"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	branch := req.Branch
	if branch == "" {
		branch = repo.DefaultBranch
	}
	if branch == "" {
		branch = "main"
	}

	settings, err := h.DB.GetRepoSettings(repo.ID)
	if err != nil || settings == nil || len(settings.Locales) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "no_locales_configured",
			"message": "Repository has no target localization languages configured.",
			"matched": false,
		})
		return
	}

	if !settings.WebhookPushEnabled {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "push_trigger_disabled",
			"message": "Push Autopilot is currently DISABLED in repository settings.",
			"matched": false,
		})
		return
	}

	branchMatches := false
	switch settings.WebhookBranchFilter {
	case "all":
		branchMatches = true
	case "custom":
		if settings.WebhookCustomBranches != "" {
			patterns := strings.Split(settings.WebhookCustomBranches, ",")
			for _, pat := range patterns {
				pat = strings.TrimSpace(pat)
				if pat == "" {
					continue
				}
				if matched, _ := filepath.Match(pat, branch); matched || pat == branch {
					branchMatches = true
					break
				}
			}
		} else {
			branchMatches = (branch == repo.DefaultBranch || repo.DefaultBranch == "")
		}
	case "default_branch", "":
		fallthrough
	default:
		branchMatches = (branch == repo.DefaultBranch || repo.DefaultBranch == "")
	}

	if !branchMatches {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "ignored_branch_filter",
			"message": fmt.Sprintf("Branch '%s' does NOT match branch filter '%s' (custom patterns: '%s')", branch, settings.WebhookBranchFilter, settings.WebhookCustomBranches),
			"matched": false,
			"branch":  branch,
		})
		return
	}

	if req.DryRun {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":         "simulated_match",
			"message":        fmt.Sprintf("✓ Webhook criteria MATCHED for branch '%s'. Autopilot job would be queued.", branch),
			"matched":        true,
			"branch":         branch,
			"action":         settings.WebhookAction,
			"branch_prefix":  settings.WebhookCustomBranchPrefix,
			"target_locales": settings.Locales,
		})
		return
	}

	job, err := h.DB.CreateJobWithBranch(repo.ID, "webhook_push", branch)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to queue job: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "job_queued",
		"message": fmt.Sprintf("Successfully simulated webhook push on branch '%s'. Job #%d queued.", branch, job.ID),
		"job_id":  job.ID,
		"branch":  branch,
		"matched": true,
	})
}

func (h *Handler) handleTestWebhookBot(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepo(w, r)
	if !ok {
		return
	}

	var req struct {
		Command string `json:"command"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Command == "" {
		writeError(w, http.StatusBadRequest, "command string is required")
		return
	}

	cmd, isBot := ghpkg.ParseBotCommand(req.Command)
	if !isBot || cmd == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"valid":   false,
			"message": "Not a valid @langpeanut or /langpeanut command. Example: '@langpeanut translate --locales ja,ko --tone formal'",
		})
		return
	}

	settings, _ := h.DB.GetRepoSettings(repo.ID)
	enabled := true
	if settings != nil {
		enabled = settings.WebhookPRCommentsEnabled
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"valid":       true,
		"enabled":     enabled,
		"action":      cmd.Action,
		"locales":     cmd.Locales,
		"tone":        cmd.Tone,
		"provider":    cmd.Provider,
		"directive":   cmd.Directive,
		"raw_command": cmd.RawCommand,
		"message":     fmt.Sprintf("✓ Parsed command action '%s' with %d locales and directive: '%s'", cmd.Action, len(cmd.Locales), cmd.Directive),
	})
}

// ─── Middleware ───────────────────────────────────────────────────────────────

// requireSession verifies the signed session cookie and injects the
// authenticated identity into the request context. Unlike the old
// X-Team-ID header (which any client could set to any value), this
// identity is derived from a token only the server could have issued.
func (h *Handler) requireSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(auth.SessionCookieName)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "not signed in")
			return
		}
		sess, err := auth.ParseSessionToken(h.SessionSecret, cookie.Value)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid session: "+err.Error())
			return
		}
		r = r.WithContext(contextWithSession(r.Context(), sess))
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

func (h *Handler) resolveClientForRepo(repo *db.Repo) llm.Client {
	return h.resolveClientForRepoWithOverride(repo, "", "")
}

func (h *Handler) resolveClientForRepoWithOverride(repo *db.Repo, overrideProvider, overrideModel string) llm.Client {
	settings, _ := h.DB.GetRepoSettings(repo.ID)
	provider := overrideProvider
	model := overrideModel

	if provider == "" && settings != nil {
		provider = settings.Provider
	}
	if provider == "" {
		provider = "gemini"
	}

	if model == "" && settings != nil {
		model = settings.Model
	}
	if model == "" {
		model = "gemini-3.7-flash"
	}

	var apiKey string
	var encKey []byte
	if settings != nil && len(settings.EncryptedAPIKeyOverride) > 0 && (settings.Provider == provider || overrideProvider == "") {
		encKey = settings.EncryptedAPIKeyOverride
	} else {
		inst, err := h.DB.GetInstallationByID(repo.InstallationID)
		if err == nil && inst != nil {
			cred, _ := h.DB.GetAPICredential(inst.TeamID, provider)
			if cred != nil {
				encKey = cred.EncryptedKey
			}
		}
	}
	if len(encKey) > 0 {
		apiKey, _ = auth.Decrypt(h.MasterKey, encKey)
	}

	if apiKey != "" {
		return llm.NewClientWithAPIKey(llm.ProviderType(provider), model, apiKey)
	}
	return llm.AutoDetectClient()
}

func (h *Handler) handleGetSEOOverview(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepo(w, r)
	if !ok {
		return
	}

	strategy, err := h.DB.GetSEOStrategy(repo.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get SEO strategy: "+err.Error())
		return
	}

	matrix, _ := h.DB.GetTranslationMatrix(repo.ID)
	enCount := 0
	if matrix != nil && matrix["en"] != nil {
		enCount = len(matrix["en"])
	}

	if strategy == nil {
		locales := []string{"ja", "de", "es"}
		if s, _ := h.DB.GetRepoSettings(repo.ID); s != nil && len(s.Locales) > 0 {
			locales = s.Locales
		}
		strategy = &db.RepoSEOStrategy{
			RepoID:             repo.ID,
			ProjectName:        repo.Name,
			Category:           "",
			ProductDescription: "",
			TargetLocales:      locales,
			Goal:               "traffic",
			ScopeTier:          "high_impact",
			CompetitorURLs:     []string{},
		}
	}

	comps, _ := h.DB.GetSEOCompetitors(repo.ID, "")
	kws, _ := h.DB.GetSEOKeywords(repo.ID, "")
	opts, _ := h.DB.GetSEOOptimizations(repo.ID, "")

	// Group by locale
	compMap := make(map[string][]db.RepoSEOCompetitor)
	for _, c := range comps {
		compMap[c.Locale] = append(compMap[c.Locale], c)
	}
	kwMap := make(map[string][]db.RepoSEOKeyword)
	for _, k := range kws {
		kwMap[k.Locale] = append(kwMap[k.Locale], k)
	}
	optMap := make(map[string][]db.RepoSEOOptimization)
	for _, o := range opts {
		optMap[o.Locale] = append(optMap[o.Locale], o)
	}

	metricsMap := make(map[string]*db.RepoSEOMetrics)
	simAgent := seo.NewSERPSimulatorAgent()
	simMap := make(map[string]*seo.SERPSimulation)
	for _, loc := range strategy.TargetLocales {
		if m, err := h.DB.GetSEOMetrics(repo.ID, loc); err == nil && m != nil {
			metricsMap[loc] = m
		}
		if oList, ok := optMap[loc]; ok && len(oList) > 0 {
			coreOpts := make([]seo.KeyOptimization, 0, len(oList))
			for _, o := range oList {
				coreOpts = append(coreOpts, seo.KeyOptimization{
					Key:                  o.TranslationKey,
					SourceEn:             o.SourceEn,
					BaselineTranslation:  o.BaselineTranslation,
					OptimizedTranslation: o.OptimizedTranslation,
					InjectedKeywords:     o.InjectedKeywords,
					Rationale:            o.Rationale,
					ImpactTier:           o.ImpactTier,
					CharacterLength:      o.CharacterLength,
					PixelWidthDesktop:    o.PixelWidthDesktop,
					IsTitleTruncated:     o.IsTitleTruncated,
					ICUVariablesMatched:  o.ICUVariablesMatched,
				})
			}
			kwList := kwMap[loc]
			coreKws := make([]seo.KeywordInsight, 0, len(kwList))
			for _, k := range kwList {
				coreKws = append(coreKws, seo.KeywordInsight{
					Keyword:          k.Keyword,
					Locale:           k.Locale,
					Intent:           k.Intent,
					VolumeTier:       k.VolumeTier,
					EstMonthlyVolume: k.EstMonthlyVolume,
					Difficulty:       k.Difficulty,
					Relevance:        k.Relevance,
					IsPrimary:        k.IsPrimary,
					IsLocked:         k.IsLocked,
				})
			}
			coreStrat := &seo.SEOStrategy{
				ProjectName:        strategy.ProjectName,
				Category:           strategy.Category,
				ProductDescription: strategy.ProductDescription,
				TargetLocales:      strategy.TargetLocales,
				Goal:               seo.GrowthGoal(strategy.Goal),
				ScopeTier:          seo.KeyScopeTier(strategy.ScopeTier),
				CompetitorURLs:     strategy.CompetitorURLs,
			}
			sim := simAgent.GenerateSimulation(coreStrat, loc, coreKws, coreOpts)
			simMap[loc] = sim
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"strategy":             strategy,
		"competitors":          compMap,
		"keywords":             kwMap,
		"optimizations":        optMap,
		"metrics":              metricsMap,
		"simulations":          simMap,
		"extracted_keys_count": enCount,
	})
}

// handleAnalyzeSEODomain executes AI Domain Analysis on the real extracted UI strings from the codebase
func (h *Handler) handleAnalyzeSEODomain(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepo(w, r)
	if !ok {
		return
	}

	// 1. Gather real extracted UI strings (or perform instant AST scan if needed)
	extractedStrings, _ := h.ensureSourceStringsExtracted(r.Context(), repo)
	if len(extractedStrings) == 0 {
		writeError(w, http.StatusBadRequest, "No UI strings found in repository. Please run an initial job or ensure codebase files contain extractable UI strings.")
		return
	}

	// 2. Resolve LLM client for this repository
	client := h.resolveClientForRepo(repo)
	if client == nil {
		writeError(w, http.StatusBadRequest, "Please configure an AI provider API key (Gemini, OpenAI, Anthropic, or Groq) in Repository Settings.")
		return
	}

	strategy, _ := h.DB.GetSEOStrategy(repo.ID)
	if strategy == nil {
		locales := []string{"ja", "de", "es"}
		if s, _ := h.DB.GetRepoSettings(repo.ID); s != nil && len(s.Locales) > 0 {
			locales = s.Locales
		}
		strategy = &db.RepoSEOStrategy{
			RepoID:         repo.ID,
			ProjectName:    repo.Name,
			TargetLocales:  locales,
			Goal:           "traffic",
			ScopeTier:      "high_impact",
			CompetitorURLs: []string{},
		}
	}

	// 3. AI Agent analyzes real UI strings
	cat, desc := seo.InferSoftwareOverview(r.Context(), client, repo.Name, extractedStrings, "", "")
	if cat == "" {
		writeError(w, http.StatusInternalServerError, "AI Agent could not determine software domain. Please verify your API key or UI strings.")
		return
	}

	strategy.Category = cat
	strategy.ProductDescription = desc
	if err := h.DB.UpsertSEOStrategy(strategy); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save SEO strategy: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"category":             cat,
		"product_description": desc,
		"extracted_keys_count": len(extractedStrings),
		"strategy":             strategy,
	})
}

// ensureSourceStringsExtracted guarantees that English UI strings are available in the translation matrix.
// If the DB matrix is empty, it runs an instant in-memory AST scan on the repository files and persists them.
func (h *Handler) ensureSourceStringsExtracted(ctx context.Context, repo *db.Repo) ([]string, map[string]map[string]string) {
	matrix, _ := h.DB.GetTranslationMatrix(repo.ID)
	if matrix == nil {
		matrix = make(map[string]map[string]string)
	}

	var extracted []string
	if enMap, ok := matrix["en"]; ok && len(enMap) > 0 {
		for _, v := range enMap {
			if v != "" {
				extracted = append(extracted, v)
			}
		}
		return extracted, matrix
	}

	// Matrix is empty: perform instant AST scan on disk
	scanDir := h.getRepoScanDir(repo.ID)
	if _, err := os.Stat(scanDir); err == nil {
		reg := platforms.NewRegistry()
		plat, _ := reg.AutoDetect(scanDir)
		if plat != nil {
			scout := agents.NewASTScoutAgent(plat)
			report, err := scout.ScanProject(scanDir, "")
			if err == nil && len(report.Candidates) > 0 {
				if matrix["en"] == nil {
					matrix["en"] = make(map[string]string)
				}
				for _, c := range report.Candidates {
					cleanVal := strings.TrimSpace(c.CleanValue)
					if cleanVal != "" {
						matrix["en"][c.Key] = cleanVal
						extracted = append(extracted, cleanVal)
					}
				}
				_ = h.DB.UpsertTranslationMatrix(repo.ID, matrix)
				slog.Info("seo: auto-extracted AST strings on first run", "repo_id", repo.ID, "count", len(extracted))
			}
		}
	}

	return extracted, matrix
}

func (h *Handler) handleSaveSEOStrategy(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepo(w, r)
	if !ok {
		return
	}

	var req db.RepoSEOStrategy
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request payload: "+err.Error())
		return
	}
	req.RepoID = repo.ID
	if len(req.TargetLocales) == 0 {
		req.TargetLocales = []string{"ja", "de", "es"}
	}
	if req.Goal == "" {
		req.Goal = "traffic"
	}
	if req.ScopeTier == "" {
		req.ScopeTier = "high_impact"
	}

	if err := h.DB.UpsertSEOStrategy(&req); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save SEO strategy: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, req)
}

func (h *Handler) handleRunSEOScout(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepo(w, r)
	if !ok {
		return
	}

	var req struct {
		Locale string `json:"locale"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	locale := req.Locale
	if locale == "" {
		locale = "en"
	}

	strategy, err := h.DB.GetSEOStrategy(repo.ID)
	if err != nil || strategy == nil {
		writeError(w, http.StatusBadRequest, "please configure SEO strategy first")
		return
	}

	client := h.resolveClientForRepo(repo)
	scoutAgent := seo.NewSERPScoutAgent(client)
	kwAgent := seo.NewKeywordIntelligenceAgent(client)

	// Collect extracted UI strings (auto-extracting via AST if matrix is empty)
	extractedStrings, _ := h.ensureSourceStringsExtracted(r.Context(), repo)

	currentCat := strategy.Category
	if currentCat == "Software Platform" || currentCat == "E-Commerce & Storefront App" || currentCat == "Cloud Productivity Software" {
		currentCat = ""
	}
	cat, desc := seo.InferSoftwareOverview(r.Context(), client, repo.Name, extractedStrings, currentCat, strategy.ProductDescription)
	if cat != "" {
		strategy.Category = cat
		strategy.ProductDescription = desc
		_ = h.DB.UpsertSEOStrategy(strategy)
	}

	coreStrategy := &seo.SEOStrategy{
		ProjectName:        strategy.ProjectName,
		Category:           cat,
		ProductDescription: desc,
		TargetLocales:      strategy.TargetLocales,
		Goal:               seo.GrowthGoal(strategy.Goal),
		ScopeTier:          seo.KeyScopeTier(strategy.ScopeTier),
		CompetitorURLs:     strategy.CompetitorURLs,
	}

	comps, err := scoutAgent.ScoutLocale(r.Context(), coreStrategy, locale)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "scouting failed: "+err.Error())
		return
	}

	dbComps := make([]db.RepoSEOCompetitor, 0, len(comps))
	for _, c := range comps {
		dbComps = append(dbComps, db.RepoSEOCompetitor{
			RepoID:          repo.ID,
			Locale:          locale,
			Domain:          c.Domain,
			Rank:            c.Rank,
			URL:             c.URL,
			Title:           c.Title,
			MetaDescription: c.MetaDescription,
			H1s:             c.H1s,
			H2s:             c.H2s,
			Keywords:        c.Keywords,
			ValueProps:      c.ValueProps,
			IsDiscovered:    c.IsDiscovered,
		})
	}
	_ = h.DB.UpsertSEOCompetitors(repo.ID, locale, dbComps)

	kws, _ := kwAgent.AnalyzeKeywords(r.Context(), coreStrategy, locale, comps)
	dbKws := make([]db.RepoSEOKeyword, 0, len(kws))
	for _, k := range kws {
		dbKws = append(dbKws, db.RepoSEOKeyword{
			RepoID:           repo.ID,
			Locale:           locale,
			Keyword:          k.Keyword,
			Intent:           k.Intent,
			VolumeTier:       k.VolumeTier,
			EstMonthlyVolume: k.EstMonthlyVolume,
			Difficulty:       k.Difficulty,
			Relevance:        k.Relevance,
			IsPrimary:        k.IsPrimary,
			IsLocked:         k.IsLocked,
		})
	}
	_ = h.DB.UpsertSEOKeywords(repo.ID, locale, dbKws)

	writeJSON(w, http.StatusOK, map[string]any{
		"locale":      locale,
		"competitors": dbComps,
		"keywords":    dbKws,
	})
}

func (h *Handler) handleRunSEOOptimize(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepo(w, r)
	if !ok {
		return
	}

	var req struct {
		Locale string `json:"locale"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	locale := req.Locale
	if locale == "" {
		locale = "en"
	}

	strategy, err := h.DB.GetSEOStrategy(repo.ID)
	if err != nil || strategy == nil {
		writeError(w, http.StatusBadRequest, "please configure SEO strategy first")
		return
	}

	client := h.resolveClientForRepo(repo)
	weaverAgent := seo.NewSemanticCopyWeaverAgent(client)
	criticAgent := seo.NewGrowthPredictorCritic()
	simAgent := seo.NewSERPSimulatorAgent()

	extractedStrings, matrix := h.ensureSourceStringsExtracted(r.Context(), repo)
	sourceKeys := make(map[string]string)
	baselineTranslations := make(map[string]string)

	if matrix["en"] != nil {
		for k, v := range matrix["en"] {
			sourceKeys[k] = v
		}
	}
	if matrix[locale] != nil {
		for k, v := range matrix[locale] {
			baselineTranslations[k] = v
			if sourceKeys[k] == "" {
				sourceKeys[k] = v
			}
		}
	}

	cat, desc := seo.InferSoftwareOverview(r.Context(), client, repo.Name, extractedStrings, strategy.Category, strategy.ProductDescription)
	if cat != "" {
		strategy.Category = cat
		strategy.ProductDescription = desc
	}

	coreStrategy := &seo.SEOStrategy{
		ProjectName:        strategy.ProjectName,
		Category:           cat,
		ProductDescription: desc,
		TargetLocales:      strategy.TargetLocales,
		Goal:               seo.GrowthGoal(strategy.Goal),
		ScopeTier:          seo.KeyScopeTier(strategy.ScopeTier),
		CompetitorURLs:     strategy.CompetitorURLs,
	}

	if len(sourceKeys) == 0 {
		// Scan directory if matrix is empty
		scanDir := h.getRepoScanDir(repo.ID)
		reg := platforms.NewRegistry()
		plat, _ := reg.AutoDetect(scanDir)
		if plat != nil {
			sourceKeys = seo.ExtractLocaleCatalog(scanDir, plat, "en")
			baselineTranslations = seo.ExtractLocaleCatalog(scanDir, plat, locale)
		}
	}

	if len(sourceKeys) == 0 {
		sourceKeys = map[string]string{
			"home.hero.title": fmt.Sprintf("The fastest platform for %s", strategy.Category),
			"home.hero.desc":  strategy.ProductDescription,
			"cta.button":      "Get Started Free",
		}
	}

	// Get keywords
	dbKws, _ := h.DB.GetSEOKeywords(repo.ID, locale)
	kwInsights := make([]seo.KeywordInsight, 0, len(dbKws))
	for _, k := range dbKws {
		kwInsights = append(kwInsights, seo.KeywordInsight{
			Keyword:          k.Keyword,
			Locale:           k.Locale,
			Intent:           k.Intent,
			VolumeTier:       k.VolumeTier,
			EstMonthlyVolume: k.EstMonthlyVolume,
			Difficulty:       k.Difficulty,
			Relevance:        k.Relevance,
			IsPrimary:        k.IsPrimary,
			IsLocked:         k.IsLocked,
		})
	}

	opts, err := weaverAgent.WeaveTranslations(r.Context(), coreStrategy, locale, sourceKeys, baselineTranslations, kwInsights)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "optimization failed: "+err.Error())
		return
	}

	dbOpts := make([]db.RepoSEOOptimization, 0, len(opts))
	for _, o := range opts {
		dbOpts = append(dbOpts, db.RepoSEOOptimization{
			RepoID:               repo.ID,
			Locale:               locale,
			TranslationKey:       o.Key,
			SourceEn:             o.SourceEn,
			BaselineTranslation:  o.BaselineTranslation,
			OptimizedTranslation: o.OptimizedTranslation,
			InjectedKeywords:     o.InjectedKeywords,
			Rationale:            o.Rationale,
			ImpactTier:           o.ImpactTier,
			CharacterLength:      o.CharacterLength,
			PixelWidthDesktop:    o.PixelWidthDesktop,
			IsTitleTruncated:     o.IsTitleTruncated,
			ICUVariablesMatched:  o.ICUVariablesMatched,
		})
	}
	_ = h.DB.UpsertSEOOptimizations(repo.ID, locale, dbOpts)

	metrics := criticAgent.EvaluateGrowth(coreStrategy, locale, kwInsights, opts)
	dbMetrics := &db.RepoSEOMetrics{
		RepoID:                repo.ID,
		Locale:                locale,
		SearchVolumeBaseline:  metrics.SearchVolumeBaseline,
		SearchVolumeOptimized: metrics.SearchVolumeOptimized,
		SearchVolumeUpliftPct: metrics.SearchVolumeUpliftPct,
		ProjectedCTRBaseline:  metrics.ProjectedCTRBaseline,
		ProjectedCTROptimized: metrics.ProjectedCTROptimized,
		ProjectedCTRUpliftPct: metrics.ProjectedCTRUpliftPct,
		AvgKeywordDifficulty:  metrics.AvgKeywordDifficulty,
		ReadabilityScore:      metrics.ReadabilityScore,
		LocalTrustScore:       metrics.LocalTrustScore,
		KeywordDensityPct:     metrics.KeywordDensityPct,
		DensitySafe:           metrics.DensitySafe,
		EstimatedRankingDays:  metrics.EstimatedRankingDays,
	}
	_ = h.DB.UpsertSEOMetrics(dbMetrics)

	sim := simAgent.GenerateSimulation(coreStrategy, locale, kwInsights, opts)

	writeJSON(w, http.StatusOK, map[string]any{
		"locale":        locale,
		"optimizations": dbOpts,
		"metrics":       dbMetrics,
		"simulation":    sim,
	})
}

func (h *Handler) handleApplySEOToMatrix(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepo(w, r)
	if !ok {
		return
	}

	var req struct {
		Locale string `json:"locale"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	opts, err := h.DB.GetSEOOptimizations(repo.ID, req.Locale)
	if err != nil || len(opts) == 0 {
		writeError(w, http.StatusBadRequest, "no SEO optimizations found to apply")
		return
	}

	appliedCount := 0
	for _, o := range opts {
		if o.OptimizedTranslation != "" {
			_ = h.DB.UpdateTranslationCell(repo.ID, o.Locale, o.TranslationKey, o.OptimizedTranslation)
			appliedCount++
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":        "applied",
		"applied_count": appliedCount,
		"locale":        req.Locale,
	})
}

// ─── Google Genkit Handlers & Repository Copilot ─────────────────────────────

func (h *Handler) handleGenkitRuntime(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepo(w, r)
	if !ok {
		return
	}

	client := h.resolveClientForRepo(repo)
	scanDir := h.getRepoScanDir(repo.ID)
	engine, err := genkit.NewGenkitEngine(scanDir, client)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to initialize genkit runtime: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, engine.GetRuntimeInfo())
}

func (h *Handler) handleGenkitFlows(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepo(w, r)
	if !ok {
		return
	}

	client := h.resolveClientForRepo(repo)
	scanDir := h.getRepoScanDir(repo.ID)
	engine, err := genkit.NewGenkitEngine(scanDir, client)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to initialize genkit: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"framework": "Google Genkit Go",
		"version":   "v1.12.0",
		"flows":     engine.ListFlows(),
		"tools":     engine.ListTools(),
	})
}

func (h *Handler) handleGenkitFlowRun(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepo(w, r)
	if !ok {
		return
	}

	flowName := r.PathValue("flowName")
	if flowName == "" {
		writeError(w, http.StatusBadRequest, "flowName is required")
		return
	}

	var input map[string]any
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		input = make(map[string]any)
	}

	client := h.resolveClientForRepo(repo)
	scanDir := h.getRepoScanDir(repo.ID)
	engine, err := genkit.NewGenkitEngine(scanDir, client)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to initialize genkit: "+err.Error())
		return
	}

	matrix, _ := h.DB.GetTranslationMatrix(repo.ID)
	if enMap, ok := matrix["en"]; ok {
		for k, v := range enMap {
			engine.UnderlyingEngine.Candidates = append(engine.UnderlyingEngine.Candidates, types.StringCandidate{
				Key:        k,
				RawValue:   v,
				CleanValue: v,
			})
		}
	}

	res, err := engine.RunFlow(r.Context(), flowName, input, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "genkit flow execution failed: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"flow":   flowName,
		"status": "completed",
		"result": res,
	})
}

func (h *Handler) handleRepoChat(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepo(w, r)
	if !ok {
		return
	}

	var req struct {
		Message  string `json:"message"`
		Provider string `json:"provider"`
		Model    string `json:"model"`
		History  []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"history"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Message) == "" {
		writeError(w, http.StatusBadRequest, "message is required")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Genkit-Framework", "Google Genkit Go")
	w.Header().Set("X-Genkit-Version", "v1.12.0")

	client := h.resolveClientForRepoWithOverride(repo, req.Provider, req.Model)
	scanDir := h.getRepoScanDir(repo.ID)
	engine, err := genkit.NewGenkitEngine(scanDir, client)
	if err != nil {
		fmt.Fprintf(w, "data: %s\n\n", `{"type":"error","error":"failed to init genkit engine"}`)
		flusher.Flush()
		return
	}

	repoMeta := map[string]any{
		"repo_id":        repo.ID,
		"owner":          repo.Owner,
		"name":           repo.Name,
		"full_name":      fmt.Sprintf("%s/%s", repo.Owner, repo.Name),
		"default_branch": repo.DefaultBranch,
	}
	var missingConfig []string

	if settings, _ := h.DB.GetRepoSettings(repo.ID); settings != nil {
		if len(settings.Locales) > 0 {
			engine.UnderlyingEngine.TargetLocales = settings.Locales
		} else {
			missingConfig = append(missingConfig, "Target locales not configured (currently using defaults).")
		}
		if settings.TonePreset != "" {
			engine.UnderlyingEngine.ToneStyle = settings.TonePreset
		}
		repoMeta["provider"] = settings.Provider
		repoMeta["model"] = settings.Model
		repoMeta["tone"] = settings.TonePreset
		repoMeta["root_dir"] = settings.RootDir
		repoMeta["user_directive"] = settings.UserDirective
		repoMeta["custom_build_cmd"] = settings.CustomBuildCmd
		repoMeta["custom_install_cmd"] = settings.CustomInstallCmd
		repoMeta["existing_translations_mode"] = settings.ExistingTranslationsMode

		// Check API key configuration
		hasKey := false
		if len(settings.EncryptedAPIKeyOverride) > 0 {
			hasKey = true
		} else {
			inst, _ := h.DB.GetInstallationByID(repo.InstallationID)
			if inst != nil {
				cred, _ := h.DB.GetAPICredential(inst.TeamID, settings.Provider)
				if cred != nil {
					hasKey = true
				}
			}
		}
		repoMeta["has_api_key"] = hasKey
		if !hasKey {
			missingConfig = append(missingConfig, fmt.Sprintf("No API key configured for active provider '%s'. Add key in Vault Keys or Repo Settings before running automated background jobs.", settings.Provider))
		}
	} else {
		missingConfig = append(missingConfig, "Repository settings not initialized.")
	}

	// Translation matrix stats
	matrix, _ := h.DB.GetTranslationMatrix(repo.ID)
	if enMap, ok := matrix["en"]; ok {
		repoMeta["extracted_strings_count"] = len(enMap)
		localeStats := make(map[string]int)
		for loc, kv := range matrix {
			localeStats[loc] = len(kv)
		}
		repoMeta["locale_key_counts"] = localeStats
	} else {
		missingConfig = append(missingConfig, "No extracted translation strings in database yet. Run an AST scan or trigger a job to extract strings.")
	}

	// Recent jobs status
	recentJobs, _ := h.DB.ListJobsByRepo(repo.ID, 1)
	if len(recentJobs) > 0 {
		lastJob := recentJobs[0]
		repoMeta["last_job_id"] = lastJob.ID
		repoMeta["last_job_status"] = lastJob.Status
		repoMeta["last_job_branch"] = lastJob.Branch
		if lastJob.PRURL != "" {
			repoMeta["last_job_pr_url"] = lastJob.PRURL
		}
		if lastJob.ErrorMessage != "" {
			repoMeta["last_job_error"] = lastJob.ErrorMessage
		}
	}

	engine.UnderlyingEngine.RepoMetadata = repoMeta
	engine.UnderlyingEngine.MissingConfig = missingConfig

	// Wire Platform Job Trigger Hook
	engine.UnderlyingEngine.JobTriggerHook = func(ctx context.Context, branch, directive string) (map[string]any, error) {
		targetBranch := strings.TrimSpace(branch)
		if targetBranch == "" {
			targetBranch = repo.DefaultBranch
		}
		if targetBranch == "" {
			targetBranch = "main"
		}

		if directive != "" {
			if s, _ := h.DB.GetRepoSettings(repo.ID); s != nil {
				s.UserDirective = directive
				_ = h.DB.UpsertRepoSettings(s)
			}
		}

		job, err := h.DB.CreateJobWithBranch(repo.ID, "manual", targetBranch)
		if err != nil {
			return nil, fmt.Errorf("create job: %w", err)
		}
		slog.Info("api: job queued via central copilot", "job_id", job.ID, "repo_id", repo.ID, "branch", targetBranch)

		return map[string]any{
			"job_id":  job.ID,
			"repo":    fmt.Sprintf("%s/%s", repo.Owner, repo.Name),
			"branch":  targetBranch,
			"status":  "queued",
			"message": fmt.Sprintf("Platform Job #%d queued for %s on branch '%s'.", job.ID, repo.Name, targetBranch),
		}, nil
	}

	// Wire Platform Config Update Hook
	engine.UnderlyingEngine.ConfigUpdateHook = func(ctx context.Context, updates map[string]any) (map[string]any, error) {
		s, err := h.DB.GetRepoSettings(repo.ID)
		if err != nil || s == nil {
			return nil, fmt.Errorf("settings not found")
		}
		if locs, ok := updates["locales"].([]string); ok && len(locs) > 0 {
			s.Locales = locs
		} else if locSlice, ok := updates["locales"].([]any); ok && len(locSlice) > 0 {
			var parsed []string
			for _, item := range locSlice {
				if str, ok := item.(string); ok && str != "" {
					parsed = append(parsed, str)
				}
			}
			if len(parsed) > 0 {
				s.Locales = parsed
			}
		}
		if tone, ok := updates["tone"].(string); ok && tone != "" {
			s.TonePreset = tone
		}
		if model, ok := updates["model"].(string); ok && model != "" {
			s.Model = model
		}
		if provider, ok := updates["provider"].(string); ok && provider != "" {
			s.Provider = provider
		}
		if dir, ok := updates["user_directive"].(string); ok {
			s.UserDirective = dir
		}
		if err := h.DB.UpsertRepoSettings(s); err != nil {
			return nil, err
		}
		return map[string]any{
			"status":   "updated",
			"settings": s,
		}, nil
	}

	// Wire Platform Matrix Update Hook
	engine.UnderlyingEngine.MatrixUpdateHook = func(ctx context.Context, trans map[string]map[string]string) error {
		matrix, _ := h.DB.GetTranslationMatrix(repo.ID)
		if matrix == nil {
			matrix = make(map[string]map[string]string)
		}
		for loc, kv := range trans {
			if matrix[loc] == nil {
				matrix[loc] = make(map[string]string)
			}
			for k, v := range kv {
				matrix[loc][k] = v
			}
		}
		return h.DB.UpsertTranslationMatrix(repo.ID, matrix)
	}

	// Wire Platform Jobs Query Hook
	engine.UnderlyingEngine.JobsQueryHook = func(ctx context.Context, limit int) ([]map[string]any, error) {
		jobs, err := h.DB.ListJobsByRepo(repo.ID, limit)
		if err != nil {
			return nil, err
		}
		var list []map[string]any
		for _, j := range jobs {
			list = append(list, map[string]any{
				"id":           j.ID,
				"status":       j.Status,
				"trigger_type": j.TriggerType,
				"branch":       j.Branch,
				"commit_sha":   j.HeadCommitSHA,
				"pr_url":       j.PRURL,
				"created_at":   j.CreatedAt,
			})
		}
		return list, nil
	}

	// Wire Platform Key Update Hook
	engine.UnderlyingEngine.KeyUpdateHook = func(ctx context.Context, locale, key, value string) error {
		matrix, _ := h.DB.GetTranslationMatrix(repo.ID)
		if matrix == nil {
			matrix = make(map[string]map[string]string)
		}
		if matrix[locale] == nil {
			matrix[locale] = make(map[string]string)
		}
		matrix[locale][key] = value
		return h.DB.UpsertTranslationMatrix(repo.ID, matrix)
	}

	if len(req.History) > 0 {
		for _, hMsg := range req.History {
			if strings.TrimSpace(hMsg.Content) == "" {
				continue
			}
			role := chat.RoleUser
			if hMsg.Role == "assistant" || hMsg.Role == "model" {
				role = chat.RoleAssistant
			}
			engine.UnderlyingEngine.History = append(engine.UnderlyingEngine.History, chat.ChatMessage{
				Role:    role,
				Content: hMsg.Content,
			})
		}
	}

	// Populate engine with database translation matrix candidates
	if enMap, ok := matrix["en"]; ok {
		for k, v := range enMap {
			engine.UnderlyingEngine.Candidates = append(engine.UnderlyingEngine.Candidates, types.StringCandidate{
				Key:        k,
				RawValue:   v,
				CleanValue: v,
			})
		}
	}

	streamChan := make(chan genkit.GenkitStreamEvent, 100)
	doneChan := make(chan struct{})

	go func() {
		defer close(doneChan)
		_, _ = engine.SendChatMessage(r.Context(), req.Message, streamChan)
	}()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-doneChan:
			for len(streamChan) > 0 {
				ev := <-streamChan
				data, _ := json.Marshal(ev)
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			}
			return
		case ev := <-streamChan:
			data, _ := json.Marshal(ev)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

