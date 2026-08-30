package platforms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/langPeanut/langPeanut/pkg/logger"
	"github.com/langPeanut/langPeanut/pkg/types"
)

// ReactCheckDependencies checks if react-i18next and i18next are declared or installed in a React/Next.js project
func ReactCheckDependencies(projectRoot string) (*types.DependencyStatus, error) {
	status := &types.DependencyStatus{
		Framework:    types.FrameworkReact,
		ManifestFile: "package.json",
		Success:      true,
	}

	pkgPath := filepath.Join(projectRoot, "package.json")
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		status.Success = false
		status.Message = "package.json not found in project root"
		return status, nil
	}

	var pkg map[string]any
	if err := json.Unmarshal(data, &pkg); err != nil {
		status.Success = false
		status.Message = fmt.Sprintf("invalid package.json: %v", err)
		return status, nil
	}

	hasReactI18next := hasPackageDep(pkg, "react-i18next")
	hasI18next := hasPackageDep(pkg, "i18next")

	if !hasReactI18next {
		status.MissingDeps = append(status.MissingDeps, "react-i18next")
	}
	if !hasI18next {
		status.MissingDeps = append(status.MissingDeps, "i18next")
	}

	if len(status.MissingDeps) == 0 {
		status.InstalledDeps = []string{"react-i18next", "i18next"}
		status.Message = "All required React localization dependencies are declared in package.json"
	} else {
		status.Message = fmt.Sprintf("Missing dependencies in package.json: %s", strings.Join(status.MissingDeps, ", "))
	}

	return status, nil
}

// ReactEnsureDependencies injects missing packages into package.json, creates i18n bootstrap, and runs package manager install
func ReactEnsureDependencies(projectRoot string, autoInstall bool) (*types.DependencyStatus, error) {
	return ReactEnsureDependenciesWithCustom(projectRoot, autoInstall, "")
}

// ReactEnsureDependenciesWithCustom injects missing packages into package.json, creates i18n bootstrap, and runs custom or auto-detected install command
func ReactEnsureDependenciesWithCustom(projectRoot string, autoInstall bool, customCmd string) (*types.DependencyStatus, error) {
	status, err := ReactCheckDependencies(projectRoot)
	if err != nil {
		return status, err
	}

	pkgPath := filepath.Join(projectRoot, "package.json")
	if FileExists(projectRoot, "package.json") && len(status.MissingDeps) > 0 {
		data, err := os.ReadFile(pkgPath)
		if err == nil {
			var pkg map[string]any
			if json.Unmarshal(data, &pkg) == nil {
				deps, ok := pkg["dependencies"].(map[string]any)
				if !ok || deps == nil {
					deps = make(map[string]any)
				}

				if !hasPackageDep(pkg, "react-i18next") {
					deps["react-i18next"] = "^14.1.2"
					status.InstalledDeps = append(status.InstalledDeps, "react-i18next")
				}
				if !hasPackageDep(pkg, "i18next") {
					deps["i18next"] = "^23.11.5"
					status.InstalledDeps = append(status.InstalledDeps, "i18next")
				}

				pkg["dependencies"] = deps
				formatted, err := json.MarshalIndent(pkg, "", "  ")
				if err == nil {
					_ = os.WriteFile(pkgPath, append(formatted, '\n'), 0644)
					status.ManifestUpdated = true
				}
			}
		}
	}

	// Ensure an i18n setup bootstrap file exists (e.g. src/i18n.ts or i18n.ts)
	bootstrapCreated := ensureReactI18nBootstrap(projectRoot)
	if bootstrapCreated != "" {
		status.ConfigCreated = append(status.ConfigCreated, bootstrapCreated)
	}

	// Execute package manager install if requested
	if autoInstall {
		if strings.TrimSpace(customCmd) != "" {
			cmdStr, out, execErr := ExecuteCustomCommand(projectRoot, customCmd)
			status.CommandExecuted = cmdStr
			status.CommandOutput = out
			if execErr != nil {
				logger.Get().Warn("DEPENDENCIES", fmt.Sprintf("Custom install command '%s' returned notice/warning: %v", customCmd, execErr))
			}
		} else if len(status.InstalledDeps) > 0 {
			cmdStr, out, execErr := executeNodeInstall(projectRoot, status.InstalledDeps)
			status.CommandExecuted = cmdStr
			status.CommandOutput = out
			if execErr != nil {
				logger.Get().Warn("DEPENDENCIES", fmt.Sprintf("Package manager install returned notice/warning: %v (package.json was updated)", execErr))
			}
		}
	}

	if len(status.MissingDeps) > 0 {
		status.Message = fmt.Sprintf("Added %s to package.json and ensured i18n initialization", strings.Join(status.MissingDeps, ", "))
	} else {
		status.Message = "React localization dependencies are up to date"
	}
	status.Success = true

	return status, nil
}

