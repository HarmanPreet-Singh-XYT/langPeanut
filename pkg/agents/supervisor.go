package agents

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/langPeanut/langPeanut/pkg/memory"
	"github.com/langPeanut/langPeanut/pkg/orchestrator"
	"github.com/langPeanut/langPeanut/pkg/platforms"
	"github.com/langPeanut/langPeanut/pkg/trajectory"
	"github.com/langPeanut/langPeanut/pkg/types"
)

// SupervisorAgent coordinates the entire multi-agent localization lifecycle
type SupervisorAgent struct {
	Platform      platforms.Platform
	Scout         *ASTScoutAgent
	Context       *ContextAgent
	Patch         *PatchEngine
	Translator    *TranslatorAgent
	Critic        *VerifierCriticAgent
	Repair        *CodeRepairAgent
	Checkpoint    *orchestrator.CheckpointManager
	Logger        *trajectory.Logger
	ProjectMemory *memory.ProjectMemory
	ProjectRoot   string
	OnProgress    func(stage string)
}

func NewSupervisorAgent(projectRoot string, p platforms.Platform) (*SupervisorAgent, error) {
	cacheDir := filepath.Join(projectRoot, ".langPeanut", "cache")
	tm, _ := memory.NewTranslationMemory(cacheDir)
	pm, _ := memory.NewProjectMemory(cacheDir)

	trajDir := filepath.Join(projectRoot, "trajectories")
	logger, _ := trajectory.NewLogger(trajDir, time.Now().Format("20060102-150405"))

	ckpt, _ := orchestrator.NewCheckpointManager(projectRoot)

	return &SupervisorAgent{
		Platform:      p,
		Scout:         NewASTScoutAgent(p),
		Context:       NewContextAgent(),
		Patch:         NewPatchEngine(),
		Translator:    NewTranslatorAgent(tm, pm),
		Critic:        NewVerifierCriticAgent(),
		Repair:        NewCodeRepairAgent(),
		Checkpoint:    ckpt,
		Logger:        logger,
		ProjectMemory: pm,
		ProjectRoot:   projectRoot,
	}, nil
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
	CodeRepairs         []types.CodeRepairResult    `json:"code_repairs,omitempty"`
	UnresolvedErrors    []types.CompilerDiagnostic  `json:"unresolved_errors,omitempty"`
	TrajectoryJSONPath  string                      `json:"trajectory_json_path"`
	TrajectoryMDPath    string                      `json:"trajectory_md_path"`
	CheckpointID        string                      `json:"checkpoint_id"`
}

