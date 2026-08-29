package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/langPeanut/langPeanut/pkg/llm"
	"github.com/langPeanut/langPeanut/pkg/platforms"
	"github.com/langPeanut/langPeanut/pkg/types"
)

// ContextAgent performs semantic disambiguation, dynamic tag judgment, and contextual key naming
type ContextAgent struct {
	LLM llm.Client
}

func NewContextAgent() *ContextAgent {
	return &ContextAgent{
		LLM: llm.AutoDetectClient(),
	}
}

func NewContextAgentWithClient(client llm.Client) *ContextAgent {
	return &ContextAgent{
		LLM: client,
	}
}

// DisambiguationMap handles common polysemous words
var domainKeywords = map[string]string{
	"flight":   "travel",
	"ticket":   "travel",
	"airline":  "travel",
	"hotel":    "travel",
	"room":     "travel",
	"cart":     "ecommerce",
	"checkout": "ecommerce",
	"price":    "ecommerce",
	"order":    "ecommerce",
	"product":  "ecommerce",
	"author":   "library",
	"chapter":  "library",
	"isbn":     "library",
	"page":     "library",
	"read":     "library",
}

// ElementProfile summarizes discovered components, tags, attributes, and patterns
type ElementProfile struct {
	Components     []string `json:"components"`
	Attributes     []string `json:"attributes"`
	VariableShapes []string `json:"variable_shapes"`
	TotalSamples   int      `json:"total_samples"`
}

// LLMJudgmentResponse maps candidate IDs to agentic classifications and semantic keys
type LLMJudgmentResponse struct {
	Decisions map[string]CandidateDecision `json:"decisions"`
	TagRules  map[string]string            `json:"tag_rules,omitempty"`
}

type CandidateDecision struct {
	Classification string  `json:"classification"` // "LOCALIZABLE" or "SKIP"
	Key            string  `json:"key"`
	Explanation    string  `json:"explanation"`
	Confidence     float64 `json:"confidence"`
}

// EnhanceFast runs instant (<1ms) deterministic rule-based semantic inference and noise filtering without blocking on remote LLMs
func (ca *ContextAgent) EnhanceFast(candidates []types.StringCandidate) []types.StringCandidate {
	if len(candidates) == 0 {
		return candidates
	}

	byFile := make(map[string][]int)
	for i, c := range candidates {
		byFile[c.FilePath] = append(byFile[c.FilePath], i)
	}

	for filePath, indices := range byFile {
		var fileSiblings []string
		for _, idx := range indices {
			fileSiblings = append(fileSiblings, candidates[idx].CleanValue)
		}

		domain := ca.inferDomain(filePath, fileSiblings)

		for _, idx := range indices {
			c := &candidates[idx]
			c.SiblingStrings = fileSiblings

			if ca.isCodeArtifactOrIdentifier(c) {
				c.Classification = types.ClassSkip
				c.Approved = false
				c.Explanation = "Filtered: internal code identifier, CSS, or dynamic key expression"
				continue
			}

			lower := strings.ToLower(c.CleanValue)
			switch lower {
			case "book":
				if domain == "travel" {
					c.Key = "reserveFlightBtn"
					c.Explanation = "Disambiguated 'Book' as verb (reserve ticket) in travel context"
					c.Confidence = 0.99
				} else if domain == "library" {
					c.Key = "readingBookTitle"
					c.Explanation = "Disambiguated 'Book' as noun (physical book) in library context"
					c.Confidence = 0.99
				}
			case "cancel":
				c.Key = "cancelActionBtn"
				c.Confidence = 0.95
			case "save":
				c.Key = "saveChangesBtn"
				c.Confidence = 0.95
			}

			componentName := getComponentNameFromFile(c.FilePath)
			if !strings.HasPrefix(strings.ToLower(c.Key), strings.ToLower(componentName)) && len(c.Key) < 15 {
				c.Key = platforms.ToCamelCase(fmt.Sprintf("%s %s", componentName, c.Key))
			}
		}
	}

	return candidates
}