// ExecuteCustomCommand executes a custom shell command within the project root directory
func ExecuteCustomCommand(projectRoot, cmdLine string) (string, string, error) {
	if strings.TrimSpace(cmdLine) == "" {
		return "", "", nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if isWindows() {
		cmd = exec.CommandContext(ctx, "cmd.exe", "/c", cmdLine)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", cmdLine)
	}
	cmd.Dir = projectRoot
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf

	err := cmd.Run()
	outputStr := strings.TrimSpace(outBuf.String())
	return cmdLine, outputStr, err
}

func isWindows() bool {
	return os.PathSeparator == '\\'
}

func hasPackageDep(pkg map[string]any, name string) bool {
	if deps, ok := pkg["dependencies"].(map[string]any); ok && deps != nil {
		if _, exists := deps[name]; exists {
			return true
		}
	}
	if devDeps, ok := pkg["devDependencies"].(map[string]any); ok && devDeps != nil {
		if _, exists := devDeps[name]; exists {
			return true
		}
	}
	return false
}

// ensureReactI18nBootstrap ensures a default i18n bootstrap config file exists for react-i18next
func ensureReactI18nBootstrap(projectRoot string) string {
	return EnsureReactI18nBootstrapWithLocales(projectRoot, nil)
}

// EnsureReactI18nBootstrapWithLocales creates or updates the i18n setup bootstrap file with all configured locale JSON resources
func EnsureReactI18nBootstrapWithLocales(projectRoot string, locales []string) string {
	candidates := []string{
		"src/i18n.ts",
		"src/i18n.js",
		"src/i18n.tsx",
		"src/i18n.jsx",
		"src/lib/i18n.ts",
		"src/lib/i18n.js",
		"lib/i18n.ts",
		"lib/i18n.js",
		"i18n.ts",
		"i18n.js",
	}

	targetRel := ""
	for _, rel := range candidates {
		if FileExists(projectRoot, rel) {
			targetRel = rel
			break
		}
	}

	if targetRel == "" {
		if DirExists(projectRoot, "src") {
			targetRel = "src/i18n.ts"
		} else {
			targetRel = "i18n.ts"
		}
	}

	targetAbs := filepath.Join(projectRoot, targetRel)
	_ = os.MkdirAll(filepath.Dir(targetAbs), 0755)

	// Determine locale directory
	localeDirRel := "locales"
	if DirExists(projectRoot, "src/locales") {
		localeDirRel = "src/locales"
	} else if DirExists(projectRoot, "public/locales") {
		localeDirRel = "public/locales"
	} else if DirExists(projectRoot, "locales") {
		localeDirRel = "locales"
	} else if DirExists(projectRoot, "src") {
		localeDirRel = "src/locales"
	}

	absLocaleDir := filepath.Join(projectRoot, localeDirRel)
	relPathToLocales, err := filepath.Rel(filepath.Dir(targetAbs), absLocaleDir)
	if err != nil {
		relPathToLocales = "./" + localeDirRel
	}
	relPathToLocales = filepath.ToSlash(relPathToLocales)
	if !strings.HasPrefix(relPathToLocales, ".") {
		relPathToLocales = "./" + relPathToLocales
	}

	// Discover existing locale files on disk or use provided list
	locSet := make(map[string]bool)
	for _, l := range locales {
		if l != "" {
			locSet[l] = true
		}
	}

	if entries, err := os.ReadDir(absLocaleDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
				code := strings.TrimSuffix(e.Name(), ".json")
				locSet[code] = true
			}
		}
	}

	if len(locSet) == 0 {
		locSet["en"] = true
		locSet["es"] = true
		locSet["de"] = true
		locSet["fr"] = true
		locSet["ja"] = true
	}

	var orderedLocs []string
	if locSet["en"] {
		orderedLocs = append(orderedLocs, "en")
	}
	for loc := range locSet {
		if loc != "en" {
			orderedLocs = append(orderedLocs, loc)
		}
	}

	var importLines []string
	var resourceLines []string

	for _, loc := range orderedLocs {
		ident := sanitizeLocaleIdent(loc)
		jsonPath := fmt.Sprintf("%s/%s.json", relPathToLocales, loc)
		importLines = append(importLines, fmt.Sprintf("import %s from '%s';", ident, jsonPath))
		resourceLines = append(resourceLines, fmt.Sprintf("    '%s': { translation: %s },", loc, ident))
	}

	content := fmt.Sprintf(`import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';

// Automatically initialized by langPeanut for seamless react-i18next localization
%s

const resources = {
%s
};

if (!i18n.isInitialized) {
  i18n.use(initReactI18next).init({
    resources,
    lng: 'en',
    fallbackLng: 'en',
    interpolation: {
      escapeValue: false, // React already escapes values safely
    },
  });
}

export default i18n;
`, strings.Join(importLines, "\n"), strings.Join(resourceLines, "\n"))

	_ = os.WriteFile(targetAbs, []byte(content), 0644)

	// Inject import into root entry point
	InjectReactI18nRootImport(projectRoot, targetRel)

	return targetRel
}

