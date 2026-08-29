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
	Use:   "langPeanut [directory]",
	Short: "langPeanut — Universal Multi-Agent Localization Workflow & Interactive App",
	Long: `🥜 langPeanut — Universal Multi-Agent Localization System for Developers

Effortlessly find hardcoded UI strings, apply surgical AST patches with 0 syntax drift, 
and translate your app into 36+ languages with automated 4-Tier Critic verification.

⚡ QUICK START EXAMPLES:
  langPeanut                          Launch the full interactive TUI app
  langPeanut ./examples/nextjs-app    Launch TUI targeting a specific project
  langPeanut scan ./examples/nextjs-app Scan and audit hardcoded strings
  langPeanut translate -l fr,es,ja    Translate into French, Spanish & Japanese
  langPeanut demo                     Launch live interactive browser demo
  langPeanut reset                    Reset demo apps to fresh unlocalized state
  langPeanut benchmark                Run 10-Case Adversarial Benchmark Suite`,
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir := projectRoot
		if len(args) > 0 {
			targetDir = args[0]
		}
		p := tea.NewProgram(tui.NewApp(targetDir), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("error running TUI app: %w", err)
		}
		return nil
	},
}

var tuiCmd = &cobra.Command{
	Use:   "tui [directory]",
	Short: "Launch the interactive Bubble Tea terminal application",
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir := projectRoot
		if len(args) > 0 {
			targetDir = args[0]
		}
		p := tea.NewProgram(tui.NewApp(targetDir), tea.WithAltScreen())
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
