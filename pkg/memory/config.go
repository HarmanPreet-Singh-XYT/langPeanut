package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// AppConfig stores user-level and project-level settings across sessions
type AppConfig struct {
	mu                       sync.RWMutex      `json:"-"`
	ActiveProvider           string            `json:"active_provider"`
	ActiveModel              string            `json:"active_model"`
	StylePreset              string            `json:"style_preset"`
	APIKeys                  map[string]string `json:"api_keys"`
	SelectedLocales          []string          `json:"selected_locales,omitempty"`
	ChunkWordBudget          int               `json:"chunk_word_budget,omitempty"` // Words/approx tokens budget per batch
	ChunkKeyCeiling          int               `json:"chunk_key_ceiling,omitempty"` // Max keys per batch
	Concurrency              int               `json:"concurrency,omitempty"`        // Max parallel calls
	AutoGitignore            *bool             `json:"auto_gitignore,omitempty"`     // Whether to auto-add .langPeanut/ and trajectories/ to .gitignore (default: true)
	CustomInstallCmd         string            `json:"custom_install_cmd,omitempty"` // Custom shell command to install dependencies (e.g. "pnpm install", "yarn add react-i18next i18next", "flutter pub get")
	CustomBuildCmd           string            `json:"custom_build_cmd,omitempty"`   // Custom shell command for compiler/build/typecheck diagnostics (e.g. "pnpm typecheck", "npm run build", "tsc --noEmit", "flutter analyze")
	ExistingTranslationsMode string            `json:"existing_translations_mode,omitempty"` // "skip" (default), "replace" (regenerate all), "prompt"
}

var (
	globalConfigDirOverride string
	globalConfigDirMu       sync.RWMutex
)

// SetGlobalConfigDirOverride allows tests to point to a temporary test directory
func SetGlobalConfigDirOverride(dir string) {
	globalConfigDirMu.Lock()
	defer globalConfigDirMu.Unlock()
	globalConfigDirOverride = dir
}

// GetGlobalConfigDir returns ~/.langPeanut (or test override)
func GetGlobalConfigDir() string {
	globalConfigDirMu.RLock()
	override := globalConfigDirOverride
	globalConfigDirMu.RUnlock()

	if override != "" {
		return override
	}

	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".langPeanut")
}

// GetGlobalConfigPath returns ~/.langPeanut/config.json
func GetGlobalConfigPath() string {
	return filepath.Join(GetGlobalConfigDir(), "config.json")
}

// LoadConfig loads preferences from ~/.langPeanut/config.json and optional project .langPeanut/config.json
func LoadConfig(projectDir string) *AppConfig {
	cfg := &AppConfig{
		ActiveProvider:           "nllb-local",
		ActiveModel:              "nllb-200-600M-q4_k_m.gguf",
		StylePreset:              "default",
		APIKeys:                  make(map[string]string),
		SelectedLocales:          []string{"fr", "es", "de", "ja"},
		ExistingTranslationsMode: "skip",
	}

	// 1. Read global ~/.langPeanut/config.json
	globalPath := GetGlobalConfigPath()
	if data, err := os.ReadFile(globalPath); err == nil {
		_ = json.Unmarshal(data, cfg)
	}

	// 2. Read project-specific .langPeanut/config.json if present
	if projectDir != "" {
		projPath := filepath.Join(projectDir, ".langPeanut", "config.json")
		if data, err := os.ReadFile(projPath); err == nil {
			_ = json.Unmarshal(data, cfg)
		}
	}

	if cfg.APIKeys == nil {
		cfg.APIKeys = make(map[string]string)
	}

	// 3. Sync loaded API keys into environment if not already set
	for k, v := range cfg.APIKeys {
		if v != "" && os.Getenv(k) == "" {
			_ = os.Setenv(k, v)
		}
	}

	return cfg
}

// ProjectConfig stores sanitized project-level settings (WITHOUT sensitive API keys)
type ProjectConfig struct {
	ActiveProvider           string   `json:"active_provider,omitempty"`
	ActiveModel              string   `json:"active_model,omitempty"`
	StylePreset              string   `json:"style_preset,omitempty"`
	SelectedLocales          []string `json:"selected_locales,omitempty"`
	ChunkWordBudget          int      `json:"chunk_word_budget,omitempty"`
	ChunkKeyCeiling          int      `json:"chunk_key_ceiling,omitempty"`
	Concurrency              int      `json:"concurrency,omitempty"`
	AutoGitignore            *bool    `json:"auto_gitignore,omitempty"`
	CustomInstallCmd         string   `json:"custom_install_cmd,omitempty"`
	CustomBuildCmd           string   `json:"custom_build_cmd,omitempty"`
	ExistingTranslationsMode string   `json:"existing_translations_mode,omitempty"`
}

// SaveGlobal writes the full configuration (including credentials) to ~/.langPeanut/config.json with 0600 permissions
func (c *AppConfig) SaveGlobal() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	globalData, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	globalDir := GetGlobalConfigDir()
	_ = os.MkdirAll(globalDir, 0700)
	return os.WriteFile(GetGlobalConfigPath(), globalData, 0600)
}

