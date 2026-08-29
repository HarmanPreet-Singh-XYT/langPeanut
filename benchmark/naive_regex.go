package benchmark

import (
	"regexp"
	"sort"
	"strings"

	"github.com/langPeanut/langPeanut/pkg/platforms"
)

// This is a genuinely "naive" regex baseline: it matches every quoted string
// literal in a file, with zero context-awareness (no AST, no skip-list, no
// distinction between UI text and code artifacts). It exists to give the
// benchmark a real, live-measured comparison point instead of a fixed
// historical estimate — this is exactly the kind of tool the project's own
// docs argue is inadequate, so its measured failure rate should stand on its
// own each run rather than being hardcoded.
var naiveQuotedStringRegex = regexp.MustCompile(`"([^"\\]*(?:\\.[^"\\]*)*)"|'([^'\\]*(?:\\.[^'\\]*)*)'`)

// knownBadPatterns identify content that should NEVER be treated as UI text.
// Used only to score false positives after the fact — the extractor itself
// does not consult this list, since a truly naive regex tool wouldn't either.
var knownBadPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^https?://`),                // URLs
	regexp.MustCompile(`^[A-Za-z0-9_/]+$`),           // route-like / identifier-like tokens without spaces (v2/execute, api paths)
	regexp.MustCompile(`(?i)^select\s+.*\s+from\s+`), // SQL
	regexp.MustCompile(`^#[0-9a-fA-F]{3,8}$`),        // hex colors
	regexp.MustCompile(`^\^.*\$$|^\^\[`),             // regex patterns
	regexp.MustCompile(`^package:|^import\s`),        // import/package statements caught by naive matching
}

// NaiveRegexResult captures what the naive-regex tool actually did to a file.
type NaiveRegexResult struct {
	TotalMatches      int
	FalsePositives    int // matches against known-bad content patterns
	CompilesCleanly   bool
	RefactoredContent string
}

type naivePatch struct {
	start, end  int
	replacement string
}

// RunNaiveRegexBaseline extracts every quoted string in the file and wraps
// each one in a fixed replacement token — no framework awareness, no import
// injection, no skip-list, no awareness of surrounding syntax (an import
// specifier, a JSX attribute value, and UI text are all treated identically).
// The rewritten file is then re-parsed with the real tree-sitter grammar for
// its extension (platforms.ParsesCleanly) to get a genuine syntax-validity
// signal, deliberately bypassing the real pipeline's PatchEngine — that
// engine gates on its own syntax validator and would silently discard a
// broken rewrite instead of reporting it as a naive-tool failure.
func RunNaiveRegexBaseline(filePath string, content []byte) NaiveRegexResult {
	src := string(content)
	matches := naiveQuotedStringRegex.FindAllStringSubmatchIndex(src, -1)

	var result NaiveRegexResult
	var patches []naivePatch

	for _, m := range matches {
		start, end := m[0], m[1]
		inner := src[start+1 : end-1]
		if strings.TrimSpace(inner) == "" {
			continue
		}

		result.TotalMatches++
		if matchesKnownBadPattern(inner) {
			result.FalsePositives++
		}

		// Naive replacement: every hit becomes the same unconditional
		// wrapper regardless of context (import specifier, JSX attribute,
		// UI text — a real naive regex tool cannot tell these apart).
		patches = append(patches, naivePatch{
			start:       start,
			end:         end,
			replacement: "t(\"" + naiveKey(inner) + "\")",
		})
	}

	sort.Slice(patches, func(i, j int) bool { return patches[i].start > patches[j].start })

	buf := []byte(src)
	for _, p := range patches {
		var next []byte
		next = append(next, buf[:p.start]...)
		next = append(next, []byte(p.replacement)...)
		next = append(next, buf[p.end:]...)
		buf = next
	}

	result.RefactoredContent = string(buf)
	result.CompilesCleanly = platforms.ParsesCleanly(filePath, buf)
	return result
}

func matchesKnownBadPattern(s string) bool {
	trimmed := strings.TrimSpace(s)
	for _, re := range knownBadPatterns {
		if re.MatchString(trimmed) {
			return true
		}
	}
	return false
}

func naiveKey(s string) string {
	reg := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	clean := strings.Trim(reg.ReplaceAllString(s, "_"), "_")
	if clean == "" {
		return "key"
	}
	if len(clean) > 40 {
		clean = clean[:40]
	}
	return strings.ToLower(clean)
}
