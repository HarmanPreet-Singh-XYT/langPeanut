package chat

import (
	"time"

	"github.com/langPeanut/langPeanut/pkg/types"
)

// Role represents chat participant role
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
	RoleTool      Role = "tool"
)

// ChatMessage represents a single message in the conversational session
type ChatMessage struct {
	ID          string       `json:"id"`
	Role        Role         `json:"role"`
	Content     string       `json:"content"`
	Timestamp   time.Time    `json:"timestamp"`
	ToolCalls   []ToolCall   `json:"tool_calls,omitempty"`
	ToolResults []ToolResult `json:"tool_results,omitempty"`
	Cards       []UICard     `json:"cards,omitempty"`
}

// ToolCall represents an agentic tool invocation requested by the model
type ToolCall struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// ToolResult represents the output returned by a deterministic tool execution
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Name       string `json:"name"`
	Output     any    `json:"output"`
	Error      string `json:"error,omitempty"`
	IsError    bool   `json:"is_error"`
}

// CardType represents the kind of visual generative UI card rendered in chat
type CardType string

const (
	CardTypeMatrix       CardType = "matrix"        // Locale Coverage & Parity Matrix
	CardTypeSERP         CardType = "serp"          // 600px Desktop & Mobile Google SERP preview
	CardTypeDiff         CardType = "diff"          // AST Patch code syntax diff
	CardTypeCritic       CardType = "critic"        // 4-Tier Critic Scorecard
	CardTypeStepper      CardType = "stepper"       // Multi-step pipeline live progress
	CardTypeCost         CardType = "cost"          // Token consumption & USD cost estimate
	CardTypeCheckpoints  CardType = "checkpoints"   // Rollback snapshot timeline
	CardTypeConfig       CardType = "config"        // Project & Global Settings Inspector
	CardTypeHelp         CardType = "help"          // Documentation & Tool Introspection
	CardTypeActionButton CardType = "action_button" // Quick interactive action triggers
)

// UICard is a generative visual component embedded inside chat messages
type UICard struct {
	Type         CardType `json:"type"`
	Title        string   `json:"title"`
	Description  string   `json:"description,omitempty"`
	Data         any      `json:"data"`
	RenderedText string   `json:"rendered_text,omitempty"` // Formatted terminal/TUI representation
}

// MatrixCardData holds locale coverage breakdown
type MatrixCardData struct {
	ProjectRoot   string               `json:"project_root"`
	Framework     string               `json:"framework"`
	TotalKeys     int                  `json:"total_keys"`
	SourceLocale  string               `json:"source_locale"`
	Locales       []LocaleCoverageItem `json:"locales"`
	OverallHealth string               `json:"overall_health"`
}

// LocaleCoverageItem represents progress for a single language
type LocaleCoverageItem struct {
	LocaleCode   string  `json:"locale_code"`
	LocaleName   string  `json:"locale_name"`
	Translated   int     `json:"translated"`
	Total        int     `json:"total"`
	Percentage   float64 `json:"percentage"`
	MissingCount int     `json:"missing_count"`
	Status       string  `json:"status"` // "clean", "needs_translation", "unlocalized"
	FilePath     string  `json:"file_path"`
}

// SERPCardData holds Google search simulation preview data
type SERPCardData struct {
	Locale           string               `json:"locale"`
	TargetKeyword    string               `json:"target_keyword"`
	Title            string               `json:"title"`
	DisplayURL       string               `json:"display_url"`
	Snippet          string               `json:"snippet"`
	PixelWidth       int                  `json:"pixel_width"`
	IsPixelSafe      bool                 `json:"is_pixel_safe"`
	PredictedCTRGain float64  `json:"predicted_ctr_gain"`
	TrustScore       int      `json:"trust_score"`
	FAQSchema        []string `json:"faq_schema,omitempty"`
}

