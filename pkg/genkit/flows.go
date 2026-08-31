package genkit

import (
	"context"
	"fmt"
	"time"

	"github.com/langPeanut/langPeanut/pkg/chat"
)

// GenkitFlowHandler executes a Genkit flow with streaming event dispatch
type GenkitFlowHandler func(ctx context.Context, input any, streamChan chan<- GenkitStreamEvent, engine *GenkitEngine) (any, error)

// GenkitFlow defines a typed, observable AI workflow in Genkit
type GenkitFlow struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	InputType   string            `json:"input_type"`
	OutputType  string            `json:"output_type"`
	Tools       []string          `json:"tools"`
	Handler     GenkitFlowHandler `json:"-"`
}

// GenkitFlowRegistry stores all registered Genkit flows
type GenkitFlowRegistry struct {
	flows map[string]*GenkitFlow
}

// NewGenkitFlowRegistry initializes registry with core Genkit flows
func NewGenkitFlowRegistry() *GenkitFlowRegistry {
	r := &GenkitFlowRegistry{
		flows: make(map[string]*GenkitFlow),
	}
	r.registerBuiltinFlows()
	return r
}

// Register registers a new flow
func (r *GenkitFlowRegistry) Register(flow *GenkitFlow) {
	r.flows[flow.Name] = flow
}

// Get retrieves a flow by name
func (r *GenkitFlowRegistry) Get(name string) (*GenkitFlow, bool) {
	f, ok := r.flows[name]
	return f, ok
}

// List returns all registered flows
func (r *GenkitFlowRegistry) List() []*GenkitFlow {
	list := make([]*GenkitFlow, 0, len(r.flows))
	for _, f := range r.flows {
		list = append(list, f)
	}
	return list
}

// GetFlowInfos returns metadata list for all flows
func (r *GenkitFlowRegistry) GetFlowInfos() []GenkitFlowInfo {
	var list []GenkitFlowInfo
	for _, f := range r.flows {
		list = append(list, GenkitFlowInfo{
			Name:        f.Name,
			Description: f.Description,
			InputType:   f.InputType,
			OutputType:  f.OutputType,
			Tools:       f.Tools,
		})
	}
	return list
}

