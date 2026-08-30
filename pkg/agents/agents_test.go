package agents

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/langPeanut/langPeanut/pkg/llm"
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

	if !strings.Contains(result, "'use client'") {
		t.Fatalf("expected 'use client' directive injected for React TSX component, got:\n%s", result)
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

// TestSupervisorAgent_PreservesExistingTranslations covers running langPeanut
// against a project that already has i18n set up (e.g. a Flutter app with
// lib/l10n/app_{en,de}.arb already translated). Only the newly added source
// string should be translated; the pre-existing German translation must
// survive untouched rather than being re-translated and overwritten.
func TestSupervisorAgent_PreservesExistingTranslations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "langpeanut-existing-i18n-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	l10nDir := filepath.Join(tmpDir, "lib", "l10n")
	if err := os.MkdirAll(l10nDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Pre-existing, already-translated catalogs (source + one target locale).
	const existingGerman = "Guten Tag"
	if err := os.WriteFile(filepath.Join(l10nDir, "app_en.arb"),
		[]byte(`{"@@locale": "en", "greeting": "Hello"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(l10nDir, "app_de.arb"),
		[]byte(`{"@@locale": "de", "greeting": "`+existingGerman+`"}`), 0644); err != nil {
		t.Fatal(err)
	}

	// New source file introduces a string that isn't in either catalog yet.
	dartFile := filepath.Join(tmpDir, "lib", "main.dart")
	if err := os.MkdirAll(filepath.Dir(dartFile), 0755); err != nil {
		t.Fatal(err)
	}
	dartContent := `import 'package:flutter/material.dart';
Widget build(BuildContext context) => const Text("Submit Order");
`
	if err := os.WriteFile(dartFile, []byte(dartContent), 0644); err != nil {
		t.Fatal(err)
	}

	reg := platforms.NewRegistry()
	p, _ := reg.Get(types.FrameworkFlutter)

	sup, err := NewSupervisorAgent(tmpDir, p)
	if err != nil {
		t.Fatalf("NewSupervisorAgent failed: %v", err)
	}

	res, err := sup.RunEndToEnd(context.Background(), "en", []string{"de"}, false)
	if err != nil {
		t.Fatalf("RunEndToEnd failed: %v", err)
	}
	if res.VerificationReport == nil || !res.VerificationReport.Passed {
		t.Fatalf("expected verification report to pass, got %+v", res.VerificationReport)
	}

	// The existing German translation for "greeting" must be untouched.
	deData, err := os.ReadFile(filepath.Join(l10nDir, "app_de.arb"))
	if err != nil {
		t.Fatalf("failed to read app_de.arb: %v", err)
	}
	locData, err := p.ParseLocaleFileForLocale(deData, "de")
	if err != nil {
		t.Fatalf("ParseLocaleFileForLocale failed: %v", err)
	}
	if locData.Entries["greeting"] != existingGerman {
		t.Errorf("expected pre-existing translation %q preserved, got %q", existingGerman, locData.Entries["greeting"])
	}

	// The newly extracted key must now be present (translated) in the German catalog.
	found := false
	for key, val := range locData.Entries {
		if key != "greeting" && val != "" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected newly extracted string to be translated into German catalog, got entries: %v", locData.Entries)
	}
}

func TestSupervisorAgent_ReplaceExistingTranslations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "langpeanut-replace-i18n-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	l10nDir := filepath.Join(tmpDir, "lib", "l10n")
	if err := os.MkdirAll(l10nDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Catalog with an outdated custom placeholder that user wants regenerated
	const staleGerman = "OLD_OUTDATED_TRANSLATION"
	if err := os.WriteFile(filepath.Join(l10nDir, "app_en.arb"),
		[]byte(`{"@@locale": "en", "greeting": "Hello"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(l10nDir, "app_de.arb"),
		[]byte(`{"@@locale": "de", "greeting": "`+staleGerman+`"}`), 0644); err != nil {
		t.Fatal(err)
	}

	dartFile := filepath.Join(tmpDir, "lib", "main.dart")
	if err := os.MkdirAll(filepath.Dir(dartFile), 0755); err != nil {
		t.Fatal(err)
	}
	dartContent := `import 'package:flutter/material.dart';
Widget build(BuildContext context) => const Text("Hello");
`
	if err := os.WriteFile(dartFile, []byte(dartContent), 0644); err != nil {
		t.Fatal(err)
	}

	reg := platforms.NewRegistry()
	p, _ := reg.Get(types.FrameworkFlutter)

	sup, err := NewSupervisorAgent(tmpDir, p)
	if err != nil {
		t.Fatalf("NewSupervisorAgent failed: %v", err)
	}
	sup.ExistingMode = "replace"

	res, err := sup.RunEndToEnd(context.Background(), "en", []string{"de"}, false)
	if err != nil {
		t.Fatalf("RunEndToEnd failed: %v", err)
	}
	if res.VerificationReport == nil || !res.VerificationReport.Passed {
		t.Fatalf("expected verification report to pass, got %+v", res.VerificationReport)
	}

	// In replace mode, the stale translation must be replaced by new translation
	deData, err := os.ReadFile(filepath.Join(l10nDir, "app_de.arb"))
	if err != nil {
		t.Fatalf("failed to read app_de.arb: %v", err)
	}
	locData, err := p.ParseLocaleFileForLocale(deData, "de")
	if err != nil {
		t.Fatalf("ParseLocaleFileForLocale failed: %v", err)
	}
	if locData.Entries["greeting"] == staleGerman {
		t.Errorf("expected stale translation %q to be replaced, but it remained", staleGerman)
	}
	if locData.Entries["greeting"] == "" {
		t.Errorf("expected greeting to have a translated value, got empty string")
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

	// Group candidates by file and test GenerateRefactorPlan + ApplyRefactorPlan
	byFile := make(map[string][]types.StringCandidate)
	for _, c := range enhanced {
		if c.Classification == types.ClassLocalizable && c.Approved {
			byFile[c.FilePath] = append(byFile[c.FilePath], c)
		}
	}

	for fPath, fileCands := range byFile {
		src, err := os.ReadFile(fPath)
		if err != nil {
			continue
		}
		plan, err := p.GenerateRefactorPlan(fPath, src, fileCands)
		if err != nil {
			t.Errorf("GenerateRefactorPlan failed on %s: %v", fPath, err)
			continue
		}
		pe := NewPatchEngine()
		refactored, err := pe.ApplyRefactorPlan(plan)
		if err != nil {
			t.Errorf("ApplyRefactorPlan failed on %s: %v", fPath, err)
			t.Logf("Plan patches count: %d", len(plan.Patches))
			for idx, pt := range plan.Patches {
				t.Logf("  Patch [%d] [%d-%d]: %q -> %s", idx+1, pt.StartByte, pt.EndByte, pt.ReplacementText, pt.Description)
			}
			continue
		}
		_ = refactored
	}
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

func TestTranslator_SingleCallWhenUnderBudget(t *testing.T) {
	entries := map[string]string{
		"k1": "Save",
		"k2": "Cancel",
		"k3": "Delete",
		"k4": "Submit",
	}

	// Large budget (e.g. 5,000 words, 100 keys) -> must produce exactly 1 chunk
	chunks := chunkMapByWordBudget(entries, 5000, 100)
	if len(chunks) != 1 {
		t.Fatalf("Expected exactly 1 chunk when total keys fit within budget, got %d", len(chunks))
	}
	if len(chunks[0]) != 4 {
		t.Fatalf("Expected all 4 keys in the single batch, got %d", len(chunks[0]))
	}
}

func TestTranslator_EffectiveChunkSettingsModelAware(t *testing.T) {
	// 1. Claude/GPT-4o (high context frontier models) -> default 50k token budget (~38000 words, 1500 keys)
	taClaude := &TranslatorAgent{}
	taClaude.LLM = &mockLLMClient{name: "claude-3-5-sonnet"}
	w1, k1, c1 := taClaude.getEffectiveChunkSettings()
	if w1 != 38000 || k1 != 1500 || c1 != 5 {
		t.Fatalf("Expected Claude defaults (38000 words, 1500 keys, 5 concurrency), got %d, %d, %d", w1, k1, c1)
	}

	// 2. Ollama -> 3000 words, 100 keys
	taOllama := &TranslatorAgent{}
	taOllama.LLM = &mockLLMClient{name: "ollama (gemma3:4b)"}
	w2, k2, c2 := taOllama.getEffectiveChunkSettings()
	if w2 != 3000 || k2 != 100 || c2 != 5 {
		t.Fatalf("Expected Ollama defaults (3000 words, 100 keys, 5 concurrency), got %d, %d, %d", w2, k2, c2)
	}

	// 3. NLLB -> 400 words, 50 keys
	taNLLB := &TranslatorAgent{}
	taNLLB.LLM = &mockLLMClient{name: "nllb-200"}
	w3, k3, c3 := taNLLB.getEffectiveChunkSettings()
	if w3 != 400 || k3 != 50 || c3 != 5 {
		t.Fatalf("Expected NLLB defaults (400 words, 50 keys, 5 concurrency), got %d, %d, %d", w3, k3, c3)
	}

	// 4. Custom overrides take strict precedence
	taCustom := &TranslatorAgent{
		ChunkWordBudget: 8000,
		ChunkKeyCeiling: 250,
		Concurrency:     8,
	}
	taCustom.LLM = &mockLLMClient{name: "claude"}
	w4, k4, c4 := taCustom.getEffectiveChunkSettings()
	if w4 != 8000 || k4 != 250 || c4 != 8 {
		t.Fatalf("Expected custom overrides (8000, 250, 8), got %d, %d, %d", w4, k4, c4)
	}
}

type mockLLMClient struct {
	name string
}

func (m *mockLLMClient) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return `{"k1":"translated"}`, nil
}

func (m *mockLLMClient) Description() string {
	return m.name
}

func (m *mockLLMClient) Name() llm.ProviderType {
	return llm.ProviderType(m.name)
}

func TestSupervisor_GeneratesMissingLanguageFilesFromReferencedKeys(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "missing-lang-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a React/Next.js component where text is ALREADY swapped with keys (no raw strings)
	compDir := filepath.Join(tmpDir, "src", "components")
	_ = os.MkdirAll(compDir, 0755)
	compContent := `import React from 'react';
import { useTranslation } from 'react-i18next';

export function FlightCard() {
  const { t } = useTranslation();
  return (
    <div>
      <h1>{t('flight_details')}</h1>
      <button>{t('book_ticket_now')}</button>
    </div>
  );
}
`
	_ = os.WriteFile(filepath.Join(compDir, "FlightCard.tsx"), []byte(compContent), 0644)

	reg := platforms.NewRegistry()
	p, _ := reg.Get(types.FrameworkReact)

	sup, err := NewSupervisorAgent(tmpDir, p)
	if err != nil {
		t.Fatalf("NewSupervisorAgent failed: %v", err)
	}

	// Run end-to-end for Spanish ("es") and French ("fr")
	res, err := sup.RunEndToEnd(context.Background(), "en", []string{"es", "fr"}, false)
	if err != nil {
		t.Fatalf("RunEndToEnd failed: %v", err)
	}

	if res.UniqueKeysCount < 2 {
		t.Fatalf("Expected at least 2 unique keys discovered, got %d", res.UniqueKeysCount)
	}

	// Verify Spanish language file was generated on disk
	esFile := p.DefaultSourceFile(tmpDir, "es")
	if !filepath.IsAbs(esFile) {
		esFile = filepath.Join(tmpDir, esFile)
	}
	esData, err := os.ReadFile(esFile)
	if err != nil {
		t.Fatalf("Expected generated Spanish language file at %s: %v", esFile, err)
	}

	esStr := string(esData)
	if !strings.Contains(esStr, "flight_details") || !strings.Contains(esStr, "book_ticket_now") {
		t.Errorf("Expected Spanish file to contain translated keys, got:\n%s", esStr)
	}
}

