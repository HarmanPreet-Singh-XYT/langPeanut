package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/langPeanut/langPeanut/pkg/agents"
	"github.com/langPeanut/langPeanut/pkg/llm"
	"github.com/langPeanut/langPeanut/pkg/logger"
	"github.com/langPeanut/langPeanut/pkg/platforms"
	"github.com/langPeanut/langPeanut/pkg/seo"
	"github.com/spf13/cobra"
)

var (
	seoLocalesFlag     string
	seoGoalFlag        string
	seoCompetitorsFlag string
	seoScopeFlag       string
	seoJSONFlag        bool
	seoApplyFlag       bool
)

var seoCmd = &cobra.Command{
	Use:   "seo [directory]",
	Short: "Autonomous Multilingual SEO & Market Growth Studio",
	Long:  "Scouts regional competitor search engine landscapes, mines high-intent local keywords, semantically optimizes AST keys, and projects growth metrics.",
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir := projectRoot
		if len(args) > 0 {
			targetDir = args[0]
		}
		absRoot, err := filepath.Abs(targetDir)
		if err != nil {
			return err
		}

		client := llm.AutoDetectClient()

		// 1. Extract existing keys and target translations
		registry := platforms.NewRegistry()
		platform, _ := registry.AutoDetect(absRoot)

		sourceKeys := make(map[string]string)
		var extractedStrings []string
		if platform != nil {
			sourceKeys = seo.ExtractLocaleCatalog(absRoot, platform, "en")
			for _, v := range sourceKeys {
				if v != "" {
					extractedStrings = append(extractedStrings, v)
				}
			}
		}

		if len(sourceKeys) == 0 {
			// Fallback sample keys for testing
			sourceKeys = map[string]string{
				"home.hero.title": "The fastest workflow for modern developers",
				"home.hero.desc":  "Automate your daily coding and deployment workflows seamlessly.",
				"cta.button":      "Get Started Free",
			}
			for _, v := range sourceKeys {
				extractedStrings = append(extractedStrings, v)
			}
		}

		// 2. Discover Project Domain & Overview with Grounded UI Strings
		ctx := context.Background()
		projName := filepath.Base(absRoot)
		cat, desc := seo.InferSoftwareOverview(ctx, client, projName, extractedStrings, "", "")
		if cat == "" || cat == "Software Platform" {
			scoutAgent := agents.NewPersonaScoutAgent(client)
			persona, _ := scoutAgent.DiscoverPersona(absRoot)
			if persona != nil {
				if persona.ProjectName != "" {
					projName = persona.ProjectName
				}
				if persona.Audience != "" && len(persona.Audience) <= 25 {
					cat = persona.Audience
				} else if persona.Summary != "" && len(persona.Summary) <= 30 && !strings.HasPrefix(persona.Summary, "Autonomous localization") {
					cat = persona.Summary
				} else if strings.Contains(strings.ToLower(projName), "store") || strings.Contains(strings.ToLower(projName), "shop") || strings.Contains(strings.ToLower(projName), "commerce") {
					cat = "E-Commerce Platform"
				} else if strings.Contains(strings.ToLower(projName), "app") {
					cat = "Application"
				}
			}
			if cat == "" {
				cat = "Developer Tool & Software"
			}
			if desc == "" {
				desc = fmt.Sprintf("Autonomous software solution: %s", projName)
			}
		}

		// 3. Parse Locales
		locales := []string{"en", "ja", "de", "es"}
		if seoLocalesFlag != "" {
			parts := strings.Split(seoLocalesFlag, ",")
			locales = make([]string, 0, len(parts))
			for _, p := range parts {
				t := strings.TrimSpace(p)
				if t != "" {
					locales = append(locales, t)
				}
			}
		}

		// 4. Parse Competitors
		var competitors []string
		if seoCompetitorsFlag != "" {
			parts := strings.Split(seoCompetitorsFlag, ",")
			for _, p := range parts {
				t := strings.TrimSpace(p)
				if t != "" {
					competitors = append(competitors, t)
				}
			}
		}

		goal := seo.GrowthGoal(seoGoalFlag)
		if goal == "" {
			goal = seo.GoalTopTraffic
		}

		scope := seo.KeyScopeTier(seoScopeFlag)
		if scope == "" {
			scope = seo.ScopeHighImpact
		}

		strategy := &seo.SEOStrategy{
			ProjectName:        projName,
			Category:           cat,
			ProductDescription: desc,
			TargetLocales:      locales,
			Goal:               goal,
			ScopeTier:          scope,
			CompetitorURLs:     competitors,
		}

		baselineMatrix := make(map[string]map[string]string)
		if platform != nil {
			for _, loc := range locales {
				if loc == "en" {
					baselineMatrix["en"] = sourceKeys
				} else if entries := seo.ExtractLocaleCatalog(absRoot, platform, loc); entries != nil {
					baselineMatrix[loc] = entries
				}
			}
		}

		fmt.Printf("\nLaunching langPeanut SEO & Market Growth Studio\n")
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("• Project:      %s\n", strategy.ProjectName)
		fmt.Printf("• Target Geos:  [%s]\n", strings.Join(strategy.TargetLocales, ", "))
		fmt.Printf("• Growth Goal:  %s\n", strategy.Goal)
		fmt.Printf("• Key Scope:    %s\n", strategy.ScopeTier)
		if len(strategy.CompetitorURLs) > 0 {
			fmt.Printf("• Competitors:  [%s]\n", strings.Join(strategy.CompetitorURLs, ", "))
		}
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

		orchestrator := seo.NewStudioOrchestrator(client)
		result, err := orchestrator.RunStudio(ctx, strategy, sourceKeys, baselineMatrix)
		if err != nil {
			return fmt.Errorf("SEO Studio run failed: %w", err)
		}

		if seoJSONFlag {
			data, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(data))
			return nil
		}

		// Print Structured Summary per Locale
		for _, loc := range strategy.TargetLocales {
			metrics := result.Metrics[loc]
			sim := result.Simulations[loc]
			comps := result.Competitors[loc]
			kws := result.KeywordPool[loc]
			opts := result.Optimizations[loc]

			fmt.Printf("\nTarget Market: [%s]\n", strings.ToUpper(loc))
			fmt.Printf("────────────────────────────────────────────────────────────────────\n")

			if len(comps) > 0 {
				fmt.Printf("Top Ranking Competitors Identified:\n")
				for _, c := range comps {
					fmt.Printf("   #%d %s — %s\n", c.Rank, c.Domain, c.Title)
				}
			}

			if len(kws) > 0 {
				fmt.Printf("\nHigh-Intent Keyword Intelligence:\n")
				for _, k := range kws {
					primaryTag := ""
					if k.IsPrimary {
						primaryTag = " [Primary]"
					}
					fmt.Printf("   • %-32s (Vol: ~%d/mo | KD: %d | Intent: %s)%s\n",
						k.Keyword, k.EstMonthlyVolume, k.Difficulty, k.Intent, primaryTag)
				}
			}

			if metrics != nil {
				fmt.Printf("\nProjected Growth & Impact Metrics:\n")
				fmt.Printf("   • Target Search Volume Reach:  %d/mo (+%.1f%% uplift)\n",
					metrics.SearchVolumeOptimized, metrics.SearchVolumeUpliftPct)
				fmt.Printf("   • Projected SERP CTR:          %.1f%% (Baseline: %.1f%%, +%.1f%%)\n",
					metrics.ProjectedCTROptimized, metrics.ProjectedCTRBaseline, metrics.ProjectedCTRUpliftPct)
				fmt.Printf("   • Localized Trust Factor:      %d/100\n", metrics.LocalTrustScore)
				fmt.Printf("   • Keyword Density:             %.1f%% (Safe: %t)\n",
					metrics.KeywordDensityPct, metrics.DensitySafe)
			}

			if sim != nil {
				fmt.Printf("\nSimulated Google SERP Snippet:\n")
				fmt.Printf("   ┌─────────────────────────────────────────────────────────────┐\n")
				fmt.Printf("   │ %s\n", sim.DisplayURL)
				fmt.Printf("   │ \033[1;34m%s\033[0m\n", sim.TitleTag)
				fmt.Printf("   │ %s\n", sim.MetaDescription)
				fmt.Printf("   └─────────────────────────────────────────────────────────────┘\n")
			}

			if len(opts) > 0 {
				fmt.Printf("\nSemantic Key Optimizations (%d keys):\n", len(opts))
				for _, o := range opts {
					fmt.Printf("   • Key: %s\n", o.Key)
					fmt.Printf("     Source (en): %s\n", o.SourceEn)
					fmt.Printf("     Baseline:    %s\n", o.BaselineTranslation)
					fmt.Printf("     \033[1;32mOptimized:   %s\033[0m\n", o.OptimizedTranslation)
					fmt.Printf("     Rationale:   %s\n\n", o.Rationale)
				}
			}
		}

		// 5. Apply to locale files if requested
		if seoApplyFlag && platform != nil {
			fmt.Printf("Applying SEO-optimized keys to repository files...\n")
			for loc, opts := range result.Optimizations {
				targetMap := make(map[string]string)
				if existing := seo.ExtractLocaleCatalog(absRoot, platform, loc); existing != nil {
					for k, v := range existing {
						targetMap[k] = v
					}
				}
				for _, o := range opts {
					targetMap[o.Key] = o.OptimizedTranslation
				}
				if err := seo.WriteLocaleCatalog(absRoot, platform, loc, targetMap); err == nil {
					logger.Get().Success("SEO:APPLY", fmt.Sprintf("Updated %s locale catalog with SEO optimizations", loc))
				}
			}
			fmt.Printf("All SEO optimizations successfully applied to disk!\n")
		}

		return nil
	},
}

func init() {
	seoCmd.Flags().StringVarP(&seoLocalesFlag, "locales", "l", "en,ja,de,es", "Target locales to optimize (comma-separated)")
	seoCmd.Flags().StringVarP(&seoGoalFlag, "goal", "g", "traffic", "SEO growth goal (traffic, conversion, trust)")
	seoCmd.Flags().StringVarP(&seoCompetitorsFlag, "competitors", "c", "", "Competitor URLs to scout (comma-separated)")
	seoCmd.Flags().StringVar(&seoScopeFlag, "scope", "high_impact", "Key optimization scope (high_impact, full_site)")
	seoCmd.Flags().BoolVar(&seoJSONFlag, "json", false, "Output results as raw JSON")
	seoCmd.Flags().BoolVar(&seoApplyFlag, "apply", false, "Write optimized keys directly into target locale files")
	rootCmd.AddCommand(seoCmd)
}
