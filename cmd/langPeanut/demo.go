package main

import (
	"fmt"
	"path/filepath"

	"github.com/langPeanut/langPeanut/pkg/web"
	"github.com/spf13/cobra"
)

var (
	demoPort     int
	autoOpenDemo bool
)

var demoCmd = &cobra.Command{
	Use:     "web [directory]",
	Aliases: []string{"ui", "studio", "serve", "demo", "preview"},
	Short:   "Launch the interactive Web Mode Studio in your browser (http://localhost:3000)",
	Long: `Starts a local, high-performance web server on http://localhost:3000 (or custom --port)
with an interactive visual UI for scanning real project files, editing keys, previewing AST diffs,
selecting tone personas, and executing multi-agent localization workflows.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir := projectRoot
		if len(args) > 0 {
			targetDir = args[0]
		}
		absRoot, err := filepath.Abs(targetDir)
		if err != nil {
			absRoot = targetDir
		}

		fmt.Printf("Starting langPeanut Localization Engineering Studio on http://localhost:%d\n", demoPort)
		fmt.Printf("Target Project: %s\n", absRoot)
		fmt.Println("Capabilities:")
		fmt.Println("   • Live AST Scout Audit & 3-Pane String Studio")
		fmt.Println("   • Multi-Locale Translation Matrix with Inline Editing")
		fmt.Println("   • Live Component Simulator & RTL Orientation Engine")
		fmt.Println("   • Interactive Before vs After AST Code Diff Viewer")
		fmt.Println("   • 4-Tier Critic Verification Scorecard")
		fmt.Println("   • Press Ctrl+C to stop the server")
		fmt.Println()

		return web.StartInteractiveWebStudio(absRoot, demoPort, autoOpenDemo)
	},
}

func init() {
	demoCmd.Flags().IntVarP(&demoPort, "port", "p", 3000, "Port to serve the interactive web demo on")
	demoCmd.Flags().BoolVar(&autoOpenDemo, "open", true, "Automatically open the website in your default browser")
	rootCmd.AddCommand(demoCmd)
}
