package github

import (
	"strings"
	"testing"

	"github.com/langPeanut/langPeanut/pkg/agents"
	"github.com/langPeanut/langPeanut/pkg/types"
)

func TestBuildPullRequest_CleanSuccess(t *testing.T) {
	result := &agents.PipelineResult{
		ExtractedCandidates: 12,
		RefactoredFiles:     []string{"src/App.tsx", "src/Home.tsx", "src/About.tsx"},
		VerificationReport: &types.VerificationReport{
			Passed: true,
		},
	}
	meta := RunMetadata{
		Locales:          []string{"fr", "es", "de", "ja"},
		TonePreset:       "Professional",
		Provider:         "anthropic",
		Model:            "claude-sonnet-5",
		PromptTokens:     4200,
		CompletionTokens: 1800,
		EstimatedCostUSD: 0.0312,
	}

	title, body, labels := BuildPullRequest(result, meta)

	wantTitle := "i18n: localize 12 string(s) across 3 file(s) (fr, es, de, ja)"
	if title != wantTitle {
		t.Errorf("title = %q, want %q", title, wantTitle)
	}
	if len(labels) != 1 || labels[0] != LabelAutomation {
		t.Errorf("labels = %v, want [%s]", labels, LabelAutomation)
	}
	if strings.Contains(body, "Needs manual review") {
		t.Errorf("body should not contain review section on clean success:\n%s", body)
	}
	if !strings.Contains(body, "all checks passed") {
		t.Errorf("body missing verification pass line:\n%s", body)
	}
	if !strings.Contains(body, "claude-sonnet-5") {
		t.Errorf("body missing model info:\n%s", body)
	}
}

func TestBuildPullRequest_NeedsReview(t *testing.T) {
	result := &agents.PipelineResult{
		ExtractedCandidates: 5,
		RefactoredFiles:     []string{"src/App.tsx"},
		VerificationReport: &types.VerificationReport{
			Passed:     false,
			ErrorCount: 1,
		},
		CodeRepairs: []types.CodeRepairResult{
			{FilePath: "src/App.tsx", Repaired: false, Attempts: 2},
		},
		UnresolvedErrors: []types.CompilerDiagnostic{
			{FilePath: "src/App.tsx", Line: 42, Message: "Cannot find name 'useTranslation'", Source: "tsc"},
		},
	}
	meta := RunMetadata{Locales: []string{"fr"}}

	title, body, labels := BuildPullRequest(result, meta)

	if !strings.Contains(title, "1 file(s) need review") {
		t.Errorf("title should flag review count, got: %q", title)
	}
	if len(labels) != 2 || labels[1] != LabelNeedsReview {
		t.Errorf("labels = %v, want [%s %s]", labels, LabelAutomation, LabelNeedsReview)
	}
	if !strings.Contains(body, "## ⚠️ Needs manual review") {
		t.Errorf("body missing needs-review section:\n%s", body)
	}
	if !strings.Contains(body, "src/App.tsx:42") {
		t.Errorf("body missing diagnostic location:\n%s", body)
	}
	if !strings.Contains(body, "Cannot find name 'useTranslation'") {
		t.Errorf("body missing diagnostic message:\n%s", body)
	}
}

func TestBuildPullRequest_NilResult(t *testing.T) {
	title, body, labels := BuildPullRequest(nil, RunMetadata{})
	if title != "" || body != "" || labels != nil {
		t.Errorf("expected empty output for nil result, got title=%q body=%q labels=%v", title, body, labels)
	}
}

func TestBuildPullRequest_HealedRepairsReported(t *testing.T) {
	result := &agents.PipelineResult{
		ExtractedCandidates: 3,
		RefactoredFiles:     []string{"lib/main.dart"},
		VerificationReport:  &types.VerificationReport{Passed: true},
		CodeRepairs: []types.CodeRepairResult{
			{FilePath: "lib/main.dart", Repaired: true, Attempts: 1},
		},
	}
	meta := RunMetadata{Locales: []string{"de"}}

	_, body, labels := BuildPullRequest(result, meta)

	if len(labels) != 1 {
		t.Errorf("healed repairs should not trigger needs-review label, got %v", labels)
	}
	if !strings.Contains(body, "Auto-healed 1 compiler issue(s) in: lib/main.dart") {
		t.Errorf("body missing auto-heal line:\n%s", body)
	}
}