func sanitizeLocaleIdent(loc string) string {
	cleaned := strings.ReplaceAll(loc, "-", "_")
	cleaned = strings.ReplaceAll(cleaned, "@", "_")
	return cleaned
}

// InjectReactI18nRootImport injects an import to the i18n bootstrap file in the application's root entry file
func InjectReactI18nRootImport(projectRoot, bootstrapRel string) bool {
	targetAbs := filepath.Join(projectRoot, bootstrapRel)

	// Check Next.js App Router (app/layout.tsx, app/layout.jsx, app/layout.js)
	appLayoutCandidates := []string{
		"app/layout.tsx",
		"app/layout.jsx",
		"app/layout.js",
		"src/app/layout.tsx",
		"src/app/layout.jsx",
		"src/app/layout.js",
	}

	for _, layoutRel := range appLayoutCandidates {
		layoutAbs := filepath.Join(projectRoot, layoutRel)
		if !FileExists(projectRoot, layoutRel) {
			continue
		}

		data, err := os.ReadFile(layoutAbs)
		if err != nil {
			continue
		}
		content := string(data)

		// Create client I18nProvider to avoid Server Component createContext errors
		providerRel := "components/I18nProvider.tsx"
		if strings.HasPrefix(layoutRel, "src/") || DirExists(projectRoot, "src/components") {
			providerRel = "src/components/I18nProvider.tsx"
		}
		providerAbs := filepath.Join(projectRoot, providerRel)
		_ = os.MkdirAll(filepath.Dir(providerAbs), 0755)

		relToI18n, _ := filepath.Rel(filepath.Dir(providerAbs), targetAbs)
		relToI18n = filepath.ToSlash(relToI18n)
		relToI18n = strings.TrimSuffix(relToI18n, ".tsx")
		relToI18n = strings.TrimSuffix(relToI18n, ".ts")
		relToI18n = strings.TrimSuffix(relToI18n, ".jsx")
		relToI18n = strings.TrimSuffix(relToI18n, ".js")
		if !strings.HasPrefix(relToI18n, ".") {
			relToI18n = "./" + relToI18n
		}

		providerCode := fmt.Sprintf(`'use client';

import React from 'react';
import '%s';

export default function I18nProvider({ children }: { children: React.ReactNode }) {
  return <>{children}</>;
}
`, relToI18n)
		_ = os.WriteFile(providerAbs, []byte(providerCode), 0644)

		// Inject provider into layout.tsx
		if !strings.Contains(content, "I18nProvider") {
			relToProvider, _ := filepath.Rel(filepath.Dir(layoutAbs), providerAbs)
			relToProvider = filepath.ToSlash(relToProvider)
			relToProvider = strings.TrimSuffix(relToProvider, ".tsx")
			relToProvider = strings.TrimSuffix(relToProvider, ".ts")
			relToProvider = strings.TrimSuffix(relToProvider, ".jsx")
			relToProvider = strings.TrimSuffix(relToProvider, ".js")
			if !strings.HasPrefix(relToProvider, ".") {
				relToProvider = "./" + relToProvider
			}

			importStmt := fmt.Sprintf("import I18nProvider from '%s';", relToProvider)
			updated := importStmt + "\n" + content

			if strings.Contains(updated, "{children}") {
				updated = strings.Replace(updated, "{children}", "<I18nProvider>\n          {children}\n        </I18nProvider>", 1)
			}

			_ = os.WriteFile(layoutAbs, []byte(updated), 0644)
			return true
		}
		return false
	}

	// For Pages Router, Vite, Create React App, and generic React
	entryCandidates := []string{
		"pages/_app.tsx",
		"pages/_app.jsx",
		"pages/_app.js",
		"src/pages/_app.tsx",
		"src/pages/_app.jsx",
		"src/pages/_app.js",
		"src/App.tsx",
		"src/App.jsx",
		"src/main.tsx",
		"src/main.jsx",
		"src/index.tsx",
		"src/index.jsx",
		"src/index.js",
		"App.tsx",
		"main.tsx",
		"index.tsx",
	}

	for _, entryRel := range entryCandidates {
		entryAbs := filepath.Join(projectRoot, entryRel)
		if !FileExists(projectRoot, entryRel) {
			continue
		}

		data, err := os.ReadFile(entryAbs)
		if err != nil {
			continue
		}
		content := string(data)

		// Check if already imports i18n
		if strings.Contains(content, "/i18n") || strings.Contains(content, "'./i18n") || strings.Contains(content, "\"./i18n") || strings.Contains(content, "'@/i18n") || strings.Contains(content, "\"@/i18n") {
			return false
		}

		rel, err := filepath.Rel(filepath.Dir(entryAbs), targetAbs)
		if err != nil {
			continue
		}
		rel = filepath.ToSlash(rel)
		rel = strings.TrimSuffix(rel, ".tsx")
		rel = strings.TrimSuffix(rel, ".ts")
		rel = strings.TrimSuffix(rel, ".jsx")
		rel = strings.TrimSuffix(rel, ".js")
		if !strings.HasPrefix(rel, ".") {
			rel = "./" + rel
		}

		importStmt := fmt.Sprintf("import '%s';", rel)

		var updated string
		trimmed := strings.TrimSpace(content)
		if strings.HasPrefix(trimmed, "'use client'") || strings.HasPrefix(trimmed, "\"use client\"") {
			// Place after use client
			idx := strings.Index(content, "\n")
			if idx != -1 {
				updated = content[:idx+1] + "\n" + importStmt + "\n" + content[idx+1:]
			} else {
				updated = content + "\n\n" + importStmt + "\n"
			}
		} else {
			updated = importStmt + "\n" + content
		}

		if err := os.WriteFile(entryAbs, []byte(updated), 0644); err == nil {
			return true
		}
	}
	return false
}

