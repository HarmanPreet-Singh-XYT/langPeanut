// Package worker implements the in-process job claim loop.
// A single goroutine polls the jobs table for pending rows, claims them
// atomically, runs the full job pipeline (mirror → dedupe → sandbox launch →
// PR creation), and persists the result.
// See cloud_plan.md §6 for the full 12-step flow this implements.
package worker

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/langPeanut/langPeanut/pkg/agents"
	ghpkg "github.com/langPeanut/langPeanut/pkg/github"
	"github.com/langPeanut/langPeanut/pkg/logger"
	"github.com/langPeanut/langPeanut/pkg/types"
	"github.com/langPeanut/langpeanut-cloud/internal/auth"
	"github.com/langPeanut/langpeanut-cloud/internal/db"
	"github.com/langPeanut/langpeanut-cloud/internal/mirror"
)

// Config holds all runtime configuration the worker needs.
type Config struct {
	DB            *db.DB
	Mirror        *mirror.Manager
	JobsDir       string        // base dir for per-job scratch volumes, e.g. /data/jobs
	MasterKey     string        // hex AES-256 master key for decrypting api_credentials
	RunnerImage   string        // docker image name for the sandbox, e.g. langpeanut-runner:latest
	AppID         string        // GitHub App ID (numeric string, as used by AppConfig)
	PrivateKeyPEM []byte        // raw PEM bytes of the GitHub App's RSA private key
	PollInterval  time.Duration // how often to poll for pending jobs (default: 5s)

	// Resource limits applied to every runner container.
	RunnerMemoryLimit string        // e.g. "512m"
	RunnerCPULimit    string        // e.g. "1"
	RunnerTimeout     time.Duration // wall-clock timeout per job (default: 10m)
}

// Run starts the worker loop.  It blocks until ctx is cancelled.
// Call this in a goroutine from cmd/server/main.go alongside the HTTP server.
func Run(ctx context.Context, cfg Config) {
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 5 * time.Second
	}
	if cfg.RunnerMemoryLimit == "" {
		cfg.RunnerMemoryLimit = "512m"
	}
	if cfg.RunnerCPULimit == "" {
		cfg.RunnerCPULimit = "1"
	}
	if cfg.RunnerTimeout == 0 {
		cfg.RunnerTimeout = 10 * time.Minute
	}

	slog.Info("worker: started", "poll_interval", cfg.PollInterval)
	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("worker: shutting down")
			return
		case <-ticker.C:
			if err := processNextJob(ctx, cfg); err != nil {
				slog.Error("worker: job error", "err", err)
			}
		}
	}
}

// processNextJob claims one pending job and runs it end-to-end.
// Returns nil if no job was pending.
func processNextJob(ctx context.Context, cfg Config) error {
	job, err := cfg.DB.ClaimNextPendingJob()
	if err != nil {
		return fmt.Errorf("claim job: %w", err)
	}
	if job == nil {
		return nil // nothing to do
	}
	slog.Info("worker: claimed job", "job_id", job.ID, "repo_id", job.RepoID)

	// Wrap in a per-job timeout so a runaway sandbox doesn't block the worker forever.
	jobCtx, cancel := context.WithTimeout(ctx, cfg.RunnerTimeout)
	defer cancel()

	if err := runJob(jobCtx, cfg, job); err != nil {
		advice := logger.ExplainError(err)
		errMsg := err.Error()
		if advice != nil {
			errMsg = fmt.Sprintf("[%s] %s — Root Cause: %s", advice.Subsystem, advice.Title, advice.RootCause)
		}
		slog.Error("worker: job failed", "job_id", job.ID, "err", err, "advice", errMsg)
		return cfg.DB.UpdateJobStatus(job.ID, "failed", job.Branch, "", "", "", errMsg)
	}
	return nil
}