// DisambiguateAndEnhance processes candidate strings and improves key naming and classification
func (ca *ContextAgent) DisambiguateAndEnhance(candidates []types.StringCandidate) ([]types.StringCandidate, error) {
	if len(candidates) == 0 {
		return candidates, nil
	}

	// 1. Group candidates by file to aggregate sibling strings
	byFile := make(map[string][]int)
	for i, c := range candidates {
		byFile[c.FilePath] = append(byFile[c.FilePath], i)
	}

	// 2. Extract Element & Tag Profile across the project
	profile := ca.profileCodebase(candidates)

	// 3. Attempt LLM-driven Semantic Judgment if a live LLM is configured
	var llmDecisions map[string]CandidateDecision
	if ca.LLM != nil && ca.LLM.Name() != llm.ProviderLocal {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		llmDecisions, _ = ca.judgeWithLLM(ctx, candidates, profile)
	}

	// 4. Apply LLM Decisions or Deterministic Rule Inference
	for filePath, indices := range byFile {
		var fileSiblings []string
		for _, idx := range indices {
			fileSiblings = append(fileSiblings, candidates[idx].CleanValue)
		}

		domain := ca.inferDomain(filePath, fileSiblings)

		for _, idx := range indices {
			c := &candidates[idx]
			c.SiblingStrings = fileSiblings

			// Check if LLM made an explicit judgment for this candidate
			if decision, found := llmDecisions[c.ID]; found {
				if decision.Classification == "SKIP" || decision.Classification == "NON_LOCALIZABLE" {
					c.Classification = types.ClassSkip
					c.Approved = false
				} else if decision.Classification == "LOCALIZABLE" {
					c.Classification = types.ClassLocalizable
					c.Approved = true
				}
				if decision.Key != "" {
					c.Key = platforms.ToCamelCase(decision.Key)
				}
				if decision.Explanation != "" {
					c.Explanation = decision.Explanation
				}
				if decision.Confidence > 0 {
					c.Confidence = decision.Confidence
				}
				continue
			}

			// Fallback Deterministic Rule-Based Semantic Inference:
			// A. Filter code identifiers, keys, CSS classes, and template interpolation noise like `key: ${key}`
			if ca.isCodeArtifactOrIdentifier(c) {
				c.Classification = types.ClassSkip
				c.Approved = false
				c.Explanation = "Filtered: internal code identifier, CSS, or dynamic key expression"
				continue
			}

			// B. Contextual disambiguation for short polysemous words
			lower := strings.ToLower(c.CleanValue)
			switch lower {
			case "book":
				if domain == "travel" {
					c.Key = "reserveFlightBtn"
					c.Explanation = "Disambiguated 'Book' as verb (reserve ticket) in travel context"
					c.Confidence = 0.99
				} else if domain == "library" {
					c.Key = "readingBookTitle"
					c.Explanation = "Disambiguated 'Book' as noun (physical book) in library context"
					c.Confidence = 0.99
				} else {
					c.Key = "bookAction"
				}
			case "order":
				if domain == "ecommerce" {
					c.Key = "placeOrderBtn"
					c.Explanation = "Disambiguated 'Order' as commerce checkout action"
				}
			case "back":
				c.Key = "goBackBtn"
			case "save":
				c.Key = "saveChangesBtn"
			case "close":
				c.Key = "closeModalBtn"
			}

			// C. Ensure key uniqueness within the file/context
			componentName := getComponentNameFromFile(c.FilePath)
			if !strings.HasPrefix(strings.ToLower(c.Key), strings.ToLower(componentName)) && len(c.Key) < 15 {
				c.Key = platforms.ToCamelCase(fmt.Sprintf("%s %s", componentName, c.Key))
			}
		}
	}

	return candidates, nil
}

// profileCodebase collects distinct components, attributes, and variable patterns
func (ca *ContextAgent) profileCodebase(candidates []types.StringCandidate) ElementProfile {
	componentSet := make(map[string]bool)
	attrSet := make(map[string]bool)
	varShapeSet := make(map[string]bool)

	for _, c := range candidates {
		if c.ParentNodeType != "" {
			if strings.HasPrefix(c.ParentNodeType, "JSXAttribute(") {
				attr := strings.TrimSuffix(strings.TrimPrefix(c.ParentNodeType, "JSXAttribute("), ")")
				attrSet[attr] = true
			} else {
				componentSet[c.ParentNodeType] = true
			}
		}
		if len(c.Variables) > 0 {
			varShapeSet[strings.Join(c.Variables, ", ")] = true
		}
	}

	var components, attrs, varShapes []string
	for comp := range componentSet {
		components = append(components, comp)
	}
	for attr := range attrSet {
		attrs = append(attrs, attr)
	}
	for vs := range varShapeSet {
		varShapes = append(varShapes, vs)
	}

	return ElementProfile{
		Components:     components,
		Attributes:     attrs,
		VariableShapes: varShapes,
		TotalSamples:   len(candidates),
	}
}

