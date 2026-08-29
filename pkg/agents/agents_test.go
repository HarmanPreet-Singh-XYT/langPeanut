package agents

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/langPeanut/langPeanut/pkg/memory"
	"github.com/langPeanut/langPeanut/pkg/platforms"
	"github.com/langPeanut/langPeanut/pkg/types"
)

func TestPatchEngine_ApplyRefactorPlan(t *testing.T) {
	pe := NewPatchEngine()

	original := `import React from 'react';

export const TestComponent = () => {
  return <h1>Hello World</h1>;
};
`
	plan := &types.FileRefactorPlan{
		FilePath:        "test.tsx",
		OriginalContent: original,
		RequiredImports: []string{"import { useTranslation } from 'react-i18next';"},
		Patches: []types.ByteRangePatch{
			{
				FilePath:        "test.tsx",
				StartByte:       78,
				EndByte:         89,
				ReplacementText: "{t('helloWorld')}",
			},
		},
	}

	result, err := pe.ApplyRefactorPlan(plan)
	if err != nil {
		t.Fatalf("ApplyRefactorPlan failed: %v", err)
	}

	if result == "" {
		t.Fatalf("expected refactored content, got empty string")
	}
}

func TestVerifierCritic_ICUParity(t *testing.T) {
	critic := NewVerifierCriticAgent()

	sourceLocale := types.LocaleData{
		LocaleCode: "en",
		Entries: map[string]string{
			"welcome": "Welcome back, {name}!",
		},
	}

	// Missing variable in translation
	targetLocales := map[string]types.LocaleData{
		"fr": {
			LocaleCode: "fr",
			Entries: map[string]string{
				"welcome": "Bienvenue !", // missing {name}
			},
		},
	}

	report := critic.VerifyAll(sourceLocale, targetLocales, nil)
	if report.Passed {
		t.Fatalf("expected critic to fail due to missing {name} variable, but report passed")
	}

	if report.ErrorCount == 0 {
		t.Fatalf("expected at least 1 error diagnostic, got 0")
	}
}

