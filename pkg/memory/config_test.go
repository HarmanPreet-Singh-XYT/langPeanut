package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureGitignore_CreatesAndAppends(t *testing.T) {
	tempGlobal, err := os.MkdirTemp("", "langpeanut-global-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(tempGlobal)
	SetGlobalConfigDirOverride(tempGlobal)
	defer SetGlobalConfigDirOverride("")

	tempDir, err := os.MkdirTemp("", "langpeanut-gitignore-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test 1: File does not exist yet -> should create .gitignore with .langPeanut/ and trajectories/
	if err := EnsureGitignore(tempDir); err != nil {
		t.Fatalf("EnsureGitignore failed on clean dir: %v", err)
	}

	gitignorePath := filepath.Join(tempDir, ".gitignore")
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("failed to read created .gitignore: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, ".langPeanut/") {
		t.Errorf("Expected .langPeanut/ in .gitignore, got:\n%s", content)
	}
	if !strings.Contains(content, "trajectories/") {
		t.Errorf("Expected trajectories/ in .gitignore, got:\n%s", content)
	}

	// Test 2: Idempotency -> calling again should not duplicate entries
	if err := EnsureGitignore(tempDir); err != nil {
		t.Fatalf("EnsureGitignore second call failed: %v", err)
	}

	data2, _ := os.ReadFile(gitignorePath)
	count := strings.Count(string(data2), ".langPeanut/")
	if count != 1 {
		t.Errorf("Expected exactly 1 occurrence of .langPeanut/, got %d", count)
	}

	// Test 3: Existing user .gitignore entries are preserved
	customDir, err := os.MkdirTemp("", "langpeanut-existing-gitignore-*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(customDir)

	existingContent := "node_modules/\n.env\ndist/\n"
	_ = os.WriteFile(filepath.Join(customDir, ".gitignore"), []byte(existingContent), 0644)

	if err := EnsureGitignore(customDir); err != nil {
		t.Fatalf("EnsureGitignore failed with existing content: %v", err)
	}

	customData, _ := os.ReadFile(filepath.Join(customDir, ".gitignore"))
	customStr := string(customData)
	if !strings.Contains(customStr, "node_modules/") || !strings.Contains(customStr, ".env") {
		t.Errorf("Existing .gitignore entries were clobbered:\n%s", customStr)
	}
	if !strings.Contains(customStr, ".langPeanut/") {
		t.Errorf("Expected .langPeanut/ appended to existing .gitignore")
	}
}

func TestAppConfig_AutoGitignoreTogglable(t *testing.T) {
	tempGlobal, err := os.MkdirTemp("", "langpeanut-global-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(tempGlobal)
	SetGlobalConfigDirOverride(tempGlobal)
	defer SetGlobalConfigDirOverride("")

	tempDir, err := os.MkdirTemp("", "langpeanut-toggle-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// When AutoGitignore is explicitly false
	disabled := false
	cfg := &AppConfig{
		AutoGitignore: &disabled,
	}
	_ = cfg.Save(tempDir)

	if err := EnsureGitignore(tempDir); err != nil {
		t.Fatalf("EnsureGitignore failed: %v", err)
	}

	// .gitignore should NOT exist when disabled
	gitignorePath := filepath.Join(tempDir, ".gitignore")
	if _, err := os.Stat(gitignorePath); !os.IsNotExist(err) {
		t.Errorf("Expected .gitignore NOT to be created when auto_gitignore=false")
	}
}

func TestAppConfig_ProjectConfigDoesNotContainAPIKeys(t *testing.T) {
	tempGlobal, err := os.MkdirTemp("", "langpeanut-global-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(tempGlobal)
	SetGlobalConfigDirOverride(tempGlobal)
	defer SetGlobalConfigDirOverride("")

	tempDir, err := os.MkdirTemp("", "langpeanut-apikey-isolation-*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := &AppConfig{
		ActiveProvider:  "openai",
		ActiveModel:     "gpt-4o",
		StylePreset:     "casual",
		SelectedLocales: []string{"es", "fr"},
		APIKeys: map[string]string{
			"OPENAI_API_KEY":    "sk-secret-1234567890",
			"ANTHROPIC_API_KEY": "sk-ant-secret-abcdef",
		},
	}

	if err := cfg.Save(tempDir); err != nil {
		t.Fatalf("cfg.Save failed: %v", err)
	}

	// Read project config file
	projConfigFile := filepath.Join(tempDir, ".langPeanut", "config.json")
	data, err := os.ReadFile(projConfigFile)
	if err != nil {
		t.Fatalf("failed to read project config: %v", err)
	}

	projStr := string(data)

	// SENSITIVE CREDENTIAL CHECKS
	if strings.Contains(projStr, "sk-secret") || strings.Contains(projStr, "sk-ant") {
		t.Fatalf("SECURITY VIOLATION: Secret API key was written to project directory config.json:\n%s", projStr)
	}
	if strings.Contains(projStr, "api_keys") {
		t.Fatalf("SECURITY VIOLATION: 'api_keys' field was found in project directory config.json:\n%s", projStr)
	}

	// Non-sensitive preferences should still be preserved in project config
	if !strings.Contains(projStr, "openai") || !strings.Contains(projStr, "gpt-4o") {
		t.Errorf("Expected non-sensitive preferences in project config, got:\n%s", projStr)
	}

	// Read global config file to verify keys ARE stored safely in global home
	globalConfigFile := filepath.Join(tempGlobal, "config.json")
	globalData, err := os.ReadFile(globalConfigFile)
	if err != nil {
		t.Fatalf("failed to read global config: %v", err)
	}

	globalStr := string(globalData)
	if !strings.Contains(globalStr, "sk-secret-1234567890") {
		t.Errorf("Expected API key in global user config, got:\n%s", globalStr)
	}
}
