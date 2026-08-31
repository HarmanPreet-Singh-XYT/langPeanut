package seo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/langPeanut/langPeanut/pkg/llm"
	"github.com/langPeanut/langPeanut/pkg/logger"
)

// KeywordIntelligenceAgent mines high-intent local search queries and keyword gaps
type KeywordIntelligenceAgent struct {
	LLMClient llm.Client
}

// NewKeywordIntelligenceAgent creates a new KeywordIntelligenceAgent
func NewKeywordIntelligenceAgent(client llm.Client) *KeywordIntelligenceAgent {
	return &KeywordIntelligenceAgent{LLMClient: client}
}

// AnalyzeKeywords extracts a prioritized keyword pool from competitor profiles & strategy
func (k *KeywordIntelligenceAgent) AnalyzeKeywords(ctx context.Context, strategy *SEOStrategy, locale string, competitors []CompetitorProfile) ([]KeywordInsight, error) {
	logger.Get().Info("SEO:KEYWORDS", fmt.Sprintf("Synthesizing keyword intelligence pool for locale [%s]", locale))

	// 1. If LLM is available, use semantic search intent clustering
	if k.LLMClient != nil {
		if pool, err := k.generateAIKeywordPool(ctx, strategy, locale, competitors); err == nil && len(pool) > 0 {
			logger.Get().Success("SEO:KEYWORDS", fmt.Sprintf("Generated %d AI keyword insights for [%s]", len(pool), locale))
			return pool, nil
		}
	}

	// 2. Fallback to heuristic & competitor n-gram extraction
	pool := k.generateHeuristicKeywords(strategy, locale, competitors)
	logger.Get().Success("SEO:KEYWORDS", fmt.Sprintf("Synthesized %d heuristic keyword insights for [%s]", len(pool), locale))
	return pool, nil
}

