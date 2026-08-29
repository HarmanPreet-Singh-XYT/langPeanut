package main

import (
	"fmt"

	"github.com/langPeanut/langPeanut/pkg/web"
	"github.com/spf13/cobra"
)

var (
	demoPort     int
	autoOpenDemo bool
)

var demoCmd = &cobra.Command{
	Use:     "demo",
	Aliases: []string{"preview", "serve", "web"},
	Short:   "Launch the live interactive web demo application in your browser (http://localhost:3000)",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("🌐 Starting langPeanut Live Interactive Web Demo on http://localhost:%d\n", demoPort)
		fmt.Println("✨ Features:")
		fmt.Println("   • Live Real-Time Multi-Language Switcher (French, Spanish, German, Japanese, Hindi, Punjabi, etc.)")
		fmt.Println("   • Interactive Before vs After Mode Toggle Switch")
		fmt.Println("   • Dynamic Gen-Z Slang Translation Mode")
		fmt.Println("   • Press Ctrl+C to stop the server")
		fmt.Println()

		return web.StartInteractiveWebDemo(demoPort, autoOpenDemo)
	},
}

func init() {
	demoCmd.Flags().IntVarP(&demoPort, "port", "p", 3000, "Port to serve the interactive web demo on")
	demoCmd.Flags().BoolVar(&autoOpenDemo, "open", true, "Automatically open the website in your default browser")
	rootCmd.AddCommand(demoCmd)
}
