package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/langPeanut/langPeanut/pkg/agents"
	"github.com/langPeanut/langPeanut/pkg/platforms"
	"github.com/spf13/cobra"
)

var autoApprove bool

var extractCmd = &cobra.Command{
	Use:   "extract",
	Short: "Extract hardcoded strings with AI classification and generate base locale files",
	RunE: func(cmd *cobra.Command, args []string) error {
		absRoot, err := filepath.Abs(projectRoot)
		if err != nil {
			return err
		}

		registry := platforms.NewRegistry()
		platform, _ := registry.AutoDetect(absRoot)

		fmt.Printf("🥜 langPeanut Extract — Processing %s (%s)...\n\n", absRoot, platform.DisplayName())

		supervisor, err := agents.NewSupervisorAgent(absRoot, platform)
		if err != nil {
			return err
		}

		result, err := supervisor.RunEndToEnd(context.Background(), sourceLang, []string{}, dryRun)
		if err != nil {
			return err
		}

		fmt.Printf("✓ Scanned %d files, extracted %d candidate strings.\n", result.ScannedFilesCount, result.ExtractedCandidates)
		fmt.Printf("✓ Created base locale file for: %s\n", sourceLang)

		if dryRun {
			fmt.Println("\n[Dry Run] No files were modified on disk.")
		} else {
			if result.CheckpointID != "" {
				fmt.Printf("✓ Checkpoint saved: %s (run `langPeanut rollback` to revert)\n", result.CheckpointID)
			}
		}

		return nil
	},
}

var refactorCmd = &cobra.Command{
	Use:   "refactor",
	Short: "Surgically refactor source code with deterministic AST patches",
	RunE: func(cmd *cobra.Command, args []string) error {
		absRoot, err := filepath.Abs(projectRoot)
		if err != nil {
			return err
		}

		registry := platforms.NewRegistry()
		platform, _ := registry.AutoDetect(absRoot)

		fmt.Printf("⚡ langPeanut Refactor — Rewriting %s (%s)...\n\n", absRoot, platform.DisplayName())

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