// executeNodeInstall detects pnpm/yarn/bun/npm and runs the appropriate add/install command
func executeNodeInstall(projectRoot string, packages []string) (string, string, error) {
	pkgManager := "npm"
	var args []string

	if FileExists(projectRoot, "pnpm-lock.yaml") {
		pkgManager = "pnpm"
		args = append([]string{"add"}, packages...)
	} else if FileExists(projectRoot, "yarn.lock") {
		pkgManager = "yarn"
		args = append([]string{"add"}, packages...)
	} else if FileExists(projectRoot, "bun.lockb") || FileExists(projectRoot, "bun.lock") {
		pkgManager = "bun"
		args = append([]string{"add"}, packages...)
	} else {
		pkgManager = "npm"
		args = append([]string{"install", "--save", "--no-audit", "--no-fund"}, packages...)
	}

	cmdLine := fmt.Sprintf("%s %s", pkgManager, strings.Join(args, " "))

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, pkgManager, args...)
	cmd.Dir = projectRoot
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf

	err := cmd.Run()
	outputStr := strings.TrimSpace(outBuf.String())
	if err != nil {
		return cmdLine, outputStr, err
	}
	return cmdLine, outputStr, nil
}

// FlutterCheckDependencies checks pubspec.yaml for flutter_localizations, intl, and flutter: generate: true
func FlutterCheckDependencies(projectRoot string) (*types.DependencyStatus, error) {
	status := &types.DependencyStatus{
		Framework:    types.FrameworkFlutter,
		ManifestFile: "pubspec.yaml",
		Success:      true,
	}

	pubspecPath := filepath.Join(projectRoot, "pubspec.yaml")
	data, err := os.ReadFile(pubspecPath)
	if err != nil {
		status.Success = false
		status.Message = "pubspec.yaml not found in project root"
		return status, nil
	}

	content := string(data)

	if !strings.Contains(content, "flutter_localizations:") {
		status.MissingDeps = append(status.MissingDeps, "flutter_localizations")
	}
	if !strings.Contains(content, "intl:") {
		status.MissingDeps = append(status.MissingDeps, "intl")
	}
	if !hasFlutterGenerateTrue(content) {
		status.MissingDeps = append(status.MissingDeps, "generate: true (flutter config)")
	}
	if !FileExists(projectRoot, "l10n.yaml") {
		status.MissingDeps = append(status.MissingDeps, "l10n.yaml")
	}

	if len(status.MissingDeps) == 0 {
		status.InstalledDeps = []string{"flutter_localizations", "intl", "generate: true", "l10n.yaml"}
		status.Message = "All Flutter localization dependencies and configs are in place"
	} else {
		status.Message = fmt.Sprintf("Missing Flutter dependencies/config: %s", strings.Join(status.MissingDeps, ", "))
	}

	return status, nil
}

