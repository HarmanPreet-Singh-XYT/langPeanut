package agents

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/langPeanut/langPeanut/pkg/logger"
	"github.com/langPeanut/langPeanut/pkg/memory"
	"github.com/langPeanut/langPeanut/pkg/orchestrator"
	"github.com/langPeanut/langPeanut/pkg/platforms"
	"github.com/langPeanut/langPeanut/pkg/trajectory"
	"github.com/langPeanut/langPeanut/pkg/types"
)

// SupervisorAgent coordinates the entire multi-agent localization lifecycle
type SupervisorAgent struct {
	Platform         platforms.Platform
	Scout            *ASTScoutAgent
	Context          *ContextAgent
	Patch            *PatchEngine
	Translator       *TranslatorAgent
	Critic           *VerifierCriticAgent
	Repair           *CodeRepairAgent
	Directive        *DirectiveAgent
	Checkpoint       *orchestrator.CheckpointManager
	Logger           *trajectory.Logger
	ProjectMemory    *memory.ProjectMemory
	ProjectRoot      string
	UserDirective    string
	CustomInstallCmd string
	CustomBuildCmd   string
	ExistingMode     string // "skip" (default), "replace" (regenerate all), "prompt"
	OnProgress       func(stage string)
}

func NewSupervisorAgent(projectRoot string, p platforms.Platform) (*SupervisorAgent, error) {
	// Automatically ignore runtime directories in version control
	_ = memory.EnsureGitignore(projectRoot)

	cacheDir := filepath.Join(projectRoot, ".langPeanut", "cache")
	tm, _ := memory.NewTranslationMemory(cacheDir)
	pm, _ := memory.NewProjectMemory(cacheDir)

	trajDir := filepath.Join(projectRoot, "trajectories")
	diagLogger, _ := trajectory.NewLogger(trajDir, time.Now().Format("20060102-150405"))

	ckpt, _ := orchestrator.NewCheckpointManager(projectRoot)

	cfg := memory.LoadConfig(projectRoot)
	translator := NewTranslatorAgent(tm, pm)
	customInstall := ""
	customBuild := ""
	existingMode := "skip"
	if cfg != nil {
		translator.ChunkWordBudget = cfg.ChunkWordBudget
		translator.ChunkKeyCeiling = cfg.ChunkKeyCeiling
		translator.Concurrency = cfg.Concurrency
		customInstall = cfg.CustomInstallCmd
		customBuild = cfg.CustomBuildCmd
		if cfg.ExistingTranslationsMode != "" {
			existingMode = cfg.ExistingTranslationsMode
		}
	}

	directiveAgent := NewDirectiveAgent()
	if translator.LLM != nil {
		directiveAgent = NewDirectiveAgentWithClient(translator.LLM)
	}

	return &SupervisorAgent{
		Platform:         p,
		Scout:            NewASTScoutAgent(p),
		Context:          NewContextAgent(),
		Patch:            NewPatchEngine(),
		Translator:       translator,
		Critic:           NewVerifierCriticAgent(),
		Repair:           NewCodeRepairAgent(),
		Directive:        directiveAgent,
		Checkpoint:       ckpt,
		Logger:           diagLogger,
		ProjectMemory:    pm,
		ProjectRoot:      projectRoot,
		CustomInstallCmd: customInstall,
		CustomBuildCmd:   customBuild,
		ExistingMode:     existingMode,
	}, nil
}

// mergeLocaleEntries combines an existing on-disk locale catalog with newly
// translated entries. Newly translated values win on key collision (e.g. a
// critic-driven retranslation correcting a flagged key); everything else
// from the existing catalog is preserved untouched.
func mergeLocaleEntries(existing, fresh map[string]string) map[string]string {
	merged := make(map[string]string, len(existing)+len(fresh))
	for k, v := range existing {
		merged[k] = v
	}
	for k, v := range fresh {
		merged[k] = v
	}
	return merged
}

