package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joho/godotenv"
	"github.com/langPeanut/langPeanut/pkg/tui"
	"github.com/spf13/cobra"
)

var (
	projectRoot string
	sourceLang  string
	targetLangs []string
	dryRun      bool
	ciMode      bool
)

var rootCmd = &cobra.Command{
	Use:   "langPeanut",
	Short: "langPeanut — Universal Multi-Agent Localization Workflow & Interactive App",
	Long: `langPeanut is a high-performance, universal multi-agent localization system.
Running 'langPeanut' without arguments launches the interactive terminal application.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		p := tea.NewProgram(tui.NewApp(projectRoot), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("error running TUI app: %w", err)
		}
		return nil
	},
}

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch the interactive Bubble Tea terminal application",
	RunE: func(cmd *cobra.Command, args []string) error {
		p := tea.NewProgram(tui.NewApp(projectRoot), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("error running TUI app: %w", err)
		}
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&projectRoot, "dir", "d", ".", "Project root directory")
	rootCmd.PersistentFlags().StringVarP(&sourceLang, "source", "s", "en", "Source language locale")
	rootCmd.PersistentFlags().StringSliceVarP(&targetLangs, "locales", "l", []string{"fr", "es", "de", "ja"}, "Target languages to translate")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Preview actions without modifying files")
	rootCmd.PersistentFlags().BoolVar(&ciMode, "ci", false, "CI mode (exits 1 if hardcoded strings are found)")

	rootCmd.AddCommand(tuiCmd)
}

func main() {
	// Best-effort: load API keys and config from a local .env file if present.
	// Never overrides variables already set in the environment.
	_ = godotenv.Load()

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
