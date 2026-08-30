package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/langPeanut/langPeanut/pkg/platforms"
)

// DoctorIssue represents a diagnostic finding by the DoctorAgent
type DoctorIssue struct {
	Severity    string `json:"severity"` // "ERROR", "WARNING", "INFO"
	Category    string `json:"category"` // "dependency", "config", "locale_files", "hardcoded_strings"
	Title       string `json:"title"`
	Description string `json:"description"`
	CanAutoFix  bool   `json:"can_auto_fix"`
	AutoFixHint string `json:"auto_fix_hint"`
}

// DoctorReport contains the comprehensive project health audit
type DoctorReport struct {
	Framework           string        `json:"framework"`
	FrameworkDisplay    string        `json:"framework_display"`
	HealthScore         int           `json:"health_score"` // 0-100
	Issues              []DoctorIssue `json:"issues"`
	AutoFixableCount    int           `json:"auto_fixable_count"`
	HardcodedStringEst  int           `json:"hardcoded_strings_estimated"`
	ConfiguredLocales   []string      `json:"configured_locales"`
	MissingDependencies []string      `json:"missing_dependencies"`
	Status              string        `json:"status"` // "EXCELLENT", "GOOD", "NEEDS_SETUP", "CRITICAL"
}

// DoctorAgent diagnoses and auto-bootstraps i18n readiness across platforms
type DoctorAgent struct {
	Platform platforms.Platform
}

// NewDoctorAgent creates a new DoctorAgent instance
func NewDoctorAgent(p platforms.Platform) *DoctorAgent {
	return &DoctorAgent{Platform: p}
}

// DiagnoseProject performs a 360-degree health audit of the repository's localization setup
func (d *DoctorAgent) DiagnoseProject(projectRoot string) (*DoctorReport, error) {
	report := &DoctorReport{
		Framework:        string(d.Platform.Name()),
		FrameworkDisplay: d.Platform.DisplayName(),
		Issues:           make([]DoctorIssue, 0),
		HealthScore:      100,
		Status:           "EXCELLENT",
	}

	// 1. Check Framework Configuration & Dependencies
	switch d.Platform.Name() {
	case "react":
		d.auditReact(projectRoot, report)
	case "flutter":
		d.auditFlutter(projectRoot, report)
	case "swift":
		d.auditSwift(projectRoot, report)
	default:
		d.auditGeneric(projectRoot, report)
	}

	// 2. Check Locale Directories & Existing Dictionaries
	locDir := filepath.Join(projectRoot, d.Platform.DefaultLocaleDir(projectRoot))
	if _, err := os.Stat(locDir); os.IsNotExist(err) {
		report.HealthScore -= 20
		report.Issues = append(report.Issues, DoctorIssue{
			Severity:    "WARNING",
			Category:    "locale_files",
			Title:       "Missing Locale Directory",
			Description: fmt.Sprintf("Locale folder not found at '%s'", d.Platform.DefaultLocaleDir(projectRoot)),
			CanAutoFix:  true,
			AutoFixHint: "Scaffold directory and create source en.json template",
		})
	} else {
		// Discover existing locales
		entries, _ := os.ReadDir(locDir)
		for _, e := range entries {
			if !e.IsDir() && (strings.HasSuffix(e.Name(), ".json") || strings.HasSuffix(e.Name(), ".arb") || strings.HasSuffix(e.Name(), ".strings")) {
				locName := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(e.Name(), ".json"), ".arb"), ".strings")
				locName = strings.TrimPrefix(locName, "app_")
				report.ConfiguredLocales = append(report.ConfiguredLocales, locName)
			}
		}
	}

	// 3. Scan for Hardcoded Literals
	scout := NewASTScoutAgent(d.Platform)
	scanReport, err := scout.ScanProject(projectRoot, "")
	if err == nil && scanReport != nil {
		report.HardcodedStringEst = scanReport.LocalizableCount
		if scanReport.LocalizableCount > 0 {
			deduction := min(30, scanReport.LocalizableCount/5+5)
			report.HealthScore -= deduction
			report.Issues = append(report.Issues, DoctorIssue{
				Severity:    "INFO",
				Category:    "hardcoded_strings",
				Title:       fmt.Sprintf("%d Untranslated Hardcoded Strings Detected", scanReport.LocalizableCount),
				Description: fmt.Sprintf("Found %d raw string literals in UI templates across %d files.", scanReport.LocalizableCount, scanReport.TotalFilesScanned),
				CanAutoFix:  true,
				AutoFixHint: "Run `langpeanut run` to automatically extract, translate, and replace strings with AST queries.",
			})
		}
	}

	// Calculate counts & status
	for _, iss := range report.Issues {
		if iss.CanAutoFix {
			report.AutoFixableCount++
		}
	}

	if report.HealthScore < 0 {
		report.HealthScore = 0
	}

	if report.HealthScore >= 90 {
		report.Status = "EXCELLENT"
	} else if report.HealthScore >= 70 {
		report.Status = "GOOD"
	} else if report.HealthScore >= 45 {
		report.Status = "NEEDS_SETUP"
	} else {
		report.Status = "CRITICAL"
	}

	return report, nil
}

