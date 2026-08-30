package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/langPeanut/langPeanut/pkg/agents"
	"github.com/langPeanut/langPeanut/pkg/llm"
	"github.com/langPeanut/langPeanut/pkg/memory"
	"github.com/langPeanut/langPeanut/pkg/orchestrator"
	"github.com/langPeanut/langPeanut/pkg/platforms"
	"github.com/spf13/cobra"
)

var (
	stylePreset     string
	providerFlag    string
	modelFlag       string
	chunkWordsFlag  int
	chunkKeysFlag   int
	concurrencyFlag int
)

var translateCmd = &cobra.Command{
	Use:     "translate [directory]",
	Aliases: []string{"i18n", "trans"},
	Short:   "Translate source strings into target locales with 4-Tier Critic verification",
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

		fmt.Printf("langPeanut Translate — Translating %s (%s) into [%s]...\n", absRoot, platform.DisplayName(), strings.Join(targetLangs, ", "))
		if stylePreset != "" && stylePreset != "default" {
			fmt.Printf("Style Tone Applied: %s\n", stylePreset)
		}
		supervisor, err := agents.NewSupervisorAgent(absRoot, platform)
		if err != nil {
			return err
		}

		if providerFlag != "" {
			supervisor.Translator.LLM = llm.NewClient(llm.ProviderType(providerFlag), modelFlag)
			fmt.Printf("Provider Override: %s (Model: %s)\n", providerFlag, modelFlag)

			if providerFlag == "nllb-local" || providerFlag == "nllb" {
				if downloaded, p, _ := llm.IsNLLBModelDownloaded(); downloaded {
					fmt.Printf("✓ Using Local Meta NLLB-200 offline model: %s\n", p)
				} else {
					fmt.Printf("⬇ Downloading Local Meta NLLB-200 offline model (~380MB GGUF)...\n")
					_, err := llm.EnsureNLLBModel(context.Background(), func(down, tot int64, pct float64) {
						fmt.Printf("\r[Download] %.1f%% (%.1f MB / %.1f MB)", pct, float64(down)/(1024*1024), float64(tot)/(1024*1024))
					})
					fmt.Println()
					if err != nil {
						fmt.Printf("⚠ Download failed: %v. Falling back to offline synthesizer.\n", err)
					} else {
						fmt.Printf("✓ Model downloaded successfully.\n")
					}
				}
			}
		} else if modelFlag != "" {
			supervisor.Translator.LLM = llm.NewClient(llm.ProviderOpenAI, modelFlag)
			fmt.Printf("Model Override: %s\n", modelFlag)
		}

		if concurrencyFlag > 0 {
			supervisor.Translator.Concurrency = concurrencyFlag
			fmt.Printf("Concurrency Override: %d parallel workers\n", concurrencyFlag)
		}
		if chunkWordsFlag > 0 {
			supervisor.Translator.ChunkWordBudget = chunkWordsFlag
			fmt.Printf("Chunk Word Budget Override: %d words per batch call\n", chunkWordsFlag)
		}
		if chunkKeysFlag > 0 {
			supervisor.Translator.ChunkKeyCeiling = chunkKeysFlag
			fmt.Printf("Chunk Key Ceiling Override: %d keys per batch call\n", chunkKeysFlag)
		}
		fmt.Println()

		if stylePreset != "" && stylePreset != "default" && supervisor.ProjectMemory != nil {
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
				fmt.Printf("│  [FAILED] Verification Issues: %d errors / %d warnings │\n", result.VerificationReport.ErrorCount, result.VerificationReport.WarnCount)
				fmt.Println("└────────────────────────────────────────────────────────┘")
				for _, d := range result.VerificationReport.Diagnostics {
					fmt.Printf("  • [%s] Tier %d: %s\n", d.Severity, d.Tier, d.Message)
				}
			}
		}

		// Print Code Repair report if any repairs were triggered
		if len(result.CodeRepairs) > 0 {
			fmt.Println("\n┌────────────────────────────────────────────────────────┐")
			fmt.Printf("│ Autonomous Code Self-Healing & Repair Report           │\n")
			fmt.Println("├────────────────────────────────────────────────────────┤")
			for _, r := range result.CodeRepairs {
				if r.Repaired {
					fmt.Printf("│  ✓ %-35s AUTO-HEALED │\n", filepath.Base(r.FilePath))
					if r.Explanation != "" {
						fmt.Printf("│    ↳ %s\n", r.Explanation)
					}
				} else {
					fmt.Printf("│  [REVIEW] %-28s MANUAL FIX NEEDED │\n", filepath.Base(r.FilePath))
				}
			}
			fmt.Println("└────────────────────────────────────────────────────────┘")
		}

		if len(result.UnresolvedErrors) > 0 {
			fmt.Printf("\nWarning: %d compiler diagnostic(s) require manual review:\n", len(result.UnresolvedErrors))
			for _, ue := range result.UnresolvedErrors {
				fmt.Printf("   • %s:%d:%d — [%s] %s\n", filepath.Base(ue.FilePath), ue.Line, ue.Column, ue.Source, ue.Message)
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
	translateCmd.Flags().IntVarP(&concurrencyFlag, "concurrency", "c", 0, "Max concurrent parallel LLM calls (default: 5 or model config)")
	translateCmd.Flags().IntVar(&chunkWordsFlag, "chunk-words", 0, "Max words (approx tokens) per LLM batch call (0 = auto model-aware)")
	translateCmd.Flags().IntVar(&chunkKeysFlag, "chunk-keys", 0, "Max keys per LLM batch call (0 = auto model-aware)")
	rootCmd.AddCommand(translateCmd)
	rootCmd.AddCommand(rollbackCmd)
}
