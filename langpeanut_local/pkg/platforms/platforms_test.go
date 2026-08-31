package platforms

import (
	"os"
	"testing"

	"github.com/langPeanut/langPeanut/pkg/types"
)

func TestReactPlatform_Extract(t *testing.T) {
	_ = types.FrameworkReact
	p := NewReactPlatform()

	src := []byte(`import React from 'react';
export const Card = () => (
	<div>
		<h2>Checkout Summary</h2>
		<button>Submit Order</button>
		<div>Clone & install dependencies</div>
		<div>Mobile (iOS & Android)</div>
		<p>Found something that isn&apos;t working right?</p>
	</div>
);
`)

	cands, err := p.ExtractCandidates("Card.tsx", src)
	if err != nil {
		t.Fatalf("ExtractCandidates failed: %v", err)
	}

	for i, c := range cands {
		t.Logf("[%d] Candidate: %q (Raw: %q)", i+1, c.CleanValue, c.RawValue)
	}

	if len(cands) != 5 {
		t.Fatalf("expected 5 candidates, got %d", len(cands))
	}

	if cands[2].CleanValue != "Clone & install dependencies" {
		t.Errorf("expected candidate 3 to be %q, got %q", "Clone & install dependencies", cands[2].CleanValue)
	}

	if cands[3].CleanValue != "Mobile (iOS & Android)" {
		t.Errorf("expected candidate 4 to be %q, got %q", "Mobile (iOS & Android)", cands[3].CleanValue)
	}

	if cands[4].CleanValue != "Found something that isn't working right?" {
		t.Errorf("expected candidate 5 to be %q, got %q", "Found something that isn't working right?", cands[4].CleanValue)
	}
}

func TestFlutterPlatform_Extract(t *testing.T) {
	p := NewFlutterPlatform()

	src := []byte(`import 'package:flutter/material.dart';
Widget build(BuildContext context) => const Text("Welcome Home");
`)

	cands, err := p.ExtractCandidates("home.dart", src)
	if err != nil {
		t.Fatalf("ExtractCandidates failed: %v", err)
	}

	if len(cands) == 0 {
		t.Fatalf("expected at least 1 candidate, got 0")
	}
}

func TestSwiftPlatform_Extract(t *testing.T) {
	p := NewSwiftPlatform()

	src := []byte(`import SwiftUI
struct View: View {
    var body: some View {
        Text("Settings")
    }
}
`)

	cands, err := p.ExtractCandidates("View.swift", src)
	if err != nil {
		t.Fatalf("ExtractCandidates failed: %v", err)
	}

	if len(cands) == 0 {
		t.Fatalf("expected at least 1 candidate, got 0")
	}
}

func TestKotlinPlatform_Extract(t *testing.T) {
	p := NewAndroidPlatform()

	src := []byte(`package com.app
import androidx.compose.material3.Text
fun Header() {
    Text(text = "Submit Order")
}
`)

	cands, err := p.ExtractCandidates("Header.kt", src)
	if err != nil {
		t.Fatalf("ExtractCandidates failed: %v", err)
	}

	if len(cands) == 0 {
		t.Fatalf("expected at least 1 candidate, got 0")
	}
}

func TestFlutterPlatform_Case04(t *testing.T) {
	p := NewFlutterPlatform()
	src := []byte(`import 'package:flutter/material.dart';

class HomeScreen extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text("Welcome Home"),
      ),
      body: Center(
        child: Column(
          children: const [
            Text("Your Dashboard"),
            Tooltip(message: "View details"),
          ],
        ),
      ),
    );
  }
}
`)

	cands, err := p.ExtractCandidates("04_flutter_const_tree.dart", src)
	if err != nil {
		t.Fatalf("ExtractCandidates failed: %v", err)
	}
	t.Logf("Found %d candidates", len(cands))
	for _, c := range cands {
		t.Logf("Candidate: %s -> %s", c.Key, c.CleanValue)
	}

	plan, err := p.GenerateRefactorPlan("04_flutter_const_tree.dart", src, cands)
	if err != nil {
		t.Fatalf("GenerateRefactorPlan failed: %v", err)
	}

	if !ParsesCleanly("04_flutter_const_tree.dart", []byte(plan.RefactoredContent)) {
		t.Logf("Refactored code:\n%s", plan.RefactoredContent)
		t.Errorf("Refactored code failed ParsesCleanly")
	}
}

