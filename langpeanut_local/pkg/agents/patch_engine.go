package agents

import (
	"fmt"
	"path/filepath"
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

	// 1. Sort all patches in descending order of StartByte (and descending EndByte if StartBytes match)
	sort.Slice(plan.Patches, func(i, j int) bool {
		if plan.Patches[i].StartByte == plan.Patches[j].StartByte {
			return plan.Patches[i].EndByte > plan.Patches[j].EndByte
		}
		return plan.Patches[i].StartByte > plan.Patches[j].StartByte
	})

	// 2. Filter out duplicate / overlapping patches
	var cleanPatches []types.ByteRangePatch
	lastStart := len(content) + 1

	for _, patch := range plan.Patches {
		if patch.StartByte < 0 || patch.EndByte > len(content) || patch.StartByte > patch.EndByte {
			continue
		}
		// If this patch overlaps with an already processed later patch (descending order), skip it
		if patch.EndByte > lastStart {
			continue
		}
		cleanPatches = append(cleanPatches, patch)
		lastStart = patch.StartByte
	}

	// 3. Apply string replacements
	for _, patch := range cleanPatches {
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

	// 5. If React / Next.js and useTranslation or react-i18next is injected, ensure 'use client' directive
	ext := strings.ToLower(filepath.Ext(plan.FilePath))
	if (ext == ".tsx" || ext == ".jsx" || ext == ".ts" || ext == ".js") && strings.Contains(resultStr, "react-i18next") {
		resultStr = EnsureUseClientDirective(resultStr)
	}

	// 6. In-memory AST syntax validation using real tree-sitter grammar
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
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "\"use client\"") || strings.HasPrefix(trimmed, "'use client'") {
			newLines := make([]string, 0, len(lines)+2)
			newLines = append(newLines, lines[:i+1]...)
			newLines = append(newLines, "", importStmt)
			newLines = append(newLines, lines[i+1:]...)
			return strings.Join(newLines, "\n")
		}
	}

	return importStmt + "\n\n" + src
}

// EnsureUseClientDirective ensures 'use client'; is placed at the top of a React/Next.js file
// if it is not already present, avoiding RSC createContext runtime errors.
func EnsureUseClientDirective(src string) string {
	trimmed := strings.TrimSpace(src)
	if strings.HasPrefix(trimmed, "\"use client\"") || strings.HasPrefix(trimmed, "'use client'") {
		return src
	}

	lines := strings.Split(src, "\n")
	insertIdx := -1

	for i, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "//") || strings.HasPrefix(t, "/*") || strings.HasPrefix(t, "*") || strings.HasPrefix(t, "#!") {
			continue
		}
		if strings.HasPrefix(t, "\"use client\"") || strings.HasPrefix(t, "'use client'") {
			return src // Directive already present
		}
		insertIdx = i
		break
	}

	if insertIdx <= 0 {
		return "'use client';\n\n" + src
	}

	newLines := make([]string, 0, len(lines)+2)
	newLines = append(newLines, lines[:insertIdx]...)
	newLines = append(newLines, "'use client';", "")
	newLines = append(newLines, lines[insertIdx:]...)
	return strings.Join(newLines, "\n")
}

// ValidateSyntax validates the refactored code using tree-sitter AST parsing
func (pe *PatchEngine) ValidateSyntax(code string, filePath string) error {
	if platforms.ParsesCleanly(filePath, []byte(code)) {
		return nil
	}
	return fmt.Errorf("tree-sitter detected syntax or grammar error in refactored output")
}