// runJob executes the full 12-step pipeline for one claimed job.
func runJob(ctx context.Context, cfg Config, job *db.Job) error {
	// ── Step 3: resolve repo + settings + decrypt API key ───────────────────
	repo, err := cfg.DB.GetRepoByID(job.RepoID)
	if err != nil || repo == nil {
		return fmt.Errorf("get repo %d: %w", job.RepoID, err)
	}

	settings, err := cfg.DB.GetRepoSettings(job.RepoID)
	if err != nil || settings == nil {
		return fmt.Errorf("get repo settings for %d: %w", job.RepoID, err)
	}

	installation, err := cfg.DB.GetInstallationByID(repo.InstallationID)
	if err != nil || installation == nil {
		return fmt.Errorf("get installation %d: %w", repo.InstallationID, err)
	}

	// Parse the App private key PEM once per job.
	privateKey, err := ghpkg.ParsePrivateKeyPEM(cfg.PrivateKeyPEM)
	if err != nil {
		return fmt.Errorf("parse private key: %w", err)
	}
	appCfg := ghpkg.AppConfig{
		AppID:      cfg.AppID,
		PrivateKey: privateKey,
	}

	// Mint a fresh installation token for this job.
	tok, err := ghpkg.CreateInstallationToken(ctx, appCfg, installation.InstallationID)
	if err != nil {
		return fmt.Errorf("mint installation token: %w", err)
	}
	installToken := tok.Token

	// Decrypt the LLM API key (check optional repo override first, then fall back to global team credential).
	var apiKey string
	if len(settings.EncryptedAPIKeyOverride) > 0 {
		key, err := auth.Decrypt(cfg.MasterKey, settings.EncryptedAPIKeyOverride)
		if err == nil && key != "" {
			apiKey = key
			slog.Info("worker: using repo-specific API key override", "job_id", job.ID, "provider", settings.Provider)
		}
	}
	if apiKey == "" {
		cred, err := cfg.DB.GetAPICredential(installation.TeamID, settings.Provider)
		if err != nil || cred == nil {
			return fmt.Errorf("no API key configured for provider '%s'. Please add your API key in Global AI Keys Vault or Repo Settings.", settings.Provider)
		}
		key, err := auth.Decrypt(cfg.MasterKey, cred.EncryptedKey)
		if err != nil {
			return fmt.Errorf("decrypt api key: %w", err)
		}
		apiKey = key
	}

	// ── Step 4–5: mirror + dedupe ────────────────────────────────────────────
	authURL := fmt.Sprintf("https://x-access-token:%s@github.com/%s/%s.git",
		installToken, repo.Owner, repo.Name)
	mirrorPath, err := cfg.Mirror.EnsureMirror(repo.ID, authURL)
	if err != nil {
		return fmt.Errorf("ensure mirror: %w", err)
	}

	headSHA, err := cfg.Mirror.HeadCommitSHA(mirrorPath, repo.DefaultBranch)
	if err != nil {
		return fmt.Errorf("head commit sha: %w", err)
	}

	settingsHash := computeSettingsHash(settings)

	dup, err := cfg.DB.HasDuplicateSuccessfulJob(job.RepoID, headSHA, settingsHash)
	if err != nil {
		return fmt.Errorf("dedupe check: %w", err)
	}
	if dup {
		slog.Info("worker: skipping — already processed", "job_id", job.ID, "sha", headSHA[:8])
		return cfg.DB.UpdateJobStatus(job.ID, "skipped_no_changes", "", headSHA, settingsHash, "", "")
	}

	// ── Step 5 cont: clone working copy from mirror ──────────────────────────
	branch := fmt.Sprintf("langpeanut/i18n-%d-%s", time.Now().Unix(), headSHA[:7])
	scratchDir := filepath.Join(cfg.JobsDir, strconv.FormatInt(job.ID, 10))
	defer os.RemoveAll(scratchDir) // unconditional cleanup per §6.3

	workDir := filepath.Join(scratchDir, "repo")
	if err := cfg.Mirror.CloneFromMirror(mirrorPath, workDir, authURL); err != nil {
		return fmt.Errorf("clone from mirror: %w", err)
	}

	// ── Step 6: launch sandboxed runner container ────────────────────────────
	resultPath := filepath.Join(scratchDir, "result.json")
	sandboxErr := launchSandbox(ctx, cfg, job, scratchDir, resultPath,
		apiKey, settings, branch, authURL)

	// ── Step 10–11: read result, open PR, persist usage ─────────────────────
	// We always attempt a PR even after sandbox error — partial results still have value.
	result, parseErr := readSandboxResult(resultPath)
	if parseErr != nil || result == nil {
		errMsg := "sandbox produced no result"
		if sandboxErr != nil {
			errMsg = sandboxErr.Error()
		}
		return cfg.DB.UpdateJobStatus(job.ID, "failed", branch, headSHA, settingsHash, "", errMsg)
	}

	// Mint a fresh token for PR creation (the job token may have expired for long runs).
	prTok, err := ghpkg.CreateInstallationToken(ctx, appCfg, installation.InstallationID)
	if err != nil {
		slog.Warn("worker: could not refresh token for PR; reusing job token", "err", err)
		prTok = tok
	}

	// Build the agents.PipelineResult needed by BuildPullRequest / OpenLocalizationPR.
	pipelineResult := sandboxResultToPipelineResult(result)

	meta := ghpkg.RunMetadata{
		Locales:          settings.Locales,
		TonePreset:       settings.TonePreset,
		Provider:         settings.Provider,
		Model:            settings.Model,
		PromptTokens:     result.TotalInputTokens,
		CompletionTokens: result.TotalOutputTokens,
		EstimatedCostUSD: result.EstimatedCostUSD,
	}

	// PR creation stays in the trusted host process — sandbox never holds the App key.
	pr, prErr := ghpkg.OpenLocalizationPR(ctx, prTok.Token, repo.Owner, repo.Name,
		branch, repo.DefaultBranch, pipelineResult, meta)
	prURL := ""
	if pr != nil {
		prURL = pr.HTMLURL
	}
	if prErr != nil {
		slog.Warn("worker: PR open failed", "job_id", job.ID, "err", prErr)
	}

	// Persist token usage rows.
	for _, u := range result.TokenUsage {
		_ = cfg.DB.RecordJobTokenUsage(&db.JobTokenUsage{
			JobID:        job.ID,
			Provider:     u.Provider,
			Model:        u.Model,
			InputTokens:  u.InputTokens,
			OutputTokens: u.OutputTokens,
			CostUSD:      u.CostUSD,
		})
	}

	finalStatus := "succeeded"
	if len(result.UnresolvedErrors) > 0 {
		finalStatus = "needs_review"
	}
	errMsg := ""
	if prErr != nil {
		advice := logger.ExplainError(prErr)
		if advice != nil {
			errMsg = fmt.Sprintf("[%s] %s — %s", advice.Subsystem, advice.Title, advice.RootCause)
		} else {
			errMsg = prErr.Error()
		}
	} else if result.PipelineError != "" {
		errMsg = result.PipelineError
	}

	logsBytes, _ := json.Marshal(result.ExecutionLogs)
	matrixBytes, _ := json.Marshal(result.Translations)
	if len(result.Translations) > 0 {
		_ = cfg.DB.UpsertTranslationMatrix(repo.ID, result.Translations)
	}

	return cfg.DB.UpdateJobStatusWithDetails(job.ID, finalStatus, branch, headSHA, settingsHash, prURL, errMsg, string(logsBytes), string(matrixBytes))
}

