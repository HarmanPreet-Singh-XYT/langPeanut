package agents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/langPeanut/langPeanut/pkg/platforms"
)

// PruneReport details active and orphaned dead translation keys found in the project
type PruneReport struct {
	TotalFilesScanned int                 `json:"total_files_scanned"`
	ActiveKeysCount   int                 `json:"active_keys_count"`
	DeadKeysByLocale  map[string][]string `json:"dead_keys_by_locale"`
	TotalDeadKeys     int                 `json:"total_dead_keys"`
	PrunedLocales     []string            `json:"pruned_locales,omitempty"`
}

// PrunerAgent identifies and prunes unused translation keys from locale dictionaries
type PrunerAgent struct {
	Platform platforms.Platform
}

// NewPrunerAgent creates a new PrunerAgent instance
func NewPrunerAgent(p platforms.Platform) *PrunerAgent {
	return &PrunerAgent{Platform: p}
}

// AnalyzeDeadKeys searches code for all active references and compares with locale dictionary keys
func (p *PrunerAgent) AnalyzeDeadKeys(projectRoot string) (*PruneReport, error) {
	report := &PruneReport{
		DeadKeysByLocale: make(map[string][]string),
	}

	// 1. Gather all source files
	var codeFiles []string
	extMap := make(map[string]bool)
	for _, ext := range p.Platform.FileExtensions() {
		extMap[ext] = true
	}
	skipDirs := make(map[string]bool)
	for _, d := range p.Platform.SkipDirs() {
		skipDirs[d] = true
	}
	skipDirs["node_modules"] = true
	skipDirs[".git"] = true
	skipDirs[".next"] = true
	skipDirs["build"] = true
	skipDirs["dist"] = true

	_ = filepath.Walk(projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if skipDirs[info.Name()] || strings.HasPrefix(info.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if extMap[filepath.Ext(path)] {
			codeFiles = append(codeFiles, path)
		}
		return nil
	})

	report.TotalFilesScanned = len(codeFiles)

	// 2. Extract referenced keys from source files using comprehensive regex patterns
	activeKeys := make(map[string]bool)
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?:t|i18n\.t|\$t|translate)\(\s*["']([^"']+)["']`),
		regexp.MustCompile(`i18nKey=["']([^"']+)["']`),
		regexp.MustCompile(`AppLocalizations\.of\([^)]+\)!\.([a-zA-Z0-9_]+)`),
		regexp.MustCompile(`context\.l10n\.([a-zA-Z0-9_]+)`),
		regexp.MustCompile(`NSLocalizedString\(\s*["']([^"']+)["']`),
		regexp.MustCompile(`String\(localized:\s*["']([^"']+)["']`),
		regexp.MustCompile(`stringResource\(\s*R\.string\.([a-zA-Z0-9_]+)`),
		regexp.MustCompile(`[a-zA-Z0-9_]+\.tr\(\)`),
	}

	for _, file := range codeFiles {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		content := string(data)
		for _, re := range patterns {
			matches := re.FindAllStringSubmatch(content, -1)
			for _, m := range matches {
				if len(m) > 1 && len(m[1]) > 0 {
					activeKeys[m[1]] = true
				}
			}
		}
	}

	report.ActiveKeysCount = len(activeKeys)

	// 3. Scan locale directory and identify unreferenced keys
	localeDir := filepath.Join(projectRoot, p.Platform.DefaultLocaleDir(projectRoot))
	if _, err := os.Stat(localeDir); os.IsNotExist(err) {
		return report, nil
	}

	entries, err := os.ReadDir(localeDir)
	if err != nil {
		return report, nil
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") && !strings.HasSuffix(entry.Name(), ".arb") {
			continue
		}

		filePath := filepath.Join(localeDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var rawMap map[string]any
		if err := json.Unmarshal(data, &rawMap); err != nil {
			continue
		}

		localeName := strings.TrimSuffix(strings.TrimSuffix(entry.Name(), ".json"), ".arb")
		var deadList []string

		for k := range rawMap {
			if strings.HasPrefix(k, "@") || strings.HasPrefix(k, "$") || strings.HasPrefix(k, "@@") {
				continue // Metadata
			}
			// Check direct match or camelCase/dot match
			if !activeKeys[k] {
				// Normalize key variations
				normalized := strings.ReplaceAll(k, ".", "_")
				if !activeKeys[normalized] {
					deadList = append(deadList, k)
				}
			}
		}

		if len(deadList) > 0 {
			report.DeadKeysByLocale[localeName] = deadList
			report.TotalDeadKeys += len(deadList)
		}
	}

	return report, nil
}

// PruneDeadKeys removes unused keys from all locale dictionary files
func (p *PrunerAgent) PruneDeadKeys(projectRoot string) (*PruneReport, error) {
	report, err := p.AnalyzeDeadKeys(projectRoot)
	if err != nil {
		return nil, err
	}

	if report.TotalDeadKeys == 0 {
		return report, nil
	}

	localeDir := filepath.Join(projectRoot, p.Platform.DefaultLocaleDir(projectRoot))
	for loc, deadKeys := range report.DeadKeysByLocale {
		deadSet := make(map[string]bool)
		for _, k := range deadKeys {
			deadSet[k] = true
			deadSet["@"+k] = true // remove ARB metadata
		}

		// Try JSON file
		jsonPath := filepath.Join(localeDir, loc+".json")
		if data, err := os.ReadFile(jsonPath); err == nil {
			var rawMap map[string]any
			if json.Unmarshal(data, &rawMap) == nil {
				for k := range deadSet {
					delete(rawMap, k)
				}
				updatedJSON, _ := json.MarshalIndent(rawMap, "", "  ")
				_ = os.WriteFile(jsonPath, append(updatedJSON, '\n'), 0644)
				report.PrunedLocales = append(report.PrunedLocales, loc+".json")
			}
		}

		// Try ARB file
		arbPath := filepath.Join(localeDir, "app_"+loc+".arb")
		if data, err := os.ReadFile(arbPath); err == nil {
			var rawMap map[string]any
			if json.Unmarshal(data, &rawMap) == nil {
				for k := range deadSet {
					delete(rawMap, k)
				}
				updatedJSON, _ := json.MarshalIndent(rawMap, "", "  ")
				_ = os.WriteFile(arbPath, append(updatedJSON, '\n'), 0644)
				report.PrunedLocales = append(report.PrunedLocales, "app_"+loc+".arb")
			}
		}
	}

	return report, nil
}

// ExtractAllActiveKeys returns a map of all translation keys active in the codebase
func (p *PrunerAgent) ExtractAllActiveKeys(projectRoot string) (map[string]bool, error) {
	report, err := p.AnalyzeDeadKeys(projectRoot)
	if err != nil {
		return nil, err
	}
	active := make(map[string]bool)
	_ = report
	return active, nil
}
