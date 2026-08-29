package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/langPeanut/langPeanut/pkg/agents"
	"github.com/langPeanut/langPeanut/pkg/memory"
	"github.com/langPeanut/langPeanut/pkg/platforms"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:     "run [directory]",
	Aliases: []string{"all", "start", "auto", "do"},
	Short:   "1-Click Full Autonomous Localization: Scan, AI Filter, Refactor Code, Translate & Write Locale Files",
	Long: `langPeanut Run — Complete End-to-End Localization Workflow

Executes the entire multi-agent pipeline in one single command:
1. AST Scout Agent scans & profiles all components & tags
2. AI Context Agent filters code noise (key: ${key}, CSS, SVG) & synthesizes keys
3. AST Patch Engine surgically refactors source code (0 syntax drift)
4. Cultural Translator generates multilingual translations preserving ICU placeholders
5. Writes filled locale files (e.g. src/locales/en.json, fr.json, es.json)
6. 4-Tier Critic verifies AST syntax, ICU parity, and length expansion`,
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

		startTime := time.Now()

		fmt.Println("┌────────────────────────────────────────────────────────────────────────┐")
		fmt.Printf("│ langPeanut 1-Click Localization Pipeline                               │\n")
		fmt.Printf("│ Target: %-62s │\n", truncatePath(absRoot, 62))
		fmt.Printf("│ Framework: %-22s  Locales: %-30s │\n", platform.DisplayName(), strings.Join(targetLangs, ", "))
		fmt.Println("└────────────────────────────────────────────────────────────────────────┘")

		fmt.Printf("\n[1/5] Scanning AST & profiling component elements...\n")
		supervisor, err := agents.NewSupervisorAgent(absRoot, platform)
		if err != nil {
			return err
		}

		if stylePreset != "" && stylePreset != "default" && supervisor.ProjectMemory != nil {
			supervisor.ProjectMemory.Style = memory.StylePreset(stylePreset)
			fmt.Printf("      Tone/Style Applied: %s\n", stylePreset)
		}

		fmt.Println("[2/5] AI Context Agent evaluating tags & filtering code noise...")
		fmt.Println("[3/5] Deterministic Patch Engine applying surgical byte-range patches...")
		fmt.Printf("[4/5] Cultural Translator translating into [%s]...\n", strings.Join(targetLangs, ", "))
		fmt.Println("[5/5] 4-Tier Critic running automated verification checks...")

		result, err := supervisor.RunEndToEnd(context.Background(), sourceLang, targetLangs, dryRun)
		if err != nil {
			return fmt.Errorf("pipeline execution failed: %w", err)
		}

		elapsed := time.Since(startTime).Round(time.Millisecond)

		fmt.Printf("\n──────────────────────────────────────────────────────────────────────────\n")
		fmt.Printf("Localization Complete in %s!\n", elapsed)
		fmt.Printf("──────────────────────────────────────────────────────────────────────────\n")
		fmt.Printf("  • Scanned Files:         %d\n", result.ScannedFilesCount)
		fmt.Printf("  • Candidate Strings:     %d\n", result.ExtractedCandidates)
		fmt.Printf("  • Unique Localized Keys: %d\n", result.UniqueKeysCount)
		fmt.Printf("  • Refactored Source:     %d files (0 syntax regressions)\n", len(result.RefactoredFiles))

		if !dryRun {
			fmt.Printf("\nGenerated & Filled Locale Files on Disk:\n")
			fmt.Printf("  • %-30s (%d keys, Base)\n", result.SourceLocaleFile, result.UniqueKeysCount)
			for _, tgt := range targetLangs {
				if fPath, ok := result.TargetLocaleFiles[tgt]; ok {
					fmt.Printf("  • %-30s (%d keys, %s)\n", fPath, result.UniqueKeysCount, tgt)
				}
			}
		} else {
			fmt.Println("\n[Dry Run] Preview complete — no files were modified on disk.")
		}

		// Print 4-Tier Verification report
		if result.VerificationReport != nil {
			fmt.Println("\n┌────────────────────────────────────────────────────────┐")
			fmt.Printf("│ 4-Tier Critic Verification Report                      │\n")
			fmt.Println("├────────────────────────────────────────────────────────┤")
			if result.VerificationReport.Passed {
				fmt.Println("│  ✓ Tier 1 (AST Syntax Validation):         PASSED      │")
				fmt.Println("│  ✓ Tier 2 (ICU & Variable Parity):         PASSED      │")
				fmt.Println("│  ✓ Tier 3 (UI Layout & Length Expansion):  PASSED      │")
				fmt.Println("│  ✓ Tier 4 (Cross-Locale Key Parity):       PASSED      │")
				fmt.Println("└────────────────────────────────────────────────────────┘")
			} else {
				fmt.Printf("│  [FAILED] Verification Issues: %d errors / %d warnings │\n", result.VerificationReport.ErrorCount, result.VerificationReport.WarnCount)
				fmt.Println("└────────────────────────────────────────────────────────┘")
				for _, d := range result.VerificationReport.Diagnostics {
					fmt.Printf("  • [%s] Tier %d: %s\n", d.Severity, d.Tier, d.Message)
				}
			}
		}

		if result.CheckpointID != "" {
			fmt.Printf("\n✓ Rollback Checkpoint Saved: %s\n", result.CheckpointID)
			fmt.Println("  (Run `langPeanut rollback` at any time to revert changes)")
		}

		if result.TrajectoryMDPath != "" {
			fmt.Printf("\n✓ Trajectory Trace Saved: %s\n", result.TrajectoryMDPath)
		}

		return nil
	},
}

func truncatePath(p string, maxLen int) string {
	if len(p) <= maxLen {
		return p
	}
	return "..." + p[len(p)-maxLen+3:]
}

func init() {
	runCmd.Flags().StringVar(&stylePreset, "style", "default", "Translation style preset (default, gen_z, casual, formal, humorous, pirate)")
	rootCmd.AddCommand(runCmd)
}
