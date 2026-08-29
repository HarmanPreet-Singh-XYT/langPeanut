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
