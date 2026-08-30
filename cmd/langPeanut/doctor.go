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
	autoFixDoctor bool
	jsonDoctor    bool
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Autonomous Framework & i18n Readiness Diagnostic Doctor",
	Long:  "Inspects repository dependencies, manifests, i18n configurations, and untranslated literals to produce an actionable health score and 1-click bootstrap.",
	RunE: func(cmd *cobra.Command, args []string) error {
		absRoot, err := filepath.Abs(projectRoot)
		if err != nil {
			return err
		}

		registry := platforms.NewRegistry()
		platform, conf := registry.AutoDetect(absRoot)

		doctor := agents.NewDoctorAgent(platform)
		fmt.Printf("🩺 Running langPeanut Diagnostic Doctor on %s...\n", absRoot)

		report, err := doctor.DiagnoseProject(absRoot)
		if err != nil {
			return fmt.Errorf("doctor audit failed: %w", err)
		}

		if autoFixDoctor {
			actions, err := doctor.AutoBootstrap(absRoot)
			if err != nil {
				return fmt.Errorf("auto-bootstrap failed: %w", err)
			}
			fmt.Printf("\n✨ Auto-Bootstrap Actions Executed:\n")
			for _, act := range actions {
				fmt.Printf("  ✓ %s\n", act)
			}
			fmt.Println()
			// Re-run diagnostic
			report, _ = doctor.DiagnoseProject(absRoot)
		}

		if jsonDoctor {
			data, _ := json.MarshalIndent(report, "", "  ")
			fmt.Println(string(data))
			return nil
		}

		statusColor := "🟢"
		if report.HealthScore < 70 && report.HealthScore >= 45 {
			statusColor = "🟡"
		} else if report.HealthScore < 45 {
			statusColor = "🔴"
		}

		fmt.Printf("\n%s i18n Readiness Audit: %s (%d/100 Health Score)\n", statusColor, report.Status, report.HealthScore)
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("• Framework:             %s (%.0f%% match)\n", report.FrameworkDisplay, conf*100)
		fmt.Printf("• Configured Locales:    [%s]\n", strings.Join(report.ConfiguredLocales, ", "))
		fmt.Printf("• Hardcoded Strings:     ~%d literals detected\n", report.HardcodedStringEst)
		fmt.Printf("• Auto-Fixable Issues:   %d of %d\n", report.AutoFixableCount, len(report.Issues))
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

		if len(report.Issues) > 0 {
			fmt.Printf("\n📋 Diagnostic Findings:\n")
			for i, iss := range report.Issues {
				badge := "⚠️ "
				if iss.Severity == "ERROR" {
					badge = "❌"
				} else if iss.Severity == "INFO" {
					badge = "ℹ️ "
				}
				fmt.Printf("  %d. %s [%s] %s\n", i+1, badge, iss.Category, iss.Title)
				fmt.Printf("     %s\n", iss.Description)
				if iss.CanAutoFix {
					fmt.Printf("     💡 Fix: %s\n", iss.AutoFixHint)
				}
			}
		}

		if !autoFixDoctor && report.AutoFixableCount > 0 {
			fmt.Printf("\n💡 Run `langpeanut doctor --fix` to autonomously bootstrap missing configs & locale templates.\n\n")
		} else {
			fmt.Println()
		}

		return nil
	},
}

func init() {
	doctorCmd.Flags().BoolVar(&autoFixDoctor, "fix", false, "Autonomously bootstrap missing configs, directories, and i18n initialization")
	doctorCmd.Flags().BoolVar(&jsonDoctor, "json", false, "Output diagnostic report as JSON")
	rootCmd.AddCommand(doctorCmd)
}