func TestFlutterPlatform_Case05(t *testing.T) {
	p := NewFlutterPlatform()
	src := []byte(`{
  "@@locale": "en",
  "welcomeUser": "Welcome back, {name}!",
  "@welcomeUser": {
    "description": "Greeting message",
    "placeholders": {
      "name": { "type": "String" }
    }
  },
  "itemCount": "You have {count} items in your cart",
  "@itemCount": {
    "placeholders": {
      "count": { "type": "int" }
    }
  }
}
`)

	cands, err := p.ExtractCandidates("05_flutter_complex_icu.arb", src)
	if err != nil {
		t.Fatalf("ExtractCandidates failed: %v", err)
	}
	t.Logf("Found %d ARB candidates", len(cands))
	plan, err := p.GenerateRefactorPlan("05_flutter_complex_icu.arb", src, cands)
	if err != nil {
		t.Fatalf("GenerateRefactorPlan failed: %v", err)
	}
	if !ParsesCleanly("05_flutter_complex_icu.arb", []byte(plan.RefactoredContent)) {
		t.Errorf("Refactored ARB failed ParsesCleanly")
	}
}

func TestReactPlatform_ExtractConditionalTernary(t *testing.T) {
	p := NewReactPlatform()
	src := []byte(`import React from 'react';
export function ActionButton({ isLoading, isError }: { isLoading: boolean, isError: boolean }) {
	return (
		<div>
			<button>{isLoading ? "Saving Order..." : "Save Changes"}</button>
			{isError && <span>Payment Failed</span>}
		</div>
	);
}
`)

	cands, err := p.ExtractCandidates("ActionButton.tsx", src)
	if err != nil {
		t.Fatalf("ExtractCandidates failed: %v", err)
	}

	if len(cands) != 3 {
		t.Fatalf("expected 3 candidates from conditional rendering, got %d", len(cands))
	}

	if cands[0].CleanValue != "Saving Order..." {
		t.Errorf("expected 'Saving Order...', got %q", cands[0].CleanValue)
	}
	if cands[1].CleanValue != "Save Changes" {
		t.Errorf("expected 'Save Changes', got %q", cands[1].CleanValue)
	}
	if cands[2].CleanValue != "Payment Failed" {
		t.Errorf("expected 'Payment Failed', got %q", cands[2].CleanValue)
	}

	plan, err := p.GenerateRefactorPlan("ActionButton.tsx", src, cands)
	if err != nil {
		t.Fatalf("GenerateRefactorPlan failed: %v", err)
	}

	if !ParsesCleanly("ActionButton.tsx", []byte(plan.RefactoredContent)) {
		t.Logf("Refactored:\n%s", plan.RefactoredContent)
		t.Errorf("Refactored code failed ParsesCleanly")
	}
}

func TestFlutterPlatform_ExtractConditionalTernary(t *testing.T) {
	p := NewFlutterPlatform()
	src := []byte(`import 'package:flutter/material.dart';
Widget build(BuildContext context, bool isLoggedIn) {
	return ElevatedButton(
		onPressed: () {},
		child: Text(isLoggedIn ? 'Welcome Back!' : 'Sign In to Continue'),
	);
}
`)

	cands, err := p.ExtractCandidates("conditional_view.dart", src)
	if err != nil {
		t.Fatalf("ExtractCandidates failed: %v", err)
	}

	if len(cands) != 2 {
		t.Fatalf("expected 2 candidates from Flutter ternary, got %d", len(cands))
	}

	if cands[0].CleanValue != "Welcome Back!" {
		t.Errorf("expected 'Welcome Back!', got %q", cands[0].CleanValue)
	}
	if cands[1].CleanValue != "Sign In to Continue" {
		t.Errorf("expected 'Sign In to Continue', got %q", cands[1].CleanValue)
	}

	plan, err := p.GenerateRefactorPlan("conditional_view.dart", src, cands)
	if err != nil {
		t.Fatalf("GenerateRefactorPlan failed: %v", err)
	}

	if !ParsesCleanly("conditional_view.dart", []byte(plan.RefactoredContent)) {
		t.Logf("Refactored:\n%s", plan.RefactoredContent)
		t.Errorf("Refactored Dart code failed ParsesCleanly")
	}
}