type PipelineResult struct {
	ScannedFilesCount   int                         `json:"scanned_files_count"`
	ExtractedCandidates int                         `json:"extracted_candidates"`
	UniqueKeysCount     int                         `json:"unique_keys_count"`
	RefactoredFiles     []string                    `json:"refactored_files"`
	GeneratedLocales    []string                    `json:"generated_locales"`
	SourceLocaleFile    string                      `json:"source_locale_file"`
	TargetLocaleFiles   map[string]string           `json:"target_locale_files"`
	VerificationReport  *types.VerificationReport   `json:"verification_report"`
	DependencyStatus    *types.DependencyStatus     `json:"dependency_status,omitempty"`
	CodeRepairs         []types.CodeRepairResult    `json:"code_repairs,omitempty"`
	DirectiveResult     *types.DirectiveResult      `json:"directive_result,omitempty"`
	UnresolvedErrors    []types.CompilerDiagnostic  `json:"unresolved_errors,omitempty"`
	DiagnosticAdvice    *logger.DiagnosticAdvice `json:"diagnostic_advice,omitempty"`
	ExecutionLogs       []logger.LogEvent        `json:"execution_logs,omitempty"`
	TrajectoryJSONPath  string                   `json:"trajectory_json_path"`
	TrajectoryMDPath    string                      `json:"trajectory_md_path"`
	CheckpointID        string                      `json:"checkpoint_id"`
}

