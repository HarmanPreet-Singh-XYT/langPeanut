package llm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ModelUsage tracks input and output token consumption for a specific model
type ModelUsage struct {
	Model            string    `json:"model"`
	Provider         string    `json:"provider"`
	InputTokens      int64     `json:"input_tokens"`
	OutputTokens     int64     `json:"output_tokens"`
	TotalTokens      int64     `json:"total_tokens"`
	Requests         int64     `json:"requests"`
	EstimatedCostUSD float64   `json:"estimated_cost_usd"`
	LastUsed         time.Time `json:"last_used"`
}

// TokenStats contains aggregate usage statistics and per-model breakdowns
type TokenStats struct {
	TotalInputTokens      int64                  `json:"total_input_tokens"`
	TotalOutputTokens     int64                  `json:"total_output_tokens"`
	TotalTokens           int64                  `json:"total_tokens"`
	TotalRequests         int64                  `json:"total_requests"`
	TotalEstimatedCostUSD float64                `json:"total_estimated_cost_usd"`
	LastUpdated           time.Time              `json:"last_updated"`
	ByModel               map[string]*ModelUsage `json:"by_model"`
}

// TokenTracker manages thread-safe tracking and disk persistence of token usage
type TokenTracker struct {
	mu           sync.RWMutex
	stats        TokenStats
	sessionStats TokenStats
	persistPath  string
}

var (
	globalTracker     *TokenTracker
	globalTrackerOnce sync.Once
)

// GetGlobalTracker returns the singleton token usage tracker
func GetGlobalTracker() *TokenTracker {
	globalTrackerOnce.Do(func() {
		homeDir, _ := os.UserHomeDir()
		p := filepath.Join(homeDir, ".langPeanut", "token_usage.json")
		globalTracker = NewTokenTracker(p)
	})
	return globalTracker
}

// NewTokenTracker initializes a token tracker with persistence
func NewTokenTracker(persistPath string) *TokenTracker {
	tracker := &TokenTracker{
		persistPath: persistPath,
		stats: TokenStats{
			ByModel: make(map[string]*ModelUsage),
		},
		sessionStats: TokenStats{
			ByModel: make(map[string]*ModelUsage),
		},
	}
	tracker.load()
	return tracker
}

// RecordUsage logs an API call with input/output token counts
func RecordUsage(provider, model string, inputTokens, outputTokens int64) {
	GetGlobalTracker().Record(provider, model, inputTokens, outputTokens)
}

// Record records input and output token counts for a model
func (t *TokenTracker) Record(provider, model string, inputTokens, outputTokens int64) {
	if model == "" {
		model = "unknown"
	}
	if inputTokens < 0 {
		inputTokens = 0
	}
	if outputTokens < 0 {
		outputTokens = 0
	}
	totalTokens := inputTokens + outputTokens
	cost := estimateCost(model, inputTokens, outputTokens)

	t.mu.Lock()
	defer t.mu.Unlock()

	// Update cumulative stats
	t.stats.TotalInputTokens += inputTokens
	t.stats.TotalOutputTokens += outputTokens
	t.stats.TotalTokens += totalTokens
	t.stats.TotalRequests++
	t.stats.TotalEstimatedCostUSD += cost
	t.stats.LastUpdated = time.Now()

	if t.stats.ByModel == nil {
		t.stats.ByModel = make(map[string]*ModelUsage)
	}

	mu, exists := t.stats.ByModel[model]
	if !exists {
		mu = &ModelUsage{
			Model:    model,
			Provider: provider,
		}
		t.stats.ByModel[model] = mu
	}
	mu.InputTokens += inputTokens
	mu.OutputTokens += outputTokens
	mu.TotalTokens += totalTokens
	mu.Requests++
	mu.EstimatedCostUSD += cost
	mu.LastUsed = time.Now()

	// Update session stats
	t.sessionStats.TotalInputTokens += inputTokens
	t.sessionStats.TotalOutputTokens += outputTokens
	t.sessionStats.TotalTokens += totalTokens
	t.sessionStats.TotalRequests++
	t.sessionStats.TotalEstimatedCostUSD += cost
	t.sessionStats.LastUpdated = time.Now()

	if t.sessionStats.ByModel == nil {
		t.sessionStats.ByModel = make(map[string]*ModelUsage)
	}
	smu, sExists := t.sessionStats.ByModel[model]
	if !sExists {
		smu = &ModelUsage{
			Model:    model,
			Provider: provider,
		}
		t.sessionStats.ByModel[model] = smu
	}
	smu.InputTokens += inputTokens
	smu.OutputTokens += outputTokens
	smu.TotalTokens += totalTokens
	smu.Requests++
	smu.EstimatedCostUSD += cost
	smu.LastUsed = time.Now()

	t.save()
}

