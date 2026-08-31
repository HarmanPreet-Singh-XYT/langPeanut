package seo

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/langPeanut/langPeanut/pkg/llm"
	"github.com/langPeanut/langPeanut/pkg/logger"
)

// SemanticCopyWeaverAgent weaves target SEO keywords into extracted AST keys
type SemanticCopyWeaverAgent struct {
	LLMClient llm.Client
}

// NewSemanticCopyWeaverAgent creates a new SemanticCopyWeaverAgent
func NewSemanticCopyWeaverAgent(client llm.Client) *SemanticCopyWeaverAgent {
	return &SemanticCopyWeaverAgent{LLMClient: client}
}

// WeaveTranslations takes raw baseline translations and enhances them with SEO keywords
func (w *SemanticCopyWeaverAgent) WeaveTranslations(
	ctx context.Context,
	strategy *SEOStrategy,
	locale string,
	sourceKeys map[string]string, // key -> en source text
	baselineTranslations map[string]string, // key -> baseline translation
	keywords []KeywordInsight,
) ([]KeyOptimization, error) {
	logger.Get().Info("SEO:WEAVER", fmt.Sprintf("Weaving SEO keywords into %d keys for locale [%s]", len(sourceKeys), locale))

	// 1. Filter and classify keys based on ScopeTier
	targetKeys := make(map[string]string)
	for k, src := range sourceKeys {
		impact := ClassifyKeyImpact(k)
		switch strategy.ScopeTier {
		case ScopeFullSite:
			targetKeys[k] = src
		case ScopeCustom:
			for _, allowed := range strategy.CustomKeyList {
				if k == allowed {
					targetKeys[k] = src
					break
				}
			}
		default: // ScopeHighImpact
			if impact == "high" {
				targetKeys[k] = src
			}
		}
	}

	// If no high-impact keys found, optimize all available keys
	if len(targetKeys) == 0 {
		targetKeys = sourceKeys
	}

	// 2. If LLM is available, perform AI semantic weaving
	if w.LLMClient != nil {
		if opts, err := w.aiSemanticWeave(ctx, strategy, locale, targetKeys, baselineTranslations, keywords); err == nil && len(opts) > 0 {
			logger.Get().Success("SEO:WEAVER", fmt.Sprintf("Successfully SEO-optimized %d keys via AI for [%s]", len(opts), locale))
			return opts, nil
		}
	}

	// 3. Fallback to heuristic rule-based keyword weaving
	opts := w.heuristicSemanticWeave(strategy, locale, targetKeys, baselineTranslations, keywords)
	logger.Get().Success("SEO:WEAVER", fmt.Sprintf("Applied heuristic SEO optimization on %d keys for [%s]", len(opts), locale))
	return opts, nil
}

