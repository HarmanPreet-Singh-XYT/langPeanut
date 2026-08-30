package seo

import (
	"fmt"
	"strings"
)

// SERPSimulatorAgent synthesizes realistic Google SERP and social share visual preview payloads
type SERPSimulatorAgent struct{}

// NewSERPSimulatorAgent creates a new SERPSimulatorAgent
func NewSERPSimulatorAgent() *SERPSimulatorAgent {
	return &SERPSimulatorAgent{}
}

// GenerateSimulation produces a complete multi-modal preview for a target locale
func (s *SERPSimulatorAgent) GenerateSimulation(
	strategy *SEOStrategy,
	locale string,
	keywords []KeywordInsight,
	optimizations []KeyOptimization,
) *SERPSimulation {
	title := fmt.Sprintf("%s | #1 Rated %s Platform", strategy.ProjectName, strategy.Category)
	metaDesc := strategy.ProductDescription
	if metaDesc == "" {
		metaDesc = fmt.Sprintf("Discover %s — the leading %s designed to help modern teams scale faster with high reliability.", strategy.ProjectName, strategy.Category)
	}

	// Look for optimized meta / hero title keys
	for _, opt := range optimizations {
		kLower := strings.ToLower(opt.Key)
		if strings.Contains(kLower, "meta.title") || strings.Contains(kLower, "home.title") || strings.Contains(kLower, "hero.title") {
			if opt.OptimizedTranslation != "" {
				title = opt.OptimizedTranslation
			}
		} else if strings.Contains(kLower, "meta.desc") || strings.Contains(kLower, "description") {
			if opt.OptimizedTranslation != "" {
				metaDesc = opt.OptimizedTranslation
			}
		}
	}

	pixelWidth := EstimatePixelWidth(title, locale)
	isTrunc := pixelWidth > 600

	primaryKw := "Platform"
	if len(keywords) > 0 {
		primaryKw = keywords[0].Keyword
	}

	displayURL := fmt.Sprintf("https://%s.io/%s", strings.ToLower(strings.ReplaceAll(strategy.ProjectName, " ", "")), locale)

	// FAQ Rich snippet generation if available in keys
	var faqs []string
	for _, opt := range optimizations {
		if strings.Contains(strings.ToLower(opt.Key), "faq") && opt.OptimizedTranslation != "" {
			faqs = append(faqs, opt.OptimizedTranslation)
			if len(faqs) >= 2 {
				break
			}
		}
	}

	return &SERPSimulation{
		Locale:            locale,
		DisplayURL:        displayURL,
		TitleTag:          title,
		MetaDescription:   metaDesc,
		TitlePixelWidth:   pixelWidth,
		IsTitleTruncated:  isTrunc,
		HighlightedQuery:  primaryKw,
		OGCardTitle:       title,
		OGCardDescription: metaDesc,
		OGCardImage:       "https://images.unsplash.com/photo-1551288049-bebda4e38f71?auto=format&fit=crop&w=1200&q=80",
		RichSnippetFAQ:    faqs,
		RichSnippetRating: 4.9,
	}
}