func TestReactPlatform_ExtractCustomHookDialog(t *testing.T) {
	p := NewReactPlatform()
	src := []byte(`import React from 'react';
export function UserSettings() {
	const { openConfirm } = useConfirmDialog();
	const handleDelete = () => {
		openConfirm({
			title: "Delete Account",
			message: "This action cannot be undone.",
			confirmLabel: "Delete Now",
			cancelLabel: "Cancel"
		});
		toast.success("Settings updated successfully");
	};

	return (
		<div>
			<button onClick={handleDelete}>Delete Account</button>
		</div>
	);
}
`)

	cands, err := p.ExtractCandidates("UserSettings.tsx", src)
	if err != nil {
		t.Fatalf("ExtractCandidates failed: %v", err)
	}

	if len(cands) < 5 {
		t.Fatalf("expected at least 5 candidates from custom hook props & toast, got %d", len(cands))
	}

	plan, err := p.GenerateRefactorPlan("UserSettings.tsx", src, cands)
	if err != nil {
		t.Fatalf("GenerateRefactorPlan failed: %v", err)
	}

	if !ParsesCleanly("UserSettings.tsx", []byte(plan.RefactoredContent)) {
		t.Logf("Refactored UserSettings.tsx:\n%s", plan.RefactoredContent)
		t.Errorf("Refactored code failed ParsesCleanly")
	}
}

func TestSwiftPlatform_ExtractModifiers(t *testing.T) {
	p := NewSwiftPlatform()
	src := []byte(`import SwiftUI
struct DashboardView: View {
    var body: some View {
        NavigationView {
            VStack {
                Text("Welcome to Dashboard")
                Button("Upgrade Plan") { }
            }
            .navigationTitle("Analytics Overview")
        }
    }
}
`)

	cands, err := p.ExtractCandidates("DashboardView.swift", src)
	if err != nil {
		t.Fatalf("ExtractCandidates failed: %v", err)
	}

	if len(cands) < 3 {
		t.Fatalf("expected at least 3 candidates from SwiftUI views & navigationTitle, got %d", len(cands))
	}
}

func TestKotlinPlatform_ExtractNamedArgs(t *testing.T) {
	p := NewAndroidPlatform()
	src := []byte(`package com.app
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable

@Composable
fun LoginForm() {
    OutlinedTextField(
        value = "",
        onValueChange = {},
        label = { Text("Email Address") },
        placeholder = { Text("Enter your email") }
    )
}
`)

	cands, err := p.ExtractCandidates("LoginForm.kt", src)
	if err != nil {
		t.Fatalf("ExtractCandidates failed: %v", err)
	}

	if len(cands) < 2 {
		t.Fatalf("expected at least 2 candidates from Compose text/label, got %d", len(cands))
	}
}

func TestPingDashboard_Refactor(t *testing.T) {
	p := NewReactPlatform()
	filePath := "/Users/harmanpreetsingh/Public/Code/pingroute-web/components/PingDashboard.tsx"
	src, err := os.ReadFile(filePath)
	if err != nil {
		t.Skipf("File not found: %v", err)
	}

	cands, err := p.ExtractCandidates(filePath, src)
	if err != nil {
		t.Fatalf("ExtractCandidates failed: %v", err)
	}
	t.Logf("Found %d candidates in PingDashboard.tsx", len(cands))

	res, err := p.GenerateRefactorPlan(filePath, src, cands)
	if err != nil {
		t.Fatalf("GenerateRefactorPlan failed: %v", err)
	}
	if !ParsesCleanly(filePath, []byte(res.RefactoredContent)) {
		t.Logf("Refactored code:\n%s", res.RefactoredContent)
		t.Errorf("Refactored code failed ParsesCleanly")
	}
}