// RunEndToEnd executes the full autonomous multi-agent pipeline with reflection loops
func (s *SupervisorAgent) RunEndToEnd(ctx context.Context, sourceLocale string, targetLocales []string, dryRun bool) (*PipelineResult, error) {
	result := &PipelineResult{
		TargetLocaleFiles: make(map[string]string),
	}

	logger.Get().Info("SUPERVISOR", fmt.Sprintf("Starting multi-agent localization pipeline: source=%s, targets=%v, dryRun=%v", sourceLocale, targetLocales, dryRun))

	// --- Step 1: AST Scout Agent (Candidate Extraction) ---
	if s.OnProgress != nil {
		s.OnProgress("[1/5] AST Scout: Scanning project files & extracting candidates...")
	}
	s.Logger.LogStep("ASTScoutAgent", "ScanProject", "Scanning source files using AST queries", "ExtractCandidates", s.ProjectRoot, nil, "", 0, true)
	scanReport, err := s.Scout.ScanProject(s.ProjectRoot, "")
	if err != nil {
		advice := logger.ExplainError(err)
		logger.Get().Error("SCOUT", fmt.Sprintf("AST scanning failed on project root %s: %v", s.ProjectRoot, err), err)
		return nil, fmt.Errorf("scout failed: %w%s", err, advice.FormatCLI())
	}
	result.ScannedFilesCount = scanReport.TotalFilesScanned
	result.ExtractedCandidates = scanReport.TotalCandidates
	logger.Get().Info("SCOUT", fmt.Sprintf("Scanned %d files, discovered %d candidates", scanReport.TotalFilesScanned, scanReport.TotalCandidates))

	sourceEntries := make(map[string]string)
	rawSourcePath := s.Platform.DefaultSourceFile(s.ProjectRoot, sourceLocale)
	if !filepath.IsAbs(rawSourcePath) {
		result.SourceLocaleFile = filepath.Join(s.ProjectRoot, rawSourcePath)
	} else {
		result.SourceLocaleFile = rawSourcePath
	}

	// Discover locale catalogs that already exist on disk for this project
	existingLocaleFiles, _ := s.Platform.DiscoverExistingLocales(s.ProjectRoot)
	existingTargetData := make(map[string]types.LocaleData)

	// Pre-load existing source locale catalog if present
	if srcPath, found := existingLocaleFiles[sourceLocale]; found {
		result.SourceLocaleFile = srcPath
		if data, err := os.ReadFile(srcPath); err == nil {
			if locData, err := s.Platform.ParseLocaleFileForLocale(data, sourceLocale); err == nil && locData != nil {
				for k, v := range locData.Entries {
					sourceEntries[k] = v
				}
			}
		}
	} else if data, err := os.ReadFile(result.SourceLocaleFile); err == nil {
		if locData, err := s.Platform.ParseLocaleFile(data, filepath.Ext(result.SourceLocaleFile)); err == nil && locData != nil {
			for k, v := range locData.Entries {
				sourceEntries[k] = v
			}
		}
	}

	// Pre-load all target locale catalogs
	for locale, path := range existingLocaleFiles {
		if locale == sourceLocale {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		locData, err := s.Platform.ParseLocaleFileForLocale(data, locale)
		if err != nil || locData == nil {
			continue
		}
		existingTargetData[locale] = *locData
	}
	if len(existingLocaleFiles) > 0 {
		logger.Get().Info("SUPERVISOR", fmt.Sprintf("Discovered existing locale catalogs on disk: %d locales already present (%d keys in source catalog)", len(existingLocaleFiles), len(sourceEntries)))
	}

	var candidates []types.StringCandidate
	if len(scanReport.Candidates) > 0 {
		// --- Step 2: Context & Disambiguation Agent ---
		if s.OnProgress != nil {
			s.OnProgress(fmt.Sprintf("[2/5] Context Agent: Disambiguating %d candidates & synthesizing keys...", len(scanReport.Candidates)))
		}
		s.Logger.LogStep("ContextAgent", "DisambiguateAndEnhance", "Analyzing component hierarchies and sibling strings for semantic keys", "Disambiguate", len(scanReport.Candidates), nil, "", 0, true)
		cands, err := s.Context.DisambiguateAndEnhance(scanReport.Candidates)
		if err == nil {
			candidates = cands
		}
		logger.Get().Info("CONTEXT", fmt.Sprintf("Disambiguated and assigned %d semantic keys across components", len(candidates)))
	}

	// Group candidates by file
	candidatesByFile := make(map[string][]types.StringCandidate)
	var touchedFiles []string

	for _, c := range candidates {
		if c.Classification == types.ClassLocalizable && c.Approved {
			candidatesByFile[c.FilePath] = append(candidatesByFile[c.FilePath], c)
			sourceEntries[c.Key] = c.CleanValue
		}
	}

	// Also extract any i18n keys already referenced in code (e.g. t('key'), AppLocalizations.of()!.key)
	// that may not yet be defined in the locale catalog files
	refKeys := extractReferencedKeys(s.ProjectRoot, s.Platform.FileExtensions(), s.Platform.SkipDirs())
	for k, v := range refKeys {
		if _, ok := sourceEntries[k]; !ok {
			sourceEntries[k] = v
		}
	}

	if len(sourceEntries) == 0 {
		logger.Get().Warn("SUPERVISOR", "No localizable string entries or language catalog keys found in project")
		return result, nil
	}

	for f := range candidatesByFile {
		touchedFiles = append(touchedFiles, f)
	}

	result.UniqueKeysCount = len(sourceEntries)

	// Baseline pre-flight typecheck & AST snapshot (to isolate pre-existing errors from new errors)
	baselineDiags, _ := platforms.RunDiagnosticsWithCustom(s.ProjectRoot, touchedFiles, s.CustomBuildCmd)
	baselineMap := make(map[string]bool)
	for _, d := range baselineDiags {
		baselineMap[fmt.Sprintf("%s:%d:%s", filepath.Clean(d.FilePath), d.Line, d.Message)] = true
	}

	// --- Step 3: Checkpoint Manager (Pre-run snapshot) ---
	if !dryRun && s.Checkpoint != nil {
		if s.OnProgress != nil {
			s.OnProgress("[3/5] Checkpoint Manager: Creating safety rollback snapshot...")
		}
		manifest, _ := s.Checkpoint.CreateCheckpoint("pre-run", "Pre-run snapshot before AST refactoring", touchedFiles)
		if manifest != nil {
			result.CheckpointID = manifest.ID
			logger.Get().Info("CHECKPOINT", fmt.Sprintf("Created safety rollback snapshot: %s (%d files protected)", manifest.ID, len(touchedFiles)))
		}
	}

	// --- Step 4: Deterministic AST Range Patch Engine ---
	refactorPlans := make(map[string]types.FileRefactorPlan)
	if len(candidatesByFile) > 0 {
		if s.OnProgress != nil {
			s.OnProgress(fmt.Sprintf("[4/5] Patch Engine: Applying surgical AST byte-range diffs across %d files...", len(candidatesByFile)))
		}
	}
	for filePath, fileCandidates := range candidatesByFile {
		content, err := os.ReadFile(filePath)
		if err != nil {
			logger.Get().Warn("PATCH", fmt.Sprintf("Could not read file %s: %v", filePath, err))
			continue
		}

		plan, err := s.Platform.GenerateRefactorPlan(filePath, content, fileCandidates)
		if err != nil {
			logger.Get().Warn("PATCH", fmt.Sprintf("Could not generate refactor plan for %s: %v", filePath, err))
			continue
		}

		refactored, err := s.Patch.ApplyRefactorPlan(plan)
		if err != nil {
			s.Logger.LogStep("PatchEngine", "ApplyRefactorPlan", "Patch validation error detected", "ValidateSyntax", filePath, err.Error(), err.Error(), 1, false)
			advice := logger.ExplainError(err)
			logger.Get().Error("PATCH", fmt.Sprintf("In-memory AST validation rejected patch on %s: %v", filePath, err), err)
			return nil, fmt.Errorf("patch engine syntax error on %s: %w%s", filePath, err, advice.FormatCLI())
		}

		refactorPlans[filePath] = *plan
		result.RefactoredFiles = append(result.RefactoredFiles, filePath)
		logger.Get().Debug("PATCH", fmt.Sprintf("Verified AST patch for %s (%d replacements)", filePath, len(plan.Patches)))
		_ = refactored
	}

	// --- Step 5: Cultural Translator Agent ---
	sourceLocaleData := types.LocaleData{
		LocaleCode: sourceLocale,
		Entries:    sourceEntries,
	}

	targetLocaleDataMap := make(map[string]types.LocaleData)
	criticFeedback := make(map[string]string)

	var mu sync.Mutex
	var wg sync.WaitGroup
	var transErr error

	activeModelName := "unknown"
	if s.Translator != nil && s.Translator.LLM != nil {
		activeModelName = s.Translator.LLM.Description()
	}

	if len(targetLocales) > 0 && s.OnProgress != nil {
		s.OnProgress(fmt.Sprintf("[5/5] Cultural Translator: Translating %d keys into [%s] via %s...", len(sourceEntries), strings.Join(targetLocales, ", "), activeModelName))
	}
	logger.Get().Info("TRANSLATOR", fmt.Sprintf("Translating %d keys into %v using engine '%s'", len(sourceEntries), targetLocales, activeModelName))

	// Limit simultaneous language translations to concurrent worker goroutines
	langConcurrency := 5
	if s.Translator != nil && s.Translator.Concurrency > 0 {
		langConcurrency = s.Translator.Concurrency
	}
	langSem := make(chan struct{}, langConcurrency)

	for _, tgtLoc := range targetLocales {
		result.TargetLocaleFiles[tgtLoc] = s.Platform.DefaultSourceFile(s.ProjectRoot, tgtLoc)
		wg.Add(1)
		go func(loc string) {
			defer wg.Done()
			langSem <- struct{}{}
			defer func() { <-langSem }()

			// Determine keys to translate based on ExistingMode
			existing := existingTargetData[loc]
			toTranslate := make(map[string]string)
			isReplaceMode := s.ExistingMode == "replace" || s.ExistingMode == "overwrite"

			if isReplaceMode {
				for k, v := range sourceEntries {
					toTranslate[k] = v
				}
			} else {
				// Default "skip": only translate missing or empty entries
				for k, v := range sourceEntries {
					val, ok := existing.Entries[k]
					if !ok || strings.TrimSpace(val) == "" {
						toTranslate[k] = v
					}
				}
			}

			if len(toTranslate) == 0 {
				mu.Lock()
				merged := existing
				merged.LocaleCode = loc
				targetLocaleDataMap[loc] = merged
				result.GeneratedLocales = append(result.GeneratedLocales, loc)
				mu.Unlock()
				logger.Get().Info("TRANSLATOR", fmt.Sprintf("Locale '%s' already has all %d keys; skipping translation", loc, len(sourceEntries)))
				return
			}

			modeDesc := s.ExistingMode
			if modeDesc == "" {
				modeDesc = "skip"
			}
			s.Logger.LogStep("TranslatorAgent", "TranslateLocale", fmt.Sprintf("Translating %d key(s) into %s (%d already present, mode: %s)", len(toTranslate), loc, len(existing.Entries), modeDesc), "Translate", loc, nil, "", 0, true)
			locData, err := s.Translator.TranslateLocale(ctx, toTranslate, sourceLocale, loc, criticFeedback)
			mu.Lock()
			defer mu.Unlock()
			if err != nil && transErr == nil {
				transErr = err
			} else {
				if isReplaceMode {
					locData.LocaleCode = loc
					locData.Metadata = existing.Metadata
					targetLocaleDataMap[loc] = locData
				} else {
					locData.Entries = mergeLocaleEntries(existing.Entries, locData.Entries)
					locData.LocaleCode = loc
					locData.Metadata = existing.Metadata
					targetLocaleDataMap[loc] = locData
				}
				result.GeneratedLocales = append(result.GeneratedLocales, loc)
				logger.Get().Info("TRANSLATOR", fmt.Sprintf("Locale '%s' generated: %d translated, %d total entries (mode: %s)", loc, len(toTranslate), len(locData.Entries), modeDesc))
			}
		}(tgtLoc)
	}
	wg.Wait()
	if transErr != nil {
		advice := logger.ExplainError(transErr)
		logger.Get().Error("TRANSLATOR", fmt.Sprintf("Translation failed: %v", transErr), transErr)
		return nil, fmt.Errorf("translation failed: %w%s", transErr, advice.FormatCLI())
	}

	// --- Step 6: 4-Tier Critic & Reflection Loop ---
	if s.OnProgress != nil {
		s.OnProgress("Verifier Critic: Validating AST syntax, ICU variables & key parity...")
	}
	s.Logger.LogStep("VerifierCriticAgent", "VerifyAll", "Executing 4-Tier verification check", "Verify", len(targetLocales), nil, "", 0, true)
	report := s.Critic.VerifyAll(sourceLocaleData, targetLocaleDataMap, refactorPlans)

	// Automated Self-Correction Loop (Up to 2 Retries if any diagnostic error exists)
	retryCount := 0
	for !report.Passed && retryCount < 2 {
		retryCount++
		if s.OnProgress != nil {
			s.OnProgress(fmt.Sprintf("Critic Self-Correction: Reflection retry %d/2 for %d diagnostic errors...", retryCount, report.ErrorCount))
		}
		s.Logger.LogStep("VerifierCriticAgent", "SelfCorrectionLoop", fmt.Sprintf("Verification failed with %d error(s). Initiating reflection retry %d", report.ErrorCount, retryCount), "Retry", report.Diagnostics, nil, "", retryCount, false)

		// Feed diagnostic hints back into translation
		for _, diag := range report.Diagnostics {
			if diag.Key != "" && diag.AutoFixHint != "" {
				criticFeedback[diag.Key] = diag.AutoFixHint
			}
		}

		// Re-run translation only for the keys the critic flagged, merging the
		// corrected values into what's already been translated/discovered —
		// a full re-translate here would re-translate (and risk overwriting)
		// every key on every retry, including ones sourced from an existing
		// on-disk catalog that never needed correction.
		flaggedKeys := make(map[string]string)
		for key := range criticFeedback {
			if v, ok := sourceEntries[key]; ok {
				flaggedKeys[key] = v
			}
		}
		if len(flaggedKeys) > 0 {
			for _, tgtLoc := range targetLocales {
				locData, err := s.Translator.TranslateLocale(ctx, flaggedKeys, sourceLocale, tgtLoc, criticFeedback)
				if err != nil {
					continue
				}
				prev := targetLocaleDataMap[tgtLoc]
				prev.Entries = mergeLocaleEntries(prev.Entries, locData.Entries)
				prev.LocaleCode = tgtLoc
				targetLocaleDataMap[tgtLoc] = prev
			}
		}

		// Re-verify
		report = s.Critic.VerifyAll(sourceLocaleData, targetLocaleDataMap, refactorPlans)
	}

	result.VerificationReport = report

	// --- Step 7: Write to Disk (if not dryRun) ---
	if !dryRun {
		if s.OnProgress != nil {
			s.OnProgress("Saving formatted locale catalogs & refactored code to disk...")
		}
		// Save refactored source files
		for filePath, plan := range refactorPlans {
			if plan.RefactoredContent != "" {
				_ = os.WriteFile(filePath, []byte(plan.RefactoredContent), 0644)
			}
		}

		// Ensure and install framework localization dependencies (e.g. react-i18next, flutter_localizations)
		if s.OnProgress != nil {
			if s.CustomInstallCmd != "" {
				s.OnProgress(fmt.Sprintf("[Dependencies] Executing custom install command: %s...", s.CustomInstallCmd))
			} else {
				s.OnProgress("[Dependencies] Ensuring localization framework packages are installed...")
			}
		}
		s.Logger.LogStep("SupervisorAgent", "EnsureDependencies", "Checking and installing required framework localization dependencies", "Dependencies", s.Platform.DisplayName(), nil, "", 0, true)

		var depStatus *types.DependencyStatus
		var depErr error
		if s.CustomInstallCmd != "" {
			cmdStr, out, execErr := platforms.ExecuteCustomCommand(s.ProjectRoot, s.CustomInstallCmd)
			depStatus, depErr = s.Platform.EnsureDependencies(s.ProjectRoot, false)
			if depStatus != nil {
				depStatus.CommandExecuted = cmdStr
				depStatus.CommandOutput = out
				if execErr != nil {
					depStatus.Message = fmt.Sprintf("Custom install command executed with notice: %v", execErr)
				} else {
					depStatus.Message = fmt.Sprintf("Custom install command '%s' completed successfully", s.CustomInstallCmd)
				}
			}
		} else {
			depStatus, depErr = s.Platform.EnsureDependencies(s.ProjectRoot, true)
		}
		if depErr == nil && depStatus != nil {
			result.DependencyStatus = depStatus
			if depStatus.ManifestUpdated || len(depStatus.InstalledDeps) > 0 || depStatus.CommandExecuted != "" {
				logger.Get().Info("DEPENDENCIES", fmt.Sprintf("Localization dependencies ensured: %v (command: %s)", depStatus.InstalledDeps, depStatus.CommandExecuted))
			}
		}

		// Post-refactor compiler & AST diagnostics verification
		postDiags, _ := platforms.RunDiagnosticsWithCustom(s.ProjectRoot, touchedFiles, s.CustomBuildCmd)
		var newDiags []types.CompilerDiagnostic
		for _, d := range postDiags {
			key := fmt.Sprintf("%s:%d:%s", filepath.Clean(d.FilePath), d.Line, d.Message)
			if !baselineMap[key] {
				newDiags = append(newDiags, d)
			}
		}

		// Autonomous Code Repair Agent: heal newly introduced compiler regressions
		if len(newDiags) > 0 {
			if s.OnProgress != nil {
				s.OnProgress(fmt.Sprintf("[Auto-Repair] Detected %d new compiler error(s). Initiating AI code repair...", len(newDiags)))
			}
			newDiagsByFile := make(map[string][]types.CompilerDiagnostic)
			for _, d := range newDiags {
				newDiagsByFile[d.FilePath] = append(newDiagsByFile[d.FilePath], d)
			}

			for fPath, fileDiags := range newDiagsByFile {
				s.Logger.LogStep("CodeRepairAgent", "RepairFile", fmt.Sprintf("Autonomous self-healing attempt for %s", fPath), "AutoRepair", fileDiags, nil, "", 1, true)
				repairRes, err := s.Repair.RepairFile(ctx, s.ProjectRoot, fPath, fileDiags, s.Platform.Name())
				if err == nil && repairRes != nil {
					result.CodeRepairs = append(result.CodeRepairs, *repairRes)
					if !repairRes.Repaired {
						result.UnresolvedErrors = append(result.UnresolvedErrors, repairRes.RemainingErrors...)
						s.Logger.LogStep("CodeRepairAgent", "RepairFile", fmt.Sprintf("Compiler regression unresolved in %s (flagged for manual review)", fPath), "FlagManual", repairRes.RemainingErrors, nil, "", 2, false)
					} else {
						s.Logger.LogStep("CodeRepairAgent", "RepairFile", fmt.Sprintf("Successfully auto-healed compiler regression in %s", fPath), "Success", nil, nil, "", 1, true)
					}
				}
			}
		}

		// Save source locale file
		rawDir := s.Platform.DefaultLocaleDir(s.ProjectRoot)
		localeDir := rawDir
		if !filepath.IsAbs(localeDir) {
			localeDir = filepath.Join(s.ProjectRoot, rawDir)
		}
		_ = os.MkdirAll(localeDir, 0755)

		srcBytes, _ := s.Platform.FormatLocaleFile(sourceLocaleData)
		_ = os.WriteFile(result.SourceLocaleFile, srcBytes, 0644)

		// Save target locale files
		for tgtCode, tgtData := range targetLocaleDataMap {
			tgtBytes, _ := s.Platform.FormatLocaleFile(tgtData)
			rawTgtPath := s.Platform.DefaultSourceFile(s.ProjectRoot, tgtCode)
			tgtFilePath := rawTgtPath
			if !filepath.IsAbs(tgtFilePath) {
				tgtFilePath = filepath.Join(s.ProjectRoot, rawTgtPath)
			}
			tgtFileDir := filepath.Dir(tgtFilePath)
			_ = os.MkdirAll(tgtFileDir, 0755)
			_ = os.WriteFile(tgtFilePath, tgtBytes, 0644)
		}

		// Sync React/Next.js i18n bootstrap with all generated locales
		if s.Platform.Name() == types.FrameworkReact {
			allLocales := append([]string{sourceLocale}, targetLocales...)
			platforms.EnsureReactI18nBootstrapWithLocales(s.ProjectRoot, allLocales)
		}

		// Save Translation Memory
		if s.Translator.Memory != nil {
			_ = s.Translator.Memory.Save()
		}

		// --- Step 7: App Integration Directive Agent (Optional Post-Localization UI Execution) ---
		if s.UserDirective != "" && s.Directive != nil {
			if s.OnProgress != nil {
				s.OnProgress(fmt.Sprintf("[App Integration] Executing directive: %s...", s.UserDirective))
			}
			s.Logger.LogStep("DirectiveAgent", "ExecuteDirective", fmt.Sprintf("Executing user directive: %s", s.UserDirective), "AppIntegration", s.UserDirective, nil, "", 1, true)
			dirRes, err := s.Directive.ExecuteDirective(ctx, s.ProjectRoot, s.UserDirective, targetLocales, s.Platform.Name())
			if err == nil && dirRes != nil {
				result.DirectiveResult = dirRes
				if dirRes.Success {
					s.Logger.LogStep("DirectiveAgent", "ExecuteDirective", "Directive executed successfully", "Success", dirRes, nil, "", 1, true)
				} else {
					s.Logger.LogStep("DirectiveAgent", "ExecuteDirective", fmt.Sprintf("Directive partial/failed: %s", dirRes.Explanation), "Warning", dirRes, nil, "", 1, false)
				}
			}
		}
	}

	// --- Step 8: Trajectory Export ---
	jsonPath, _ := s.Logger.ExportJSON()
	mdPath, _ := s.Logger.ExportMarkdown()
	result.TrajectoryJSONPath = jsonPath
	result.TrajectoryMDPath = mdPath
	result.ExecutionLogs = logger.Get().GetRecent(50)

	return result, nil
}

var (
	reI18nKey1 = regexp.MustCompile(`(?:(?:\b|\$|i18n\.)t|translate|formatMessage)\(\s*['"]([a-zA-Z0-9_.\-]+)['"]`)
	reI18nKey2 = regexp.MustCompile(`AppLocalizations\.of\([^)]+\)!?\.([a-zA-Z0-9_]+)`)
	reI18nKey3 = regexp.MustCompile(`(?:NSLocalizedString|String\(localized:)\s*\(\s*['"]([a-zA-Z0-9_.\-]+)['"]`)
	reI18nKey4 = regexp.MustCompile(`(?:stringResource|getString)\s*\(\s*R\.string\.([a-zA-Z0-9_]+)`)
	reI18nKey5 = regexp.MustCompile(`(?:i18nKey|msgid)=['"]([a-zA-Z0-9_.\-]+)['"]`)
)

// extractReferencedKeys scans source files for i18n keys already referenced in code
// (e.g. t('flight_details'), AppLocalizations.of(context)!.welcome_user)
func extractReferencedKeys(projectRoot string, extList, skipDirs []string) map[string]string {
	keys := make(map[string]string)
	extMap := make(map[string]bool)
	for _, ext := range extList {
		extMap[ext] = true
	}
	skipMap := make(map[string]bool)
	for _, d := range skipDirs {
		skipMap[d] = true
	}

	_ = filepath.Walk(projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if skipMap[info.Name()] || strings.HasPrefix(info.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !extMap[filepath.Ext(path)] {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(data)

		for _, re := range []*regexp.Regexp{reI18nKey1, reI18nKey2, reI18nKey3, reI18nKey4, reI18nKey5} {
			matches := re.FindAllStringSubmatch(content, -1)
			for _, m := range matches {
				if len(m) > 1 && m[1] != "" {
					k := m[1]
					if _, ok := keys[k]; !ok {
						keys[k] = humanizeKey(k)
					}
				}
			}
		}
		return nil
	})

	return keys
}

// humanizeKey converts a key identifier like "flight_details" or "checkoutSummary" into readable English
func humanizeKey(key string) string {
	clean := strings.ReplaceAll(key, "_", " ")
	clean = strings.ReplaceAll(clean, ".", " ")
	clean = strings.ReplaceAll(clean, "-", " ")

	var words []string
	var cur strings.Builder
	for i, r := range clean {
		if i > 0 && r >= 'A' && r <= 'Z' && (clean[i-1] >= 'a' && clean[i-1] <= 'z') {
			words = append(words, cur.String())
			cur.Reset()
		}
		cur.WriteRune(r)
	}
	if cur.Len() > 0 {
		words = append(words, cur.String())
	}

	for i := range words {
		w := strings.TrimSpace(words[i])
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
		}
	}
	res := strings.Join(words, " ")
	if res == "" {
		return key
	}
	return res
}
