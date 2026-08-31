package genkit

import (
	"time"

	"github.com/langPeanut/langPeanut/pkg/chat"
	"github.com/langPeanut/langPeanut/pkg/types"
)

// GenkitFlowType identifies standard registered Genkit Flows
type GenkitFlowType string

const (
	FlowRepoCopilotChat     GenkitFlowType = "repoCopilotChatFlow"
	FlowScanRepository      GenkitFlowType = "scanRepositoryFlow"
	FlowPlanLocalization    GenkitFlowType = "planLocalizationFlow"
	FlowExecuteLocalization GenkitFlowType = "executeLocalizationFlow"
	FlowVerifyTranslations  GenkitFlowType = "verifyTranslationsFlow"
	FlowSEOSimulate         GenkitFlowType = "seoSimulateFlow"
	FlowManageCheckpoints   GenkitFlowType = "manageCheckpointsFlow"
	FlowManageConfig        GenkitFlowType = "manageConfigFlow"
	FlowDiagnoseSystem      GenkitFlowType = "diagnoseSystemFlow"
	FlowExplainConcept      GenkitFlowType = "explainConceptFlow"
)

// GenkitFlowInfo represents metadata about an active Genkit Flow
type GenkitFlowInfo struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputType   string         `json:"input_type"`
	OutputType  string         `json:"output_type"`
	Tools       []string       `json:"tools"`
	Schema      map[string]any `json:"schema,omitempty"`
}

// GenkitToolInfo represents metadata for a registered Genkit Tool
type GenkitToolInfo struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// GenkitChatRequest is the standard input payload for Genkit chat flows
type GenkitChatRequest struct {
	Message       string            `json:"message"`
	RepoID        int64             `json:"repo_id,omitempty"`
	ProjectRoot   string            `json:"project_root,omitempty"`
	Provider      string            `json:"provider,omitempty"`
	Model         string            `json:"model,omitempty"`
	SourceLocale  string            `json:"source_locale,omitempty"`
	TargetLocales []string          `json:"target_locales,omitempty"`
	ToneStyle     string            `json:"tone_style,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// GenkitChatResponse is the final structured response from a Genkit chat flow
type GenkitChatResponse struct {
	FlowName    string            `json:"flow_name"`
	Response    string            `json:"response"`
	Thoughts    []string          `json:"thoughts,omitempty"`
	ToolCalls   []chat.ToolCall   `json:"tool_calls,omitempty"`
	ToolResults []chat.ToolResult `json:"tool_results,omitempty"`
	Cards       []chat.UICard     `json:"cards,omitempty"`
	DurationMs  int64             `json:"duration_ms"`
	Timestamp   time.Time         `json:"timestamp"`
}

// GenkitStreamEvent is an SSE event emitted during Genkit flow execution
type GenkitStreamEvent struct {
	Type       string           `json:"type"` // "flow_start", "thought", "reasoning", "tool_start", "tool_end", "card", "chunk", "flow_end", "done", "error"
	FlowName   string           `json:"flow_name,omitempty"`
	Content    string           `json:"content,omitempty"`
	Reasoning  string           `json:"reasoning,omitempty"`
	ToolCall   *chat.ToolCall   `json:"tool_call,omitempty"`
	ToolResult *chat.ToolResult `json:"tool_result,omitempty"`
	Card       *chat.UICard     `json:"card,omitempty"`
	Error      string           `json:"error,omitempty"`
	Timestamp  int64            `json:"timestamp"`
}

// GenkitRuntimeInfo reports active Genkit runtime capabilities
type GenkitRuntimeInfo struct {
	Framework       string           `json:"framework"` // "Google Genkit Go"
	Version         string           `json:"version"`
	ActivePlugins   []string         `json:"active_plugins"`
	RegisteredFlows []GenkitFlowInfo `json:"registered_flows"`
	RegisteredTools []GenkitToolInfo `json:"registered_tools"`
	ActiveModel     string           `json:"active_model"`
	ActiveProvider  string           `json:"active_provider"`
}

// GenkitCandidatesProvider supplies string candidates from DB or AST
type GenkitCandidatesProvider interface {
	GetCandidates() []types.StringCandidate
}
