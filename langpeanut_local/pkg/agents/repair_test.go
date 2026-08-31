package agents

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/langPeanut/langPeanut/pkg/types"
)

func TestCodeRepairAgent_HeuristicRepair(t *testing.T) {
	tempDir := t.TempDir()
	componentFile := filepath.Join(tempDir, "Navbar.tsx")

	// Missing import statement error for useTranslation
	brokenContent := `import React from 'react';

export function Navbar() {
  const { t } = useTranslation();
  return <nav>{t('home')}</nav>;
}`
	_ = os.WriteFile(componentFile, []byte(brokenContent), 0644)

	diags := []types.CompilerDiagnostic{
		{
			FilePath: componentFile,
			Line:     4,
			Column:   17,
			Message:  "Cannot find name 'useTranslation'",
			Source:   "tsc",
			Severity: "ERROR",
		},
	}

	repairAgent := NewCodeRepairAgent()
	res, err := repairAgent.RepairFile(context.Background(), tempDir, componentFile, diags, types.FrameworkReact)
	if err != nil {
		t.Fatalf("RepairFile returned error: %v", err)
	}

	if !res.Repaired {
		t.Fatalf("Expected file to be auto-repaired, got false")
	}

	repairedBytes, _ := os.ReadFile(componentFile)
	repairedStr := string(repairedBytes)
	if !testing.Short() {
		t.Logf("Repaired file content:\n%s", repairedStr)
	}

	if !containsStr(repairedStr, "useTranslation") || !containsStr(repairedStr, "react-i18next") {
		t.Fatalf("Expected useTranslation import in repaired file, got:\n%s", repairedStr)
	}
}

func TestCodeRepairAgent_NextJSUseClientRepair(t *testing.T) {
	tempDir := t.TempDir()
	footerFile := filepath.Join(tempDir, "Footer.tsx")

	// Server component importing useTranslation without 'use client'
	brokenContent := `import { GithubIcon } from "@/components/icons";
import { useTranslation } from 'react-i18next';

export function Footer() {
  const { t } = useTranslation();
  return <footer>{t('copyright')}</footer>;
}`
	_ = os.WriteFile(footerFile, []byte(brokenContent), 0644)

	diags := []types.CompilerDiagnostic{
		{
			FilePath: footerFile,
			Line:     7,
			Column:   1,
			Message:  "createContext only works in Client Components. Add the \"use client\" directive at the top of the file to use it.",
			Source:   "nextjs",
			Severity: "ERROR",
		},
	}

	repairAgent := NewCodeRepairAgent()
	res, err := repairAgent.RepairFile(context.Background(), tempDir, footerFile, diags, types.FrameworkNextJS)
	if err != nil {
		t.Fatalf("RepairFile returned error: %v", err)
	}

	if !res.Repaired {
		t.Fatalf("Expected Next.js RSC error to be auto-repaired, got false")
	}

	repairedBytes, _ := os.ReadFile(footerFile)
	repairedStr := string(repairedBytes)

	if !containsStr(repairedStr, "use client") {
		t.Fatalf("Expected 'use client' directive at top of repaired file, got:\n%s", repairedStr)
	}
}

func TestEnsureUseClientDirective(t *testing.T) {
	// Case 1: Simple file without directive
	code1 := `import { useTranslation } from 'react-i18next';
export function App() { return <div>Hello</div>; }`
	res1 := EnsureUseClientDirective(code1)
	if !containsStr(res1, "'use client';") {
		t.Fatalf("Expected 'use client'; in res1, got:\n%s", res1)
	}

	// Case 2: File already containing 'use client'
	code2 := `'use client';
import { useTranslation } from 'react-i18next';`
	res2 := EnsureUseClientDirective(code2)
	if res2 != code2 {
		t.Fatalf("Expected unchanged code2, got:\n%s", res2)
	}

	// Case 3: File with leading comments
	code3 := `// Copyright 2026 Acme Corp
/* Another comment */
import { useTranslation } from 'react-i18next';`
	res3 := EnsureUseClientDirective(code3)
	if !containsStr(res3, "'use client';") {
		t.Fatalf("Expected 'use client'; in res3, got:\n%s", res3)
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || (len(s) > len(sub) && findSub(s, sub)))
}

func findSub(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
