package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/langPeanut/langPeanut/pkg/agents"
	"github.com/langPeanut/langPeanut/pkg/llm"
	"github.com/spf13/cobra"
)

var personaJSON bool

var personaCmd = &cobra.Command{
	Use:   "persona",
	Short: "Autonomous Brand Persona & Glossary Mining Agent (Zero-Config setup)",
	Long:  "Scans repository documentation, manifests, and README to automatically discover brand keywords, tone, and audience profile.",
	RunE: func(cmd *cobra.Command, args []string) error {
		absRoot, err := filepath.Abs(projectRoot)
		if err != nil {
			return err
		}

		client := llm.AutoDetectClient()
		scout := agents.NewPersonaScoutAgent(client)
		fmt.Printf("🔍 Running Persona Scout Agent on %s...\n", absRoot)

		report, err := scout.DiscoverPersona(absRoot)
		if err != nil {
			return fmt.Errorf("persona discovery failed: %w", err)
		}

		if personaJSON {
			data, _ := json.MarshalIndent(report, "", "  ")
			fmt.Println(string(data))
			return nil
		}

		fmt.Printf("\n✨ Brand Persona & Lexicon Discovery Report:\n")
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("• Project Name:       %s\n", report.ProjectName)
		fmt.Printf("• Discovered Tone:    %s\n", report.RecommendedTone)
		fmt.Printf("• Target Audience:    %s\n", report.Audience)
		fmt.Printf("• Discovered Lexicon: [%s]\n", strings.Join(report.BrandLexicon, ", "))
		fmt.Printf("• Suggested Locales:  [%s]\n", strings.Join(report.LocalesSuggested, ", "))
		fmt.Printf("• Confidence Score:   %.0f%%\n", report.ConfidenceScore*100)
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("💡 Summary: %s\n\n", report.Summary)
		return nil
	},
}

func init() {
	personaCmd.Flags().BoolVar(&personaJSON, "json", false, "Output persona report as JSON")
	rootCmd.AddCommand(personaCmd)
}