func (r *GenkitFlowRegistry) registerBuiltinFlows() {
	// 1. repoCopilotChatFlow - Central Agentic Supervisor Flow
	r.Register(&GenkitFlow{
		Name:        string(FlowRepoCopilotChat),
		Description: "Central Autonomous Agentic Copilot Flow that coordinates AST Scout, Context Agent, Translator, 4-Tier Critic, and Patch Engine.",
		InputType:   "GenkitChatRequest",
		OutputType:  "GenkitChatResponse",
		Tools: []string{
			"scan_repository", "plan_localization", "execute_localization",
			"verify_translations", "apply_ast_patch", "seo_simulate_serp",
			"seo_analyze_competitor", "manage_checkpoints", "manage_config",
			"diagnose_system", "explain_tool_or_concept",
		},
		Handler: handleRepoCopilotChatFlow,
	})

	// 2. scanRepositoryFlow
	r.Register(&GenkitFlow{
		Name:        string(FlowScanRepository),
		Description: "AST Scout static analysis flow to discover hardcoded strings and compute locale coverage matrix.",
		InputType:   "map[string]any",
		OutputType:  "map[string]any",
		Tools:       []string{"scan_repository"},
		Handler: func(ctx context.Context, input any, streamChan chan<- GenkitStreamEvent, ge *GenkitEngine) (any, error) {
			args, _ := input.(map[string]any)
			emitGenkitEvent(streamChan, GenkitStreamEvent{
				Type:     "flow_start",
				FlowName: string(FlowScanRepository),
				Content:  "Starting AST Scout repository scan flow...",
			})
			tool, _ := ge.Tools.Get("scan_repository")
			out, card, err := tool.Handler(ctx, args, ge)
			if err != nil {
				return nil, err
			}
			if card != nil {
				emitGenkitEvent(streamChan, GenkitStreamEvent{Type: "card", Card: card})
			}
			emitGenkitEvent(streamChan, GenkitStreamEvent{
				Type:     "flow_end",
				FlowName: string(FlowScanRepository),
				Content:  "AST scan completed successfully.",
			})
			return out, nil
		},
	})

	// 3. verifyTranslationsFlow
	r.Register(&GenkitFlow{
		Name:        string(FlowVerifyTranslations),
		Description: "4-Tier Critic flow verifying AST syntax, ICU variables, UI expansion risk, and key parity.",
		InputType:   "map[string]any",
		OutputType:  "CriticCardData",
		Tools:       []string{"verify_translations"},
		Handler: func(ctx context.Context, input any, streamChan chan<- GenkitStreamEvent, ge *GenkitEngine) (any, error) {
			args, _ := input.(map[string]any)
			emitGenkitEvent(streamChan, GenkitStreamEvent{
				Type:     "flow_start",
				FlowName: string(FlowVerifyTranslations),
				Content:  "Running 4-Tier Critic verification flow...",
			})
			tool, _ := ge.Tools.Get("verify_translations")
			out, card, err := tool.Handler(ctx, args, ge)
			if err != nil {
				return nil, err
			}
			if card != nil {
				emitGenkitEvent(streamChan, GenkitStreamEvent{Type: "card", Card: card})
			}
			emitGenkitEvent(streamChan, GenkitStreamEvent{
				Type:     "flow_end",
				FlowName: string(FlowVerifyTranslations),
				Content:  "Critic verification completed.",
			})
			return out, nil
		},
	})

	// 4. seoSimulateFlow
	r.Register(&GenkitFlow{
		Name:        string(FlowSEOSimulate),
		Description: "Google SERP simulation flow with 600px desktop and mobile rendering checks.",
		InputType:   "map[string]any",
		OutputType:  "SERPCardData",
		Tools:       []string{"seo_simulate_serp"},
		Handler: func(ctx context.Context, input any, streamChan chan<- GenkitStreamEvent, ge *GenkitEngine) (any, error) {
			args, _ := input.(map[string]any)
			emitGenkitEvent(streamChan, GenkitStreamEvent{
				Type:     "flow_start",
				FlowName: string(FlowSEOSimulate),
				Content:  "Simulating Google SERP previews...",
			})
			tool, _ := ge.Tools.Get("seo_simulate_serp")
			out, card, err := tool.Handler(ctx, args, ge)
			if err != nil {
				return nil, err
			}
			if card != nil {
				emitGenkitEvent(streamChan, GenkitStreamEvent{Type: "card", Card: card})
			}
			emitGenkitEvent(streamChan, GenkitStreamEvent{
				Type:     "flow_end",
				FlowName: string(FlowSEOSimulate),
				Content:  "SERP simulation completed.",
			})
			return out, nil
		},
	})
}

