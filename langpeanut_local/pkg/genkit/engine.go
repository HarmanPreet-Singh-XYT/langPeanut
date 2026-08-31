package genkit

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/langPeanut/langPeanut/pkg/chat"
	"github.com/langPeanut/langPeanut/pkg/llm"
	"github.com/langPeanut/langPeanut/pkg/memory"
)

// GenkitEngine orchestrates Google Genkit Go flows, tools, and telemetry
type GenkitEngine struct {
	mu               sync.RWMutex
	ProjectRoot      string
	UnderlyingEngine *chat.Engine
	Flows            *GenkitFlowRegistry
	Tools            *GenkitToolRegistry
	ActiveProvider   string
	ActiveModel      string
	Plugins          []string
}

// NewGenkitEngine initializes the Google Genkit engine for a project
func NewGenkitEngine(projectRoot string, client llm.Client) (*GenkitEngine, error) {
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		absRoot = projectRoot
	}

	underlying, err := chat.NewEngine(absRoot, client)
	if err != nil {
		return nil, fmt.Errorf("failed creating underlying chat engine: %w", err)
	}

	provider := "gemini"
	model := "gemini-3.7-flash"
	cfg := memory.LoadConfig(absRoot)
	if cfg != nil {
		if cfg.ActiveProvider != "" {
			provider = cfg.ActiveProvider
		}
		if cfg.ActiveModel != "" {
			model = cfg.ActiveModel
		}
	} else if client != nil {
		provider = string(client.Name())
	}

	ge := &GenkitEngine{
		ProjectRoot:      absRoot,
		UnderlyingEngine: underlying,
		Flows:            NewGenkitFlowRegistry(),
		Tools:            NewGenkitToolRegistry(),
		ActiveProvider:   provider,
		ActiveModel:      model,
		Plugins: []string{
			"googleai/gemini",
			"anthropic/claude",
			"openai/gpt",
			"ollama/local",
			"genkit/tracing",
		},
	}

	return ge, nil
}

// RunFlow executes a registered Genkit Flow with real-time streaming
func (ge *GenkitEngine) RunFlow(ctx context.Context, flowName string, input any, streamChan chan<- GenkitStreamEvent) (any, error) {
	ge.mu.RLock()
	flow, exists := ge.Flows.Get(flowName)
	ge.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("genkit flow '%s' is not registered", flowName)
	}

	return flow.Handler(ctx, input, streamChan, ge)
}

// SendChatMessage runs the Central Agentic Copilot Chat Flow
func (ge *GenkitEngine) SendChatMessage(ctx context.Context, userText string, streamChan chan<- GenkitStreamEvent) (*GenkitChatResponse, error) {
	req := GenkitChatRequest{
		Message:      userText,
		ProjectRoot:  ge.ProjectRoot,
		Provider:     ge.ActiveProvider,
		Model:        ge.ActiveModel,
		SourceLocale: ge.UnderlyingEngine.SourceLocale,
		TargetLocales: ge.UnderlyingEngine.TargetLocales,
		ToneStyle:    ge.UnderlyingEngine.ToneStyle,
	}

	res, err := ge.RunFlow(ctx, string(FlowRepoCopilotChat), req, streamChan)
	if err != nil {
		return nil, err
	}

	if chatResp, ok := res.(*GenkitChatResponse); ok {
		return chatResp, nil
	}
	return nil, fmt.Errorf("unexpected return type from chat flow: %T", res)
}

// GetRuntimeInfo returns metadata on runtime capabilities, plugins, and registered flows/tools
func (ge *GenkitEngine) GetRuntimeInfo() GenkitRuntimeInfo {
	ge.mu.RLock()
	defer ge.mu.RUnlock()

	return GenkitRuntimeInfo{
		Framework:       "Google Genkit Go",
		Version:         "v1.12.0",
		ActivePlugins:   ge.Plugins,
		RegisteredFlows: ge.Flows.GetFlowInfos(),
		RegisteredTools: ge.Tools.GetToolInfos(),
		ActiveModel:     ge.ActiveModel,
		ActiveProvider:  ge.ActiveProvider,
	}
}

// ListFlows returns all registered Genkit Flows
func (ge *GenkitEngine) ListFlows() []GenkitFlowInfo {
	return ge.Flows.GetFlowInfos()
}

// ListTools returns all registered Genkit Tools
func (ge *GenkitEngine) ListTools() []GenkitToolInfo {
	return ge.Tools.GetToolInfos()
}

// GetUnderlyingEngine returns access to underlying AST candidates and history
func (ge *GenkitEngine) GetUnderlyingEngine() *chat.Engine {
	return ge.UnderlyingEngine
}
