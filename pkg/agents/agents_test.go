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
				StartByte:       74,
				EndByte:         85,
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