// DiffCardData holds AST code refactoring diffs
type DiffCardData struct {
	FilePath          string   `json:"file_path"`
	Framework         string   `json:"framework"`
	OriginalCode      string   `json:"original_code"`
	PatchedCode       string   `json:"patched_code"`
	DiffHunks         []string `json:"diff_hunks"`
	RequiredImports   []string `json:"required_imports"`
	RequiredHooks     []string `json:"required_hooks"`
	RemovedConstCount int      `json:"removed_const_count"`
}

// CriticCardData holds 4-tier verification scorecard details
type CriticCardData struct {
	OverallPassed bool               `json:"overall_passed"`
	Tier1Syntax   TierStatus         `json:"tier1_syntax"`
	Tier2ICU      TierStatus         `json:"tier2_icu"`
	Tier3Expansion TierStatus        `json:"tier3_expansion"`
	Tier4Parity   TierStatus         `json:"tier4_parity"`
	Diagnostics   []types.Diagnostic `json:"diagnostics,omitempty"`
}

// TierStatus summarizes a specific critic verification tier
type TierStatus struct {
	TierName     string `json:"tier_name"`
	Passed       bool   `json:"passed"`
	Summary      string `json:"summary"`
	WarningCount int    `json:"warning_count"`
}

// StepperCardData holds pipeline stage progress
type StepperCardData struct {
	CurrentStep int      `json:"current_step"`
	TotalSteps  int      `json:"total_steps"`
	StageName   string   `json:"stage_name"`
	Status      string   `json:"status"` // "pending", "running", "completed", "failed"
	StepLog     []string `json:"step_log"`
	ElapsedMs   int64    `json:"elapsed_ms"`
}

// CostCardData holds token and dollar usage breakdown
type CostCardData struct {
	ModelID          string  `json:"model_id"`
	Provider         string  `json:"provider"`
	InputTokens      int     `json:"input_tokens"`
	OutputTokens     int     `json:"output_tokens"`
	CachedReadTokens int     `json:"cached_read_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	EstimatedCostUSD float64 `json:"estimated_cost_usd"`
	SavingsPercent   float64 `json:"savings_percent"`
}

// CheckpointCardData holds snapshot timeline
type CheckpointCardData struct {
	Checkpoints []CheckpointItem `json:"checkpoints"`
	ActiveID    string           `json:"active_id,omitempty"`
}

// CheckpointItem represents a single snapshot
type CheckpointItem struct {
	ID        string    `json:"id"`
	Stage     string    `json:"stage"`
	Summary   string    `json:"summary"`
	CreatedAt time.Time `json:"created_at"`
	FileCount int       `json:"file_count"`
}

// ConfigCardData holds project & global configuration items
type ConfigCardData struct {
	ActiveProvider string          `json:"active_provider"`
	ActiveModel    string          `json:"active_model"`
	StylePreset    string          `json:"style_preset"`
	Concurrency    int             `json:"concurrency"`
	ChunkWords     int             `json:"chunk_words"`
	ChunkKeys      int             `json:"chunk_keys"`
	AutoGitignore  bool            `json:"auto_gitignore"`
	ProjectRoot    string          `json:"project_root"`
	APIKeyConfig   map[string]bool `json:"api_key_configured"`
}

// ActionButton represents a clickable UI trigger
type ActionButton struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	Action       string `json:"action"` // e.g. "apply_patch", "rollback", "translate_missing"
	Payload      any    `json:"payload,omitempty"`
	StyleVariant string `json:"style_variant"` // "primary", "secondary", "danger", "success"
}

// ChatEvent represents a real-time event for SSE streaming and TUI reactive updates
type ChatEvent struct {
	Type        string      `json:"type"` // "thought", "chunk", "tool_start", "tool_end", "card", "done", "error"
	Content     string      `json:"content,omitempty"`
	ToolCall    *ToolCall   `json:"tool_call,omitempty"`
	ToolResult  *ToolResult `json:"tool_result,omitempty"`
	Card        *UICard     `json:"card,omitempty"`
	Error       string      `json:"error,omitempty"`
}