func hasFlutterGenerateTrue(content string) bool {
	lines := strings.Split(content, "\n")
	inFlutter := false
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(l, "flutter:") {
			inFlutter = true
			continue
		}
		if inFlutter {
			if strings.HasPrefix(trimmed, "generate:") && strings.Contains(trimmed, "true") {
				return true
			}
			if len(l) > 0 && !strings.HasPrefix(l, " ") && !strings.HasPrefix(l, "\t") && !strings.HasPrefix(trimmed, "#") {
				inFlutter = false
			}
		}
	}
	return false
}

// FlutterEnsureDependencies injects flutter_localizations, intl, generate: true, l10n.yaml, and runs flutter pub get
func FlutterEnsureDependencies(projectRoot string, autoInstall bool) (*types.DependencyStatus, error) {
	return FlutterEnsureDependenciesWithCustom(projectRoot, autoInstall, "")
}

// FlutterEnsureDependenciesWithCustom injects flutter_localizations, intl, generate: true, l10n.yaml, and runs custom or auto-detected install command
func FlutterEnsureDependenciesWithCustom(projectRoot string, autoInstall bool, customCmd string) (*types.DependencyStatus, error) {
	status, err := FlutterCheckDependencies(projectRoot)
	if err != nil {
		return status, err
	}

	pubspecPath := filepath.Join(projectRoot, "pubspec.yaml")
	if FileExists(projectRoot, "pubspec.yaml") {
		data, err := os.ReadFile(pubspecPath)
		if err == nil {
			updatedContent, changed := patchPubspecYAML(string(data))
			if changed {
				_ = os.WriteFile(pubspecPath, []byte(updatedContent), 0644)
				status.ManifestUpdated = true
				status.InstalledDeps = append(status.InstalledDeps, "flutter_localizations", "intl")
			}
		}
	}

	// Ensure l10n.yaml exists
	if !FileExists(projectRoot, "l10n.yaml") {
		l10nContent := `arb-dir: lib/l10n
template-arb-file: app_en.arb
output-localization-file: app_localizations.dart
`
		_ = os.WriteFile(filepath.Join(projectRoot, "l10n.yaml"), []byte(l10nContent), 0644)
		status.ConfigCreated = append(status.ConfigCreated, "l10n.yaml")
	}

	// Ensure lib/l10n directory exists
	_ = os.MkdirAll(filepath.Join(projectRoot, "lib", "l10n"), 0755)

	if autoInstall {
		if strings.TrimSpace(customCmd) != "" {
			cmdStr, out, execErr := ExecuteCustomCommand(projectRoot, customCmd)
			status.CommandExecuted = cmdStr
			status.CommandOutput = out
			if execErr != nil {
				logger.Get().Warn("DEPENDENCIES", fmt.Sprintf("Custom install command '%s' returned notice/warning: %v", customCmd, execErr))
			}
		} else {
			cmdStr, out, execErr := executeFlutterPubGet(projectRoot)
			status.CommandExecuted = cmdStr
			status.CommandOutput = out
			if execErr != nil {
				logger.Get().Warn("DEPENDENCIES", fmt.Sprintf("flutter pub get returned notice: %v", execErr))
			}
		}
	}

	if len(status.MissingDeps) > 0 {
		status.Message = fmt.Sprintf("Configured Flutter localization: %s", strings.Join(status.MissingDeps, ", "))
	} else {
		status.Message = "Flutter localization dependencies are up to date"
	}
	status.Success = true

	return status, nil
}

