package platforms

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReactEnsureDependencies_InjectsMissingDependenciesAndBootstrap(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Create a minimal package.json without react-i18next or i18next
	initialPkg := map[string]any{
		"name":    "demo-react-app",
		"version": "1.0.0",
		"dependencies": map[string]any{
			"react":     "^18.2.0",
			"react-dom": "^18.2.0",
		},
	}
	pkgBytes, _ := json.MarshalIndent(initialPkg, "", "  ")
	_ = os.WriteFile(filepath.Join(tmpDir, "package.json"), pkgBytes, 0644)
	_ = os.MkdirAll(filepath.Join(tmpDir, "src"), 0755)

	// Check dependencies before
	checkStatus, err := ReactCheckDependencies(tmpDir)
	if err != nil {
		t.Fatalf("ReactCheckDependencies returned error: %v", err)
	}
	if len(checkStatus.MissingDeps) != 2 {
		t.Errorf("expected 2 missing deps, got: %v", checkStatus.MissingDeps)
	}

	// Ensure dependencies (without autoInstall executing external npm binary in unit test)
	status, err := ReactEnsureDependencies(tmpDir, false)
	if err != nil {
		t.Fatalf("ReactEnsureDependencies returned error: %v", err)
	}

	if !status.Success {
		t.Errorf("expected status.Success to be true")
	}
	if !status.ManifestUpdated {
		t.Errorf("expected ManifestUpdated to be true")
	}

	// Verify package.json contains react-i18next and i18next
	updatedBytes, err := os.ReadFile(filepath.Join(tmpDir, "package.json"))
	if err != nil {
		t.Fatalf("could not read updated package.json: %v", err)
	}
	var updatedPkg map[string]any
	if err := json.Unmarshal(updatedBytes, &updatedPkg); err != nil {
		t.Fatalf("updated package.json is invalid JSON: %v", err)
	}

	deps, ok := updatedPkg["dependencies"].(map[string]any)
	if !ok {
		t.Fatalf("expected dependencies object in package.json")
	}
	if _, ok := deps["react-i18next"]; !ok {
		t.Errorf("expected react-i18next in package.json dependencies")
	}
	if _, ok := deps["i18next"]; !ok {
		t.Errorf("expected i18next in package.json dependencies")
	}
	// Verify original dependencies preserved
	if _, ok := deps["react"]; !ok {
		t.Errorf("expected original react dependency to be preserved")
	}

	// Verify i18n.ts bootstrap was created in src/
	bootstrapPath := filepath.Join(tmpDir, "src", "i18n.ts")
	if !FileExists(tmpDir, "src/i18n.ts") {
		t.Errorf("expected src/i18n.ts bootstrap file to be created")
	}
	bootstrapContent, _ := os.ReadFile(bootstrapPath)
	if !strings.Contains(string(bootstrapContent), "initReactI18next") {
		t.Errorf("expected src/i18n.ts to configure initReactI18next, got:\n%s", string(bootstrapContent))
	}
}

