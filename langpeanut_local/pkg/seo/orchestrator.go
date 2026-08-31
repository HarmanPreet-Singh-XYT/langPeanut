package seo

import (
	"context"
	"fmt"
	"time"

	"github.com/langPeanut/langPeanut/pkg/llm"
	"github.com/langPeanut/langPeanut/pkg/logger"
)

// StudioOrchestrator coordinates the end-to-end SEO and Growth pipeline
type StudioOrchestrator struct {
	LLMClient   llm.Client
	Scout       *SERPScoutAgent
	Keywords    *KeywordIntelligenceAgent
	Weaver      *SemanticCopyWeaverAgent
	Critic      *GrowthPredictorCritic
	Simulator   *SERPSimulatorAgent
}

// NewStudioOrchestrator creates a new StudioOrchestrator instance
func NewStudioOrchestrator(client llm.Client) *StudioOrchestrator {
	return &StudioOrchestrator{
		LLMClient: client,
		Scout:     NewSERPScoutAgent(client),
		Keywords:  NewKeywordIntelligenceAgent(client),
		Weaver:    NewSemanticCopyWeaverAgent(client),
		Critic:    NewGrowthPredictorCritic(),
		Simulator: NewSERPSimulatorAgent(),
	}
}

// RunStudio executes the full SEO growth optimization pipeline across all target locales
func (o *StudioOrchestrator) RunStudio(
	ctx context.Context,
	strategy *SEOStrategy,
	sourceKeys map[string]string, // key -> en source text
	baselineMatrix map[string]map[string]string, // locale -> (key -> baseline translation)
) (*SEOResult, error) {
	if strategy == nil {
		strategy = &SEOStrategy{
			ProjectName:        "Application",
			Category:           "Software Platform",
			ProductDescription: "Modern software application",
			TargetLocales:      []string{"ja", "de", "es"},
			Goal:               GoalTopTraffic,
			ScopeTier:          ScopeHighImpact,
		}
	}
	strategy.UpdatedAt = time.Now()

	logger.Get().Info("SEO:STUDIO", fmt.Sprintf("Launching SEO Growth Studio Pipeline for %d target locales (Goal: %s)", len(strategy.TargetLocales), strategy.Goal))

	result := &SEOResult{
		Strategy:      strategy,
		Competitors:   make(map[string][]CompetitorProfile),
		KeywordPool:   make(map[string][]KeywordInsight),
		Optimizations: make(map[string][]KeyOptimization),
		Metrics:       make(map[string]*GrowthMetrics),
		Simulations:   make(map[string]*SERPSimulation),
	}

	for _, loc := range strategy.TargetLocales {
		logger.Get().Info("SEO:STUDIO", fmt.Sprintf("─── Processing Locale: %s ───", loc))

		// 1. Competitor Scouting
		comps, err := o.Scout.ScoutLocale(ctx, strategy, loc)
		if err != nil {
			logger.Get().Warn("SEO:STUDIO", fmt.Sprintf("Scout error for %s: %v", loc, err))
		}
		result.Competitors[loc] = comps

		// 2. Keyword Intelligence & Intent Mining
		kwPool, err := o.Keywords.AnalyzeKeywords(ctx, strategy, loc, comps)
		if err != nil {
			logger.Get().Warn("SEO:STUDIO", fmt.Sprintf("Keyword analysis error for %s: %v", loc, err))
		}
		result.KeywordPool[loc] = kwPool

		// 3. Semantic Copy Weaving
		baseMap := baselineMatrix[loc]
		if baseMap == nil {
			baseMap = make(map[string]string)
		}
		opts, err := o.Weaver.WeaveTranslations(ctx, strategy, loc, sourceKeys, baseMap, kwPool)
		if err != nil {
			logger.Get().Warn("SEO:STUDIO", fmt.Sprintf("Copy weaving error for %s: %v", loc, err))
		}
		result.Optimizations[loc] = opts

		// 4. Growth Metric Critic
		metrics := o.Critic.EvaluateGrowth(strategy, loc, kwPool, opts)
		result.Metrics[loc] = metrics

		// 5. Visual SERP & Social Simulation
		sim := o.Simulator.GenerateSimulation(strategy, loc, kwPool, opts)
		result.Simulations[loc] = sim
	}

	logger.Get().Success("SEO:STUDIO", fmt.Sprintf("SEO Growth Studio execution completed successfully across %d locales", len(strategy.TargetLocales)))
	return result, nil
}