func patchPubspecYAML(content string) (string, bool) {
	changed := false
	lines := strings.Split(content, "\n")
	var newLines []string

	hasLoc := strings.Contains(content, "flutter_localizations:")
	hasIntl := strings.Contains(content, "intl:")
	hasGen := hasFlutterGenerateTrue(content)

	if hasLoc && hasIntl && hasGen {
		return content, false
	}

	inDependencies := false
	dependenciesInjected := false
	inFlutter := false
	flutterFound := false

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(line, "dependencies:") {
			inDependencies = true
			newLines = append(newLines, line)
			continue
		}

		if inDependencies && !dependenciesInjected {
			// If we hit an unindented top-level key or end of dependencies
			if len(line) > 0 && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && !strings.HasPrefix(trimmed, "#") {
				// Inject before leaving dependencies
				if !hasLoc {
					newLines = append(newLines, "  flutter_localizations:\n    sdk: flutter")
					changed = true
				}
				if !hasIntl {
					newLines = append(newLines, "  intl: any")
					changed = true
				}
				dependenciesInjected = true
				inDependencies = false
			}
		}

		if strings.HasPrefix(line, "flutter:") {
			inFlutter = true
			flutterFound = true
			newLines = append(newLines, line)
			if !hasGen {
				newLines = append(newLines, "  generate: true")
				changed = true
			}
			continue
		}

		if inFlutter && len(line) > 0 && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && !strings.HasPrefix(trimmed, "#") {
			inFlutter = false
		}

		newLines = append(newLines, line)
	}

	// If dependencies was at the very end of file and never flushed
	if inDependencies && !dependenciesInjected {
		if !hasLoc {
			newLines = append(newLines, "  flutter_localizations:\n    sdk: flutter")
			changed = true
		}
		if !hasIntl {
			newLines = append(newLines, "  intl: any")
			changed = true
		}
	}

	// If flutter: block was completely missing
	if !flutterFound && !hasGen {
		newLines = append(newLines, "", "flutter:", "  generate: true")
		changed = true
	}

	return strings.Join(newLines, "\n"), changed
}

