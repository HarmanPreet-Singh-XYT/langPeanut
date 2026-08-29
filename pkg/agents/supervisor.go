package agents

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/langPeanut/langPeanut/pkg/memory"
	"github.com/langPeanut/langPeanut/pkg/orchestrator"
	"github.com/langPeanut/langPeanut/pkg/platforms"
	"github.com/langPeanut/langPeanut/pkg/trajectory"
	"github.com/langPeanut/langPeanut/pkg/types"
)

// SupervisorAgent coordinates the entire multi-agent localization lifecycle
type SupervisorAgent struct {
	Platform    platforms.Platform
	Scout         *ASTScoutAgent
	Context       *ContextAgent
	Patch         *PatchEngine
	Translator    *TranslatorAgent
	Critic        *VerifierCriticAgent
	Checkpoint    *orchestrator.CheckpointManager
	Logger        *trajectory.Logger
	ProjectMemory *memory.ProjectMemory
	ProjectRoot   string
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
		Checkpoint:    ckpt,
		Logger:        logger,
		ProjectMemory: pm,
		ProjectRoot:   projectRoot,
	}, nil
}

type PipelineResult struct {
	ScannedFilesCount   int                         `json:"scanned_files_count"`
	ExtractedCandidates int                         `json:"extracted_candidates"`
	RefactoredFiles     []string                    `json:"refactored_files"`
	GeneratedLocales    []string                    `json:"generated_locales"`
	VerificationReport  *types.VerificationReport   `json:"verification_report"`
	TrajectoryJSONPath  string                      `json:"trajectory_json_path"`
	TrajectoryMDPath    string                      `json:"trajectory_md_path"`
	CheckpointID        string                      `json:"checkpoint_id"`
}

// RunEndToEnd executes the full autonomous multi-agent pipeline with reflection loops
func (s *SupervisorAgent) RunEndToEnd(ctx context.Context, sourceLocale string, targetLocales []string, dryRun bool) (*PipelineResult, error) {
	result := &PipelineResult{}

	// --- Step 1: AST Scout Agent (Candidate Extraction) ---
	s.Logger.LogStep("ASTScoutAgent", "ScanProject", "Scanning source files using AST queries", "ExtractCandidates", s.ProjectRoot, nil, "", 0, true)
	scanReport, err := s.Scout.ScanProject(s.ProjectRoot, "")
	if err != nil {
		return nil, fmt.Errorf("scout failed: %w", err)
	}
	result.ScannedFilesCount = scanReport.TotalFilesScanned
	result.ExtractedCandidates = scanReport.TotalCandidates

	if len(scanReport.Candidates) == 0 {
		return result, nil
	}

	// --- Step 2: Context & Disambiguation Agent ---
	s.Logger.LogStep("ContextAgent", "DisambiguateAndEnhance", "Analyzing component hierarchies and sibling strings for semantic keys", "Disambiguate", len(scanReport.Candidates), nil, "", 0, true)
	candidates, err := s.Context.DisambiguateAndEnhance(scanReport.Candidates)
	if err != nil {
		return nil, fmt.Errorf("context agent failed: %w", err)
	}

	// Group candidates by file
	candidatesByFile := make(map[string][]types.StringCandidate)
	var touchedFiles []string
	sourceEntries := make(map[string]string)

	for _, c := range candidates {
		if c.Classification == types.ClassLocalizable && c.Approved {
			candidatesByFile[c.FilePath] = append(candidatesByFile[c.FilePath], c)
			sourceEntries[c.Key] = c.CleanValue
		}
	}

	for f := range candidatesByFile {
		touchedFiles = append(touchedFiles, f)
	}

	// --- Step 3: Checkpoint Manager (Pre-run snapshot) ---
	if !dryRun && s.Checkpoint != nil {
		manifest, _ := s.Checkpoint.CreateCheckpoint("pre-run", "Pre-run snapshot before AST refactoring", touchedFiles)
		if manifest != nil {
			result.CheckpointID = manifest.ID
		}
	}

	// --- Step 4: Deterministic AST Range Patch Engine ---
	refactorPlans := make(map[string]types.FileRefactorPlan)
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

	for _, tgtLoc := range targetLocales {
		s.Logger.LogStep("TranslatorAgent", "TranslateLocale", fmt.Sprintf("Translating %d keys into %s", len(sourceEntries), tgtLoc), "Translate", tgtLoc, nil, "", 0, true)
		locData, err := s.Translator.TranslateLocale(ctx, sourceEntries, sourceLocale, tgtLoc, criticFeedback)
		if err != nil {
			return nil, err
		}
		targetLocaleDataMap[tgtLoc] = locData
		result.GeneratedLocales = append(result.GeneratedLocales, tgtLoc)
	}

	// --- Step 6: 4-Tier Critic & Reflection Loop ---
	s.Logger.LogStep("VerifierCriticAgent", "VerifyAll", "Executing 4-Tier verification check", "Verify", len(targetLocales), nil, "", 0, true)
	report := s.Critic.VerifyAll(sourceLocaleData, targetLocaleDataMap, refactorPlans)

	// Automated Self-Correction Loop (Up to 2 Retries if any diagnostic error exists)
	retryCount := 0
	for !report.Passed && retryCount < 2 {
		retryCount++
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
		// Save refactored source files
		for filePath, plan := range refactorPlans {
			if plan.RefactoredContent != "" {
				_ = os.WriteFile(filePath, []byte(plan.RefactoredContent), 0644)
			}
		}

		// Save source locale file
		localeDir := filepath.Join(s.ProjectRoot, s.Platform.DefaultLocaleDir(s.ProjectRoot))
		_ = os.MkdirAll(localeDir, 0755)

		srcBytes, _ := s.Platform.FormatLocaleFile(sourceLocaleData)
		srcFilePath := filepath.Join(s.ProjectRoot, s.Platform.DefaultSourceFile(s.ProjectRoot, sourceLocale))
		_ = os.WriteFile(srcFilePath, srcBytes, 0644)

		// Save target locale files
		for tgtCode, tgtData := range targetLocaleDataMap {
			tgtBytes, _ := s.Platform.FormatLocaleFile(tgtData)
			tgtFilePath := filepath.Join(s.ProjectRoot, s.Platform.DefaultSourceFile(s.ProjectRoot, tgtCode))
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
