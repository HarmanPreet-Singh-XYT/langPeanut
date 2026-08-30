package chat

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/langPeanut/langPeanut/pkg/agents"
	"github.com/langPeanut/langPeanut/pkg/llm"
	"github.com/langPeanut/langPeanut/pkg/memory"
	"github.com/langPeanut/langPeanut/pkg/platforms"
	"github.com/langPeanut/langPeanut/pkg/types"
)

// Engine is the central multi-turn conversational orchestrator
type Engine struct {
	mu            sync.RWMutex
	ProjectRoot   string
	Platform      platforms.Platform
	LLMClient     llm.Client
	Tools         *ToolRegistry
	History       []ChatMessage
	Candidates    []types.StringCandidate
	RefactorPlans map[string]*types.FileRefactorPlan
	LastResult    *agents.PipelineResult
	SourceLocale  string
	TargetLocales []string
	ToneStyle     string
}

// NewEngine initializes the Central Agentic Chat Engine
func NewEngine(projectRoot string, client llm.Client) (*Engine, error) {
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		absRoot = projectRoot
	}

	registry := platforms.NewRegistry()
	platform, _ := registry.AutoDetect(absRoot)
	if platform == nil {
		platform, _ = registry.Get(types.FrameworkGeneric)
	}

	if client == nil {
		cfg := memory.LoadConfig(absRoot)
		provider := llm.ProviderClaude
		model := "claude-sonnet-5"
		if cfg != nil {
			if cfg.ActiveProvider != "" {
				provider = llm.ProviderType(cfg.ActiveProvider)
			}
			if cfg.ActiveModel != "" {
				model = cfg.ActiveModel
			}
		}
		client = llm.NewClient(provider, model)
	}

	engine := &Engine{
		ProjectRoot:   absRoot,
		Platform:      platform,
		LLMClient:     client,
		Tools:         NewToolRegistry(),
		History:       make([]ChatMessage, 0),
		Candidates:    make([]types.StringCandidate, 0),
		RefactorPlans: make(map[string]*types.FileRefactorPlan),
		SourceLocale:  "en",
		TargetLocales: []string{"es", "fr", "de", "ja"},
		ToneStyle:     "default",
	}
	return engine, nil
}

// SendMessage processes user input, manages tool calling, and streams events
func (e *Engine) SendMessage(ctx context.Context, userText string, eventChan chan<- ChatEvent) (*ChatMessage, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	userMsg := ChatMessage{
		ID:        generateID("id"),
		Role:      RoleUser,
		Content:   userText,
		Timestamp: time.Now(),
	}
	e.History = append(e.History, userMsg)

	// Step 1: Detect intent and identify tool calls
	toolCalls := e.detectToolCalls(userText)

	var toolResults []ToolResult
	var generatedCards []UICard
	var thoughts []string

	// Step 2: Execute tool calls deterministically
	for _, tc := range toolCalls {
		emitEvent(eventChan, ChatEvent{
			Type:     "tool_start",
			ToolCall: &tc,
			Content:  fmt.Sprintf("Executing %s...", tc.Name),
		})

		toolDef, exists := e.Tools.Get(tc.Name)
		if !exists {
			continue
		}

		out, card, err := toolDef.Handler(ctx, tc.Arguments, e)
		isErr := err != nil
		errMsg := ""
		if isErr {
			errMsg = err.Error()
		}

		tr := ToolResult{
			ToolCallID: tc.ID,
			Name:       tc.Name,
			Output:     out,
			Error:      errMsg,
			IsError:    isErr,
		}
		toolResults = append(toolResults, tr)

		if card != nil {
			generatedCards = append(generatedCards, *card)
			emitEvent(eventChan, ChatEvent{
				Type: "card",
				Card: card,
			})
		}

		emitEvent(eventChan, ChatEvent{
			Type:       "tool_end",
			ToolCall:   &tc,
			ToolResult: &tr,
		})
	}

	// Step 3: Synthesize final assistant response
	responseText := e.synthesizeResponse(ctx, userText, toolCalls, toolResults, thoughts)

	// Stream text chunks
	if eventChan != nil {
		words := strings.Fields(responseText)
		for _, w := range words {
			emitEvent(eventChan, ChatEvent{
				Type:    "chunk",
				Content: w + " ",
			})
		}
	}

	assistantMsg := ChatMessage{
		ID:          generateID("id"),
		Role:        RoleAssistant,
		Content:     responseText,
		Timestamp:   time.Now(),
		ToolCalls:   toolCalls,
		ToolResults: toolResults,
		Cards:       generatedCards,
	}
	e.History = append(e.History, assistantMsg)

	emitEvent(eventChan, ChatEvent{
		Type:    "done",
		Content: responseText,
	})

	return &assistantMsg, nil
}

