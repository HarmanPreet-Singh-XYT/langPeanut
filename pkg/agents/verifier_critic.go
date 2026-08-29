package agents

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/langPeanut/langPeanut/pkg/types"
)

// VerifierCriticAgent implements the 4-Tier Automated Verification and Self-Correction Critic
type VerifierCriticAgent struct{}

func NewVerifierCriticAgent() *VerifierCriticAgent {
	return &VerifierCriticAgent{}
}

// VerifyAll executes all 4 verification tiers across the source and translated locale data
func (vc *VerifierCriticAgent) VerifyAll(sourceLocale types.LocaleData, targetLocales map[string]types.LocaleData, refactoredPlans map[string]types.FileRefactorPlan) *types.VerificationReport {
	report := &types.VerificationReport{
		Passed: true,
	}

	// Tier 1: AST & Refactor Plan Syntax Verification
	vc.verifyTier1Syntax(refactoredPlans, targetLocales, report)

	// Tier 2: ICU & Variable Token Alignment Parity
	vc.verifyTier2ICUTokens(sourceLocale, targetLocales, report)

	// Tier 3: UI Linguistic Character Expansion & Overflow Risk
	vc.verifyTier3UIExpansion(sourceLocale, targetLocales, report)

	// Tier 4: Cross-Locale Key Parity & Orphan Prevention
	vc.verifyTier4LocaleParity(sourceLocale, targetLocales, report)

	// Determine final pass/fail
	for _, d := range report.Diagnostics {
		if d.Severity == "ERROR" {
			report.ErrorCount++
			report.Passed = false
		} else if d.Severity == "WARNING" {
			report.WarnCount++
		}
	}

	return report
}

// Tier 1: Validates syntax of refactored code and locale serialized files
func (vc *VerifierCriticAgent) verifyTier1Syntax(plans map[string]types.FileRefactorPlan, targetLocales map[string]types.LocaleData, report *types.VerificationReport) {
	pe := NewPatchEngine()

	for filePath, plan := range plans {
		if plan.RefactoredContent != "" {
			if err := pe.ValidateSyntax(plan.RefactoredContent, filePath); err != nil {
				report.Diagnostics = append(report.Diagnostics, types.Diagnostic{
					Tier:        types.Tier1SyntaxAST,
					Severity:    "ERROR",
					Message:     fmt.Sprintf("Syntax error in refactored file %s: %v", filePath, err),
					CanAutoFix:  true,
					AutoFixHint: "Re-run patch engine with balanced AST ranges",
				})
			}
		}
	}

	// Verify JSON/ARB serializability
	for loc, data := range targetLocales {
		if _, err := json.Marshal(data.Entries); err != nil {
			report.Diagnostics = append(report.Diagnostics, types.Diagnostic{
				Tier:       types.Tier1SyntaxAST,
				Severity:   "ERROR",
				Locale:     loc,
				Message:    fmt.Sprintf("Locale file %s failed JSON serialization: %v", loc, err),
				CanAutoFix: false,
			})
		}
	}
}

