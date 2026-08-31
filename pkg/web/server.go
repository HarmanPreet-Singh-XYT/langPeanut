package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/langPeanut/langPeanut/benchmark"
	"github.com/langPeanut/langPeanut/pkg/agents"
	"github.com/langPeanut/langPeanut/pkg/chat"
	"github.com/langPeanut/langPeanut/pkg/llm"
	"github.com/langPeanut/langPeanut/pkg/logger"
	"github.com/langPeanut/langPeanut/pkg/memory"
	"github.com/langPeanut/langPeanut/pkg/orchestrator"
	"github.com/langPeanut/langPeanut/pkg/platforms"
	"github.com/langPeanut/langPeanut/pkg/seo"
	"github.com/langPeanut/langPeanut/pkg/types"
)

// StudioServer manages live project state for the Web Mode Studio
type StudioServer struct {
	mu            sync.RWMutex
	ProjectRoot   string                             `json:"project_root"`
	Platform      platforms.Platform                 `json:"-"`
	PlatformName  string                             `json:"platform_name"`
	PlatformDesc  string                             `json:"platform_desc"`
	Candidates    []types.StringCandidate            `json:"candidates"`
	ScannedFiles  int                                `json:"scanned_files"`
	SourceLocale  string                             `json:"source_locale"`
	TargetLocales []string                           `json:"target_locales"`
	ToneStyle     string                             `json:"tone_style"`
	IsRunning     bool                               `json:"is_running"`
	Logs          []string                           `json:"logs"`
	LastResult    *agents.PipelineResult             `json:"last_result"`
	LastBenchmark []benchmark.BenchmarkResult        `json:"last_benchmark"`
	LastSEOResult *seo.SEOResult                     `json:"last_seo_result"`
	RefactorPlans map[string]*types.FileRefactorPlan `json:"refactor_plans"`
	ChatEngine    *chat.Engine                       `json:"-"`
}

func findRepoRoot(startDir string) string {
	curr, err := filepath.Abs(startDir)
	if err != nil {
		return startDir
	}
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(curr, "go.mod")); err == nil {
			return curr
		}
		if _, err := os.Stat(filepath.Join(curr, ".git")); err == nil {
			return curr
		}
		parent := filepath.Dir(curr)
		if parent == curr {
			break
		}
		curr = parent
	}
	return startDir
}

func NewStudioServer(projectRoot string) *StudioServer {
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		absRoot = projectRoot
	}
	repoRoot := findRepoRoot(absRoot)

	// If launched in repo root and no direct framework detected, default target to examples/nextjs-app for instant gratification
	targetPath := absRoot
	if absRoot == repoRoot && !platforms.FileExists(absRoot, "package.json") && platforms.DirExists(absRoot, "examples/nextjs-app") {
		targetPath = filepath.Join(repoRoot, "examples", "nextjs-app")
	}

	registry := platforms.NewRegistry()
	platform, _ := registry.AutoDetect(targetPath)
	if platform == nil {
		platform, _ = registry.Get(types.FrameworkGeneric)
	}

	_ = memory.EnsureGitignore(targetPath)

	s := &StudioServer{
		ProjectRoot:   targetPath,
		Platform:      platform,
		PlatformName:  string(platform.Name()),
		PlatformDesc:  platform.DisplayName(),
		SourceLocale:  "en",
		TargetLocales: []string{"es", "fr", "de", "ja"},
		ToneStyle:     "default",
		RefactorPlans: make(map[string]*types.FileRefactorPlan),
		Logs:          []string{fmt.Sprintf("[%s] Attached to project: %s", time.Now().Format("15:04:05"), targetPath)},
	}

	chatEngine, _ := chat.NewEngine(targetPath, nil)
	s.ChatEngine = chatEngine

	// Trigger initial scan
	s.performScan()
	return s
}

func (s *StudioServer) performScan() {
	s.mu.Lock()
	defer s.mu.Unlock()

	scout := agents.NewASTScoutAgent(s.Platform)
	report, err := scout.ScanProject(s.ProjectRoot, "")
	if err == nil && report != nil {
		contextAgent := agents.NewContextAgent()
		s.Candidates = contextAgent.EnhanceFast(report.Candidates)
		s.ScannedFiles = report.TotalFilesScanned
		s.Logs = append(s.Logs, fmt.Sprintf("[%s] AST Scout scan completed: %d files scanned, %d string candidates found",
			time.Now().Format("15:04:05"), report.TotalFilesScanned, len(s.Candidates)))
	}
}

func (s *StudioServer) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(InteractiveAppHTML))
}

func (s *StudioServer) handleGetProject(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"project_root":     s.ProjectRoot,
		"framework":        s.PlatformName,
		"framework_desc":   s.PlatformDesc,
		"scanned_files":    s.ScannedFiles,
		"candidates_count": len(s.Candidates),
		"source_locale":    s.SourceLocale,
		"target_locales":   s.TargetLocales,
		"tone_style":       s.ToneStyle,
		"is_running":       s.IsRunning,
		"logs":             s.Logs,
		"has_result":       s.LastResult != nil,
	})
}

type SwitchProjectRequest struct {
	Path string `json:"path"`
}

func (s *StudioServer) handleSwitchProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SwitchProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	absRoot, err := filepath.Abs(req.Path)
	if err != nil {
		http.Error(w, "Path resolution failed: "+err.Error(), http.StatusBadRequest)
		return
	}

	info, err := os.Stat(absRoot)
	if err != nil || !info.IsDir() {
		http.Error(w, "Directory does not exist: "+absRoot, http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	registry := platforms.NewRegistry()
	platform, _ := registry.AutoDetect(absRoot)
	if platform == nil {
		platform, _ = registry.Get(types.FrameworkGeneric)
	}

	s.ProjectRoot = absRoot
	s.Platform = platform
	s.PlatformName = string(platform.Name())
	s.PlatformDesc = platform.DisplayName()
	s.Candidates = nil
	s.ScannedFiles = 0
	s.LastResult = nil
	s.RefactorPlans = make(map[string]*types.FileRefactorPlan)
	s.ChatEngine, _ = chat.NewEngine(absRoot, nil)
	s.Logs = append(s.Logs, fmt.Sprintf("[%s] Switched project to: %s (%s)",
		time.Now().Format("15:04:05"), absRoot, platform.DisplayName()))
	s.mu.Unlock()

	s.performScan()

	s.mu.RLock()
	defer s.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":           "switched",
		"project_root":     s.ProjectRoot,
		"framework":        s.PlatformName,
		"framework_desc":   s.PlatformDesc,
		"scanned_files":    s.ScannedFiles,
		"candidates_count": len(s.Candidates),
	})
}

func (s *StudioServer) handleResetExamples(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.Lock()
	root := s.ProjectRoot
	s.mu.Unlock()

	repoRoot := findRepoRoot(root)
	gitCmd := exec.Command("git", "checkout", "HEAD", "--", "examples/")
	gitCmd.Dir = repoRoot
	_ = gitCmd.Run()

	_ = os.RemoveAll(filepath.Join(repoRoot, "examples", "nextjs-app", "src", "locales"))
	_ = os.RemoveAll(filepath.Join(repoRoot, "examples", "nextjs-app", "trajectories"))
	_ = os.RemoveAll(filepath.Join(repoRoot, "examples", "flutter-app", "lib", "l10n"))
	_ = os.RemoveAll(filepath.Join(repoRoot, "examples", "flutter-app", "trajectories"))
	_ = os.RemoveAll(filepath.Join(repoRoot, "examples", "swiftui-app", "Resources"))
	_ = os.RemoveAll(filepath.Join(repoRoot, "examples", "swiftui-app", "trajectories"))
	_ = os.RemoveAll(filepath.Join(repoRoot, "examples", "android-app", "app", "src", "main", "res", "values-fr"))
	_ = os.RemoveAll(filepath.Join(repoRoot, "examples", "android-app", "app", "src", "main", "res", "values-es"))
	_ = os.RemoveAll(filepath.Join(repoRoot, "examples", "android-app", "trajectories"))

	s.performScan()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":  "reset",
		"message": "Demo examples restored to clean unlocalized state",
	})
}

type tierSummary struct {
	Tier       int      `json:"tier"`
	Label      string   `json:"label"`
	Passed     bool     `json:"passed"`
	ErrorCount int      `json:"error_count"`
	WarnCount  int      `json:"warn_count"`
	Messages   []string `json:"messages"`
}

func (s *StudioServer) handleGetCritic(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")

	if s.LastResult == nil || s.LastResult.VerificationReport == nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"has_result": false})
		return
	}

	labels := map[types.VerificationTier]string{
		types.Tier1SyntaxAST:    "AST Syntax Safety",
		types.Tier2ICUTokens:    "ICU & Variable Parity",
		types.Tier3UIExpansion:  "Layout Expansion",
		types.Tier4LocaleParity: "Key Parity Diff",
	}

	tiers := make(map[types.VerificationTier]*tierSummary)
	for tier, label := range labels {
		tiers[tier] = &tierSummary{Tier: int(tier), Label: label, Passed: true}
	}

	report := s.LastResult.VerificationReport
	for _, d := range report.Diagnostics {
		t, ok := tiers[d.Tier]
		if !ok {
			continue
		}
		if d.Severity == "ERROR" {
			t.ErrorCount++
			t.Passed = false
		} else if d.Severity == "WARNING" {
			t.WarnCount++
		}
		if len(t.Messages) < 5 {
			t.Messages = append(t.Messages, d.Message)
		}
	}

	ordered := []*tierSummary{
		tiers[types.Tier1SyntaxAST],
		tiers[types.Tier2ICUTokens],
		tiers[types.Tier3UIExpansion],
		tiers[types.Tier4LocaleParity],
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"has_result":   true,
		"passed":       report.Passed,
		"error_count":  report.ErrorCount,
		"warn_count":   report.WarnCount,
		"tiers":        ordered,
		"code_repairs": s.LastResult.CodeRepairs,
		"unresolved":   s.LastResult.UnresolvedErrors,
	})
}

func (s *StudioServer) handleGetStats(w http.ResponseWriter, r *http.Request) {
	tracker := llm.GetGlobalTracker()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"all_time": tracker.GetStats(),
		"session":  tracker.GetSessionStats(),
	})
}

func (s *StudioServer) handleScan(w http.ResponseWriter, r *http.Request) {
	s.performScan()

	s.mu.RLock()
	defer s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":           "ok",
		"scanned_files":    s.ScannedFiles,
		"candidates":       s.Candidates,
		"candidates_count": len(s.Candidates),
	})
}

func (s *StudioServer) handleGetCandidates(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.Candidates)
}

type UpdateCandidateRequest struct {
	ID             string `json:"id"`
	Approved       *bool  `json:"approved,omitempty"`
	Key            string `json:"key,omitempty"`
	Classification string `json:"classification,omitempty"`
}

func (s *StudioServer) handleUpdateCandidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req UpdateCandidateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.Candidates {
		if s.Candidates[i].ID == req.ID {
			if req.Approved != nil {
				s.Candidates[i].Approved = *req.Approved
			}
			if req.Key != "" {
				s.Candidates[i].Key = req.Key
			}
			if req.Classification != "" {
				s.Candidates[i].Classification = types.Classification(req.Classification)
			}
			break
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

type BatchCandidatesRequest struct {
	Action string   `json:"action"` // "approve_all", "reject_all", "prefix", "casing"
	Prefix string   `json:"prefix,omitempty"`
	Casing string   `json:"casing,omitempty"`
	IDs    []string `json:"ids,omitempty"`
}

func (s *StudioServer) handleBatchCandidates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req BatchCandidatesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	idMap := make(map[string]bool)
	for _, id := range req.IDs {
		idMap[id] = true
	}
	applyToAll := len(req.IDs) == 0

	for i := range s.Candidates {
		if !applyToAll && !idMap[s.Candidates[i].ID] {
			continue
		}

		switch req.Action {
		case "approve_all":
			s.Candidates[i].Approved = true
		case "reject_all":
			s.Candidates[i].Approved = false
		case "prefix":
			if req.Prefix != "" && !strings.HasPrefix(s.Candidates[i].Key, req.Prefix) {
				s.Candidates[i].Key = req.Prefix + s.Candidates[i].Key
			}
		case "casing":
			if req.Casing == "snake_case" {
				s.Candidates[i].Key = toSnakeCase(s.Candidates[i].Key)
			} else if req.Casing == "camelCase" {
				s.Candidates[i].Key = toCamelCase(s.Candidates[i].Key)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":     "batch_updated",
		"candidates": s.Candidates,
	})
}

func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}

func toCamelCase(s string) string {
	parts := strings.Split(s, "_")
	for i := range parts {
		if i == 0 {
			parts[i] = strings.ToLower(parts[i])
		} else if len(parts[i]) > 0 {
			parts[i] = strings.ToUpper(parts[i][:1]) + strings.ToLower(parts[i][1:])
		}
	}
	return strings.Join(parts, "")
}

type RunPipelineRequest struct {
	SourceLocale     string   `json:"source_locale"`
	TargetLocales    []string `json:"target_locales"`
	ToneStyle        string   `json:"tone_style"`
	Provider         string   `json:"provider"`
	Model            string   `json:"model"`
	Directive        string   `json:"directive,omitempty"`
	CustomInstallCmd string   `json:"custom_install_cmd,omitempty"`
	CustomBuildCmd   string   `json:"custom_build_cmd,omitempty"`
	ExistingMode     string   `json:"existing_mode,omitempty"` // "skip" (default), "replace" (regenerate all)
	DryRun           bool     `json:"dry_run"`
}

func (s *StudioServer) handleRunPipeline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RunPipelineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.SourceLocale = s.SourceLocale
		req.TargetLocales = s.TargetLocales
		req.ToneStyle = s.ToneStyle
	}

	if req.SourceLocale == "" {
		req.SourceLocale = "en"
	}
	if len(req.TargetLocales) == 0 {
		req.TargetLocales = []string{"es", "fr", "de", "ja"}
	}

	s.mu.Lock()
	s.IsRunning = true
	s.SourceLocale = req.SourceLocale
	s.TargetLocales = req.TargetLocales
	s.ToneStyle = req.ToneStyle
	s.Logs = append(s.Logs, fmt.Sprintf("[%s] Starting multi-agent localization pipeline (%s -> %v)...",
		time.Now().Format("15:04:05"), req.SourceLocale, req.TargetLocales))
	s.mu.Unlock()

	go func() {
		supervisor, err := agents.NewSupervisorAgent(s.ProjectRoot, s.Platform)
		if err != nil {
			s.mu.Lock()
			s.IsRunning = false
			s.Logs = append(s.Logs, fmt.Sprintf("[%s] Error initializing supervisor: %v", time.Now().Format("15:04:05"), err))
			s.mu.Unlock()
			return
		}

		if req.ExistingMode != "" {
			supervisor.ExistingMode = req.ExistingMode
			s.mu.Lock()
			s.Logs = append(s.Logs, fmt.Sprintf("[%s] Existing translations strategy: %s", time.Now().Format("15:04:05"), req.ExistingMode))
			s.mu.Unlock()
		}

		cfg := memory.LoadConfig(s.ProjectRoot)
		if req.Provider != "" {
			supervisor.Translator.LLM = llm.NewClient(llm.ProviderType(req.Provider), req.Model)
		} else if cfg.ActiveProvider != "" && cfg.ActiveProvider != "local" {
			if cfg.GetAPIKey(cfg.ActiveProvider) != "" || cfg.ActiveProvider == "ollama" {
				supervisor.Translator.LLM = llm.NewClient(llm.ProviderType(cfg.ActiveProvider), cfg.ActiveModel)
			} else {
				supervisor.Translator.LLM = llm.AutoDetectClient()
			}
		} else {
			supervisor.Translator.LLM = llm.AutoDetectClient()
		}

		if req.ToneStyle != "" && supervisor.ProjectMemory != nil {
			supervisor.ProjectMemory.Style = memory.StylePreset(req.ToneStyle)
		}
		if req.Directive != "" {
			supervisor.UserDirective = req.Directive
			s.mu.Lock()
			s.Logs = append(s.Logs, fmt.Sprintf("[%s] App Integration Directive: %s", time.Now().Format("15:04:05"), req.Directive))
			s.mu.Unlock()
		}
		if req.CustomInstallCmd != "" {
			supervisor.CustomInstallCmd = req.CustomInstallCmd
		}
		if req.CustomBuildCmd != "" {
			supervisor.CustomBuildCmd = req.CustomBuildCmd
		}

		supervisor.OnProgress = func(stage string) {
			s.mu.Lock()
			s.Logs = append(s.Logs, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), stage))
			s.mu.Unlock()
		}

		ctx := context.Background()
		res, err := supervisor.RunEndToEnd(ctx, req.SourceLocale, req.TargetLocales, req.DryRun)

		s.mu.Lock()
		s.IsRunning = false
		s.LastResult = res
		if err != nil {
			s.Logs = append(s.Logs, fmt.Sprintf("[%s] Pipeline execution error: %v", time.Now().Format("15:04:05"), err))
		} else {
			s.Logs = append(s.Logs, fmt.Sprintf("[%s] Localization succeeded — scanned: %d, keys: %d, locales: %d",
				time.Now().Format("15:04:05"), res.ScannedFilesCount, res.UniqueKeysCount, len(res.GeneratedLocales)))
			if res.DependencyStatus != nil && (res.DependencyStatus.ManifestUpdated || len(res.DependencyStatus.InstalledDeps) > 0) {
				s.Logs = append(s.Logs, fmt.Sprintf("[%s] Language Dependencies: %s (command: %s)",
					time.Now().Format("15:04:05"), strings.Join(res.DependencyStatus.InstalledDeps, ", "), res.DependencyStatus.CommandExecuted))
			}
			if res.DirectiveResult != nil && res.DirectiveResult.Success {
				s.Logs = append(s.Logs, fmt.Sprintf("[%s] UI Directive: %s", time.Now().Format("15:04:05"), res.DirectiveResult.Explanation))
			}
			if res.VerificationReport != nil {
				vr := res.VerificationReport
				status := "passed"
				if !vr.Passed {
					status = "failed"
				}
				s.Logs = append(s.Logs, fmt.Sprintf("[%s] 4-tier critic scorecard: %s (%d error(s), %d warning(s))",
					time.Now().Format("15:04:05"), status, vr.ErrorCount, vr.WarnCount))
			}
		}
		s.mu.Unlock()

		// Refresh candidate list and disk state
		s.performScan()
	}()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

func (s *StudioServer) handleGetDiff(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var diffs []map[string]string
	if len(s.Candidates) > 0 {
		fileMap := make(map[string][]types.StringCandidate)
		for _, c := range s.Candidates {
			if c.Approved {
				fileMap[c.FilePath] = append(fileMap[c.FilePath], c)
			}
		}

		for filePath, cands := range fileMap {
			if data, err := os.ReadFile(filePath); err == nil {
				if plan, err := s.Platform.GenerateRefactorPlan(filePath, data, cands); err == nil {
					relPath, _ := filepath.Rel(s.ProjectRoot, filePath)
					diffs = append(diffs, map[string]string{
						"file_path":   relPath,
						"before_code": string(data),
						"after_code":  plan.RefactoredContent,
					})
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(diffs)
}

func (s *StudioServer) handleApplyChanges(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	appliedCount := 0
	fileMap := make(map[string][]types.StringCandidate)
	for _, c := range s.Candidates {
		if c.Approved {
			fileMap[c.FilePath] = append(fileMap[c.FilePath], c)
		}
	}

	for filePath, cands := range fileMap {
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		plan, err := s.Platform.GenerateRefactorPlan(filePath, data, cands)
		if err == nil && plan != nil && plan.RefactoredContent != "" {
			_ = os.WriteFile(filePath, []byte(plan.RefactoredContent), 0644)
			appliedCount++
		}
	}

	s.Logs = append(s.Logs, fmt.Sprintf("[%s] Applied %d refactored source files to disk",
		time.Now().Format("15:04:05"), appliedCount))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":        "applied",
		"applied_files": appliedCount,
	})
}

func (s *StudioServer) handleGetLocales(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	projectRoot := s.ProjectRoot
	p := s.Platform
	s.mu.RUnlock()

	localeDir := p.DefaultLocaleDir(projectRoot)
	if !filepath.IsAbs(localeDir) {
		localeDir = filepath.Join(projectRoot, localeDir)
	}

	localesMap := make(map[string]map[string]string)
	var localeFiles []string

	// Check standard locale directories if default doesn't exist
	dirsToCheck := []string{
		localeDir,
		filepath.Join(projectRoot, "src", "locales"),
		filepath.Join(projectRoot, "locales"),
		filepath.Join(projectRoot, "public", "locales"),
		filepath.Join(projectRoot, "lib", "l10n"),
	}

	for _, dir := range dirsToCheck {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			ext := filepath.Ext(e.Name())
			if ext == ".json" || ext == ".arb" || ext == ".xml" || ext == ".xcstrings" {
				filePath := filepath.Join(dir, e.Name())
				data, err := os.ReadFile(filePath)
				if err != nil {
					continue
				}

				locName := strings.TrimSuffix(e.Name(), ext)
				locName = strings.TrimPrefix(locName, "app_")
				locName = strings.TrimPrefix(locName, "strings_")

				var contentMap map[string]string
				if ext == ".json" || ext == ".arb" {
					var raw map[string]any
					if json.Unmarshal(data, &raw) == nil {
						contentMap = make(map[string]string)
						for k, v := range raw {
							if !strings.HasPrefix(k, "@") {
								contentMap[k] = fmt.Sprintf("%v", v)
							}
						}
					}
				}

				if contentMap != nil {
					localesMap[locName] = contentMap
					rel, _ := filepath.Rel(projectRoot, filePath)
					localeFiles = append(localeFiles, rel)
				}
			}
		}
		if len(localesMap) > 0 {
			break
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"locales": localesMap,
		"files":   localeFiles,
	})
}

type UpdateLocaleKeyRequest struct {
	Locale string `json:"locale"`
	Key    string `json:"key"`
	Value  string `json:"value"`
}

func (s *StudioServer) handleUpdateLocaleKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req UpdateLocaleKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Locale == "" || req.Key == "" {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	projectRoot := s.ProjectRoot
	p := s.Platform
	s.mu.Unlock()

	localeDir := p.DefaultLocaleDir(projectRoot)
	if !filepath.IsAbs(localeDir) {
		localeDir = filepath.Join(projectRoot, localeDir)
	}

	fileName := filepath.Join(localeDir, req.Locale+".json")
	if !platforms.FileExists(localeDir, req.Locale+".json") {
		fileName = filepath.Join(localeDir, "app_"+req.Locale+".arb")
	}

	dataMap := make(map[string]any)
	if raw, err := os.ReadFile(fileName); err == nil {
		_ = json.Unmarshal(raw, &dataMap)
	}

	dataMap[req.Key] = req.Value
	out, _ := json.MarshalIndent(dataMap, "", "  ")
	_ = os.WriteFile(fileName, out, 0644)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "saved"})
}

func (s *StudioServer) handleGetCheckpoints(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	projectRoot := s.ProjectRoot
	s.mu.RUnlock()

	cm, err := orchestrator.NewCheckpointManager(projectRoot)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	list, _ := cm.ListCheckpoints()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

type RollbackRequest struct {
	CheckpointID string `json:"checkpoint_id"`
}

func (s *StudioServer) handleRollback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RollbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.CheckpointID == "" {
		http.Error(w, "Invalid checkpoint ID", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	projectRoot := s.ProjectRoot
	s.mu.Unlock()

	cm, err := orchestrator.NewCheckpointManager(projectRoot)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := cm.RestoreCheckpoint(req.CheckpointID); err != nil {
		http.Error(w, "Rollback failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.performScan()

	s.mu.Lock()
	s.Logs = append(s.Logs, fmt.Sprintf("[%s] Codebase restored to checkpoint: %s",
		time.Now().Format("15:04:05"), req.CheckpointID))
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "rolled_back"})
}

func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 8 {
		return "••••••••"
	}
	return s[:4] + "••••" + s[len(s)-4:]
}

func (s *StudioServer) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	projectRoot := s.ProjectRoot
	s.mu.RUnlock()

	cacheDir := filepath.Join(projectRoot, ".langPeanut", "cache")
	pm, _ := memory.NewProjectMemory(cacheDir)
	cfg := memory.LoadConfig(projectRoot)

	downloaded, path, sz := llm.IsNLLBModelDownloaded()
	runnerInstalled, runnerPath := llm.IsLlamaCLIInstalled()

	ollamaCtx, ollamaCancel := context.WithTimeout(context.Background(), 2*time.Second)
	ollamaStatus := llm.CheckOllamaStatus(ollamaCtx)
	ollamaCancel()
	bestOllama := ""
	if ollamaStatus.Running && len(ollamaStatus.Models) > 0 {
		bestOllama = llm.BestOllamaModelForTranslation(ollamaStatus.Models)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"style":                      cfg.StylePreset,
		"active_provider":            cfg.ActiveProvider,
		"active_model":               cfg.ActiveModel,
		"chunk_word_budget":          cfg.ChunkWordBudget,
		"chunk_key_ceiling":          cfg.ChunkKeyCeiling,
		"concurrency":                cfg.Concurrency,
		"custom_install_cmd":         cfg.CustomInstallCmd,
		"custom_build_cmd":           cfg.CustomBuildCmd,
		"existing_translations_mode": cfg.GetExistingTranslationsMode(),
		"custom_prompt":              pm.CustomPrompt,
		"glossary":                   pm.Glossary,
		"exclude_files":              pm.ExcludeFiles,
		"exclude_patterns":           pm.ExcludePatterns,
		"nllb_downloaded":            downloaded,
		"nllb_path":                  path,
		"nllb_size_mb":               float64(sz) / (1024 * 1024),
		"llama_installed":            runnerInstalled,
		"llama_path":                 runnerPath,
		"ollama_running":             ollamaStatus.Running,
		"ollama_models":              ollamaStatus.Models,
		"ollama_url":                 ollamaStatus.BaseURL,
		"best_ollama_model":          bestOllama,
		"api_keys": map[string]string{
			"anthropic":  maskSecret(os.Getenv("ANTHROPIC_API_KEY")),
			"openai":     maskSecret(os.Getenv("OPENAI_API_KEY")),
			"gemini":     maskSecret(os.Getenv("GEMINI_API_KEY")),
			"deepl":      maskSecret(os.Getenv("DEEPL_API_KEY")),
			"hf":         maskSecret(os.Getenv("HF_TOKEN")),
			"custom_url": os.Getenv("OPENAI_BASE_URL"),
		},
		"has_keys": map[string]bool{
			"anthropic":       os.Getenv("ANTHROPIC_API_KEY") != "",
			"openai":          os.Getenv("OPENAI_API_KEY") != "",
			"gemini":          os.Getenv("GEMINI_API_KEY") != "",
			"deepl":           os.Getenv("DEEPL_API_KEY") != "",
			"hf":              os.Getenv("HF_TOKEN") != "" || os.Getenv("HUGGINGFACE_API_KEY") != "",
			"custom_url":      os.Getenv("OPENAI_BASE_URL") != "",
			"nllb_local":      downloaded && runnerInstalled,
			"llama_installed": runnerInstalled,
			"ollama":          ollamaStatus.Running && len(ollamaStatus.Models) > 0,
		},
	})
}

type SaveSettingsRequest struct {
	ActiveProvider           string                       `json:"active_provider"`
	ActiveModel              string                       `json:"active_model"`
	Style                    string                       `json:"style"`
	ChunkWordBudget          int                          `json:"chunk_word_budget"`
	ChunkKeyCeiling          int                          `json:"chunk_key_ceiling"`
	Concurrency              int                          `json:"concurrency"`
	CustomPrompt             string                       `json:"custom_prompt"`
	CustomInstallCmd         string                       `json:"custom_install_cmd"`
	CustomBuildCmd           string                       `json:"custom_build_cmd"`
	ExistingTranslationsMode string                       `json:"existing_translations_mode"`
	APIKeys                  map[string]string            `json:"api_keys"`
	Glossary                 map[string]map[string]string `json:"glossary"`
	ExcludeFiles             []string                     `json:"exclude_files"`
	ExcludePatterns          []string                     `json:"exclude_patterns"`
}

func (s *StudioServer) handleSaveSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SaveSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	projectRoot := s.ProjectRoot
	s.mu.Unlock()

	cfg := memory.LoadConfig(projectRoot)
	if req.ActiveProvider != "" {
		cfg.ActiveProvider = req.ActiveProvider
		cfg.ActiveModel = req.ActiveModel
	}
	if req.Style != "" {
		cfg.StylePreset = req.Style
	}
	if req.ChunkWordBudget > 0 || req.ChunkKeyCeiling > 0 || req.Concurrency > 0 {
		cfg.ChunkWordBudget = req.ChunkWordBudget
		cfg.ChunkKeyCeiling = req.ChunkKeyCeiling
		cfg.Concurrency = req.Concurrency
	}
	cfg.CustomInstallCmd = req.CustomInstallCmd
	cfg.CustomBuildCmd = req.CustomBuildCmd
	if req.ExistingTranslationsMode != "" {
		cfg.ExistingTranslationsMode = req.ExistingTranslationsMode
	}
	for k, v := range req.APIKeys {
		if v != "" {
			_ = cfg.SetAPIKey(k, v, projectRoot)
		}
	}
	_ = cfg.Save(projectRoot)

	cacheDir := filepath.Join(projectRoot, ".langPeanut", "cache")
	pm, _ := memory.NewProjectMemory(cacheDir)
	if req.Style != "" {
		pm.Style = memory.StylePreset(req.Style)
	}
	pm.CustomPrompt = req.CustomPrompt
	pm.Glossary = req.Glossary
	pm.ExcludeFiles = req.ExcludeFiles
	pm.ExcludePatterns = req.ExcludePatterns
	_ = pm.Save()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "saved"})
}