// GetStats returns a copy of cumulative token stats
func (t *TokenTracker) GetStats() TokenStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	copyStats := TokenStats{
		TotalInputTokens:      t.stats.TotalInputTokens,
		TotalOutputTokens:     t.stats.TotalOutputTokens,
		TotalTokens:           t.stats.TotalTokens,
		TotalRequests:         t.stats.TotalRequests,
		TotalEstimatedCostUSD: t.stats.TotalEstimatedCostUSD,
		LastUpdated:           t.stats.LastUpdated,
		ByModel:               make(map[string]*ModelUsage),
	}
	for k, v := range t.stats.ByModel {
		cp := *v
		copyStats.ByModel[k] = &cp
	}
	return copyStats
}

// GetSessionStats returns tokens used during the current application session
func (t *TokenTracker) GetSessionStats() TokenStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	copyStats := TokenStats{
		TotalInputTokens:      t.sessionStats.TotalInputTokens,
		TotalOutputTokens:     t.sessionStats.TotalOutputTokens,
		TotalTokens:           t.sessionStats.TotalTokens,
		TotalRequests:         t.sessionStats.TotalRequests,
		TotalEstimatedCostUSD: t.sessionStats.TotalEstimatedCostUSD,
		LastUpdated:           t.sessionStats.LastUpdated,
		ByModel:               make(map[string]*ModelUsage),
	}
	for k, v := range t.sessionStats.ByModel {
		cp := *v
		copyStats.ByModel[k] = &cp
	}
	return copyStats
}

// Reset clears recorded cumulative token metrics
func (t *TokenTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.stats = TokenStats{
		ByModel: make(map[string]*ModelUsage),
	}
	t.sessionStats = TokenStats{
		ByModel: make(map[string]*ModelUsage),
	}
	if t.persistPath != "" {
		_ = os.Remove(t.persistPath)
	}
}

func (t *TokenTracker) save() {
	if t.persistPath == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(t.persistPath), 0755)
	data, err := json.MarshalIndent(t.stats, "", "  ")
	if err == nil {
		_ = os.WriteFile(t.persistPath, data, 0644)
	}
}

func (t *TokenTracker) load() {
	if t.persistPath == "" {
		return
	}
	data, err := os.ReadFile(t.persistPath)
	if err != nil {
		return
	}
	var loaded TokenStats
	if err := json.Unmarshal(data, &loaded); err == nil {
		t.stats = loaded
		if t.stats.ByModel == nil {
			t.stats.ByModel = make(map[string]*ModelUsage)
		}
	}
}

// EstimateTokens calculates an approximate token count for offline/fallback estimation
func EstimateTokens(text string) int64 {
	words := len(strings.Fields(text))
	if words == 0 {
		return 0
	}
	// Industry heuristic: ~4 characters or 0.75 words per token (1 word ≈ 1.33 tokens)
	tokens := int64(float64(words) * 1.33)
	if tokens == 0 {
		return 1
	}
	return tokens
}

// ModelSpec contains context limits, max output, and pricing per 1M tokens
type ModelSpec struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Provider      string  `json:"provider"`
	ContextWindow int64   `json:"context_window"`
	MaxOutput     int64   `json:"max_output"`
	InputCost1M   float64 `json:"input_cost_1m"`
	OutputCost1M  float64 `json:"output_cost_1m"`
	Description   string  `json:"description"`
}

