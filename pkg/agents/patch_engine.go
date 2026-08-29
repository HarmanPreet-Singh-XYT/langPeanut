package agents

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

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

	// 5. In-memory AST syntax validation
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

	// If no existing imports, prepend at top
	return importStmt + "\n\n" + src
}

// ValidateSyntax performs bracket parity, delimiter balance, and JSX tag checks
func (pe *PatchEngine) ValidateSyntax(code string, filePath string) error {
	var stack []rune
	pairs := map[rune]rune{
		')': '(',
		'}': '{',
		']': '[',
	}

	inSingleQuote := false
	inDoubleQuote := false
	inBacktick := false
	inLineComment := false
	inBlockComment := false

	runes := []rune(code)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		next := rune(0)
		if i+1 < len(runes) {
			next = runes[i+1]
		}

		if inLineComment {
			if r == '\n' {
				inLineComment = false
			}
			continue
		}

		if inBlockComment {
			if r == '*' && next == '/' {
				inBlockComment = false
				i++
			}
			continue
		}

		if r == '/' && next == '/' && !inSingleQuote && !inDoubleQuote && !inBacktick {
			inLineComment = true
			i++
			continue
		}

		if r == '/' && next == '*' && !inSingleQuote && !inDoubleQuote && !inBacktick {
			inBlockComment = true
			i++
			continue
		}

		// Handle quotes
		if r == '\'' && !inDoubleQuote && !inBacktick {
			inSingleQuote = !inSingleQuote
			continue
		}
		if r == '"' && !inSingleQuote && !inBacktick {
			inDoubleQuote = !inDoubleQuote
			continue
		}
		if r == '`' && !inSingleQuote && !inDoubleQuote {
			inBacktick = !inBacktick
			continue
		}

		if inSingleQuote || inDoubleQuote || inBacktick {
			continue
		}

		// Bracket balance
		if r == '(' || r == '{' || r == '[' {
			stack = append(stack, r)
		} else if match, ok := pairs[r]; ok {
			if len(stack) == 0 || stack[len(stack)-1] != match {
				return fmt.Errorf("unbalanced bracket '%c' at character %d", r, i)
			}
			stack = stack[:len(stack)-1]
		}
	}

	if len(stack) > 0 {
		return fmt.Errorf("unclosed bracket '%c' remaining at end of file", stack[len(stack)-1])
	}

	// Check JSX tag matching if TSX/JSX
	if strings.HasSuffix(filePath, ".tsx") || strings.HasSuffix(filePath, ".jsx") {
		if err := validateJSXTagBalance(code); err != nil {
			return err
		}
	}

	return nil
}

func validateJSXTagBalance(code string) error {
	// Simple JSX tag counter for common elements
	openTagRegex := regexp.MustCompile(`<([A-Z][a-zA-Z0-9]*)[^>]*[^/]>`)
	closeTagRegex := regexp.MustCompile(`</([A-Z][a-zA-Z0-9]*)>`)

	openMatches := openTagRegex.FindAllStringSubmatch(code, -1)
	closeMatches := closeTagRegex.FindAllStringSubmatch(code, -1)

	openCount := make(map[string]int)
	for _, m := range openMatches {
		if len(m) > 1 {
			openCount[m[1]]++
		}
	}

	for _, m := range closeMatches {
		if len(m) > 1 {
			openCount[m[1]]--
		}
	}

	for tag, diff := range openCount {
		if diff != 0 {
			return fmt.Errorf("JSX tag <%s> mismatch (open vs close delta: %d)", tag, diff)
		}
	}

	return nil
}