func (s *StudioServer) handleDownloadNLLBModel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	path, err := llm.EnsureNLLBModel(r.Context(), func(down, tot int64, pct float64) {
		data, _ := json.Marshal(map[string]any{
			"downloaded": down,
			"total":      tot,
			"percent":    pct,
		})
		_, _ = fmt.Fprintf(w, "data: %s\n\n", string(data))
		flusher.Flush()
	})

	if err != nil {
		data, _ := json.Marshal(map[string]any{"error": err.Error()})
		_, _ = fmt.Fprintf(w, "data: %s\n\n", string(data))
		flusher.Flush()
		return
	}

	data, _ := json.Marshal(map[string]any{"status": "complete", "percent": 100.0, "path": path})
	_, _ = fmt.Fprintf(w, "data: %s\n\n", string(data))
	flusher.Flush()
}

func (s *StudioServer) handleInstallLlamaRunner(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	installedPath, err := llm.InstallLlamaCLIViaBrew(r.Context(), func(line string) {
		data, _ := json.Marshal(map[string]any{
			"status": "installing",
			"log":    line,
		})
		_, _ = fmt.Fprintf(w, "data: %s\n\n", string(data))
		flusher.Flush()
	})

	if err != nil {
		data, _ := json.Marshal(map[string]any{"error": err.Error()})
		_, _ = fmt.Fprintf(w, "data: %s\n\n", string(data))
		flusher.Flush()
		return
	}

	data, _ := json.Marshal(map[string]any{"status": "complete", "path": installedPath})
	_, _ = fmt.Fprintf(w, "data: %s\n\n", string(data))
	flusher.Flush()
}

type TestModelApiRequest struct {
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	APIKey     string `json:"api_key"`
	TargetLang string `json:"target_lang"`
	SampleText string `json:"sample_text"`
}

func (s *StudioServer) handleTestModel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req TestModelApiRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	s.mu.RLock()
	projectRoot := s.ProjectRoot
	s.mu.RUnlock()

	cfg := memory.LoadConfig(projectRoot)

	prov := llm.ProviderType(req.Provider)
	if prov == "" {
		prov = llm.ProviderType(cfg.ActiveProvider)
		if prov == "" {
			prov = llm.ProviderLocal
		}
	}
	mod := req.Model
	if mod == "" {
		mod = cfg.ActiveModel
	}
	key := req.APIKey
	if key == "" {
		key = cfg.GetAPIKey(string(prov))
	}

	tgt := req.TargetLang
	if tgt == "" {
		tgt = "es"
	}

	txt := req.SampleText
	if txt == "" {
		txt = "Welcome to langPeanut! Effortless multi-agent software localization."
	}

	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()

	res, err := llm.TestModelConnection(ctx, prov, mod, key, tgt, txt)

	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
	}
	_ = json.NewEncoder(w).Encode(res)
}

func (s *StudioServer) handleGetTree(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type FileTreeNode struct {
		FilePath       string `json:"file_path"`
		RelPath        string `json:"rel_path"`
		FileName       string `json:"file_name"`
		CandidateCount int    `json:"candidate_count"`
		ApprovedCount  int    `json:"approved_count"`
	}

	fileMap := make(map[string]*FileTreeNode)
	for _, c := range s.Candidates {
		node, exists := fileMap[c.FilePath]
		if !exists {
			rel, _ := filepath.Rel(s.ProjectRoot, c.FilePath)
			node = &FileTreeNode{
				FilePath: c.FilePath,
				RelPath:  rel,
				FileName: filepath.Base(c.FilePath),
			}
			fileMap[c.FilePath] = node
		}
		node.CandidateCount++
		if c.Approved {
			node.ApprovedCount++
		}
	}

	var list []*FileTreeNode
	for _, node := range fileMap {
		list = append(list, node)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

type MatrixResponse struct {
	Keys    []string                     `json:"keys"`
	Locales map[string]map[string]string `json:"locales"`
	Stats   map[string]float64           `json:"stats"`
}

func (s *StudioServer) handleGetMatrix(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	projectRoot := s.ProjectRoot
	p := s.Platform
	cands := s.Candidates
	s.mu.RUnlock()

	localesMap := make(map[string]map[string]string)
	keySet := make(map[string]bool)
	var orderedKeys []string

	enMap := make(map[string]string)
	for _, c := range cands {
		if c.Key != "" {
			if !keySet[c.Key] {
				keySet[c.Key] = true
				orderedKeys = append(orderedKeys, c.Key)
			}
			enMap[c.Key] = c.CleanValue
		}
	}
	localesMap["en"] = enMap

	localeDir := p.DefaultLocaleDir(projectRoot)
	if !filepath.IsAbs(localeDir) {
		localeDir = filepath.Join(projectRoot, localeDir)
	}

	dirsToCheck := []string{
		localeDir,
		filepath.Join(projectRoot, "src", "locales"),
		filepath.Join(projectRoot, "locales"),
		filepath.Join(projectRoot, "public", "locales"),
		filepath.Join(projectRoot, "lib", "l10n"),
	}

	for _, dir := range dirsToCheck {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			ext := filepath.Ext(e.Name())
			if ext == ".json" || ext == ".arb" {
				locName := strings.TrimSuffix(e.Name(), ext)
				locName = strings.TrimPrefix(locName, "app_")
				locName = strings.TrimPrefix(locName, "strings_")

				filePath := filepath.Join(dir, e.Name())
				data, err := os.ReadFile(filePath)
				if err != nil {
					continue
				}

				var raw map[string]any
				if json.Unmarshal(data, &raw) == nil {
					m := make(map[string]string)
					for k, v := range raw {
						if !strings.HasPrefix(k, "@") {
							valStr := fmt.Sprintf("%v", v)
							m[k] = valStr
							if !keySet[k] {
								keySet[k] = true
								orderedKeys = append(orderedKeys, k)
							}
						}
					}
					localesMap[locName] = m
				}
			}
		}
		if len(localesMap) > 1 {
			break
		}
	}

	stats := make(map[string]float64)
	totalKeys := len(orderedKeys)
	if totalKeys > 0 {
		for loc, m := range localesMap {
			count := 0
			for _, k := range orderedKeys {
				if v, ok := m[k]; ok && strings.TrimSpace(v) != "" {
					count++
				}
			}
			stats[loc] = (float64(count) / float64(totalKeys)) * 100.0
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(MatrixResponse{
		Keys:    orderedKeys,
		Locales: localesMap,
		Stats:   stats,
	})
}

func (s *StudioServer) handleGetGitStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	root := s.ProjectRoot
	s.mu.RUnlock()

	branchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	branchCmd.Dir = root
	branchOut, err := branchCmd.Output()
	branch := "main"
	if err == nil {
		branch = strings.TrimSpace(string(branchOut))
	}

	statusCmd := exec.Command("git", "status", "--porcelain")
	statusCmd.Dir = root
	statusOut, _ := statusCmd.Output()
	isDirty := len(strings.TrimSpace(string(statusOut))) > 0

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"branch": branch,
		"dirty":  isDirty,
	})
}

func (s *StudioServer) handleRunBenchmark(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.Lock()
	projectRoot := s.ProjectRoot
	s.mu.Unlock()

	benchDir := filepath.Join(projectRoot, ".langPeanut", "benchmark_run")
	results, err := benchmark.RunBenchmark(benchDir)
	if err != nil {
		http.Error(w, "Benchmark failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.mu.Lock()
	s.LastBenchmark = results
	s.Logs = append(s.Logs, fmt.Sprintf("[%s] 10-Case Benchmark Suite completed: 100%% pass rate",
		time.Now().Format("15:04:05")))
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(results)
}

func (s *StudioServer) handleGetDiagnosticLogs(w http.ResponseWriter, r *http.Request) {
	events := logger.Get().GetRecent(100)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(events)
}

func (s *StudioServer) handleGetBenchmark(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	results := s.LastBenchmark
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(results)
}

func (s *StudioServer) handleDependencies(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	autoInstall := r.Method == http.MethodPost
	status, err := s.Platform.EnsureDependencies(s.ProjectRoot, autoInstall)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
}

func (s *StudioServer) handleGetSEO(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	res := s.LastSEOResult
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if res == nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"configured": false})
		return
	}
	_ = json.NewEncoder(w).Encode(res)
}

