package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
		ensureGitignore(absRoot)
		fmt.Println()

		fmt.Println("Ready! Next steps:")
		fmt.Println("  langPeanut web         → launch interactive browser studio")
		fmt.Println("  langPeanut audit       → scan and report hardcoded strings")
		fmt.Println("  langPeanut extract     → extract and review candidates")
		fmt.Println("  langPeanut translate   → auto-translate to target locales")
		return nil
	},
}

func ensureGitignore(projectRoot string) {
	gitignorePath := filepath.Join(projectRoot, ".gitignore")
	var content string
	if data, err := os.ReadFile(gitignorePath); err == nil {
		content = string(data)
	}

	entriesToAdd := []string{".langPeanut/", ".langpeanut/"}
	var toAppend []string
	for _, entry := range entriesToAdd {
		if !strings.Contains(content, entry) {
			toAppend = append(toAppend, entry)
		}
	}

	if len(toAppend) > 0 {
		var newContent string
		if content != "" && !strings.HasSuffix(content, "\n") {
			newContent = content + "\n"
		} else {
			newContent = content
		}
		newContent += "\n# langPeanut CLI Runtime State\n" + strings.Join(toAppend, "\n") + "\n"
		_ = os.WriteFile(gitignorePath, []byte(newContent), 0644)
		fmt.Printf("✓ Added %s to .gitignore\n", strings.Join(toAppend, ", "))
	}
}

func init() {
	rootCmd.AddCommand(initCmd)
}
