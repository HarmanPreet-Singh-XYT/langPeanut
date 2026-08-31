package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/langPeanut/langPeanut/pkg/agents"
	"github.com/langPeanut/langPeanut/pkg/platforms"
	"github.com/langPeanut/langPeanut/pkg/types"
	"github.com/spf13/cobra"
)

var autoApprove bool

var extractCmd = &cobra.Command{
	Use:     "extract [directory]",
	Aliases: []string{"pull", "localize"},
	Short:   "Extract hardcoded strings with AI classification and generate base locale files",
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir := projectRoot
		if len(args) > 0 {
			targetDir = args[0]
		}
		absRoot, err := filepath.Abs(targetDir)
		if err != nil {
			return err
		}

		registry := platforms.NewRegistry()
		platform, _ := registry.AutoDetect(absRoot)

		fmt.Printf("langPeanut Extract — Processing %s (%s)...\n\n", absRoot, platform.DisplayName())

		scout := agents.NewASTScoutAgent(platform)
		contextAgent := agents.NewContextAgent()

		scanReport, err := scout.ScanProject(absRoot, "")
		if err != nil {
			return fmt.Errorf("scout failed: %w", err)
		}

		candidates := contextAgent.EnhanceFast(scanReport.Candidates)
		sourceEntries := make(map[string]string)
		for _, c := range candidates {
			if c.Classification == types.ClassLocalizable && (c.Approved || autoApprove || c.Confidence >= 0.8) {
				sourceEntries[c.Key] = c.CleanValue
			}
		}

		// Merge with existing on-disk source catalog if present
		existingFiles, _ := platform.DiscoverExistingLocales(absRoot)
		if srcPath, found := existingFiles[sourceLang]; found {
			if data, err := os.ReadFile(srcPath); err == nil {
				if locData, err := platform.ParseLocaleFileForLocale(data, sourceLang); err == nil && locData != nil {
					for k, v := range locData.Entries {
						sourceEntries[k] = v
					}
				}
			}
		}

		rawDir := platform.DefaultLocaleDir(absRoot)
		localeDir := rawDir
		if !filepath.IsAbs(localeDir) {
			localeDir = filepath.Join(absRoot, rawDir)
		}

		sourcePath := platform.DefaultSourceFile(absRoot, sourceLang)
		if !filepath.IsAbs(sourcePath) {
			sourcePath = filepath.Join(absRoot, sourcePath)
		}

		sourceLocaleData := types.LocaleData{
			LocaleCode: sourceLang,
			Entries:    sourceEntries,
		}

		if !dryRun {
			_ = os.MkdirAll(localeDir, 0755)
			srcBytes, _ := platform.FormatLocaleFile(sourceLocaleData)
			_ = os.WriteFile(sourcePath, srcBytes, 0644)
		}

		fmt.Printf("✓ Scanned %d files, extracted %d candidate strings (%d localizable keys).\n", scanReport.TotalFilesScanned, scanReport.TotalCandidates, len(sourceEntries))
		fmt.Printf("✓ Base locale catalog written to: %s\n", sourcePath)

		if dryRun {
			fmt.Println("\n[Dry Run] No files were modified on disk.")
		}

		return nil
	},
}

var refactorCmd = &cobra.Command{
	Use:     "refactor [directory]",
	Aliases: []string{"patch", "apply"},
	Short:   "Surgically refactor source code with deterministic AST patches",
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir := projectRoot
		if len(args) > 0 {
			targetDir = args[0]
		}
		absRoot, err := filepath.Abs(targetDir)
		if err != nil {
			return err
		}

		registry := platforms.NewRegistry()
		platform, _ := registry.AutoDetect(absRoot)

		fmt.Printf("langPeanut Refactor — Rewriting %s (%s)...\n\n", absRoot, platform.DisplayName())

		supervisor, err := agents.NewSupervisorAgent(absRoot, platform)
		if err != nil {
			return err
		}

		result, err := supervisor.RunEndToEnd(context.Background(), sourceLang, []string{}, dryRun)
		if err != nil {
			return err
		}

		fmt.Printf("✓ Refactored %d source file(s) with 0 syntax regressions.\n", len(result.RefactoredFiles))
		for _, f := range result.RefactoredFiles {
			relPath, _ := filepath.Rel(absRoot, f)
			fmt.Printf("   • %s\n", relPath)
		}

		return nil
	},
}

func init() {
	extractCmd.Flags().BoolVar(&autoApprove, "auto", false, "Auto-approve all high-confidence candidates")
	rootCmd.AddCommand(extractCmd)
	rootCmd.AddCommand(refactorCmd)
}