// TestFlutterPlatform_DiscoverExistingLocales exercises the "existing project
// with i18n already set up" scenario: a Flutter app with lib/l10n/app_{en,de,
// es,fr,ja}.arb already present and translated, without an l10n.yaml.
func TestFlutterPlatform_DiscoverExistingLocales(t *testing.T) {
	root := t.TempDir()
	l10nDir := root + "/lib/l10n"
	if err := os.MkdirAll(l10nDir, 0755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"app_en.arb": `{"@@locale": "en", "greeting": "Hello"}`,
		"app_de.arb": `{"@@locale": "de", "greeting": "Hallo"}`,
		"app_es.arb": `{"@@locale": "es", "greeting": "Hola"}`,
		"app_fr.arb": `{"@@locale": "fr", "greeting": "Bonjour"}`,
		"app_ja.arb": `{"@@locale": "ja", "greeting": "こんにちは"}`,
	}
	for name, content := range files {
		if err := os.WriteFile(l10nDir+"/"+name, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	p := NewFlutterPlatform()
	found, err := p.DiscoverExistingLocales(root)
	if err != nil {
		t.Fatalf("DiscoverExistingLocales failed: %v", err)
	}
	if len(found) != 5 {
		t.Fatalf("expected 5 discovered locales, got %d: %v", len(found), found)
	}
	for _, loc := range []string{"en", "de", "es", "fr", "ja"} {
		if _, ok := found[loc]; !ok {
			t.Errorf("expected locale %q to be discovered, got %v", loc, found)
		}
	}

	// A target locale not yet present must not show up.
	if _, ok := found["pt"]; ok {
		t.Errorf("locale 'pt' should not have been discovered")
	}

	// Existing translations must round-trip untouched via ParseLocaleFileForLocale.
	deData, err := os.ReadFile(found["de"])
	if err != nil {
		t.Fatal(err)
	}
	locData, err := p.ParseLocaleFileForLocale(deData, "de")
	if err != nil {
		t.Fatalf("ParseLocaleFileForLocale failed: %v", err)
	}
	if locData.Entries["greeting"] != "Hallo" {
		t.Errorf("expected existing German translation 'Hallo' preserved, got %q", locData.Entries["greeting"])
	}
}

// TestFlutterPlatform_L10nYAML verifies arb-dir/template-arb-file from an
// existing project's l10n.yaml override the lib/l10n + app_ defaults.
func TestFlutterPlatform_L10nYAML(t *testing.T) {
	root := t.TempDir()
	customDir := root + "/lib/i18n"
	if err := os.MkdirAll(customDir, 0755); err != nil {
		t.Fatal(err)
	}
	l10nYAML := "arb-dir: lib/i18n\ntemplate-arb-file: intl_en.arb\n"
	if err := os.WriteFile(root+"/l10n.yaml", []byte(l10nYAML), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(customDir+"/intl_en.arb", []byte(`{"@@locale": "en", "hi": "Hi"}`), 0644); err != nil {
		t.Fatal(err)
	}

	p := NewFlutterPlatform()
	if got := p.DefaultLocaleDir(root); got != "lib/i18n" {
		t.Errorf("expected DefaultLocaleDir to honor l10n.yaml arb-dir, got %q", got)
	}
	if got := p.DefaultSourceFile(root, "en"); got != "lib/i18n/intl_en.arb" {
		t.Errorf("expected DefaultSourceFile to honor template-arb-file prefix, got %q", got)
	}

	found, err := p.DiscoverExistingLocales(root)
	if err != nil {
		t.Fatalf("DiscoverExistingLocales failed: %v", err)
	}
	if _, ok := found["en"]; !ok {
		t.Errorf("expected 'en' discovered via l10n.yaml-configured dir, got %v", found)
	}
}

// TestReactPlatform_DiscoverExistingLocales covers both the flat
// "{lang}.json" layout and the i18next "{lang}/{namespace}.json" layout.
func TestReactPlatform_DiscoverExistingLocales(t *testing.T) {
	root := t.TempDir()
	localesDir := root + "/public/locales"

	for _, lang := range []string{"en", "de", "fr"} {
		dir := localesDir + "/" + lang
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dir+"/common.json", []byte(`{"hello": "hi"}`), 0644); err != nil {
			t.Fatal(err)
		}
	}

	p := NewReactPlatform()
	found, err := p.DiscoverExistingLocales(root)
	if err != nil {
		t.Fatalf("DiscoverExistingLocales failed: %v", err)
	}
	if len(found) != 3 {
		t.Fatalf("expected 3 discovered locales, got %d: %v", len(found), found)
	}

	if got := p.DefaultSourceFile(root, "en"); got != "public/locales/en/common.json" {
		t.Errorf("expected i18next-style source file, got %q", got)
	}
}

// TestAndroidPlatform_DiscoverExistingLocales covers values-{qualifier}/
// directories, including the pt-rBR region-qualifier naming.
func TestAndroidPlatform_DiscoverExistingLocales(t *testing.T) {
	root := t.TempDir()
	resDir := root + "/app/src/main/res"

	for _, dir := range []string{"values", "values-de", "values-pt-rBR"} {
		full := resDir + "/" + dir
		if err := os.MkdirAll(full, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full+"/strings.xml", []byte(`<resources><string name="hi">Hi</string></resources>`), 0644); err != nil {
			t.Fatal(err)
		}
	}

	p := NewAndroidPlatform()
	found, err := p.DiscoverExistingLocales(root)
	if err != nil {
		t.Fatalf("DiscoverExistingLocales failed: %v", err)
	}
	if _, ok := found["en"]; !ok {
		t.Errorf("expected default 'values' dir mapped to 'en', got %v", found)
	}
	if _, ok := found["de"]; !ok {
		t.Errorf("expected 'de' discovered, got %v", found)
	}
	if _, ok := found["pt-BR"]; !ok {
		t.Errorf("expected 'pt-BR' discovered from values-pt-rBR, got %v", found)
	}
}

// TestSwiftPlatform_MergePreservesOtherLocales verifies that formatting an
// .xcstrings catalog for one locale merges into the existing shared catalog
// rather than discarding every other locale's translations (a single
// .xcstrings file holds all locales together, unlike ARB/strings.xml/i18next
// JSON which use one file per locale).
func TestSwiftPlatform_MergePreservesOtherLocales(t *testing.T) {
	p := NewSwiftPlatform()

	existingCatalog := []byte(`{
		"sourceLanguage": "en",
		"version": "1.0",
		"strings": {
			"greeting": {
				"extractionState": "manual",
				"localizations": {
					"en": {"stringUnit": {"state": "translated", "value": "Hello"}},
					"de": {"stringUnit": {"state": "translated", "value": "Hallo"}}
				}
			}
		}
	}`)

	// Simulate adding a French translation for the same key.
	locData, err := p.ParseLocaleFileForLocale(existingCatalog, "fr")
	if err != nil {
		t.Fatalf("ParseLocaleFileForLocale failed: %v", err)
	}
	locData.Entries["greeting"] = "Bonjour"

	out, err := p.FormatLocaleFile(*locData)
	if err != nil {
		t.Fatalf("FormatLocaleFile failed: %v", err)
	}

	// Re-parse per-locale to confirm all three locales survived the merge.
	for locale, want := range map[string]string{"en": "Hello", "de": "Hallo", "fr": "Bonjour"} {
		ld, err := p.ParseLocaleFileForLocale(out, locale)
		if err != nil {
			t.Fatalf("ParseLocaleFileForLocale(%q) failed: %v", locale, err)
		}
		if ld.Entries["greeting"] != want {
			t.Errorf("locale %q: expected %q, got %q (merge overwrote existing catalog)", locale, want, ld.Entries["greeting"])
		}
	}
}

