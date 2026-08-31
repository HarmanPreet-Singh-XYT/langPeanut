package chat

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/langPeanut/langPeanut/pkg/llm"
	"github.com/langPeanut/langPeanut/pkg/types"
)

func TestToolRegistry_Builtins(t *testing.T) {
	reg := NewToolRegistry()
	tools := reg.List()
	if len(tools) < 10 {
		t.Fatalf("expected at least 10 built-in tools, got %d", len(tools))
	}

	expected := []string{
		"scan_repository", "inspect_string_context", "find_hardcoded_strings",
		"plan_localization", "execute_localization", "verify_translations",
		"apply_ast_patch", "seo_analyze_competitor", "seo_simulate_serp",
		"seo_weave_copy", "manage_checkpoints", "manage_config", "diagnose_system",
	}

	for _, name := range expected {
		if _, ok := reg.Get(name); !ok {
			t.Errorf("expected tool '%s' to be registered", name)
		}
	}
}

func TestEngine_ScanAndMatrixCard(t *testing.T) {
	tempDir := t.TempDir()
	// Create sample React file
	reactFile := filepath.Join(tempDir, "App.tsx")
	_ = os.WriteFile(reactFile, []byte(`
export default function App() {
    return <h1>Welcome to langPeanut</h1>;
}
`), 0644)

	mockClient := llm.NewClient(llm.ProviderClaude, "claude-sonnet-5")
	engine, err := NewEngine(tempDir, mockClient)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	eventChan := make(chan ChatEvent, 50)
	msg, err := engine.SendMessage(context.Background(), "scan repository and check coverage", eventChan)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if msg.Role != RoleAssistant {
		t.Errorf("expected role assistant, got %s", msg.Role)
	}

	if len(msg.Cards) == 0 {
		t.Fatalf("expected at least 1 visual card generated, got %d", len(msg.Cards))
	}

	hasMatrix := false
	for _, c := range msg.Cards {
		if c.Type == CardTypeMatrix {
			hasMatrix = true
			if c.RenderedText == "" {
				t.Errorf("expected non-empty RenderedText for CLI/TUI")
			}
		}
	}
	if !hasMatrix {
		t.Errorf("expected CardTypeMatrix in response cards")
	}
}

func TestEngine_PlanLocalizationAndCostCard(t *testing.T) {
	tempDir := t.TempDir()
	mockClient := llm.NewClient(llm.ProviderClaude, "claude-sonnet-5")
	engine, err := NewEngine(tempDir, mockClient)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	engine.Candidates = []types.StringCandidate{
		{ID: "c1", Key: "btn_save", RawValue: "Save Settings"},
		{ID: "c2", Key: "hdr_welcome", RawValue: "Welcome"},
	}

	eventChan := make(chan ChatEvent, 50)
	msg, err := engine.SendMessage(context.Background(), "plan localization for spanish and german with casual tone", eventChan)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	hasCost := false
	for _, c := range msg.Cards {
		if c.Type == CardTypeCost {
			hasCost = true
			costData, ok := c.Data.(*CostCardData)
			if !ok || costData.TotalTokens <= 0 {
				t.Errorf("invalid cost card data: %+v", c.Data)
			}
		}
	}
	if !hasCost {
		t.Errorf("expected CardTypeCost in generated cards")
	}
}

func TestEngine_SEOSimulationCard(t *testing.T) {
	tempDir := t.TempDir()
	mockClient := llm.NewClient(llm.ProviderClaude, "claude-sonnet-5")
	engine, err := NewEngine(tempDir, mockClient)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	eventChan := make(chan ChatEvent, 50)
	msg, err := engine.SendMessage(context.Background(), "simulate japanese SEO SERP ranking", eventChan)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	hasSERP := false
	for _, c := range msg.Cards {
		if c.Type == CardTypeSERP {
			hasSERP = true
			serpData, ok := c.Data.(*SERPCardData)
			if !ok || serpData.PixelWidth <= 0 {
				t.Errorf("invalid SERP card data: %+v", c.Data)
			}
		}
	}
	if !hasSERP {
		t.Errorf("expected CardTypeSERP in generated cards")
	}
}
