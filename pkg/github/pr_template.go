// Package github contains the GitHub-integration layer for langPeanut Cloud:
// deterministic PR formatting, App authentication, repo push, and PR creation.
package github

import (
	"fmt"
	"sort"
	"strings"

	"github.com/langPeanut/langPeanut/pkg/agents"
	"github.com/langPeanut/langPeanut/pkg/types"
)

// LabelAutomation and LabelNeedsReview are applied to every langPeanut-opened PR.
const (
	LabelAutomation  = "i18n-automation"
	LabelNeedsReview = "needs-manual-review"
)

// RunMetadata carries run-level context that PipelineResult itself doesn't track
// (locales requested, tone, and which provider/model + token spend were used).
type RunMetadata struct {
	Locales          []string
	TonePreset       string
	Provider         string
	Model            string
	PromptTokens     int64
	CompletionTokens int64
	EstimatedCostUSD float64
}

// BuildPullRequest deterministically formats a PR title, body, and label set from
// a completed pipeline run. It never calls an LLM: title and body are built purely
// from PipelineResult + RunMetadata fields, matching the project's "Zero-Generation
// Principle" (see README.md #4) — the LLM is for translation decisions, not for
// generating its own PR description.
//
// The PR is always meant to be opened, whether or not repairs fully succeeded;
// this function only signals the failure state via body content and labels.
func BuildPullRequest(result *agents.PipelineResult, meta RunMetadata) (title, body string, labels []string) {
	if result == nil {
		return "", "", nil
	}

	needsReview := len(result.UnresolvedErrors) > 0
	fileCount := len(result.RefactoredFiles)
	localeList := strings.Join(meta.Locales, ", ")

	if result.ExtractedCandidates > 0 {
		title = fmt.Sprintf("i18n: localize %d string(s) across %d file(s) (%s)",
			result.ExtractedCandidates, fileCount, localeList)
	} else if len(result.TargetLocaleFiles) > 0 || len(result.Translations) > 0 {
		title = fmt.Sprintf("i18n: sync translation matrix & locale catalogs (%s)", localeList)
	} else {
		title = fmt.Sprintf("i18n: automated localization check (%s)", localeList)
	}
	if needsReview {
		unresolvedFiles := countUnresolvedFiles(result.UnresolvedErrors)
		title += fmt.Sprintf(" — %d file(s) need review", unresolvedFiles)
	}

	labels = []string{LabelAutomation}
	if needsReview {
		labels = append(labels, LabelNeedsReview)
	}

	body = buildBody(result, meta, needsReview)
	return title, body, labels
}

func buildBody(result *agents.PipelineResult, meta RunMetadata, needsReview bool) string {
	var b strings.Builder

	if needsReview {
		writeNeedsReviewSection(&b, result.UnresolvedErrors)
	}

	b.WriteString("## Summary\n")
	fmt.Fprintf(&b, "- **Files changed:** %d\n", len(result.RefactoredFiles))
	fmt.Fprintf(&b, "- **Strings localized:** %d\n", result.ExtractedCandidates)
	fmt.Fprintf(&b, "- **Languages:** %s\n", strings.Join(meta.Locales, ", "))
	if meta.TonePreset != "" {
		fmt.Fprintf(&b, "- **Tone/style:** %s\n", meta.TonePreset)
	}
	if meta.Provider != "" || meta.Model != "" {
		fmt.Fprintf(&b, "- **Provider/model:** %s/%s\n", meta.Provider, meta.Model)
	}
	if meta.PromptTokens > 0 || meta.CompletionTokens > 0 {
		fmt.Fprintf(&b, "- **Tokens used:** %d in / %d out (~$%.4f)\n",
			meta.PromptTokens, meta.CompletionTokens, meta.EstimatedCostUSD)
	}
	b.WriteString("\n")

	if len(result.RefactoredFiles) > 0 {
		b.WriteString("## Files touched\n")
		files := append([]string(nil), result.RefactoredFiles...)
		sort.Strings(files)
		for _, f := range files {
			fmt.Fprintf(&b, "- `%s`\n", f)
		}
		b.WriteString("\n")
	}

	writeVerificationSection(&b, result)

	return strings.TrimRight(b.String(), "\n") + "\n"
}

func writeVerificationSection(b *strings.Builder, result *agents.PipelineResult) {
	b.WriteString("## Verification\n")

	if result.VerificationReport != nil {
		if result.VerificationReport.Passed {
			b.WriteString("- [Passed] 4-Tier Critic: all checks passed\n")
		} else {
			fmt.Fprintf(b, "- [Warning] 4-Tier Critic: %d error(s), %d warning(s)\n",
				result.VerificationReport.ErrorCount, result.VerificationReport.WarnCount)
		}
	}

	if len(result.CodeRepairs) > 0 {
		healed := 0
		var healedFiles []string
		for _, r := range result.CodeRepairs {
			if r.Repaired {
				healed++
				healedFiles = append(healedFiles, r.FilePath)
			}
		}
		if healed > 0 {
			fmt.Fprintf(b, "- [Auto-Repair] Auto-healed %d compiler issue(s) in: %s\n",
				healed, strings.Join(healedFiles, ", "))
		}
	}
	b.WriteString("\n")
}

func writeNeedsReviewSection(b *strings.Builder, unresolved []types.CompilerDiagnostic) {
	b.WriteString("## Needs manual review\n")
	b.WriteString("The automated code-repair agent could not resolve the following issue(s) introduced by this change:\n\n")
	for _, d := range unresolved {
		fmt.Fprintf(b, "- `%s:%d` — %s (%s)\n", d.FilePath, d.Line, d.Message, d.Source)
	}
	b.WriteString("\nPlease review and fix before merging.\n\n")
}

func countUnresolvedFiles(unresolved []types.CompilerDiagnostic) int {
	seen := make(map[string]struct{})
	for _, d := range unresolved {
		seen[d.FilePath] = struct{}{}
	}
	return len(seen)
}
