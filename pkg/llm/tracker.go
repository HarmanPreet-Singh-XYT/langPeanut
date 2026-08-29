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

// estimateCost computes approximate cost in USD based on model pricing per 1M tokens
func estimateCost(model string, inputTokens, outputTokens int64) float64 {
	var inputCostPerMillion, outputCostPerMillion float64

	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "gpt-5.4-mini") || strings.Contains(m, "gpt-4o-mini"):
		inputCostPerMillion = 0.15
		outputCostPerMillion = 0.60
	case strings.Contains(m, "gpt-4o"):
		inputCostPerMillion = 2.50
		outputCostPerMillion = 10.00
	case strings.Contains(m, "gpt-4.5") || strings.Contains(m, "o1"):
		inputCostPerMillion = 15.00
		outputCostPerMillion = 60.00
	case strings.Contains(m, "claude-3-7-sonnet") || strings.Contains(m, "claude-3-5-sonnet"):
		inputCostPerMillion = 3.00
		outputCostPerMillion = 15.00
	case strings.Contains(m, "claude-3-5-haiku"):
		inputCostPerMillion = 0.80
		outputCostPerMillion = 4.00
	case strings.Contains(m, "gemini-2.5-flash") || strings.Contains(m, "gemini-1.5-flash"):
		inputCostPerMillion = 0.075
		outputCostPerMillion = 0.30
	case strings.Contains(m, "gemini-1.5-pro") || strings.Contains(m, "gemini-2.0-pro"):
		inputCostPerMillion = 1.25
		outputCostPerMillion = 5.00
	case strings.Contains(m, "deepseek"):
		inputCostPerMillion = 0.14
		outputCostPerMillion = 0.28
	default:
		// Default lightweight rate
		inputCostPerMillion = 0.50
		outputCostPerMillion = 1.50
	}

	costIn := (float64(inputTokens) / 1_000_000.0) * inputCostPerMillion
	costOut := (float64(outputTokens) / 1_000_000.0) * outputCostPerMillion
	return costIn + costOut
}
