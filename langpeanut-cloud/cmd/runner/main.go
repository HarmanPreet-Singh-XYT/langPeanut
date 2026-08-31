// cmd/runner/main.go — langpeanut-runner sandboxed job entrypoint.
//
// This binary runs INSIDE a Docker container (the langpeanut-runner image).
// It receives one job's configuration via environment variables, runs the
// full 6-agent localization pipeline, commits + pushes the result, then
// writes the PipelineResult JSON to /work/result.json and exits.
//
// What this binary DOES NOT have access to:
//   - The Docker socket (no nested container spawning).
//   - The SQLite database file.
//   - The GitHub App private key (it gets only a pre-minted, short-lived
//     installation token embedded in GIT_AUTH_URL).
//   - Any other job's scratch volume.
//
// The host worker process reads /work/result.json after this container exits
// and calls the GitHub PR API — that credential-bearing step stays in the
// trusted host process only.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/langPeanut/langPeanut/pkg/agents"
	"github.com/langPeanut/langPeanut/pkg/llm"
	"github.com/langPeanut/langPeanut/pkg/logger"
	"github.com/langPeanut/langPeanut/pkg/memory"
	"github.com/langPeanut/langPeanut/pkg/platforms"
	"github.com/langPeanut/langPeanut/pkg/types"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg := runnerConfig{
		workDir:                  getenv("WORK_DIR", "/work/repo"),
		resultPath:               getenv("RESULT_PATH", "/work/result.json"),
		apiKey:                   mustGetenv("LLM_API_KEY"),
		provider:                 getenv("LLM_PROVIDER", "openai"),
		model:                    getenv("LLM_MODEL", "gpt-4o-mini"),
		branch:                   mustGetenv("BRANCH"),
		gitAuthURL:               mustGetenv("GIT_AUTH_URL"),
		tonePreset:               getenv("TONE_PRESET", "neutral"),
		userDirective:            getenv("USER_DIRECTIVE", ""),
		customInstallCmd:         getenv("CUSTOM_INSTALL_CMD", ""),
		customBuildCmd:           getenv("CUSTOM_BUILD_CMD", ""),
		rootDir:                  getenv("ROOT_DIR", ""),
		existingTranslationsMode: getenv("EXISTING_TRANSLATIONS_MODE", "skip"),
		baseBranch:               getenv("BASE_BRANCH", ""),
		translationMatrix:        getenv("TRANSLATION_MATRIX", ""),
	}

	// Parse target locales from JSON env var: '["fr","es","de"]'
	var locales []string
	if raw := os.Getenv("TARGET_LOCALES"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &locales); err != nil {
			slog.Error("parse TARGET_LOCALES", "err", err)
			os.Exit(1)
		}
	}
	if len(locales) == 0 {
		slog.Error("TARGET_LOCALES is empty — nothing to translate")
		os.Exit(1)
	}

	// ── Set up git identity inside the sandbox ───────────────────────────────
	gitConfig("safe.directory", "*")
	gitConfig("user.email", "bot@langpeanut.ai")
	gitConfig("user.name", "langPeanut Cloud Bot")

	// ── Checkout the requested base branch if specified ──────────────────────
	if cfg.baseBranch != "" {
		if err := gitRun(cfg.workDir, "checkout", cfg.baseBranch); err != nil {
			slog.Info("switching to base branch via tracking branch", "branch", cfg.baseBranch)
			_ = gitRun(cfg.workDir, "checkout", "-B", cfg.baseBranch, "origin/"+cfg.baseBranch)
		}
	}

	// ── Create the feature branch ─────────────────────────────────────────────
	if err := gitRun(cfg.workDir, "checkout", "-b", cfg.branch); err != nil {
		slog.Error("git checkout -b", "branch", cfg.branch, "err", err)
		os.Exit(1)
	}

	// ── Resolve project directory (supports monorepo subdirectories) ───────────
	projectDir := cfg.workDir
	if cfg.rootDir != "" {
		projectDir = filepath.Join(cfg.workDir, cfg.rootDir)
		slog.Info("runner: targeting subdirectory project root", "dir", projectDir)
	}

	// ── Build the agent pipeline (100% shared with CLI) ──────────────────────
	registry := platforms.NewRegistry()
	platform, _ := registry.AutoDetect(projectDir)

	supervisor, err := agents.NewSupervisorAgent(projectDir, platform)
	if err != nil {
		slog.Error("new supervisor agent", "err", err)
		os.Exit(1)
	}

	if cfg.existingTranslationsMode != "" {
		supervisor.ExistingMode = cfg.existingTranslationsMode
		slog.Info("runner: applied existing translations mode", "mode", cfg.existingTranslationsMode)
	}

	if cfg.apiKey != "" {
		supervisor.Translator.LLM = llm.NewClientWithAPIKey(llm.ProviderType(cfg.provider), cfg.model, cfg.apiKey)
	} else if cfg.provider != "" {
		supervisor.Translator.LLM = llm.NewClient(llm.ProviderType(cfg.provider), cfg.model)
	}

	if cfg.tonePreset != "" && cfg.tonePreset != "default" && supervisor.ProjectMemory != nil {
		supervisor.ProjectMemory.Style = memory.StylePreset(cfg.tonePreset)
	}

	if cfg.userDirective != "" {
		supervisor.UserDirective = cfg.userDirective
		slog.Info("runner: applied user directive", "directive", cfg.userDirective)
	}
	if cfg.customInstallCmd != "" {
		supervisor.CustomInstallCmd = cfg.customInstallCmd
		slog.Info("runner: applied custom install cmd", "cmd", cfg.customInstallCmd)
	}
	if cfg.customBuildCmd != "" {
		supervisor.CustomBuildCmd = cfg.customBuildCmd
		slog.Info("runner: applied custom build cmd", "cmd", cfg.customBuildCmd)
	}

	supervisor.OnProgress = func(msg string) {
		logger.Get().Info("SUPERVISOR", msg)
		slog.Info("progress", "msg", msg)
	}

	// ── Parse translation matrix overrides (from SEO Studio & Matrix editor) ─
	var studioMatrix map[string]map[string]string
	if cfg.translationMatrix != "" && cfg.translationMatrix != "{}" {
		_ = json.Unmarshal([]byte(cfg.translationMatrix), &studioMatrix)
	}

	// ── Run the pipeline ──────────────────────────────────────────────────────
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	result, err := supervisor.RunEndToEnd(ctx, "en", locales, false)
	if err != nil {
		slog.Error("pipeline failed", "err", err)
		// Still write a partial result so the host knows what happened.
		writeResult(cfg.resultPath, result, err)
		os.Exit(1)
	}

	// ── Apply Translation Matrix & SEO Optimizations to locale catalogs ──────
	if len(studioMatrix) > 0 {
		slog.Info("runner: merging studio translation matrix entries", "locales", len(studioMatrix))
		if result.Translations == nil {
			result.Translations = make(map[string]map[string]string)
		}
		for loc, entries := range studioMatrix {
			if len(entries) == 0 {
				continue
			}
			tgtPath := platform.DefaultSourceFile(projectDir, loc)
			if !filepath.IsAbs(tgtPath) {
				tgtPath = filepath.Join(projectDir, tgtPath)
			}
			_ = os.MkdirAll(filepath.Dir(tgtPath), 0755)

			existingEntries := make(map[string]string)
			if raw, rErr := os.ReadFile(tgtPath); rErr == nil {
				if parsed, pErr := platform.ParseLocaleFile(raw, filepath.Ext(tgtPath)); pErr == nil && parsed != nil {
					existingEntries = parsed.Entries
				} else {
					var jMap map[string]string
					if json.Unmarshal(raw, &jMap) == nil {
						existingEntries = jMap
					}
				}
			}

			for k, v := range entries {
				if strings.TrimSpace(v) != "" {
					existingEntries[k] = v
				}
			}

			locData := types.LocaleData{
				LocaleCode: loc,
				Entries:    existingEntries,
			}
			if formatted, fErr := platform.FormatLocaleFile(locData); fErr == nil && len(formatted) > 0 {
				_ = os.WriteFile(tgtPath, formatted, 0644)
				result.TargetLocaleFiles[loc] = tgtPath
			}

			if result.Translations[loc] == nil {
				result.Translations[loc] = make(map[string]string)
			}
			for k, v := range existingEntries {
				result.Translations[loc][k] = v
			}
		}
	}

	// ── Commit + push ─────────────────────────────────────────────────────────
	// Remove ephemeral directories so only pure code & locale files are committed to the PR
	_ = os.RemoveAll(filepath.Join(cfg.workDir, "trajectories"))
	_ = os.RemoveAll(filepath.Join(cfg.workDir, ".langPeanut"))

	if err := commitAndPush(cfg); err != nil {
		slog.Error("commit/push failed", "err", err)
		writeResult(cfg.resultPath, result, err)
		os.Exit(1)
	}

	// ── Write result for host to read ────────────────────────────────────────
	writeResult(cfg.resultPath, result, nil)
	slog.Info("runner: done", "branch", cfg.branch)
}

