// Package api contains all HTTP handlers for langpeanut-cloud.
// Routes are registered in RegisterRoutes(); the caller (cmd/server/main.go)
// owns the http.Server lifecycle.
//
// All handlers return JSON. Error responses follow the shape:
//   {"error": "human-readable message"}
package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/langPeanut/langpeanut-cloud/internal/auth"
	"github.com/langPeanut/langpeanut-cloud/internal/db"
	"github.com/langPeanut/langPeanut/pkg/agents"
	"github.com/langPeanut/langPeanut/pkg/chat"
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
	mux.HandleFunc("GET /api/repos/{repoID}/matrix", h.requireSession(h.handleGetMatrix))
	mux.HandleFunc("PUT /api/repos/{repoID}/matrix", h.requireSession(h.handleUpdateMatrixCell))
	mux.HandleFunc("POST /api/repos/{repoID}/matrix/copilot", h.requireSession(h.handleMatrixCopilot))
	mux.HandleFunc("GET /api/repos/{repoID}/branches", h.requireSession(h.handleListBranches))

	// ── SEO & Market Growth Studio ────────────────────────────────────────────
	mux.HandleFunc("GET /api/repos/{repoID}/seo", h.requireSession(h.handleGetSEOOverview))
	mux.HandleFunc("POST /api/repos/{repoID}/seo/strategy", h.requireSession(h.handleSaveSEOStrategy))
	mux.HandleFunc("POST /api/repos/{repoID}/seo/scout", h.requireSession(h.handleRunSEOScout))
	mux.HandleFunc("POST /api/repos/{repoID}/seo/optimize", h.requireSession(h.handleRunSEOOptimize))
	mux.HandleFunc("POST /api/repos/{repoID}/seo/apply", h.requireSession(h.handleApplySEOToMatrix))

	// ── Agentic Capabilities: Doctor, Persona, Pruner & Central Copilot ─────
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

	teamID := sessionFromCtx(r).TeamID

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

	// Remove mirror cache if exists
	mirrorPath := filepath.Join("data", "mirrors", fmt.Sprintf("%d.git", repo.ID))
	_ = os.RemoveAll(mirrorPath)

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

	// Clean up mirror bare git repo
	mirrorPath := filepath.Join("data", "mirrors", fmt.Sprintf("%d.git", repo.ID))
	_ = os.RemoveAll(mirrorPath)

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
		"repo_id":                    s.RepoID,
		"locales":                    s.Locales,
		"tone_preset":                s.TonePreset,
		"provider":                   s.Provider,
		"model":                      s.Model,
		"safety_mode":                s.SafetyMode,
		"chunk_word_budget":          s.ChunkWordBudget,
		"chunk_key_ceiling":          s.ChunkKeyCeiling,
		"custom_install_cmd":         s.CustomInstallCmd,
		"custom_build_cmd":           s.CustomBuildCmd,
		"root_dir":                   s.RootDir,
		"existing_translations_mode": s.ExistingTranslationsMode,
		"user_directive":            s.UserDirective,
		"has_api_key_override":       len(s.EncryptedAPIKeyOverride) > 0,
		"updated_at":                 s.UpdatedAt,
	}
	writeJSON(w, http.StatusOK, resp)
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
	UserDirective            string   `json:"user_directive,omitempty"`
	APIKeyOverride           string   `json:"api_key_override,omitempty"`
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
		req.Model = "gemini-3.5-flash"
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

	s := &db.RepoSettings{
		RepoID:                   repo.ID,
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
		EncryptedAPIKeyOverride:  encryptedOverride,
		UserDirective:            req.UserDirective,
	}
	if err := h.DB.UpsertRepoSettings(s); err != nil {
		writeError(w, http.StatusInternalServerError, "upsert settings: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
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
		dataDir = "data"
	}
	// Check jobs directory for latest unpacked working copy
	jobsDir := filepath.Join(dataDir, "jobs")
	entries, err := os.ReadDir(jobsDir)
	if err == nil {
		for i := len(entries) - 1; i >= 0; i-- {
			e := entries[i]
			if e.IsDir() {
				targetRepoDir := filepath.Join(jobsDir, e.Name(), "repo")
				if _, err := os.Stat(targetRepoDir); err == nil {
					return targetRepoDir
				}
			}
		}
	}
	return "."
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
	settings, err := h.DB.GetRepoSettings(repo.ID)
	if err != nil || settings == nil {
		return llm.AutoDetectClient()
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

	if apiKey != "" {
		return llm.NewClientWithAPIKey(llm.ProviderType(settings.Provider), settings.Model, apiKey)
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

	// Auto-infer if empty
	if strategy == nil {
		scanDir := h.getRepoScanDir(repo.ID)
		client := h.resolveClientForRepo(repo)
		scout := agents.NewPersonaScoutAgent(client)
		persona, _ := scout.DiscoverPersona(scanDir)

		projName := repo.Name
		cat := "Software Platform"
		locales := []string{"ja", "de", "es"}
		if persona != nil {
			if persona.ProjectName != "" {
				projName = persona.ProjectName
			}
			if persona.Audience != "" && len(persona.Audience) <= 25 {
				cat = persona.Audience
			} else if persona.Summary != "" && len(persona.Summary) <= 30 && !strings.HasPrefix(persona.Summary, "Autonomous localization") {
				cat = persona.Summary
			} else if strings.Contains(strings.ToLower(projName), "store") || strings.Contains(strings.ToLower(projName), "shop") || strings.Contains(strings.ToLower(projName), "commerce") {
				cat = "E-Commerce Platform"
			} else if strings.Contains(strings.ToLower(projName), "app") {
				cat = "Application"
			}
			if len(persona.LocalesSuggested) > 0 {
				locales = persona.LocalesSuggested
			}
		}

		strategy = &db.RepoSEOStrategy{
			RepoID:             repo.ID,
			ProjectName:        projName,
			Category:           cat,
			ProductDescription: fmt.Sprintf("Modern software platform: %s", projName),
			TargetLocales:      locales,
			Goal:               "traffic",
			ScopeTier:          "high_impact",
			CompetitorURLs:     []string{},
		}
		_ = h.DB.UpsertSEOStrategy(strategy)
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
		"strategy":      strategy,
		"competitors":   compMap,
		"keywords":      kwMap,
		"optimizations": optMap,
		"metrics":       metricsMap,
		"simulations":   simMap,
	})
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
		locale = "ja"
	}

	strategy, err := h.DB.GetSEOStrategy(repo.ID)
	if err != nil || strategy == nil {
		writeError(w, http.StatusBadRequest, "please configure SEO strategy first")
		return
	}

	client := h.resolveClientForRepo(repo)
	scoutAgent := seo.NewSERPScoutAgent(client)
	kwAgent := seo.NewKeywordIntelligenceAgent(client)

	coreStrategy := &seo.SEOStrategy{
		ProjectName:        strategy.ProjectName,
		Category:           strategy.Category,
		ProductDescription: strategy.ProductDescription,
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
		locale = "ja"
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

	coreStrategy := &seo.SEOStrategy{
		ProjectName:        strategy.ProjectName,
		Category:           strategy.Category,
		ProductDescription: strategy.ProductDescription,
		TargetLocales:      strategy.TargetLocales,
		Goal:               seo.GrowthGoal(strategy.Goal),
		ScopeTier:          seo.KeyScopeTier(strategy.ScopeTier),
		CompetitorURLs:     strategy.CompetitorURLs,
	}

	// Fetch existing translation matrix keys for English and target locale
	matrix, _ := h.DB.GetTranslationMatrix(repo.ID)
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

func (h *Handler) handleRepoChat(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepo(w, r)
	if !ok {
		return
	}

	var req struct {
		Message string `json:"message"`
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

	settings, _ := h.DB.GetRepoSettings(repo.ID)
	provider := llm.ProviderClaude
	model := "claude-sonnet-5"
	if settings != nil {
		if settings.Provider != "" {
			provider = llm.ProviderType(settings.Provider)
		}
		if settings.Model != "" {
			model = settings.Model
		}
	}

	client := llm.NewClient(provider, model)
	engine, err := chat.NewEngine(".", client)
	if err != nil {
		fmt.Fprintf(w, "data: %s\n\n", `{"type":"error","error":"failed to init engine"}`)
		flusher.Flush()
		return
	}

	// Populate engine with database translation matrix candidates
	matrix, _ := h.DB.GetTranslationMatrix(repo.ID)
	if enMap, ok := matrix["en"]; ok {
		for k, v := range enMap {
			engine.Candidates = append(engine.Candidates, types.StringCandidate{
				Key:        k,
				RawValue:   v,
				CleanValue: v,
			})
		}
	}

	eventChan := make(chan chat.ChatEvent, 100)
	doneChan := make(chan struct{})

	go func() {
		defer close(doneChan)
		_, _ = engine.SendMessage(r.Context(), req.Message, eventChan)
	}()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-doneChan:
			for len(eventChan) > 0 {
				ev := <-eventChan
				data, _ := json.Marshal(ev)
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			}
			return
		case ev := <-eventChan:
			data, _ := json.Marshal(ev)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

