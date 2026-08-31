package seo

import (
	"context"
	"testing"
)

func TestSEO_ClassifyKeyImpact(t *testing.T) {
	tests := []struct {
		key      string
		expected string
	}{
		{"home.hero.title", "high"},
		{"landing.meta.description", "high"},
		{"features.analytics.heading", "high"},
		{"faq.q1", "high"},
		{"buttons.submit", "standard"},
		{"modal.close_label", "standard"},
		{"auth.password_placeholder", "standard"},
	}

	for _, tt := range tests {
		got := ClassifyKeyImpact(tt.key)
		if got != tt.expected {
			t.Errorf("ClassifyKeyImpact(%q) = %q; want %q", tt.key, got, tt.expected)
		}
	}
}

func TestSEO_EstimatePixelWidth(t *testing.T) {
	latinText := "Simple Headline"
	latinW := EstimatePixelWidth(latinText, "en")
	if latinW <= 0 {
		t.Errorf("EstimatePixelWidth latin = %d; want > 0", latinW)
	}

	cjkText := "無料請求書作成ソフト・インボイス制度対応"
	cjkW := EstimatePixelWidth(cjkText, "ja")
	if cjkW <= latinW {
		t.Errorf("EstimatePixelWidth CJK (%d) should be wider than short latin (%d)", cjkW, latinW)
	}
}

func TestSEO_ValidateICUVariables(t *testing.T) {
	src := "You have {count} pending invoices for {userName}"
	validTarget := "あなたは {userName} の保留中の請求書が {count} 件あります"
	invalidTarget := "請求書があります"

	if !validateICUVariables(src, validTarget) {
		t.Errorf("validateICUVariables should return true when all variables are present")
	}
	if validateICUVariables(src, invalidTarget) {
		t.Errorf("validateICUVariables should return false when variables are missing")
	}
}

func TestSEO_GrowthPredictorCritic(t *testing.T) {
	critic := NewGrowthPredictorCritic()
	strategy := &SEOStrategy{
		ProjectName: "FastInvoice",
		Category:    "Invoicing Software",
		Goal:        GoalTopTraffic,
	}

	keywords := []KeywordInsight{
		{Keyword: "無料請求書ソフト", EstMonthlyVolume: 20000, Difficulty: 30, IsPrimary: true},
		{Keyword: "インボイス対応", EstMonthlyVolume: 15000, Difficulty: 40, IsPrimary: true},
	}

	opts := []KeyOptimization{
		{
			Key:                  "home.hero.title",
			OptimizedTranslation: "無料請求書ソフト・インボイス対応の決定版",
		},
	}

	metrics := critic.EvaluateGrowth(strategy, "ja", keywords, opts)
	if metrics == nil {
		t.Fatalf("expected non-nil GrowthMetrics")
	}

	if metrics.SearchVolumeOptimized != 35000 {
		t.Errorf("SearchVolumeOptimized = %d; want 35000", metrics.SearchVolumeOptimized)
	}

	if metrics.SearchVolumeUpliftPct <= 0 {
		t.Errorf("SearchVolumeUpliftPct = %f; want > 0", metrics.SearchVolumeUpliftPct)
	}

	if metrics.ProjectedCTROptimized <= metrics.ProjectedCTRBaseline {
		t.Errorf("ProjectedCTROptimized (%f) should be higher than baseline (%f)", metrics.ProjectedCTROptimized, metrics.ProjectedCTRBaseline)
	}

	if !metrics.DensitySafe {
		t.Errorf("Keyword density should be safe for non-stuffed copy")
	}
}

func TestSEO_StudioOrchestratorEndToEnd(t *testing.T) {
	orchestrator := NewStudioOrchestrator(nil) // Offline synthetic mode

	strategy := &SEOStrategy{
		ProjectName:        "CloudInvoice",
		Category:           "Accounting & Invoicing SaaS",
		ProductDescription: "Automated billing and invoice generation for freelancers and agencies",
		TargetLocales:      []string{"ja", "de"},
		Goal:               GoalTopTraffic,
		ScopeTier:          ScopeHighImpact,
	}

	sourceKeys := map[string]string{
		"home.hero.title":     "Fastest Invoice Generator",
		"home.hero.desc":      "Create professional invoices in seconds.",
		"buttons.submit":      "Send Invoice",
	}

	baselineMatrix := map[string]map[string]string{
		"ja": {
			"home.hero.title": "最速の請求書作成",
			"home.hero.desc":  "数秒でプロフェッショナルな請求書を作成します。",
			"buttons.submit":  "請求書を送信",
		},
		"de": {
			"home.hero.title": "Schnellster Rechnungsgenerator",
			"home.hero.desc":  "Erstellen Sie professionelle Rechnungen in Sekunden.",
			"buttons.submit":  "Rechnung senden",
		},
	}

	ctx := context.Background()
	result, err := orchestrator.RunStudio(ctx, strategy, sourceKeys, baselineMatrix)
	if err != nil {
		t.Fatalf("RunStudio failed: %v", err)
	}

	if len(result.Competitors) != 2 {
		t.Errorf("len(Competitors) = %d; want 2", len(result.Competitors))
	}
	if len(result.KeywordPool["ja"]) == 0 {
		t.Errorf("KeywordPool for 'ja' should not be empty")
	}
	if len(result.Optimizations["ja"]) == 0 {
		t.Errorf("Optimizations for 'ja' should not be empty")
	}
	if result.Metrics["ja"] == nil {
		t.Errorf("Metrics for 'ja' should not be nil")
	}
	if result.Simulations["ja"] == nil {
		t.Errorf("Simulation for 'ja' should not be nil")
	}
}