func TestFlutterEnsureDependencies_InjectsPubspecAndL10nYaml(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Create a minimal pubspec.yaml
	initialPubspec := `name: demo_flutter_app
description: "A demo flutter application"
version: 1.0.0+1

environment:
  sdk: '>=3.0.0 <4.0.0'

dependencies:
  flutter:
    sdk: flutter
  cupertino_icons: ^1.0.2

flutter:
  uses-material-design: true
`
	_ = os.WriteFile(filepath.Join(tmpDir, "pubspec.yaml"), []byte(initialPubspec), 0644)

	// Check dependencies before
	checkStatus, err := FlutterCheckDependencies(tmpDir)
	if err != nil {
		t.Fatalf("FlutterCheckDependencies returned error: %v", err)
	}
	if len(checkStatus.MissingDeps) == 0 {
		t.Errorf("expected missing deps in unconfigured pubspec.yaml")
	}

	// Ensure dependencies
	status, err := FlutterEnsureDependencies(tmpDir, false)
	if err != nil {
		t.Fatalf("FlutterEnsureDependencies returned error: %v", err)
	}

	if !status.Success {
		t.Errorf("expected status.Success to be true")
	}
	if !status.ManifestUpdated {
		t.Errorf("expected ManifestUpdated to be true")
	}

	// Verify pubspec.yaml contains flutter_localizations and generate: true
	updatedContent, err := os.ReadFile(filepath.Join(tmpDir, "pubspec.yaml"))
	if err != nil {
		t.Fatalf("could not read updated pubspec.yaml: %v", err)
	}
	contentStr := string(updatedContent)

	if !strings.Contains(contentStr, "flutter_localizations:") {
		t.Errorf("expected flutter_localizations in pubspec.yaml, got:\n%s", contentStr)
	}
	if !strings.Contains(contentStr, "intl:") {
		t.Errorf("expected intl in pubspec.yaml, got:\n%s", contentStr)
	}
	if !strings.Contains(contentStr, "generate: true") {
		t.Errorf("expected generate: true under flutter:, got:\n%s", contentStr)
	}
	if !strings.Contains(contentStr, "cupertino_icons:") {
		t.Errorf("expected existing cupertino_icons to be preserved in pubspec.yaml")
	}

	// Verify l10n.yaml created
	if !FileExists(tmpDir, "l10n.yaml") {
		t.Errorf("expected l10n.yaml to be created")
	}
	l10nBytes, _ := os.ReadFile(filepath.Join(tmpDir, "l10n.yaml"))
	if !strings.Contains(string(l10nBytes), "template-arb-file") {
		t.Errorf("expected l10n.yaml to define template-arb-file")
	}
}

func TestAndroidAndSwiftDependencies(t *testing.T) {
	tmpDir := t.TempDir()

	androidStatus, err := AndroidEnsureDependencies(tmpDir, false)
	if err != nil || !androidStatus.Success {
		t.Errorf("expected AndroidEnsureDependencies to succeed")
	}
	if !DirExists(tmpDir, "app/src/main/res/values") {
		t.Errorf("expected Android res/values directory to be created")
	}

	swiftStatus, err := SwiftEnsureDependencies(tmpDir, false)
	if err != nil || !swiftStatus.Success {
		t.Errorf("expected SwiftEnsureDependencies to succeed")
	}
	if !DirExists(tmpDir, "Resources") {
		t.Errorf("expected Swift Resources directory to be created")
	}
}

func TestCustomCommands_InstallAndBuild(t *testing.T) {
	tmpDir := t.TempDir()

	// Test custom install command execution
	cmd, out, err := ExecuteCustomCommand(tmpDir, "echo 'custom install completed'")
	if err != nil {
		t.Fatalf("ExecuteCustomCommand failed: %v", err)
	}
	if !strings.Contains(out, "custom install completed") {
		t.Errorf("expected output to contain 'custom install completed', got: %s", out)
	}
	if cmd != "echo 'custom install completed'" {
		t.Errorf("expected cmd to match input, got: %s", cmd)
	}

	// Test custom build diagnostics
	diags, err := RunDiagnosticsWithCustom(tmpDir, []string{}, "echo 'build ok'")
	if err != nil {
		t.Fatalf("RunDiagnosticsWithCustom failed: %v", err)
	}
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics for successful build, got: %v", diags)
	}

	// Test custom build failure captures diagnostic
	failDiags, err := RunDiagnosticsWithCustom(tmpDir, []string{}, "echo 'FATAL_TYPE_ERROR: syntax failure' && exit 1")
	if err != nil {
		t.Fatalf("RunDiagnosticsWithCustom returned unexpected err: %v", err)
	}
	if len(failDiags) == 0 {
		t.Errorf("expected diagnostic for failed build command")
	}
}

