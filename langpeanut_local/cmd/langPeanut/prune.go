package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/langPeanut/langPeanut/pkg/agents"
	"github.com/langPeanut/langPeanut/pkg/platforms"
	"github.com/spf13/cobra"
)

var (
	dryRunPrune bool
	jsonPrune   bool
)

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Autonomous Stale String & Dead Key Garbage Collector",
	Long:  "Scans project AST to find translation keys in locale dictionaries that are no longer referenced in source code, and prunes them.",
	RunE: func(cmd *cobra.Command, args []string) error {
		absRoot, err := filepath.Abs(projectRoot)
		if err != nil {
			return err
		}

		registry := platforms.NewRegistry()
		platform, _ := registry.AutoDetect(absRoot)

		pruner := agents.NewPrunerAgent(platform)
		fmt.Printf("🔍 Scanning %d file types in %s for dead translation keys...\n", len(platform.FileExtensions()), absRoot)

		var report *agents.PruneReport
		if dryRunPrune {
			report, err = pruner.AnalyzeDeadKeys(absRoot)
		} else {
			report, err = pruner.PruneDeadKeys(absRoot)
		}

		if err != nil {
			return fmt.Errorf("prune analysis failed: %w", err)
		}

		if jsonPrune {
			data, _ := json.MarshalIndent(report, "", "  ")
			fmt.Println(string(data))
			return nil
		}

		fmt.Printf("\n🧹 Dead Key Garbage Collection Report:\n")
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("• Source Files Scanned: %d\n", report.TotalFilesScanned)
		fmt.Printf("• Active Keys in Code:  %d\n", report.ActiveKeysCount)
		fmt.Printf("• Stale / Dead Keys:    %d\n", report.TotalDeadKeys)

		if report.TotalDeadKeys == 0 {
			fmt.Printf("✓ 100%% Clean! All locale keys are actively referenced in source code.\n")
		} else {
			for loc, keys := range report.DeadKeysByLocale {
				fmt.Printf("  └─ [%s]: %d dead keys (e.g. %s)\n", loc, len(keys), strings.Join(keys[:min(len(keys), 3)], ", "))
			}
			if dryRunPrune {
				fmt.Printf("\n💡 Run `langpeanut prune` without `--dry-run` to delete stale keys from locale files.\n")
			} else {
				fmt.Printf("\n✓ Successfully pruned %d stale keys across %s\n", report.TotalDeadKeys, strings.Join(report.PrunedLocales, ", "))
			}
		}
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
		return nil
	},
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func init() {
	pruneCmd.Flags().BoolVar(&dryRunPrune, "dry-run", false, "Analyze and report dead keys without deleting them")
	pruneCmd.Flags().BoolVar(&jsonPrune, "json", false, "Output dead key report as JSON")
	rootCmd.AddCommand(pruneCmd)
}