func executeFlutterPubGet(projectRoot string) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	cmdName := "flutter"
	args := []string{"pub", "get"}
	if _, err := exec.LookPath("flutter"); err != nil {
		if _, errDart := exec.LookPath("dart"); errDart == nil {
			cmdName = "dart"
			args = []string{"pub", "get"}
		} else {
			return "flutter pub get", "flutter toolchain not in PATH; pubspec.yaml updated directly", nil
		}
	}

	cmdLine := fmt.Sprintf("%s %s", cmdName, strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, cmdName, args...)
	cmd.Dir = projectRoot
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf

	err := cmd.Run()
	return cmdLine, strings.TrimSpace(outBuf.String()), err
}

// AndroidCheckDependencies checks Android Gradle & resource configurations
func AndroidCheckDependencies(projectRoot string) (*types.DependencyStatus, error) {
	status := &types.DependencyStatus{
		Framework:    types.FrameworkAndroid,
		ManifestFile: "app/src/main/res",
		Success:      true,
		Message:      "Android XML resources (values/strings.xml) are natively supported",
	}
	return status, nil
}

// AndroidEnsureDependencies ensures Android res/values structure exists
func AndroidEnsureDependencies(projectRoot string, autoInstall bool) (*types.DependencyStatus, error) {
	status, err := AndroidCheckDependencies(projectRoot)
	if err != nil {
		return status, err
	}
	_ = os.MkdirAll(filepath.Join(projectRoot, "app", "src", "main", "res", "values"), 0755)
	return status, nil
}

// SwiftCheckDependencies checks iOS/SwiftUI .xcstrings configuration
func SwiftCheckDependencies(projectRoot string) (*types.DependencyStatus, error) {
	status := &types.DependencyStatus{
		Framework:    types.FrameworkSwiftUI,
		ManifestFile: "Resources/Localizable.xcstrings",
		Success:      true,
		Message:      "Apple Foundation String Catalogs (.xcstrings) are natively supported",
	}
	return status, nil
}

// SwiftEnsureDependencies ensures Resources directory exists for Swift String Catalogs
func SwiftEnsureDependencies(projectRoot string, autoInstall bool) (*types.DependencyStatus, error) {
	status, err := SwiftCheckDependencies(projectRoot)
	if err != nil {
		return status, err
	}
	_ = os.MkdirAll(filepath.Join(projectRoot, "Resources"), 0755)
	return status, nil
}

// GenericCheckDependencies checks generic framework manifests (e.g. package.json, requirements.txt, go.mod)
func GenericCheckDependencies(projectRoot string) (*types.DependencyStatus, error) {
	status := &types.DependencyStatus{
		Framework: types.FrameworkGeneric,
		Success:   true,
	}

	if FileExists(projectRoot, "package.json") {
		return ReactCheckDependencies(projectRoot)
	}

	status.Message = "Generic localization JSON catalog ready"
	return status, nil
}

// GenericEnsureDependencies ensures generic framework dependencies
func GenericEnsureDependencies(projectRoot string, autoInstall bool) (*types.DependencyStatus, error) {
	return GenericEnsureDependenciesWithCustom(projectRoot, autoInstall, "")
}

// GenericEnsureDependenciesWithCustom ensures generic framework dependencies with optional custom install command
func GenericEnsureDependenciesWithCustom(projectRoot string, autoInstall bool, customCmd string) (*types.DependencyStatus, error) {
	if FileExists(projectRoot, "package.json") {
		return ReactEnsureDependenciesWithCustom(projectRoot, autoInstall, customCmd)
	}
	_ = os.MkdirAll(filepath.Join(projectRoot, "locales"), 0755)

	status := &types.DependencyStatus{
		Framework: types.FrameworkGeneric,
		Success:   true,
		Message:   "Generic localization directory ensured",
	}

	if autoInstall && strings.TrimSpace(customCmd) != "" {
		cmdStr, out, execErr := ExecuteCustomCommand(projectRoot, customCmd)
		status.CommandExecuted = cmdStr
		status.CommandOutput = out
		if execErr != nil {
			logger.Get().Warn("DEPENDENCIES", fmt.Sprintf("Custom install command '%s' returned notice/warning: %v", customCmd, execErr))
		}
	}

	return status, nil
}