// Tier 2: Ensures all variable placeholders in source exist identically in target translations
func (vc *VerifierCriticAgent) verifyTier2ICUTokens(sourceLocale types.LocaleData, targetLocales map[string]types.LocaleData, report *types.VerificationReport) {
	for key, sourceText := range sourceLocale.Entries {
		sourcePlaceholders := extractAllPlaceholders(sourceText)
		if len(sourcePlaceholders) == 0 {
			continue
		}

		sort.Strings(sourcePlaceholders)

		for targetCode, targetData := range targetLocales {
			targetText, ok := targetData.Entries[key]
			if !ok {
				continue // handled in Tier 4
			}

			targetPlaceholders := extractAllPlaceholders(targetText)
			sort.Strings(targetPlaceholders)

			// Check for missing or corrupted placeholders
			sourceMap := make(map[string]bool)
			for _, ph := range sourcePlaceholders {
				sourceMap[ph] = true
			}

			for _, ph := range targetPlaceholders {
				delete(sourceMap, ph)
			}

			if len(sourceMap) > 0 {
				var missing []string
				for ph := range sourceMap {
					missing = append(missing, ph)
				}

				report.Diagnostics = append(report.Diagnostics, types.Diagnostic{
					Tier:        types.Tier2ICUTokens,
					Severity:    "ERROR",
					Key:         key,
					Locale:      targetCode,
					Expected:    strings.Join(sourcePlaceholders, ", "),
					Actual:      strings.Join(targetPlaceholders, ", "),
					Message:     fmt.Sprintf("Locale '%s' for key '%s' omitted variable placeholder(s): %s", targetCode, key, strings.Join(missing, ", ")),
					CanAutoFix:  true,
					AutoFixHint: fmt.Sprintf("Include exact placeholder(s) %s without translating them", strings.Join(missing, ", ")),
				})
			}
		}
	}
}

// Tier 3: Evaluates character expansion ratios and warns on potential UI layout overflow
func (vc *VerifierCriticAgent) verifyTier3UIExpansion(sourceLocale types.LocaleData, targetLocales map[string]types.LocaleData, report *types.VerificationReport) {
	for key, sourceText := range sourceLocale.Entries {
		srcLen := len(sourceText)
		if srcLen < 5 {
			continue
		}

		for targetCode, targetData := range targetLocales {
			targetText := targetData.Entries[key]
			tgtLen := len(targetText)

			ratio := float64(tgtLen) / float64(srcLen)

			// Expansion warning thresholds: German/Russian can be 1.4x, but > 2.5x indicates potential run-away generation
			if ratio > 2.5 {
				report.Diagnostics = append(report.Diagnostics, types.Diagnostic{
					Tier:        types.Tier3UIExpansion,
					Severity:    "WARNING",
					Key:         key,
					Locale:      targetCode,
					Expected:    fmt.Sprintf("Length <= %d chars", int(float64(srcLen)*2.0)),
					Actual:      fmt.Sprintf("%d chars (%.1fx expansion)", tgtLen, ratio),
					Message:     fmt.Sprintf("Locale '%s' key '%s' has extreme character expansion (%.1fx). May clip in fixed-width UI components.", targetCode, key, ratio),
					CanAutoFix:  false,
					AutoFixHint: "Consider concise phrasing or testing on small screen sizes",
				})
			}
		}
	}
}

// Tier 4: Validates complete 1:1 key parity across all locale files
func (vc *VerifierCriticAgent) verifyTier4LocaleParity(sourceLocale types.LocaleData, targetLocales map[string]types.LocaleData, report *types.VerificationReport) {
	for targetCode, targetData := range targetLocales {
		// Check for missing keys
		for srcKey := range sourceLocale.Entries {
			if _, exists := targetData.Entries[srcKey]; !exists {
				report.Diagnostics = append(report.Diagnostics, types.Diagnostic{
					Tier:        types.Tier4LocaleParity,
					Severity:    "ERROR",
					Key:         srcKey,
					Locale:      targetCode,
					Message:     fmt.Sprintf("Key '%s' is missing in target locale '%s'", srcKey, targetCode),
					CanAutoFix:  true,
					AutoFixHint: "Translate and append missing key to target locale",
				})
			}
		}

		// Check for orphaned keys
		for tgtKey := range targetData.Entries {
			if _, exists := sourceLocale.Entries[tgtKey]; !exists {
				report.Diagnostics = append(report.Diagnostics, types.Diagnostic{
					Tier:        types.Tier4LocaleParity,
					Severity:    "WARNING",
					Key:         tgtKey,
					Locale:      targetCode,
					Message:     fmt.Sprintf("Orphaned key '%s' in locale '%s' not present in source locale", tgtKey, targetCode),
					CanAutoFix:  true,
					AutoFixHint: "Remove orphaned key if no longer referenced in source",
				})
			}
		}
	}
}
