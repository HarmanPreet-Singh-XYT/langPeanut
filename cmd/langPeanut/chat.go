package main

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/langPeanut/langPeanut/pkg/llm"
	"github.com/langPeanut/langPeanut/pkg/tui"
	"github.com/spf13/cobra"
)

var (
	chatProvider string
	chatModel    string
	chatTone     string
)

var chatCmd = &cobra.Command{
	Use:     "chat [directory]",
	Aliases: []string{"copilot", "ask", "ai"},
	Short:   "Launch the Central Agentic Chat Copilot for natural language localization & ops",
	Long: `Launch the interactive Central Agentic Chat Copilot.

Talk directly with the central autonomous supervisor agent to scan repos, translate
missing keys, run 4-tier verification critics, simulate Google SERP rankings, manage
rollback checkpoints, and configure model settings in plain conversational English.

EXAMPLES:
  langPeanut chat                         Launch Chat Copilot in current directory
  langPeanut chat ./examples/nextjs-app   Attach Chat Copilot to a specific subproject
  langPeanut chat --model claude-sonnet-5 Use frontier Claude Sonnet 5
  langPeanut chat --provider ollama       Use 100% offline local Ollama model`,
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir := projectRoot
		if len(args) > 0 {
			targetDir = args[0]
		}

		var client llm.Client
		if chatProvider != "" {
			providerType := llm.ProviderType(chatProvider)
			modelName := chatModel
			if modelName == "" {
				if providerType == llm.ProviderClaude {
					modelName = "claude-sonnet-5"
				} else if providerType == llm.ProviderOpenAI {
					modelName = "gpt-5.4-mini"
				} else if providerType == llm.ProviderGemini {
					modelName = "gemini-3.5-flash"
				}
			}
			client = llm.NewClient(providerType, modelName)
		} else if chatModel != "" {
			client = llm.NewClient(llm.ProviderOpenAI, chatModel)
		} else {
			client = llm.AutoDetectClient()
		}

		p := tea.NewProgram(tui.NewChatUIModel(targetDir, client), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("error running Chat Copilot: %w", err)
		}
		return nil
	},
}

func init() {
	chatCmd.Flags().StringVarP(&chatProvider, "provider", "p", "", "LLM provider (claude, openai, gemini, ollama, deepl, nllb)")
	chatCmd.Flags().StringVarP(&chatModel, "model", "m", "", "Model identifier (e.g. claude-sonnet-5, gpt-5.4-mini, qwen2.5:7b)")
	chatCmd.Flags().StringVarP(&chatTone, "tone", "t", "default", "Tone style preset (default, casual, formal, gen_z, pirate)")
	rootCmd.AddCommand(chatCmd)
}