func (d *DoctorAgent) auditReact(projectRoot string, report *DoctorReport) {
	pkgPath := filepath.Join(projectRoot, "package.json")
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		report.HealthScore -= 25
		report.Issues = append(report.Issues, DoctorIssue{
			Severity:    "ERROR",
			Category:    "dependency",
			Title:       "package.json Not Found",
			Description: "Could not find package.json in project root.",
			CanAutoFix:  false,
		})
		return
	}

	content := string(data)
	hasI18nPkg := strings.Contains(content, "i18next") ||
		strings.Contains(content, "react-i18next") ||
		strings.Contains(content, "next-intl") ||
		strings.Contains(content, "next-i18next") ||
		strings.Contains(content, "@lingui")

	if !hasI18nPkg {
		report.HealthScore -= 25
		report.MissingDependencies = append(report.MissingDependencies, "i18next", "react-i18next")
		report.Issues = append(report.Issues, DoctorIssue{
			Severity:    "WARNING",
			Category:    "dependency",
			Title:       "i18n Runtime Package Missing",
			Description: "Neither react-i18next nor next-intl found in dependencies.",
			CanAutoFix:  true,
			AutoFixHint: "langpeanut auto-bootstraps i18next runtime and bootstrap file",
		})
	}
}

func (d *DoctorAgent) auditFlutter(projectRoot string, report *DoctorReport) {
	pubPath := filepath.Join(projectRoot, "pubspec.yaml")
	data, err := os.ReadFile(pubPath)
	if err != nil {
		report.HealthScore -= 25
		report.Issues = append(report.Issues, DoctorIssue{
			Severity:    "ERROR",
			Category:    "dependency",
			Title:       "pubspec.yaml Not Found",
			Description: "Could not find pubspec.yaml in project root.",
			CanAutoFix:  false,
		})
		return
	}

	content := string(data)
	if !strings.Contains(content, "flutter_localizations") {
		report.HealthScore -= 20
		report.MissingDependencies = append(report.MissingDependencies, "flutter_localizations")
		report.Issues = append(report.Issues, DoctorIssue{
			Severity:    "WARNING",
			Category:    "dependency",
			Title:       "flutter_localizations Missing in pubspec.yaml",
			Description: "Flutter internationalization SDK is not declared under dependencies.",
			CanAutoFix:  true,
			AutoFixHint: "Add flutter_localizations to pubspec.yaml",
		})
	}
}

func (d *DoctorAgent) auditSwift(projectRoot string, report *DoctorReport) {
	// Check for xcstrings or Localizable.strings
	var hasStringsFile bool
	_ = filepath.Walk(projectRoot, func(path string, info os.FileInfo, err error) error {
		if err == nil && (strings.HasSuffix(path, ".xcstrings") || strings.HasSuffix(path, "Localizable.strings")) {
			hasStringsFile = true
			return filepath.SkipDir
		}
		return nil
	})
	if !hasStringsFile {
		report.HealthScore -= 15
		report.Issues = append(report.Issues, DoctorIssue{
			Severity:    "INFO",
			Category:    "locale_files",
			Title:       "No Existing Localizable.xcstrings",
			Description: "Project does not contain a String Catalog (.xcstrings).",
			CanAutoFix:  true,
			AutoFixHint: "langpeanut will automatically generate Localizable.xcstrings during pipeline run.",
		})
	}
}

func (d *DoctorAgent) auditGeneric(projectRoot string, report *DoctorReport) {
	configPath := filepath.Join(projectRoot, "langPeanut.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		report.HealthScore -= 15
		report.Issues = append(report.Issues, DoctorIssue{
			Severity:    "INFO",
			Category:    "config",
			Title:       "langPeanut.yaml Config Missing",
			Description: "Project has not been initialized with `langpeanut init`.",
			CanAutoFix:  true,
			AutoFixHint: "Run `langpeanut init` to create default configuration.",
		})
	}
}

// AutoBootstrap resolves fixable issues automatically
func (d *DoctorAgent) AutoBootstrap(projectRoot string) ([]string, error) {
	var actionsTaken []string

	// 1. Scaffold locale directory
	locDir := filepath.Join(projectRoot, d.Platform.DefaultLocaleDir(projectRoot))
	if err := os.MkdirAll(locDir, 0755); err == nil {
		actionsTaken = append(actionsTaken, fmt.Sprintf("Scaffolded locale directory: %s", d.Platform.DefaultLocaleDir(projectRoot)))
	}

	// 2. If React, ensure bootstrap i18n.ts
	if d.Platform.Name() == "react" {
		platforms.EnsureReactI18nBootstrapWithLocales(projectRoot, nil)
		actionsTaken = append(actionsTaken, "Bootstrapped react-i18next runtime initialization")
	}

	// 3. Create default template if missing
	enPath := filepath.Join(locDir, "en.json")
	if _, err := os.Stat(enPath); os.IsNotExist(err) {
		_ = os.WriteFile(enPath, []byte("{\n  \"app.title\": \"My App\"\n}\n"), 0644)
		actionsTaken = append(actionsTaken, "Created template source locale: en.json")
	}

	return actionsTaken, nil
}
