package benchmark

import (
	"context"
	"os"
	"regexp"
	"strings"

	"github.com/langPeanut/langPeanut/pkg/llm"
	"github.com/langPeanut/langPeanut/pkg/platforms"
)

// LLMBaselineResult captures the outcome of the zero-shot single-prompt baseline.
type LLMBaselineResult struct {
	PassRate     float64
	ICUIntegrity float64
	Live         bool   // true if this was measured against a real API call
	Provider     string // which provider actually answered, or "historical-estimate"
}

// zeroShotSystemPrompt mirrors the naive, single-prompt approach the project's
// own docs argue against: no AST tooling, no structured output contract, just
// "do the whole refactor yourself" — used here as a genuine, live baseline
// rather than an assumed number.
const zeroShotSystemPrompt = `You are a code localization assistant. Extract all user-facing strings from the given source file and refactor it to use a localization framework call (e.g. t('key'), AppLocalizations.of(context).key, or the idiomatic equivalent for the file's language/framework). Preserve all variable placeholders exactly. Return ONLY the complete refactored source file, with no explanation, no markdown code fences, and no commentary.`

// variablePlaceholderRegex finds ICU-style {var}, Dart/JS $var and ${var},
// and Swift \(var) placeholders in a string so integrity can be checked
// before/after the LLM's rewrite.
var variablePlaceholderRegex = regexp.MustCompile(`\{[a-zA-Z0-9_]+\}|\$\{[^}]+\}|\$[a-zA-Z_][a-zA-Z0-9_]*|\\\([^)]+\)`)

// RunZeroShotLLMBaseline sends the raw source file to a real LLM with a single
// unstructured prompt and measures the result against the same syntax
// validator and placeholder-integrity check used elsewhere in the benchmark.
//
// It prefers OpenAI (paid tier, no surprise daily-quota exhaustion) when
// OPENAI_API_KEY is configured, falling back to Gemini's free tier when only
// GEMINI_API_KEY is set. Both are loaded from .env by the CLI entrypoint,
// never hardcoded. Without either key, it returns a clearly labeled
// historical estimate from earlier manual baseline runs rather than
// fabricating a live result.
func RunZeroShotLLMBaseline(ctx context.Context, fileName string, content []byte) LLMBaselineResult {
	var client llm.Client
	var providerName string

	switch {
	case os.Getenv("OPENAI_API_KEY") != "":
		client = llm.NewClient(llm.ProviderOpenAI, "gpt-4o-mini")
		providerName = "openai"
	case os.Getenv("GEMINI_API_KEY") != "":
		client = llm.NewClient(llm.ProviderGemini, "")
		providerName = "gemini"
	default:
		return LLMBaselineResult{
			PassRate:     42.0, // historical estimate from earlier manual zero-shot runs — see CHANGELOG.md
			ICUIntegrity: 60.0,
			Live:         false,
			Provider:     "historical-estimate",
		}
	}

	userPrompt := "File: " + fileName + "\n\n```\n" + string(content) + "\n```"

	response, err := client.Complete(ctx, zeroShotSystemPrompt, userPrompt)
	if err != nil {
		// A real API call was attempted but failed (rate limit, network,
		// auth) — treat as a failed baseline run rather than silently
		// falling back to the historical estimate, since a fallback here
		// would misrepresent an actual live attempt as a fixed number.
		return LLMBaselineResult{
			PassRate:     0.0,
			ICUIntegrity: 0.0,
			Live:         true,
			Provider:     providerName,
		}
	}

	refactored := stripMarkdownFences(response)

	passRate := 0.0
	if platforms.ParsesCleanly(fileName, []byte(refactored)) {
		passRate = 100.0
	}

	icuIntegrity := 100.0
	if !placeholdersPreserved(string(content), refactored) {
		icuIntegrity = 0.0
	}

	return LLMBaselineResult{
		PassRate:     passRate,
		ICUIntegrity: icuIntegrity,
		Live:         true,
		Provider:     providerName,
	}
}

// placeholdersPreserved checks that every variable placeholder found in the
// original source still appears, byte-for-byte, in the model's rewrite. A
// single-prompt LLM commonly "translates" or renames these, which is exactly
// the failure mode the project's docs cite as the reason naive prompting
// fails — this check measures it directly instead of assuming it.
func placeholdersPreserved(original, refactored string) bool {
	originals := variablePlaceholderRegex.FindAllString(original, -1)
	if len(originals) == 0 {
		return true
	}
	for _, ph := range originals {
		if !strings.Contains(refactored, ph) {
			return false
		}
	}
	return true
}

func stripMarkdownFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		lines := strings.Split(s, "\n")
		if len(lines) > 1 {
			lines = lines[1:]
		}
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "```" {
			lines = lines[:len(lines)-1]
		}
		s = strings.Join(lines, "\n")
	}
	return s
}
