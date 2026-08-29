package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/langPeanut/langPeanut/pkg/agents"
	"github.com/langPeanut/langPeanut/pkg/platforms"
	"github.com/langPeanut/langPeanut/pkg/types"
	"github.com/spf13/cobra"
)

var auditFile string

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Scan codebase and report hardcoded strings and localization coverage",
	RunE: func(cmd *cobra.Command, args []string) error {
		absRoot, err := filepath.Abs(projectRoot)
		if err != nil {
			return err
		}

		registry := platforms.NewRegistry()
		platform, _ := registry.AutoDetect(absRoot)

		scout := agents.NewASTScoutAgent(platform)
		contextAgent := agents.NewContextAgent()

		fmt.Printf("🔍 langPeanut Audit — Scanning %s (%s)...\n\n", absRoot, platform.DisplayName())

		report, err := scout.ScanProject(absRoot, auditFile)
		if err != nil {
			return err
		}

		candidates, err := contextAgent.DisambiguateAndEnhance(report.Candidates)
		if err != nil {
			return err
		}

		fmt.Println("┌────────────────────────────────────────────────────────┐")
		fmt.Printf("│ Audit Summary                                          │\n")
		fmt.Println("├────────────────────────────────────────────────────────┤")
		fmt.Printf("│  Total Files Scanned:       %-26d │\n", report.TotalFilesScanned)
		fmt.Printf("│  Total Candidates Found:    %-26d │\n", report.TotalCandidates)
		fmt.Printf("│  Localizable UI Strings:    %-26d │\n", report.LocalizableCount)
		fmt.Printf("│  Auto-Skipped (Non-UI):     %-26d │\n", report.SkipCount)
		fmt.Printf("│  Uncertain Strings:         %-26d │\n", report.UncertainCount)
		fmt.Println("└────────────────────────────────────────────────────────┘")
		fmt.Println()

		if len(candidates) > 0 {
			fmt.Println("Candidate Hardcoded Strings Found:")
			fmt.Println("──────────────────────────────────────────────────────────────────────────")
			for i, c := range candidates {
				if c.Classification == types.ClassLocalizable {
					relPath, _ := filepath.Rel(absRoot, c.FilePath)
					fmt.Printf(" [%2d] %-30s (line %d:%d)\n", i+1, relPath, c.StartLine, c.StartCol)
					fmt.Printf("      Found:   \"%s\"\n", c.CleanValue)
					fmt.Printf("      Key:     %s\n", c.Key)
					if c.Explanation != "" {
						fmt.Printf("      Note:    %s\n", c.Explanation)
					}
					fmt.Println()
				}
			}
		}

		if ciMode && report.LocalizableCount > 0 {
			fmt.Fprintf(os.Stderr, "❌ CI Check Failed: %d hardcoded string(s) detected.\n", report.LocalizableCount)
			os.Exit(1)
		}

		return nil
	},
}

func init() {
	auditCmd.Flags().StringVarP(&auditFile, "file", "f", "", "Audit a specific file only")
	rootCmd.AddCommand(auditCmd)
}
