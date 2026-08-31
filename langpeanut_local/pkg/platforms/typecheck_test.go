package platforms

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckASTErrors_ValidTSX(t *testing.T) {
	tempDir := t.TempDir()
	validFile := filepath.Join(tempDir, "Valid.tsx")
	content := `import React from 'react';
export function Valid() {
  return <div>Hello World</div>;
}`
	_ = os.WriteFile(validFile, []byte(content), 0644)

	diags, err := RunDiagnostics(tempDir, []string{validFile})
	if err != nil {
		t.Fatalf("RunDiagnostics failed: %v", err)
	}
	if len(diags) != 0 {
		t.Fatalf("Expected 0 diagnostics for valid TSX, got %d: %+v", len(diags), diags)
	}
}

func TestCheckASTErrors_InvalidTSX(t *testing.T) {
	tempDir := t.TempDir()
	brokenFile := filepath.Join(tempDir, "Broken.tsx")
	// Missing closing bracket and unquoted attribute syntax error
	content := `import React from 'react';
export function Broken() {
  return <div className=t("broken")>Unclosed;
`
	_ = os.WriteFile(brokenFile, []byte(content), 0644)

	diags, err := RunDiagnostics(tempDir, []string{brokenFile})
	if err != nil {
		t.Fatalf("RunDiagnostics failed: %v", err)
	}
	if len(diags) == 0 {
		t.Fatalf("Expected AST diagnostics for broken TSX syntax, got 0")
	}
	t.Logf("Detected AST errors: %+v", diags)
}

func TestParseTypeScriptOutput(t *testing.T) {
	output := `src/components/Cart.tsx(14,23): error TS2304: Cannot find name 't'.
src/components/Hero.tsx(20,5): error TS1005: ';' expected.
other/Ignored.tsx(1,1): error TS2304: Cannot find name 'foo'.`

	filter := []string{"src/components/Cart.tsx", "src/components/Hero.tsx"}
	diags := parseTypeScriptOutput(output, filter)

	if len(diags) != 2 {
		t.Fatalf("Expected 2 filtered diagnostics, got %d", len(diags))
	}
	if diags[0].Line != 14 || diags[0].Column != 23 {
		t.Fatalf("Expected line 14 col 23, got line %d col %d", diags[0].Line, diags[0].Column)
	}
}
