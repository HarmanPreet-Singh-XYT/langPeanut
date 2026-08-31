package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirectiveAgent_ScanFileOutline(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "directive-outline-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a mock large file (e.g. 500 lines)
	var sb strings.Builder
	sb.WriteString("import React, { useState } from 'react';\n")
	sb.WriteString("import { useTranslation } from 'react-i18next';\n")
	sb.WriteString("import { UserAvatar } from './UserAvatar';\n\n")

	for i := 5; i <= 200; i++ {
		sb.WriteString(fmt.Sprintf("// line %d padding comment\n", i))
	}

	sb.WriteString("export function MainNavbar() {\n")
	sb.WriteString("  const { t } = useTranslation();\n")
	sb.WriteString("  return (\n")
	sb.WriteString("    <header className=\"flex justify-between\">\n")
	sb.WriteString("      <h1>{t('appTitle')}</h1>\n")
	sb.WriteString("      <UserAvatar />\n")
	sb.WriteString("    </header>\n")
	sb.WriteString("  );\n")
	sb.WriteString("}\n")

	for i := 212; i <= 400; i++ {
		sb.WriteString(fmt.Sprintf("// footer padding line %d\n", i))
	}

	testFile := filepath.Join(tempDir, "Navbar.tsx")
	if err := os.WriteFile(testFile, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	da := NewDirectiveAgent()
	outline := da.scanFileOutline(testFile)

	if !strings.Contains(outline, "Total Lines: ") {
		t.Errorf("Expected Total Lines in outline, got:\n%s", outline)
	}
	if !strings.Contains(outline, "import { useTranslation }") {
		t.Errorf("Expected top imports in outline")
	}
	if !strings.Contains(outline, "export function MainNavbar") {
		t.Errorf("Expected MainNavbar signature in outline")
	}
	// Verify that outline did NOT dump all 400 lines
	outlineLineCount := len(strings.Split(outline, "\n"))
	if outlineLineCount > 60 {
		t.Errorf("Outline too large (%d lines), expected compact outline < 60 lines", outlineLineCount)
	}
}

func TestDirectiveAgent_ReadCodeWindow(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "directive-window-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(tempDir)

	var sb strings.Builder
	for i := 1; i <= 300; i++ {
		sb.WriteString(fmt.Sprintf("line %d: code content\n", i))
	}

	testFile := filepath.Join(tempDir, "App.tsx")
	_ = os.WriteFile(testFile, []byte(sb.String()), 0644)

	da := NewDirectiveAgent()
	window := da.readCodeWindow(testFile, 50, 60)

	if !strings.Contains(window, "  50: line 50") || !strings.Contains(window, "  60: line 60") {
		t.Errorf("Expected lines 50-60 in window, got:\n%s", window)
	}
	if strings.Contains(window, "line 49") || strings.Contains(window, "line 61") {
		t.Errorf("Window bled outside requested range [50, 60]")
	}
}

func TestDirectiveAgent_ApplySurgicalPatch(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "directive-patch-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalCode := `import React from 'react';

export function Header() {
  return (
    <nav>
      <Logo />
      <ThemeToggle />
    </nav>
  );
}
`
	testFile := filepath.Join(tempDir, "Header.tsx")
	_ = os.WriteFile(testFile, []byte(originalCode), 0644)

	da := NewDirectiveAgent()
	search := "<ThemeToggle />"
	replace := "<ThemeToggle />\n      <LanguagePicker />"

	if err := da.applySurgicalPatch(testFile, search, replace); err != nil {
		t.Fatalf("applySurgicalPatch failed: %v", err)
	}

	data, _ := os.ReadFile(testFile)
	if !strings.Contains(string(data), "<LanguagePicker />") {
		t.Errorf("Expected <LanguagePicker /> in patched file, got:\n%s", string(data))
	}
}

func TestDirectiveAgent_ParseActionJSON(t *testing.T) {
	da := NewDirectiveAgent()

	rawMarkdown := "```json\n{\n  \"action\": \"write_component\",\n  \"file_path\": \"src/components/LanguagePicker.tsx\",\n  \"content\": \"export function LanguagePicker() { return null; }\",\n  \"explanation\": \"Created LanguagePicker component\"\n}\n```"

	action, err := da.parseActionJSON(rawMarkdown)
	if err != nil {
		t.Fatalf("parseActionJSON failed on markdown JSON: %v", err)
	}

	if action.Action != "write_component" {
		t.Errorf("Expected action write_component, got %s", action.Action)
	}
	if action.FilePath != "src/components/LanguagePicker.tsx" {
		t.Errorf("Expected file path src/components/LanguagePicker.tsx, got %s", action.FilePath)
	}
}

func TestDirectiveAgent_AutoLinkComponent(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "directive-autolink-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(tempDir)

	_ = os.MkdirAll(filepath.Join(tempDir, "components"), 0755)

	navbarCode := `'use client';
import React from 'react';

export default function Navbar() {
  const toggleTheme = () => {};
  return (
    <nav className="flex justify-between">
      <div>Logo</div>
      <div className="flex items-center gap-2">
        <button onClick={toggleTheme}>Theme</button>
      </div>
    </nav>
  );
}
`
	_ = os.WriteFile(filepath.Join(tempDir, "components", "Navbar.tsx"), []byte(navbarCode), 0644)

	switcherCode := `'use client';
import React from 'react';
import { useTranslation } from 'react-i18next';

export default function LanguageSwitcher() {
  const { i18n } = useTranslation();
  return <select onChange={(e) => i18n.changeLanguage(e.target.value)}><option value="en">EN</option></select>;
}
`
	_ = os.WriteFile(filepath.Join(tempDir, "components", "LanguageSwitcher.tsx"), []byte(switcherCode), 0644)

	da := NewDirectiveAgent()
	created := map[string]bool{"components/LanguageSwitcher.tsx": true}
	patched := make(map[string]bool)

	da.autoLinkComponent(tempDir, created, patched, "react")

	data, _ := os.ReadFile(filepath.Join(tempDir, "components", "Navbar.tsx"))
	content := string(data)

	if !strings.Contains(content, "import LanguageSwitcher from './LanguageSwitcher';") {
		t.Errorf("Expected import LanguageSwitcher in Navbar.tsx, got:\n%s", content)
	}
	if !strings.Contains(content, "<LanguageSwitcher />") {
		t.Errorf("Expected <LanguageSwitcher /> JSX rendered in Navbar.tsx, got:\n%s", content)
	}
}
