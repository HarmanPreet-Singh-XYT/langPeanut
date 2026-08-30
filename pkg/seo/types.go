package seo

import "time"

// GrowthGoal represents the commercial intent of SEO localization
type GrowthGoal string

const (
	GoalTopTraffic   GrowthGoal = "traffic"    // High-volume discovery & top-of-funnel reach
	GoalConversion   GrowthGoal = "conversion" // High-intent commercial buyer keywords
	GoalTrust        GrowthGoal = "trust"      // Regional compliance, security & local authority
)

// KeyScopeTier dictates which AST keys are targeted for SEO optimization
type KeyScopeTier string

const (
	ScopeHighImpact KeyScopeTier = "high_impact" // Hero, Title, Meta, Headings, Features, FAQs
	ScopeFullSite   KeyScopeTier = "full_site"   // All keys including buttons, labels, dialogs
	ScopeCustom     KeyScopeTier = "custom"      // User-selected key list
)

// SEOStrategy captures product identity, goals, target markets, and competitors
type SEOStrategy struct {
	ProjectName        string       `json:"project_name"`
	Category           string       `json:"category"`
	ProductDescription string       `json:"product_description"`
	TargetLocales      []string     `json:"target_locales"`
	Goal               GrowthGoal   `json:"goal"`
	ScopeTier          KeyScopeTier `json:"scope_tier"`
	CustomKeyList      []string     `json:"custom_key_list,omitempty"`
	CompetitorURLs     []string     `json:"competitor_urls"`
	UpdatedAt          time.Time    `json:"updated_at"`
}

// CompetitorProfile represents a regional market competitor analyzed by the scout
type CompetitorProfile struct {
	URL             string   `json:"url"`
	Domain          string   `json:"domain"`
	Rank            int      `json:"rank"`
	Title           string   `json:"title"`
	MetaDescription string   `json:"meta_description"`
	H1s             []string `json:"h1s"`
	H2s             []string `json:"h2s"`
	Keywords        []string `json:"keywords"`
	ValueProps      []string `json:"value_props"`
	IsDiscovered    bool     `json:"is_discovered"`
}

// KeywordInsight models regional search query intelligence
type KeywordInsight struct {
	Keyword          string `json:"keyword"`
	Locale           string `json:"locale"`
	Intent           string `json:"intent"`       // "commercial", "informational", "transactional"
	VolumeTier       string `json:"volume_tier"`  // "high", "medium", "low"
	EstMonthlyVolume int    `json:"est_monthly_volume"`
	Difficulty       int    `json:"difficulty"`   // 0 - 100
	Relevance        int    `json:"relevance"`    // 0 - 100
	IsPrimary        bool   `json:"is_primary"`
	IsLocked         bool   `json:"is_locked"`    // User pinned keyword
}

// KeyOptimization represents a single AST key optimized for search & conversion
type KeyOptimization struct {
	Key                  string   `json:"key"`
	Locale               string   `json:"locale"`
	SourceEn             string   `json:"source_en"`
	BaselineTranslation  string   `json:"baseline_translation"`
	OptimizedTranslation string   `json:"optimized_translation"`
	InjectedKeywords     []string `json:"injected_keywords"`
	Rationale            string   `json:"rationale"`
	ImpactTier           string   `json:"impact_tier"` // "high", "standard"
	CharacterLength      int      `json:"character_length"`
	PixelWidthDesktop    int      `json:"pixel_width_desktop"`
	IsTitleTruncated     bool     `json:"is_title_truncated"`
	ICUVariablesMatched  bool     `json:"icu_variables_matched"`
}

// GrowthMetrics represents predictive impact projections per target locale
type GrowthMetrics struct {
	TargetLocale          string  `json:"target_locale"`
	SearchVolumeBaseline  int     `json:"search_volume_baseline"`
	SearchVolumeOptimized int     `json:"search_volume_optimized"`
	SearchVolumeUpliftPct float64 `json:"search_volume_uplift_pct"`
	ProjectedCTRBaseline  float64 `json:"projected_ctr_baseline"`
	ProjectedCTROptimized float64 `json:"projected_ctr_optimized"`
	ProjectedCTRUpliftPct float64 `json:"projected_ctr_uplift_pct"`
	AvgKeywordDifficulty  int     `json:"avg_keyword_difficulty"`
	ReadabilityScore      int     `json:"readability_score"`      // 0 - 100
	LocalTrustScore       int     `json:"local_trust_score"`      // 0 - 100
	KeywordDensityPct     float64 `json:"keyword_density_pct"`    // e.g. 2.4%
	DensitySafe           bool    `json:"density_safe"`           // True if under 3.5%
	EstimatedRankingDays  int     `json:"estimated_ranking_days"` // Time to rank top 10
}

// SERPSimulation models the visual rendering of search results & social share cards
type SERPSimulation struct {
	Locale            string   `json:"locale"`
	DisplayURL        string   `json:"display_url"`
	TitleTag          string   `json:"title_tag"`
	MetaDescription   string   `json:"meta_description"`
	TitlePixelWidth   int      `json:"title_pixel_width"`
	IsTitleTruncated  bool     `json:"is_title_truncated"`
	HighlightedQuery  string   `json:"highlighted_query"`
	OGCardTitle       string   `json:"og_card_title"`
	OGCardDescription string   `json:"og_card_description"`
	OGCardImage       string   `json:"og_card_image"`
	RichSnippetFAQ    []string `json:"rich_snippet_faq,omitempty"`
	RichSnippetRating float64  `json:"rich_snippet_rating,omitempty"`
}

// SEOResult aggregates the full outcome of an SEO & Growth studio run
type SEOResult struct {
	Strategy      *SEOStrategy                   `json:"strategy"`
	Competitors   map[string][]CompetitorProfile `json:"competitors"`   // locale -> competitors
	KeywordPool   map[string][]KeywordInsight    `json:"keyword_pool"`   // locale -> keywords
	Optimizations map[string][]KeyOptimization   `json:"optimizations"` // locale -> key optimizations
	Metrics       map[string]*GrowthMetrics      `json:"metrics"`       // locale -> growth metrics
	Simulations   map[string]*SERPSimulation     `json:"simulations"`   // locale -> visual simulation
}