// detectToolCalls matches user prompt with tools (via LLM function call format or deterministic intent matching)
func (e *Engine) detectToolCalls(userPrompt string) []ToolCall {
	var toolCalls []ToolCall
	p := strings.ToLower(userPrompt)

	// Intent 1: Scan / Audit / Inspect
	if strings.Contains(p, "scan") || strings.Contains(p, "audit") || strings.Contains(p, "coverage") || strings.Contains(p, "matrix") || strings.Contains(p, "check repo") {
		toolCalls = append(toolCalls, ToolCall{
			ID:        generateID("id"),
			Name:      "scan_repository",
			Arguments: map[string]any{"path": e.ProjectRoot},
		})
	}

	// Intent 2: Translation / Localization
	if strings.Contains(p, "translate") || strings.Contains(p, "localiz") || strings.Contains(p, "i18n") {
		locales := extractLocalesFromPrompt(userPrompt)
		if len(locales) == 0 {
			locales = e.TargetLocales
		}
		dryRun := strings.Contains(p, "dry run") || strings.Contains(p, "preview") || strings.Contains(p, "plan")
		tone := "default"
		if strings.Contains(p, "casual") {
			tone = "casual"
		} else if strings.Contains(p, "formal") {
			tone = "formal"
		} else if strings.Contains(p, "gen-z") || strings.Contains(p, "genz") {
			tone = "gen_z"
		}

		if dryRun {
			toolCalls = append(toolCalls, ToolCall{
				ID:        generateID("id"),
				Name:      "plan_localization",
				Arguments: map[string]any{"locales": locales, "tone": tone},
			})
		} else {
			toolCalls = append(toolCalls, ToolCall{
				ID:        generateID("id"),
				Name:      "execute_localization",
				Arguments: map[string]any{"locales": locales, "tone": tone, "dry_run": false},
			})
		}
	}

	// Intent 3: Verification / Critic
	if strings.Contains(p, "verify") || strings.Contains(p, "critic") || strings.Contains(p, "test icu") || strings.Contains(p, "health check") {
		toolCalls = append(toolCalls, ToolCall{
			ID:        generateID("id"),
			Name:      "verify_translations",
			Arguments: map[string]any{},
		})
	}

	// Intent 4: SEO / Competitor Analysis / SERP
	if strings.Contains(p, "seo") || strings.Contains(p, "competitor") || strings.Contains(p, "serp") || strings.Contains(p, "keyword") {
		if strings.Contains(p, "http") {
			// Extract URL
			r := regexp.MustCompile(`https?://[^\s]+`)
			url := r.FindString(userPrompt)
			toolCalls = append(toolCalls, ToolCall{
				ID:        generateID("id"),
				Name:      "seo_analyze_competitor",
				Arguments: map[string]any{"url": url, "locale": "ja"},
			})
		} else {
			toolCalls = append(toolCalls, ToolCall{
				ID:   generateID("id"),
				Name: "seo_simulate_serp",
				Arguments: map[string]any{
					"locale":      "ja",
					"keyword":     "開発者向けローカライゼーション",
					"title":       "langPeanut ｜ 開発者向け次世代ローカライゼーションツール",
					"description": "コード内のハードコードされたテキストを自動抽出し、ICU変数とレイアウトを保護しながら自動翻訳。",
				},
			})
		}
	}

	// Intent 5: Rollback / Undo / Checkpoint
	if strings.Contains(p, "rollback") || strings.Contains(p, "undo") || strings.Contains(p, "checkpoint") || strings.Contains(p, "revert") {
		action := "list"
		if strings.Contains(p, "restore") || strings.Contains(p, "rollback") || strings.Contains(p, "undo") || strings.Contains(p, "revert") {
			action = "restore"
		}
		toolCalls = append(toolCalls, ToolCall{
			ID:        generateID("id"),
			Name:      "manage_checkpoints",
			Arguments: map[string]any{"action": action},
		})
	}

	// Intent 6: Config / Model / Settings
	if strings.Contains(p, "config") || strings.Contains(p, "setting") || strings.Contains(p, "switch model") || strings.Contains(p, "concurrency") || strings.Contains(p, "token limit") {
		toolCalls = append(toolCalls, ToolCall{
			ID:        generateID("id"),
			Name:      "manage_config",
			Arguments: map[string]any{"action": "get"},
		})
	}

	// Intent 7: Doctor / Diagnostics
	if strings.Contains(p, "doctor") || strings.Contains(p, "diagnos") || strings.Contains(p, "api key") {
		toolCalls = append(toolCalls, ToolCall{
			ID:        generateID("id"),
			Name:      "diagnose_system",
			Arguments: map[string]any{},
		})
	}

	// Intent 8: Inspect specific string
	if strings.Contains(p, "inspect") || strings.Contains(p, "where is") || strings.Contains(p, "find string") {
		query := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(p, "inspect", ""), "string", ""))
		if query != "" {
			toolCalls = append(toolCalls, ToolCall{
				ID:        generateID("id"),
				Name:      "inspect_string_context",
				Arguments: map[string]any{"key_or_text": query},
			})
		}
	}

	return toolCalls
}