func TestContextAgent_Disambiguation(t *testing.T) {
	ca := NewContextAgent()

	candidates := []types.StringCandidate{
		{
			ID:             "1",
			FilePath:       "FlightCard.tsx",
			CleanValue:     "Book",
			ContextHint:    "FlightBookingPanel -> Button",
			Classification: types.ClassLocalizable,
			Approved:       true,
		},
	}

	results, err := ca.DisambiguateAndEnhance(candidates)
	if err != nil {
		t.Fatalf("DisambiguateAndEnhance failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatalf("expected at least 1 result")
	}

	if results[0].Key != "reserveFlight" {
		t.Logf("synthesized key: %s", results[0].Key)
	}
}

func TestContextAgent_TagProfilingAndFiltering(t *testing.T) {
	ca := NewContextAgent()

	candidates := []types.StringCandidate{
		{
			ID:             "1",
			FilePath:       "IssueForm.tsx",
			ParentNodeType: "JSXAttribute(submitLabel)",
			RawValue:       "submitLabel=\"Open Bug Report on GitHub\"",
			CleanValue:     "Open Bug Report on GitHub",
			Classification: types.ClassLocalizable,
			Approved:       true,
		},
		{
			ID:             "2",
			FilePath:       "Table.tsx",
			ParentNodeType: "JSXAttribute(key)",
			RawValue:       "key: `${key}`",
			CleanValue:     "key: ${key}",
			Classification: types.ClassLocalizable,
			Approved:       true,
		},
		{
			ID:             "3",
			FilePath:       "Chart.tsx",
			ParentNodeType: "JSXAttribute(d)",
			RawValue:       "M 0 0 L 100 100 Z",
			CleanValue:     "M 0 0 L 100 100 Z",
			Classification: types.ClassLocalizable,
			Approved:       true,
		},
		{
			ID:             "4",
			FilePath:       "Terminal.tsx",
			ParentNodeType: "JSXElement",
			RawValue:       "git clone https://github.com/repo.git",
			CleanValue:     "git clone https://github.com/repo.git",
			Classification: types.ClassLocalizable,
			Approved:       true,
		},
	}

	results, err := ca.DisambiguateAndEnhance(candidates)
	if err != nil {
		t.Fatalf("DisambiguateAndEnhance failed: %v", err)
	}

	// Candidate 1 (submitLabel UI string) should remain LOCALIZABLE
	if results[0].Classification != types.ClassLocalizable {
		t.Errorf("expected candidate 1 to be LOCALIZABLE, got %v", results[0].Classification)
	}

	// Candidate 2 (`key: ${key}`) must be filtered to SKIP
	if results[1].Classification != types.ClassSkip {
		t.Errorf("expected candidate 2 (key: ${key}) to be SKIP, got %v", results[1].Classification)
	}

	// Candidate 3 (SVG path) must be filtered to SKIP
	if results[2].Classification != types.ClassSkip {
		t.Errorf("expected candidate 3 (SVG path) to be SKIP, got %v", results[2].Classification)
	}

	// Candidate 4 (CLI command) must be filtered to SKIP
	if results[3].Classification != types.ClassSkip {
		t.Errorf("expected candidate 4 (CLI command) to be SKIP, got %v", results[3].Classification)
	}
}

func TestSupervisorAgent_EndToEnd(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "langpeanut-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	sampleFile := filepath.Join(tmpDir, "App.tsx")
	content := `import React from 'react';

export const App = () => {
  return <div><h1>Welcome back, {name}!</h1><button>Submit Order</button></div>;
};
`
	_ = os.WriteFile(sampleFile, []byte(content), 0644)

	reg := platforms.NewRegistry()
	p, _ := reg.Get(types.FrameworkReact)

	sup, err := NewSupervisorAgent(tmpDir, p)
	if err != nil {
		t.Fatalf("NewSupervisorAgent failed: %v", err)
	}

	res, err := sup.RunEndToEnd(context.Background(), "en", []string{"fr", "es"}, false)
	if err != nil {
		t.Fatalf("RunEndToEnd failed: %v", err)
	}

	if res.ExtractedCandidates == 0 {
		t.Fatalf("expected candidates extracted, got 0")
	}

	if res.VerificationReport == nil || !res.VerificationReport.Passed {
		t.Fatalf("expected verification report to pass")
	}
}

func TestStyleMemory_GenZ(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "pm-test-*")
	defer os.RemoveAll(tmpDir)

	pm, err := memory.NewProjectMemory(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	pm.Style = memory.StyleGenZ
	instruction := pm.GetStyleInstruction()
	if instruction == "" {
		t.Fatalf("expected non-empty Gen-Z style instruction")
	}
}

func TestPingRoute_Supervisor(t *testing.T) {
	targetDir := "/Users/harmanpreetsingh/Public/Code/pingroute-web"
	if _, err := os.Stat(targetDir); err != nil {
		t.Skip("pingroute-web not found")
	}

	reg := platforms.NewRegistry()
	p, _ := reg.AutoDetect(targetDir)

	sup, err := NewSupervisorAgent(targetDir, p)
	if err != nil {
		t.Fatalf("NewSupervisorAgent failed: %v", err)
	}

	report, err := sup.Scout.ScanProject(targetDir, "")
	if err != nil {
		t.Fatalf("ScanProject failed: %v", err)
	}
	t.Logf("Total scanned files: %d, Total candidates: %d", report.TotalFilesScanned, report.TotalCandidates)

	enhanced, err := sup.Context.DisambiguateAndEnhance(report.Candidates)
	if err != nil {
		t.Fatalf("DisambiguateAndEnhance failed: %v", err)
	}

	localizableCount := 0
	sourceEntries := make(map[string]string)
	for _, c := range enhanced {
		if c.Classification == types.ClassLocalizable && c.Approved {
			localizableCount++
			sourceEntries[c.Key] = c.CleanValue
		}
	}
	t.Logf("Total localizable & approved: %d, Unique keys in sourceEntries: %d", localizableCount, len(sourceEntries))
}

func TestTranslator_DynamicWordBudgetChunking(t *testing.T) {
	// Sample dataset with mix of short buttons and long paragraph descriptions
	entries := map[string]string{
		"k1": "Save",
		"k2": "Cancel",
		"k3": "Delete",
		"k4": "Submit Order",
		"k5": "This is a longer paragraph describing the complete feature set of our modern network diagnostic telemetry suite with real-time route tracing and hop inspection charts.", // 24 words
		"k6": "Another detailed explanation paragraph with lots of instructions for users configuring custom DNS resolvers, latency thresholds, packet loss alerts, and gateway endpoints.", // 23 words
		"k7": "Short text",
	}

	// Chunk with a small word budget of 30 words and max 5 keys
	chunks := chunkMapByWordBudget(entries, 30, 5)

	if len(chunks) < 2 {
		t.Fatalf("Expected at least 2 chunks due to word budget constraints, got %d", len(chunks))
	}

	// Verify all keys are preserved across chunks
	totalKeys := 0
	for _, chunk := range chunks {
		totalKeys += len(chunk)
	}
	if totalKeys != len(entries) {
		t.Fatalf("Expected all %d keys preserved, got %d", len(entries), totalKeys)
	}
}