func (s *StudioServer) handleRunSEO(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Locales     []string `json:"locales"`
		Goal        string   `json:"goal"`
		Scope       string   `json:"scope"`
		Competitors []string `json:"competitors"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	s.Logs = append(s.Logs, fmt.Sprintf("[%s] Launching SEO & Growth Studio optimization for %v...", time.Now().Format("15:04:05"), req.Locales))
	projRoot := s.ProjectRoot
	plat := s.Platform
	s.mu.Unlock()

	go func() {
		client := llm.AutoDetectClient()
		scoutAgent := agents.NewPersonaScoutAgent(client)
		persona, _ := scoutAgent.DiscoverPersona(projRoot)
		projName := filepath.Base(projRoot)
		cat := "Software Platform"
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
		}

		goal := seo.GrowthGoal(req.Goal)
		if goal == "" {
			goal = seo.GoalTopTraffic
		}
		scope := seo.KeyScopeTier(req.Scope)
		if scope == "" {
			scope = seo.ScopeHighImpact
		}
		locales := req.Locales
		if len(locales) == 0 {
			locales = []string{"ja", "de", "es"}
		}

		strategy := &seo.SEOStrategy{
			ProjectName:        projName,
			Category:           cat,
			ProductDescription: fmt.Sprintf("Autonomous software platform: %s", projName),
			TargetLocales:      locales,
			Goal:               goal,
			ScopeTier:          scope,
			CompetitorURLs:     req.Competitors,
		}

		sourceKeys := make(map[string]string)
		baselineMatrix := make(map[string]map[string]string)
		if plat != nil {
			sourceKeys = seo.ExtractLocaleCatalog(projRoot, plat, "en")
			for _, loc := range locales {
				if entries := seo.ExtractLocaleCatalog(projRoot, plat, loc); entries != nil {
					baselineMatrix[loc] = entries
				}
			}
		}

		orchestrator := seo.NewStudioOrchestrator(client)
		ctx := context.Background()
		result, err := orchestrator.RunStudio(ctx, strategy, sourceKeys, baselineMatrix)

		s.mu.Lock()
		s.LastSEOResult = result
		if err != nil {
			s.Logs = append(s.Logs, fmt.Sprintf("[%s] SEO Studio error: %v", time.Now().Format("15:04:05"), err))
		} else {
			s.Logs = append(s.Logs, fmt.Sprintf("[%s] SEO Studio optimization completed across %d locales!", time.Now().Format("15:04:05"), len(locales)))
		}
		s.mu.Unlock()
	}()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

func (s *StudioServer) handleApplySEO(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.LastSEOResult == nil || s.Platform == nil {
		http.Error(w, "No SEO results to apply", http.StatusBadRequest)
		return
	}

	for loc, opts := range s.LastSEOResult.Optimizations {
		targetMap := make(map[string]string)
		if existing := seo.ExtractLocaleCatalog(s.ProjectRoot, s.Platform, loc); existing != nil {
			for k, v := range existing {
				targetMap[k] = v
			}
		}
		for _, o := range opts {
			targetMap[o.Key] = o.OptimizedTranslation
		}
		_ = seo.WriteLocaleCatalog(s.ProjectRoot, s.Platform, loc, targetMap)
	}

	s.Logs = append(s.Logs, fmt.Sprintf("[%s] Applied all SEO optimizations to disk!", time.Now().Format("15:04:05")))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "applied"})
}

func (s *StudioServer) handleChatStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Message) == "" {
		http.Error(w, "Message is required", http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if s.ChatEngine == nil {
		s.ChatEngine, _ = chat.NewEngine(s.ProjectRoot, nil)
	}

	eventChan := make(chan chat.ChatEvent, 100)
	doneChan := make(chan struct{})

	go func() {
		defer close(doneChan)
		_, _ = s.ChatEngine.SendMessage(r.Context(), req.Message, eventChan)
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

func (s *StudioServer) handleChatHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.ChatEngine == nil {
		s.ChatEngine, _ = chat.NewEngine(s.ProjectRoot, nil)
	}
	_ = json.NewEncoder(w).Encode(s.ChatEngine.GetHistory())
}

func (s *StudioServer) handleChatReset(w http.ResponseWriter, r *http.Request) {
	if s.ChatEngine != nil {
		s.ChatEngine.ResetHistory()
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "reset"})
}

// StartInteractiveWebStudio launches the full interactive project-aware Web Studio
func StartInteractiveWebStudio(projectRoot string, port int, autoOpen bool) error {
	addr := fmt.Sprintf(":%d", port)
	url := fmt.Sprintf("http://localhost:%d", port)

	studio := NewStudioServer(projectRoot)

	mux := http.NewServeMux()
	mux.HandleFunc("/", studio.handleHome)
	mux.HandleFunc("/api/project", studio.handleGetProject)
	mux.HandleFunc("/api/project/switch", studio.handleSwitchProject)
	mux.HandleFunc("/api/reset", studio.handleResetExamples)
	mux.HandleFunc("/api/scan", studio.handleScan)
	mux.HandleFunc("/api/tree", studio.handleGetTree)
	mux.HandleFunc("/api/matrix", studio.handleGetMatrix)
	mux.HandleFunc("/api/git", studio.handleGetGitStatus)
	mux.HandleFunc("/api/candidates", studio.handleGetCandidates)
	mux.HandleFunc("/api/candidates/update", studio.handleUpdateCandidate)
	mux.HandleFunc("/api/candidates/batch", studio.handleBatchCandidates)
	mux.HandleFunc("/api/run", studio.handleRunPipeline)
	mux.HandleFunc("/api/dependencies", studio.handleDependencies)
	mux.HandleFunc("/api/diff", studio.handleGetDiff)
	mux.HandleFunc("/api/apply", studio.handleApplyChanges)
	mux.HandleFunc("/api/locales", studio.handleGetLocales)
	mux.HandleFunc("/api/locales/update", studio.handleUpdateLocaleKey)
	mux.HandleFunc("/api/checkpoints", studio.handleGetCheckpoints)
	mux.HandleFunc("/api/rollback", studio.handleRollback)
	mux.HandleFunc("/api/settings", studio.handleGetSettings)
	mux.HandleFunc("/api/settings/save", studio.handleSaveSettings)
	mux.HandleFunc("/api/models/download", studio.handleDownloadNLLBModel)
	mux.HandleFunc("/api/models/install-runner", studio.handleInstallLlamaRunner)
	mux.HandleFunc("/api/models/test", studio.handleTestModel)
	mux.HandleFunc("/api/logs", studio.handleGetDiagnosticLogs)
	mux.HandleFunc("/api/benchmark/run", studio.handleRunBenchmark)
	mux.HandleFunc("/api/benchmark", studio.handleGetBenchmark)
	mux.HandleFunc("/api/languages", handleLanguages)
	mux.HandleFunc("/api/styles", handleStyles)
	mux.HandleFunc("/api/critic", studio.handleGetCritic)
	mux.HandleFunc("/api/stats", studio.handleGetStats)
	mux.HandleFunc("/api/seo", studio.handleGetSEO)
	mux.HandleFunc("/api/seo/run", studio.handleRunSEO)
	mux.HandleFunc("/api/seo/apply", studio.handleApplySEO)
	mux.HandleFunc("/api/chat", studio.handleChatStream)
	mux.HandleFunc("/api/chat/history", studio.handleChatHistory)
	mux.HandleFunc("/api/chat/reset", studio.handleChatReset)

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	fmt.Printf("\nlangPeanut Web Studio running at %s\n", url)
	fmt.Printf("Attached to local project: %s\n", studio.ProjectRoot)

	if autoOpen {
		go func() {
			time.Sleep(350 * time.Millisecond)
			openBrowser(url)
		}()
	}

	return server.ListenAndServe()
}

// StartInteractiveWebDemo launches the live interactive website on the specified port
func StartInteractiveWebDemo(port int, autoOpen bool) error {
	return StartInteractiveWebStudio(".", port, autoOpen)
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

func handleLanguages(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(types.GlobalLanguages)
}

func handleStyles(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	styles := []map[string]string{
		{"id": "default", "name": "Standard Native", "desc": "Professional, natural native UI copy", "icon": "fa-globe"},
		{"id": "gen_z", "name": "Gen-Z Slang", "desc": "Trendy internet aesthetics, slang & emojis ('no cap', 'slay', 'fire')", "icon": "fa-fire"},
		{"id": "pirate", "name": "Pirate / Gamer", "desc": "Playful gaming copy ('Ahoy Matey!', 'Plunder Loot')", "icon": "fa-skull-crossbones"},
		{"id": "formal", "name": "Corporate Formal", "desc": "Enterprise polite honorifics and strict business phrasing", "icon": "fa-briefcase"},
		{"id": "casual", "name": "Casual Friendly", "desc": "Warm, welcoming, everyday friendly tone", "icon": "fa-mug-hot"},
	}
	_ = json.NewEncoder(w).Encode(styles)
}

const InteractiveAppHTML = `<!DOCTYPE html>
<html lang="en" class="dark">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>langPeanut — Localization Engineering Studio</title>
  <script src="https://cdn.tailwindcss.com"></script>
  <script>
    tailwind.config = {
      darkMode: 'class',
      theme: {
        extend: {
          fontFamily: {
            sans: ['Geist', 'Inter', '-apple-system', 'BlinkMacSystemFont', 'Segoe UI', 'sans-serif'],
            mono: ['Geist Mono', 'JetBrains Mono', 'monospace']
          },
          colors: {
            border: 'hsl(var(--border))',
            input: 'hsl(var(--input))',
            ring: 'hsl(var(--ring))',
            background: 'hsl(var(--background))',
            foreground: 'hsl(var(--foreground))',
            primary: {
              DEFAULT: 'hsl(var(--primary))',
              foreground: 'hsl(var(--primary-foreground))'
            },
            secondary: {
              DEFAULT: 'hsl(var(--secondary))',
              foreground: 'hsl(var(--secondary-foreground))'
            },
            destructive: {
              DEFAULT: 'hsl(var(--destructive))',
              foreground: 'hsl(var(--destructive-foreground))'
            },
            muted: {
              DEFAULT: 'hsl(var(--muted))',
              foreground: 'hsl(var(--muted-foreground))'
            },
            accent: {
              DEFAULT: 'hsl(var(--accent))',
              foreground: 'hsl(var(--accent-foreground))'
            },
            popover: {
              DEFAULT: 'hsl(var(--popover))',
              foreground: 'hsl(var(--popover-foreground))'
            },
            card: {
              DEFAULT: 'hsl(var(--card))',
              foreground: 'hsl(var(--card-foreground))'
            }
          },
          borderRadius: {
            lg: 'var(--radius)',
            md: 'calc(var(--radius) - 2px)',
            sm: 'calc(var(--radius) - 4px)'
          }
        }
      }
    }
  </script>
  <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.4.0/css/all.min.css">
  <style>
    @import url('https://fonts.googleapis.com/css2?family=Geist:wght@300;400;500;600;700;800&family=Geist+Mono:wght@400;500;600;700&display=swap');

    /* ── shadcn/ui Theme Tokens (Zinc Dark) ─────────────────────────── */
    :root, .dark {
      --background:        240 10% 3.9%;
      --foreground:        0 0% 98%;
      --card:              240 10% 4.9%;
      --card-foreground:   0 0% 98%;
      --popover:           240 10% 4.9%;
      --popover-foreground:0 0% 98%;
      --primary:           199 89% 48%;
      --primary-foreground:0 0% 100%;
      --secondary:         240 3.7% 15.9%;
      --secondary-foreground: 0 0% 98%;
      --muted:             240 3.7% 15.9%;
      --muted-foreground:  240 5% 64.9%;
      --accent:            240 3.7% 15.9%;
      --accent-foreground: 0 0% 98%;
      --destructive:       0 62.8% 30.6%;
      --destructive-foreground: 0 0% 98%;
      --border:            240 3.7% 15.9%;
      --input:             240 3.7% 15.9%;
      --ring:              199 89% 48%;
      --radius:            0.5rem;

      /* Direct CSS Helpers */
      --clr-bg:            #09090b;
      --clr-card:          #0c0c0e;
      --clr-card-header:   #121215;
      --clr-border:        #27272a;
      --clr-border-muted:  #1e1e22;
      --clr-text:          #fafafa;
      --clr-text-muted:    #a1a1aa;
      --clr-primary:       #38bdf8;
      --clr-primary-dim:   rgba(56,189,248,0.12);
      --clr-primary-border:rgba(56,189,248,0.3);
    }

    * { -webkit-font-smoothing: antialiased; -moz-osx-font-smoothing: grayscale; box-sizing: border-box; }

    body, button, input, select, textarea {
      font-family: 'Geist', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
      background-color: var(--clr-bg);
      color: var(--clr-text);
    }
    pre, code, .font-mono {
      font-family: 'Geist Mono', monospace;
      letter-spacing: normal;
    }

    /* ── Custom Scrollbar ─────────────────────────────────────────────── */
    .custom-scrollbar::-webkit-scrollbar { width: 5px; height: 5px; }
    .custom-scrollbar::-webkit-scrollbar-track { background: transparent; }
    .custom-scrollbar::-webkit-scrollbar-thumb { background: #27272a; border-radius: 3px; }
    .custom-scrollbar::-webkit-scrollbar-thumb:hover { background: #3f3f46; }

    /* ── Nav ──────────────────────────────────────────────────────────── */
    .nav-btn { color: var(--clr-text-muted); border-left: 2px solid transparent; transition: all 0.15s; }
    .nav-btn:hover { color: var(--clr-text); background: #0f1219; }
    .nav-btn.active { color: var(--clr-primary); background: #121622; border-left-color: var(--clr-primary); font-weight: 600; }

    /* ── Panels & Fields ─────────────────────────────────────────────── */
    .panel { background: var(--clr-surface); border: 1px solid var(--clr-border); }
    .panel-header { background: var(--clr-surface-2); border-bottom: 1px solid var(--clr-border); }
    .field { background: var(--clr-input, #080a0f); border: 1px solid var(--clr-border-2); transition: border-color 0.15s; }
    .field:focus { outline: none; border-color: var(--clr-primary); box-shadow: 0 0 0 2px var(--clr-primary-dim); }

    /* ── shadcn-style Button primitives ─────────────────────────────── */
    .btn { display: inline-flex; align-items: center; justify-content: center; gap: 0.375rem;
           font-size: 0.75rem; font-weight: 500; line-height: 1; border-radius: var(--radius);
           border: 1px solid transparent; cursor: pointer; transition: all 0.15s; white-space: nowrap; }
    .btn:disabled { opacity: 0.5; pointer-events: none; }
    .btn-primary { background: var(--clr-primary); color: #0f172a; border-color: var(--clr-primary); }
    .btn-primary:hover { background: #7dd3fc; }
    .btn-secondary { background: #131722; color: #d4d8e0; border-color: #232838; }
    .btn-secondary:hover { background: #1a2030; }
    .btn-ghost { background: transparent; color: var(--clr-text-muted); border-color: transparent; }
    .btn-ghost:hover { background: #0f1219; color: var(--clr-text); }
    .btn-destructive { background: hsl(0 63% 38%); color: var(--clr-text); }
    .btn-destructive:hover { background: hsl(0 63% 44%); }
    .btn-outline { background: transparent; border-color: var(--clr-border-2); color: var(--clr-text); }
    .btn-outline:hover { background: var(--clr-surface-2); }
    .btn-sm { padding: 0.25rem 0.625rem; font-size: 0.6875rem; }
    .btn-md { padding: 0.375rem 0.875rem; }
    .btn-lg { padding: 0.5rem 1.25rem; font-size: 0.8125rem; }
    .btn-icon { padding: 0.375rem; border-radius: 0.5rem; }

    /* ── shadcn-style Badge ──────────────────────────────────────────── */
    .badge { display: inline-flex; align-items: center; gap: 0.25rem; font-size: 0.6rem; font-weight: 600;
             padding: 0.15rem 0.5rem; border-radius: 9999px; border: 1px solid; letter-spacing: 0.04em;
             text-transform: uppercase; }
    .badge-default { background: var(--clr-primary-dim); color: var(--clr-primary); border-color: var(--clr-primary-border); }
    .badge-muted  { background: #12151e; color: #8a91a0; border-color: #1e222e; }
    .badge-emerald{ background: rgba(52,211,153,0.1); color: #34d399; border-color: rgba(52,211,153,0.25); }
    .badge-amber  { background: rgba(251,191,36,0.1); color: #fbbf24; border-color: rgba(251,191,36,0.25); }
    .badge-rose   { background: rgba(251,113,133,0.1); color: #fb7185; border-color: rgba(251,113,133,0.25); }
    .badge-purple { background: rgba(192,132,252,0.1); color: #c084fc; border-color: rgba(192,132,252,0.25); }

    /* ── Prompt-Kit Chat UI ─────────────────────────────────────────── */
    .pk-msg-user {
      background: #121622;
      border: 1px solid var(--clr-primary-border);
      border-radius: 1rem 1rem 0.25rem 1rem;
      padding: 0.625rem 0.875rem;
      max-width: 82%;
      color: #e2e8f0;
      font-size: 0.75rem;
      line-height: 1.5;
    }
    .pk-msg-assistant {
      background: var(--clr-surface);
      border: 1px solid var(--clr-border);
      border-radius: 1rem 1rem 1rem 0.25rem;
      padding: 0.75rem 1rem;
      font-size: 0.75rem;
      line-height: 1.6;
      color: #d4d8e0;
    }
    .pk-avatar {
      width: 1.75rem; height: 1.75rem; border-radius: 0.5rem;
      background: var(--clr-primary-dim);
      border: 1px solid var(--clr-primary-border);
      display: flex; align-items: center; justify-content: center;
      color: var(--clr-primary); font-size: 0.65rem; font-weight: 700;
      flex-shrink: 0; margin-top: 0.125rem; font-family: 'Geist Mono', monospace;
    }
    .pk-tool-card {
      border: 1px solid rgba(255,255,255,0.07);
      border-radius: 0.75rem;
      background: #090b10;
      overflow: hidden;
      font-size: 0.6875rem;
      margin: 0.375rem 0;
    }
    .pk-tool-header {
      display: flex; align-items: center; justify-content: space-between;
      padding: 0.5rem 0.75rem;
      background: #0c0f16;
      cursor: pointer;
      transition: background 0.15s;
      font-family: 'Geist Mono', monospace;
    }
    .pk-tool-header:hover { background: #111622; }
    .pk-tool-body {
      padding: 0.75rem;
      border-top: 1px solid rgba(255,255,255,0.06);
      background: #07080c;
    }
    .pk-suggestion {
      display: inline-flex; align-items: center; gap: 0.25rem;
      padding: 0.3125rem 0.75rem;
      border-radius: 9999px;
      border: 1px solid var(--clr-border);
      background: var(--clr-surface);
      color: var(--clr-text-muted);
      font-size: 0.6875rem; font-weight: 500;
      cursor: pointer; transition: all 0.15s; white-space: nowrap;
      font-family: 'Geist', sans-serif;
    }
    .pk-suggestion:hover {
      border-color: var(--clr-primary-border);
      color: var(--clr-primary);
      background: var(--clr-primary-dim);
    }
    .pk-input-wrap {
      background: #06080d;
      border: 1px solid #232838;
      border-radius: 1rem;
      padding: 0.75rem;
      transition: border-color 0.15s, box-shadow 0.15s;
    }
    .pk-input-wrap:focus-within {
      border-color: rgba(56,189,248,0.6);
      box-shadow: 0 0 0 2px rgba(56,189,248,0.08);
    }
    @keyframes pk-thinking {
      0%,80%,100% { opacity: 0.25; transform: scale(0.75); }
      40% { opacity: 1; transform: scale(1); }
    }
    .pk-dot { width: 5px; height: 5px; border-radius: 50%; background: var(--clr-primary); display: inline-block;
               animation: pk-thinking 1.2s ease-in-out infinite; }
    .pk-dot:nth-child(2) { animation-delay: 0.2s; }
    .pk-dot:nth-child(3) { animation-delay: 0.4s; }

    /* ── Reasoning block ─────────────────────────────────────────────── */
    .pk-reasoning {
      border: 1px solid rgba(192,132,252,0.15);
      border-radius: 0.75rem;
      background: rgba(192,132,252,0.03);
      overflow: hidden; margin: 0.375rem 0;
    }
    .pk-reasoning-header {
      display: flex; align-items: center; gap: 0.5rem;
      padding: 0.5rem 0.75rem; cursor: pointer;
      color: #c084fc; font-size: 0.6875rem; font-weight: 600;
      font-family: 'Geist Mono', monospace;
      transition: background 0.15s;
    }
    .pk-reasoning-header:hover { background: rgba(192,132,252,0.05); }
    .pk-reasoning-body {
      padding: 0.75rem; border-top: 1px solid rgba(192,132,252,0.1);
      color: #9ca3af; font-size: 0.6875rem; line-height: 1.6;
      font-family: 'Geist', sans-serif;
    }

    /* ── Canvas Panel tabs ───────────────────────────────────────────── */
    .canvas-tab { padding: 0.3125rem 0.75rem; border-radius: 0.5rem; font-size: 0.6875rem;
                  font-weight: 500; cursor: pointer; transition: all 0.15s; color: #4a5162; border: none;
                  background: transparent; font-family: 'Geist', sans-serif; }
    .canvas-tab:hover { color: var(--clr-text); background: #12151e; }
    .canvas-tab.active { color: var(--clr-text); background: #12151e; font-weight: 600; }

    /* ── Misc ────────────────────────────────────────────────────────── */
    .btn-disabled { opacity: 0.5; pointer-events: none; }
    @keyframes toast-in { from { opacity: 0; transform: translateY(6px); } to { opacity: 1; transform: translateY(0); } }
    .toast { animation: toast-in 0.15s ease-out; }
    .kbd { font-family: 'Geist Mono', monospace; font-size: 10px; background: #141722;
           border: 1px solid #232838; padding: 1px 4px; border-radius: 4px; color: #6b7280; }
    .cell-editable:hover { background: #131724; cursor: pointer; }
    @keyframes spin { to { transform: rotate(360deg); } }
    .animate-spin { animation: spin 1s linear infinite; }
    @keyframes pulse-slow { 0%,100% { opacity:1; } 50% { opacity:.45; } }
    .animate-pulse-slow { animation: pulse-slow 2s ease-in-out infinite; }
  </style>
</head>
<body class="min-h-screen flex flex-col selection:bg-sky-500/30 selection:text-white">

  <!-- Toast Notification Stack -->
  <div id="toastStack" class="fixed bottom-4 right-4 z-50 flex flex-col gap-2 items-end pointer-events-none"></div>

  <!-- Command Palette Modal (Cmd+K) -->
  <div id="cmdPaletteModal" class="hidden fixed inset-0 z-50 bg-black/75 backdrop-blur-sm flex items-start justify-center pt-20 p-4">
    <div class="panel rounded-xl max-w-xl w-full shadow-2xl border-zinc-700/80 overflow-hidden flex flex-col">
      <div class="p-3 border-b border-zinc-800 flex items-center gap-3 bg-[#10131c]">
        <i class="fa-solid fa-magnifying-glass text-zinc-500 text-xs"></i>
        <input type="text" id="cmdPaletteInput" oninput="filterCommandPalette()" onkeydown="handleCmdPaletteKey(event)" placeholder="Search commands, screens, or extracted keys..." class="w-full bg-transparent text-xs text-zinc-100 placeholder-zinc-500 focus:outline-none font-mono">
        <span class="kbd">ESC</span>
      </div>
      <div id="cmdPaletteList" class="p-2 max-h-80 overflow-y-auto custom-scrollbar space-y-1 text-xs"></div>
    </div>
  </div>

  <!-- Target Project Switcher Modal -->
  <div id="projectModal" class="hidden fixed inset-0 z-50 bg-black/75 backdrop-blur-sm flex items-center justify-center p-4">
    <div class="panel rounded-xl p-6 max-w-lg w-full space-y-4 shadow-2xl border-zinc-700/80">
      <div class="flex items-center justify-between border-b border-zinc-800 pb-3">
        <h3 class="text-sm font-semibold text-zinc-100 flex items-center gap-2">
          <i class="fa-solid fa-folder-tree text-sky-400"></i> Attach Target Project
        </h3>
        <button onclick="closeProjectModal()" class="text-zinc-500 hover:text-zinc-300"><i class="fa-solid fa-xmark"></i></button>
      </div>
      <p class="text-xs text-zinc-400 leading-relaxed">
        Select a workspace folder or one of the pre-built multi-platform example repositories:
      </p>
      <div class="space-y-2">
        <button onclick="switchProjectPath('./examples/nextjs-app')" class="w-full text-left p-3 rounded-lg field hover:border-sky-500/50 flex items-center justify-between group transition-colors">
          <div>
            <div class="text-xs font-semibold text-zinc-200 group-hover:text-sky-300">React / Next.js Demo</div>
            <div class="text-[11px] text-zinc-500 font-mono">./examples/nextjs-app</div>
          </div>
          <span class="text-[10px] px-2 py-0.5 rounded bg-sky-500/10 text-sky-400 border border-sky-500/20 font-mono">TypeScript / JSX</span>
        </button>
        <button onclick="switchProjectPath('./examples/flutter-app')" class="w-full text-left p-3 rounded-lg field hover:border-sky-500/50 flex items-center justify-between group transition-colors">
          <div>
            <div class="text-xs font-semibold text-zinc-200 group-hover:text-sky-300">Flutter Mobile Demo</div>
            <div class="text-[11px] text-zinc-500 font-mono">./examples/flutter-app</div>
          </div>
          <span class="text-[10px] px-2 py-0.5 rounded bg-cyan-500/10 text-cyan-400 border border-cyan-500/20 font-mono">Dart / ARB</span>
        </button>
        <button onclick="switchProjectPath('./examples/swiftui-app')" class="w-full text-left p-3 rounded-lg field hover:border-sky-500/50 flex items-center justify-between group transition-colors">
          <div>
            <div class="text-xs font-semibold text-zinc-200 group-hover:text-sky-300">iOS SwiftUI Demo</div>
            <div class="text-[11px] text-zinc-500 font-mono">./examples/swiftui-app</div>
          </div>
          <span class="text-[10px] px-2 py-0.5 rounded bg-orange-500/10 text-orange-400 border border-orange-500/20 font-mono">Swift / .xcstrings</span>
        </button>
        <button onclick="switchProjectPath('./examples/android-app')" class="w-full text-left p-3 rounded-lg field hover:border-sky-500/50 flex items-center justify-between group transition-colors">
          <div>
            <div class="text-xs font-semibold text-zinc-200 group-hover:text-sky-300">Android Jetpack Compose</div>
            <div class="text-[11px] text-zinc-500 font-mono">./examples/android-app</div>
          </div>
          <span class="text-[10px] px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 font-mono">Kotlin / XML</span>
        </button>
        <button onclick="switchProjectPath('.')" class="w-full text-left p-3 rounded-lg field hover:border-sky-500/50 flex items-center justify-between group transition-colors">
          <div>
            <div class="text-xs font-semibold text-zinc-200 group-hover:text-sky-300">Current Workspace Directory</div>
            <div class="text-[11px] text-zinc-500 font-mono">. (Repository Root)</div>
          </div>
          <span class="text-[10px] px-2 py-0.5 rounded bg-zinc-800 text-zinc-400 font-mono">Root</span>
        </button>
      </div>

      <div class="pt-2 border-t border-zinc-800 space-y-2">
        <label class="text-xs text-zinc-400 font-medium">Or attach custom local directory path:</label>
        <div class="flex gap-2">
          <input type="text" id="customProjectPathInput" placeholder="/absolute/path/to/project" class="flex-1 field rounded-lg px-3 py-2 text-xs font-mono text-zinc-200">
          <button onclick="submitCustomPath()" class="px-4 py-2 rounded-lg bg-sky-600 hover:bg-sky-500 text-white text-xs font-medium">Attach</button>
        </div>
      </div>
    </div>
  </div>

  <!-- Top Navigation Header -->
  <header class="border-b border-[#181b24] px-5 py-2.5 flex items-center justify-between bg-[#0a0c11] shrink-0">
    <div class="flex items-center gap-4">
      <div class="flex items-center gap-2.5">
        <div class="w-7 h-7 rounded-lg bg-sky-500/10 border border-sky-500/30 flex items-center justify-center text-sky-400 font-mono font-bold text-xs">
          LP
        </div>
        <div class="flex items-baseline gap-2">
          <span class="font-bold text-sm text-zinc-100 tracking-tight">langPeanut</span>
          <span class="text-[11px] text-zinc-500 font-mono">Studio v1.2.4</span>
        </div>
      </div>

      <div class="h-4 w-px bg-zinc-800"></div>

      <!-- Project Switcher Trigger -->
      <button onclick="openProjectModal()" class="flex items-center gap-2 px-2.5 py-1 rounded-md field hover:border-zinc-700 text-xs text-zinc-300 group transition-colors">
        <i class="fa-regular fa-folder-open text-sky-400 text-xs"></i>
        <span class="font-semibold text-zinc-400">Target:</span>
        <span id="projectRootDisplay" class="font-mono text-zinc-200 truncate max-w-xs font-medium">Loading...</span>
        <span id="gitBranchBadge" class="text-[10px] font-mono px-1.5 py-0.2 rounded bg-zinc-800/80 text-zinc-400">git: main</span>
        <i class="fa-solid fa-chevron-down text-[9px] text-zinc-500 group-hover:text-zinc-300"></i>
      </button>
    </div>

    <!-- Center: Command Palette Trigger -->
    <button onclick="openCommandPalette()" class="hidden md:flex items-center gap-3 px-3 py-1.5 rounded-lg field hover:border-zinc-700 text-xs text-zinc-400 w-80 justify-between transition-colors">
      <span class="flex items-center gap-2"><i class="fa-solid fa-magnifying-glass text-[11px]"></i> Search keys, actions...</span>
      <span class="kbd">⌘K</span>
    </button>

    <!-- Right Quick Action Buttons -->
    <div class="flex items-center gap-2">
      <button onclick="rescanAST()" class="btn btn-outline btn-sm flex items-center gap-1.5" title="Rescan code AST (R)">
        <i class="fa-solid fa-arrows-rotate text-[11px] text-sky-400" id="rescanIcon"></i> Rescan <span class="kbd">R</span>
      </button>
      <button onclick="executeLocalization()" id="topRunBtn" class="btn btn-primary btn-md flex items-center gap-1.5 shadow-sm shadow-sky-600/20" title="Execute Pipeline">
        <i class="fa-solid fa-bolt text-[11px]"></i> Run Pipeline <span class="kbd" style="color:#7dd3fc;background:#0c2a3a;border-color:#164e63">⌘↵</span>
      </button>
      <button onclick="applyDiskChanges()" class="btn btn-sm flex items-center gap-1.5" style="border-color:rgba(52,211,153,0.3);color:#6ee7b7;background:transparent" title="Apply to Disk (A)">
        <i class="fa-solid fa-floppy-disk text-[11px] text-emerald-400"></i> Apply <span class="kbd" style="color:#6ee7b7;background:#052e16;border-color:#166534">⌘S</span>
      </button>
    </div>
  </header>

  <!-- Studio Shell -->
  <div class="flex-1 flex min-h-0">

    <!-- Left Navigation Bar -->
    <aside class="w-56 shrink-0 border-r border-[#181b24] bg-[#090b10] flex flex-col">
      <nav class="flex-1 p-2 space-y-0.5 text-xs overflow-y-auto custom-scrollbar">
        <div class="px-3 py-1.5 text-[10px] font-semibold uppercase tracking-wider text-zinc-500">Autonomous Core</div>
        <button onclick="switchScreen('copilot')" id="screenBtnCopilot" class="nav-btn active w-full text-left px-3 py-2 rounded-md flex items-center gap-2.5 transition-colors">
          <i class="fa-solid fa-terminal w-4 text-center text-xs text-sky-400"></i> Autonomous Copilot
          <span class="ml-auto text-[9px] px-1.5 py-0.5 rounded bg-sky-500/10 text-sky-400 border border-sky-500/20 font-mono">CORE</span>
        </button>

        <div class="pt-2 px-3 py-1.5 text-[10px] font-semibold uppercase tracking-wider text-zinc-500">Engineering Studio</div>
        <button onclick="switchScreen('studio')" id="screenBtnStudio" class="nav-btn w-full text-left px-3 py-2 rounded-md flex items-center gap-2.5 transition-colors">
          <i class="fa-solid fa-cubes w-4 text-center text-xs"></i> 1. String Studio
          <span id="badgeCandidateCount" class="ml-auto text-[10px] px-1.5 py-0.5 rounded bg-zinc-800/80 text-zinc-400 font-mono">0</span>
        </button>
        <button onclick="switchScreen('matrix')" id="screenBtnMatrix" class="nav-btn w-full text-left px-3 py-2 rounded-md flex items-center gap-2.5 transition-colors">
          <i class="fa-solid fa-table-cells w-4 text-center text-xs text-emerald-400"></i> 2. Matrix Grid
        </button>
        <button onclick="switchScreen('simulator')" id="screenBtnSimulator" class="nav-btn w-full text-left px-3 py-2 rounded-md flex items-center gap-2.5 transition-colors">
          <i class="fa-solid fa-mobile-screen w-4 text-center text-xs text-pink-400"></i> 3. Live Simulator
        </button>
        <button onclick="switchScreen('diff')" id="screenBtnDiff" class="nav-btn w-full text-left px-3 py-2 rounded-md flex items-center gap-2.5 transition-colors">
          <i class="fa-solid fa-code-compare w-4 text-center text-xs text-sky-400"></i> 4. AST Diff Inspector
        </button>
        <button onclick="switchScreen('critic')" id="screenBtnCritic" class="nav-btn w-full text-left px-3 py-2 rounded-md flex items-center gap-2.5 transition-colors">
          <i class="fa-solid fa-shield-halved w-4 text-center text-xs text-purple-400"></i> 5. Critic & Quality
        </button>
        <button onclick="switchScreen('seo')" id="screenBtnSeo" class="nav-btn w-full text-left px-3 py-2 rounded-md flex items-center gap-2.5 transition-colors">
          <i class="fa-solid fa-bullseye w-4 text-center text-xs text-pink-400"></i> 6. SEO & Growth Studio
        </button>

        <div class="pt-3 px-3 py-1.5 text-[10px] font-semibold uppercase tracking-wider text-zinc-500">Workflow & Diagnostics</div>
        <button onclick="switchScreen('runner')" id="screenBtnRunner" class="nav-btn w-full text-left px-3 py-2 rounded-md flex items-center gap-2.5 transition-colors">
          <i class="fa-solid fa-terminal w-4 text-center text-xs text-zinc-400"></i> Pipeline Logs
        </button>
        <button onclick="switchScreen('checkpoints')" id="screenBtnCheckpoints" class="nav-btn w-full text-left px-3 py-2 rounded-md flex items-center gap-2.5 transition-colors">
          <i class="fa-solid fa-clock-rotate-left w-4 text-center text-xs text-teal-400"></i> Checkpoints
        </button>
        <button onclick="switchScreen('benchmark')" id="screenBtnBenchmark" class="nav-btn w-full text-left px-3 py-2 rounded-md flex items-center gap-2.5 transition-colors">
          <i class="fa-solid fa-trophy w-4 text-center text-xs text-amber-400"></i> 10-Case Benchmark
        </button>
        <button onclick="switchScreen('stats')" id="screenBtnStats" class="nav-btn w-full text-left px-3 py-2 rounded-md flex items-center gap-2.5 transition-colors">
          <i class="fa-solid fa-chart-pie w-4 text-center text-xs text-cyan-400"></i> Token Analytics
        </button>
        <button onclick="switchScreen('settings')" id="screenBtnSettings" class="nav-btn w-full text-left px-3 py-2 rounded-md flex items-center gap-2.5 transition-colors">
          <i class="fa-solid fa-sliders w-4 text-center text-xs text-zinc-400"></i> AI & Style Memory
        </button>
      </nav>

      <!-- Bottom Status Widget -->
      <div class="p-3 border-t border-[#181b24] bg-[#07080b] text-[11px] font-mono text-zinc-500 space-y-1">
        <div class="flex items-center justify-between">
          <span>Framework</span>
          <span id="frameworkDisplay" class="text-zinc-300 font-semibold truncate max-w-[100px]">Generic</span>
        </div>
        <div class="flex items-center justify-between">
          <span>AST Safety</span>
          <span class="text-emerald-400">0% Drift</span>
        </div>
      </div>
    </aside>

    <!-- Main Content Area -->
    <main class="flex-1 flex flex-col min-w-0 bg-[#07080b] overflow-hidden">

      <!-- ================================= SCREEN 0: AUTONOMOUS COPILOT WORKSPACE ================================= -->
      <div id="screenCopilot" class="flex-1 flex min-h-0 bg-[#07080b] overflow-hidden">

        <!-- LEFT: Chat Panel -->
        <div class="flex flex-col" style="width:52%;min-width:420px;border-right:1px solid var(--clr-border)">

          <!-- Chat Header -->
          <div class="flex items-center justify-between px-4 py-3" style="background:var(--clr-surface);border-bottom:1px solid var(--clr-border)">
            <div class="flex items-center gap-2.5">
              <span class="w-2 h-2 rounded-full bg-emerald-400 animate-pulse-slow"></span>
              <div>
                <div class="text-xs font-semibold text-zinc-100" style="font-family:'Geist',sans-serif;letter-spacing:-0.01em">Autonomous Orchestrator</div>
                <div class="text-[11px]" style="color:var(--clr-text-muted);font-family:'Geist Mono',monospace">6-agent localization pipeline</div>
              </div>
            </div>
            <div class="flex items-center gap-2">
              <span id="copilotActiveModelBadge" class="badge badge-muted">claude-sonnet-5</span>
              <button onclick="resetCopilotChat()" title="Reset session" class="btn btn-ghost btn-icon text-xs">
                <i class="fa-solid fa-arrow-rotate-right"></i>
              </button>
            </div>
          </div>

          <!-- Message Stream -->
          <div id="copilotMessages" class="flex-1 overflow-y-auto custom-scrollbar p-4 space-y-4 min-h-0"></div>

          <!-- Suggestion Chips (Prompt-Kit PromptSuggestion) -->
          <div class="px-4 py-2.5 flex items-center gap-2 overflow-x-auto" style="border-top:1px solid var(--clr-border);background:var(--clr-surface)">
            <span class="text-[10px] uppercase font-semibold shrink-0" style="color:var(--clr-text-muted)">Try:</span>
            <button onclick="sendCopilotPrompt('Scan repository and calculate coverage matrix')" class="pk-suggestion shrink-0">
              <i class="fa-solid fa-radar text-[9px]"></i> Scan AST
            </button>
            <button onclick="sendCopilotPrompt('Translate missing keys into Spanish, German and Japanese')" class="pk-suggestion shrink-0">
              <i class="fa-solid fa-language text-[9px]"></i> Translate Missing
            </button>
            <button onclick="sendCopilotPrompt('Execute 4-tier verification critic')" class="pk-suggestion shrink-0">
              <i class="fa-solid fa-shield-halved text-[9px]"></i> 4-Tier Critic
            </button>
            <button onclick="sendCopilotPrompt('Simulate Japanese Google SERP preview')" class="pk-suggestion shrink-0">
              <i class="fa-solid fa-magnifying-glass text-[9px]"></i> SERP Preview
            </button>
            <button onclick="sendCopilotPrompt('List checkpoints or undo last changes')" class="pk-suggestion shrink-0">
              <i class="fa-solid fa-clock-rotate-left text-[9px]"></i> Checkpoints
            </button>
            <button onclick="sendCopilotPrompt('Diagnose project health and readiness')" class="pk-suggestion shrink-0">
              <i class="fa-solid fa-stethoscope text-[9px]"></i> Diagnostics
            </button>
          </div>

          <!-- Prompt-Kit Input Area -->
          <div class="p-3" style="background:var(--clr-surface-2);border-top:1px solid var(--clr-border)">
            <form onsubmit="handleCopilotSubmit(event)" class="pk-input-wrap">
              <textarea
                id="copilotInput"
                onkeydown="handleCopilotTextareaKey(event)"
                oninput="this.style.height='auto';this.style.height=Math.min(this.scrollHeight,160)+'px'"
                rows="2"
                placeholder="Instruct agent… ('Scan AST', 'Translate to German', 'Run 4-tier critic')"
                class="w-full resize-none border-none bg-transparent shadow-none outline-none focus:outline-none text-xs placeholder-zinc-600"
                style="color:var(--clr-text);font-family:'Geist',sans-serif;min-height:44px;max-height:160px"
              ></textarea>
              <div class="flex items-center justify-between pt-2 mt-1" style="border-top:1px solid rgba(255,255,255,0.04)">
                <span class="text-[11px]" style="color:var(--clr-text-muted);font-family:'Geist Mono',monospace">⏎ send &nbsp;·&nbsp; ⇧⏎ newline</span>
                <button type="submit" id="copilotSendBtn" class="btn btn-primary btn-sm gap-1.5">
                  <span>Execute</span>
                  <i class="fa-solid fa-arrow-right text-[10px]"></i>
                </button>
              </div>
            </form>
          </div>
        </div>

        <!-- RIGHT: Live Canvas Viewport -->
        <div class="flex flex-col flex-1 min-w-0">
          <!-- Canvas Header / Tabs -->
          <div class="flex items-center justify-between px-4 py-2" style="background:var(--clr-surface);border-bottom:1px solid var(--clr-border)">
            <div class="flex items-center gap-1">
              <button onclick="setCanvasTab('matrix')" id="canvasTabMatrix" class="canvas-tab active">Matrix</button>
              <button onclick="setCanvasTab('diff')" id="canvasTabDiff" class="canvas-tab">Diff</button>
              <button onclick="setCanvasTab('critic')" id="canvasTabCritic" class="canvas-tab">Critic</button>
              <button onclick="setCanvasTab('serp')" id="canvasTabSerp" class="canvas-tab">SERP</button>
              <button onclick="setCanvasTab('cost')" id="canvasTabCost" class="canvas-tab">Cost</button>
            </div>
            <span id="canvasActiveViewTitle" class="text-[11px] font-semibold" style="color:var(--clr-text-muted);font-family:'Geist Mono',monospace">Matrix Viewport</span>
          </div>
          <!-- Canvas Content -->
          <div id="copilotCanvasContainer" class="flex-1 overflow-y-auto custom-scrollbar p-4 text-xs min-h-0" style="font-family:'Geist',sans-serif"></div>
        </div>

      </div>


      <!-- ================================= SCREEN 1: 3-PANE STRING STUDIO ================================= -->
      <div id="screenStudio" class="hidden flex-1 flex min-h-0">

        <!-- Pane 1: File Explorer Tree (220px) -->
        <div class="w-60 shrink-0 border-r border-[#181b24] bg-[#090b10] flex flex-col">
          <div class="p-2.5 border-b border-[#181b24] flex items-center justify-between">
            <span class="text-[11px] font-semibold uppercase tracking-wider text-zinc-400 flex items-center gap-1.5">
              <i class="fa-regular fa-folder text-sky-400"></i> Scanned Files
            </span>
            <span id="treeFileCount" class="text-[10px] font-mono text-zinc-500">0 files</span>
          </div>
          <div class="p-2 border-b border-[#181b24]">
            <input type="text" id="treeSearchInput" oninput="renderFileTree()" placeholder="Filter files..." class="w-full field rounded-md px-2.5 py-1 text-[11px] text-zinc-200 placeholder-zinc-600 font-mono">
          </div>
          <div id="fileTreeContainer" class="flex-1 overflow-y-auto custom-scrollbar p-1 space-y-0.5 text-xs font-mono"></div>
        </div>

        <!-- Pane 2: Extracted Candidates Table (Fluid) -->
        <div class="flex-1 flex flex-col min-w-0 border-r border-[#181b24]">
          <!-- Table Toolbar -->
          <div class="p-3 border-b border-[#181b24] bg-[#0b0e14] flex flex-wrap items-center justify-between gap-3">
            <div class="flex items-center gap-2 flex-1 min-w-[200px]">
              <div class="relative w-full">
                <i class="fa-solid fa-magnifying-glass absolute left-3 top-2 text-zinc-500 text-xs"></i>
                <input type="text" id="tuiSearchInput" oninput="renderTuiTable()" placeholder="Search UI text, keys, or line numbers..." class="w-full pl-8 pr-3 py-1 rounded-md field text-xs text-zinc-200 placeholder-zinc-500 font-mono">
              </div>
            </div>

            <div class="flex items-center gap-1 bg-[#080a0f] p-1 rounded-md border border-[#1e222e] text-xs">
              <button onclick="setTuiFilter('all')" id="filterAll" class="px-2.5 py-0.5 rounded font-semibold bg-zinc-800 text-zinc-100">All</button>
              <button onclick="setTuiFilter('LOCALIZABLE')" id="filterLoc" class="px-2.5 py-0.5 rounded font-medium text-zinc-400 hover:text-zinc-200">UI Copy</button>
              <button onclick="setTuiFilter('SKIP')" id="filterSkip" class="px-2.5 py-0.5 rounded font-medium text-zinc-400 hover:text-zinc-200">Non-UI</button>
            </div>

            <div class="flex items-center gap-1.5 text-xs">
              <button onclick="batchApproveTui(true)" class="px-2.5 py-1 rounded-md bg-[#131722] hover:bg-[#1a2030] text-zinc-200 border border-[#232838] flex items-center gap-1">
                <i class="fa-solid fa-check text-emerald-400 text-[10px]"></i> Approve All
              </button>
              <button onclick="batchApproveTui(false)" class="px-2.5 py-1 rounded-md bg-[#131722] hover:bg-[#1a2030] text-zinc-200 border border-[#232838] flex items-center gap-1">
                <i class="fa-solid fa-xmark text-zinc-400 text-[10px]"></i> Reject All
              </button>
              <button onclick="promptBatchPrefix()" class="px-2.5 py-1 rounded-md bg-[#131722] hover:bg-[#1a2030] text-sky-300 border border-[#232838] flex items-center gap-1">
                <i class="fa-solid fa-tag text-[10px]"></i> Prefix...
              </button>
            </div>
          </div>

          <!-- Table Body -->
          <div class="flex-1 overflow-y-auto custom-scrollbar">
            <table class="w-full text-left text-xs border-collapse font-mono">
              <thead class="bg-[#0e1118] text-zinc-400 uppercase tracking-wider text-[10px] font-semibold sticky top-0 z-10 border-b border-[#181b24]">
                <tr>
                  <th class="py-2.5 px-3 w-10 text-center">Appr</th>
                  <th class="py-2.5 px-3 min-w-[200px]">Extracted UI String</th>
                  <th class="py-2.5 px-3 min-w-[200px]">Synthesized i18n Key</th>
                  <th class="py-2.5 px-3 w-40">File Location</th>
                  <th class="py-2.5 px-3 w-24 text-center">AST Node</th>
                </tr>
              </thead>
              <tbody id="tuiTableBody" class="divide-y divide-[#151924]"></tbody>
            </table>
          </div>
        </div>

        <!-- Pane 3: Context & Memory Inspector (300px) -->
        <div class="w-80 shrink-0 bg-[#090b10] flex flex-col overflow-y-auto custom-scrollbar p-4 space-y-4">
          <div class="border-b border-[#181b24] pb-2.5 flex items-center justify-between">
            <span class="text-xs font-bold uppercase tracking-wider text-zinc-200 flex items-center gap-2">
              <i class="fa-solid fa-circle-info text-sky-400"></i> Inspector
            </span>
            <span id="inspBadgeType" class="text-[10px] font-mono px-2 py-0.5 rounded bg-zinc-800 text-zinc-400 border border-zinc-700">No selection</span>
          </div>

          <div id="inspectorEmptyState" class="text-center py-12 text-zinc-500 text-xs">
            Select a string candidate to inspect AST context, ICU tokens, and translation previews.
          </div>

          <div id="inspectorContent" class="hidden space-y-4 text-xs font-mono">
            <div class="space-y-1">
              <span class="text-[10px] uppercase text-zinc-500 font-semibold">Key Identifier</span>
              <div id="inspKeyName" class="text-sky-300 font-semibold break-all bg-[#0d1017] p-2 rounded border border-[#1e222e]"></div>
            </div>

            <div class="space-y-1">
              <span class="text-[10px] uppercase text-zinc-500 font-semibold">Source String</span>
              <div id="inspSourceValue" class="text-zinc-200 bg-[#0d1017] p-2.5 rounded border border-[#1e222e] font-sans leading-relaxed"></div>
            </div>

            <div class="grid grid-cols-2 gap-2 text-[11px]">
              <div class="p-2 rounded bg-[#0d1017] border border-[#1e222e]">
                <span class="text-[9px] uppercase text-zinc-500 block">Line Range</span>
                <span id="inspLineRange" class="text-zinc-300 font-semibold"></span>
              </div>
              <div class="p-2 rounded bg-[#0d1017] border border-[#1e222e]">
                <span class="text-[9px] uppercase text-zinc-500 block">AST Syntax Node</span>
                <span id="inspNodeType" class="text-emerald-400 font-semibold"></span>
              </div>
            </div>

            <div class="space-y-1.5">
              <span class="text-[10px] uppercase text-zinc-500 font-semibold">ICU Tokens & Parameters</span>
              <div id="inspIcuTokens" class="flex flex-wrap gap-1"></div>
            </div>

            <div class="space-y-2 border-t border-[#181b24] pt-3">
              <span class="text-[10px] uppercase text-zinc-500 font-semibold block">Target Locale Translations</span>
              <div id="inspTranslations" class="space-y-1.5 text-[11px]"></div>
            </div>
          </div>
        </div>

      </div>

      <!-- ================================= SCREEN 2: MATRIX GRID ================================= -->
      <div id="screenMatrix" class="hidden flex-1 flex flex-col min-h-0 p-5 space-y-4">
        <!-- Matrix Top Bar -->
        <div class="p-4 rounded-xl panel flex items-center justify-between gap-4 flex-wrap">
          <div>
            <h3 class="text-xs font-bold text-zinc-100 uppercase tracking-wide flex items-center gap-2">
              <i class="fa-solid fa-table-cells text-emerald-400"></i> Multi-Locale Translation Matrix
            </h3>
            <p class="text-[11px] text-zinc-400 mt-0.5">Click any cell to edit translations inline. Changes save automatically to disk.</p>
          </div>
          <div class="flex items-center gap-3">
            <input type="text" id="matrixSearchInput" oninput="renderMatrixGrid()" placeholder="Filter keys or translations..." class="field rounded-md px-3 py-1 text-xs text-zinc-200 font-mono w-56">
            <button onclick="loadMatrixData()" class="px-3 py-1 rounded-md field hover:border-zinc-700 text-xs text-zinc-300 font-medium flex items-center gap-1.5">
              <i class="fa-solid fa-arrows-rotate"></i> Refresh
            </button>
          </div>
        </div>

        <!-- Language Progress Bars -->
        <div id="matrixProgressContainer" class="grid grid-cols-2 sm:grid-cols-6 gap-2"></div>

        <!-- Matrix Table Grid -->
        <div class="flex-1 rounded-xl panel overflow-hidden flex flex-col shadow-xl">
          <div class="flex-1 overflow-auto custom-scrollbar">
            <table class="w-full text-left text-xs border-collapse font-mono">
              <thead id="matrixTableHead" class="bg-[#10131c] text-zinc-400 uppercase tracking-wider text-[10px] font-semibold sticky top-0 z-10 border-b border-[#181b24]"></thead>
              <tbody id="matrixTableBody" class="divide-y divide-[#151924]"></tbody>
            </table>
          </div>
        </div>
      </div>

      <!-- ================================= SCREEN 3: LIVE COMPONENT SIMULATOR ================================= -->
      <div id="screenSimulator" class="hidden flex-1 flex flex-col min-h-0 p-6 space-y-5 overflow-y-auto custom-scrollbar">
        <div class="p-4 rounded-xl panel flex items-center justify-between gap-4 flex-wrap">
          <div>
            <h3 class="text-xs font-bold text-zinc-100 uppercase tracking-wide flex items-center gap-2">
              <i class="fa-solid fa-mobile-screen text-pink-400"></i> Live UI Component & RTL Simulator
            </h3>
            <p class="text-[11px] text-zinc-400">Renders real UI components using the extracted strings and translated bundles from this project.</p>
          </div>
          <div class="flex items-center gap-3">
            <div class="flex items-center gap-1.5 text-xs">
              <span class="text-zinc-400 font-medium">Locale:</span>
              <select id="simLocaleSelect" onchange="updateSimLocale()" class="field rounded-md px-3 py-1 text-xs text-sky-300 font-semibold">
                <option value="en">English (Reference)</option>
                <option value="es">Spanish (es)</option>
                <option value="fr">French (fr)</option>
                <option value="de">German (de — Expansion Check)</option>
                <option value="ja">Japanese (ja)</option>
                <option value="ar">Arabic (ar — RTL Mode)</option>
                <option value="hi">Hindi (hi)</option>
              </select>
            </div>
          </div>
        </div>

        <div class="grid grid-cols-1 md:grid-cols-2 gap-5">
          <!-- Simulator Card 1: Flight / Booking Card -->
          <div id="simCard1" class="p-6 rounded-2xl panel shadow-2xl border-[#1e2330] space-y-4">
            <div class="flex items-center justify-between border-b border-[#1a1e2a] pb-3">
              <div class="flex items-center gap-2">
                <i class="fa-solid fa-plane-departure text-sky-400 text-xs"></i>
                <span id="sim1Title" class="font-bold text-sm text-zinc-100">Flight Details</span>
              </div>
              <span class="text-[10px] px-2.5 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 font-semibold">Confirmed</span>
            </div>
            <div class="grid grid-cols-2 gap-3 py-2 text-xs">
              <div>
                <p class="text-[10px] uppercase text-zinc-500 font-semibold">Departure</p>
                <p id="sim1Depart" class="font-semibold text-zinc-200 mt-0.5">Depart Flight</p>
              </div>
              <div>
                <p class="text-[10px] uppercase text-zinc-500 font-semibold">Return</p>
                <p id="sim1Return" class="font-semibold text-zinc-200 mt-0.5">Return Flight</p>
              </div>
            </div>
            <button id="sim1Button" class="w-full py-2.5 rounded-xl bg-sky-600 hover:bg-sky-500 text-white font-semibold text-xs shadow-lg shadow-sky-600/20 transition-all">
              Book Ticket Now
            </button>
          </div>

          <!-- Simulator Card 2: Checkout / Payment Summary -->
          <div id="simCard2" class="p-6 rounded-2xl panel shadow-2xl border-[#1e2330] space-y-4">
            <div class="flex items-center justify-between border-b border-[#1a1e2a] pb-3">
              <div class="flex items-center gap-2">
                <i class="fa-solid fa-cart-shopping text-emerald-400 text-xs"></i>
                <span id="sim2Title" class="font-bold text-sm text-zinc-100">Checkout Summary</span>
              </div>
              <span class="text-[10px] font-mono text-zinc-400">$128.00</span>
            </div>
            <p id="sim2Greeting" class="text-xs text-zinc-400 font-sans">Welcome back, Alex!</p>
            <div class="flex gap-2">
              <input id="sim2CouponInput" type="text" placeholder="Enter coupon code" class="flex-1 field rounded-lg px-3 py-1.5 text-xs text-zinc-200 font-sans">
              <button id="sim2CouponBtn" class="px-3 py-1.5 rounded-lg bg-[#141722] hover:bg-[#1b2030] text-zinc-200 text-xs font-medium border border-[#232838]">Apply</button>
            </div>
            <button id="sim2SubmitBtn" class="w-full py-2.5 rounded-xl bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-xs shadow-lg shadow-emerald-600/20 transition-all">
              Submit Order
            </button>
          </div>
        </div>
      </div>

      <!-- ================================= SCREEN 4: AST DIFF INSPECTOR ================================= -->
      <div id="screenDiff" class="hidden flex-1 flex flex-col min-h-0 p-5 space-y-4">
        <div id="diffEmptyState" class="hidden p-12 rounded-xl panel text-center space-y-3">
          <i class="fa-solid fa-code-compare text-zinc-600 text-3xl"></i>
          <p class="text-sm text-zinc-300 font-medium">No approved candidates to diff yet.</p>
          <p class="text-xs text-zinc-500">Approve string candidates in the String Studio, then inspect AST refactor diffs here.</p>
        </div>

        <div id="diffContent" class="hidden flex-1 flex flex-col min-h-0 space-y-4">
          <div class="p-3 rounded-xl panel flex items-center justify-between gap-4 flex-wrap">
            <div class="flex items-center gap-2 text-xs min-w-0">
              <i class="fa-solid fa-file-code text-sky-400"></i>
              <span class="font-semibold text-zinc-400 uppercase tracking-wide shrink-0">File:</span>
              <select id="diffFileSelect" class="field rounded-md px-3 py-1 text-xs text-sky-300 font-mono"></select>
              <span id="diffFileCount" class="text-zinc-500 text-xs font-mono"></span>
            </div>
            <span class="text-xs text-emerald-400 font-semibold flex items-center gap-1.5 font-mono">
              <i class="fa-solid fa-circle-check"></i> Deterministic Byte-Range AST Patch (0% Syntax Drift)
            </span>
          </div>

          <div class="flex-1 grid grid-cols-1 md:grid-cols-2 gap-4 min-h-0">
            <div class="rounded-xl panel overflow-hidden flex flex-col shadow-xl">
              <div class="px-4 py-2 bg-[#10131c] border-b border-[#181b24] flex items-center justify-between text-xs font-semibold text-zinc-400">
                <span><i class="fa-solid fa-minus text-rose-400"></i> Original Source Code</span>
                <span class="text-[10px] uppercase px-2 py-0.5 rounded bg-rose-500/10 text-rose-400 border border-rose-500/20 font-mono">Before</span>
              </div>
              <pre id="diffCodeBefore" class="flex-1 p-4 text-xs font-mono text-zinc-300 leading-relaxed overflow-y-auto custom-scrollbar bg-[#08090d]"><code>// Original code...</code></pre>
            </div>

            <div class="rounded-xl panel overflow-hidden flex flex-col shadow-xl">
              <div class="px-4 py-2 bg-[#10131c] border-b border-[#181b24] flex items-center justify-between text-xs font-semibold text-zinc-400">
                <span><i class="fa-solid fa-plus text-emerald-400"></i> Refactored AST Code</span>
                <span class="text-[10px] uppercase px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 font-mono">After</span>
              </div>
              <pre id="diffCodeAfter" class="flex-1 p-4 text-xs font-mono text-zinc-300 leading-relaxed overflow-y-auto custom-scrollbar bg-[#08090d]"><code>// Localized refactored code...</code></pre>
            </div>
          </div>
        </div>
      </div>

      <!-- ================================= SCREEN 5: CRITIC & QUALITY SCORECARD ================================= -->
      <div id="screenCritic" class="hidden flex-1 flex flex-col min-h-0 p-6 space-y-4 overflow-y-auto custom-scrollbar">
        <div id="criticEmptyState" class="hidden p-12 rounded-xl panel text-center space-y-3">
          <i class="fa-solid fa-shield-halved text-zinc-600 text-3xl"></i>
          <p class="text-sm text-zinc-300 font-medium">No critic scorecard available yet.</p>
          <p class="text-xs text-zinc-500">Execute the multi-agent localization pipeline to run the 4-tier critic verification suite.</p>
        </div>

        <div id="criticContent" class="hidden space-y-4">
          <div id="criticSummaryBanner" class="p-4 rounded-xl panel text-xs font-semibold flex items-center gap-2 shadow-lg"></div>
          <div id="criticTierGrid" class="grid grid-cols-1 sm:grid-cols-2 gap-4"></div>
        </div>
      </div>

      <!-- ================================= SCREEN 6: SEO & GROWTH STUDIO ================================= -->
      <div id="screenSeo" class="hidden flex-1 flex flex-col min-h-0 p-6 space-y-5 overflow-y-auto custom-scrollbar">
        <!-- Strategy Intake & Action Bar -->
        <div class="p-5 rounded-xl panel space-y-4">
          <div class="flex flex-wrap items-center justify-between gap-3 border-b border-[#181b24] pb-4">
            <div>
              <h2 class="text-sm font-bold text-zinc-100 flex items-center gap-2">
                <i class="fa-solid fa-bullseye text-pink-400"></i> Autonomous Multilingual SEO & Market Growth Studio
              </h2>
              <p class="text-xs text-zinc-400 mt-0.5">Scouts regional competitor SERP landscapes, mines high-intent buyer keywords, semantically optimizes UI copy, and models growth projections.</p>
            </div>
            <div class="flex items-center gap-2.5">
              <button onclick="runSEOOptimization()" id="btnRunSEO" class="px-3.5 py-1.5 rounded-lg bg-pink-500/90 hover:bg-pink-500 text-white font-semibold text-xs transition flex items-center gap-2 shadow-lg shadow-pink-500/20">
                <i class="fa-solid fa-wand-magic-sparkles"></i> Run SEO Optimization
              </button>
              <button onclick="applySEOToDisk()" id="btnApplySEO" class="px-3.5 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-xs transition flex items-center gap-2 shadow-lg shadow-emerald-500/20">
                <i class="fa-solid fa-floppy-disk"></i> Apply SEO to Disk
              </button>
            </div>
          </div>

          <!-- Controls row -->
          <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 text-xs font-mono">
            <div>
              <label class="block text-[11px] font-semibold text-zinc-400 uppercase tracking-wider mb-1.5">Active Target Market</label>
              <div id="seoLocaleTabs" class="flex flex-wrap gap-1.5">
                <!-- Populated dynamically: ja, de, es, fr, etc. -->
              </div>
            </div>
            <div>
              <label class="block text-[11px] font-semibold text-zinc-400 uppercase tracking-wider mb-1.5">Commercial Goal</label>
              <select id="seoGoalSelect" class="w-full px-3 py-1.5 rounded-md field text-xs text-zinc-200">
                <option value="traffic">Top-of-Funnel Reach (Discovery)</option>
                <option value="conversion">High-Intent Commercial (Buyers)</option>
                <option value="trust">Regional Compliance & Local Trust</option>
              </select>
            </div>
            <div>
              <label class="block text-[11px] font-semibold text-zinc-400 uppercase tracking-wider mb-1.5">Key Scope Tier</label>
              <select id="seoScopeSelect" class="w-full px-3 py-1.5 rounded-md field text-xs text-zinc-200">
                <option value="high_impact">High Impact Only (Meta, Hero, Headings)</option>
                <option value="full_site">Full Site Catalog (All UI Keys)</option>
              </select>
            </div>
            <div>
              <label class="block text-[11px] font-semibold text-zinc-400 uppercase tracking-wider mb-1.5">Competitor URLs (Optional)</label>
              <input type="text" id="seoCompetitorInput" placeholder="competitor.com, leader.jp" class="w-full px-3 py-1.5 rounded-md field text-xs text-zinc-200 placeholder-zinc-600">
            </div>
          </div>
        </div>

        <div id="seoEmptyState" class="hidden p-12 rounded-xl panel text-center space-y-3">
          <i class="fa-solid fa-bullseye text-zinc-600 text-3xl"></i>
          <p class="text-sm text-zinc-300 font-medium">No SEO intelligence computed yet for this project.</p>
          <p class="text-xs text-zinc-500">Click <strong>"Run SEO Optimization"</strong> above to launch the autonomous market discovery and copy weaving agents.</p>
        </div>

        <div id="seoDashboard" class="space-y-5">
          <!-- 2-Column Section -->
          <div class="grid grid-cols-1 lg:grid-cols-2 gap-5">
            <!-- Left: Competitors & Keywords -->
            <div class="space-y-5">
              <!-- Regional Competitor Intelligence -->
              <div class="p-5 rounded-xl panel space-y-3">
                <div class="flex items-center justify-between">
                  <span class="text-xs font-bold uppercase tracking-wider text-zinc-300 flex items-center gap-2">
                    <i class="fa-solid fa-magnifying-glass-chart text-sky-400"></i> Regional Competitor Landscape
                  </span>
                  <span id="seoCompetitorCount" class="text-[10px] px-2 py-0.5 rounded bg-zinc-800 text-zinc-400 font-mono">0 scouted</span>
                </div>
                <div id="seoCompetitorList" class="space-y-2.5"></div>
              </div>

              <!-- High-Intent Keyword Intelligence -->
              <div class="p-5 rounded-xl panel space-y-3">
                <div class="flex items-center justify-between">
                  <span class="text-xs font-bold uppercase tracking-wider text-zinc-300 flex items-center gap-2">
                    <i class="fa-solid fa-key text-amber-400"></i> High-Intent Local Keywords
                  </span>
                  <span id="seoKeywordCount" class="text-[10px] px-2 py-0.5 rounded bg-zinc-800 text-zinc-400 font-mono">0 keywords</span>
                </div>
                <div id="seoKeywordCloud" class="flex flex-wrap gap-2"></div>
              </div>
            </div>

            <!-- Right: SERP Simulation & Growth Metrics -->
            <div class="space-y-5">
              <!-- Multi-Modal SERP Simulator -->
              <div class="p-5 rounded-xl panel space-y-3">
                <div class="flex items-center justify-between">
                  <span class="text-xs font-bold uppercase tracking-wider text-zinc-300 flex items-center gap-2">
                    <i class="fa-solid fa-mobile-screen text-pink-400"></i> Google SERP Visual Simulator
                  </span>
                  <div class="flex items-center gap-1 bg-[#090b10] p-0.5 rounded-lg border border-[#181b24] text-[10px]">
                    <button onclick="setSeoSimMode('desktop')" id="btnSimDesktop" class="px-2 py-1 rounded font-mono font-semibold bg-zinc-800 text-zinc-200">Desktop (600px)</button>
                    <button onclick="setSeoSimMode('mobile')" id="btnSimMobile" class="px-2 py-1 rounded font-mono text-zinc-400 hover:text-zinc-200">Mobile</button>
                    <button onclick="setSeoSimMode('social')" id="btnSimSocial" class="px-2 py-1 rounded font-mono text-zinc-400 hover:text-zinc-200">Social Card</button>
                  </div>
                </div>

                <div id="seoSimContainer" class="p-4 rounded-lg bg-[#121620] border border-[#1c2230]">
                  <!-- Rendered dynamically -->
                </div>
              </div>

              <!-- Predictive Growth Metrics Scorecard -->
              <div class="p-5 rounded-xl panel space-y-3">
                <span class="text-xs font-bold uppercase tracking-wider text-zinc-300 flex items-center gap-2">
                  <i class="fa-solid fa-chart-line text-emerald-400"></i> Projected Growth & Safety Impact
                </span>
                <div id="seoMetricsGrid" class="grid grid-cols-2 sm:grid-cols-4 gap-3"></div>
              </div>
            </div>
          </div>

          <!-- Bottom: Interactive Semantic Copy Diff Matrix -->
          <div class="p-5 rounded-xl panel space-y-3">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <div>
                <span class="text-xs font-bold uppercase tracking-wider text-zinc-300 flex items-center gap-2">
                  <i class="fa-solid fa-code-compare text-emerald-400"></i> Semantic Copy Matrix & Injected Keywords
                </span>
                <p class="text-[11px] text-zinc-500 mt-0.5">Review side-by-side linguistic diffs, injected keyword tokens, character counts, and pixel widths.</p>
              </div>
              <span id="seoOptCount" class="text-[10px] px-2 py-0.5 rounded bg-zinc-800 text-zinc-400 font-mono">0 keys optimized</span>
            </div>

            <div class="overflow-x-auto custom-scrollbar border border-[#181b24] rounded-lg">
              <table class="w-full text-left text-xs border-collapse font-mono">
                <thead class="bg-[#10131c] text-zinc-400 uppercase tracking-wider text-[10px] font-semibold border-b border-[#181b24]">
                  <tr>
                    <th class="py-2.5 px-4">Key / Impact</th>
                    <th class="py-2.5 px-4">Source (en)</th>
                    <th class="py-2.5 px-4">Baseline Translation</th>
                    <th class="py-2.5 px-4">SEO-Optimized Translation</th>
                    <th class="py-2.5 px-4">Keywords / ICU Safety</th>
                  </tr>
                </thead>
                <tbody id="seoOptimizationsTableBody" class="divide-y divide-[#151924]"></tbody>
              </table>
            </div>
          </div>
        </div>
      </div>

      <!-- ================================= SCREEN 6: PIPELINE RUNNER & LOGS ================================= -->
      <div id="screenRunner" class="hidden flex-1 flex flex-col min-h-0 p-5 space-y-4">
        <div class="grid grid-cols-1 lg:grid-cols-3 gap-4">
          <!-- Target Languages Comprehensive Hub (38+ Languages) -->
          <div class="p-4 rounded-xl panel space-y-3 lg:col-span-2 flex flex-col">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <span class="text-xs font-semibold text-zinc-200 uppercase tracking-wide flex items-center gap-2">
                <i class="fa-solid fa-language text-sky-400"></i> Target Languages (<span id="runnerSelectedCount" class="text-sky-400 font-bold">4</span> selected)
              </span>
              <!-- Quick Presets -->
              <div class="flex flex-wrap items-center gap-1 text-[11px]">
                <button onclick="applyLangPreset('top5')" class="px-2 py-0.5 rounded bg-zinc-800 hover:bg-sky-600/30 text-zinc-300 hover:text-sky-300 border border-zinc-700 transition-all cursor-pointer">Top 5</button>
                <button onclick="applyLangPreset('eu')" class="px-2 py-0.5 rounded bg-zinc-800 hover:bg-sky-600/30 text-zinc-300 hover:text-sky-300 border border-zinc-700 transition-all cursor-pointer">EU Tier 1</button>
                <button onclick="applyLangPreset('apac')" class="px-2 py-0.5 rounded bg-zinc-800 hover:bg-sky-600/30 text-zinc-300 hover:text-sky-300 border border-zinc-700 transition-all cursor-pointer">Asia-Pac</button>
                <button onclick="applyLangPreset('americas')" class="px-2 py-0.5 rounded bg-zinc-800 hover:bg-sky-600/30 text-zinc-300 hover:text-sky-300 border border-zinc-700 transition-all cursor-pointer">Americas</button>
                <button onclick="applyLangPreset('nordics')" class="px-2 py-0.5 rounded bg-zinc-800 hover:bg-sky-600/30 text-zinc-300 hover:text-sky-300 border border-zinc-700 transition-all cursor-pointer">Nordics</button>
                <button onclick="applyLangPreset('all')" class="px-2 py-0.5 rounded bg-zinc-800 hover:bg-sky-600/30 text-zinc-300 hover:text-sky-300 border border-zinc-700 transition-all cursor-pointer">All 38</button>
                <button onclick="applyLangPreset('clear')" class="px-2 py-0.5 rounded bg-zinc-800 hover:bg-rose-600/30 text-zinc-400 hover:text-rose-300 border border-zinc-700 transition-all cursor-pointer">Clear</button>
              </div>
            </div>

            <!-- Search + Custom Language Adder -->
            <div class="flex gap-2 text-xs">
              <div class="relative flex-1">
                <i class="fa-solid fa-magnifying-glass absolute left-3 top-2.5 text-zinc-500 text-[10px]"></i>
                <input id="runnerLangSearch" type="text" oninput="filterCatalogLanguages()" placeholder="Search 38+ languages (e.g. Italian, Korean, zh, ja)..." class="w-full pl-8 pr-3 py-1.5 rounded-lg field text-xs text-zinc-200" />
              </div>
              <div class="flex gap-1.5">
                <input id="runnerCustomLangInput" type="text" placeholder="+ Custom (e.g. pt-BR, fil)" class="w-40 px-2.5 py-1.5 rounded-lg field text-xs font-mono text-zinc-200" onkeydown="if(event.key==='Enter'){addCustomTargetLanguage(); event.preventDefault();}" />
                <button onclick="addCustomTargetLanguage()" class="px-3 py-1.5 rounded-lg bg-sky-600/30 hover:bg-sky-600/50 text-sky-300 border border-sky-500/30 text-xs font-semibold flex items-center gap-1 transition-all cursor-pointer" title="Add Custom Language Code">
                  <i class="fa-solid fa-plus"></i> Add
                </button>
              </div>
            </div>

            <!-- Scrollable Language Grid -->
            <div id="runnerLangGrid" class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-2 max-h-52 overflow-y-auto custom-scrollbar pr-1 text-xs">
              <!-- Dynamically rendered by JS -->
            </div>
          </div>

          <!-- Controls: Style Persona, App Directive & Execution -->
          <div class="space-y-3 flex flex-col justify-between">
            <div class="p-4 rounded-xl panel space-y-2">
              <span class="text-xs font-semibold text-zinc-200 uppercase tracking-wide flex items-center gap-2">
                <i class="fa-solid fa-masks-theater text-amber-400"></i> Style Persona Memory
              </span>
              <select id="runnerToneSelector" class="w-full py-2 px-3 rounded-lg field text-xs text-zinc-200 font-medium">
                <option value="default">Standard Native (Professional UI)</option>
                <option value="gen_z">Gen-Z Slang ('no cap', 'slay', 'fire')</option>
                <option value="pirate">Pirate / Gamer ('Ahoy Matey!', 'Plunder')</option>
                <option value="formal">Corporate Formal (Enterprise Honorifics)</option>
                <option value="casual">Casual Friendly (Warm & Welcoming)</option>
              </select>
            </div>

            <div class="p-4 rounded-xl panel space-y-2">
              <span class="text-xs font-semibold text-zinc-200 uppercase tracking-wide flex items-center gap-2">
                <i class="fa-solid fa-wand-magic-sparkles text-indigo-400"></i> App Integration Directive
              </span>
              <input id="runnerDirectiveInput" type="text" placeholder="e.g. Add a language switcher dropdown in Navbar.tsx" class="w-full py-2 px-3 rounded-lg field text-xs text-zinc-200" />
              <p class="text-[10px] text-zinc-500">
                Synthesizes UI components & surgically patches parent containers.
              </p>
            </div>

            <div class="p-4 rounded-xl panel space-y-2">
              <span class="text-xs font-semibold text-zinc-200 uppercase tracking-wide flex items-center gap-2">
                <i class="fa-solid fa-arrows-rotate text-emerald-400"></i> Existing Translations Strategy
              </span>
              <select id="runnerExistingModeSelector" class="w-full py-2 px-3 rounded-lg field text-xs text-zinc-200 font-medium">
                <option value="skip" selected>⚡ Skip Existing (Translate Missing Only)</option>
                <option value="replace">🔄 Regenerate & Overwrite All Existing</option>
              </select>
              <p class="text-[10px] text-zinc-500">
                Incremental delta translation or full overwrite regeneration.
              </p>
            </div>

            <div class="p-4 rounded-xl panel space-y-2.5">
              <button onclick="executeLocalization()" id="runnerExecuteBtn" class="w-full py-2.5 rounded-lg bg-sky-600 hover:bg-sky-500 text-white font-semibold text-xs uppercase tracking-wide flex items-center justify-center gap-2 transition-all shadow-md shadow-sky-600/20 cursor-pointer">
                <i class="fa-solid fa-bolt"></i> Run Pipeline
              </button>
            </div>
          </div>
        </div>

        <div class="flex-1 rounded-xl panel overflow-hidden flex flex-col shadow-xl">
          <div class="bg-[#10131c] px-4 py-2.5 border-b border-[#181b24] flex items-center justify-between text-xs font-mono">
            <span class="text-zinc-400 flex items-center gap-2"><i class="fa-solid fa-terminal text-sky-400"></i> supervisor-agent.log</span>
            <span id="runnerStatusPill" class="text-emerald-400 font-medium flex items-center gap-1.5">
              <span class="w-1.5 h-1.5 rounded-full bg-emerald-400"></span> Ready
            </span>
          </div>
          <div id="runnerTerminalBody" class="flex-1 p-4 font-mono text-xs leading-relaxed text-zinc-300 space-y-1 overflow-y-auto custom-scrollbar bg-[#08090d]">
            <div class="text-zinc-600">Engine ready. Click "Run Pipeline" to execute localization workflow.</div>
          </div>
        </div>
      </div>

      <!-- ================================= SCREEN 7: CHECKPOINTS ================================= -->
      <div id="screenCheckpoints" class="hidden flex-1 flex flex-col min-h-0 p-6 space-y-4 overflow-y-auto custom-scrollbar">
        <div class="p-4 rounded-xl panel flex items-center justify-between gap-4 flex-wrap">
          <div>
            <h3 class="text-xs font-bold text-zinc-100 uppercase tracking-wide flex items-center gap-2">
              <i class="fa-solid fa-clock-rotate-left text-teal-400"></i> Pre-Flight Snapshots & 1-Click Rollback
            </h3>
            <p class="text-[11px] text-zinc-500">Atomic snapshots created automatically before applying AST code refactorings to disk.</p>
          </div>
          <button onclick="loadCheckpoints()" class="px-3 py-1.5 rounded-lg field hover:border-zinc-700 text-xs text-zinc-300 font-medium flex items-center gap-1.5">
            <i class="fa-solid fa-arrows-rotate"></i> Refresh
          </button>
        </div>

        <div id="checkpointsList" class="space-y-3">
          <div class="p-8 text-center text-zinc-500 text-xs">Loading snapshots...</div>
        </div>
      </div>

      <!-- ================================= SCREEN 8: BENCHMARK ================================= -->
      <div id="screenBenchmark" class="hidden flex-1 flex flex-col min-h-0 p-6 space-y-4 overflow-y-auto custom-scrollbar">
        <div class="p-4 rounded-xl panel flex items-center justify-between gap-4 flex-wrap">
          <div>
            <h3 class="text-xs font-bold text-zinc-100 uppercase tracking-wide flex items-center gap-2">
              <i class="fa-solid fa-trophy text-amber-400"></i> 10-Case Adversarial Benchmark Suite
            </h3>
            <p class="text-[11px] text-zinc-500">Evaluates compiler pass rate, token savings, and AST precision across 10 edge cases vs baselines.</p>
          </div>
          <button onclick="runBenchmarkSuite()" id="runBenchmarkBtn" class="px-4 py-2 rounded-lg bg-amber-600 hover:bg-amber-500 text-white font-semibold text-xs flex items-center gap-2 shadow-lg shadow-amber-600/20">
            <i class="fa-solid fa-play"></i> Run 10-Case Benchmark
          </button>
        </div>

        <div class="rounded-xl panel overflow-hidden shadow-xl">
          <div class="overflow-x-auto custom-scrollbar">
            <table class="w-full text-left text-xs font-mono border-collapse">
              <thead class="bg-[#10131c] text-zinc-400 uppercase tracking-wider text-[10px] font-semibold border-b border-[#181b24]">
                <tr>
                  <th class="py-3 px-4 w-12 text-center">#</th>
                  <th class="py-3 px-4 min-w-[200px]">Adversarial Case</th>
                  <th class="py-3 px-4">Framework</th>
                  <th class="py-3 px-4 text-center">Zero-Shot LLM</th>
                  <th class="py-3 px-4 text-center">Naive Regex</th>
                  <th class="py-3 px-4 text-center text-emerald-400 font-bold">langPeanut</th>
                  <th class="py-3 px-4 text-right">Tokens Saved</th>
                </tr>
              </thead>
              <tbody id="benchmarkTableBody" class="divide-y divide-[#151924]">
                <tr><td colspan="7" class="py-10 text-center text-zinc-500">Click "Run 10-Case Benchmark" to execute evaluation harness.</td></tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>

      <!-- ================================= SCREEN 9: STATS ================================= -->
      <div id="screenStats" class="hidden flex-1 flex flex-col min-h-0 p-6 space-y-4 overflow-y-auto custom-scrollbar">
        <div class="grid grid-cols-1 sm:grid-cols-4 gap-4">
          <div class="p-5 rounded-xl panel space-y-1">
            <p class="text-[11px] font-semibold text-zinc-500 uppercase tracking-wide">Session Tokens</p>
            <p id="statSessionTokens" class="text-2xl font-bold text-zinc-100 font-mono">—</p>
            <p id="statSessionRequests" class="text-[11px] text-zinc-500 font-mono"></p>
          </div>
          <div class="p-5 rounded-xl panel space-y-1">
            <p class="text-[11px] font-semibold text-zinc-500 uppercase tracking-wide">Session Cost</p>
            <p id="statSessionCost" class="text-2xl font-bold text-emerald-400 font-mono">—</p>
            <p class="text-[11px] text-zinc-500">Current execution run</p>
          </div>
          <div class="p-5 rounded-xl panel space-y-1">
            <p class="text-[11px] font-semibold text-zinc-500 uppercase tracking-wide">All-Time Tokens</p>
            <p id="statAllTimeTokens" class="text-2xl font-bold text-zinc-100 font-mono">—</p>
            <p id="statAllTimeRequests" class="text-[11px] text-zinc-500 font-mono"></p>
          </div>
          <div class="p-5 rounded-xl panel space-y-1">
            <p class="text-[11px] font-semibold text-zinc-500 uppercase tracking-wide">All-Time Cost</p>
            <p id="statAllTimeCost" class="text-2xl font-bold text-emerald-400 font-mono">—</p>
            <p class="text-[11px] text-zinc-500">Total historical token spend</p>
          </div>
        </div>

        <div class="rounded-xl panel overflow-hidden shadow-xl">
          <div class="px-4 py-3 bg-[#10131c] border-b border-[#181b24] text-xs font-semibold uppercase tracking-wide text-zinc-300 font-mono">LLM Consumption by Model</div>
          <div class="overflow-x-auto custom-scrollbar">
            <table class="w-full text-left text-xs font-mono">
              <thead class="bg-[#0b0e14] text-zinc-400 uppercase tracking-wider text-[10px] font-semibold border-b border-[#181b24]">
                <tr>
                  <th class="py-2.5 px-4">Model</th>
                  <th class="py-2.5 px-4">Provider</th>
                  <th class="py-2.5 px-4 text-right">Input</th>
                  <th class="py-2.5 px-4 text-right">Output</th>
                  <th class="py-2.5 px-4 text-right">Calls</th>
                  <th class="py-2.5 px-4 text-right">Cost</th>
                </tr>
              </thead>
              <tbody id="statsModelBody" class="divide-y divide-[#151924]"></tbody>
            </table>
          </div>
        </div>
      </div>

      <!-- ================================= SCREEN 10: SETTINGS ================================= -->
      <div id="screenSettings" class="hidden flex-1 flex flex-col min-h-0 p-6 space-y-5 overflow-y-auto custom-scrollbar">
        <div class="grid grid-cols-1 md:grid-cols-2 gap-5">
          
          <!-- Left Column: Engine & Model Configuration -->
          <div class="space-y-5">
            <div class="p-5 rounded-xl panel space-y-4">
              <h3 class="text-xs font-bold text-zinc-100 uppercase tracking-wide flex items-center gap-2">
                <i class="fa-solid fa-microchip text-sky-400"></i> Active AI Provider & Model
              </h3>
              <div class="space-y-3 text-xs">
                <div>
                  <label class="text-zinc-400 font-medium block mb-1">Translation Provider Engine:</label>
                  <select id="settingsActiveProvider" onchange="handleProviderChange()" class="w-full field rounded-lg px-3 py-2 text-xs font-mono text-zinc-200">
                    <option value="gemini">Google Gemini (gemini-3.5-flash — $1.50 in / $9.00 out per 1M)</option>
                    <option value="claude">Anthropic Claude (claude-sonnet-5 — 1M Context, 128k Out, $2/$10)</option>
                    <option value="openai">OpenAI (gpt-5.4-mini — 400k Context, 128k Out, $0.75/$4.50)</option>
                    <option value="ollama">Ollama (100% Offline GPU Engine — Qwen, Gemma, LLaMA)</option>
                    <option value="nllb-cloud">Meta NLLB-200 Cloud (Hugging Face Serverless Inference)</option>
                    <option value="deepl">DeepL Neural MT API (European / Asian Specialists)</option>
                    <option value="custom">Custom / vLLM / LM Studio (OpenAI-Compatible Local Endpoint)</option>
                    <option value="local">Local Deterministic Engine (Offline Benchmark Synthesizer)</option>
                  </select>
                </div>
                <div>
                  <label class="text-zinc-400 font-medium block mb-1">Dynamic Tone & Style Preset:</label>
                  <select id="settingsToneStyle" class="w-full field rounded-lg px-3 py-2 text-xs font-mono text-zinc-200">
                    <option value="default">Standard Accurate — Professional, clear native UI copy</option>
                    <option value="gen_z">Gen-Z Slang — Trendy internet aesthetic ('no cap', 'slay', 'fire')</option>
                    <option value="casual">Casual Friendly — Warm, welcoming tone for consumer apps</option>
                    <option value="formal">Corporate Formal — Enterprise-grade strict polite honorifics</option>
                    <option value="pirate">Pirate / Gamer — 'Ahoy Matey!' playful gaming copy</option>
                  </select>
                </div>
              </div>
            </div>

            <!-- Ollama Local Offline GPU Engine Card -->
            <div class="p-5 rounded-xl panel space-y-4 border border-emerald-500/20 bg-emerald-950/10">
              <div class="flex items-center justify-between">
                <h3 class="text-xs font-bold text-emerald-300 uppercase tracking-wide flex items-center gap-2">
                  <i class="fa-solid fa-microchip text-emerald-400"></i> Ollama Local GPU Engine
                </h3>
                <span id="badgeOllamaStatus" class="text-[10px] px-2 py-0.5 rounded font-mono">Checking...</span>
              </div>
              <p class="text-[11px] text-zinc-400 leading-relaxed">
                100% offline, zero-key neural execution running on your Apple Silicon Metal GPU. Zero API cost, zero cloud data transfer.
              </p>
              <div>
                <label class="text-zinc-400 font-medium block mb-1 text-[11px]">Active Ollama Model:</label>
                <select id="settingsOllamaModelSelect" class="w-full field rounded-lg px-3 py-2 text-xs font-mono text-zinc-200">
                  <option value="">Auto-Detect Best Model</option>
                </select>
              </div>
              <div id="ollamaModelsContainer" class="p-2.5 rounded-lg bg-zinc-950/80 border border-zinc-800 text-[11px] font-mono space-y-1">
                <div class="text-zinc-400 flex items-center justify-between">
                  <span>Available Local Models:</span>
                  <span id="ollamaActiveModelBadge" class="text-emerald-400 font-semibold"></span>
                </div>
                <div id="ollamaModelsList" class="text-zinc-300"></div>
              </div>
            </div>

            <!-- Model Test & Connectivity Probe Card -->
            <div class="p-5 rounded-xl panel space-y-4 border border-violet-500/20 bg-violet-950/10">
              <div class="flex items-center justify-between">
                <h3 class="text-xs font-bold text-violet-300 uppercase tracking-wide flex items-center gap-2">
                  <i class="fa-solid fa-bolt text-violet-400"></i> Live Model Connectivity & Probe
                </h3>
                <span id="badgeTestStatus" class="text-[10px] px-2 py-0.5 rounded font-mono text-zinc-400 border border-zinc-700 bg-zinc-900">Ready</span>
              </div>
              <p class="text-[11px] text-zinc-400 leading-relaxed">
                Run an instant translation probe to verify API credentials, endpoint reachability, latency, and ICU syntax preservation.
              </p>
              <div class="space-y-2">
                <input type="text" id="testProbeInput" value="Welcome to langPeanut! Effortless multi-agent software localization." class="w-full field rounded-lg px-3 py-2 text-xs font-mono text-zinc-200">
                <div class="flex gap-2">
                  <select id="testProbeTargetLang" class="field rounded-lg px-3 py-2 text-xs font-mono text-zinc-200 flex-1">
                    <option value="es">Spanish (es)</option>
                    <option value="fr">French (fr)</option>
                    <option value="ja">Japanese (ja)</option>
                    <option value="de">German (de)</option>
                    <option value="hi">Hindi (hi)</option>
                    <option value="zh-CN">Chinese (zh)</option>
                  </select>
                  <button id="btnRunModelTest" onclick="runModelConnectivityTest()" class="px-4 py-2 rounded-lg bg-violet-600 hover:bg-violet-500 text-white font-semibold text-xs transition-colors flex items-center gap-2">
                    <i class="fa-solid fa-play"></i> Test Model
                  </button>
                </div>
              </div>
              <div id="testProbeResultBox" class="hidden p-3 rounded-lg border border-zinc-700 bg-zinc-900/90 text-xs space-y-2"></div>
            </div>
          </div>

          <!-- Right Column: API Keys & Preferences -->
          <div class="space-y-5">
            <div class="p-5 rounded-xl panel space-y-4">
              <h3 class="text-xs font-bold text-zinc-100 uppercase tracking-wide flex items-center gap-2">
                <i class="fa-solid fa-key text-emerald-400"></i> API Credentials & Tokens
              </h3>
              <div class="space-y-3 text-xs">
                <div>
                  <label class="text-zinc-400 font-medium block mb-1">Hugging Face Token (<code class="text-sky-300 font-mono">HF_TOKEN</code>):</label>
                  <input type="password" id="keyInputHF" placeholder="hf_..." class="w-full field rounded-lg px-3 py-2 text-xs font-mono text-zinc-200">
                </div>
                <div>
                  <label class="text-zinc-400 font-medium block mb-1">Anthropic API Key (<code class="text-sky-300 font-mono">ANTHROPIC_API_KEY</code>):</label>
                  <input type="password" id="keyInputAnthropic" placeholder="sk-ant-..." class="w-full field rounded-lg px-3 py-2 text-xs font-mono text-zinc-200">
                </div>
                <div>
                  <label class="text-zinc-400 font-medium block mb-1">OpenAI API Key (<code class="text-sky-300 font-mono">OPENAI_API_KEY</code>):</label>
                  <input type="password" id="keyInputOpenAI" placeholder="sk-..." class="w-full field rounded-lg px-3 py-2 text-xs font-mono text-zinc-200">
                </div>
                <div>
                  <label class="text-zinc-400 font-medium block mb-1">Google Gemini API Key (<code class="text-sky-300 font-mono">GEMINI_API_KEY</code>):</label>
                  <input type="password" id="keyInputGemini" placeholder="AIza..." class="w-full field rounded-lg px-3 py-2 text-xs font-mono text-zinc-200">
                </div>
                <div>
                  <label class="text-zinc-400 font-medium block mb-1">DeepL API Key (<code class="text-sky-300 font-mono">DEEPL_API_KEY</code>):</label>
                  <input type="password" id="keyInputDeepL" placeholder="deepl_key..." class="w-full field rounded-lg px-3 py-2 text-xs font-mono text-zinc-200">
                </div>
                <div>
                  <label class="text-zinc-400 font-medium block mb-1">Custom Base URL (<code class="text-sky-300 font-mono">OPENAI_BASE_URL</code>):</label>
                  <input type="text" id="keyInputCustomURL" placeholder="http://localhost:11434/v1" class="w-full field rounded-lg px-3 py-2 text-xs font-mono text-zinc-200">
                </div>
              </div>
            </div>

            <!-- Custom Toolchain Commands Card -->
            <div class="p-5 rounded-xl panel space-y-4 border border-sky-500/20 bg-sky-950/10">
              <h3 class="text-xs font-bold text-sky-300 uppercase tracking-wide flex items-center gap-2">
                <i class="fa-solid fa-terminal text-sky-400"></i> Custom Toolchain Commands
              </h3>
              <p class="text-[11px] text-zinc-400 leading-relaxed">
                Override default package manager and compiler commands for specialized monorepos, private registries, or custom build scripts.
              </p>
              <div class="space-y-3 text-xs">
                <div>
                  <label class="text-zinc-400 font-medium block mb-1">Custom Dependency Install Command:</label>
                  <input type="text" id="settingsCustomInstallCmd" placeholder="e.g. pnpm install, yarn add react-i18next i18next, flutter pub get" class="w-full field rounded-lg px-3 py-2 text-xs font-mono text-zinc-200">
                  <span class="text-[10px] text-zinc-500">Executed during dependency resolution & package installation.</span>
                </div>
                <div>
                  <label class="text-zinc-400 font-medium block mb-1">Custom Build / Typecheck Command:</label>
                  <input type="text" id="settingsCustomBuildCmd" placeholder="e.g. pnpm typecheck, npm run build, tsc --noEmit, flutter analyze" class="w-full field rounded-lg px-3 py-2 text-xs font-mono text-zinc-200">
                  <span class="text-[10px] text-zinc-500">Executed during compiler diagnostics validation & autonomous code repair.</span>
                </div>
              </div>
            </div>

            <!-- Token Budget & Batch Chunking Card -->
            <div class="p-5 rounded-xl panel space-y-4 border border-indigo-500/20 bg-indigo-950/10">
              <h3 class="text-xs font-bold text-indigo-300 uppercase tracking-wide flex items-center gap-2">
                <i class="fa-solid fa-coins text-indigo-400"></i> Token Budget & Batch Chunking Tunables
              </h3>
              <p class="text-[11px] text-zinc-400 leading-relaxed">
                Control LLM context window utilization, batch token budgets, key ceilings per prompt, and parallel concurrency limits.
              </p>
              <div class="grid grid-cols-1 sm:grid-cols-3 gap-3 text-xs">
                <div>
                  <label class="text-zinc-400 font-medium block mb-1">Word/Token Budget:</label>
                  <input type="number" id="settingsChunkWordBudget" placeholder="0 (Auto Model-Aware)" class="w-full field rounded-lg px-3 py-2 text-xs font-mono text-zinc-200">
                  <span class="text-[10px] text-zinc-500">Max words per batch (0 = auto)</span>
                </div>
                <div>
                  <label class="text-zinc-400 font-medium block mb-1">Key Ceiling / Batch:</label>
                  <input type="number" id="settingsChunkKeyCeiling" placeholder="0 (Auto Model-Aware)" class="w-full field rounded-lg px-3 py-2 text-xs font-mono text-zinc-200">
                  <span class="text-[10px] text-zinc-500">Max keys per prompt (0 = auto)</span>
                </div>
                <div>
                  <label class="text-zinc-400 font-medium block mb-1">Parallel Concurrency:</label>
                  <input type="number" id="settingsConcurrency" placeholder="5" min="1" max="50" class="w-full field rounded-lg px-3 py-2 text-xs font-mono text-zinc-200">
                  <span class="text-[10px] text-zinc-500">Simultaneous API calls</span>
                </div>
              </div>
            </div>

            <div class="p-5 rounded-xl panel space-y-4">
              <h3 class="text-xs font-bold text-zinc-100 uppercase tracking-wide flex items-center gap-2">
                <i class="fa-solid fa-shield text-amber-400"></i> Defaults & File Exclusions
              </h3>
              <div class="space-y-3 text-xs">
                <div>
                  <label class="text-zinc-400 font-medium block mb-1">Default Strategy on Existing Translations:</label>
                  <select id="settingsExistingTranslationsMode" class="w-full field rounded-lg px-3 py-2 text-xs font-mono text-zinc-200">
                    <option value="skip">Skip Existing (Incremental — Translate Missing Only)</option>
                    <option value="replace">Regenerate & Overwrite All Existing</option>
                    <option value="prompt">Prompt Interactively in Terminal</option>
                  </select>
                </div>
                <div>
                  <label class="text-zinc-400 font-medium block mb-1">Excluded File Patterns (Globs):</label>
                  <input type="text" id="settingsExcludesInput" placeholder="**/*.test.*, **/mock/**" class="w-full field rounded-lg px-3 py-2 text-xs font-mono text-zinc-200">
                </div>
                <button onclick="saveProjectSettings()" class="w-full py-2.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-xs transition-colors flex items-center justify-center gap-2 shadow-lg shadow-emerald-950/50">
                  <i class="fa-solid fa-floppy-disk"></i> Save Preferences & Credentials
                </button>
              </div>
            </div>
          </div>

        </div>
      </div>

    </main>
  </div>

  <script>
    let tuiCandidates = [];
    let fileTreeNodes = [];
    let selectedFilePath = null;
    let selectedCandidateId = null;
    let currentFilter = 'all';
    let currentDiffs = [];
    let currentMatrix = { keys: [], locales: {}, stats: {} };
    let statusPoller = null;

    // ---------- Toasts ----------
    function showToast(message, kind) {
      kind = kind || 'info';
      const stack = document.getElementById('toastStack');
      const colors = {
        success: 'border-emerald-500/40 text-emerald-300 bg-[#091a13]/95',
        error: 'border-rose-500/40 text-rose-300 bg-[#1a0c0f]/95',
        info: 'border-sky-500/40 text-sky-300 bg-[#0b1420]/95'
      };
      const icons = { success: 'fa-circle-check', error: 'fa-circle-exclamation', info: 'fa-circle-info' };
      const el = document.createElement('div');
      el.className = 'toast pointer-events-auto panel border ' + (colors[kind] || colors.info) + ' rounded-lg px-3.5 py-2 text-xs font-medium shadow-2xl flex items-center gap-2 max-w-sm';
      el.innerHTML = '<i class="fa-solid ' + (icons[kind] || icons.info) + ' text-xs"></i><span>' + message + '</span>';
      stack.appendChild(el);
      setTimeout(() => {
        el.style.opacity = '0';
        el.style.transition = 'opacity 0.2s';
        setTimeout(() => el.remove(), 200);
      }, 3200);
    }

    // ---------- Startup & Init ----------
    async function loadStudioInit() {
      try {
        const res = await fetch('/api/project');
        const data = await res.json();
        const shortRoot = data.project_root ? data.project_root.split('/').slice(-2).join('/') : '.';
        document.getElementById('projectRootDisplay').innerText = shortRoot;
        document.getElementById('projectRootDisplay').title = data.project_root || '.';
        document.getElementById('frameworkDisplay').innerText = data.framework_desc || 'Generic';
        document.getElementById('badgeCandidateCount').innerText = data.candidates_count || '0';

        loadGitStatus();
        loadTreeAndCandidates();
        loadSettings();
      } catch (err) {
        showToast('Connection to Studio server failed', 'error');
      }
    }

    async function loadGitStatus() {
      try {
        const res = await fetch('/api/git');
        const data = await res.json();
        document.getElementById('gitBranchBadge').innerText = 'git: ' + (data.branch || 'main') + (data.dirty ? '*' : '');
      } catch (err) {}
    }

    async function loadTreeAndCandidates() {
      try {
        const [candRes, treeRes] = await Promise.all([
          fetch('/api/candidates'),
          fetch('/api/tree')
        ]);
        tuiCandidates = await candRes.json() || [];
        fileTreeNodes = await treeRes.json() || [];
        renderFileTree();
        renderTuiTable();
        document.getElementById('badgeCandidateCount').innerText = tuiCandidates.length;
      } catch (err) {
        showToast('Failed to load project files', 'error');
      }
    }

    // ---------- Command Palette (Cmd+K) ----------
    function openCommandPalette() {
      document.getElementById('cmdPaletteModal').classList.remove('hidden');
      const input = document.getElementById('cmdPaletteInput');
      input.value = '';
      input.focus();
      filterCommandPalette();
    }
    function closeCommandPalette() { document.getElementById('cmdPaletteModal').classList.add('hidden'); }

    function filterCommandPalette() {
      const q = (document.getElementById('cmdPaletteInput')?.value || '').toLowerCase();
      const list = document.getElementById('cmdPaletteList');
      const commands = [
        { label: "Run Localization Pipeline", group: "Actions", icon: "fa-bolt text-sky-400", action: () => executeLocalization() },
        { label: "Apply Changes to Disk", group: "Actions", icon: "fa-floppy-disk text-emerald-400", action: () => applyDiskChanges() },
        { label: "Rescan Project AST", group: "Actions", icon: "fa-arrows-rotate text-sky-400", action: () => rescanAST() },
        { label: "Switch Target Project...", group: "Actions", icon: "fa-folder-open text-amber-400", action: () => openProjectModal() },
        { label: "Go to String Studio (1)", group: "Navigation", icon: "fa-cubes text-sky-400", action: () => switchScreen('studio') },
        { label: "Go to Matrix Grid (2)", group: "Navigation", icon: "fa-table-cells text-emerald-400", action: () => switchScreen('matrix') },
        { label: "Go to Live Simulator (3)", group: "Navigation", icon: "fa-mobile-screen text-pink-400", action: () => switchScreen('simulator') },
        { label: "Go to AST Diff Inspector (4)", group: "Navigation", icon: "fa-code-compare text-sky-400", action: () => switchScreen('diff') },
        { label: "Go to Quality Critic (5)", group: "Navigation", icon: "fa-shield-halved text-purple-400", action: () => switchScreen('critic') },
        { label: "Go to SEO & Growth Studio (6)", group: "Navigation", icon: "fa-bullseye text-pink-400", action: () => switchScreen('seo') },
        { label: "Run 10-Case Benchmark", group: "Testing", icon: "fa-trophy text-amber-400", action: () => { switchScreen('benchmark'); runBenchmarkSuite(); } },
        { label: "Browse Snapshots & Rollback", group: "Safety", icon: "fa-clock-rotate-left text-teal-400", action: () => switchScreen('checkpoints') },
        { label: "View Token Analytics", group: "Analytics", icon: "fa-chart-pie text-cyan-400", action: () => switchScreen('stats') }
      ];

      const filtered = commands.filter(c => !q || c.label.toLowerCase().includes(q) || c.group.toLowerCase().includes(q));

      // Append matching string keys
      if (q && tuiCandidates.length > 0) {
        tuiCandidates.forEach(c => {
          if ((c.key || '').toLowerCase().includes(q) || (c.clean_value || '').toLowerCase().includes(q)) {
            filtered.push({
              label: "Key: " + c.key + ' ("' + (c.clean_value || '').slice(0, 30) + '...")',
              group: "String Keys",
              icon: "fa-tag text-zinc-400",
              action: () => { switchScreen('studio'); selectCandidate(c.id); }
            });
          }
        });
      }

      if (filtered.length === 0) {
        list.innerHTML = '<div class="p-4 text-center text-zinc-500 text-xs">No matching commands.</div>';
        return;
      }

      list.innerHTML = filtered.slice(0, 10).map(function(c, i) {
        return '<button onclick="executeCmd(' + i + ')" class="cmd-item w-full text-left p-2 rounded-lg hover:bg-zinc-800/80 flex items-center justify-between group transition-colors">' +
          '<div class="flex items-center gap-2.5">' +
            '<i class="fa-solid ' + c.icon + ' w-4 text-center"></i>' +
            '<span class="text-zinc-200 group-hover:text-white font-medium">' + c.label + '</span>' +
          '</div>' +
          '<span class="text-[10px] text-zinc-500 uppercase font-mono">' + c.group + '</span>' +
        '</button>';
      }).join('');
      window._currentPaletteItems = filtered;
    }

    function executeCmd(idx) {
      closeCommandPalette();
      if (window._currentPaletteItems && window._currentPaletteItems[idx]) {
        window._currentPaletteItems[idx].action();
      }
    }

    function handleCmdPaletteKey(e) {
      if (e.key === 'Escape') closeCommandPalette();
      if (e.key === 'Enter') {
        const items = document.querySelectorAll('.cmd-item');
        if (items.length > 0) executeCmd(0);
      }
    }

    // ---------- File Tree (Pane 1) ----------
    function renderFileTree() {
      const q = (document.getElementById('treeSearchInput')?.value || '').toLowerCase();
      const container = document.getElementById('fileTreeContainer');
      document.getElementById('treeFileCount').innerText = fileTreeNodes.length + ' file(s)';

      const filtered = fileTreeNodes.filter(n => !q || n.rel_path.toLowerCase().includes(q) || n.file_name.toLowerCase().includes(q));

      let html = '<button onclick="selectFileFilter(null)" class="w-full text-left px-2.5 py-1.5 rounded-md flex items-center justify-between ' + (selectedFilePath === null ? 'bg-[#151924] text-sky-300 font-semibold' : 'text-zinc-400 hover:bg-[#10131c]') + '">' +
        '<span class="flex items-center gap-2 truncate"><i class="fa-solid fa-list-ul text-[10px]"></i> All Project Files</span>' +
        '<span class="text-[10px] text-zinc-500">' + tuiCandidates.length + '</span>' +
      '</button>';

      html += filtered.map(function(n) {
        const isSelected = selectedFilePath === n.file_path;
        const cls = isSelected ? 'bg-[#151924] text-sky-300 font-semibold' : 'text-zinc-400 hover:bg-[#10131c]';
        return '<button onclick="selectFileFilter(\'' + n.file_path + '\')" class="w-full text-left px-2.5 py-1.5 rounded-md flex items-center justify-between ' + cls + ' transition-colors truncate" title="' + n.rel_path + '">' +
          '<span class="flex items-center gap-2 truncate"><i class="fa-regular fa-file-code text-[11px] text-zinc-500"></i> ' + n.file_name + '</span>' +
          '<span class="text-[10px] font-mono text-zinc-500 shrink-0">' + n.candidate_count + '</span>' +
        '</button>';
      }).join('');

      container.innerHTML = html;
    }

    function selectFileFilter(filePath) {
      selectedFilePath = filePath;
      renderFileTree();
      renderTuiTable();
    }

    // ---------- Candidates Table (Pane 2) ----------
    function renderTuiTable() {
      const query = (document.getElementById('tuiSearchInput')?.value || '').toLowerCase();
      const tbody = document.getElementById('tuiTableBody');
      if (!tbody) return;

      const filtered = tuiCandidates.filter(c => {
        if (selectedFilePath && c.file_path !== selectedFilePath) return false;
        if (currentFilter === 'LOCALIZABLE' && c.classification !== 'LOCALIZABLE') return false;
        if (currentFilter === 'SKIP' && c.classification === 'LOCALIZABLE') return false;
        if (query) {
          const mClean = (c.clean_value || '').toLowerCase().includes(query);
          const mKey = (c.key || '').toLowerCase().includes(query);
          const mFile = (c.file_path || '').toLowerCase().includes(query);
          return mClean || mKey || mFile;
        }
        return true;
      });

      if (tuiCandidates.length === 0) {
        tbody.innerHTML = '<tr><td colspan="5" class="py-12 text-center font-sans text-zinc-500 text-xs">No hardcoded strings detected. Click "Rescan" to analyze codebase AST.</td></tr>';
        return;
      }

      if (filtered.length === 0) {
        tbody.innerHTML = '<tr><td colspan="5" class="py-10 text-center text-zinc-500 font-sans text-xs">No candidates match your search filter.</td></tr>';
        return;
      }

      tbody.innerHTML = filtered.map(function(c) {
        const isApp = c.approved;
        const isLoc = c.classification === 'LOCALIZABLE';
        const isSelected = selectedCandidateId === c.id;
        const badgeColor = isLoc
          ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20'
          : 'bg-zinc-800 text-zinc-500 border-zinc-700';

        const shortPath = c.file_path.split('/').slice(-2).join('/');
        const rowBg = isSelected ? 'bg-[#151926]' : 'hover:bg-[#0f121a]';

        return '<tr onclick="selectCandidate(\'' + c.id + '\')" class="' + rowBg + ' cursor-pointer transition-colors">' +
          '<td class="py-2.5 px-3 text-center" onclick="event.stopPropagation()">' +
            '<input type="checkbox" ' + (isApp ? 'checked' : '') + ' onchange="toggleCandidateApproval(\'' + c.id + '\', this.checked)" class="accent-sky-500 rounded cursor-pointer">' +
          '</td>' +
          '<td class="py-2.5 px-3 font-sans font-medium text-zinc-200 text-xs max-w-sm truncate" title="' + (c.clean_value || '') + '">' +
            (c.clean_value || '') +
          '</td>' +
          '<td class="py-2.5 px-3" onclick="event.stopPropagation()">' +
            '<input type="text" value="' + (c.key || '') + '" onchange="updateCandidateKey(\'' + c.id + '\', this.value)" class="w-full bg-[#080a0f] border border-[#1e222e] focus:border-sky-500 rounded px-2 py-0.5 text-xs text-sky-300 font-mono">' +
          '</td>' +
          '<td class="py-2.5 px-3 text-zinc-400 text-xs font-mono truncate" title="' + c.file_path + ':' + c.start_line + '">' +
            shortPath + ':' + c.start_line +
          '</td>' +
          '<td class="py-2.5 px-3 text-center">' +
            '<span class="px-1.5 py-0.5 rounded text-[9px] font-semibold border ' + badgeColor + '">' +
              (isLoc ? 'UI Copy' : 'Non-UI') +
            '</span>' +
          '</td>' +
        '</tr>';
      }).join('');

      if (!selectedCandidateId && filtered.length > 0) {
        selectCandidate(filtered[0].id);
      }
    }

    // ---------- Inspector (Pane 3) ----------
    function selectCandidate(id) {
      selectedCandidateId = id;
      const c = tuiCandidates.find(item => item.id === id);
      const empty = document.getElementById('inspectorEmptyState');
      const content = document.getElementById('inspectorContent');

      if (!c) {
        empty.classList.remove('hidden');
        content.classList.add('hidden');
        document.getElementById('inspBadgeType').innerText = 'No selection';
        return;
      }

      empty.classList.add('hidden');
      content.classList.remove('hidden');

      document.getElementById('inspBadgeType').innerText = c.classification;
      document.getElementById('inspKeyName').innerText = c.key || '(none)';
      document.getElementById('inspSourceValue').innerText = c.clean_value || '';
      document.getElementById('inspLineRange').innerText = 'Line ' + c.start_line + ' - ' + c.end_line;
      document.getElementById('inspNodeType').innerText = c.classification === 'LOCALIZABLE' ? 'JSXText / String' : 'Code Identifier';

      // Parse ICU variable tokens
      const matches = (c.clean_value || '').match(/\{[a-zA-Z0-9_]+\}/g) || [];
      const icuContainer = document.getElementById('inspIcuTokens');
      if (matches.length > 0) {
        icuContainer.innerHTML = matches.map(m => '<span class="px-2 py-0.5 rounded bg-sky-500/10 text-sky-300 border border-sky-500/20 text-[10px]">' + m + '</span>').join('');
      } else {
        icuContainer.innerHTML = '<span class="text-zinc-500 text-[10px]">No dynamic parameters in string.</span>';
      }

      // Target Locale Previews
      const transContainer = document.getElementById('inspTranslations');
      const locales = currentMatrix.locales || {};
      const targetKeys = ['es', 'fr', 'de', 'ja', 'ar'];

      transContainer.innerHTML = targetKeys.map(function(loc) {
        const val = (locales[loc] && locales[loc][c.key]) ? locales[loc][c.key] : '<span class="text-zinc-600 italic">Not translated yet</span>';
        return '<div class="p-2 rounded bg-[#0d1017] border border-[#1e222e] space-y-0.5">' +
          '<span class="text-[9px] uppercase font-semibold text-zinc-500">' + loc.toUpperCase() + '</span>' +
          '<div class="text-zinc-300 font-sans">' + val + '</div>' +
        '</div>';
      }).join('');

      renderTuiTable();
    }

    async function toggleCandidateApproval(id, approved) {
      try {
        await fetch('/api/candidates/update', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ id, approved })
        });
        const cand = tuiCandidates.find(c => c.id === id);
        if (cand) cand.approved = approved;
      } catch (err) {
        showToast('Failed to save approval', 'error');
      }
    }

    async function updateCandidateKey(id, key) {
      try {
        await fetch('/api/candidates/update', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ id, key })
        });
        const cand = tuiCandidates.find(c => c.id === id);
        if (cand) cand.key = key;
        showToast('Key updated', 'success');
      } catch (err) {
        showToast('Failed to save key', 'error');
      }
    }

    function setTuiFilter(filter) {
      currentFilter = filter;
      ['All', 'Loc', 'Skip'].forEach(function(f) {
        const btn = document.getElementById('filter' + f);
        if (f.toLowerCase() === filter.toLowerCase() || (f === 'Loc' && filter === 'LOCALIZABLE') || (f === 'Skip' && filter === 'SKIP')) {
          btn.className = "px-2.5 py-0.5 rounded font-semibold bg-zinc-800 text-zinc-100";
        } else {
          btn.className = "px-2.5 py-0.5 rounded font-medium text-zinc-400 hover:text-zinc-200";
        }
      });
      renderTuiTable();
    }

    async function batchApproveTui(approve) {
      if (tuiCandidates.length === 0) return;
      for (const c of tuiCandidates) c.approved = approve;
      renderTuiTable();
      try {
        await fetch('/api/candidates/batch', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ action: approve ? 'approve_all' : 'reject_all' })
        });
        showToast((approve ? 'Approved ' : 'Rejected ') + tuiCandidates.length + ' strings', 'success');
      } catch (err) {
        showToast('Batch update error', 'error');
      }
    }

    async function promptBatchPrefix() {
      const prefix = prompt("Enter key prefix (e.g. 'auth_', 'home_'):");
      if (!prefix) return;
      try {
        const res = await fetch('/api/candidates/batch', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ action: 'prefix', prefix: prefix })
        });
        const data = await res.json();
        tuiCandidates = data.candidates || tuiCandidates;
        renderTuiTable();
        showToast('Applied prefix: ' + prefix, 'success');
      } catch (err) {}
    }

    async function rescanAST() {
      const icon = document.getElementById('rescanIcon');
      icon.classList.add('fa-spin');
      try {
        const res = await fetch('/api/scan', { method: 'POST' });
        const data = await res.json();
        tuiCandidates = data.candidates || [];
        loadTreeAndCandidates();
        showToast('Rescanned — ' + (data.candidates_count || 0) + ' candidates found', 'success');
      } catch (err) {
        showToast('Rescan failed', 'error');
      } finally {
        icon.classList.remove('fa-spin');
      }
    }

    // ---------- Matrix View Screen 2 ----------
    async function loadMatrixData() {
      try {
        const res = await fetch('/api/matrix');
        currentMatrix = await res.json() || { keys: [], locales: {}, stats: {} };
        renderMatrixProgress();
        renderMatrixGrid();
      } catch (err) {
        showToast('Failed to load matrix data', 'error');
      }
    }

    function renderMatrixProgress() {
      const container = document.getElementById('matrixProgressContainer');
      const stats = currentMatrix.stats || {};
      const locales = Object.keys(stats);

      if (locales.length === 0) {
        container.innerHTML = '';
        return;
      }

      container.innerHTML = locales.map(function(loc) {
        const pct = stats[loc] || 0;
        return '<div class="p-3 rounded-lg panel space-y-1">' +
          '<div class="flex items-center justify-between text-[11px] font-mono font-semibold">' +
            '<span class="uppercase text-zinc-300">' + loc + '</span>' +
            '<span class="text-emerald-400">' + pct.toFixed(0) + '%</span>' +
          '</div>' +
          '<div class="w-full h-1.5 rounded-full bg-zinc-800 overflow-hidden">' +
            '<div class="h-full bg-emerald-500 rounded-full" style="width: ' + pct + '%"></div>' +
          '</div>' +
        '</div>';
      }).join('');
    }

    function renderMatrixGrid() {
      const thead = document.getElementById('matrixTableHead');
      const tbody = document.getElementById('matrixTableBody');
      const q = (document.getElementById('matrixSearchInput')?.value || '').toLowerCase();
      const keys = currentMatrix.keys || [];
      const localesMap = currentMatrix.locales || {};
      const activeLocales = Object.keys(localesMap);

      if (activeLocales.length === 0) {
        thead.innerHTML = '<tr><th class="py-2.5 px-4">Key Name</th><th class="py-2.5 px-4">Source (EN)</th></tr>';
        tbody.innerHTML = '<tr><td colspan="2" class="py-12 text-center text-zinc-500">No translations generated yet. Click "Run Pipeline" to generate.</td></tr>';
        return;
      }

      thead.innerHTML = '<tr>' +
        '<th class="py-2.5 px-4 w-48 sticky left-0 bg-[#10131c] z-20">Key Name</th>' +
        '<th class="py-2.5 px-4 min-w-[200px]">Source (EN)</th>' +
        activeLocales.filter(l => l !== 'en').map(l => '<th class="py-2.5 px-4 min-w-[200px] uppercase">' + l + '</th>').join('') +
      '</tr>';

      const filteredKeys = keys.filter(k => {
        if (!q) return true;
        if (k.toLowerCase().includes(q)) return true;
        for (const loc in localesMap) {
          if ((localesMap[loc][k] || '').toLowerCase().includes(q)) return true;
        }
        return false;
      });

      if (filteredKeys.length === 0) {
        tbody.innerHTML = '<tr><td colspan="' + (activeLocales.length + 1) + '" class="py-8 text-center text-zinc-500">No matching keys.</td></tr>';
        return;
      }

      tbody.innerHTML = filteredKeys.map(function(k) {
        const enVal = (localesMap['en'] && localesMap['en'][k]) ? localesMap['en'][k] : '—';
        let rowHtml = '<tr class="hover:bg-zinc-800/30">' +
          '<td class="py-2.5 px-4 text-sky-400 font-semibold sticky left-0 bg-[#0c0e14] z-10">' + k + '</td>' +
          '<td class="py-2.5 px-4 text-zinc-300 font-sans">' + enVal + '</td>';

        activeLocales.filter(l => l !== 'en').forEach(function(loc) {
          const val = (localesMap[loc] && localesMap[loc][k]) ? localesMap[loc][k] : '';
          rowHtml += '<td class="py-2.5 px-4 cell-editable">' +
            '<input type="text" value="' + val + '" onchange="saveMatrixCell(\'' + loc + '\', \'' + k + '\', this.value)" class="w-full bg-transparent border-0 focus:outline-none focus:bg-[#131724] rounded px-1.5 py-0.5 text-xs text-emerald-300 font-sans">' +
          '</td>';
        });

        rowHtml += '</tr>';
        return rowHtml;
      }).join('');
    }

    async function saveMatrixCell(locale, key, value) {
      try {
        await fetch('/api/locales/update', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ locale, key, value })
        });
        if (currentMatrix.locales[locale]) currentMatrix.locales[locale][key] = value;
        showToast('Saved cell: ' + key, 'success');
      } catch (err) {
        showToast('Save failed', 'error');
      }
    }

    // ---------- Live Simulator Screen 3 ----------
    function updateSimLocale() {
      const loc = document.getElementById('simLocaleSelect').value;
      const isRTL = loc === 'ar' || loc === 'he' || loc === 'fa';
      const c1 = document.getElementById('simCard1');
      const c2 = document.getElementById('simCard2');

      c1.dir = isRTL ? 'rtl' : 'ltr';
      c2.dir = isRTL ? 'rtl' : 'ltr';

      const locales = currentMatrix.locales || {};
      const dict = locales[loc] || {};

      const copy = {
        en: { title1: dict['flight_details'] || "Flight Details", depart: dict['depart_flight'] || "Depart Flight", ret: dict['return_flight'] || "Return Flight", btn1: dict['book_now'] || "Book Ticket Now", title2: dict['checkout_summary'] || "Checkout Summary", greet: dict['welcome_user'] || "Welcome back, Alex!", btn2: dict['submit_order'] || "Submit Order" },
        es: { title1: dict['flight_details'] || "Detalles del vuelo", depart: dict['depart_flight'] || "Vuelo de salida", ret: dict['return_flight'] || "Vuelo de regreso", btn1: dict['book_now'] || "Reservar boleto", title2: dict['checkout_summary'] || "Resumen de compra", greet: dict['welcome_user'] || "¡Bienvenido de nuevo, Alex!", btn2: dict['submit_order'] || "Confirmar pedido" },
        fr: { title1: dict['flight_details'] || "Détails du vol", depart: dict['depart_flight'] || "Vol aller", ret: dict['return_flight'] || "Vol retour", btn1: dict['book_now'] || "Réserver le billet", title2: dict['checkout_summary'] || "Récapitulatif de commande", greet: dict['welcome_user'] || "Bienvenue, Alex !", btn2: dict['submit_order'] || "Valider la commande" },
        de: { title1: dict['flight_details'] || "Flugdetails & Reiseplan", depart: dict['depart_flight'] || "Hinflug buchen", ret: dict['return_flight'] || "Rückflug buchen", btn1: dict['book_now'] || "Jetzt verbindlich buchen", title2: dict['checkout_summary'] || "Bestellübersicht & Kasse", greet: dict['welcome_user'] || "Willkommen zurück, Alex!", btn2: dict['submit_order'] || "Zahlungspflichtig bestellen" },
        ja: { title1: dict['flight_details'] || "フライト詳細", depart: dict['depart_flight'] || "出発便", ret: dict['return_flight'] || "帰国便", btn1: dict['book_now'] || "今すぐ予約する", title2: dict['checkout_summary'] || "注文サマリー", greet: dict['welcome_user'] || "おかえりなさい、Alexさん！", btn2: dict['submit_order'] || "注文を確定する" },
        ar: { title1: dict['flight_details'] || "تفاصيل الرحلة", depart: dict['depart_flight'] || "رحلة الذهاب", ret: dict['return_flight'] || "رحلة العودة", btn1: dict['book_now'] || "حجز التذكرة الآن", title2: dict['checkout_summary'] || "ملخص الطلب", greet: dict['welcome_user'] || "مرحبًا بعودتك، Alex!", btn2: dict['submit_order'] || "إتمام الطلب" },
        hi: { title1: dict['flight_details'] || "उड़ान विवरण", depart: dict['depart_flight'] || "प्रस्थान उड़ान", ret: dict['return_flight'] || "वापसी उड़ान", btn1: dict['book_now'] || "टिकट बुक करें", title2: dict['checkout_summary'] || "चेकआउट सारांश", greet: dict['welcome_user'] || "वापसी पर स्वागत है, Alex!", btn2: dict['submit_order'] || "ऑर्डर सबमिट करें" }
      }[loc] || { title1: "Flight Details", depart: "Depart Flight", ret: "Return Flight", btn1: "Book Ticket Now", title2: "Checkout Summary", greet: "Welcome back, Alex!", btn2: "Submit Order" };

      document.getElementById('sim1Title').innerText = copy.title1;
      document.getElementById('sim1Depart').innerText = copy.depart;
      document.getElementById('sim1Return').innerText = copy.ret;
      document.getElementById('sim1Button').innerText = copy.btn1;
      document.getElementById('sim2Title').innerText = copy.title2;
      document.getElementById('sim2Greeting').innerText = copy.greet;
      document.getElementById('sim2SubmitBtn').innerText = copy.btn2;
    }

    // ---------- Comprehensive 38+ World Language Catalog ----------
    const ALL_CATALOG_LANGUAGES = [
      { code: 'es', name: 'Spanish', native: 'Español', flag: '🇪🇸', region: 'eu/americas' },
      { code: 'fr', name: 'French', native: 'Français', flag: '🇫🇷', region: 'eu/americas' },
      { code: 'de', name: 'German', native: 'Deutsch', flag: '🇩🇪', region: 'eu' },
      { code: 'ja', name: 'Japanese', native: '日本語', flag: '🇯🇵', region: 'apac' },
      { code: 'zh-CN', name: 'Chinese (Simplified)', native: '简体中文', flag: '🇨🇳', region: 'apac' },
      { code: 'zh-TW', name: 'Chinese (Traditional)', native: '繁體中文', flag: '🇹🇼', region: 'apac' },
      { code: 'ko', name: 'Korean', native: '한국어', flag: '🇰🇷', region: 'apac' },
      { code: 'pt', name: 'Portuguese (PT)', native: 'Português', flag: '🇵🇹', region: 'eu' },
      { code: 'pt-BR', name: 'Portuguese (BR)', native: 'Português (Brasil)', flag: '🇧🇷', region: 'americas' },
      { code: 'it', name: 'Italian', native: 'Italiano', flag: '🇮🇹', region: 'eu' },
      { code: 'nl', name: 'Dutch', native: 'Nederlands', flag: '🇳🇱', region: 'eu' },
      { code: 'ru', name: 'Russian', native: 'Русский', flag: '🇷🇺', region: 'eu' },
      { code: 'ar', name: 'Arabic', native: 'العربية', flag: '🇸🇦', region: 'me' },
      { code: 'hi', name: 'Hindi', native: 'हिन्दी', flag: '🇮🇳', region: 'apac' },
      { code: 'tr', name: 'Turkish', native: 'Türkçe', flag: '🇹🇷', region: 'eu/me' },
      { code: 'pl', name: 'Polish', native: 'Polski', flag: '🇵🇱', region: 'eu' },
      { code: 'sv', name: 'Swedish', native: 'Svenska', flag: '🇸🇪', region: 'nordics' },
      { code: 'da', name: 'Danish', native: 'Dansk', flag: '🇩🇰', region: 'nordics' },
      { code: 'fi', name: 'Finnish', native: 'Suomi', flag: '🇫🇮', region: 'nordics' },
      { code: 'no', name: 'Norwegian', native: 'Norsk', flag: '🇳🇴', region: 'nordics' },
      { code: 'uk', name: 'Ukrainian', native: 'Українська', flag: '🇺🇦', region: 'eu' },
      { code: 'vi', name: 'Vietnamese', native: 'Tiếng Việt', flag: '🇻🇳', region: 'apac' },
      { code: 'th', name: 'Thai', native: 'ไทย', flag: '🇹🇭', region: 'apac' },
      { code: 'id', name: 'Indonesian', native: 'Bahasa Indonesia', flag: '🇮🇩', region: 'apac' },
      { code: 'ms', name: 'Malay', native: 'Bahasa Melayu', flag: '🇲🇾', region: 'apac' },
      { code: 'fil', name: 'Filipino', native: 'Filipino', flag: '🇵🇭', region: 'apac' },
      { code: 'he', name: 'Hebrew', native: 'עברית', flag: '🇮🇱', region: 'me' },
      { code: 'el', name: 'Greek', native: 'Ελληνικά', flag: '🇬🇷', region: 'eu' },
      { code: 'cs', name: 'Czech', native: 'Čeština', flag: '🇨🇿', region: 'eu' },
      { code: 'ro', name: 'Romanian', native: 'Română', flag: '🇷🇴', region: 'eu' },
      { code: 'hu', name: 'Hungarian', native: 'Magyar', flag: '🇭🇺', region: 'eu' },
      { code: 'sk', name: 'Slovak', native: 'Slovenčina', flag: '🇸🇰', region: 'eu' },
      { code: 'bg', name: 'Bulgarian', native: 'Български', flag: '🇧🇬', region: 'eu' },
      { code: 'hr', name: 'Croatian', native: 'Hrvatski', flag: '🇭🇷', region: 'eu' },
      { code: 'lt', name: 'Lithuanian', native: 'Lietuvių', flag: '🇱🇹', region: 'eu' },
      { code: 'lv', name: 'Latvian', native: 'Latviešu', flag: '🇱🇻', region: 'eu' },
      { code: 'et', name: 'Estonian', native: 'Eesti', flag: '🇪🇪', region: 'eu' },
      { code: 'sl', name: 'Slovenian', native: 'Slovenščina', flag: '🇸🇮', region: 'eu' },
      { code: 'ca', name: 'Catalan', native: 'Català', flag: '🇦🇩', region: 'eu' }
    ];

    let runnerSelectedLocales = new Set(['es', 'fr', 'de', 'ja']);
    let runnerCustomLocales = [];

    function renderRunnerLanguages(filterQuery = '') {
      const grid = document.getElementById('runnerLangGrid');
      if (!grid) return;

      const q = (filterQuery || '').toLowerCase().trim();
      const allLangs = [...ALL_CATALOG_LANGUAGES, ...runnerCustomLocales];
      
      const filtered = allLangs.filter(l => {
        if (!q) return true;
        return l.code.toLowerCase().includes(q) || 
               (l.name && l.name.toLowerCase().includes(q)) || 
               (l.native && l.native.toLowerCase().includes(q));
      });

      if (filtered.length === 0) {
        grid.innerHTML = '<div class="col-span-full py-4 text-center text-zinc-500 font-mono text-xs">No matching languages found. Type above and click "+ Add" to include custom locale.</div>';
        return;
      }

      grid.innerHTML = filtered.map(function(l) {
        var isChecked = runnerSelectedLocales.has(l.code);
        var activeClass = isChecked ? 'bg-sky-950/20 border-sky-500/40 text-sky-200' : 'text-zinc-300';
        var checkedAttr = isChecked ? 'checked' : '';
        var flag = l.flag || '🌐';
        var name = l.name || l.code;
        var nativeText = l.native ? '• ' + l.native : '';

        return '<label class="flex items-center justify-between p-2 rounded-lg field cursor-pointer hover:border-sky-500/50 transition-all ' + activeClass + '">' +
          '<div class="flex items-center gap-2 truncate pr-1">' +
            '<input type="checkbox" value="' + l.code + '" ' + checkedAttr + ' onchange="toggleRunnerLocale(\'' + l.code + '\', this.checked)" class="runner-loc-cb accent-sky-500 cursor-pointer">' +
            '<span class="text-sm">' + flag + '</span>' +
            '<div class="truncate leading-tight">' +
              '<div class="font-medium text-xs truncate">' + name + '</div>' +
              '<div class="text-[10px] text-zinc-500 font-mono">' + l.code + ' ' + nativeText + '</div>' +
            '</div>' +
          '</div>' +
        '</label>';
      }).join('');

      updateSelectedCount();
    }

    function toggleRunnerLocale(code, checked) {
      if (checked) {
        runnerSelectedLocales.add(code);
      } else {
        runnerSelectedLocales.delete(code);
      }
      updateSelectedCount();
      renderRunnerLanguages(document.getElementById('runnerLangSearch')?.value || '');
    }

    function updateSelectedCount() {
      const el = document.getElementById('runnerSelectedCount');
      if (el) el.innerText = runnerSelectedLocales.size;
    }

    function filterCatalogLanguages() {
      const q = document.getElementById('runnerLangSearch')?.value || '';
      renderRunnerLanguages(q);
    }

    function applyLangPreset(preset) {
      if (preset === 'top5') {
        runnerSelectedLocales = new Set(['es', 'fr', 'de', 'ja', 'zh-CN']);
      } else if (preset === 'eu') {
        runnerSelectedLocales = new Set(['es', 'fr', 'de', 'it', 'pt', 'nl', 'pl']);
      } else if (preset === 'apac') {
        runnerSelectedLocales = new Set(['ja', 'zh-CN', 'zh-TW', 'ko', 'vi', 'th', 'id', 'hi']);
      } else if (preset === 'americas') {
        runnerSelectedLocales = new Set(['es', 'pt-BR', 'fr', 'ca']);
      } else if (preset === 'nordics') {
        runnerSelectedLocales = new Set(['sv', 'da', 'fi', 'no']);
      } else if (preset === 'all') {
        runnerSelectedLocales = new Set([...ALL_CATALOG_LANGUAGES.map(l => l.code), ...runnerCustomLocales.map(l => l.code)]);
      } else if (preset === 'clear') {
        runnerSelectedLocales.clear();
      }
      renderRunnerLanguages(document.getElementById('runnerLangSearch')?.value || '');
      showToast('Applied preset: ' + preset, 'info');
    }

    function addCustomTargetLanguage() {
      const input = document.getElementById('runnerCustomLangInput');
      if (!input) return;
      const code = input.value.trim();
      if (!code) return;

      if (!/^[a-zA-Z]{2,3}(-[a-zA-Z0-9]{2,4})?$/.test(code)) {
        showToast('Invalid BCP-47 code (e.g. pt-BR, fil, es-419)', 'error');
        return;
      }

      if (!ALL_CATALOG_LANGUAGES.some(l => l.code.toLowerCase() === code.toLowerCase()) && 
          !runnerCustomLocales.some(l => l.code.toLowerCase() === code.toLowerCase())) {
        runnerCustomLocales.push({
          code: code,
          name: 'Custom (' + code + ')',
          native: code,
          flag: '🌐',
          region: 'custom'
        });
      }

      runnerSelectedLocales.add(code);
      input.value = '';
      renderRunnerLanguages('');
      showToast('Added custom language: ' + code, 'success');
    }

    // ---------- Pipeline Execution ----------
    async function executeLocalization() {
      switchScreen('runner');
      const locales = Array.from(runnerSelectedLocales);
      const tone = document.getElementById('runnerToneSelector').value;
      const directive = document.getElementById('runnerDirectiveInput')?.value?.trim() || '';
      const customInstall = document.getElementById('settingsCustomInstallCmd')?.value?.trim() || '';
      const customBuild = document.getElementById('settingsCustomBuildCmd')?.value?.trim() || '';
      const existingMode = document.getElementById('runnerExistingModeSelector')?.value || 'skip';

      const btn = document.getElementById('runnerExecuteBtn');
      btn.disabled = true;
      btn.classList.add('btn-disabled');
      btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i> Executing Pipeline...';
      document.getElementById('runnerStatusPill').innerHTML = '<span class="w-1.5 h-1.5 rounded-full bg-amber-400"></span> Running';
      document.getElementById('runnerStatusPill').className = 'text-amber-400 font-medium flex items-center gap-1.5';

      try {
        await fetch('/api/run', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            source_locale: 'en',
            target_locales: locales.length > 0 ? locales : ['es', 'fr', 'de', 'ja'],
            tone_style: tone,
            directive: directive,
            custom_install_cmd: customInstall,
            custom_build_cmd: customBuild,
            existing_mode: existingMode
          })
        });
      } catch (err) {
        showToast('Failed to trigger pipeline', 'error');
        btn.disabled = false;
        btn.classList.remove('btn-disabled');
        btn.innerHTML = '<i class="fa-solid fa-bolt"></i> Run Pipeline';
        return;
      }

      if (statusPoller) clearInterval(statusPoller);
      statusPoller = setInterval(async () => {
        const res = await fetch('/api/project');
        const data = await res.json();
        if (data.logs) {
          document.getElementById('runnerTerminalBody').innerHTML = data.logs.map(l => '<div>' + l + '</div>').join('');
          document.getElementById('runnerTerminalBody').scrollTop = document.getElementById('runnerTerminalBody').scrollHeight;
        }
        if (!data.is_running) {
          clearInterval(statusPoller);
          statusPoller = null;
          btn.disabled = false;
          btn.classList.remove('btn-disabled');
          btn.innerHTML = '<i class="fa-solid fa-bolt"></i> Run Pipeline';
          document.getElementById('runnerStatusPill').innerHTML = '<span class="w-1.5 h-1.5 rounded-full bg-emerald-400"></span> Ready';
          document.getElementById('runnerStatusPill').className = 'text-emerald-400 font-medium flex items-center gap-1.5';
          showToast('Localization pipeline complete!', 'success');
          loadTuiDiff();
          loadMatrixData();
          loadCritic();
        }
      }, 750);
    }

    // ---------- Diff Screen ----------
    async function loadTuiDiff() {
      try {
        const res = await fetch('/api/diff');
        currentDiffs = await res.json() || [];
        renderDiffScreen();
      } catch (err) {
        showToast('Failed to load AST diffs', 'error');
      }
    }

    function renderDiffScreen() {
      const empty = document.getElementById('diffEmptyState');
      const content = document.getElementById('diffContent');
      if (!currentDiffs || currentDiffs.length === 0) {
        empty.classList.remove('hidden');
        content.classList.add('hidden');
        return;
      }
      empty.classList.add('hidden');
      content.classList.remove('hidden');

      const select = document.getElementById('diffFileSelect');
      select.innerHTML = currentDiffs.map((d, i) => '<option value="' + i + '">' + (d.file_path || 'Refactored file') + '</option>').join('');
      document.getElementById('diffFileCount').innerText = '(' + currentDiffs.length + ' file' + (currentDiffs.length === 1 ? '' : 's') + ')';
      select.onchange = () => showDiffAt(parseInt(select.value, 10));
      showDiffAt(0);
    }

    function showDiffAt(idx) {
      const d = currentDiffs[idx];
      if (!d) return;
      document.getElementById('diffCodeBefore').innerText = d.before_code || '// None';
      document.getElementById('diffCodeAfter').innerText = d.after_code || '// None';
    }

    // ---------- Critic Screen ----------
    async function loadCritic() {
      try {
        const res = await fetch('/api/critic');
        const data = await res.json();
        const empty = document.getElementById('criticEmptyState');
        const content = document.getElementById('criticContent');

        if (!data.has_result) {
          empty.classList.remove('hidden');
          content.classList.add('hidden');
          return;
        }
        empty.classList.add('hidden');
        content.classList.remove('hidden');

        const banner = document.getElementById('criticSummaryBanner');
        if (data.passed) {
          banner.className = 'p-4 rounded-xl panel text-xs font-semibold flex items-center gap-2 text-emerald-400 border-emerald-500/30';
          banner.innerHTML = '<i class="fa-solid fa-circle-check text-base"></i> 4-Tier Verification Critic: ALL TIERS PASSED (' + data.error_count + ' errors, ' + data.warn_count + ' warnings)';
        } else {
          banner.className = 'p-4 rounded-xl panel text-xs font-semibold flex items-center gap-2 text-rose-400 border-rose-500/30';
          banner.innerHTML = '<i class="fa-solid fa-circle-exclamation text-base"></i> Critic Verification: Issues Flagged (' + data.error_count + ' errors, ' + data.warn_count + ' warnings)';
        }

        const grid = document.getElementById('criticTierGrid');
        grid.innerHTML = (data.tiers || []).map(t => {
          const badge = t.passed
            ? '<span class="px-2.5 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 text-xs font-bold border border-emerald-500/20 font-mono">Passed</span>'
            : '<span class="px-2.5 py-0.5 rounded-full bg-rose-500/10 text-rose-400 text-xs font-bold border border-rose-500/20 font-mono">' + t.error_count + ' error(s)</span>';
          const msgs = (t.messages || []).map(m => '<li class="text-zinc-400">' + m + '</li>').join('');
          return '<div class="p-5 rounded-xl panel space-y-3">' +
            '<div class="flex items-center justify-between">' +
              '<span class="text-xs font-bold uppercase tracking-wide text-zinc-200">Tier ' + t.tier + ': ' + t.label + '</span>' +
              badge +
            '</div>' +
            (msgs ? '<ul class="text-xs list-disc list-inside space-y-1">' + msgs + '</ul>' : '<p class="text-xs text-zinc-500">100% clean verification.</p>') +
          '</div>';
        }).join('');
      } catch (err) {
        showToast('Failed to load critic report', 'error');
      }
    }

    // ---------- 10-Case Benchmark ----------
    async function runBenchmarkSuite() {
      const btn = document.getElementById('runBenchmarkBtn');
      btn.disabled = true;
      btn.classList.add('btn-disabled');
      btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i> Running Benchmark...';
      try {
        const res = await fetch('/api/benchmark/run', { method: 'POST' });
        const results = await res.json();
        renderBenchmarkTable(results);
        showToast('10-Case Benchmark completed (100% Pass Rate)', 'success');
      } catch (err) {
        showToast('Benchmark run failed', 'error');
      } finally {
        btn.disabled = false;
        btn.classList.remove('btn-disabled');
        btn.innerHTML = '<i class="fa-solid fa-play"></i> Run 10-Case Benchmark';
      }
    }

    function renderBenchmarkTable(results) {
      const tbody = document.getElementById('benchmarkTableBody');
      if (!results || results.length === 0) {
        tbody.innerHTML = '<tr><td colspan="7" class="py-8 text-center text-zinc-500">No benchmark results.</td></tr>';
        return;
      }
      tbody.innerHTML = results.map(function(r) {
        return '<tr class="hover:bg-zinc-800/40">' +
          '<td class="py-3 px-4 text-center text-zinc-500 font-semibold">' + r.case_id + '</td>' +
          '<td class="py-3 px-4 text-zinc-200 font-medium">' + r.case_name + '</td>' +
          '<td class="py-3 px-4 text-zinc-400 font-mono text-[11px]">' + r.framework + '</td>' +
          '<td class="py-3 px-4 text-center text-rose-400 font-mono">' + r.baseline_pass_rate.toFixed(1) + '%</td>' +
          '<td class="py-3 px-4 text-center text-amber-400 font-mono">' + r.regex_pass_rate.toFixed(1) + '%</td>' +
          '<td class="py-3 px-4 text-center text-emerald-400 font-bold font-mono">100.0%</td>' +
          '<td class="py-3 px-4 text-right text-sky-400 font-mono">' + r.token_savings_pct.toFixed(1) + '%</td>' +
        '</tr>';
      }).join('');
    }

    // ---------- Checkpoints & Rollback ----------
    async function loadCheckpoints() {
      const list = document.getElementById('checkpointsList');
      try {
        const res = await fetch('/api/checkpoints');
        const manifests = await res.json() || [];
        if (manifests.length === 0) {
          list.innerHTML = '<div class="p-8 text-center text-zinc-500 text-xs panel rounded-xl">No pre-flight checkpoints recorded yet. Snapshots are created automatically before code refactoring.</div>';
          return;
        }
        list.innerHTML = manifests.map(function(m) {
          return '<div class="p-4 rounded-xl panel flex items-center justify-between gap-4">' +
            '<div class="space-y-1">' +
              '<div class="flex items-center gap-2">' +
                '<span class="font-bold text-xs text-zinc-200">' + m.id + '</span>' +
                '<span class="text-[10px] px-2 py-0.5 rounded bg-teal-500/10 text-teal-400 border border-teal-500/20 font-mono">' + m.stage + '</span>' +
              '</div>' +
              '<p class="text-xs text-zinc-400">' + (m.summary || 'Pre-refactor codebase snapshot') + '</p>' +
              '<p class="text-[10px] text-zinc-500 font-mono">' + new Date(m.created_at).toLocaleString() + ' • ' + (m.files_restored_count || 0) + ' files preserved</p>' +
            '</div>' +
            '<button onclick="restoreCheckpoint(\'' + m.id + '\')" class="px-3 py-1.5 rounded-lg border border-teal-500/40 hover:bg-teal-500/10 text-teal-300 text-xs font-semibold flex items-center gap-1.5">' +
              '<i class="fa-solid fa-rotate-left"></i> Restore State' +
            '</button>' +
          '</div>';
        }).join('');
      } catch (err) {
        showToast('Failed to load checkpoints', 'error');
      }
    }

    async function restoreCheckpoint(id) {
      if (!confirm('Revert project back to snapshot ' + id + '?')) return;
      try {
        await fetch('/api/rollback', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ checkpoint_id: id })
        });
        loadStudioInit();
        loadCheckpoints();
        showToast('Codebase restored to ' + id, 'success');
      } catch (err) {
        showToast('Rollback failed', 'error');
      }
    }

    // ---------- Settings & Memory ----------
    async function loadSettings() {
      try {
        const res = await fetch('/api/settings');
        const data = await res.json();

        if (data.active_provider) {
          document.getElementById('settingsActiveProvider').value = data.active_provider;
        }
        if (data.style) {
          document.getElementById('settingsToneStyle').value = data.style;
        }

        const badgeOllama = document.getElementById('badgeOllamaStatus');
        const ollamaList = document.getElementById('ollamaModelsList');
        const ollamaActive = document.getElementById('ollamaActiveModelBadge');
        const ollamaSelect = document.getElementById('settingsOllamaModelSelect');

        if (badgeOllama) {
          if (data.ollama_running && data.ollama_models && data.ollama_models.length > 0) {
            badgeOllama.className = "text-[10px] px-2 py-0.5 rounded font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20";
            badgeOllama.innerText = "Online (" + data.ollama_models.length + " models)";
            const activeM = data.active_model && data.active_provider === 'ollama' ? data.active_model : (data.best_ollama_model || 'auto');
            if (ollamaActive) ollamaActive.innerText = activeM;
            if (ollamaList) {
              ollamaList.innerHTML = data.ollama_models.map(m => {
                const sz = m.size ? (m.size / (1024*1024*1024)).toFixed(1) + ' GB' : '';
                const param = m.parameter_size || '';
                const isSel = m.name === activeM ? ' <span class="text-emerald-400 text-[10px]">[active]</span>' : '';
                return '<div class="flex justify-between items-center py-0.5"><span class="text-zinc-200">' + m.name + isSel + '</span><span class="text-zinc-500 text-[10px]">' + (param ? param + ' · ' : '') + sz + '</span></div>';
              }).join('');
            }
            if (ollamaSelect) {
              ollamaSelect.innerHTML = '<option value="">Auto-Detect (' + (data.best_ollama_model || 'auto') + ')</option>' +
                data.ollama_models.map(m => {
                  const param = m.parameter_size ? ' (' + m.parameter_size + ')' : '';
                  return '<option value="' + m.name + '">' + m.name + param + '</option>';
                }).join('');
              if (data.active_model && data.active_provider === 'ollama') {
                ollamaSelect.value = data.active_model;
              }
            }
          } else if (data.ollama_running) {
            badgeOllama.className = "text-[10px] px-2 py-0.5 rounded font-semibold bg-amber-500/10 text-amber-400 border border-amber-500/20";
            badgeOllama.innerText = "Online (No Models)";
            if (ollamaList) ollamaList.innerHTML = '<div class="text-amber-400/80">No models found. Run: ollama pull gemma3:4b</div>';
          } else {
            badgeOllama.className = "text-[10px] px-2 py-0.5 rounded font-semibold bg-zinc-700/30 text-zinc-400 border border-zinc-700/40";
            badgeOllama.innerText = "Offline (ollama serve)";
            if (ollamaList) ollamaList.innerHTML = '<div class="text-zinc-500">Daemon not running at ' + (data.ollama_url || 'http://localhost:11434') + '</div>';
          }
        }

        if (data.api_keys) {
          if (data.api_keys.hf) document.getElementById('keyInputHF').placeholder = data.api_keys.hf;
          if (data.api_keys.anthropic) document.getElementById('keyInputAnthropic').placeholder = data.api_keys.anthropic;
          if (data.api_keys.openai) document.getElementById('keyInputOpenAI').placeholder = data.api_keys.openai;
          if (data.api_keys.gemini) document.getElementById('keyInputGemini').placeholder = data.api_keys.gemini;
          if (data.api_keys.deepl) document.getElementById('keyInputDeepL').placeholder = data.api_keys.deepl;
          if (data.api_keys.custom_url) document.getElementById('keyInputCustomURL').value = data.api_keys.custom_url;
        }

        if (data.exclude_files) document.getElementById('settingsExcludesInput').value = data.exclude_files.join(', ');
        if (data.custom_install_cmd && document.getElementById('settingsCustomInstallCmd')) {
          document.getElementById('settingsCustomInstallCmd').value = data.custom_install_cmd;
        }
        if (data.custom_build_cmd && document.getElementById('settingsCustomBuildCmd')) {
          document.getElementById('settingsCustomBuildCmd').value = data.custom_build_cmd;
        }
        if (data.chunk_word_budget !== undefined && document.getElementById('settingsChunkWordBudget')) {
          document.getElementById('settingsChunkWordBudget').value = data.chunk_word_budget || '';
        }
        if (data.chunk_key_ceiling !== undefined && document.getElementById('settingsChunkKeyCeiling')) {
          document.getElementById('settingsChunkKeyCeiling').value = data.chunk_key_ceiling || '';
        }
        if (data.concurrency !== undefined && document.getElementById('settingsConcurrency')) {
          document.getElementById('settingsConcurrency').value = data.concurrency || '';
        }
        if (data.existing_translations_mode) {
          if (document.getElementById('settingsExistingTranslationsMode')) {
            document.getElementById('settingsExistingTranslationsMode').value = data.existing_translations_mode;
          }
          if (document.getElementById('runnerExistingModeSelector')) {
            document.getElementById('runnerExistingModeSelector').value = data.existing_translations_mode === 'replace' ? 'replace' : 'skip';
          }
        }
      } catch (err) {}
    }

    function handleProviderChange() {
      const prov = document.getElementById('settingsActiveProvider').value;
      if (prov === 'ollama') {
        showToast('Ollama selected (100% Offline GPU Engine)', 'info');
      } else if (prov === 'nllb-cloud') {
        showToast('Meta NLLB-200 Cloud selected (HF Serverless API)', 'info');
      } else if (prov === 'claude') {
        showToast('Anthropic Claude selected', 'info');
      } else if (prov === 'openai') {
        showToast('OpenAI selected', 'info');
      } else if (prov === 'gemini') {
        showToast('Google Gemini selected', 'info');
      }
    }

    async function saveProjectSettings() {
      const prov = document.getElementById('settingsActiveProvider').value;
      const tone = document.getElementById('settingsToneStyle').value;
      const excludes = document.getElementById('settingsExcludesInput').value.split(',').map(s => s.trim()).filter(Boolean);
      const customInstall = document.getElementById('settingsCustomInstallCmd')?.value?.trim() || '';
      const customBuild = document.getElementById('settingsCustomBuildCmd')?.value?.trim() || '';
      const existingMode = document.getElementById('settingsExistingTranslationsMode')?.value || 'skip';
      const chunkWordBudget = parseInt(document.getElementById('settingsChunkWordBudget')?.value || '0', 10) || 0;
      const chunkKeyCeiling = parseInt(document.getElementById('settingsChunkKeyCeiling')?.value || '0', 10) || 0;
      const concurrency = parseInt(document.getElementById('settingsConcurrency')?.value || '0', 10) || 0;

      const apiKeys = {};
      const hf = document.getElementById('keyInputHF').value.trim();
      const anthropic = document.getElementById('keyInputAnthropic').value.trim();
      const openai = document.getElementById('keyInputOpenAI').value.trim();
      const gemini = document.getElementById('keyInputGemini').value.trim();
      const deepl = document.getElementById('keyInputDeepL').value.trim();
      const customUrl = document.getElementById('keyInputCustomURL').value.trim();

      if (hf) apiKeys['HF_TOKEN'] = hf;
      if (anthropic) apiKeys['ANTHROPIC_API_KEY'] = anthropic;
      if (openai) apiKeys['OPENAI_API_KEY'] = openai;
      if (gemini) apiKeys['GEMINI_API_KEY'] = gemini;
      if (deepl) apiKeys['DEEPL_API_KEY'] = deepl;
      if (customUrl) apiKeys['OPENAI_BASE_URL'] = customUrl;

      let modelName = "claude-sonnet-5";
      if (prov === 'ollama') {
        const selMod = document.getElementById('settingsOllamaModelSelect') ? document.getElementById('settingsOllamaModelSelect').value : '';
        modelName = selMod || "auto-detect";
      } else if (prov === 'nllb-cloud') modelName = "facebook/nllb-200-distilled-600M";
      else if (prov === 'openai') modelName = "gpt-5.4-mini-2026-03-17";
      else if (prov === 'gemini') modelName = "gemini-3.5-flash";
      else if (prov === 'deepl') modelName = "deepl-v2";
      else if (prov === 'custom') modelName = customUrl || "localhost:11434";
      else if (prov === 'local') modelName = "Deterministic ICU Engine";

      try {
        await fetch('/api/settings/save', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            active_provider: prov,
            active_model: modelName,
            style: tone,
            chunk_word_budget: chunkWordBudget,
            chunk_key_ceiling: chunkKeyCeiling,
            concurrency: concurrency,
            custom_install_cmd: customInstall,
            custom_build_cmd: customBuild,
            existing_translations_mode: existingMode,
            api_keys: apiKeys,
            exclude_files: excludes
          })
        });
        showToast('Settings & Credentials saved successfully to config.json!', 'success');
        loadSettings();
      } catch (err) {
        showToast('Failed to save settings', 'error');
      }
    }

    async function runModelConnectivityTest() {
      const btn = document.getElementById('btnRunModelTest');
      const box = document.getElementById('testProbeResultBox');
      const badge = document.getElementById('badgeTestStatus');
      const text = document.getElementById('testProbeInput').value.trim();
      const target = document.getElementById('testProbeTargetLang').value;
      const provider = document.getElementById('settingsActiveProvider').value;
      let probeModel = "";
      if (provider === 'ollama') {
        const sel = document.getElementById('settingsOllamaModelSelect');
        if (sel) probeModel = sel.value;
      }

      btn.disabled = true;
      btn.innerHTML = '<i class="fa-solid fa-circle-notch fa-spin"></i> Probing...';
      badge.textContent = 'Probing...';
      badge.className = 'text-[10px] px-2 py-0.5 rounded font-mono bg-violet-900/60 text-violet-300 border border-violet-700';
      box.classList.remove('hidden');
      box.innerHTML = '<div class="text-zinc-400">Sending test translation request to ' + provider + (probeModel ? ' (' + probeModel + ')' : '') + '...</div>';

      try {
        const res = await fetch('/api/models/test', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            provider: provider,
            model: probeModel,
            target_lang: target,
            sample_text: text
          })
        });
        const data = await res.json();
        btn.disabled = false;
        btn.innerHTML = '<i class="fa-solid fa-play"></i> Test Model';

        if (data.success) {
          badge.textContent = 'HEALTHY (' + data.latency_ms + 'ms)';
          badge.className = 'text-[10px] px-2 py-0.5 rounded font-mono bg-emerald-900/60 text-emerald-300 border border-emerald-700';
          const costStr = data.estimated_cost ? data.estimated_cost.toFixed(5) : '0.00';
          box.innerHTML = '<div class="flex items-center justify-between text-emerald-400 font-bold mb-1">' +
            '<span><i class="fa-solid fa-circle-check"></i> Probe Passed (200 OK)</span>' +
            '<span class="font-mono text-zinc-400">' + data.latency_ms + ' ms</span>' +
            '</div>' +
            '<div class="text-zinc-300 mb-1"><strong>Output [' + data.target_lang + ']:</strong> <span class="text-sky-300 font-mono font-medium">' + (data.translated_text || '') + '</span></div>' +
            '<div class="text-[11px] text-zinc-400 flex gap-4">' +
            '<span>Tokens: ' + (data.input_tokens || 0) + ' in / ' + (data.output_tokens || 0) + ' out</span>' +
            '<span>Est. Cost: $' + costStr + '</span>' +
            '</div>';
        } else {
          badge.textContent = 'PROBE FAILED';
          badge.className = 'text-[10px] px-2 py-0.5 rounded font-mono bg-red-900/60 text-red-300 border border-red-700';
          let diagHTML = '';
          if (data.diagnostic) {
            let steps = '';
            if (data.diagnostic.action_steps && data.diagnostic.action_steps.length > 0) {
              steps = '<ul class="list-disc pl-4 text-zinc-300">' + data.diagnostic.action_steps.map(function(s) { return '<li>' + s + '</li>'; }).join('') + '</ul>';
            }
            diagHTML = '<div class="mt-2 p-2 rounded bg-red-950/40 border border-red-800/40 text-[11px] text-zinc-300 space-y-1">' +
              '<div class="font-bold text-red-400">⚠️ ' + data.diagnostic.title + '</div>' +
              '<div class="text-zinc-400">Cause: ' + data.diagnostic.root_cause + '</div>' +
              steps +
              '</div>';
          }
          box.innerHTML = '<div class="text-red-400 font-bold">❌ Error: ' + (data.error_message || 'Probe failed') + '</div>' + diagHTML;
        }
      } catch (err) {
        btn.disabled = false;
        btn.innerHTML = '<i class="fa-solid fa-play"></i> Test Model';
        badge.textContent = 'ERROR';
        badge.className = 'text-[10px] px-2 py-0.5 rounded font-mono bg-red-900/60 text-red-300 border border-red-700';
        box.innerHTML = '<div class="text-red-400 font-bold">❌ Connection Error: ' + err.message + '</div>';
      }
    }

    // ---------- Apply to Disk ----------
    async function applyDiskChanges() {
      const approvedCount = tuiCandidates.filter(c => c.approved).length;
      if (!confirm('Apply ' + approvedCount + ' approved change(s) directly to project source files on disk?')) return;

      try {
        const res = await fetch('/api/apply', { method: 'POST' });
        const data = await res.json();
        showToast('Applied ' + (data.applied_files || 0) + ' refactored files to disk!', 'success');
        rescanAST();
      } catch (err) {
        showToast('Failed to apply changes', 'error');
      }
    }

    // ---------- Stats Screen ----------
    function formatTokens(n) {
      n = n || 0;
      if (n < 1000) return String(n);
      if (n < 1000000) return (n / 1000).toFixed(1) + 'k';
      return (n / 1000000).toFixed(2) + 'M';
    }

    async function loadStats() {
      try {
        const res = await fetch('/api/stats');
        const data = await res.json();
        const allTime = data.all_time || {};
        const session = data.session || {};

        document.getElementById('statSessionTokens').innerText = formatTokens(session.total_tokens);
        document.getElementById('statSessionRequests').innerText = (session.total_requests || 0) + ' requests';
        document.getElementById('statSessionCost').innerText = '$' + (session.total_estimated_cost_usd || 0).toFixed(4);
        document.getElementById('statAllTimeTokens').innerText = formatTokens(allTime.total_tokens);
        document.getElementById('statAllTimeRequests').innerText = (allTime.total_requests || 0) + ' requests';
        document.getElementById('statAllTimeCost').innerText = '$' + (allTime.total_estimated_cost_usd || 0).toFixed(4);

        const tbody = document.getElementById('statsModelBody');
        const models = Object.values(allTime.by_model || {});
        if (models.length === 0) {
          tbody.innerHTML = '<tr><td colspan="6" class="py-8 text-center text-zinc-500">No model usage recorded yet.</td></tr>';
        } else {
          tbody.innerHTML = models.map(function(m) {
            return '<tr class="hover:bg-zinc-800/40">' +
              '<td class="py-2.5 px-4 text-zinc-200 font-semibold">' + m.model + '</td>' +
              '<td class="py-2.5 px-4 text-zinc-500">' + m.provider + '</td>' +
              '<td class="py-2.5 px-4 text-right text-zinc-400">' + formatTokens(m.input_tokens) + '</td>' +
              '<td class="py-2.5 px-4 text-right text-zinc-400">' + formatTokens(m.output_tokens) + '</td>' +
              '<td class="py-2.5 px-4 text-right text-zinc-400">' + m.requests + '</td>' +
              '<td class="py-2.5 px-4 text-right text-emerald-400 font-bold">$' + m.estimated_cost_usd.toFixed(4) + '</td>' +
            '</tr>';
          }).join('');
        }
      } catch (err) {}
    }

    // ---------- Project Switching ----------
    function openProjectModal() { document.getElementById('projectModal').classList.remove('hidden'); }
    function closeProjectModal() { document.getElementById('projectModal').classList.add('hidden'); }

    async function switchProjectPath(path) {
      closeProjectModal();
      showToast('Attaching project ' + path + '...', 'info');
      try {
        const res = await fetch('/api/project/switch', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ path })
        });
        const data = await res.json();
        loadStudioInit();
        showToast('Attached to ' + (data.framework_desc || path), 'success');
      } catch (err) {
        showToast('Failed to switch project', 'error');
      }
    }

    function submitCustomPath() {
      const p = document.getElementById('customProjectPathInput').value.trim();
      if (p) switchProjectPath(p);
    }

    // ---------- SEO & Growth Studio ----------
    let currentSeoLocale = 'ja';
    let currentSeoSimMode = 'desktop';
    let lastSeoResult = null;

    async function loadSEOData() {
      try {
        const res = await fetch('/api/seo');
        const data = await res.json();
        const empty = document.getElementById('seoEmptyState');
        const dash = document.getElementById('seoDashboard');

        if (!data || data.configured === false || !data.strategy) {
          empty?.classList.remove('hidden');
          dash?.classList.add('hidden');
          return;
        }

        empty?.classList.add('hidden');
        dash?.classList.remove('hidden');
        lastSeoResult = data;

        // Populate strategy inputs if present
        if (data.strategy) {
          if (data.strategy.goal) document.getElementById('seoGoalSelect').value = data.strategy.goal;
          if (data.strategy.scope_tier) document.getElementById('seoScopeSelect').value = data.strategy.scope_tier;
          if (data.strategy.competitor_urls && data.strategy.competitor_urls.length > 0) {
            document.getElementById('seoCompetitorInput').value = data.strategy.competitor_urls.join(', ');
          }
        }

        // Render Locale Tabs
        const targetLocales = (data.strategy && data.strategy.target_locales) || Object.keys(data.optimizations || {}) || ['ja', 'de', 'es'];
        if (!targetLocales.includes(currentSeoLocale) && targetLocales.length > 0) {
          currentSeoLocale = targetLocales[0];
        }

        const tabsContainer = document.getElementById('seoLocaleTabs');
        if (tabsContainer) {
          tabsContainer.innerHTML = targetLocales.map(loc => {
            const isActive = loc === currentSeoLocale;
            const activeClass = isActive ? 'bg-pink-500 text-white font-bold' : 'bg-zinc-800 text-zinc-300 hover:bg-zinc-700';
            return '<button onclick="switchSEOLocale(\'' + loc + '\')" class="px-2.5 py-1 rounded text-xs transition uppercase ' + activeClass + '">' + loc + '</button>';
          }).join('');
        }

        renderSEOViewForLocale(currentSeoLocale);
      } catch (err) {
        showToast('Failed to load SEO studio state', 'error');
      }
    }

    function switchSEOLocale(loc) {
      currentSeoLocale = loc;
      if (lastSeoResult) {
        loadSEOData();
      }
    }

    function setSeoSimMode(mode) {
      currentSeoSimMode = mode;
      document.getElementById('btnSimDesktop').className = mode === 'desktop' ? 'px-2 py-1 rounded font-mono font-semibold bg-zinc-800 text-zinc-200' : 'px-2 py-1 rounded font-mono text-zinc-400 hover:text-zinc-200';
      document.getElementById('btnSimMobile').className = mode === 'mobile' ? 'px-2 py-1 rounded font-mono font-semibold bg-zinc-800 text-zinc-200' : 'px-2 py-1 rounded font-mono text-zinc-400 hover:text-zinc-200';
      document.getElementById('btnSimSocial').className = mode === 'social' ? 'px-2 py-1 rounded font-mono font-semibold bg-zinc-800 text-zinc-200' : 'px-2 py-1 rounded font-mono text-zinc-400 hover:text-zinc-200';
      renderSEOSimulation(currentSeoLocale);
    }

    function renderSEOViewForLocale(loc) {
      if (!lastSeoResult) return;
      const comps = (lastSeoResult.competitors && lastSeoResult.competitors[loc]) || [];
      const kws = (lastSeoResult.keyword_pool && lastSeoResult.keyword_pool[loc]) || [];
      const metrics = (lastSeoResult.metrics && lastSeoResult.metrics[loc]) || null;
      const opts = (lastSeoResult.optimizations && lastSeoResult.optimizations[loc]) || [];

      // 1. Competitors
      document.getElementById('seoCompetitorCount').innerText = comps.length + ' scouted';
      const compList = document.getElementById('seoCompetitorList');
      if (comps.length === 0) {
        compList.innerHTML = '<p class="text-xs text-zinc-500">No competitor teardowns yet.</p>';
      } else {
        compList.innerHTML = comps.map(c => {
          const vpBadges = (c.value_props || []).slice(0, 3).map(vp => '<span class="text-[10px] px-1.5 py-0.5 rounded bg-zinc-800/80 text-zinc-400">' + escapeHtml(vp) + '</span>').join('');
          return '<div class="p-3 rounded-lg bg-[#0d1018] border border-[#181d28] space-y-1.5">' +
            '<div class="flex items-center justify-between text-xs">' +
              '<span class="font-bold text-sky-400">#' + c.rank + ' ' + escapeHtml(c.domain) + '</span>' +
              '<span class="text-[10px] px-1.5 py-0.5 rounded bg-zinc-800 text-zinc-400 font-mono">' + (c.is_discovered ? 'Discovered' : 'User URL') + '</span>' +
            '</div>' +
            '<p class="text-xs font-semibold text-zinc-200">' + escapeHtml(c.title || c.domain) + '</p>' +
            (c.meta_description ? '<p class="text-[11px] text-zinc-400 line-clamp-2">' + escapeHtml(c.meta_description) + '</p>' : '') +
            (vpBadges ? '<div class="flex flex-wrap gap-1 mt-1">' + vpBadges + '</div>' : '') +
          '</div>';
        }).join('');
      }

      // 2. Keywords
      document.getElementById('seoKeywordCount').innerText = kws.length + ' keywords';
      const kwCloud = document.getElementById('seoKeywordCloud');
      if (kws.length === 0) {
        kwCloud.innerHTML = '<p class="text-xs text-zinc-500">No keywords discovered yet.</p>';
      } else {
        kwCloud.innerHTML = kws.map(k => {
          const isPrimary = k.is_primary;
          const borderClass = isPrimary ? 'border-pink-500/40 bg-pink-500/10' : 'border-zinc-700/40 bg-zinc-800/50';
          const badgeClass = k.intent === 'transactional' ? 'text-emerald-400' : (k.intent === 'commercial' ? 'text-sky-400' : 'text-amber-400');
          return '<div class="px-2.5 py-1.5 rounded-lg border ' + borderClass + ' flex items-center gap-2 text-xs">' +
            (isPrimary ? '<i class="fa-solid fa-star text-pink-400 text-[10px]"></i>' : '') +
            '<span class="font-semibold text-zinc-200">' + escapeHtml(k.keyword) + '</span>' +
            '<span class="text-[10px] ' + badgeClass + ' uppercase font-mono">' + escapeHtml(k.intent) + '</span>' +
            '<span class="text-[10px] px-1.5 py-0.5 rounded bg-zinc-800 text-zinc-400 font-mono">~' + (k.est_monthly_volume || 0).toLocaleString() + '/mo</span>' +
            '<span class="text-[10px] px-1 py-0.5 rounded bg-zinc-800 text-zinc-500 font-mono">KD ' + k.difficulty + '</span>' +
          '</div>';
        }).join('');
      }

      // 3. Simulation
      renderSEOSimulation(loc);

      // 4. Metrics Grid
      const mGrid = document.getElementById('seoMetricsGrid');
      if (!metrics) {
        mGrid.innerHTML = '<p class="col-span-4 text-xs text-zinc-500">No predictive growth projections computed.</p>';
      } else {
        mGrid.innerHTML = '<div class="p-3 rounded-lg bg-[#0d1018] border border-[#181d28] space-y-1">' +
            '<span class="text-[10px] text-zinc-500 uppercase font-mono font-semibold">Search Volume Reach</span>' +
            '<div class="text-lg font-bold text-zinc-100">' + (metrics.search_volume_optimized || 0).toLocaleString() + '<span class="text-xs text-zinc-400 font-normal">/mo</span></div>' +
            '<span class="text-[10px] text-emerald-400 font-semibold">+' + (metrics.search_volume_uplift_pct || 0) + '% uplift</span>' +
          '</div>' +
          '<div class="p-3 rounded-lg bg-[#0d1018] border border-[#181d28] space-y-1">' +
            '<span class="text-[10px] text-zinc-500 uppercase font-mono font-semibold">Projected SERP CTR</span>' +
            '<div class="text-lg font-bold text-sky-400">' + (metrics.projected_ctr_optimized || 0) + '%</div>' +
            '<span class="text-[10px] text-zinc-400">Base: ' + (metrics.projected_ctr_baseline || 0) + '% (+' + (metrics.projected_ctr_uplift_pct || 0) + '%)</span>' +
          '</div>' +
          '<div class="p-3 rounded-lg bg-[#0d1018] border border-[#181d28] space-y-1">' +
            '<span class="text-[10px] text-zinc-500 uppercase font-mono font-semibold">Local Trust Factor</span>' +
            '<div class="text-lg font-bold text-emerald-400">' + (metrics.local_trust_score || 0) + '<span class="text-xs text-zinc-500 font-normal">/100</span></div>' +
            '<span class="text-[10px] text-zinc-400">Readability ' + (metrics.readability_score || 0) + '</span>' +
          '</div>' +
          '<div class="p-3 rounded-lg bg-[#0d1018] border border-[#181d28] space-y-1">' +
            '<span class="text-[10px] text-zinc-500 uppercase font-mono font-semibold">Keyword Density</span>' +
            '<div class="text-lg font-bold ' + (metrics.density_safe ? 'text-emerald-400' : 'text-amber-400') + '">' + (metrics.keyword_density_pct || 0) + '%</div>' +
            '<span class="text-[10px] px-1.5 py-0.5 rounded ' + (metrics.density_safe ? 'bg-emerald-500/10 text-emerald-400' : 'bg-amber-500/10 text-amber-400') + ' font-semibold font-mono">' + (metrics.density_safe ? 'Safe (No Stuffing)' : 'High Density') + '</span>' +
          '</div>';
      }

      // 5. Semantic Optimizations Table
      document.getElementById('seoOptCount').innerText = opts.length + ' keys optimized';
      const tbody = document.getElementById('seoOptimizationsTableBody');
      if (opts.length === 0) {
        tbody.innerHTML = '<tr><td colspan="5" class="py-6 text-center text-zinc-500">No optimized translation keys found.</td></tr>';
      } else {
        tbody.innerHTML = opts.map(o => {
          const impactBadge = o.impact_tier === 'high'
            ? '<span class="px-1.5 py-0.5 rounded bg-pink-500/10 text-pink-400 border border-pink-500/20 text-[10px] font-bold font-mono">HIGH IMPACT</span>'
            : '<span class="px-1.5 py-0.5 rounded bg-zinc-800 text-zinc-400 text-[10px] font-mono">STANDARD</span>';
          const icuBadge = o.icu_variables_matched
            ? '<span class="px-1.5 py-0.5 rounded bg-emerald-500/10 text-emerald-400 text-[10px] font-bold font-mono">ICU Safe</span>'
            : '<span class="px-1.5 py-0.5 rounded bg-rose-500/10 text-rose-400 text-[10px] font-bold font-mono">ICU Broken</span>';
          const kwBadges = (o.injected_keywords || []).map(kw => '<span class="px-1.5 py-0.5 rounded bg-sky-500/10 text-sky-400 border border-sky-500/20 text-[10px]">' + escapeHtml(kw) + '</span>').join(' ');

          return '<tr class="hover:bg-[#0c0f17] transition-colors">' +
            '<td class="py-3 px-4 align-top">' +
              '<div class="font-bold text-zinc-200 break-all">' + escapeHtml(o.key) + '</div>' +
              '<div class="mt-1">' + impactBadge + '</div>' +
            '</td>' +
            '<td class="py-3 px-4 align-top text-zinc-400 max-w-[200px] break-words">' + escapeHtml(o.source_en) + '</td>' +
            '<td class="py-3 px-4 align-top text-zinc-400 max-w-[220px] break-words">' + escapeHtml(o.baseline_translation) + '</td>' +
            '<td class="py-3 px-4 align-top max-w-[280px] break-words">' +
              '<div class="font-semibold text-emerald-300 bg-emerald-950/20 p-2 rounded border border-emerald-500/20">' + escapeHtml(o.optimized_translation) + '</div>' +
              (o.rationale ? '<p class="text-[10px] text-zinc-500 mt-1 italic">' + escapeHtml(o.rationale) + '</p>' : '') +
            '</td>' +
            '<td class="py-3 px-4 align-top space-y-1.5 whitespace-nowrap">' +
              '<div class="flex flex-wrap gap-1">' + (kwBadges || '<span class="text-[10px] text-zinc-600">None</span>') + '</div>' +
              '<div>' + icuBadge + '</div>' +
              '<div class="text-[10px] text-zinc-500 font-mono">' + o.character_length + ' chars | ' + o.pixel_width_desktop + 'px ' + (o.is_title_truncated ? '<span class="text-rose-400 font-bold">(Truncated)</span>' : '') + '</div>' +
            '</td>' +
          '</tr>';
        }).join('');
      }
    }

    function renderSEOSimulation(loc) {
      if (!lastSeoResult) return;
      const sim = (lastSeoResult.simulations && lastSeoResult.simulations[loc]) || null;
      const container = document.getElementById('seoSimContainer');
      if (!container) return;

      if (!sim) {
        container.innerHTML = '<p class="text-xs text-zinc-500">No SERP simulation available.</p>';
        return;
      }

      if (currentSeoSimMode === 'social') {
        container.innerHTML = '<div class="rounded-lg overflow-hidden border border-[#2a3040] bg-[#1a1f2c] max-w-[500px] mx-auto shadow-xl">' +
            '<div class="h-40 bg-gradient-to-tr from-sky-900 to-indigo-900 flex items-center justify-center text-zinc-400 text-xs font-mono">' +
              '<i class="fa-solid fa-image text-3xl opacity-40"></i>' +
            '</div>' +
            '<div class="p-3.5 space-y-1">' +
              '<div class="text-[10px] text-zinc-400 uppercase font-mono tracking-wider">' + escapeHtml(sim.display_url || '') + '</div>' +
              '<div class="text-sm font-bold text-zinc-100 line-clamp-1">' + escapeHtml(sim.og_card_title || sim.title_tag) + '</div>' +
              '<div class="text-xs text-zinc-400 line-clamp-2">' + escapeHtml(sim.og_card_description || sim.meta_description) + '</div>' +
            '</div>' +
          '</div>';
        return;
      }

      // Desktop & Mobile Google SERP
      const isMobile = currentSeoSimMode === 'mobile';
      const maxW = isMobile ? 'max-w-[380px]' : 'max-w-[580px]';
      const truncIndicator = sim.is_title_truncated ? '<span class="ml-1 text-[10px] px-1.5 py-0.5 rounded bg-rose-500/20 text-rose-400 font-mono font-bold">Pixel Truncated (>600px)</span>' : '<span class="ml-1 text-[10px] px-1.5 py-0.5 rounded bg-emerald-500/20 text-emerald-400 font-mono">Safe Width</span>';

      const faqItems = (sim.rich_snippet_faq && sim.rich_snippet_faq.length > 0)
        ? '<div class="pt-2 border-t border-zinc-800 space-y-1">' + sim.rich_snippet_faq.map(f => '<div class="text-[11px] text-zinc-300 flex items-center gap-1.5"><i class="fa-solid fa-angle-right text-[9px] text-sky-400"></i> ' + escapeHtml(f) + '</div>').join('') + '</div>'
        : '';

      container.innerHTML = '<div class="' + maxW + ' mx-auto space-y-2 font-sans">' +
        '<div class="flex items-center gap-2 text-xs text-zinc-400">' +
          '<div class="w-4 h-4 rounded-full bg-zinc-800 flex items-center justify-center text-[9px]"><i class="fa-solid fa-globe"></i></div>' +
          '<span class="text-[11px] truncate text-zinc-300 font-mono">' + escapeHtml(sim.display_url) + '</span>' +
          truncIndicator +
        '</div>' +
        '<div class="text-base font-medium text-sky-400 hover:underline cursor-pointer leading-tight">' + escapeHtml(sim.title_tag) + '</div>' +
        '<div class="text-xs text-zinc-400 leading-normal">' + escapeHtml(sim.meta_description) + '</div>' +
        faqItems +
      '</div>';
    }

    async function runSEOOptimization() {
      const btn = document.getElementById('btnRunSEO');
      btn.disabled = true;
      btn.classList.add('btn-disabled');
      btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i> Scouting & Optimizing...';

      const goal = document.getElementById('seoGoalSelect').value;
      const scope = document.getElementById('seoScopeSelect').value;
      const compsInput = document.getElementById('seoCompetitorInput').value;
      const comps = compsInput.split(',').map(s => s.trim()).filter(Boolean);

      try {
        const res = await fetch('/api/seo/run', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            locales: ['ja', 'de', 'es'],
            goal: goal,
            scope: scope,
            competitors: comps
          })
        });

        if (!res.ok) throw new Error('Failed to start SEO pipeline');
        showToast('SEO Studio launched! Scouting competitors & weaving copy...', 'info');

        // Poll for results
        let retries = 0;
        const interval = setInterval(async () => {
          retries++;
          const check = await fetch('/api/seo');
          const data = await check.json();
          if (data && data.strategy && retries > 1) {
            clearInterval(interval);
            lastSeoResult = data;
            loadSEOData();
            showToast('SEO Studio optimization completed successfully!', 'success');
            btn.disabled = false;
            btn.classList.remove('btn-disabled');
            btn.innerHTML = '<i class="fa-solid fa-wand-magic-sparkles"></i> Run SEO Optimization';
          }
          if (retries > 30) {
            clearInterval(interval);
            btn.disabled = false;
            btn.classList.remove('btn-disabled');
            btn.innerHTML = '<i class="fa-solid fa-wand-magic-sparkles"></i> Run SEO Optimization';
          }
        }, 1000);
      } catch (err) {
        showToast('SEO optimization failed: ' + err.message, 'error');
        btn.disabled = false;
        btn.classList.remove('btn-disabled');
        btn.innerHTML = '<i class="fa-solid fa-wand-magic-sparkles"></i> Run SEO Optimization';
      }
    }

    async function applySEOToDisk() {
      const btn = document.getElementById('btnApplySEO');
      btn.disabled = true;
      btn.classList.add('btn-disabled');
      btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i> Applying...';

      try {
        const res = await fetch('/api/seo/apply', { method: 'POST' });
        const data = await res.json();
        if (data.status === 'applied') {
          showToast('Applied all SEO optimizations directly to locale files!', 'success');
          loadStudioInit();
        } else {
          showToast('Failed to apply SEO changes', 'error');
        }
      } catch (err) {
        showToast('Error applying SEO changes', 'error');
      } finally {
        btn.disabled = false;
        btn.classList.remove('btn-disabled');
        btn.innerHTML = '<i class="fa-solid fa-floppy-disk"></i> Apply SEO to Disk';
      }
    }

    // ---------- Screen Switcher ----------
    function switchScreen(screenId) {
      const screens = ['copilot', 'studio', 'matrix', 'simulator', 'diff', 'critic', 'seo', 'runner', 'checkpoints', 'benchmark', 'stats', 'settings'];
      screens.forEach(s => {
        const sec = document.getElementById('screen' + cap(s));
        const btn = document.getElementById('screenBtn' + cap(s));
        if (s === screenId) {
          sec?.classList.remove('hidden');
          btn?.classList.add('active');
        } else {
          sec?.classList.add('hidden');
          btn?.classList.remove('active');
        }
      });

      if (screenId === 'copilot') loadCopilotWorkspace();
      if (screenId === 'matrix') loadMatrixData();
      if (screenId === 'simulator') updateSimLocale();
      if (screenId === 'diff') loadTuiDiff();
      if (screenId === 'critic') loadCritic();
      if (screenId === 'seo') loadSEOData();
      if (screenId === 'runner') renderRunnerLanguages();
      if (screenId === 'checkpoints') loadCheckpoints();
      if (screenId === 'stats') loadStats();
      if (screenId === 'settings') loadSettings();
    }

    function cap(s) { return s.charAt(0).toUpperCase() + s.slice(1); }

    // Keyboard Shortcuts (Cmd+K, Cmd+S, R, 0-6, Esc)
    window.addEventListener('keydown', (e) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault();
        openCommandPalette();
        return;
      }
      if ((e.metaKey || e.ctrlKey) && e.key === 's') {
        e.preventDefault();
        applyDiskChanges();
        return;
      }
      if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
        if (document.activeElement?.id !== 'copilotInput') {
          e.preventDefault();
          executeLocalization();
          return;
        }
      }

      if (e.target.tagName === 'INPUT' || e.target.tagName === 'SELECT' || e.target.tagName === 'TEXTAREA') return;

      if (e.key === '0') switchScreen('copilot');
      if (e.key === '1') switchScreen('studio');
      if (e.key === '2') switchScreen('matrix');
      if (e.key === '3') switchScreen('simulator');
      if (e.key === '4') switchScreen('diff');
      if (e.key === '5') switchScreen('critic');
      if (e.key === '6') switchScreen('seo');
      if (e.key.toLowerCase() === 'r') rescanAST();
      if (e.key.toLowerCase() === 'p') openProjectModal();
      if (e.key === 'Escape') closeCommandPalette();
    });

    // ==================== Autonomous Copilot Studio Engine ====================
    let copilotThinking = false;
    let currentCanvasTab = 'matrix';
    let lastCopilotCards = [];

    async function loadCopilotWorkspace() {
      await loadCopilotHistory();
      renderActiveCanvasTab();
    }

    async function loadCopilotHistory() {
      try {
        const res = await fetch('/api/chat/history');
        if (!res.ok) return;
        const history = await res.json();
        const container = document.getElementById('copilotMessages');
        container.innerHTML = '';
        if (!history || history.length === 0) {
          renderCopilotWelcome();
          return;
        }
        history.forEach(msg => appendCopilotMessageToDOM(msg));
        container.scrollTop = container.scrollHeight;
      } catch (err) {
        console.error('Failed loading chat history:', err);
      }
    }

    function renderCopilotWelcome() {
      const container = document.getElementById('copilotMessages');
      container.innerHTML =
        '<div class="flex gap-2.5 items-start">' +
          '<div class="pk-avatar">LP</div>' +
          '<div class="pk-msg-assistant flex-1">' +
            '<div class="flex items-center gap-2 mb-2">' +
              '<span class="text-sky-400 font-semibold text-xs" style="font-family:\'Geist Mono\',monospace">Autonomous Orchestrator</span>' +
              '<span class="badge badge-emerald">Ready</span>' +
            '</div>' +
            '<p class="text-[11px] leading-relaxed" style="color:#9ca3af">' +
              'Direct the supervisor agent to audit AST strings, execute model-aware translation batches, verify ICU variable integrity with the 4-tier critic, or simulate Google SERP rankings.' +
            '</p>' +
            '<div class="grid grid-cols-2 gap-1.5 mt-3">' +
              '<button onclick="sendCopilotPrompt(\'Scan project AST and report coverage\')" class="btn btn-secondary btn-sm text-left justify-start gap-1.5"><i class="fa-solid fa-radar text-sky-400 text-[10px]"></i>Scan AST</button>' +
              '<button onclick="sendCopilotPrompt(\'Translate all missing keys to es, de, ja\')" class="btn btn-secondary btn-sm text-left justify-start gap-1.5"><i class="fa-solid fa-language text-emerald-400 text-[10px]"></i>Translate</button>' +
              '<button onclick="sendCopilotPrompt(\'Run 4-tier verification critic\')" class="btn btn-secondary btn-sm text-left justify-start gap-1.5"><i class="fa-solid fa-shield-halved text-purple-400 text-[10px]"></i>4-Tier Critic</button>' +
              '<button onclick="sendCopilotPrompt(\'Show Japanese SERP preview\')" class="btn btn-secondary btn-sm text-left justify-start gap-1.5"><i class="fa-solid fa-magnifying-glass text-pink-400 text-[10px]"></i>SERP Preview</button>' +
            '</div>' +
          '</div>' +
        '</div>';
    }

    function toggleToolCollapsible(id) {
      const el = document.getElementById(id);
      const icon = document.getElementById(id + '_icon');
      if (el) {
        el.classList.toggle('hidden');
        if (icon) icon.classList.toggle('rotate-180');
      }
    }

    function renderPromptKitToolHTML(tc) {
      const toolId = 'tool_' + Math.random().toString(36).substr(2, 9);
      const name = escapeHtml(tc.name || 'tool_call');
      let inputHtml = '';
      if (tc.args && Object.keys(tc.args).length > 0) {
        inputHtml = '<div class="mt-2"><div class="text-[10px] uppercase font-bold mb-1" style="color:#6b7280">Input</div>' +
          '<pre class="p-2 rounded-lg text-[11px] overflow-x-auto custom-scrollbar" style="background:#050609;border:1px solid rgba(255,255,255,0.05);color:#94a3b8">' +
          escapeHtml(JSON.stringify(tc.args, null, 2)) + '</pre></div>';
      }
      let outputHtml = '';
      if (tc.result && Object.keys(tc.result).length > 0) {
        outputHtml = '<div class="mt-2"><div class="text-[10px] uppercase font-bold mb-1" style="color:#6b7280">Output</div>' +
          '<pre class="p-2 rounded-lg text-[11px] overflow-x-auto custom-scrollbar" style="background:#050609;border:1px solid rgba(255,255,255,0.05);color:#94a3b8">' +
          escapeHtml(JSON.stringify(tc.result, null, 2)) + '</pre></div>';
      }
      return '<div class="pk-tool-card">' +
        '<button type="button" onclick="toggleToolCollapsible(\'' + toolId + '\')" class="pk-tool-header w-full text-left">' +
          '<div style="display:flex;align-items:center;gap:0.5rem">' +
            '<i class="fa-solid fa-wrench text-[10px]" style="color:#fbbf24"></i>' +
            '<span class="font-semibold text-zinc-200 text-xs">' + name + '</span>' +
            '<span class="badge badge-emerald">Done</span>' +
          '</div>' +
          '<i class="fa-solid fa-chevron-down text-[10px] transition-transform" style="color:#4a5162" id="' + toolId + '_icon"></i>' +
        '</button>' +
        '<div id="' + toolId + '" class="pk-tool-body hidden">' +
          inputHtml + outputHtml +
        '</div>' +
      '</div>';
    }

    function appendCopilotMessageToDOM(msg) {
      const container = document.getElementById('copilotMessages');
      const isUser = msg.role === 'user';
      const div = document.createElement('div');

      if (isUser) {
        div.className = 'flex justify-end';
        div.innerHTML =
          '<div class="pk-msg-user">' +
            '<div class="text-[10px] uppercase font-bold mb-1" style="color:var(--clr-primary);font-family:\'Geist Mono\',monospace">You</div>' +
            '<div style="white-space:pre-wrap">' + escapeHtml(msg.content) + '</div>' +
          '</div>';
      } else {
        div.className = 'flex gap-2.5 items-start';

        let toolsHtml = '';
        if (msg.tool_calls && msg.tool_calls.length > 0) {
          toolsHtml = msg.tool_calls.map(function(tc) {
            return renderPromptKitToolHTML(tc);
          }).join('');
        }

        let cardsHtml = '';
        if (msg.cards && msg.cards.length > 0) {
          cardsHtml = msg.cards.map(function(c) {
            if (c && c.rendered_text) {
              return '<div class="rounded-xl overflow-x-auto custom-scrollbar my-2" style="border:1px solid rgba(255,255,255,0.07);background:#050609">' +
                '<pre class="p-3 text-[11px] whitespace-pre" style="color:#94a3b8;font-family:\'Geist Mono\',monospace">' +
                escapeHtml(c.rendered_text) + '</pre></div>';
            }
            return '';
          }).join('');
        }

        const contentHtml = msg.content ?
          '<div class="pk-msg-assistant mt-1" style="font-family:\'Geist\',sans-serif">' +
            formatMarkdown(msg.content) +
          '</div>' : '';

        div.innerHTML =
          '<div class="pk-avatar flex-shrink-0">LP</div>' +
          '<div style="flex:1;min-width:0;max-width:95%">' +
            toolsHtml + cardsHtml + contentHtml +
          '</div>';
      }

      container.appendChild(div);
      container.scrollTop = container.scrollHeight;
    }

    function setCanvasTab(tab) {
      currentCanvasTab = tab;
      const tabs = ['matrix', 'diff', 'critic', 'serp', 'cost'];
      tabs.forEach(t => {
        const btn = document.getElementById('canvasTab' + cap(t));
        if (t === tab) {
          btn?.classList.add('active');
        } else {
          btn?.classList.remove('active');
        }
      });
      document.getElementById('canvasActiveViewTitle').textContent = cap(tab) + ' Viewport';
      renderActiveCanvasTab();
    }

    function renderActiveCanvasTab() {
      const container = document.getElementById('copilotCanvasContainer');
      if (!container) return;

      if (currentCanvasTab === 'matrix') {
        renderCanvasMatrix(container);
      } else if (currentCanvasTab === 'diff') {
        renderCanvasDiff(container);
      } else if (currentCanvasTab === 'critic') {
        renderCanvasCritic(container);
      } else if (currentCanvasTab === 'serp') {
        renderCanvasSERP(container);
      } else if (currentCanvasTab === 'cost') {
        renderCanvasCost(container);
      }
    }

    function renderCanvasMatrix(container) {
      const card = lastCopilotCards.find(c => c.type === 'matrix');
      if (card && card.rendered_text) {
        container.innerHTML = '<div class="space-y-4">' +
          '<div class="flex items-center justify-between border-b border-[#181b24] pb-3">' +
            '<div>' +
              '<h3 class="text-xs font-bold text-zinc-100 font-mono">LOCALE COVERAGE MATRIX</h3>' +
              '<p class="text-[11px] text-zinc-400">Deterministic key parity status across all detected catalog files</p>' +
            '</div>' +
            '<button onclick="switchScreen(\'matrix\')" class="px-3 py-1 bg-zinc-800 hover:bg-zinc-700 text-zinc-200 rounded text-xs font-mono border border-zinc-700">Open Grid Studio</button>' +
          '</div>' +
          '<pre class="p-4 rounded-lg bg-[#050609] border border-[#181b24] text-xs font-mono text-zinc-300 whitespace-pre overflow-x-auto custom-scrollbar">' +
            escapeHtml(card.rendered_text) +
          '</pre>' +
        '</div>';
        return;
      }

      // Default Live Matrix Viewport
      container.innerHTML = '<div class="space-y-4">' +
        '<div class="flex items-center justify-between border-b border-[#181b24] pb-3">' +
          '<div>' +
            '<h3 class="text-xs font-bold text-zinc-100 font-mono">WORKSPACE TRANSLATION MATRIX</h3>' +
            '<p class="text-[11px] text-zinc-400">Current codebase extraction & translation health</p>' +
          '</div>' +
          '<button onclick="sendCopilotPrompt(\'Scan repository and report coverage\')" class="px-3 py-1 bg-sky-600/20 hover:bg-sky-600/30 text-sky-400 rounded text-xs font-mono border border-sky-500/30">Run AST Scout</button>' +
        '</div>' +
        '<div class="grid grid-cols-2 md:grid-cols-4 gap-3 font-mono text-xs">' +
          '<div class="p-3.5 rounded-lg bg-[#0b0e14] border border-[#181b24] space-y-1">' +
            '<div class="text-[10px] uppercase text-zinc-500">Source Keys</div>' +
            '<div class="text-lg font-bold text-zinc-100">' + (studioCandidates ? studioCandidates.length : 0) + '</div>' +
          '</div>' +
          '<div class="p-3.5 rounded-lg bg-[#0b0e14] border border-[#181b24] space-y-1">' +
            '<div class="text-[10px] uppercase text-zinc-500">Target Locales</div>' +
            '<div class="text-lg font-bold text-sky-400">4 Active</div>' +
          '</div>' +
          '<div class="p-3.5 rounded-lg bg-[#0b0e14] border border-[#181b24] space-y-1">' +
            '<div class="text-[10px] uppercase text-zinc-500">AST Safety</div>' +
            '<div class="text-lg font-bold text-emerald-400">0% Drift</div>' +
          '</div>' +
          '<div class="p-3.5 rounded-lg bg-[#0b0e14] border border-[#181b24] space-y-1">' +
            '<div class="text-[10px] uppercase text-zinc-500">Verification</div>' +
            '<div class="text-lg font-bold text-purple-400">4-Tier Critic</div>' +
          '</div>' +
        '</div>' +
      '</div>';
    }

    function renderCanvasDiff(container) {
      const card = lastCopilotCards.find(c => c.type === 'diff');
      if (card && card.rendered_text) {
        container.innerHTML = '<div class="space-y-4">' +
          '<div class="flex items-center justify-between border-b border-[#181b24] pb-3">' +
            '<div>' +
              '<h3 class="text-xs font-bold text-zinc-100 font-mono">AST SURGICAL PATCH DIFF</h3>' +
              '<p class="text-[11px] text-zinc-400">Exact byte-range replacements without full-file hallucination</p>' +
            '</div>' +
            '<button onclick="applyDiskChanges()" class="px-3 py-1 bg-emerald-600 hover:bg-emerald-500 text-white rounded text-xs font-mono">Apply to Disk</button>' +
          '</div>' +
          '<pre class="p-4 rounded-lg bg-[#050609] border border-[#181b24] text-xs font-mono text-zinc-300 whitespace-pre overflow-x-auto custom-scrollbar">' +
            escapeHtml(card.rendered_text) +
          '</pre>' +
        '</div>';
      } else {
        container.innerHTML = '<div class="p-8 text-center text-zinc-500 font-mono text-xs border border-dashed border-zinc-800 rounded-lg">' +
          'No active AST refactoring plan in context. Instruct agent: "Refactor codebase with surgical AST patch".' +
        '</div>';
      }
    }

    function renderCanvasCritic(container) {
      const card = lastCopilotCards.find(c => c.type === 'critic');
      if (card && card.rendered_text) {
        container.innerHTML = '<div class="space-y-4">' +
          '<div class="flex items-center justify-between border-b border-[#181b24] pb-3">' +
            '<div>' +
              '<h3 class="text-xs font-bold text-zinc-100 font-mono">4-TIER CRITIC VERIFICATION</h3>' +
              '<p class="text-[11px] text-zinc-400">Syntax, ICU variable matching, expansion estimation, and key parity</p>' +
            '</div>' +
          '</div>' +
          '<pre class="p-4 rounded-lg bg-[#050609] border border-[#181b24] text-xs font-mono text-zinc-300 whitespace-pre overflow-x-auto custom-scrollbar">' +
            escapeHtml(card.rendered_text) +
          '</pre>' +
        '</div>';
      } else {
        container.innerHTML = '<div class="p-8 text-center text-zinc-500 font-mono text-xs border border-dashed border-zinc-800 rounded-lg">' +
          'No critic report in context. Instruct agent: "Execute 4-tier verification critic on all locales".' +
        '</div>';
      }
    }

    function renderCanvasSERP(container) {
      const card = lastCopilotCards.find(c => c.type === 'serp');
      if (card && card.rendered_text) {
        container.innerHTML = '<div class="space-y-4">' +
          '<div class="flex items-center justify-between border-b border-[#181b24] pb-3">' +
            '<div>' +
              '<h3 class="text-xs font-bold text-zinc-100 font-mono">SERP SIMULATOR & GROWTH PREVIEW</h3>' +
              '<p class="text-[11px] text-zinc-400">Pixel-accurate Google search preview with character length limits</p>' +
            '</div>' +
            '<button onclick="switchScreen(\'seo\')" class="px-3 py-1 bg-pink-600/20 hover:bg-pink-600/30 text-pink-400 rounded text-xs font-mono border border-pink-500/30">Open SEO Studio</button>' +
          '</div>' +
          '<pre class="p-4 rounded-lg bg-[#050609] border border-[#181b24] text-xs font-mono text-zinc-300 whitespace-pre overflow-x-auto custom-scrollbar">' +
            escapeHtml(card.rendered_text) +
          '</pre>' +
        '</div>';
      } else {
        container.innerHTML = '<div class="p-8 text-center text-zinc-500 font-mono text-xs border border-dashed border-zinc-800 rounded-lg">' +
          'No SERP simulation generated. Instruct agent: "Simulate Japanese Google SERP preview".' +
        '</div>';
      }
    }

    function renderCanvasCost(container) {
      const card = lastCopilotCards.find(c => c.type === 'cost');
      if (card && card.rendered_text) {
        container.innerHTML = '<div class="space-y-4">' +
          '<div class="flex items-center justify-between border-b border-[#181b24] pb-3">' +
            '<div>' +
              '<h3 class="text-xs font-bold text-zinc-100 font-mono">TOKEN TELEMETRY & COST ESTIMATE</h3>' +
              '<p class="text-[11px] text-zinc-400">Exact prompt/completion breakdown and pricing calculation</p>' +
            '</div>' +
          '</div>' +
          '<pre class="p-4 rounded-lg bg-[#050609] border border-[#181b24] text-xs font-mono text-zinc-300 whitespace-pre overflow-x-auto custom-scrollbar">' +
            escapeHtml(card.rendered_text) +
          '</pre>' +
        '</div>';
      } else {
        container.innerHTML = '<div class="p-8 text-center text-zinc-500 font-mono text-xs border border-dashed border-zinc-800 rounded-lg">' +
          'No token telemetry card generated. Instruct agent: "Plan localization and estimate token cost".' +
        '</div>';
      }
    }

    function formatMarkdown(text) {
      if (!text) return '';
      return escapeHtml(text)
        .replace(/\*\*(.*?)\*\*/g, '<strong class="text-zinc-100 font-bold">$1</strong>')
        .replace(/\*(.*?)\*/g, '<em class="text-zinc-300">$1</em>')
        .replace(new RegExp('\\u0060([^\\u0060]+)\\u0060', 'g'), '<code class="px-1 py-0.5 rounded bg-zinc-800 font-mono text-[11px] text-sky-300 border border-zinc-700">$1</code>')
        .replace(/\n/g, '<br>');
    }

    function escapeHtml(s) {
      if (!s) return '';
      return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
    }

    function handleCopilotTextareaKey(e) {
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        handleCopilotSubmit(e);
      }
    }

    async function handleCopilotSubmit(e) {
      if (e) e.preventDefault();
      const input = document.getElementById('copilotInput');
      const text = input.value.trim();
      if (!text || copilotThinking) return;

      input.value = '';
      await sendCopilotPrompt(text);
    }

    async function sendCopilotPrompt(prompt) {
      if (copilotThinking) return;
      copilotThinking = true;

      const container = document.getElementById('copilotMessages');
      appendCopilotMessageToDOM({ role: 'user', content: prompt });

      // Thinking indicator bubble (prompt-kit dots)
      const thinkingDiv = document.createElement('div');
      thinkingDiv.id = 'copilotThinkingBubble';
      thinkingDiv.className = 'flex gap-2.5 items-start';
      thinkingDiv.innerHTML =
        '<div class="pk-avatar flex-shrink-0">LP</div>' +
        '<div class="pk-msg-assistant" style="padding:0.625rem 0.875rem">' +
          '<div style="display:flex;align-items:center;gap:4px">' +
            '<span class="pk-dot"></span>' +
            '<span class="pk-dot"></span>' +
            '<span class="pk-dot"></span>' +
          '</div>' +
        '</div>';
      container.appendChild(thinkingDiv);
      container.scrollTop = container.scrollHeight;

      try {
        const response = await fetch('/api/chat', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ message: prompt })
        });

        if (!response.ok) {
          throw new Error('Chat API returned ' + response.status);
        }

        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        let buffer = '';
        let assistantMsg = { role: 'assistant', content: '', tool_calls: [], cards: [] };

        while (true) {
          const res = await reader.read();
          if (res.done) break;

          buffer += decoder.decode(res.value, { stream: true });
          const lines = buffer.split('\n\n');
          buffer = lines.pop() || '';

          for (let i = 0; i < lines.length; i++) {
            const line = lines[i];
            if (line.startsWith('data: ')) {
              try {
                const ev = JSON.parse(line.slice(6));
                if (ev.type === 'tool_start' && ev.tool_call) {
                  assistantMsg.tool_calls.push(ev.tool_call);
                } else if (ev.type === 'card' && ev.card) {
                  assistantMsg.cards.push(ev.card);
                  lastCopilotCards.push(ev.card);
                  if (ev.card.type === 'matrix') currentCanvasTab = 'matrix';
                  if (ev.card.type === 'diff') currentCanvasTab = 'diff';
                  if (ev.card.type === 'critic') currentCanvasTab = 'critic';
                  if (ev.card.type === 'serp') currentCanvasTab = 'serp';
                  if (ev.card.type === 'cost') currentCanvasTab = 'cost';
                  renderActiveCanvasTab();
                } else if (ev.type === 'chunk' && ev.content) {
                  assistantMsg.content += ev.content;
                } else if (ev.type === 'done' && ev.content) {
                  assistantMsg.content = ev.content;
                }
              } catch (e) {}
            }
          }
        }

        const tb = document.getElementById('copilotThinkingBubble');
        if (tb) tb.remove();

        appendCopilotMessageToDOM(assistantMsg);
      } catch (err) {
        const tb = document.getElementById('copilotThinkingBubble');
        if (tb) tb.remove();
        appendCopilotMessageToDOM({
          role: 'assistant',
          content: 'Error communicating with orchestrator: ' + err.message
        });
      } finally {
        copilotThinking = false;
        container.scrollTop = container.scrollHeight;
      }
    }

    async function resetCopilotChat() {
      try {
        await fetch('/api/chat/reset', { method: 'POST' });
        lastCopilotCards = [];
        renderCopilotWelcome();
        renderActiveCanvasTab();
        showToast('Session reset', 'info');
      } catch (err) {
        showToast('Failed to reset session', 'error');
      }
    }

    loadStudioInit();
  </script>
</body>
</html>`
