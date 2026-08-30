package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type LogLevel string

const (
	LevelDebug   LogLevel = "DEBUG"
	LevelInfo    LogLevel = "INFO"
	LevelWarn    LogLevel = "WARN"
	LevelError   LogLevel = "ERROR"
	LevelSuccess LogLevel = "SUCCESS"
)

// DiagnosticAdvice contains actionable knowledge for resolving system errors
type DiagnosticAdvice struct {
	Title        string   `json:"title"`
	Subsystem    string   `json:"subsystem"`
	RootCause    string   `json:"root_cause"`
	ActionSteps  []string `json:"action_steps"`
	AutoHealNote string   `json:"auto_heal_note,omitempty"`
}

// LogEvent represents a structured diagnostic event
type LogEvent struct {
	ID        int64             `json:"id"`
	Timestamp time.Time         `json:"timestamp"`
	Level     LogLevel          `json:"level"`
	Subsystem string            `json:"subsystem"`
	Message   string            `json:"message"`
	Details   map[string]any    `json:"details,omitempty"`
	Advice    *DiagnosticAdvice `json:"advice,omitempty"`
}

// DiagnosticLogger provides thread-safe, multi-sink structured logging with in-memory ring buffers and disk persistence
type DiagnosticLogger struct {
	mu          sync.RWMutex
	events      []LogEvent
	maxEvents   int
	nextID      int64
	subscribers []chan LogEvent
	logFilePath string
	fileWriter  *os.File
}

var (
	globalLogger *DiagnosticLogger
	loggerOnce   sync.Once
)

// Get returns the process-wide default diagnostic logger
func Get() *DiagnosticLogger {
	loggerOnce.Do(func() {
		home, _ := os.UserHomeDir()
		logDir := filepath.Join(home, ".langPeanut", "logs")
		_ = os.MkdirAll(logDir, 0755)
		logPath := filepath.Join(logDir, "langPeanut.log")

		f, _ := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)

		globalLogger = &DiagnosticLogger{
			events:      make([]LogEvent, 0, 1000),
			maxEvents:   1000,
			subscribers: make([]chan LogEvent, 0),
			logFilePath: logPath,
			fileWriter:  f,
		}
	})
	return globalLogger
}

// Log adds a structured log event with optional diagnostic advice
func (l *DiagnosticLogger) Log(level LogLevel, subsystem, msg string, details map[string]any, advice *DiagnosticAdvice) LogEvent {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.nextID++
	event := LogEvent{
		ID:        l.nextID,
		Timestamp: time.Now(),
		Level:     level,
		Subsystem: subsystem,
		Message:   msg,
		Details:   details,
		Advice:    advice,
	}

	if len(l.events) >= l.maxEvents {
		l.events = l.events[1:]
	}
	l.events = append(l.events, event)

	// Write to disk
	if l.fileWriter != nil {
		line := fmt.Sprintf("[%s] [%-7s] [%-12s] %s\n",
			event.Timestamp.Format("2006-01-02 15:04:05.000"),
			event.Level,
			event.Subsystem,
			event.Message)
		if advice != nil {
			line += fmt.Sprintf("   -> DIAGNOSTIC: %s (Cause: %s)\n", advice.Title, advice.RootCause)
			for _, step := range advice.ActionSteps {
				line += fmt.Sprintf("      * Fix: %s\n", step)
			}
		}
		_, _ = l.fileWriter.WriteString(line)
	}

	// Notify active subscribers non-blockingly
	for _, sub := range l.subscribers {
		select {
		case sub <- event:
		default:
		}
	}

	return event
}

func (l *DiagnosticLogger) Debug(subsystem, msg string, details ...map[string]any) LogEvent {
	var d map[string]any
	if len(details) > 0 {
		d = details[0]
	}
	return l.Log(LevelDebug, subsystem, msg, d, nil)
}

func (l *DiagnosticLogger) Info(subsystem, msg string, details ...map[string]any) LogEvent {
	var d map[string]any
	if len(details) > 0 {
		d = details[0]
	}
	return l.Log(LevelInfo, subsystem, msg, d, nil)
}

