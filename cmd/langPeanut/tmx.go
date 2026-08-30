package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/langPeanut/langPeanut/pkg/memory"
	"github.com/langPeanut/langPeanut/pkg/platforms"
	"github.com/spf13/cobra"
)

var (
	exportFormat string
	exportOutput string
	importFormat string
)

var exportCmd = &cobra.Command{
	Use:   "export [directory]",
	Short: "Export translation bundles to industry standard TMX 1.4b or XLIFF 1.2 format",
	Long: `Exports discovered locale files and translation memory into standard TMX or XLIFF interchange files
compatible with enterprise TMS systems (Crowdin, Phrase, Lokalise, Trados).

EXAMPLES:
  langPeanut export --format tmx --output memory.tmx
  langPeanut export --format xliff --locales fr --output translations_fr.xliff`,
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir := projectRoot
		if len(args) > 0 {
			targetDir = args[0]
		}
		absDir, _ := filepath.Abs(targetDir)

		registry := platforms.NewRegistry()
		platform, _ := registry.AutoDetect(absDir)
		if platform == nil {
			platform, _ = registry.Get("generic")
		}

		existingLocales, err := platform.DiscoverExistingLocales(absDir)
		if err != nil || len(existingLocales) == 0 {
			return fmt.Errorf("no existing locale files found in %s", absDir)
		}

		sourceFile := platform.DefaultSourceFile(absDir, sourceLang)
		srcData, _ := os.ReadFile(sourceFile)
		sourceLocale, _ := platform.ParseLocaleFile(srcData, filepath.Ext(sourceFile))

		var units []memory.TMUnit
		for loc, path := range existingLocales {
			if strings.EqualFold(loc, sourceLang) {
				continue
			}
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			targetData, err := platform.ParseLocaleFileForLocale(data, loc)
			if err != nil || targetData == nil {
				continue
			}

			for k, v := range targetData.Entries {
				srcText := ""
				if sourceLocale != nil {
					srcText = sourceLocale.Entries[k]
				}
				if srcText == "" {
					srcText = k
				}

				units = append(units, memory.TMUnit{
					Key:        k,
					SourceLang: sourceLang,
					SourceText: srcText,
					TargetLang: loc,
					TargetText: v,
				})
			}
		}

		var outBytes []byte
		switch strings.ToLower(exportFormat) {
		case "xliff", "xlf":
			target := "es"
			if len(targetLangs) > 0 {
				target = targetLangs[0]
			}
			outBytes, err = memory.ExportXLIFF(units, sourceLang, target)
			if exportOutput == "" {
				exportOutput = fmt.Sprintf("bundle_%s.xliff", target)
			}
		default:
			outBytes, err = memory.ExportTMX(units, sourceLang)
			if exportOutput == "" {
				exportOutput = "translations.tmx"
			}
		}

		if err != nil {
			return fmt.Errorf("export failed: %w", err)
		}

		if err := os.WriteFile(exportOutput, outBytes, 0644); err != nil {
			return fmt.Errorf("failed to write export file: %w", err)
		}

		fmt.Printf("✓ Successfully exported %d translation unit(s) to %s (%s format)\n", len(units), exportOutput, strings.ToUpper(exportFormat))
		return nil
	},
}

var importCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import standard TMX 1.4b or XLIFF 1.2 translation memory file into cache",
	Long: `Imports translation segments from a TMX or XLIFF interchange file into the project's
local Translation Memory cache (.langPeanut/cache/translations_memory.json), warming cache for instant 0-cost hits.

EXAMPLES:
  langPeanut import memory.tmx
  langPeanut import legacy_translations.xliff`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]
		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("could not read file %s: %w", filePath, err)
		}

		var units []memory.TMUnit
		if strings.HasSuffix(filePath, ".xliff") || strings.HasSuffix(filePath, ".xlf") || importFormat == "xliff" {
			units, err = memory.ImportXLIFF(data)
		} else {
			units, err = memory.ImportTMX(data)
		}

		if err != nil {
			return fmt.Errorf("import failed: %w", err)
		}

		cacheDir := filepath.Join(projectRoot, ".langPeanut", "cache")
		tm, err := memory.NewTranslationMemory(cacheDir)
		if err != nil {
			return fmt.Errorf("failed to open translation memory: %w", err)
		}

		added := memory.MergeUnitsIntoTM(tm, units)
		fmt.Printf("✓ Successfully imported %d translation segment(s) into Translation Memory (%s)\n", added, cacheDir)
		return nil
	},
}

func init() {
	exportCmd.Flags().StringVarP(&exportFormat, "format", "f", "tmx", "Export format: 'tmx' (Translation Memory eXchange) or 'xliff' (XLIFF 1.2)")
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "Destination output file path")

	importCmd.Flags().StringVarP(&importFormat, "format", "f", "", "Format override ('tmx' or 'xliff')")

	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(importCmd)
}
