package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// StylePreset defines tonal adaptations for translations
type StylePreset string

const (
	StyleDefault   StylePreset = "default"
	StyleGenZ      StylePreset = "gen_z"     // e.g. "no cap", "bet", "slay", "lowkey", "fire"
	StyleCasual    StylePreset = "casual"    // friendly, approachable
	StyleFormal    StylePreset = "formal"    // corporate, strict polite
	StyleHumorous  StylePreset = "humorous"  // witty, playful
	StylePirate    StylePreset = "pirate"    // "Ahoy, Matey!"
)

// ProjectMemory stores persistent style guides, glossaries, and ignore rules
type ProjectMemory struct {
	mu             sync.RWMutex
	filePath       string
	Style          StylePreset                  `json:"style"`
	CustomPrompt   string                       `json:"custom_prompt,omitempty"`
	Glossary       map[string]map[string]string `json:"glossary,omitempty"` // locale -> (term -> custom translation)
	ExcludeFiles   []string                     `json:"exclude_files,omitempty"`
	ExcludePatterns []string                    `json:"exclude_patterns,omitempty"`
	ClassificationCache map[string]string       `json:"classification_cache,omitempty"` // hash(content:type) -> "LOCALIZABLE"/"SKIP"
}

func NewProjectMemory(cacheDir string) (*ProjectMemory, error) {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, err
	}

	memFile := filepath.Join(cacheDir, "project_memory.json")
	pm := &ProjectMemory{
		filePath:            memFile,
		Style:               StyleDefault,
		Glossary:            make(map[string]map[string]string),
		ClassificationCache: make(map[string]string),
		ExcludeFiles:        []string{"**/*.test.*", "**/*.spec.*", "**/mock/**", "**/fixtures/**"},
		ExcludePatterns:     []string{`^https?://`, `^/api/`, `^[A-Z0-9_]{3,}$`},
	}

	pm.load()
	return pm, nil
}

func (pm *ProjectMemory) load() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	data, err := os.ReadFile(pm.filePath)
	if err == nil {
		_ = json.Unmarshal(data, pm)
	}
}

func (pm *ProjectMemory) Save() error {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	data, err := json.MarshalIndent(pm, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(pm.filePath, data, 0644)
}

// GetStyleInstruction returns the prompt augmentation for the configured style
func (pm *ProjectMemory) GetStyleInstruction() string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pm.CustomPrompt != "" {
		return pm.CustomPrompt
	}

	switch pm.Style {
	case StyleGenZ:
		return "Translate using modern Gen-Z internet slang and aesthetic (use lowercase vibe where appropriate, words like 'no cap', 'slay', 'fire', 'bet', 'vibes', 'valid', while strictly preserving all ICU variable placeholders like {name})."
	case StyleCasual:
		return "Translate with a warm, friendly, casual tone suitable for modern consumer apps."
	case StyleFormal:
		return "Translate with a professional, polished, enterprise-ready tone using polite honorifics."
	case StyleHumorous:
		return "Translate with witty, cheerful, playful copy that delights the user."
	case StylePirate:
		return "Translate like a swashbuckling pirate (e.g., 'Ahoy!', 'Aye', 'Treasure') while preserving code variables."
	default:
		return "Translate clearly, accurately, and idiomatically for native app users."
	}
}

// ShouldExcludeFile checks if a file path matches any exclusion patterns
func (pm *ProjectMemory) ShouldExcludeFile(filePath string) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	normalized := filepath.ToSlash(filePath)
	for _, pattern := range pm.ExcludeFiles {
		matched, err := filepath.Match(pattern, filepath.Base(filePath))
		if err == nil && matched {
			return true
		}
		if strings.Contains(normalized, strings.Trim(pattern, "*")) {
			return true
		}
	}
	return false
}

// ShouldExcludeString checks if a string literal matches technical skip patterns
func (pm *ProjectMemory) ShouldExcludeString(s string) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	for _, pattern := range pm.ExcludePatterns {
		reg, err := regexp.Compile(pattern)
		if err == nil && reg.MatchString(s) {
			return true
		}
	}
	return false
}

// LookupGlossary checks for user-defined term overrides
func (pm *ProjectMemory) LookupGlossary(locale, term string) (string, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if localeMap, ok := pm.Glossary[locale]; ok {
		val, found := localeMap[term]
		return val, found
	}
	return "", false
}

// SetGlossary stores a custom term translation override
func (pm *ProjectMemory) SetGlossary(locale, term, translation string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.Glossary == nil {
		pm.Glossary = make(map[string]map[string]string)
	}
	if _, ok := pm.Glossary[locale]; !ok {
		pm.Glossary[locale] = make(map[string]string)
	}
	pm.Glossary[locale][term] = translation
}