// SaveProject writes the sanitized project configuration (WITHOUT API keys) to projectDir/.langPeanut/config.json
func (c *AppConfig) SaveProject(projectDir string) error {
	if projectDir == "" {
		return nil
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	projCfg := ProjectConfig{
		ActiveProvider:           c.ActiveProvider,
		ActiveModel:              c.ActiveModel,
		StylePreset:              c.StylePreset,
		SelectedLocales:          c.SelectedLocales,
		ChunkWordBudget:          c.ChunkWordBudget,
		ChunkKeyCeiling:          c.ChunkKeyCeiling,
		Concurrency:              c.Concurrency,
		AutoGitignore:            c.AutoGitignore,
		CustomInstallCmd:         c.CustomInstallCmd,
		CustomBuildCmd:           c.CustomBuildCmd,
		ExistingTranslationsMode: c.ExistingTranslationsMode,
	}

	projData, err := json.MarshalIndent(projCfg, "", "  ")
	if err != nil {
		return err
	}

	projDir := filepath.Join(projectDir, ".langPeanut")
	_ = os.MkdirAll(projDir, 0755)
	return os.WriteFile(filepath.Join(projDir, "config.json"), projData, 0644)
}

// Save writes preferences to global and project directories.
// SENSITIVE CREDENTIALS (APIKeys) are saved exclusively to the user's global ~/.langPeanut/config.json
// with secure 0600 file permissions and are NEVER written to the project directory.
func (c *AppConfig) Save(projectDir string) error {
	if err := c.SaveGlobal(); err != nil {
		return err
	}
	if projectDir != "" {
		return c.SaveProject(projectDir)
	}
	return nil
}

// SetAPIKey saves an API key to the config and sets the current process environment variable
func (c *AppConfig) SetAPIKey(key, value, projectDir string) error {
	c.mu.Lock()
	if c.APIKeys == nil {
		c.APIKeys = make(map[string]string)
	}
	c.APIKeys[key] = value
	c.mu.Unlock()

	_ = os.Setenv(key, value)
	return c.Save(projectDir)
}

// SetProvider saves the active provider and model choice
func (c *AppConfig) SetProvider(provider, model, projectDir string) error {
	c.mu.Lock()
	c.ActiveProvider = provider
	c.ActiveModel = model
	c.mu.Unlock()

	return c.Save(projectDir)
}

// SetStyle saves the active style preset
func (c *AppConfig) SetStyle(style, projectDir string) error {
	c.mu.Lock()
	c.StylePreset = style
	c.mu.Unlock()

	return c.Save(projectDir)
}

// SetExistingTranslationsMode saves the preference for handling existing translations (skip, replace, prompt)
func (c *AppConfig) SetExistingTranslationsMode(mode, projectDir string) error {
	c.mu.Lock()
	c.ExistingTranslationsMode = mode
	c.mu.Unlock()

	return c.Save(projectDir)
}

// GetExistingTranslationsMode retrieves the strategy for existing translations (defaults to 'skip')
func (c *AppConfig) GetExistingTranslationsMode() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.ExistingTranslationsMode == "" {
		return "skip"
	}
	return c.ExistingTranslationsMode
}

// GetAPIKey retrieves the active credential for a given provider type
func (c *AppConfig) GetAPIKey(provider string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var envVar string
	switch provider {
	case "claude":
		envVar = "ANTHROPIC_API_KEY"
	case "openai":
		envVar = "OPENAI_API_KEY"
	case "gemini":
		envVar = "GEMINI_API_KEY"
	case "deepl":
		envVar = "DEEPL_API_KEY"
	case "nllb", "nllb-cloud":
		envVar = "HF_TOKEN"
		if os.Getenv(envVar) == "" && os.Getenv("HUGGINGFACE_API_KEY") != "" {
			return os.Getenv("HUGGINGFACE_API_KEY")
		}
	case "custom":
		envVar = "OPENAI_API_KEY"
	}

	if envVar != "" {
		if val := os.Getenv(envVar); val != "" {
			return val
		}
		if c.APIKeys != nil && c.APIKeys[envVar] != "" {
			return c.APIKeys[envVar]
		}
	}
	return ""
}

// SetChunkSettings updates the batch chunking and concurrency preferences
func (c *AppConfig) SetChunkSettings(wordBudget, keyCeiling, concurrency int, projectDir string) error {
	c.mu.Lock()
	c.ChunkWordBudget = wordBudget
	c.ChunkKeyCeiling = keyCeiling
	c.Concurrency = concurrency
	c.mu.Unlock()

	return c.Save(projectDir)
}

// ShouldAutoGitignore checks whether auto_gitignore is enabled (defaults to true)
func (c *AppConfig) ShouldAutoGitignore() bool {
	if c.AutoGitignore == nil {
		return true
	}
	return *c.AutoGitignore
}

// EnsureGitignore ensures that internal runtime directories (.langPeanut/, trajectories/, etc.)
// are listed in the project's .gitignore file to prevent cluttering version control and pull requests.
func EnsureGitignore(projectRoot string) error {
	if projectRoot == "" {
		return nil
	}

	cfg := LoadConfig(projectRoot)
	if cfg != nil && !cfg.ShouldAutoGitignore() {
		return nil
	}

	gitignorePath := filepath.Join(projectRoot, ".gitignore")
	var content string
	if data, err := os.ReadFile(gitignorePath); err == nil {
		content = string(data)
	}

	entriesToAdd := []string{
		".langPeanut/",
		".langpeanut/",
		"trajectories/",
		".langPeanut-snapshots/",
		".langpeanut.lock",
	}

	var toAppend []string
	for _, entry := range entriesToAdd {
		if !strings.Contains(content, entry) {
			toAppend = append(toAppend, entry)
		}
	}

	if len(toAppend) == 0 {
		return nil
	}

	var newContent string
	if content != "" && !strings.HasSuffix(content, "\n") {
		newContent = content + "\n"
	} else {
		newContent = content
	}

	newContent += "\n# langPeanut localization runtime state & telemetry\n" + strings.Join(toAppend, "\n") + "\n"
	return os.WriteFile(gitignorePath, []byte(newContent), 0644)
}