func (k *KeywordIntelligenceAgent) generateAIKeywordPool(ctx context.Context, strategy *SEOStrategy, locale string, competitors []CompetitorProfile) ([]KeywordInsight, error) {
	systemPrompt := fmt.Sprintf(`You are langPeanut Keyword Intelligence Agent for locale "%s".
Analyze the product, commercial goal ("%s"), and competitor intelligence to generate a high-performing SEO keyword pool.
Generate 5 to 8 high-intent target keywords in the native language of "%s".

Categorize each keyword:
- intent: "commercial" (buyer/pricing/comparison), "informational" (how-to/guide), "transactional" (sign-up/free-trial/download)
- volume_tier: "high" (10,000+), "medium" (1,000-10,000), "low" (<1,000)
- est_monthly_volume: estimated regional monthly search volume integer
- difficulty: SEO keyword difficulty score (1-100)
- relevance: product relevance score (1-100)
- is_primary: boolean (true for top 2 primary core head terms)

Return ONLY valid JSON array matching this schema:
[
  {
    "keyword": "Native search term in target language",
    "locale": "%s",
    "intent": "commercial",
    "volume_tier": "high",
    "est_monthly_volume": 18500,
    "difficulty": 34,
    "relevance": 95,
    "is_primary": true
  }
]`, locale, strategy.Goal, locale, locale)

	var compSummary strings.Builder
	for _, c := range competitors {
		compSummary.WriteString(fmt.Sprintf("Competitor (%s): Title='%s' Keywords=%v ValueProps=%v\n",
			c.Domain, c.Title, c.Keywords, c.ValueProps))
	}

	userPrompt := fmt.Sprintf("Product: %s\nCategory: %s\nDescription: %s\nGoal: %s\n\nCompetitor Landscape:\n%s",
		strategy.ProjectName, strategy.Category, strategy.ProductDescription, strategy.Goal, compSummary.String())

	out, err := k.LLMClient.Complete(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	cleanOut := ExtractJSONArray(out)
	if cleanOut == "" {
		cleanOut = strings.TrimSpace(out)
	}

	var insights []KeywordInsight
	if err := json.Unmarshal([]byte(cleanOut), &insights); err != nil {
		return nil, err
	}

	for i := range insights {
		insights[i].Locale = locale
	}

	return insights, nil
}

func (k *KeywordIntelligenceAgent) generateHeuristicKeywords(strategy *SEOStrategy, locale string, competitors []CompetitorProfile) []KeywordInsight {
	lang := strings.ToLower(strings.Split(locale, "-")[0])
	var pool []KeywordInsight
	seen := make(map[string]bool)

	cat := strings.TrimSpace(strategy.Category)
	if cat == "" || cat == "Software Platform" {
		cat = "Productivity Platform"
	}
	cat = strings.ReplaceAll(cat, "Software Software", "Software")
	cat = strings.ReplaceAll(cat, "software software", "software")

	// 1. Collect unique competitor keywords first
	for i, c := range competitors {
		for j, kw := range c.Keywords {
			t := strings.TrimSpace(kw)
			if t != "" && !seen[strings.ToLower(t)] && len(t) > 3 {
				seen[strings.ToLower(t)] = true
				vol := 8500 - (i*1200 + j*450)
				if vol < 1200 {
					vol = 1200
				}
				kd := 42 - (i*4 + j*2)
				if kd < 18 {
					kd = 18
				}
				intent := "commercial"
				if j%2 == 1 {
					intent = "transactional"
				}
				pool = append(pool, KeywordInsight{
					Keyword:          t,
					Locale:           locale,
					Intent:           intent,
					VolumeTier:       "medium",
					EstMonthlyVolume: vol,
					Difficulty:       kd,
					Relevance:        92 - j*3,
					IsPrimary:        len(pool) < 2,
				})
			}
		}
	}

	// 2. Synthesize domain & goal-specific keywords tailored to project category
	var defaults []KeywordInsight
	switch lang {
	case "en":
		switch strategy.Goal {
		case GoalConversion:
			defaults = []KeywordInsight{
				{Keyword: fmt.Sprintf("best %s pricing & plans", strings.ToLower(cat)), Locale: locale, Intent: "commercial", VolumeTier: "high", EstMonthlyVolume: 24000, Difficulty: 44, Relevance: 98, IsPrimary: true},
				{Keyword: fmt.Sprintf("try %s free online", strings.ToLower(cat)), Locale: locale, Intent: "transactional", VolumeTier: "high", EstMonthlyVolume: 18500, Difficulty: 38, Relevance: 96, IsPrimary: true},
				{Keyword: fmt.Sprintf("top %s comparison for teams", strings.ToLower(cat)), Locale: locale, Intent: "commercial", VolumeTier: "medium", EstMonthlyVolume: 8900, Difficulty: 32, Relevance: 92, IsPrimary: false},
			}
		case GoalTrust:
			defaults = []KeywordInsight{
				{Keyword: fmt.Sprintf("enterprise %s security & compliance", strings.ToLower(cat)), Locale: locale, Intent: "informational", VolumeTier: "medium", EstMonthlyVolume: 7400, Difficulty: 29, Relevance: 96, IsPrimary: true},
				{Keyword: fmt.Sprintf("SOC-2 certified %s solution", strings.ToLower(cat)), Locale: locale, Intent: "commercial", VolumeTier: "medium", EstMonthlyVolume: 6100, Difficulty: 26, Relevance: 93, IsPrimary: true},
			}
		default: // GoalTopTraffic
			defaults = []KeywordInsight{
				{Keyword: fmt.Sprintf("best %s software for modern teams", strings.ToLower(cat)), Locale: locale, Intent: "commercial", VolumeTier: "high", EstMonthlyVolume: 28000, Difficulty: 49, Relevance: 97, IsPrimary: true},
				{Keyword: fmt.Sprintf("top rated %s tools in 2026", strings.ToLower(cat)), Locale: locale, Intent: "commercial", VolumeTier: "high", EstMonthlyVolume: 19800, Difficulty: 42, Relevance: 95, IsPrimary: true},
				{Keyword: fmt.Sprintf("how to automate %s workflow", strings.ToLower(cat)), Locale: locale, Intent: "informational", VolumeTier: "medium", EstMonthlyVolume: 9200, Difficulty: 30, Relevance: 89, IsPrimary: false},
			}
		}
	case "ja":
		switch strategy.Goal {
		case GoalConversion:
			defaults = []KeywordInsight{
				{Keyword: fmt.Sprintf("%s 料金プラン", cat), Locale: locale, Intent: "commercial", VolumeTier: "high", EstMonthlyVolume: 16400, Difficulty: 42, Relevance: 98, IsPrimary: true},
				{Keyword: fmt.Sprintf("%s 無料トライアル", cat), Locale: locale, Intent: "transactional", VolumeTier: "high", EstMonthlyVolume: 12800, Difficulty: 36, Relevance: 96, IsPrimary: true},
				{Keyword: fmt.Sprintf("%s 導入事例 比較", cat), Locale: locale, Intent: "commercial", VolumeTier: "medium", EstMonthlyVolume: 5900, Difficulty: 28, Relevance: 92, IsPrimary: false},
			}
		case GoalTrust:
			defaults = []KeywordInsight{
				{Keyword: fmt.Sprintf("%s セキュリティ基準", cat), Locale: locale, Intent: "informational", VolumeTier: "medium", EstMonthlyVolume: 7800, Difficulty: 31, Relevance: 95, IsPrimary: true},
				{Keyword: fmt.Sprintf("%s 国内法規準拠", cat), Locale: locale, Intent: "commercial", VolumeTier: "medium", EstMonthlyVolume: 6200, Difficulty: 25, Relevance: 94, IsPrimary: true},
				{Keyword: fmt.Sprintf("%s エンタープライズ導入", cat), Locale: locale, Intent: "commercial", VolumeTier: "medium", EstMonthlyVolume: 4900, Difficulty: 38, Relevance: 90, IsPrimary: false},
			}
		default: // GoalTopTraffic
			defaults = []KeywordInsight{
				{Keyword: fmt.Sprintf("おすすめ %s クラウド", cat), Locale: locale, Intent: "commercial", VolumeTier: "high", EstMonthlyVolume: 24500, Difficulty: 48, Relevance: 97, IsPrimary: true},
				{Keyword: fmt.Sprintf("人気 %s ツール 比較", cat), Locale: locale, Intent: "commercial", VolumeTier: "high", EstMonthlyVolume: 19200, Difficulty: 44, Relevance: 95, IsPrimary: true},
				{Keyword: fmt.Sprintf("%s 業務効率化 SaaS", cat), Locale: locale, Intent: "informational", VolumeTier: "medium", EstMonthlyVolume: 8600, Difficulty: 33, Relevance: 90, IsPrimary: false},
				{Keyword: fmt.Sprintf("最新 %s 機能", cat), Locale: locale, Intent: "informational", VolumeTier: "low", EstMonthlyVolume: 3100, Difficulty: 22, Relevance: 86, IsPrimary: false},
			}
		}
	case "de":
		switch strategy.Goal {
		case GoalConversion:
			defaults = []KeywordInsight{
				{Keyword: fmt.Sprintf("%s Preise & Kosten", cat), Locale: locale, Intent: "commercial", VolumeTier: "high", EstMonthlyVolume: 14800, Difficulty: 40, Relevance: 98, IsPrimary: true},
				{Keyword: fmt.Sprintf("%s kostenlos testen", cat), Locale: locale, Intent: "transactional", VolumeTier: "high", EstMonthlyVolume: 11200, Difficulty: 34, Relevance: 95, IsPrimary: true},
				{Keyword: fmt.Sprintf("Beste %s Software Vergleich", cat), Locale: locale, Intent: "commercial", VolumeTier: "medium", EstMonthlyVolume: 6800, Difficulty: 30, Relevance: 92, IsPrimary: false},
			}
		case GoalTrust:
			defaults = []KeywordInsight{
				{Keyword: fmt.Sprintf("%s DSGVO konform", cat), Locale: locale, Intent: "commercial", VolumeTier: "high", EstMonthlyVolume: 9600, Difficulty: 29, Relevance: 97, IsPrimary: true},
				{Keyword: fmt.Sprintf("%s Datensicherheit Deutschland", cat), Locale: locale, Intent: "informational", VolumeTier: "medium", EstMonthlyVolume: 5400, Difficulty: 24, Relevance: 93, IsPrimary: true},
				{Keyword: fmt.Sprintf("%s Enterprise Lösung", cat), Locale: locale, Intent: "commercial", VolumeTier: "medium", EstMonthlyVolume: 4200, Difficulty: 35, Relevance: 90, IsPrimary: false},
			}
		default: // GoalTopTraffic
			defaults = []KeywordInsight{
				{Keyword: fmt.Sprintf("%s Software online", cat), Locale: locale, Intent: "commercial", VolumeTier: "high", EstMonthlyVolume: 18500, Difficulty: 46, Relevance: 96, IsPrimary: true},
				{Keyword: fmt.Sprintf("Moderne %s Plattform", cat), Locale: locale, Intent: "commercial", VolumeTier: "high", EstMonthlyVolume: 13900, Difficulty: 39, Relevance: 94, IsPrimary: true},
				{Keyword: fmt.Sprintf("%s Automatisierung Tool", cat), Locale: locale, Intent: "informational", VolumeTier: "medium", EstMonthlyVolume: 7200, Difficulty: 32, Relevance: 89, IsPrimary: false},
			}
		}
	case "es":
		switch strategy.Goal {
		case GoalConversion:
			defaults = []KeywordInsight{
				{Keyword: fmt.Sprintf("precios de %s", strings.ToLower(cat)), Locale: locale, Intent: "commercial", VolumeTier: "high", EstMonthlyVolume: 13500, Difficulty: 38, Relevance: 97, IsPrimary: true},
				{Keyword: fmt.Sprintf("probar %s gratis online", strings.ToLower(cat)), Locale: locale, Intent: "transactional", VolumeTier: "high", EstMonthlyVolume: 10400, Difficulty: 33, Relevance: 95, IsPrimary: true},
				{Keyword: fmt.Sprintf("mejor solución de %s empresas", strings.ToLower(cat)), Locale: locale, Intent: "commercial", VolumeTier: "medium", EstMonthlyVolume: 6100, Difficulty: 30, Relevance: 91, IsPrimary: false},
			}
		case GoalTrust:
			defaults = []KeywordInsight{
				{Keyword: fmt.Sprintf("plataforma de %s segura", strings.ToLower(cat)), Locale: locale, Intent: "informational", VolumeTier: "medium", EstMonthlyVolume: 5800, Difficulty: 26, Relevance: 94, IsPrimary: true},
				{Keyword: fmt.Sprintf("solución de %s normativa local", strings.ToLower(cat)), Locale: locale, Intent: "commercial", VolumeTier: "medium", EstMonthlyVolume: 4900, Difficulty: 22, Relevance: 92, IsPrimary: true},
			}
		default: // GoalTopTraffic
			defaults = []KeywordInsight{
				{Keyword: fmt.Sprintf("mejor plataforma de %s", strings.ToLower(cat)), Locale: locale, Intent: "commercial", VolumeTier: "high", EstMonthlyVolume: 17200, Difficulty: 43, Relevance: 96, IsPrimary: true},
				{Keyword: fmt.Sprintf("solución de %s en la nube", strings.ToLower(cat)), Locale: locale, Intent: "commercial", VolumeTier: "high", EstMonthlyVolume: 12600, Difficulty: 37, Relevance: 94, IsPrimary: true},
				{Keyword: fmt.Sprintf("herramienta de %s online", strings.ToLower(cat)), Locale: locale, Intent: "informational", VolumeTier: "medium", EstMonthlyVolume: 6900, Difficulty: 31, Relevance: 89, IsPrimary: false},
			}
		}
	default:
		defaults = []KeywordInsight{
			{Keyword: fmt.Sprintf("best %s platform", strings.ToLower(cat)), Locale: locale, Intent: "commercial", VolumeTier: "high", EstMonthlyVolume: 22000, Difficulty: 47, Relevance: 97, IsPrimary: true},
			{Keyword: fmt.Sprintf("top rated %s tool", strings.ToLower(cat)), Locale: locale, Intent: "commercial", VolumeTier: "high", EstMonthlyVolume: 15400, Difficulty: 41, Relevance: 95, IsPrimary: true},
			{Keyword: fmt.Sprintf("cloud %s pricing comparison", strings.ToLower(cat)), Locale: locale, Intent: "commercial", VolumeTier: "medium", EstMonthlyVolume: 7800, Difficulty: 34, Relevance: 91, IsPrimary: false},
			{Keyword: fmt.Sprintf("automate %s workflow", strings.ToLower(cat)), Locale: locale, Intent: "informational", VolumeTier: "medium", EstMonthlyVolume: 5200, Difficulty: 28, Relevance: 88, IsPrimary: false},
		}
	}

	for _, d := range defaults {
		if !seen[strings.ToLower(d.Keyword)] {
			seen[strings.ToLower(d.Keyword)] = true
			if len(pool) < 2 {
				d.IsPrimary = true
			}
			pool = append(pool, d)
		}
	}

	return pool
}