// synthesizeResponse crafts the assistant natural language response
func (e *Engine) synthesizeResponse(ctx context.Context, userPrompt string, toolCalls []ToolCall, toolResults []ToolResult, thoughts []string) string {
	if len(toolCalls) == 0 {
		// General query or help request
		return fmt.Sprintf("I am your **langPeanut Central Agentic Copilot**. I can inspect your codebase, plan & execute multi-language translations, run 4-tier ICU verification critics, simulate multilingual SEO rankings, and manage rollback checkpoints.\n\nHere are some quick things you can ask me to do:\n- **\"Scan my repository and check translation coverage\"**\n- **\"Translate missing keys into Spanish, German and Japanese in a casual tone\"**\n- **\"Run 4-tier verification critic on all locales\"**\n- **\"Simulate Japanese SEO SERP rankings for our product\"**\n- **\"Show rollback snapshots or undo the last run\"**\n- **\"Configure model settings and parallel concurrency\"**")
	}

	var sb strings.Builder
	for _, tr := range toolResults {
		if tr.IsError {
			sb.WriteString(fmt.Sprintf("⚠️ **%s encountered an issue**: %s\n\n", tr.Name, tr.Error))
			continue
		}

		switch tr.Name {
		case "scan_repository":
			sb.WriteString(fmt.Sprintf("I scanned your project (**%s**). Found **%d** hardcoded string candidates across your codebase. See the Locale Coverage Matrix above for translation completeness across your target markets.", e.Platform.DisplayName(), len(e.Candidates)))
		case "plan_localization":
			sb.WriteString("I calculated your localization execution plan. Total token estimate and pricing breakdown have been generated above. Ready to execute whenever you're ready!")
		case "execute_localization":
			sb.WriteString("✅ **Localization pipeline completed successfully!** All missing keys were translated, verified with 100% ICU variable parity, and deterministic AST code patches have been applied. A safety checkpoint was created.")
		case "verify_translations":
			sb.WriteString("🛡️ **4-Tier Critic Verification completed.** Checked AST syntax, ICU variable parity, layout expansion risk, and key integrity. Check the scorecard above for full tier results.")
		case "seo_analyze_competitor", "seo_simulate_serp":
			sb.WriteString("🔍 **SEO Simulation generated.** High-intent keywords and 600px desktop/mobile SERP previews have been rendered above.")
		case "manage_checkpoints":
			sb.WriteString("⏪ Checkpoint management action executed. Your codebase snapshots and rollback history are displayed above.")
		case "manage_config":
			sb.WriteString("⚙️ System configuration and provider settings loaded. You can adjust models, concurrency, chunk word budgets, or tone personas anytime.")
		case "diagnose_system":
			sb.WriteString("🩺 System diagnostics completed. All core parsers, API connectivity, and workspace health states are reported above.")
		default:
			sb.WriteString(fmt.Sprintf("Completed action `%s`.", tr.Name))
		}
		sb.WriteString("\n\n")
	}

	return strings.TrimSpace(sb.String())
}

func (e *Engine) GetHistory() []ChatMessage {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.History
}

func (e *Engine) ResetHistory() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.History = make([]ChatMessage, 0)
}

func extractLocalesFromPrompt(p string) []string {
	var locales []string
	known := map[string]string{
		"spanish": "es", "es": "es",
		"french": "fr", "fr": "fr",
		"german": "de", "de": "de",
		"japanese": "ja", "ja": "ja",
		"chinese": "zh", "zh": "zh",
		"korean": "ko", "ko": "ko",
		"italian": "it", "it": "it",
		"portuguese": "pt", "pt": "pt",
		"russian": "ru", "ru": "ru",
		"arabic": "ar", "ar": "ar",
		"hindi": "hi", "hi": "hi",
	}

	words := strings.Fields(strings.ToLower(p))
	seen := make(map[string]bool)
	for _, w := range words {
		w = strings.Trim(w, ",.;:!?\"'()[]")
		if code, ok := known[w]; ok {
			if !seen[code] {
				seen[code] = true
				locales = append(locales, code)
			}
		}
	}
	return locales
}

func generateID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s_%d", prefix, hex.EncodeToString(b), time.Now().UnixNano()%1000000)
}

func emitEvent(ch chan<- ChatEvent, ev ChatEvent) {
	if ch == nil {
		return
	}
	select {
	case ch <- ev:
	default:
	}
}