func (w *SemanticCopyWeaverAgent) aiSemanticWeave(
	ctx context.Context,
	strategy *SEOStrategy,
	locale string,
	targetKeys map[string]string,
	baselineTranslations map[string]string,
	keywords []KeywordInsight,
) ([]KeyOptimization, error) {
	// Build compact keyword context
	var kwList []string
	for _, kw := range keywords {
		kwList = append(kwList, fmt.Sprintf("%s (%s intent, %d vol)", kw.Keyword, kw.Intent, kw.EstMonthlyVolume))
	}

	// Build key payload
	type KeyItem struct {
		Key      string `json:"key"`
		SourceEn string `json:"source_en"`
		Baseline string `json:"baseline"`
	}
	var items []KeyItem
	for k, src := range targetKeys {
		base := baselineTranslations[k]
		if base == "" {
			base = src
		}
		items = append(items, KeyItem{Key: k, SourceEn: src, Baseline: base})
	}
	itemsJSON, _ := json.MarshalIndent(items, "", "  ")

	systemPrompt := fmt.Sprintf(`You are langPeanut Semantic Copy Weaver Agent for locale "%s".
You are an expert native multilingual copywriter and search engine marketing specialist.
Your goal is to optimize software UI and landing page translation keys to maximize organic search visibility, click-through-rates, and local market conversion.

TARGET KEYWORDS:
%s

MANDATORY RULES:
1. WEAVE KEYWORDS NATURALLY: Integrate target search keywords smoothly into titles, headlines, and descriptions without awkward keyword stuffing or stiff phrasing.
2. PRESERVE ICU INVARIANTS: You MUST retain all ICU format placeholders and variables ({count}, {name}, {0}, %%s, %%d) EXACTLY as they appear in the source.
3. CONVERSION PUNCHINESS: Keep copy compelling, authoritative, and native to local buyer psychology in "%s".
4. GENERATE RATIONALE: Provide a clear 1-sentence rationale explaining the keyword insertion and search intent impact.

Return ONLY a valid JSON array matching this exact schema:
[
  {
    "key": "key_name",
    "optimized_translation": "SEO-enriched native copy",
    "injected_keywords": ["keyword1", "keyword2"],
    "rationale": "Explanation of search volume capture and tone preservation"
  }
]`, locale, strings.Join(kwList, "\n"), locale)

	userPrompt := fmt.Sprintf("Product: %s\nCategory: %s\nGoal: %s\n\nKeys to SEO-Optimize:\n%s",
		strategy.ProjectName, strategy.Category, strategy.Goal, string(itemsJSON))

	out, err := w.LLMClient.Complete(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	cleanOut := ExtractJSONArray(out)
	if cleanOut == "" {
		cleanOut = strings.TrimSpace(out)
	}

	type AIResult struct {
		Key                  string   `json:"key"`
		OptimizedTranslation string   `json:"optimized_translation"`
		InjectedKeywords     []string `json:"injected_keywords"`
		Rationale            string   `json:"rationale"`
	}

	var results []AIResult
	if err := json.Unmarshal([]byte(cleanOut), &results); err != nil {
		return nil, err
	}

	var optimizations []KeyOptimization
	resMap := make(map[string]AIResult)
	for _, r := range results {
		resMap[r.Key] = r
	}

	for k, src := range targetKeys {
		base := baselineTranslations[k]
		if base == "" {
			base = src
		}

		optText := base
		injected := []string{}
		rationale := "Preserved native baseline phrasing"

		if ai, ok := resMap[k]; ok && ai.OptimizedTranslation != "" {
			optText = ai.OptimizedTranslation
			injected = ai.InjectedKeywords
			rationale = ai.Rationale
		}

		icuOK := validateICUVariables(src, optText)
		charLen := len([]rune(optText))
		pixelW := EstimatePixelWidth(optText, locale)
		isTrunc := pixelW > 600 && strings.Contains(strings.ToLower(k), "title")

		optimizations = append(optimizations, KeyOptimization{
			Key:                  k,
			Locale:               locale,
			SourceEn:             src,
			BaselineTranslation:  base,
			OptimizedTranslation: optText,
			InjectedKeywords:     injected,
			Rationale:            rationale,
			ImpactTier:           ClassifyKeyImpact(k),
			CharacterLength:      charLen,
			PixelWidthDesktop:    pixelW,
			IsTitleTruncated:     isTrunc,
			ICUVariablesMatched:  icuOK,
		})
	}

	return optimizations, nil
}

func (w *SemanticCopyWeaverAgent) heuristicSemanticWeave(
	strategy *SEOStrategy,
	locale string,
	targetKeys map[string]string,
	baselineTranslations map[string]string,
	keywords []KeywordInsight,
) []KeyOptimization {
	var optimizations []KeyOptimization
	lang := strings.ToLower(strings.Split(locale, "-")[0])

	primaryKw := strategy.Category
	if len(keywords) > 0 {
		primaryKw = keywords[0].Keyword
	}

	for k, src := range targetKeys {
		base := baselineTranslations[k]
		if base == "" {
			base = src
		}

		optText := base
		injected := []string{}
		rationale := "Standard linguistic baseline"
		impact := ClassifyKeyImpact(k)

		kLower := strings.ToLower(k)
		if strings.Contains(kLower, "title") || strings.Contains(kLower, "hero") || strings.Contains(kLower, "meta") || strings.Contains(kLower, "desc") {
			switch lang {
			case "ja":
				if strings.Contains(kLower, "title") || strings.Contains(kLower, "hero.title") {
					if !strings.Contains(base, primaryKw) {
						optText = fmt.Sprintf("%s ｜ %s", base, primaryKw)
						injected = []string{primaryKw}
						rationale = fmt.Sprintf("検索ボリューム上位のキーワード「%s」をタイトル末尾に最適配置", primaryKw)
					}
				} else if strings.Contains(kLower, "desc") || strings.Contains(kLower, "description") {
					if !strings.Contains(base, primaryKw) {
						optText = fmt.Sprintf("%s %sに対応し、高い生産性と信頼性を提供します。", base, primaryKw)
						injected = []string{primaryKw}
						rationale = fmt.Sprintf("スニペット説明文に「%s」を自然に補完", primaryKw)
					}
				}
			case "de":
				if strings.Contains(kLower, "title") || strings.Contains(kLower, "hero.title") {
					if !strings.Contains(base, primaryKw) {
						optText = fmt.Sprintf("%s | %s", base, primaryKw)
						injected = []string{primaryKw}
						rationale = fmt.Sprintf("Optimiert mit Fokus-Keyword '%s' für deutsche Suchergebnisse", primaryKw)
					}
				} else if strings.Contains(kLower, "desc") || strings.Contains(kLower, "description") {
					if !strings.Contains(base, primaryKw) {
						optText = fmt.Sprintf("%s — Entdecken Sie die führende Lösung für %s.", base, primaryKw)
						injected = []string{primaryKw}
						rationale = fmt.Sprintf("Meta-Beschreibung mit Keyword '%s' angereichert", primaryKw)
					}
				}
			case "es":
				if strings.Contains(kLower, "title") || strings.Contains(kLower, "hero.title") {
					if !strings.Contains(base, primaryKw) {
						optText = fmt.Sprintf("%s — %s", base, primaryKw)
						injected = []string{primaryKw}
						rationale = fmt.Sprintf("Título enriquecido con término de búsqueda clave '%s'", primaryKw)
					}
				} else if strings.Contains(kLower, "desc") || strings.Contains(kLower, "description") {
					if !strings.Contains(base, primaryKw) {
						optText = fmt.Sprintf("%s. La plataforma líder en %s.", base, primaryKw)
						injected = []string{primaryKw}
						rationale = fmt.Sprintf("Snippet descriptivo integrado con '%s'", primaryKw)
					}
				}
			default:
				if strings.Contains(kLower, "title") || strings.Contains(kLower, "hero.title") {
					if !strings.Contains(base, primaryKw) {
						optText = fmt.Sprintf("%s | %s", base, primaryKw)
						injected = []string{primaryKw}
						rationale = fmt.Sprintf("Enriched headline with primary search query '%s'", primaryKw)
					}
				} else if strings.Contains(kLower, "desc") || strings.Contains(kLower, "description") {
					if !strings.Contains(base, primaryKw) {
						optText = fmt.Sprintf("%s Discover the leading platform for %s.", base, primaryKw)
						injected = []string{primaryKw}
						rationale = fmt.Sprintf("Integrated target keyword '%s' into description", primaryKw)
					}
				}
			}
		}

		// Ensure ICU placeholders were not lost during weaving
		if !validateICUVariables(src, optText) {
			optText = base // Revert to base if ICU invariants broken
			injected = []string{}
			rationale = "Preserved baseline to safeguard ICU variables"
		}

		charLen := len([]rune(optText))
		pixelW := EstimatePixelWidth(optText, locale)
		isTrunc := pixelW > 600 && strings.Contains(kLower, "title")

		optimizations = append(optimizations, KeyOptimization{
			Key:                  k,
			Locale:               locale,
			SourceEn:             src,
			BaselineTranslation:  base,
			OptimizedTranslation: optText,
			InjectedKeywords:     injected,
			Rationale:            rationale,
			ImpactTier:           impact,
			CharacterLength:      charLen,
			PixelWidthDesktop:    pixelW,
			IsTitleTruncated:     isTrunc,
			ICUVariablesMatched:  validateICUVariables(src, optText),
		})
	}

	return optimizations
}

// ClassifyKeyImpact categorizes keys into High-Impact SEO Tier vs Standard UI Tier
func ClassifyKeyImpact(key string) string {
	lower := strings.ToLower(key)
	highIndicators := []string{
		"meta", "title", "hero", "header", "heading", "h1", "h2", "h3",
		"desc", "feature", "pricing", "faq", "subtitle", "slogan", "tagline",
	}
	for _, ind := range highIndicators {
		if strings.Contains(lower, ind) {
			return "high"
		}
	}
	return "standard"
}

// EstimatePixelWidth calculates approximate rendered pixel width in Google SERP font (Arial 18px / 20px)
func EstimatePixelWidth(text string, locale string) int {
	isCJK := strings.HasPrefix(locale, "ja") || strings.HasPrefix(locale, "zh") || strings.HasPrefix(locale, "ko")
	total := 0
	for _, r := range text {
		if isCJK && r > 0x2E80 {
			total += 18 // Full-width CJK character ~18px
		} else if r >= 'A' && r <= 'Z' {
			total += 13 // Uppercase Latin ~13px
		} else if r == 'i' || r == 'l' || r == 'j' || r == '.' || r == ' ' {
			total += 5  // Narrow glyphs
		} else if r == 'm' || r == 'w' || r == 'M' || r == 'W' {
			total += 16 // Wide glyphs
		} else {
			total += 10 // Average Latin glyph ~10px
		}
	}
	return total
}

func validateICUVariables(source, target string) bool {
	re := regexp.MustCompile(`\{[a-zA-Z0-9_]+\}|%[0-9]*[a-zA-Z]`)
	srcVars := re.FindAllString(source, -1)
	for _, v := range srcVars {
		if !strings.Contains(target, v) {
			return false
		}
	}
	return true
}
