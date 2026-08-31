package genkit

import (
	"context"
	"fmt"

	"github.com/langPeanut/langPeanut/pkg/chat"
)

// GenkitToolHandler represents the execution logic of a Genkit tool
type GenkitToolHandler func(ctx context.Context, args map[string]any, engine *GenkitEngine) (any, *chat.UICard, error)

// GenkitTool encapsulates the metadata, JSON schema, and execution logic of a tool
type GenkitTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]any         `json:"parameters"`
	Handler     GenkitToolHandler      `json:"-"`
}

// GenkitToolRegistry stores registered Genkit tools
type GenkitToolRegistry struct {
	tools map[string]*GenkitTool
}

// NewGenkitToolRegistry initializes the Genkit tool registry with built-in tools
func NewGenkitToolRegistry() *GenkitToolRegistry {
	r := &GenkitToolRegistry{
		tools: make(map[string]*GenkitTool),
	}
	r.registerBuiltins()
	return r
}

// Register registers a tool
func (r *GenkitToolRegistry) Register(tool *GenkitTool) {
	r.tools[tool.Name] = tool
}

// Get retrieves a tool by name
func (r *GenkitToolRegistry) Get(name string) (*GenkitTool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// List returns all registered tools
func (r *GenkitToolRegistry) List() []*GenkitTool {
	list := make([]*GenkitTool, 0, len(r.tools))
	for _, t := range r.tools {
		list = append(list, t)
	}
	return list
}

// GetToolInfos returns metadata for all tools
func (r *GenkitToolRegistry) GetToolInfos() []GenkitToolInfo {
	var list []GenkitToolInfo
	for _, t := range r.tools {
		list = append(list, GenkitToolInfo{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Parameters,
		})
	}
	return list
}

func (r *GenkitToolRegistry) registerBuiltins() {
	// 1. scan_repository
	r.Register(&GenkitTool{
		Name:        "scan_repository",
		Description: "Scans project codebase with AST Scout, detects UI framework, extracts string candidates, and builds locale matrix.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Directory path to scan (defaults to current project root).",
				},
			},
		},
		Handler: func(ctx context.Context, args map[string]any, ge *GenkitEngine) (any, *chat.UICard, error) {
			toolDef, ok := ge.UnderlyingEngine.Tools.Get("scan_repository")
			if !ok {
				return nil, nil, fmt.Errorf("scan_repository tool not found in underlying engine")
			}
			return toolDef.Handler(ctx, args, ge.UnderlyingEngine)
		},
	})

	// 2. plan_localization
	r.Register(&GenkitTool{
		Name:        "plan_localization",
		Description: "Estimates token consumption, USD costs, and recommended batch chunk sizes for localization.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"locales": map[string]any{
					"type": "array",
					"items": map[string]any{"type": "string"},
					"description": "Target locale codes (e.g. ['es', 'de', 'ja']).",
				},
				"tone": map[string]any{
					"type":        "string",
					"description": "Tone persona (default, casual, formal, gen_z).",
				},
			},
		},
		Handler: func(ctx context.Context, args map[string]any, ge *GenkitEngine) (any, *chat.UICard, error) {
			toolDef, ok := ge.UnderlyingEngine.Tools.Get("plan_localization")
			if !ok {
				return nil, nil, fmt.Errorf("plan_localization tool not found")
			}
			return toolDef.Handler(ctx, args, ge.UnderlyingEngine)
		},
	})

	// 3. execute_localization
	r.Register(&GenkitTool{
		Name:        "execute_localization",
		Description: "Executes the full multi-agent localization pipeline with safety checkpoint, translation, 4-tier critic, and AST patching.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"locales": map[string]any{
					"type": "array",
					"items": map[string]any{"type": "string"},
					"description": "Target locales to translate.",
				},
				"tone": map[string]any{
					"type":        "string",
					"description": "Translation tone style.",
				},
				"dry_run": map[string]any{
					"type":        "boolean",
					"description": "If true, simulates execution and shows diffs without modifying files on disk.",
				},
			},
		},
		Handler: func(ctx context.Context, args map[string]any, ge *GenkitEngine) (any, *chat.UICard, error) {
			toolDef, ok := ge.UnderlyingEngine.Tools.Get("execute_localization")
			if !ok {
				return nil, nil, fmt.Errorf("execute_localization tool not found")
			}
			return toolDef.Handler(ctx, args, ge.UnderlyingEngine)
		},
	})

	// 4. verify_translations
	r.Register(&GenkitTool{
		Name:        "verify_translations",
		Description: "Runs the 4-Tier Critic (AST Syntax, ICU Variables, UI Expansion Risk, Key Parity) on locale catalogs.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"locales": map[string]any{
					"type": "array",
					"items": map[string]any{"type": "string"},
					"description": "Target locale codes to verify.",
				},
			},
		},
		Handler: func(ctx context.Context, args map[string]any, ge *GenkitEngine) (any, *chat.UICard, error) {
			toolDef, ok := ge.UnderlyingEngine.Tools.Get("verify_translations")
			if !ok {
				return nil, nil, fmt.Errorf("verify_translations tool not found")
			}
			return toolDef.Handler(ctx, args, ge.UnderlyingEngine)
		},
	})

	// 5. apply_ast_patch
	r.Register(&GenkitTool{
		Name:        "apply_ast_patch",
		Description: "Applies deterministic byte-range AST patches to source files and writes catalogs to disk.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"dry_run": map[string]any{
					"type":        "boolean",
					"description": "If true, generates diffs without disk write.",
				},
			},
		},
		Handler: func(ctx context.Context, args map[string]any, ge *GenkitEngine) (any, *chat.UICard, error) {
			toolDef, ok := ge.UnderlyingEngine.Tools.Get("apply_ast_patch")
			if !ok {
				return nil, nil, fmt.Errorf("apply_ast_patch tool not found")
			}
			return toolDef.Handler(ctx, args, ge.UnderlyingEngine)
		},
	})

	// 6. seo_simulate_serp
	r.Register(&GenkitTool{
		Name:        "seo_simulate_serp",
		Description: "Generates Google SERP desktop (600px) and mobile search previews with pixel-width safety checks and rich snippets.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"locale": map[string]any{
					"type":        "string",
					"description": "Target locale code.",
				},
				"keyword": map[string]any{
					"type":        "string",
					"description": "Target SEO keyword.",
				},
				"title": map[string]any{
					"type":        "string",
					"description": "Proposed title tag.",
				},
				"description": map[string]any{
					"type":        "string",
					"description": "Proposed meta description.",
				},
			},
			"required": []string{"locale", "title"},
		},
		Handler: func(ctx context.Context, args map[string]any, ge *GenkitEngine) (any, *chat.UICard, error) {
			toolDef, ok := ge.UnderlyingEngine.Tools.Get("seo_simulate_serp")
			if !ok {
				return nil, nil, fmt.Errorf("seo_simulate_serp tool not found")
			}
			return toolDef.Handler(ctx, args, ge.UnderlyingEngine)
		},
	})

	// 7. seo_analyze_competitor
	r.Register(&GenkitTool{
		Name:        "seo_analyze_competitor",
		Description: "Scrapes and analyzes competitor URLs or target markets to extract high-traffic regional keyword opportunities.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "Competitor website URL.",
				},
				"locale": map[string]any{
					"type":        "string",
					"description": "Target locale code.",
				},
			},
			"required": []string{"url"},
		},
		Handler: func(ctx context.Context, args map[string]any, ge *GenkitEngine) (any, *chat.UICard, error) {
			toolDef, ok := ge.UnderlyingEngine.Tools.Get("seo_analyze_competitor")
			if !ok {
				return nil, nil, fmt.Errorf("seo_analyze_competitor tool not found")
			}
			return toolDef.Handler(ctx, args, ge.UnderlyingEngine)
		},
	})

	// 8. manage_checkpoints
	r.Register(&GenkitTool{
		Name:        "manage_checkpoints",
		Description: "Lists snapshots, previews file diffs, or executes 1-click atomic rollback to undo changes.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type":        "string",
					"enum":        []string{"list", "restore", "diff"},
					"description": "Action to perform: 'list', 'restore', or 'diff'.",
				},
				"checkpoint_id": map[string]any{
					"type":        "string",
					"description": "ID of snapshot to restore.",
				},
			},
			"required": []string{"action"},
		},
		Handler: func(ctx context.Context, args map[string]any, ge *GenkitEngine) (any, *chat.UICard, error) {
			toolDef, ok := ge.UnderlyingEngine.Tools.Get("manage_checkpoints")
			if !ok {
				return nil, nil, fmt.Errorf("manage_checkpoints tool not found")
			}
			return toolDef.Handler(ctx, args, ge.UnderlyingEngine)
		},
	})

	// 9. manage_config
	r.Register(&GenkitTool{
		Name:        "manage_config",
		Description: "Inspects or updates active LLM provider, models, concurrency, chunk word budgets, or tone presets.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type":        "string",
					"enum":        []string{"get", "update"},
					"description": "'get' or 'update'.",
				},
				"provider": map[string]any{
					"type":        "string",
					"description": "LLM provider: gemini, claude, openai, ollama.",
				},
				"model": map[string]any{
					"type":        "string",
					"description": "Model ID.",
				},
			},
			"required": []string{"action"},
		},
		Handler: func(ctx context.Context, args map[string]any, ge *GenkitEngine) (any, *chat.UICard, error) {
			toolDef, ok := ge.UnderlyingEngine.Tools.Get("manage_config")
			if !ok {
				return nil, nil, fmt.Errorf("manage_config tool not found")
			}
			return toolDef.Handler(ctx, args, ge.UnderlyingEngine)
		},
	})

	// 10. diagnose_system
	r.Register(&GenkitTool{
		Name:        "diagnose_system",
		Description: "Runs comprehensive system doctor diagnostics: API keys, Tree-sitter parsers, Git config, TM caches.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: func(ctx context.Context, args map[string]any, ge *GenkitEngine) (any, *chat.UICard, error) {
			toolDef, ok := ge.UnderlyingEngine.Tools.Get("diagnose_system")
			if !ok {
				return nil, nil, fmt.Errorf("diagnose_system tool not found")
			}
			return toolDef.Handler(ctx, args, ge.UnderlyingEngine)
		},
	})

	// 11. explain_tool_or_concept
	r.Register(&GenkitTool{
		Name:        "explain_tool_or_concept",
		Description: "Provides in-depth architectural explanations of langPeanut tools, AST safety mechanisms, ICU formatting, and best practices.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"topic": map[string]any{
					"type":        "string",
					"description": "Topic or concept to explain.",
				},
			},
			"required": []string{"topic"},
		},
		Handler: func(ctx context.Context, args map[string]any, ge *GenkitEngine) (any, *chat.UICard, error) {
			toolDef, ok := ge.UnderlyingEngine.Tools.Get("explain_tool_or_concept")
			if !ok {
				return nil, nil, fmt.Errorf("explain_tool_or_concept tool not found")
			}
			return toolDef.Handler(ctx, args, ge.UnderlyingEngine)
		},
	})
}
