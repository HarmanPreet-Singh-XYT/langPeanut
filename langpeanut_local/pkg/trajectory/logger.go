package trajectory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/langPeanut/langPeanut/pkg/types"
)

// Logger records structured agent trajectories for the Hackathon submission
type Logger struct {
	mu          sync.Mutex
	steps       []types.TrajectoryStep
	outDir      string
	sessionName string
}

func NewLogger(outDir string, sessionName string) (*Logger, error) {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return nil, err
	}
	return &Logger{
		outDir:      outDir,
		sessionName: sessionName,
		steps:       make([]types.TrajectoryStep, 0),
	}, nil
}

// LogStep records an individual agent reasoning or tool action
func (l *Logger) LogStep(agentName, action, thought, toolCall string, toolInput, toolOutput any, criticFeedback string, retryCount int, passed bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	step := types.TrajectoryStep{
		StepIndex:      len(l.steps) + 1,
		Timestamp:      time.Now(),
		AgentName:      agentName,
		Action:         action,
		Thought:        thought,
		ToolCall:       toolCall,
		ToolInput:      toolInput,
		ToolOutput:     toolOutput,
		CriticFeedback: criticFeedback,
		RetryCount:     retryCount,
		PassedCheck:    passed,
	}

	l.steps = append(l.steps, step)
}

// ExportJSON writes the full structured trajectory to a JSON file
func (l *Logger) ExportJSON() (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	targetPath := filepath.Join(l.outDir, fmt.Sprintf("trajectory_%s.json", l.sessionName))
	data, err := json.MarshalIndent(l.steps, "", "  ")
	if err != nil {
		return "", err
	}

	err = os.WriteFile(targetPath, data, 0644)
	return targetPath, err
}

// ExportMarkdown writes a human-readable trajectory trace
func (l *Logger) ExportMarkdown() (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	targetPath := filepath.Join(l.outDir, fmt.Sprintf("trajectory_%s.md", l.sessionName))
	var sb fmt.Stringer
	_ = sb

	content := fmt.Sprintf("# Agent Trajectory Trace: %s\n\n", l.sessionName)
	content += fmt.Sprintf("> Generated at: %s | Total Steps: %d\n\n---\n\n", time.Now().Format(time.RFC3339), len(l.steps))

	for _, s := range l.steps {
		content += fmt.Sprintf("### Step %d: %s (`%s`)\n", s.StepIndex, s.Action, s.AgentName)
		content += fmt.Sprintf("- **Timestamp**: `%s`\n", s.Timestamp.Format("15:04:05.000"))
		if s.Thought != "" {
			content += fmt.Sprintf("- **Agent Thought**: *%s*\n", s.Thought)
		}
		if s.ToolCall != "" {
			content += fmt.Sprintf("- **Tool Call**: `%s`\n", s.ToolCall)
		}
		if s.CriticFeedback != "" {
			content += fmt.Sprintf("- **Critic Feedback**: ⚠️ `%s`\n", s.CriticFeedback)
		}
		if s.RetryCount > 0 {
			content += fmt.Sprintf("- **Retry Count**: %d\n", s.RetryCount)
		}
		content += fmt.Sprintf("- **Verification Status**: %v\n\n", s.PassedCheck)
	}

	err := os.WriteFile(targetPath, []byte(content), 0644)
	return targetPath, err
}
