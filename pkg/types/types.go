package types

import "time"

// Framework represents the supported target framework
type Framework string

const (
	FrameworkReact     Framework = "react"
	FrameworkNextJS    Framework = "nextjs"
	FrameworkReactNative Framework = "react_native"
	FrameworkFlutter   Framework = "flutter"
	FrameworkSwiftUI   Framework = "swiftui"
	FrameworkAndroid   Framework = "android"
	FrameworkVue       Framework = "vue"
	FrameworkAngular   Framework = "angular"
	FrameworkPython    Framework = "python"
	FrameworkGo        Framework = "go"
	FrameworkGeneric   Framework = "generic"
)

// Classification represents whether a string is UI, Code, or Uncertain
type Classification string

const (
	ClassLocalizable Classification = "LOCALIZABLE"
	ClassSkip        Classification = "SKIP"
	ClassUncertain   Classification = "UNCERTAIN"
)

// StringCandidate represents an extracted candidate string literal
type StringCandidate struct {
	ID             string         `json:"id"`
	FilePath       string         `json:"file_path"`
	StartByte      int            `json:"start_byte"`
	EndByte        int            `json:"end_byte"`
	StartLine      int            `json:"start_line"`
	StartCol       int            `json:"start_col"`
	EndLine        int            `json:"end_line"`
	EndCol         int            `json:"end_col"`
	RawValue       string         `json:"raw_value"`
	CleanValue     string         `json:"clean_value"`
	Key            string         `json:"key"`
	ParentNodeType string         `json:"parent_node_type"`
	ContextHint    string         `json:"context_hint"`
	SiblingStrings []string       `json:"sibling_strings,omitempty"`
	Classification Classification `json:"classification"`
	Confidence     float64        `json:"confidence"`
	Explanation    string         `json:"explanation,omitempty"`
	Variables      []string       `json:"variables,omitempty"`
	IsPlural       bool           `json:"is_plural"`
	PluralForms    map[string]string `json:"plural_forms,omitempty"`
	Approved       bool           `json:"approved"`
	IsConstContext bool           `json:"is_const_context,omitempty"`
	// ConstByteRange holds the exact [start,end) byte offsets of the `const`
	// keyword token that must be stripped once this candidate's string literal
	// is replaced with a non-constant lookup, when known precisely (e.g. from
	// AST-based extraction). Nil means no precise range was recorded.
	ConstByteRange *[2]int `json:"const_byte_range,omitempty"`
}

// ByteRangePatch represents a surgical code replacement in a file
type ByteRangePatch struct {
	FilePath        string `json:"file_path"`
	StartByte       int    `json:"start_byte"`
	EndByte         int    `json:"end_byte"`
	ReplacementText string `json:"replacement_text"`
	Description     string `json:"description"`
}

// FileRefactorPlan represents the complete refactoring actions for a single source file
type FileRefactorPlan struct {
	FilePath         string           `json:"file_path"`
	OriginalContent  string           `json:"original_content"`
	Patches          []ByteRangePatch `json:"patches"`
	RequiredImports  []string         `json:"required_imports"`
	RequiredHooks    []string         `json:"required_hooks"`
	RemoveConstBytes [][2]int         `json:"remove_const_bytes,omitempty"`
	RefactoredContent string          `json:"refactored_content,omitempty"`
}

// LocaleData represents key-value translations for a locale
type LocaleData struct {
	LocaleCode string            `json:"locale_code"`
	Format     string            `json:"format"` // arb, json_i18next, strings_xml, xcstrings
	Entries    map[string]string `json:"entries"`
	Metadata   map[string]any    `json:"metadata,omitempty"` // for ARB @key attributes
}

// VerificationTier represents the tier level in the 4-Tier Critic
type VerificationTier int

const (
	Tier1SyntaxAST VerificationTier = 1
	Tier2ICUTokens VerificationTier = 2
	Tier3UIExpansion VerificationTier = 3
	Tier4LocaleParity VerificationTier = 4
)

// Diagnostic represents an error or warning emitted by the Verifier Critic
type Diagnostic struct {
	Tier        VerificationTier `json:"tier"`
	Severity    string           `json:"severity"` // "ERROR", "WARNING", "INFO"
	Key         string           `json:"key,omitempty"`
	Locale      string           `json:"locale,omitempty"`
	Message     string           `json:"message"`
	Expected    string           `json:"expected,omitempty"`
	Actual      string           `json:"actual,omitempty"`
	CanAutoFix  bool             `json:"can_auto_fix"`
	AutoFixHint string           `json:"auto_fix_hint,omitempty"`
}

// VerificationReport contains all diagnostics from the 4-tier critic
type VerificationReport struct {
	Passed      bool         `json:"passed"`
	Diagnostics []Diagnostic `json:"diagnostics"`
	ErrorCount  int          `json:"error_count"`
	WarnCount   int          `json:"warn_count"`
}

// TrajectoryStep captures an individual step in the agent workflow for Hackathon Deliverable 04
type TrajectoryStep struct {
	StepIndex    int            `json:"step_index"`
	Timestamp    time.Time      `json:"timestamp"`
	AgentName    string         `json:"agent_name"`
	Action       string         `json:"action"`
	Thought      string         `json:"thought,omitempty"`
	ToolCall     string         `json:"tool_call,omitempty"`
	ToolInput    any            `json:"tool_input,omitempty"`
	ToolOutput   any            `json:"tool_output,omitempty"`
	CriticFeedback string       `json:"critic_feedback,omitempty"`
	RetryCount   int            `json:"retry_count,omitempty"`
	PassedCheck  bool           `json:"passed_check"`
}

// SessionState tracks the persistent session state for checkpoint and resume
type SessionState struct {
	SessionID        string                     `json:"session_id"`
	Stage            string                     `json:"stage"` // "init", "scanned", "classified", "refactored", "translated", "verified"
	CreatedAt        time.Time                  `json:"created_at"`
	UpdatedAt        time.Time                  `json:"updated_at"`
	ProjectRoot      string                     `json:"project_root"`
	DetectedFramework Framework                 `json:"detected_framework"`
	SourceLocale     string                     `json:"source_locale"`
	TargetLocales    []string                   `json:"target_locales"`
	Candidates       map[string]StringCandidate `json:"candidates"`
	RefactorPlans    map[string]FileRefactorPlan `json:"refactor_plans"`
	LocaleFiles      map[string]LocaleData      `json:"locale_files"`
	Trajectory       []TrajectoryStep           `json:"trajectory"`
	Checkpoints      []string                   `json:"checkpoints"`
}
