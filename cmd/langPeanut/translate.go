package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/langPeanut/langPeanut/pkg/agents"
	"github.com/langPeanut/langPeanut/pkg/memory"
	"github.com/langPeanut/langPeanut/pkg/orchestrator"
	"github.com/langPeanut/langPeanut/pkg/platforms"
	"github.com/spf13/cobra"
)

var (
	stylePreset  string
	providerFlag string
	modelFlag    string
)

var translateCmd = &cobra.Command{
	Use:   "translate",
	Short: "Translate source strings into target locales with 4-Tier Critic verification",
	RunE: func(cmd *cobra.Command, args []string) error {
		absRoot, err := filepath.Abs(projectRoot)
		if err != nil {
			return err
		}

		registry := platforms.NewRegistry()
		platform, _ := registry.AutoDetect(absRoot)

		fmt.Printf("🌐 langPeanut Translate — Translating %s (%s) into [%s]...\n", absRoot, platform.DisplayName(), strings.Join(targetLangs, ", "))
		if stylePreset != "" && stylePreset != "default" {
			fmt.Printf("🎭 Style Tone Applied: %s\n", stylePreset)
		}
		if providerFlag != "" {
			fmt.Printf("🤖 Provider Override: %s (Model: %s)\n", providerFlag, modelFlag)
		}
		fmt.Println()

		supervisor, err := agents.NewSupervisorAgent(absRoot, platform)
		if err != nil {
			return err
		}

		if stylePreset != "" && supervisor.ProjectMemory != nil {
			supervisor.ProjectMemory.Style = memory.StylePreset(stylePreset)
		}

		result, err := supervisor.RunEndToEnd(context.Background(), sourceLang, targetLangs, dryRun)
		if err != nil {
			return err
		}

		fmt.Printf("\n✓ Extracted %d keys from source locale '%s'\n", result.ExtractedCandidates, sourceLang)
		fmt.Printf("✓ Generated translations for: %s\n", strings.Join(result.GeneratedLocales, ", "))

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
				fmt.Printf("│  ❌ Verification Failed with %d errors / %d warnings   │\n", result.VerificationReport.ErrorCount, result.VerificationReport.WarnCount)
				fmt.Println("└────────────────────────────────────────────────────────┘")
				for _, d := range result.VerificationReport.Diagnostics {
					fmt.Printf("  • [%s] Tier %d: %s\n", d.Severity, d.Tier, d.Message)
				}
			}
		}

		if result.TrajectoryMDPath != "" {
			fmt.Printf("\n✓ Agent Trajectory trace exported to: %s\n", result.TrajectoryMDPath)
		}

		return nil
	},
}

var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Restore codebase and locale files to a previous checkpoint",
	RunE: func(cmd *cobra.Command, args []string) error {
		absRoot, err := filepath.Abs(projectRoot)
		if err != nil {
			return err
		}

		ckptMgr, err := orchestrator.NewCheckpointManager(absRoot)
		if err != nil {
			return err
		}

		checkpoints, err := ckptMgr.ListCheckpoints()
		if err != nil || len(checkpoints) == 0 {
			fmt.Println("No checkpoints found to restore.")
			return nil
		}

		if len(args) == 0 {
			fmt.Println("Available Checkpoints:")
			fmt.Println("──────────────────────────────────────────────────────────────────────────")
			for i, c := range checkpoints {
				fmt.Printf(" [%d] %-32s (%s) — %s\n", i+1, c.ID, c.CreatedAt.Format("15:04:05"), c.Summary)
			}
			fmt.Println("\nTo restore, run: langPeanut rollback <checkpoint_id>")
			return nil
		}

		targetID := args[0]
		err = ckptMgr.RestoreCheckpoint(targetID)
		if err != nil {
			return err
		}

		fmt.Printf("✓ Successfully restored codebase to checkpoint: %s\n", targetID)
		return nil
	},
}

func init() {
	translateCmd.Flags().StringVar(&stylePreset, "style", "default", "Translation style preset (default, gen_z, casual, formal, humorous, pirate)")
	translateCmd.Flags().StringVar(&providerFlag, "provider", "", "LLM provider override (claude, openai, gemini, deepl, custom, local)")
	translateCmd.Flags().StringVar(&modelFlag, "model", "", "Custom model tag override (e.g. claude-3-5-haiku, gpt-4.5-preview, gemini-2.5-pro)")
	rootCmd.AddCommand(translateCmd)
	rootCmd.AddCommand(rollbackCmd)
}