// commitAndPush stages all changes, creates a commit, and pushes the branch.
func commitAndPush(cfg runnerConfig) error {
	if err := gitRun(cfg.workDir, "add", "-A"); err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	// Check if there is anything to commit.
	if err := gitRun(cfg.workDir, "diff", "--cached", "--quiet"); err == nil {
		// Even if all strings were already up to date, create a synchronization marker
		// so git commit & push succeed and the Pull Request is created on GitHub.
		slog.Info("runner: catalogs up to date, creating sync marker commit")
		_ = os.WriteFile(filepath.Join(cfg.workDir, ".langpeanut-sync"), []byte(time.Now().UTC().Format(time.RFC3339)), 0644)
		_ = gitRun(cfg.workDir, "add", ".langpeanut-sync")
	}
	msg := fmt.Sprintf("i18n: automated localization via langPeanut Cloud\n\nBranch: %s\nTime: %s",
		cfg.branch, time.Now().UTC().Format(time.RFC3339))
	if err := gitRun(cfg.workDir, "commit", "-m", msg); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}
	// Push to GitHub using the authenticated remote URL (token embedded).
	if err := gitRun(cfg.workDir, "push", cfg.gitAuthURL, cfg.branch); err != nil {
		return fmt.Errorf("git push: %s", redactOutput(err.Error(), cfg.gitAuthURL))
	}
	return nil
}

