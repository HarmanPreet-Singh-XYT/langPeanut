package platforms

import (
	"encoding/json"
	"path/filepath"
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// ParsesCleanly re-parses the given content with the real tree-sitter grammar
// for its file extension and reports whether the result is free of grammar
// errors (sitter's Tree.RootNode().HasError()). Unlike a bracket-balance
// heuristic, this catches structurally invalid code that still has matched
// braces/quotes — e.g. `className=t("x")` (an unquoted JSX attribute value)
// or a string literal spliced into the middle of an import specifier.
//
// Used to score arbitrary/untrusted rewrites (a naive regex tool's output, an
// LLM's zero-shot rewrite) where — unlike our own AST-derived byte-range
// patches — there is no guarantee the result is syntactically valid.
func ParsesCleanly(filePath string, content []byte) bool {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".tsx", ".jsx", ".ts", ".js":
		return parsesCleanlyWith(newTSXParser(), content)
	case ".dart":
		return parsesCleanlyWith(newDartParser(), content)
	case ".swift":
		return parsesCleanlyWith(newSwiftParser(), content)
	case ".kt":
		return parsesCleanlyWith(newKotlinParser(), content)
	case ".arb", ".json":
		return json.Valid(content)
	default:
		// No grammar available for this extension — cannot make a claim
		// either way, so don't count it as a pass.
		return false
	}
}

func parsesCleanlyWith(parser *sitter.Parser, content []byte) bool {
	defer parser.Close()

	tree := parser.Parse(content, nil)
	if tree == nil {
		return false
	}
	defer tree.Close()

	return !tree.RootNode().HasError()
}
