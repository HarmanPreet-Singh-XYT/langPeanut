package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/langPeanut/langPeanut/pkg/chat"
	"github.com/langPeanut/langPeanut/pkg/llm"
)

// ChatUIModel represents the interactive terminal chat session
type ChatUIModel struct {
	engine     *chat.Engine
	viewport   viewport.Model
	textInput  textinput.Model
	spinner    spinner.Model
	isThinking bool
	statusMsg  string
	width      int
	height     int
	messages   []chat.ChatMessage
	err        error
}

type chatStreamMsg struct {
	event chat.ChatEvent
}

type chatDoneMsg struct {
	msg *chat.ChatMessage
	err error
}

func NewChatUIModel(projectRoot string, client llm.Client) *ChatUIModel {
	engine, _ := chat.NewEngine(projectRoot, client)

	ti := textinput.New()
	ti.Placeholder = "Ask or request actions (e.g. 'Scan repository', 'Translate missing keys to German', 'SERP preview')"
	ti.Focus()
	ti.CharLimit = 500
	ti.Width = 80

	sp := spinner.New()
	sp.Spinner = spinner.Line
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))

	vp := viewport.New(80, 20)
	vp.SetContent(renderWelcomeBanner())

	return &ChatUIModel{
		engine:    engine,
		viewport:  vp,
		textInput: ti,
		spinner:   sp,
		messages:  make([]chat.ChatMessage, 0),
	}
}

func (m *ChatUIModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

func (m *ChatUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
		spCmd tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEsc:
			if m.textInput.Value() != "" {
				m.textInput.Reset()
				return m, nil
			}
			return m, tea.Quit
		case tea.KeyEnter:
			text := strings.TrimSpace(m.textInput.Value())
			if text != "" && !m.isThinking {
				m.textInput.Reset()
				m.isThinking = true
				m.statusMsg = "Routing deterministic tools..."
				m.messages = append(m.messages, chat.ChatMessage{
					Role:      chat.RoleUser,
					Content:   text,
					Timestamp: time.Now(),
				})
				m.updateViewport()
				return m, m.sendMessageCmd(text)
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 7
		m.textInput.Width = msg.Width - 6
		m.updateViewport()

	case chatStreamMsg:
		if msg.event.Type == "tool_start" && msg.event.ToolCall != nil {
			m.statusMsg = fmt.Sprintf("[TOOL: %s] executing...", msg.event.ToolCall.Name)
		} else if msg.event.Type == "tool_end" && msg.event.ToolResult != nil {
			m.statusMsg = fmt.Sprintf("[TOOL: %s] completed", msg.event.ToolResult.Name)
		}
		return m, m.spinner.Tick

	case chatDoneMsg:
		m.isThinking = false
		m.statusMsg = ""
		if msg.err != nil {
			m.err = msg.err
		} else if msg.msg != nil {
			m.messages = append(m.messages, *msg.msg)
		}
		m.updateViewport()
		m.viewport.GotoBottom()
		return m, nil

	case spinner.TickMsg:
		if m.isThinking {
			m.spinner, spCmd = m.spinner.Update(msg)
			return m, spCmd
		}
	}

	m.textInput, tiCmd = m.textInput.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)
	return m, tea.Batch(tiCmd, vpCmd)
}

func (m *ChatUIModel) sendMessageCmd(prompt string) tea.Cmd {
	return func() tea.Msg {
		eventChan := make(chan chat.ChatEvent, 50)
		msg, err := m.engine.SendMessage(context.Background(), prompt, eventChan)
		return chatDoneMsg{msg: msg, err: err}
	}
}

func (m *ChatUIModel) updateViewport() {
	var sb strings.Builder
	sb.WriteString(renderWelcomeBanner())
	sb.WriteString("\n\n")

	userStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39")).
		MarginBottom(1)

	assistantStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("255")).
		MarginBottom(1)

	toolChipStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("243")).
		Italic(true)

	for _, msg := range m.messages {
		if msg.Role == chat.RoleUser {
			sb.WriteString(userStyle.Render(fmt.Sprintf("> USER: %s", msg.Content)))
			sb.WriteString("\n\n")
		} else if msg.Role == chat.RoleAssistant {
			for _, tc := range msg.ToolCalls {
				sb.WriteString(toolChipStyle.Render(fmt.Sprintf("  [INVOKE] %s", tc.Name)))
				sb.WriteString("\n")
			}
			if len(msg.ToolCalls) > 0 {
				sb.WriteString("\n")
			}

			for _, card := range msg.Cards {
				if card.RenderedText != "" {
					sb.WriteString(card.RenderedText)
					sb.WriteString("\n\n")
				}
			}

			sb.WriteString(assistantStyle.Render(fmt.Sprintf("AGENT:\n%s", msg.Content)))
			sb.WriteString("\n\n────────────────────────────────────────────────────────────────────────\n\n")
		}
	}

	m.viewport.SetContent(sb.String())
	m.viewport.GotoBottom()
}

func (m *ChatUIModel) View() string {
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("255")).
		Background(lipgloss.Color("236")).
		Padding(0, 1)

	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginTop(1)

	statusLine := ""
	if m.isThinking {
		statusLine = fmt.Sprintf(" %s %s", m.spinner.View(), m.statusMsg)
	} else if m.err != nil {
		statusLine = fmt.Sprintf(" [ERROR] %v", m.err)
	} else {
		statusLine = " Type prompt or command. Press [Esc] to exit."
	}

	header := headerStyle.Render("langPeanut Autonomous Agent Orchestrator")
	return fmt.Sprintf("%s\n\n%s\n\n%s\n%s",
		header,
		m.viewport.View(),
		m.textInput.View(),
		footerStyle.Render(statusLine),
	)
}

func renderWelcomeBanner() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("255"))

	subStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))

	return fmt.Sprintf("%s\n%s\n\n%s",
		titleStyle.Render("LANGPEANUT AUTONOMOUS AGENT ORCHESTRATOR"),
		subStyle.Render("Deterministic multi-agent pipeline with AST verification and SERP growth engine."),
		"Commands & Queries:\n * \"Scan repository and report coverage\"\n * \"Translate missing keys to Spanish, German and Japanese\"\n * \"Execute 4-tier verification critic\"\n * \"Simulate Japanese Google SERP preview\"\n * \"List rollback snapshots or revert last run\"",
	)
}