// writeResult serialises a sandboxResult to the result path.
// The host worker reads this file after the container exits.
func writeResult(path string, result *agents.PipelineResult, pipelineErr error) {
	type tokenUsageRecord struct {
		Provider     string  `json:"provider"`
		Model        string  `json:"model"`
		InputTokens  int64   `json:"input_tokens"`
		OutputTokens int64   `json:"output_tokens"`
		CostUSD      float64 `json:"cost_usd"`
	}

	type sandboxResult struct {
		TotalInputTokens    int64                        `json:"total_input_tokens"`
		TotalOutputTokens   int64                        `json:"total_output_tokens"`
		EstimatedCostUSD    float64                      `json:"estimated_cost_usd"`
		ScannedFilesCount   int                          `json:"scanned_files_count"`
		ExtractedCandidates int                          `json:"extracted_candidates"`
		UniqueKeysCount     int                          `json:"unique_keys_count"`
		RefactoredFiles     []string                     `json:"refactored_files"`
		GeneratedLocales    []string                     `json:"generated_locales"`
		SourceLocaleFile    string                       `json:"source_locale_file"`
		TargetLocaleFiles   map[string]string            `json:"target_locale_files"`
		VerificationReport  *types.VerificationReport    `json:"verification_report,omitempty"`
		DirectiveResult     *types.DirectiveResult       `json:"directive_result,omitempty"`
		Translations        map[string]map[string]string `json:"translations,omitempty"`
		UnresolvedErrors    []types.CompilerDiagnostic   `json:"unresolved_errors,omitempty"`
		DiagnosticAdvice    *logger.DiagnosticAdvice     `json:"diagnostic_advice,omitempty"`
		ExecutionLogs       []logger.LogEvent            `json:"execution_logs,omitempty"`
		PipelineError       string                       `json:"pipeline_error,omitempty"`
		TokenUsage          []tokenUsageRecord           `json:"token_usage,omitempty"`
	}

	res := sandboxResult{}
	if result != nil {
		res.ScannedFilesCount = result.ScannedFilesCount
		res.ExtractedCandidates = result.ExtractedCandidates
		res.UniqueKeysCount = result.UniqueKeysCount
		res.RefactoredFiles = result.RefactoredFiles
		res.GeneratedLocales = result.GeneratedLocales
		res.SourceLocaleFile = result.SourceLocaleFile
		res.TargetLocaleFiles = result.TargetLocaleFiles
		res.VerificationReport = result.VerificationReport
		res.DirectiveResult = result.DirectiveResult
		res.Translations = result.Translations
		res.UnresolvedErrors = result.UnresolvedErrors
		res.DiagnosticAdvice = result.DiagnosticAdvice
		res.ExecutionLogs = result.ExecutionLogs
	}
	if len(res.ExecutionLogs) == 0 {
		res.ExecutionLogs = logger.Get().GetRecent(100)
	}

	sessionStats := llm.GetGlobalTracker().GetSessionStats()
	res.TotalInputTokens = sessionStats.TotalInputTokens
	res.TotalOutputTokens = sessionStats.TotalOutputTokens
	res.EstimatedCostUSD = sessionStats.TotalEstimatedCostUSD

	for _, mu := range sessionStats.ByModel {
		res.TokenUsage = append(res.TokenUsage, tokenUsageRecord{
			Provider:     mu.Provider,
			Model:        mu.Model,
			InputTokens:  mu.InputTokens,
			OutputTokens: mu.OutputTokens,
			CostUSD:      mu.EstimatedCostUSD,
		})
	}

	if pipelineErr != nil {
		res.PipelineError = pipelineErr.Error()
	}

	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	data, _ := json.MarshalIndent(res, "", "  ")
	_ = os.WriteFile(path, data, 0o644)
}