func (l *DiagnosticLogger) Warn(subsystem, msg string, details ...map[string]any) LogEvent {
	var d map[string]any
	if len(details) > 0 {
		d = details[0]
	}
	return l.Log(LevelWarn, subsystem, msg, d, nil)
}

func (l *DiagnosticLogger) Error(subsystem, msg string, err error, details ...map[string]any) LogEvent {
	var d map[string]any
	if len(details) > 0 {
		d = details[0]
	}
	advice := ExplainError(err)
	return l.Log(LevelError, subsystem, msg, d, advice)
}

func (l *DiagnosticLogger) Success(subsystem, msg string, details ...map[string]any) LogEvent {
	var d map[string]any
	if len(details) > 0 {
		d = details[0]
	}
	return l.Log(LevelSuccess, subsystem, msg, d, nil)
}

// GetRecent returns the last N log events
func (l *DiagnosticLogger) GetRecent(limit int) []LogEvent {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if limit <= 0 || limit > len(l.events) {
		limit = len(l.events)
	}
	out := make([]LogEvent, limit)
	copy(out, l.events[len(l.events)-limit:])
	return out
}

// Subscribe returns a channel that receives live log events
func (l *DiagnosticLogger) Subscribe() chan LogEvent {
	l.mu.Lock()
	defer l.mu.Unlock()
	ch := make(chan LogEvent, 100)
	l.subscribers = append(l.subscribers, ch)
	return ch
}

// Unsubscribe removes a listener channel
func (l *DiagnosticLogger) Unsubscribe(ch chan LogEvent) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i, sub := range l.subscribers {
		if sub == ch {
			l.subscribers = append(l.subscribers[:i], l.subscribers[i+1:]...)
			close(ch)
			break
		}
	}
}