// RunEndToEnd executes the full autonomous multi-agent pipeline with reflection loops
func (s *SupervisorAgent) RunEndToEnd(ctx context.Context, sourceLocale string, targetLocales []string, dryRun bool) (*PipelineResult, error) {
	result := &PipelineResult{
		TargetLocaleFiles: make(map[string]string),
	}

	// --- Step 1: AST Scout Agent (Candidate Extraction) ---
	if s.OnProgress != nil {
		s.OnProgress("🚀 [1/5] AST Scout: Scanning project files & extracting candidates...")
	}
	s.Logger.LogStep("ASTScoutAgent", "ScanProject", "Scanning source files using AST queries", "ExtractCandidates", s.ProjectRoot, nil, "", 0, true)
	scanReport, err := s.Scout.ScanProject(s.ProjectRoot, "")
	if err != nil {
		return nil, fmt.Errorf("scout failed: %w", err)
	}
	result.ScannedFilesCount = scanReport.TotalFilesScanned
	result.ExtractedCandidates = scanReport.TotalCandidates

	sourceEntries := make(map[string]string)
	rawSourcePath := s.Platform.DefaultSourceFile(s.ProjectRoot, sourceLocale)
	if !filepath.IsAbs(rawSourcePath) {
		result.SourceLocaleFile = filepath.Join(s.ProjectRoot, rawSourcePath)
	} else {
		result.SourceLocaleFile = rawSourcePath
	}

	// Pre-load existing source locale catalog if present (e.g. en.json, strings.xml, app_en.arb)
	if data, err := os.ReadFile(result.SourceLocaleFile); err == nil {
		if locData, err := s.Platform.ParseLocaleFile(data, filepath.Ext(result.SourceLocaleFile)); err == nil && locData != nil {
			for k, v := range locData.Entries {
				sourceEntries[k] = v
			}
		}
	}

	var candidates []types.StringCandidate
	if len(scanReport.Candidates) > 0 {
		// --- Step 2: Context & Disambiguation Agent ---
		if s.OnProgress != nil {
			s.OnProgress(fmt.Sprintf("🧠 [2/5] Context Agent: Disambiguating %d candidates & synthesizing keys...", len(scanReport.Candidates)))
		}
		s.Logger.LogStep("ContextAgent", "DisambiguateAndEnhance", "Analyzing component hierarchies and sibling strings for semantic keys", "Disambiguate", len(scanReport.Candidates), nil, "", 0, true)
		cands, err := s.Context.DisambiguateAndEnhance(scanReport.Candidates)
		if err == nil {
			candidates = cands
		}
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

	if len(sourceEntries) == 0 {
		return result, nil
	}

	for f := range candidatesByFile {
		touchedFiles = append(touchedFiles, f)
	}

	result.UniqueKeysCount = len(sourceEntries)

	// Baseline pre-flight typecheck & AST snapshot (to isolate pre-existing errors from new errors)
	baselineDiags, _ := platforms.RunDiagnostics(s.ProjectRoot, touchedFiles)
	baselineMap := make(map[string]bool)
	for _, d := range baselineDiags {
		baselineMap[fmt.Sprintf("%s:%d:%s", filepath.Clean(d.FilePath), d.Line, d.Message)] = true
	}

	// --- Step 3: Checkpoint Manager (Pre-run snapshot) ---
	if !dryRun && s.Checkpoint != nil {
		if s.OnProgress != nil {
			s.OnProgress("🛡️  [3/5] Checkpoint Manager: Creating safety rollback snapshot...")
		}
		manifest, _ := s.Checkpoint.CreateCheckpoint("pre-run", "Pre-run snapshot before AST refactoring", touchedFiles)
		if manifest != nil {
			result.CheckpointID = manifest.ID
		}
	}

	// --- Step 4: Deterministic AST Range Patch Engine ---
	refactorPlans := make(map[string]types.FileRefactorPlan)
	if len(candidatesByFile) > 0 {
		if s.OnProgress != nil {
			s.OnProgress(fmt.Sprintf("⚡ [4/5] Patch Engine: Applying surgical AST byte-range diffs across %d files...", len(candidatesByFile)))
		}
	}
	for filePath, fileCandidates := range candidatesByFile {
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		plan, err := s.Platform.GenerateRefactorPlan(filePath, content, fileCandidates)
		if err != nil {
			continue
		}

		refactored, err := s.Patch.ApplyRefactorPlan(plan)
		if err != nil {
			s.Logger.LogStep("PatchEngine", "ApplyRefactorPlan", "Patch validation error detected", "ValidateSyntax", filePath, err.Error(), err.Error(), 1, false)
			return nil, fmt.Errorf("patch engine syntax error on %s: %w", filePath, err)
		}

		refactorPlans[filePath] = *plan
		result.RefactoredFiles = append(result.RefactoredFiles, filePath)
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

	if len(targetLocales) > 0 && s.OnProgress != nil {
		s.OnProgress(fmt.Sprintf("🌐 [5/5] Cultural Translator: Translating %d keys into [%s] (5 parallel workers)...", len(sourceEntries), strings.Join(targetLocales, ", ")))
	}

	// Limit simultaneous language translations to 5 concurrent worker goroutines
	langSem := make(chan struct{}, 5)

	for _, tgtLoc := range targetLocales {
		result.TargetLocaleFiles[tgtLoc] = s.Platform.DefaultSourceFile(s.ProjectRoot, tgtLoc)
		wg.Add(1)
		go func(loc string) {
			defer wg.Done()
			langSem <- struct{}{}
			defer func() { <-langSem }()

			s.Logger.LogStep("TranslatorAgent", "TranslateLocale", fmt.Sprintf("Translating %d keys into %s", len(sourceEntries), loc), "Translate", loc, nil, "", 0, true)
			locData, err := s.Translator.TranslateLocale(ctx, sourceEntries, sourceLocale, loc, criticFeedback)
			mu.Lock()
			defer mu.Unlock()
			if err != nil && transErr == nil {
				transErr = err
			} else {
				targetLocaleDataMap[loc] = locData
				result.GeneratedLocales = append(result.GeneratedLocales, loc)
			}
		}(tgtLoc)
	}
	wg.Wait()
	if transErr != nil {
		return nil, transErr
	}

	// --- Step 6: 4-Tier Critic & Reflection Loop ---
	if s.OnProgress != nil {
		s.OnProgress("🔍 Verifier Critic: Validating AST syntax, ICU variables & key parity...")
	}
	s.Logger.LogStep("VerifierCriticAgent", "VerifyAll", "Executing 4-Tier verification check", "Verify", len(targetLocales), nil, "", 0, true)
	report := s.Critic.VerifyAll(sourceLocaleData, targetLocaleDataMap, refactorPlans)

	// Automated Self-Correction Loop (Up to 2 Retries if any diagnostic error exists)
	retryCount := 0
	for !report.Passed && retryCount < 2 {
		retryCount++
		if s.OnProgress != nil {
			s.OnProgress(fmt.Sprintf("🔄 Critic Self-Correction: Reflection retry %d/2 for %d diagnostic errors...", retryCount, report.ErrorCount))
		}
		s.Logger.LogStep("VerifierCriticAgent", "SelfCorrectionLoop", fmt.Sprintf("Verification failed with %d error(s). Initiating reflection retry %d", report.ErrorCount, retryCount), "Retry", report.Diagnostics, nil, "", retryCount, false)

		// Feed diagnostic hints back into translation
		for _, diag := range report.Diagnostics {
			if diag.Key != "" && diag.AutoFixHint != "" {
				criticFeedback[diag.Key] = diag.AutoFixHint
			}
		}

		// Re-run translations with feedback
		for _, tgtLoc := range targetLocales {
			locData, _ := s.Translator.TranslateLocale(ctx, sourceEntries, sourceLocale, tgtLoc, criticFeedback)
			targetLocaleDataMap[tgtLoc] = locData
		}

		// Re-verify
		report = s.Critic.VerifyAll(sourceLocaleData, targetLocaleDataMap, refactorPlans)
	}

	result.VerificationReport = report

	// --- Step 7: Write to Disk (if not dryRun) ---
	if !dryRun {
		if s.OnProgress != nil {
			s.OnProgress("💾 Saving formatted locale catalogs & refactored code to disk...")
		}
		// Save refactored source files
		for filePath, plan := range refactorPlans {
			if plan.RefactoredContent != "" {
				_ = os.WriteFile(filePath, []byte(plan.RefactoredContent), 0644)
			}
		}

		// Post-refactor compiler & AST diagnostics verification
		postDiags, _ := platforms.RunDiagnostics(s.ProjectRoot, touchedFiles)
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
				s.OnProgress(fmt.Sprintf("🔧 [Auto-Repair] Detected %d new compiler error(s). Initiating AI code repair...", len(newDiags)))
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

		// Save Translation Memory
		if s.Translator.Memory != nil {
			_ = s.Translator.Memory.Save()
		}
	}

	// --- Step 8: Trajectory Export ---
	jsonPath, _ := s.Logger.ExportJSON()
	mdPath, _ := s.Logger.ExportMarkdown()
	result.TrajectoryJSONPath = jsonPath
	result.TrajectoryMDPath = mdPath

	return result, nil
}
