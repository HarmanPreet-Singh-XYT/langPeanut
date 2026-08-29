package agents

import (
	"fmt"
	"sort"
	"strings"

	"github.com/langPeanut/langPeanut/pkg/platforms"
	"github.com/langPeanut/langPeanut/pkg/types"
)

// PatchEngine applies surgical, deterministic byte-range mutations to source code
type PatchEngine struct{}

func NewPatchEngine() *PatchEngine {
	return &PatchEngine{}
}

// ApplyRefactorPlan executes the byte-range replacements and import injections safely in memory
func (pe *PatchEngine) ApplyRefactorPlan(plan *types.FileRefactorPlan) (string, error) {
	content := []byte(plan.OriginalContent)

	// 1. Sort all patches in descending order of StartByte so later replacements don't invalidate earlier offsets
	sort.Slice(plan.Patches, func(i, j int) bool {
		return plan.Patches[i].StartByte > plan.Patches[j].StartByte
	})

	// 2. Apply string replacements
	for _, patch := range plan.Patches {
		if patch.StartByte < 0 || patch.EndByte > len(content) || patch.StartByte > patch.EndByte {
			return "", fmt.Errorf("invalid patch byte range [%d, %d] on file size %d", patch.StartByte, patch.EndByte, len(content))
		}

		prefix := content[:patch.StartByte]
		suffix := content[patch.EndByte:]

		var buf []byte
		buf = append(buf, prefix...)
		buf = append(buf, []byte(patch.ReplacementText)...)
		buf = append(buf, suffix...)
		content = buf
	}

	resultStr := string(content)

	// 4. Inject required imports
	for _, imp := range plan.RequiredImports {
		if !strings.Contains(resultStr, imp) {
			resultStr = pe.injectImport(resultStr, imp)
		}
	}

	// 5. In-memory AST syntax validation using real tree-sitter grammar
	if err := pe.ValidateSyntax(resultStr, plan.FilePath); err != nil {
		return "", fmt.Errorf("in-memory AST validation failed for %s: %w", plan.FilePath, err)
	}

	plan.RefactoredContent = resultStr
	return resultStr, nil
}

// injectImport places the import statement cleanly after existing imports
func (pe *PatchEngine) injectImport(src, importStmt string) string {
	lines := strings.Split(src, "\n")
	lastImportIdx := -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "import ") || strings.HasPrefix(trimmed, "import '") || strings.HasPrefix(trimmed, "import \"") {
			lastImportIdx = i
		}
	}

	if lastImportIdx != -1 {
		// Insert after last import
		newLines := make([]string, 0, len(lines)+1)
		newLines = append(newLines, lines[:lastImportIdx+1]...)
		newLines = append(newLines, importStmt)
		newLines = append(newLines, lines[lastImportIdx+1:]...)
		return strings.Join(newLines, "\n")
	}

	// If no existing imports, prepend at top (after "use client" if present)
	if len(lines) > 0 && (strings.HasPrefix(strings.TrimSpace(lines[0]), "\"use client\"") || strings.HasPrefix(strings.TrimSpace(lines[0]), "'use client'")) {
		newLines := make([]string, 0, len(lines)+2)
		newLines = append(newLines, lines[0], "", importStmt)
		newLines = append(newLines, lines[1:]...)
		return strings.Join(newLines, "\n")
	}

	return importStmt + "\n\n" + src
}

// ValidateSyntax validates the refactored code using tree-sitter AST parsing
func (pe *PatchEngine) ValidateSyntax(code string, filePath string) error {
	if platforms.ParsesCleanly(filePath, []byte(code)) {
		return nil
	}
	return fmt.Errorf("tree-sitter detected syntax or grammar error in refactored output")
}
