package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/langPeanut/langPeanut/pkg/llm"
)

// PersonaReport contains the auto-discovered brand lexicon, tone, and audience profile
type PersonaReport struct {
	ProjectName      string   `json:"project_name"`
	BrandLexicon     []string `json:"brand_lexicon"`
	RecommendedTone  string   `json:"recommended_tone"` // neutral, casual, corporate, technical, genz
	Audience         string   `json:"audience"`
	Summary          string   `json:"summary"`
	LocalesSuggested []string `json:"locales_suggested"`
	ConfidenceScore  float64  `json:"confidence_score"`
}

// PersonaScoutAgent discovers brand voice and glossary tokens from repository assets
type PersonaScoutAgent struct {
	LLMClient llm.Client
}

// NewPersonaScoutAgent creates a new PersonaScoutAgent
func NewPersonaScoutAgent(client llm.Client) *PersonaScoutAgent {
	return &PersonaScoutAgent{LLMClient: client}
}

// DiscoverPersona scans README, manifests, and documentation to infer persona and glossary
func (p *PersonaScoutAgent) DiscoverPersona(projectRoot string) (*PersonaReport, error) {
	report := &PersonaReport{
		BrandLexicon:     make([]string, 0),
		LocalesSuggested: []string{"es", "fr", "de", "ja"},
		RecommendedTone:  "neutral",
		ConfidenceScore:  0.85,
	}

	// 1. Gather raw text signals
	var textCorpus strings.Builder
	projectName := filepath.Base(projectRoot)

	// Check package.json
	pkgPath := filepath.Join(projectRoot, "package.json")
	if data, err := os.ReadFile(pkgPath); err == nil {
		var pkg struct {
			Name        string   `json:"name"`
			Description string   `json:"description"`
			Keywords    []string `json:"keywords"`
		}
		if json.Unmarshal(data, &pkg) == nil {
			if pkg.Name != "" {
				projectName = pkg.Name
				report.BrandLexicon = append(report.BrandLexicon, pkg.Name)
			}
			textCorpus.WriteString(fmt.Sprintf("Package Name: %s\nDescription: %s\nKeywords: %s\n",
				pkg.Name, pkg.Description, strings.Join(pkg.Keywords, ", ")))
		}
	}

	// Check pubspec.yaml
	pubspecPath := filepath.Join(projectRoot, "pubspec.yaml")
	if data, err := os.ReadFile(pubspecPath); err == nil {
		if len(data) > 2000 {
			textCorpus.WriteString(string(data[:2000]) + "\n")
		} else {
			textCorpus.WriteString(string(data) + "\n")
		}
	}

	// Check README files
	readmeNames := []string{"README.md", "README", "readme.md", "README.markdown"}
	for _, rName := range readmeNames {
		rPath := filepath.Join(projectRoot, rName)
		if data, err := os.ReadFile(rPath); err == nil {
			snippet := string(data)
			if len(snippet) > 4000 {
				snippet = snippet[:4000]
			}
			textCorpus.WriteString("\n--- README ---\n" + snippet)
			break
		}
	}

	report.ProjectName = projectName
	corpusStr := textCorpus.String()

	// 2. If LLM is available, use agentic reasoning to synthesize brand lexicon & tone
	if p.LLMClient != nil && len(corpusStr) > 20 {
		ctx := context.Background()
		systemPrompt := `You are langPeanut Persona Scout Agent.
Analyze the provided codebase metadata and README documentation to extract brand identity.
Return ONLY valid JSON matching this exact schema:
{
  "brand_lexicon": ["BrandName", "ProductFeature", "CLICommand"],
  "recommended_tone": "neutral" | "casual" | "corporate" | "technical" | "genz",
  "audience": "Brief target audience description",
  "summary": "1-sentence brand summary",
  "locales_suggested": ["es", "fr", "de", "ja"]
}`

		userPrompt := fmt.Sprintf("Project Name: %s\nRepository Documentation & Metadata:\n%s", projectName, corpusStr)
		out, err := p.LLMClient.Complete(ctx, systemPrompt, userPrompt)
		if err == nil {
			cleanOut := strings.TrimSpace(out)
			if strings.HasPrefix(cleanOut, "```") {
				lines := strings.Split(cleanOut, "\n")
				if len(lines) > 2 {
					cleanOut = strings.Join(lines[1:len(lines)-1], "\n")
				}
			}
			var aiReport PersonaReport
			if json.Unmarshal([]byte(cleanOut), &aiReport) == nil {
				if len(aiReport.BrandLexicon) > 0 {
					report.BrandLexicon = deduplicateStrings(append(report.BrandLexicon, aiReport.BrandLexicon...))
				}
				if aiReport.RecommendedTone != "" {
					report.RecommendedTone = aiReport.RecommendedTone
				}
				if aiReport.Audience != "" {
					report.Audience = aiReport.Audience
				}
				if aiReport.Summary != "" {
					report.Summary = aiReport.Summary
				}
				if len(aiReport.LocalesSuggested) > 0 {
					report.LocalesSuggested = aiReport.LocalesSuggested
				}
				report.ConfidenceScore = 0.95
				return report, nil
			}
		}
	}

	// 3. Fallback Heuristic Rule-Based Mining
	report.BrandLexicon = deduplicateStrings(append(report.BrandLexicon, extractBrandHeuristics(projectName, corpusStr)...))
	report.RecommendedTone = inferToneHeuristics(corpusStr)
	report.Audience = "Software Developers & End Users"
	report.Summary = fmt.Sprintf("Autonomous localization strategy for %s", projectName)

	return report, nil
}

func deduplicateStrings(list []string) []string {
	seen := make(map[string]bool)
	var res []string
	for _, item := range list {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" && !seen[strings.ToLower(trimmed)] {
			seen[strings.ToLower(trimmed)] = true
			res = append(res, trimmed)
		}
	}
	return res
}

func extractBrandHeuristics(projectName, corpus string) []string {
	var brands []string
	if projectName != "" {
		brands = append(brands, projectName)
	}

	// Extract PascalCase/CamelCase keywords and CLI flags
	re := regexp.MustCompile(`\b[A-Z][a-zA-Z0-9]+[A-Z][a-zA-Z0-9]*\b`)
	matches := re.FindAllString(corpus, 20)
	for _, m := range matches {
		if len(m) > 3 && !isCommonEnglishWord(m) {
			brands = append(brands, m)
		}
	}
	return brands
}

func isCommonEnglishWord(w string) bool {
	common := map[string]bool{
		"JavaScript": false, "TypeScript": false, "React": false, "Next": false,
		"Flutter": false, "GitHub": false, "Boolean": true, "String": true, "Array": true,
	}
	v, ok := common[w]
	if ok {
		return v
	}
	return false
}

func inferToneHeuristics(corpus string) string {
	lower := strings.ToLower(corpus)
	if strings.Contains(lower, "enterprise") || strings.Contains(lower, "compliance") || strings.Contains(lower, "b2b") {
		return "corporate"
	}
	if strings.Contains(lower, "cli") || strings.Contains(lower, "sdk") || strings.Contains(lower, "api") || strings.Contains(lower, "ast") {
		return "technical"
	}
	if strings.Contains(lower, "fun") || strings.Contains(lower, "game") || strings.Contains(lower, "vibes") {
		return "casual"
	}
	return "neutral"
}
