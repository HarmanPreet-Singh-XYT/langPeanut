package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/langPeanut/langPeanut/pkg/memory"
	"github.com/langPeanut/langPeanut/pkg/platforms"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Detect framework and initialize langPeanut configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		absRoot, err := filepath.Abs(projectRoot)
		if err != nil {
			return err
		}

		fmt.Println("langPeanut — Scanning project...")

		registry := platforms.NewRegistry()
		platform, confidence := registry.AutoDetect(absRoot)

		fmt.Printf("\n✓ Detected Framework: %s (Confidence: %.0f%%)\n", platform.DisplayName(), confidence*100)

		// Create .langPeanut directory
		dotDir := filepath.Join(absRoot, ".langPeanut")
		_ = os.MkdirAll(dotDir, 0755)

		// Create default config file
		configContent := fmt.Sprintf(`# langPeanut Configuration
framework: %s
source_locale: %s
target_locales: [%s]
locale_dir: %s
confidence_threshold: 0.85
review_mode: interactive
strict_dedup: true
`, platform.Name(), sourceLang, "fr, es, de, ja", platform.DefaultLocaleDir(absRoot))

		configFile := filepath.Join(absRoot, "langPeanut.yaml")
		if _, err := os.Stat(configFile); os.IsNotExist(err) {
			_ = os.WriteFile(configFile, []byte(configContent), 0644)
			fmt.Printf("✓ Created config: langPeanut.yaml\n")
		} else {
			fmt.Printf("✓ Existing config found: langPeanut.yaml\n")
		}

		// Scaffold locale directory
		locDir := filepath.Join(absRoot, platform.DefaultLocaleDir(absRoot))
		_ = os.MkdirAll(locDir, 0755)
		fmt.Printf("✓ Scaffolded locale directory: %s\n", platform.DefaultLocaleDir(absRoot))

		// Ensure .gitignore ignores CLI runtime cache
		_ = memory.EnsureGitignore(absRoot)
		fmt.Println("✓ Verified .gitignore for .langPeanut/ and trajectories/")
		fmt.Println()

		fmt.Println("Ready! Next steps:")
		fmt.Println("  langPeanut web         → launch interactive browser studio")
		fmt.Println("  langPeanut audit       → scan and report hardcoded strings")
		fmt.Println("  langPeanut extract     → extract and review candidates")
		fmt.Println("  langPeanut translate   → auto-translate to target locales")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