// ─── helpers ─────────────────────────────────────────────────────────────────

type runnerConfig struct {
	workDir                  string
	resultPath               string
	apiKey                   string
	provider                 string
	model                    string
	branch                   string
	gitAuthURL               string
	tonePreset               string
	userDirective            string
	customInstallCmd         string
	customBuildCmd           string
	rootDir                  string
	existingTranslationsMode string
	baseBranch               string
	translationMatrix        string
}

func gitRun(dir string, args ...string) error {
	fullArgs := append([]string{"-c", "safe.directory=*"}, args...)
	cmd := exec.Command("git", fullArgs...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(), "HOME=/tmp", "GIT_CONFIG_GLOBAL=/tmp/.gitconfig")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return nil
}

func gitConfig(key, value string) {
	cmd := exec.Command("git", "-c", "safe.directory=*", "config", "--global", "--add", key, value)
	cmd.Env = append(os.Environ(), "HOME=/tmp", "GIT_CONFIG_GLOBAL=/tmp/.gitconfig")
	_ = cmd.Run()
}

func redactOutput(s, authURL string) string {
	if idx := strings.Index(authURL, "@"); idx != -1 {
		token := authURL[len("https://x-access-token:"):idx]
		return strings.ReplaceAll(s, token, "***")
	}
	return s
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustGetenv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required env var not set", "var", key)
		os.Exit(1)
	}
	return v
}