// ModelRegistry holds known model specifications across all providers
var ModelRegistry = map[string]ModelSpec{
	// ── Anthropic Claude ──────────────────────────────────────────────────────
	"claude-fable-5":   {ID: "claude-fable-5", Name: "Claude Fable 5", Provider: "claude", ContextWindow: 1_000_000, MaxOutput: 128_000, InputCost1M: 10.00, OutputCost1M: 50.00, Description: "Frontier narrative and hyper-contextual reasoning"},
	"claude-opus-5":    {ID: "claude-opus-5", Name: "Claude Opus 5", Provider: "claude", ContextWindow: 1_000_000, MaxOutput: 128_000, InputCost1M: 5.00, OutputCost1M: 25.00, Description: "Flagship intelligence for deep code refactoring"},
	"claude-opus-4.8":  {ID: "claude-opus-4.8", Name: "Claude Opus 4.8", Provider: "claude", ContextWindow: 1_000_000, MaxOutput: 128_000, InputCost1M: 5.00, OutputCost1M: 25.00, Description: "Enterprise complex reasoning model"},
	"claude-opus-4.7":  {ID: "claude-opus-4.7", Name: "Claude Opus 4.7", Provider: "claude", ContextWindow: 1_000_000, MaxOutput: 128_000, InputCost1M: 5.00, OutputCost1M: 25.00, Description: "Advanced code synthesis and architectural modeling"},
	"claude-opus-4.6":  {ID: "claude-opus-4.6", Name: "Claude Opus 4.6", Provider: "claude", ContextWindow: 1_000_000, MaxOutput: 128_000, InputCost1M: 5.00, OutputCost1M: 25.00, Description: "High-precision multi-file AST transform engine"},
	"claude-sonnet-5":  {ID: "claude-sonnet-5", Name: "Claude Sonnet 5", Provider: "claude", ContextWindow: 1_000_000, MaxOutput: 128_000, InputCost1M: 2.00, OutputCost1M: 10.00, Description: "Optimal balance of frontier intelligence and speed"},
	"claude-sonnet-4.6":{ID: "claude-sonnet-4.6", Name: "Claude Sonnet 4.6", Provider: "claude", ContextWindow: 1_000_000, MaxOutput: 128_000, InputCost1M: 3.00, OutputCost1M: 15.00, Description: "High-accuracy AST localization and grammar fidelity"},
	"claude-sonnet-4.5":{ID: "claude-sonnet-4.5", Name: "Claude Sonnet 4.5", Provider: "claude", ContextWindow: 200_000, MaxOutput: 8_192, InputCost1M: 3.00, OutputCost1M: 15.00, Description: "Reliable production workhorse"},
	"claude-haiku-4.5": {ID: "claude-haiku-4.5", Name: "Claude Haiku 4.5", Provider: "claude", ContextWindow: 200_000, MaxOutput: 64_000, InputCost1M: 1.00, OutputCost1M: 5.00, Description: "Ultra-fast translation and key validation"},

	// ── Google Gemini ─────────────────────────────────────────────────────────
	"gemini-3.7-flash":             {ID: "gemini-3.7-flash", Name: "Gemini 3.7 Flash", Provider: "gemini", ContextWindow: 1_000_000, MaxOutput: 8_192, InputCost1M: 0.75, OutputCost1M: 3.75, Description: "High-intelligence workhorse model for coding and agentic tasks"},
	"gemini-3.6-flash":             {ID: "gemini-3.6-flash", Name: "Gemini 3.6 Flash", Provider: "gemini", ContextWindow: 1_000_000, MaxOutput: 8_192, InputCost1M: 0.75, OutputCost1M: 3.75, Description: "Fast multimodal agentic execution"},
	"gemini-3.5-flash":             {ID: "gemini-3.5-flash", Name: "Gemini 3.5 Flash", Provider: "gemini", ContextWindow: 1_000_000, MaxOutput: 8_192, InputCost1M: 1.50, OutputCost1M: 9.00, Description: "Optimized for fast, long-horizon agentic workflows and autonomous loops"},
	"gemini-3.5-flash-lite":        {ID: "gemini-3.5-flash-lite", Name: "Gemini 3.5 Flash-Lite", Provider: "gemini", ContextWindow: 1_000_000, MaxOutput: 8_192, InputCost1M: 0.30, OutputCost1M: 2.50, Description: "High-volume, cost-sensitive workhorse model"},
	"gemini-3.1-pro-preview":       {ID: "gemini-3.1-pro-preview", Name: "Gemini 3.1 Pro", Provider: "gemini", ContextWindow: 1_000_000, MaxOutput: 65_536, InputCost1M: 2.00, OutputCost1M: 12.00, Description: "Advanced reasoning and complex coding model"},
	"gemini-3.1-flash-live-preview":{ID: "gemini-3.1-flash-live-preview", Name: "Gemini 3.1 Flash Live", Provider: "gemini", ContextWindow: 1_000_000, MaxOutput: 8_192, InputCost1M: 0.75, OutputCost1M: 3.75, Description: "Low-latency audio-to-audio dialogue model"},
	"gemini-2.5-flash":             {ID: "gemini-2.5-flash", Name: "Gemini 2.5 Flash", Provider: "gemini", ContextWindow: 1_000_000, MaxOutput: 8_192, InputCost1M: 0.10, OutputCost1M: 0.40, Description: "Ultra cost-efficient high-speed model"},

	// ── OpenAI ────────────────────────────────────────────────────────────────
	"gpt-5.6-sol":   {ID: "gpt-5.6-sol", Name: "GPT-5.6 Sol", Provider: "openai", ContextWindow: 1_050_000, MaxOutput: 128_000, InputCost1M: 4.00, OutputCost1M: 20.00, Description: "Current Flagship Generation with 1.05M context window"},
	"gpt-5.6-terra": {ID: "gpt-5.6-terra", Name: "GPT-5.6 Terra", Provider: "openai", ContextWindow: 1_050_000, MaxOutput: 128_000, InputCost1M: 2.00, OutputCost1M: 12.00, Description: "Balanced flagship model for large-scale codebases"},
	"gpt-5.6-luna":  {ID: "gpt-5.6-luna", Name: "GPT-5.6 Luna", Provider: "openai", ContextWindow: 1_050_000, MaxOutput: 128_000, InputCost1M: 0.20, OutputCost1M: 1.20, Description: "High-speed, ultra-low-cost tier with 1.05M context"},
	"gpt-5.5":       {ID: "gpt-5.5", Name: "GPT-5.5 Standard", Provider: "openai", ContextWindow: 500_000, MaxOutput: 128_000, InputCost1M: 5.00, OutputCost1M: 25.00, Description: "Previous-generation standard architecture"},
	"gpt-5.5-pro":   {ID: "gpt-5.5-pro", Name: "GPT-5.5 Pro", Provider: "openai", ContextWindow: 500_000, MaxOutput: 128_000, InputCost1M: 30.00, OutputCost1M: 180.00, Description: "Maintained for intensive reasoning capabilities"},
	"gpt-5.4":       {ID: "gpt-5.4", Name: "GPT-5.4 Standard", Provider: "openai", ContextWindow: 500_000, MaxOutput: 128_000, InputCost1M: 2.50, OutputCost1M: 15.00, Description: "Standard production text and localization"},
	"gpt-5.4-mini":  {ID: "gpt-5.4-mini", Name: "GPT-5.4 Mini", Provider: "openai", ContextWindow: 400_000, MaxOutput: 128_000, InputCost1M: 0.75, OutputCost1M: 4.50, Description: "Fast, efficient 400K context model"},
	"gpt-5.4-pro":   {ID: "gpt-5.4-pro", Name: "GPT-5.4 Pro", Provider: "openai", ContextWindow: 500_000, MaxOutput: 128_000, InputCost1M: 30.00, OutputCost1M: 180.00, Description: "Intensive reasoning and complex code refactoring"},
}