// ExplainError produces comprehensive, human-readable troubleshooting knowledge from any error
func ExplainError(err error) *DiagnosticAdvice {
	if err == nil {
		return nil
	}
	str := err.Error()
	low := strings.ToLower(str)

	// 1. Anthropic Claude Errors
	if strings.Contains(low, "anthropic") || strings.Contains(low, "x-api-key") || strings.Contains(low, "claude") {
		if strings.Contains(low, "invalid x-api-key") || strings.Contains(low, "authentication_error") || strings.Contains(low, "401") {
			return &DiagnosticAdvice{
				Title:     "Anthropic Claude Authentication Failed",
				Subsystem: "MODEL:CLAUDE",
				RootCause: "The provided ANTHROPIC_API_KEY is invalid or lacks access to the requested Claude model.",
				ActionSteps: []string{
					"Open Settings (Press '8' in TUI or Settings tab in Web Studio) and enter your active ANTHROPIC_API_KEY.",
					"Verify your Anthropic API key at https://console.anthropic.com/settings/keys",
					"Or export ANTHROPIC_API_KEY=sk-ant-... in your environment before launching.",
				},
				AutoHealNote: "Switching to 'Meta NLLB-200 Local' enables 100% offline translation with zero API keys.",
			}
		}
		if strings.Contains(low, "credit_balance_too_low") || strings.Contains(low, "insufficient_quota") || strings.Contains(low, "402") {
			return &DiagnosticAdvice{
				Title:     "Anthropic API Credit Exhausted",
				Subsystem: "MODEL:CLAUDE",
				RootCause: "Your Anthropic account has run out of prepaid credits.",
				ActionSteps: []string{
					"Add prepaid credits to your Anthropic billing dashboard at https://console.anthropic.com/settings/plans",
					"Or switch to 'Meta NLLB-200 Local' in Settings (8) for free offline translations.",
				},
			}
		}
		if strings.Contains(low, "rate_limit") || strings.Contains(low, "429") {
			return &DiagnosticAdvice{
				Title:     "Anthropic Claude Rate Limit Exceeded",
				Subsystem: "MODEL:CLAUDE",
				RootCause: "Your request exceeded Anthropic's Tokens Per Minute (TPM) or Requests Per Minute (RPM) limits.",
				ActionSteps: []string{
					"langPeanut automatically applies exponential retry backoff.",
					"Wait 30 seconds before re-running full localization.",
					"Switch to 'Meta NLLB-200 Local' for zero rate limits and unlimited local batching.",
				},
				AutoHealNote: "Automatic retry with backoff is enabled.",
			}
		}
	}

	// 2. OpenAI Errors
	if strings.Contains(low, "openai") || strings.Contains(low, "gpt-4") || strings.Contains(low, "gpt-5") {
		if strings.Contains(low, "incorrect api key") || strings.Contains(low, "invalid_api_key") || strings.Contains(low, "401") {
			return &DiagnosticAdvice{
				Title:     "OpenAI API Authentication Failed",
				Subsystem: "MODEL:OPENAI",
				RootCause: "The provided OPENAI_API_KEY is incorrect, deactivated, or lacks project permissions.",
				ActionSteps: []string{
					"Open Settings (Press '8' in TUI or Settings tab in Web Studio) and enter your valid OPENAI_API_KEY.",
					"Verify your key at https://platform.openai.com/api-keys",
					"Ensure your project has active billing at https://platform.openai.com/account/billing/overview",
				},
				AutoHealNote: "Switch to 'Meta NLLB-200 Local' for zero-cost offline execution.",
			}
		}
		if strings.Contains(low, "insufficient_quota") || strings.Contains(low, "quota_exceeded") {
			return &DiagnosticAdvice{
				Title:     "OpenAI API Quota Limit Exceeded",
				Subsystem: "MODEL:OPENAI",
				RootCause: "You have exceeded your current OpenAI API quota or billing limit.",
				ActionSteps: []string{
					"Check your credit balance at https://platform.openai.com/account/billing/overview",
					"Switch to 'Meta NLLB-200 Local' in Settings (8) for offline translation.",
				},
			}
		}
	}

	// 3. NLLB-200 mBART Architecture Incompatibility with llama.cpp
	if strings.Contains(low, "mbart") || strings.Contains(low, "nllb-200 uses mbart") || strings.Contains(low, "gguf inference runtime not found") {
		return &DiagnosticAdvice{
			Title:     "NLLB-200 (mBART) Cannot Run via llama.cpp",
			Subsystem: "MODEL:NLLB_LOCAL",
			RootCause: "Meta NLLB-200 uses the mBART encoder-decoder architecture. llama.cpp only supports GPT-family decoder-only models (LLaMA, Mistral, Qwen). These are fundamentally different neural network designs.",
			ActionSteps: []string{
				"Option 1 — Zero-setup (Recommended): Switch to 'Meta NLLB-200 Cloud' in Settings. This runs the same Meta NLLB-200 600M model on Hugging Face's GPU servers using a free HF token.",
				"Option 2 — Local Ollama LLM: You have Ollama installed. Run 'ollama serve' then pull a multilingual model: 'ollama run qwen2.5:7b'. Select 'Custom / Ollama' in Settings.",
				"Option 3 — API Model: Use Anthropic Claude, OpenAI GPT-4o, or Google Gemini 2.5 Flash via API key in Settings.",
				"Option 4 — Advanced local NLLB server: Run a CTranslate2 Python server (pip install ctranslate2) and set NLLB_LOCAL_URL=http://localhost:8000.",
			},
			AutoHealNote: "Switch to 'Meta NLLB-200 Cloud' or 'Custom / Ollama' (with 'ollama serve') for real neural translation.",
		}
	}

	// 4. Google Gemini Errors
	if strings.Contains(low, "gemini") || strings.Contains(low, "generativelanguage") {
		if strings.Contains(low, "api key not valid") || strings.Contains(low, "permission_denied") || strings.Contains(low, "400") || strings.Contains(low, "403") {
			return &DiagnosticAdvice{
				Title:     "Google Gemini API Key Invalid",
				Subsystem: "MODEL:GEMINI",
				RootCause: "The configured GEMINI_API_KEY is not recognized by Google AI Studio.",
				ActionSteps: []string{
					"Generate a free API key at https://aistudio.google.com/app/apikey",
					"Enter the key in Settings (8) or export GEMINI_API_KEY=AIza...",
				},
				AutoHealNote: "Switch to 'Meta NLLB-200 Local' for zero-cost offline execution.",
			}
		}
	}

	// 4. DeepL Errors
	if strings.Contains(low, "deepl") {
		if strings.Contains(low, "403") || strings.Contains(low, "authorization") {
			return &DiagnosticAdvice{
				Title:     "DeepL API Authentication Failed",
				Subsystem: "MODEL:DEEPL",
				RootCause: "Invalid DEEPL_API_KEY or mismatch between DeepL Free (:fx suffix) and DeepL Pro endpoints.",
				ActionSteps: []string{
					"Free tier DeepL keys end with ':fx' and use 'api-free.deepl.com'.",
					"Pro tier DeepL keys use 'api.deepl.com'.",
					"Verify your subscription at https://www.deepl.com/pro-account/plan",
				},
			}
		}
		if strings.Contains(low, "456") || strings.Contains(low, "quota") {
			return &DiagnosticAdvice{
				Title:     "DeepL Monthly Translation Limit Reached",
				Subsystem: "MODEL:DEEPL",
				RootCause: "Your monthly character quota (500,000 chars on Free) has been exhausted.",
				ActionSteps: []string{
					"Switch to 'Meta NLLB-200 Local' or 'Meta NLLB-200 Cloud' in Settings (8) for unlimited translations.",
				},
			}
		}
	}

	// 5. Custom / Ollama / Remote Connection Failures
	if strings.Contains(low, "connection refused") || strings.Contains(low, "11434") || strings.Contains(low, "no such host") {
		return &DiagnosticAdvice{
			Title:     "Custom Endpoint / Ollama Unreachable",
			Subsystem: "MODEL:CUSTOM",
			RootCause: "langPeanut could not establish an HTTP connection to your local or remote LLM endpoint.",
			ActionSteps: []string{
				"If using local Ollama: Start the daemon with 'ollama serve' in another terminal.",
				"Check if your model is pulled: 'ollama run qwen2.5:32b' or 'ollama run llama3.3'.",
				"If using a custom remote URL: Update OPENAI_BASE_URL in Settings (8) (e.g. http://localhost:11434/v1).",
			},
			AutoHealNote: "Switch to 'Meta NLLB-200 Local' or 'Local Engine' for standalone offline execution.",
		}
	}

	// 6. Model Download 401 / Authorization
	if strings.Contains(low, "401") || strings.Contains(low, "unauthorized") {
		return &DiagnosticAdvice{
			Title:     "Hugging Face Authentication Required",
			Subsystem: "MODEL:NLLB",
			RootCause: "The requested Hugging Face model repository or API endpoint requires authorization credentials.",
			ActionSteps: []string{
				"Open Settings (Press '8' in TUI or navigate to Settings tab in Web Studio) and enter your HF_TOKEN.",
				"Or run 'export HF_TOKEN=your_huggingface_token' in your terminal before launching langPeanut.",
				"Ensure your Hugging Face token has 'read' permissions at https://huggingface.co/settings/tokens",
			},
			AutoHealNote: "Switching to 'Meta NLLB-200 Local' uses verified public CDN mirrors with zero token requirement.",
		}
	}

	// 7. Hugging Face 503 Model Loading / Warm-up
	if strings.Contains(low, "503") || strings.Contains(low, "loading") || strings.Contains(low, "estimated_time") {
		return &DiagnosticAdvice{
			Title:     "Hugging Face Serverless Cold Start",
			Subsystem: "MODEL:NLLB",
			RootCause: "The serverless Meta NLLB-200 model instance on Hugging Face is spinning up from cold storage.",
			ActionSteps: []string{
				"langPeanut automatically pauses and retries for up to 3 attempts with exponential backoff.",
				"For zero latency & offline autonomy, switch to 'Meta NLLB-200 Local' in Settings (8).",
			},
			AutoHealNote: "Autonomous retry with backoff is currently active.",
		}
	}

	// 8. Rate Limit 429
	if strings.Contains(low, "429") || strings.Contains(low, "rate limit") {
		return &DiagnosticAdvice{
			Title:     "AI Provider Rate Limit Exceeded",
			Subsystem: "MODEL:LLM",
			RootCause: "Your configured API provider returned HTTP 429 (Too Many Requests).",
			ActionSteps: []string{
				"Wait 30-60 seconds before re-running the pipeline.",
				"Check your quota or usage tier on the provider's dashboard.",
				"Switch to 'Meta NLLB-200 Local' or 'Local Engine' in Settings (8) for unlimited local throughput.",
			},
		}
	}

	// 9. Token Limit > 512
	if strings.Contains(low, "512") || strings.Contains(low, "token limit") || strings.Contains(low, "context length") {
		return &DiagnosticAdvice{
			Title:     "Input Text Exceeds Sequence Budget",
			Subsystem: "MODEL:NLLB",
			RootCause: "Meta NLLB-200 has an architectural constraint of 512 tokens per sequence.",
			ActionSteps: []string{
				"langPeanut automatically splits long text blocks into 350-word sentence chunks and reassembles them.",
				"If translating full documents, consider using Anthropic Claude or OpenAI in Settings (8).",
			},
			AutoHealNote: "Automatic sentence chunking & reassembly is enabled by default.",
		}
	}

	// 10. AST Validation / Syntax Error
	if strings.Contains(low, "in-memory ast validation failed") || strings.Contains(low, "patch engine syntax error") {
		return &DiagnosticAdvice{
			Title:     "AST Syntax Safety Guard Triggered",
			Subsystem: "PATCH:ENGINE",
			RootCause: "Applying the surgical string replacement would produce invalid syntax in the target file (e.g. malformed JSX closing tag or unescaped quote).",
			ActionSteps: []string{
				"langPeanut rejected the in-memory patch and preserved your original file untouched on disk.",
				"Inspect the file using 'langPeanut web' Diff Viewer or run with '--dry-run' to review candidate ranges.",
				"If the component has complex nested template literals, add an exclusion comment '// langPeanut-ignore' above the line.",
			},
			AutoHealNote: "Safety Guard: No corrupt or unparsable code was written to disk.",
		}
	}

	// 11. Missing API Key
	if strings.Contains(low, "api_key") || strings.Contains(low, "not set") || strings.Contains(low, "missing key") {
		return &DiagnosticAdvice{
			Title:     "Missing AI Provider Credentials",
			Subsystem: "CONFIG",
			RootCause: "The selected model engine requires an API credential that is not configured in your environment.",
			ActionSteps: []string{
				"Press '8' in the TUI to open Settings, navigate to Section 2 (API Keys), and press Enter to paste your key.",
				"Or enter the key directly in the Web Studio Settings tab.",
				"Or switch to 'Meta NLLB-200 Local' for 100% offline translation with zero API keys.",
			},
		}
	}

	// 12. Generic Fallback Advice
	return &DiagnosticAdvice{
		Title:     "Pipeline Diagnostic Warning",
		Subsystem: "PIPELINE",
		RootCause: str,
		ActionSteps: []string{
			"Review recent detailed logs in ~/.langPeanut/logs/langPeanut.log",
			"Run 'langPeanut translate --dry-run' to preview refactoring without altering source files.",
			"Check 'langPeanut status' to inspect project memory and candidate status.",
		},
	}
}

// FormatCLI formats diagnostic advice with ANSI styling for console display
func (d *DiagnosticAdvice) FormatCLI() string {
	if d == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n\033[1;31m[DIAGNOSTIC KNOWLEDGE] %s\033[0m\n", d.Title))
	sb.WriteString(fmt.Sprintf("  \033[90mSubsystem:\033[0m %s\n", d.Subsystem))
	sb.WriteString(fmt.Sprintf("  \033[90mRoot Cause:\033[0m %s\n", d.RootCause))
	if len(d.ActionSteps) > 0 {
		sb.WriteString("  \033[1;33mActionable Steps to Resolve:\033[0m\n")
		for _, step := range d.ActionSteps {
			sb.WriteString(fmt.Sprintf("    \033[36m•\033[0m %s\n", step))
		}
	}
	if d.AutoHealNote != "" {
		sb.WriteString(fmt.Sprintf("  \033[32m✓ Self-Healing Active:\033[0m %s\n", d.AutoHealNote))
	}
	return sb.String()
}
