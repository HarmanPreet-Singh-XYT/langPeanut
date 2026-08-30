package seo

import (
	"os"
	"path/filepath"

	"github.com/langPeanut/langPeanut/pkg/platforms"
	"github.com/langPeanut/langPeanut/pkg/types"
)

// ExtractLocaleCatalog reads existing translations for a locale from the project
func ExtractLocaleCatalog(projectRoot string, platform platforms.Platform, locale string) map[string]string {
	if platform == nil {
		return nil
	}
	existing, err := platform.DiscoverExistingLocales(projectRoot)
	if err == nil && existing[locale] != "" {
		filePath := existing[locale]
		if data, err := os.ReadFile(filePath); err == nil {
			if locData, err := platform.ParseLocaleFileForLocale(data, locale); err == nil && locData != nil {
				return locData.Entries
			}
		}
	}
	// Try default locale file path
	defaultFile := platform.DefaultSourceFile(projectRoot, locale)
	if data, err := os.ReadFile(defaultFile); err == nil {
		if locData, err := platform.ParseLocaleFileForLocale(data, locale); err == nil && locData != nil {
			return locData.Entries
		}
	}
	return nil
}

// WriteLocaleCatalog writes translation entries to disk for a given locale
func WriteLocaleCatalog(projectRoot string, platform platforms.Platform, locale string, entries map[string]string) error {
	if platform == nil {
		return nil
	}
	existing, _ := platform.DiscoverExistingLocales(projectRoot)
	filePath := existing[locale]
	if filePath == "" {
		filePath = platform.DefaultSourceFile(projectRoot, locale)
	}

	localeData := types.LocaleData{
		LocaleCode: locale,
		Format:     filepath.Ext(filePath),
		Entries:    entries,
	}

	formatted, err := platform.FormatLocaleFile(localeData)
	if err != nil {
		return err
	}

	_ = os.MkdirAll(filepath.Dir(filePath), 0755)
	return os.WriteFile(filePath, formatted, 0644)
}