// estimateCost computes approximate cost in USD based on model pricing per 1M tokens
func estimateCost(model string, inputTokens, outputTokens int64) float64 {
	var inputCostPerMillion, outputCostPerMillion float64

	m := strings.ToLower(model)

	// Check exact match in registry first
	if spec, ok := ModelRegistry[m]; ok {
		inputCostPerMillion = spec.InputCost1M
		outputCostPerMillion = spec.OutputCost1M
	} else {
		// Heuristic matching
		switch {
		// Anthropic
		case strings.Contains(m, "claude-fable-5"):
			inputCostPerMillion = 10.00
			outputCostPerMillion = 50.00
		case strings.Contains(m, "claude-opus-5") || strings.Contains(m, "claude-opus-4"):
			inputCostPerMillion = 5.00
			outputCostPerMillion = 25.00
		case strings.Contains(m, "claude-sonnet-5"):
			inputCostPerMillion = 2.00
			outputCostPerMillion = 10.00
		case strings.Contains(m, "claude-sonnet-4.6") || strings.Contains(m, "claude-sonnet-4.5") || strings.Contains(m, "claude-3-7-sonnet") || strings.Contains(m, "claude-3-5-sonnet"):
			inputCostPerMillion = 3.00
			outputCostPerMillion = 15.00
		case strings.Contains(m, "claude-haiku"):
			inputCostPerMillion = 1.00
			outputCostPerMillion = 5.00

		// Gemini
		case strings.Contains(m, "gemini-3.1-pro"):
			inputCostPerMillion = 2.00
			outputCostPerMillion = 12.00
		case strings.Contains(m, "gemini-3.7-flash") || strings.Contains(m, "gemini-3.6-flash") || strings.Contains(m, "gemini-3.1-flash-live"):
			inputCostPerMillion = 0.75
			outputCostPerMillion = 3.75
		case strings.Contains(m, "gemini-3.5-flash-lite"):
			inputCostPerMillion = 0.30
			outputCostPerMillion = 2.50
		case strings.Contains(m, "gemini-3.5-flash"):
			inputCostPerMillion = 1.50
			outputCostPerMillion = 9.00
		case strings.Contains(m, "gemini-2.5-flash") || strings.Contains(m, "gemini-1.5-flash"):
			inputCostPerMillion = 0.10
			outputCostPerMillion = 0.40

		// OpenAI
		case strings.Contains(m, "gpt-5.6-sol"):
			inputCostPerMillion = 4.00
			outputCostPerMillion = 20.00
		case strings.Contains(m, "gpt-5.6-terra"):
			inputCostPerMillion = 2.00
			outputCostPerMillion = 12.00
		case strings.Contains(m, "gpt-5.6-luna"):
			inputCostPerMillion = 0.20
			outputCostPerMillion = 1.20
		case strings.Contains(m, "gpt-5.5-pro") || strings.Contains(m, "gpt-5.4-pro"):
			inputCostPerMillion = 30.00
			outputCostPerMillion = 180.00
		case strings.Contains(m, "gpt-5.5"):
			inputCostPerMillion = 5.00
			outputCostPerMillion = 25.00
		case strings.Contains(m, "gpt-5.4-mini"):
			inputCostPerMillion = 0.75
			outputCostPerMillion = 4.50
		case strings.Contains(m, "gpt-5.4"):
			inputCostPerMillion = 2.50
			outputCostPerMillion = 15.00
		case strings.Contains(m, "gpt-4o-mini"):
			inputCostPerMillion = 0.15
			outputCostPerMillion = 0.60
		case strings.Contains(m, "gpt-4o"):
			inputCostPerMillion = 2.50
			outputCostPerMillion = 10.00
		case strings.Contains(m, "deepseek"):
			inputCostPerMillion = 0.14
			outputCostPerMillion = 0.28
		default:
			// Default lightweight rate
			inputCostPerMillion = 0.50
			outputCostPerMillion = 1.50
		}
	}

	costIn := (float64(inputTokens) / 1_000_000.0) * inputCostPerMillion
	costOut := (float64(outputTokens) / 1_000_000.0) * outputCostPerMillion
	return costIn + costOut
}