// judgeWithLLM prompts the active LLM agent to classify candidates and generate semantic keys
func (ca *ContextAgent) judgeWithLLM(ctx context.Context, candidates []types.StringCandidate, profile ElementProfile) (map[string]CandidateDecision, error) {
	// Sample up to 50 candidates for prompt efficiency
	sampleCount := min(50, len(candidates))
	type sampleItem struct {
		ID       string `json:"id"`
		Tag      string `json:"tag"`
		Raw      string `json:"raw"`
		Clean    string `json:"clean"`
		Context  string `json:"context"`
	}

	var samples []sampleItem
	for i := 0; i < sampleCount; i++ {
		c := candidates[i]
		samples = append(samples, sampleItem{
			ID:      c.ID,
			Tag:     c.ParentNodeType,
			Raw:     c.RawValue,
			Clean:   c.CleanValue,
			Context: c.ContextHint,
		})
	}

	sampleBytes, _ := json.MarshalIndent(samples, "", "  ")

	systemPrompt := `You are the langPeanut Context & Element Judgment Agent.
Your task is to analyze extracted string candidates and AST tags/attributes from a software codebase, and decide whether each string is a user-facing UI text (LOCALIZABLE) or non-UI code construct (SKIP).

Rules for Judgment:
1. LOCALIZABLE:
   - Button labels, screen titles, headings, error messages, user notifications, tooltips, placeholders, modal text, badge labels.
   - Text with user variables (e.g. "Welcome back, {name}!").
2. SKIP (Non-UI):
   - Internal identifiers, keys (e.g. key: ${key}, id="foo", name="field"), CSS classes, SVG paths ("M 0 0 L ...").
   - URLs, query params, routing endpoints, file paths, extensions (.png, .svg).
   - CLI commands (git clone, flutter run, npm install).
3. KEY CONVENTIONS:
   - Provide a descriptive camelCase semantic key (e.g. "submitBugReportBtn", "downloadTitle").

Return ONLY a valid JSON object matching this schema:
{
  "decisions": {
    "<candidate_id>": {
      "classification": "LOCALIZABLE" or "SKIP",
      "key": "descriptiveCamelCaseKey",
      "explanation": "Brief reasoning",
      "confidence": 0.98
    }
  }
}`

	userPrompt := fmt.Sprintf("Codebase Element Profile:\nAttributes: %v\nComponents: %v\n\nCandidate Samples:\n%s",
		profile.Attributes, profile.Components, string(sampleBytes))

	resp, err := ca.LLM.Complete(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	// Clean code fence blocks if returned
	resp = strings.TrimSpace(resp)
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimPrefix(resp, "```")
	resp = strings.TrimSuffix(resp, "```")
	resp = strings.TrimSpace(resp)

	var judgment LLMJudgmentResponse
	if err := json.Unmarshal([]byte(resp), &judgment); err != nil {
		return nil, err
	}

	return judgment.Decisions, nil
}

// isCodeArtifactOrIdentifier checks for code noise like `key: ${key}`, CSS classes, and SVG paths
func (ca *ContextAgent) isCodeArtifactOrIdentifier(c *types.StringCandidate) bool {
	raw := strings.TrimSpace(c.RawValue)
	clean := strings.TrimSpace(c.CleanValue)

	// Filter key expressions: `key: ${key}`, `key:${id}`, `${key}`
	keyPattern := regexp.MustCompile(`(?i)(?:^|[\s{,])key\s*:\s*\$\{`)
	if keyPattern.MatchString(raw) || keyPattern.MatchString(clean) {
		return true
	}

	// Filter single variable tokens or code identifiers without letters
	if strings.HasPrefix(clean, "{") && strings.HasSuffix(clean, "}") && len(c.Variables) == 1 {
		return true
	}

	// Filter SVG commands and coordinates
	if strings.Contains(clean, " L ") || strings.Contains(clean, " Z") || strings.Contains(clean, " M ") {
		return true
	}

	// Filter CLI commands
	if strings.HasPrefix(clean, "git ") || strings.HasPrefix(clean, "flutter ") || strings.HasPrefix(clean, "npm ") ||
		strings.HasPrefix(clean, "cargo ") || strings.HasPrefix(clean, "cd ") || strings.HasPrefix(clean, "pip ") {
		return true
	}

	// Filter Markdown templates
	if strings.HasPrefix(clean, "### ") || strings.HasPrefix(clean, "## ") || strings.HasPrefix(clean, "# ") {
		return true
	}

	return false
}

func (ca *ContextAgent) inferDomain(filePath string, siblings []string) string {
	lowerPath := strings.ToLower(filePath)
	for kw, domain := range domainKeywords {
		if strings.Contains(lowerPath, kw) {
			return domain
		}
	}

	// Check siblings
	for _, s := range siblings {
		lowerS := strings.ToLower(s)
		for kw, domain := range domainKeywords {
			if strings.Contains(lowerS, kw) {
				return domain
			}
		}
	}

	return "general"
}

func getComponentNameFromFile(filePath string) string {
	base := filepath.Base(filePath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	name = strings.ReplaceAll(name, ".screen", "")
	name = strings.ReplaceAll(name, ".view", "")
	name = strings.ReplaceAll(name, "_screen", "")
	name = strings.ReplaceAll(name, "_view", "")
	name = strings.ReplaceAll(name, "_modal", "")
	return platforms.ToCamelCase(name)
}