// launchSandbox spawns a langpeanut-runner container per §6.3 or falls back to runner binary.
func launchSandbox(ctx context.Context, cfg Config, job *db.Job,
	scratchDir, resultPath, apiKey string,
	settings *db.RepoSettings, branch, authURL string,
) error {
	localesJSON, _ := json.Marshal(settings.Locales)
	hostJobsDir := os.Getenv("HOST_JOBS_DIR")
	hostScratchDir := scratchDir
	if hostJobsDir != "" {
		hostScratchDir = filepath.Join(hostJobsDir, strconv.FormatInt(job.ID, 10))
	}

	directive := settings.UserDirective
	if directive == "" {
		directive = os.Getenv("USER_DIRECTIVE")
	}

	// 1. Try Docker sandbox execution if docker CLI is present
	dockerPath, dockerErr := exec.LookPath("docker")
	if dockerErr == nil {
		args := []string{
			"run", "--rm",
			"--memory", cfg.RunnerMemoryLimit,
			"--cpus", cfg.RunnerCPULimit,
			// Scratch volume: only this job's directory.
			"-v", hostScratchDir + ":/work",
			// LLM API key — passed as env var, never written to disk inside sandbox.
			"-e", "LLM_API_KEY=" + apiKey,
			"-e", "LLM_PROVIDER=" + settings.Provider,
			"-e", "LLM_MODEL=" + settings.Model,
			"-e", "TARGET_LOCALES=" + string(localesJSON),
			"-e", "TONE_PRESET=" + settings.TonePreset,
			"-e", "USER_DIRECTIVE=" + directive,
			"-e", "BRANCH=" + branch,
			"-e", "GIT_AUTH_URL=" + authURL,
			"-e", "CUSTOM_INSTALL_CMD=" + settings.CustomInstallCmd,
			"-e", "CUSTOM_BUILD_CMD=" + settings.CustomBuildCmd,
			"-e", "ROOT_DIR=" + settings.RootDir,
			"-e", "EXISTING_TRANSLATIONS_MODE=" + settings.ExistingTranslationsMode,
			"-e", "RESULT_PATH=/work/result.json",
			"-e", "WORK_DIR=/work/repo",
			cfg.RunnerImage,
		}

		cmd := exec.CommandContext(ctx, dockerPath, args...)
		out, err := cmd.CombinedOutput()
		if err == nil {
			slog.Info("worker: sandbox completed via docker", "job_id", job.ID)
			return nil
		}
		slog.Warn("worker: docker run failed, attempting runner binary fallback", "job_id", job.ID, "err", err, "out", strings.TrimSpace(string(out)))
	}

	// 2. Direct runner binary execution fallback
	runnerPath, err := exec.LookPath("langpeanut-runner")
	if err != nil {
		candidates := []string{
			"./runner",
			"./langpeanut-runner",
			"/usr/local/bin/langpeanut-runner",
			"/app/langpeanut-runner",
			filepath.Join(filepath.Dir(os.Args[0]), "runner"),
			filepath.Join(filepath.Dir(os.Args[0]), "langpeanut-runner"),
		}
		for _, c := range candidates {
			if _, statErr := os.Stat(c); statErr == nil {
				runnerPath = c
				err = nil
				break
			}
		}
	}
	if err != nil || runnerPath == "" {
		if dockerErr == nil {
			return fmt.Errorf("docker sandbox failed and langpeanut-runner binary not found")
		}
		return fmt.Errorf("neither docker nor langpeanut-runner binary found on system")
	}

	cmd := exec.CommandContext(ctx, runnerPath)
	cmd.Env = append(os.Environ(),
		"WORK_DIR="+filepath.Join(scratchDir, "repo"),
		"RESULT_PATH="+resultPath,
		"LLM_API_KEY="+apiKey,
		"LLM_PROVIDER="+settings.Provider,
		"LLM_MODEL="+settings.Model,
		"TARGET_LOCALES="+string(localesJSON),
		"TONE_PRESET="+settings.TonePreset,
		"USER_DIRECTIVE="+directive,
		"BRANCH="+branch,
		"GIT_AUTH_URL="+authURL,
		"CUSTOM_INSTALL_CMD="+settings.CustomInstallCmd,
		"CUSTOM_BUILD_CMD="+settings.CustomBuildCmd,
		"ROOT_DIR="+settings.RootDir,
		"EXISTING_TRANSLATIONS_MODE="+settings.ExistingTranslationsMode,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("runner binary (job %d) failed: %s", job.ID, strings.TrimSpace(string(out)))
	}
	slog.Info("worker: sandbox completed via runner binary", "job_id", job.ID)
	return nil
}

