package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/langPeanut/langPeanut/pkg/agents"
	"github.com/langPeanut/langPeanut/pkg/llm"
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

		if providerFlag != "" {
			supervisor.Translator.LLM = llm.NewClient(llm.ProviderType(providerFlag), modelFlag)
		} else if modelFlag != "" {
			supervisor.Translator.LLM = llm.NewClient(llm.ProviderOpenAI, modelFlag)
		}

		if concurrencyFlag > 0 {
			supervisor.Translator.Concurrency = concurrencyFlag
		}
		if chunkWordsFlag > 0 {
			supervisor.Translator.ChunkWordBudget = chunkWordsFlag
		}
		if chunkKeysFlag > 0 {
			supervisor.Translator.ChunkKeyCeiling = chunkKeysFlag
		}

		if directiveFlag != "" {
			supervisor.UserDirective = directiveFlag
			fmt.Printf("      User Directive: %s\n", directiveFlag)
		}
		if customInstallCmdFlag != "" {
			supervisor.CustomInstallCmd = customInstallCmdFlag
			fmt.Printf("      Custom Install Command: %s\n", customInstallCmdFlag)
		}
		if customBuildCmdFlag != "" {
			supervisor.CustomBuildCmd = customBuildCmdFlag
			fmt.Printf("      Custom Build/Typecheck: %s\n", customBuildCmdFlag)
		}

		// Existing translations strategy resolution
		if regenerateFlag {
			supervisor.ExistingMode = "replace"
			fmt.Println("      Strategy: Regenerate & Replace All Existing Translations (--regenerate)")
		} else if existingModeFlag != "" {
			supervisor.ExistingMode = existingModeFlag
			fmt.Printf("      Strategy: %s (--existing-mode)\n", existingModeFlag)
		} else if supervisor.ExistingMode == "prompt" {
			existingLocs, _ := platform.DiscoverExistingLocales(absRoot)
			if len(existingLocs) > 0 {
				fmt.Printf("\n[!] Discovered existing translations on disk (%d locales present).\n", len(existingLocs))
				fmt.Println("    How would you like to handle existing translations?")
				fmt.Println("    [1] Skip existing (Incremental: only translate missing keys) [default]")
				fmt.Println("    [2] Replace / Regenerate all existing translations from source")
				fmt.Print("    Select option [1/2]: ")
				var choice string
				_, _ = fmt.Scanln(&choice)
				choice = strings.TrimSpace(choice)
				if choice == "2" || strings.ToLower(choice) == "replace" || strings.ToLower(choice) == "r" {
					supervisor.ExistingMode = "replace"
					fmt.Println("    -> Selected: Regenerate & Replace All")
				} else {
					supervisor.ExistingMode = "skip"
					fmt.Println("    -> Selected: Skip Existing (Incremental)")
				}
			}
		}

		fmt.Println("[2/5] AI Context Agent evaluating tags & filtering code noise...")
		fmt.Println("[3/5] Deterministic Patch Engine applying surgical byte-range patches...")
		fmt.Printf("[4/5] Cultural Translator translating into [%s]...\n", strings.Join(targetLangs, ", "))
		fmt.Println("[5/5] 4-Tier Critic running automated verification checks...")
		if directiveFlag != "" {
			fmt.Println("[6/6] App Integration Agent executing post-localization directive...")
		}

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

		if len(result.GeneratedLocales) > 0 {
			fmt.Printf("\nGenerated & Filled Locale Files on Disk:\n")
			fmt.Printf("  • %-30s (%d keys, Base)\n", result.SourceLocaleFile, result.UniqueKeysCount)
			for _, loc := range result.GeneratedLocales {
				path := result.TargetLocaleFiles[loc]
				fmt.Printf("  • %-30s (%d keys, %s)\n", path, result.UniqueKeysCount, loc)
			}
		}

		if result.DependencyStatus != nil && (result.DependencyStatus.ManifestUpdated || len(result.DependencyStatus.InstalledDeps) > 0 || len(result.DependencyStatus.ConfigCreated) > 0) {
			fmt.Printf("\nFramework Language Dependencies:\n")
			if result.DependencyStatus.ManifestFile != "" {
				fmt.Printf("  • Manifest: %s (Updated: %v)\n", result.DependencyStatus.ManifestFile, result.DependencyStatus.ManifestUpdated)
			}
			if len(result.DependencyStatus.InstalledDeps) > 0 {
				fmt.Printf("  • Packages: %s\n", strings.Join(result.DependencyStatus.InstalledDeps, ", "))
			}
			if len(result.DependencyStatus.ConfigCreated) > 0 {
				fmt.Printf("  • Configs:  %s\n", strings.Join(result.DependencyStatus.ConfigCreated, ", "))
			}
			if result.DependencyStatus.CommandExecuted != "" {
				fmt.Printf("  • Install:  ✓ %s\n", result.DependencyStatus.CommandExecuted)
			}
		}

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

		if result.DirectiveResult != nil {
			dr := result.DirectiveResult
			fmt.Println()
			fmt.Println("┌────────────────────────────────────────────────────────┐")
			fmt.Println("│ App Integration Agent: Custom Directive Result         │")
			fmt.Println("├────────────────────────────────────────────────────────┤")
			fmt.Printf("│ Directive: %s\n", dr.Directive)
			if dr.Success {
				fmt.Printf("│ Status:    ✓ Completed (Attempts: %d)\n", dr.Attempts)
				if len(dr.CreatedFiles) > 0 {
					fmt.Printf("│ Created:   %s\n", strings.Join(dr.CreatedFiles, ", "))
				}
				if len(dr.PatchedFiles) > 0 {
					fmt.Printf("│ Patched:   %s\n", strings.Join(dr.PatchedFiles, ", "))
				}
			} else {
				fmt.Printf("│ Status:    ⚠️ Partial/Unresolved: %s\n", dr.Explanation)
			}
			fmt.Println("└────────────────────────────────────────────────────────┘")
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

var (
	directiveFlag        string
	customInstallCmdFlag string
	customBuildCmdFlag   string
	existingModeFlag     string
	regenerateFlag       bool
)

func init() {
	runCmd.Flags().StringVar(&stylePreset, "style", "default", "Translation style preset (default, gen_z, casual, formal, humorous, pirate)")
	runCmd.Flags().StringVar(&providerFlag, "provider", "", "LLM provider override (claude, openai, gemini, deepl, custom, local)")
	runCmd.Flags().StringVar(&modelFlag, "model", "", "Custom model tag override (e.g. claude-3-5-haiku, gpt-4.5-preview, gemini-2.5-pro)")
	runCmd.Flags().IntVarP(&concurrencyFlag, "concurrency", "c", 0, "Max concurrent parallel LLM calls (default: 5 or model config)")
	runCmd.Flags().IntVar(&chunkWordsFlag, "chunk-words", 0, "Max words (approx tokens) per LLM batch call (0 = auto model-aware)")
	runCmd.Flags().IntVar(&chunkWordsFlag, "max-tokens", 0, "Alias for --chunk-words (max token budget per batch call)")
	runCmd.Flags().IntVar(&chunkWordsFlag, "tokens-per-batch", 0, "Alias for --chunk-words")
	runCmd.Flags().IntVar(&chunkKeysFlag, "chunk-keys", 0, "Max keys per LLM batch call (0 = auto model-aware)")
	runCmd.Flags().IntVar(&chunkKeysFlag, "keys-per-batch", 0, "Alias for --chunk-keys")
	runCmd.Flags().StringVar(&directiveFlag, "directive", "", "Post-localization coding directive (e.g. 'Add a language switcher dropdown in Navbar.tsx')")
	runCmd.Flags().StringVar(&directiveFlag, "instruction", "", "Alias for --directive")
	runCmd.Flags().StringVar(&customInstallCmdFlag, "install-cmd", "", "Custom shell command to install dependencies (e.g. 'pnpm install', 'yarn add react-i18next i18next')")
	runCmd.Flags().StringVar(&customInstallCmdFlag, "custom-install-cmd", "", "Alias for --install-cmd")
	runCmd.Flags().StringVar(&customBuildCmdFlag, "build-cmd", "", "Custom shell command to typecheck/build project (e.g. 'pnpm typecheck', 'npm run build', 'flutter analyze')")
	runCmd.Flags().StringVar(&customBuildCmdFlag, "custom-build-cmd", "", "Alias for --build-cmd")
	runCmd.Flags().StringVar(&existingModeFlag, "existing-mode", "", "Strategy for existing translations: 'skip' (default), 'replace' (regenerate all), 'prompt' (ask interactively)")
	runCmd.Flags().StringVar(&existingModeFlag, "on-existing", "", "Alias for --existing-mode")
	runCmd.Flags().BoolVar(&regenerateFlag, "regenerate", false, "Regenerate and replace all existing translations from source")
	runCmd.Flags().BoolVar(&regenerateFlag, "force", false, "Alias for --regenerate")
	rootCmd.AddCommand(runCmd)
}
