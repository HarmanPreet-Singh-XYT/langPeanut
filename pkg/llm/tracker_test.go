package llm

import (
	"path/filepath"
	"testing"
)

func TestTokenTracker_RecordAndPersist(t *testing.T) {
	tempDir := t.TempDir()
	persistPath := filepath.Join(tempDir, "token_usage.json")

	tracker := NewTokenTracker(persistPath)

	// Record an OpenAI call
	tracker.Record("openai", "gpt-5.4-mini-2026-03-17", 1200, 800)

	// Record a Claude call
	tracker.Record("claude", "claude-3-7-sonnet-20250219", 2500, 1500)

	stats := tracker.GetStats()
	if stats.TotalInputTokens != 3700 {
		t.Fatalf("Expected TotalInputTokens=3700, got %d", stats.TotalInputTokens)
	}
	if stats.TotalOutputTokens != 2300 {
		t.Fatalf("Expected TotalOutputTokens=2300, got %d", stats.TotalOutputTokens)
	}
	if stats.TotalTokens != 6000 {
		t.Fatalf("Expected TotalTokens=6000, got %d", stats.TotalTokens)
	}
	if stats.TotalRequests != 2 {
		t.Fatalf("Expected TotalRequests=2, got %d", stats.TotalRequests)
	}
	if stats.TotalEstimatedCostUSD <= 0 {
		t.Fatalf("Expected non-zero estimated cost, got %f", stats.TotalEstimatedCostUSD)
	}

	// Verify persistence: create a new tracker from same file
	tracker2 := NewTokenTracker(persistPath)
	stats2 := tracker2.GetStats()
	if stats2.TotalTokens != 6000 {
		t.Fatalf("Expected reloaded TotalTokens=6000, got %d", stats2.TotalTokens)
	}
	if len(stats2.ByModel) != 2 {
		t.Fatalf("Expected 2 models in ByModel, got %d", len(stats2.ByModel))
	}

	// Reset
	tracker2.Reset()
	statsReset := tracker2.GetStats()
	if statsReset.TotalTokens != 0 {
		t.Fatalf("Expected TotalTokens=0 after reset, got %d", statsReset.TotalTokens)
	}
}

func TestEstimateTokens(t *testing.T) {
	text := "Hello world, this is a test of token estimation for langPeanut."
	tokens := EstimateTokens(text)
	if tokens < 5 || tokens > 25 {
		t.Fatalf("Expected realistic token estimate, got %d", tokens)
	}
}