// readSandboxResult reads the PipelineResult JSON written by the runner to resultPath.
func readSandboxResult(resultPath string) (*sandboxResult, error) {
	data, err := os.ReadFile(resultPath)
	if err != nil {
		return nil, err
	}
	var r sandboxResult
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parse sandbox result: %w", err)
	}
	return &r, nil
}

// sandboxResultToPipelineResult converts the compact sandbox JSON into the
// agents.PipelineResult shape that pkg/github's BuildPullRequest / OpenLocalizationPR need.
func sandboxResultToPipelineResult(r *sandboxResult) *agents.PipelineResult {
	if r == nil {
		return &agents.PipelineResult{}
	}
	pr := &agents.PipelineResult{
		ScannedFilesCount:   r.ScannedFilesCount,
		ExtractedCandidates: r.ExtractedCandidates,
		UniqueKeysCount:     r.UniqueKeysCount,
		RefactoredFiles:     r.RefactoredFiles,
		GeneratedLocales:    r.GeneratedLocales,
		SourceLocaleFile:    r.SourceLocaleFile,
		TargetLocaleFiles:   r.TargetLocaleFiles,
		VerificationReport:  r.VerificationReport,
		DirectiveResult:     r.DirectiveResult,
		Translations:        r.Translations,
		UnresolvedErrors:    r.UnresolvedErrors,
		DiagnosticAdvice:    r.DiagnosticAdvice,
		ExecutionLogs:       r.ExecutionLogs,
	}
	return pr
}

// sandboxResult is the JSON contract the runner writes to /work/result.json.
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
	TokenUsage          []tokenUsageRecord           `json:"token_usage"`
}

type tokenUsageRecord struct {
	Provider     string  `json:"provider"`
	Model        string  `json:"model"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	CostUSD      float64 `json:"cost_usd"`
}

// computeSettingsHash returns a deterministic SHA-256 of the repo settings JSON
// used as the dedupe key alongside head_commit_sha (§6.2).
func computeSettingsHash(s *db.RepoSettings) string {
	b, _ := json.Marshal(s)
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h)
}
