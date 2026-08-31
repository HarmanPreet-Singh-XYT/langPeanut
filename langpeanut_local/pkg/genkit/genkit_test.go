package genkit

import (
	"context"
	"testing"
	"time"

	"github.com/langPeanut/langPeanut/pkg/chat"
	"github.com/langPeanut/langPeanut/pkg/types"
)

func TestGenkitEngine_Initialization(t *testing.T) {
	engine, err := NewGenkitEngine(".", nil)
	if err != nil {
		t.Fatalf("failed initializing Genkit engine: %v", err)
	}

	info := engine.GetRuntimeInfo()
	if info.Framework != "Google Genkit Go" {
		t.Errorf("expected framework 'Google Genkit Go', got '%s'", info.Framework)
	}

	if len(info.RegisteredFlows) == 0 {
		t.Errorf("expected registered flows, got 0")
	}

	if len(info.RegisteredTools) == 0 {
		t.Errorf("expected registered tools, got 0")
	}

	t.Logf("Genkit initialized: %d flows, %d tools, active plugins: %v",
		len(info.RegisteredFlows), len(info.RegisteredTools), info.ActivePlugins)
}

func TestGenkitEngine_ChatFlowStreaming(t *testing.T) {
	engine, err := NewGenkitEngine(".", nil)
	if err != nil {
		t.Fatalf("failed initializing Genkit engine: %v", err)
	}

	// Inject dummy string candidates for coverage test
	engine.UnderlyingEngine.Candidates = []types.StringCandidate{
		{
			Key:        "home.title",
			RawValue:   "Welcome to our platform",
			CleanValue: "Welcome to our platform",
			FilePath:   "app/page.tsx",
			StartLine:  10,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	streamChan := make(chan GenkitStreamEvent, 50)
	doneChan := make(chan struct{})

	var events []GenkitStreamEvent
	go func() {
		defer close(doneChan)
		for ev := range streamChan {
			events = append(events, ev)
		}
	}()

	resp, err := engine.SendChatMessage(ctx, "Scan repository and show coverage matrix", streamChan)
	close(streamChan)
	<-doneChan

	if err != nil {
		t.Fatalf("chat flow error: %v", err)
	}

	if resp == nil {
		t.Fatalf("expected non-nil response")
	}

	if resp.FlowName != string(FlowRepoCopilotChat) {
		t.Errorf("expected flow name '%s', got '%s'", FlowRepoCopilotChat, resp.FlowName)
	}

	if len(events) == 0 {
		t.Errorf("expected streamed events, got 0")
	}

	hasToolStart := false
	hasCard := false
	hasDone := false

	for _, ev := range events {
		if ev.Type == "tool_start" {
			hasToolStart = true
		}
		if ev.Type == "card" {
			hasCard = true
		}
		if ev.Type == "done" || ev.Type == "flow_end" {
			hasDone = true
		}
	}

	if !hasToolStart {
		t.Errorf("expected tool_start event in stream")
	}
	if !hasCard {
		t.Errorf("expected card event in stream")
	}
	if !hasDone {
		t.Errorf("expected done / flow_end event in stream")
	}

	t.Logf("Chat flow executed cleanly with %d stream events and %d cards", len(events), len(resp.Cards))
}

func TestGenkitEngine_StandaloneFlows(t *testing.T) {
	engine, err := NewGenkitEngine(".", nil)
	if err != nil {
		t.Fatalf("failed initializing Genkit engine: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Test verifyTranslationsFlow
	verifyRes, err := engine.RunFlow(ctx, string(FlowVerifyTranslations), map[string]any{}, nil)
	if err != nil {
		t.Fatalf("verifyTranslationsFlow error: %v", err)
	}
	if verifyRes == nil {
		t.Errorf("expected non-nil verify result")
	}

	// Test seoSimulateFlow
	seoRes, err := engine.RunFlow(ctx, string(FlowSEOSimulate), map[string]any{
		"locale": "ja",
		"title":  "Test Title for Japanese",
	}, nil)
	if err != nil {
		t.Fatalf("seoSimulateFlow error: %v", err)
	}
	if seoRes == nil {
		t.Errorf("expected non-nil seo result")
	}
}

func TestGenkitEngine_ToolsRegistry(t *testing.T) {
	engine, err := NewGenkitEngine(".", nil)
	if err != nil {
		t.Fatalf("failed initializing Genkit engine: %v", err)
	}

	tools := engine.ListTools()
	if len(tools) < 8 {
		t.Errorf("expected at least 8 registered tools, got %d", len(tools))
	}

	tool, exists := engine.Tools.Get("scan_repository")
	if !exists || tool == nil {
		t.Fatalf("expected 'scan_repository' tool to exist")
	}

	if tool.Description == "" {
		t.Errorf("expected tool description to be non-empty")
	}
}

func TestGenkitEngine_ExplainConcept(t *testing.T) {
	engine, err := NewGenkitEngine(".", nil)
	if err != nil {
		t.Fatalf("failed initializing Genkit engine: %v", err)
	}

	ctx := context.Background()
	tool, exists := engine.Tools.Get("explain_tool_or_concept")
	if !exists {
		t.Fatalf("tool explain_tool_or_concept missing")
	}

	out, card, err := tool.Handler(ctx, map[string]any{"topic": "AST Patching"}, engine)
	if err != nil {
		t.Fatalf("explain concept handler failed: %v", err)
	}

	if out == nil || card == nil {
		t.Fatalf("expected non-nil output and card")
	}

	if card.Type != chat.CardTypeHelp {
		t.Errorf("expected CardTypeHelp, got %v", card.Type)
	}
}