func handleRepoCopilotChatFlow(ctx context.Context, input any, streamChan chan<- GenkitStreamEvent, ge *GenkitEngine) (any, error) {
	start := time.Now()
	req, ok := input.(GenkitChatRequest)
	if !ok {
		// Try parsing from map[string]any
		if m, isMap := input.(map[string]any); isMap {
			req.Message, _ = m["message"].(string)
			req.Provider, _ = m["provider"].(string)
			req.Model, _ = m["model"].(string)
		}
	}

	if req.Message == "" {
		return nil, fmt.Errorf("message is required in chat flow input")
	}

	emitGenkitEvent(streamChan, GenkitStreamEvent{
		Type:      "flow_start",
		FlowName:  string(FlowRepoCopilotChat),
		Content:   "Initializing Genkit agent flow execution...",
		Timestamp: time.Now().UnixMilli(),
	})

	// Emit initial reasoning/thought stream
	emitGenkitEvent(streamChan, GenkitStreamEvent{
		Type:      "thought",
		FlowName:  string(FlowRepoCopilotChat),
		Reasoning: fmt.Sprintf("Analyzing user directive: '%s'", req.Message),
		Content:   "Analyzing request & orchestrating tools...",
		Timestamp: time.Now().UnixMilli(),
	})

	// Bridge chat events from underlying engine to GenkitStreamEvents
	bridgeChan := make(chan chat.ChatEvent, 100)
	doneChan := make(chan struct{})

	var assistantMsg *chat.ChatMessage
	var runErr error

	go func() {
		defer close(doneChan)
		assistantMsg, runErr = ge.UnderlyingEngine.SendMessage(ctx, req.Message, bridgeChan)
	}()

	var toolCalls []chat.ToolCall
	var toolResults []chat.ToolResult
	var cards []chat.UICard
	var thoughts []string

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case ev, ok := <-bridgeChan:
			if !ok {
				break
			}
			switch ev.Type {
			case "thought":
				thoughts = append(thoughts, ev.Content)
				emitGenkitEvent(streamChan, GenkitStreamEvent{
					Type:      "thought",
					FlowName:  string(FlowRepoCopilotChat),
					Reasoning: ev.Content,
					Timestamp: time.Now().UnixMilli(),
				})
			case "tool_start":
				if ev.ToolCall != nil {
					toolCalls = append(toolCalls, *ev.ToolCall)
					emitGenkitEvent(streamChan, GenkitStreamEvent{
						Type:      "tool_start",
						FlowName:  string(FlowRepoCopilotChat),
						ToolCall:  ev.ToolCall,
						Content:   ev.Content,
						Timestamp: time.Now().UnixMilli(),
					})
				}
			case "tool_end":
				if ev.ToolResult != nil {
					toolResults = append(toolResults, *ev.ToolResult)
					emitGenkitEvent(streamChan, GenkitStreamEvent{
						Type:       "tool_end",
						FlowName:   string(FlowRepoCopilotChat),
						ToolCall:   ev.ToolCall,
						ToolResult: ev.ToolResult,
						Timestamp:  time.Now().UnixMilli(),
					})
				}
			case "card":
				if ev.Card != nil {
					cards = append(cards, *ev.Card)
					emitGenkitEvent(streamChan, GenkitStreamEvent{
						Type:      "card",
						FlowName:  string(FlowRepoCopilotChat),
						Card:      ev.Card,
						Timestamp: time.Now().UnixMilli(),
					})
				}
			case "chunk":
				emitGenkitEvent(streamChan, GenkitStreamEvent{
					Type:      "chunk",
					FlowName:  string(FlowRepoCopilotChat),
					Content:   ev.Content,
					Timestamp: time.Now().UnixMilli(),
				})
			case "done":
				emitGenkitEvent(streamChan, GenkitStreamEvent{
					Type:      "done",
					FlowName:  string(FlowRepoCopilotChat),
					Content:   ev.Content,
					Timestamp: time.Now().UnixMilli(),
				})
			case "error":
				emitGenkitEvent(streamChan, GenkitStreamEvent{
					Type:      "error",
					FlowName:  string(FlowRepoCopilotChat),
					Error:     ev.Error,
					Timestamp: time.Now().UnixMilli(),
				})
			}
		case <-doneChan:
			// Drain remaining events
			for len(bridgeChan) > 0 {
				ev := <-bridgeChan
				switch ev.Type {
				case "chunk":
					emitGenkitEvent(streamChan, GenkitStreamEvent{
						Type:      "chunk",
						FlowName:  string(FlowRepoCopilotChat),
						Content:   ev.Content,
						Timestamp: time.Now().UnixMilli(),
					})
				case "card":
					if ev.Card != nil {
						cards = append(cards, *ev.Card)
						emitGenkitEvent(streamChan, GenkitStreamEvent{
							Type:      "card",
							FlowName:  string(FlowRepoCopilotChat),
							Card:      ev.Card,
							Timestamp: time.Now().UnixMilli(),
						})
					}
				case "done":
					emitGenkitEvent(streamChan, GenkitStreamEvent{
						Type:      "done",
						FlowName:  string(FlowRepoCopilotChat),
						Content:   ev.Content,
						Timestamp: time.Now().UnixMilli(),
					})
				}
			}

			if runErr != nil {
				return nil, runErr
			}

			respContent := ""
			if assistantMsg != nil {
				respContent = assistantMsg.Content
				if len(toolCalls) == 0 && len(assistantMsg.ToolCalls) > 0 {
					toolCalls = assistantMsg.ToolCalls
				}
				if len(toolResults) == 0 && len(assistantMsg.ToolResults) > 0 {
					toolResults = assistantMsg.ToolResults
				}
				if len(cards) == 0 && len(assistantMsg.Cards) > 0 {
					cards = assistantMsg.Cards
				}
			}

			emitGenkitEvent(streamChan, GenkitStreamEvent{
				Type:      "flow_end",
				FlowName:  string(FlowRepoCopilotChat),
				Content:   respContent,
				Timestamp: time.Now().UnixMilli(),
			})

			return &GenkitChatResponse{
				FlowName:    string(FlowRepoCopilotChat),
				Response:    respContent,
				Thoughts:    thoughts,
				ToolCalls:   toolCalls,
				ToolResults: toolResults,
				Cards:       cards,
				DurationMs:  time.Since(start).Milliseconds(),
				Timestamp:   time.Now(),
			}, nil
		}
	}
}

func emitGenkitEvent(ch chan<- GenkitStreamEvent, ev GenkitStreamEvent) {
	if ch == nil {
		return
	}
	ch <- ev
}
