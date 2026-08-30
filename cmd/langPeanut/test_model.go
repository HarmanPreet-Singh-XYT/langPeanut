package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/langPeanut/langPeanut/pkg/llm"
	"github.com/langPeanut/langPeanut/pkg/memory"
	"github.com/spf13/cobra"
)

var (
	testProvider string
	testModel    string
	testAPIKey   string
	testTarget   string
	testText     string
)

var testModelCmd = &cobra.Command{
	Use:     "test-model",
	Aliases: []string{"test", "probe"},
	Short:   "Test AI model connectivity, translation accuracy & latency",
	Long: `Test and verify AI translation model connectivity, credentials, and inference quality.

Runs a live probe with an ICU test string and outputs round-trip latency, 
token metrics, estimated cost, and actionable diagnostic guidance if errors occur.

EXAMPLES:
  langPeanut test-model
  langPeanut test-model --provider ollama                          # Auto-detect Ollama model
  langPeanut test-model --provider ollama --model gemma3:4b --target fr
  langPeanut test-model --provider claude --target fr
  langPeanut test-model --provider openai --model gpt-5.4-mini-2026-03-17 --target ja
  langPeanut test-model --provider nllb-cloud --target de
  langPeanut test-model --provider custom --model qwen2.5:32b --target es`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := memory.LoadConfig("")

		prov := llm.ProviderType(testProvider)
		if prov == "" {
			prov = llm.ProviderType(cfg.ActiveProvider)
			if prov == "" {
				prov = llm.ProviderLocal
			}
		}

		mod := testModel
		// For Ollama, don't inherit a stored model name — it may be from a different provider
		if mod == "" && prov != llm.ProviderOllama {
			mod = cfg.ActiveModel
		}

		key := testAPIKey
		if key == "" {
			key = cfg.GetAPIKey(string(prov))
		}

		tgt := testTarget
		if tgt == "" {
			tgt = "es"
		}

		text := testText
		if text == "" {
			text = "Welcome to langPeanut! Effortless multi-agent software localization."
		}

		fmt.Printf("\n\033[1;36mlangPeanut AI Model Connectivity & Diagnostic Probe\033[0m\n")
		fmt.Println("──────────────────────────────────────────────────────────────────────────")
		fmt.Printf(" • Provider:   \033[1m%s\033[0m\n", prov)
		if mod != "" {
			fmt.Printf(" • Model:      \033[1m%s\033[0m\n", mod)
		} else if prov == llm.ProviderOllama {
			fmt.Printf(" • Model:      \033[33m[auto-detecting from running Ollama...]\033[0m\n")
		}
		keyStatus := "\033[32mConfigured\033[0m"
		switch {
		case prov == llm.ProviderOllama || prov == llm.ProviderLocal:
			keyStatus = "\033[32mZero-key offline engine\033[0m"
		case prov == llm.ProviderNLLBLocal:
			keyStatus = "\033[32mZero-key offline engine\033[0m"
		case key == "":
			keyStatus = "\033[33mNot set (checking environment)\033[0m"
		}
		fmt.Printf(" • API Key:    %s\n", keyStatus)
		fmt.Printf(" • Probe Text: %q\n", text)
		fmt.Printf(" • Target:     %s\n", tgt)
		fmt.Println("──────────────────────────────────────────────────────────────────────────")
		fmt.Printf("Sending probe request to %s...\n", prov)

		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		res, err := llm.TestModelConnection(ctx, prov, mod, key, tgt, text)

		if err != nil {
			fmt.Printf("\n\033[1;31m❌ [PROBE FAILED]\033[0m %v\n", err)
			if res != nil && res.Diagnostic != nil {
				fmt.Println(res.Diagnostic.FormatCLI())
			}
			os.Exit(1)
			return nil
		}

		fmt.Println("\n\033[1;32m✓ [PROBE PASSED — MODEL IS HEALTHY & OPERATIONAL]\033[0m")
		fmt.Println("──────────────────────────────────────────────────────────────────────────")
		fmt.Printf(" • Provider:       %s\n", res.Provider)
		fmt.Printf(" • Model:          \033[1m%s\033[0m\n", res.Model)
		fmt.Printf(" • Latency:        \033[1;32m%d ms\033[0m\n", res.LatencyMs)
		fmt.Printf(" • Source [en]:    %s\n", res.SourceText)
		fmt.Printf(" • Output [%s]:    \033[1;36m%s\033[0m\n", res.TargetLang, res.TranslatedText)
		fmt.Printf(" • Token Usage:    %d prompt in / %d completion out\n", res.InputTokens, res.OutputTokens)
		if res.EstimatedCost > 0 {
			fmt.Printf(" • Est. Cost:      $%.5f USD\n", res.EstimatedCost)
		} else {
			fmt.Printf(" • Est. Cost:      $0.00 (Zero API cost)\n")
		}
		fmt.Println("──────────────────────────────────────────────────────────────────────────")
		fmt.Println("This model is verified and ready for full project localization runs.")
		return nil
	},
}

func init() {
	testModelCmd.Flags().StringVarP(&testProvider, "provider", "p", "", "AI Provider (ollama, claude, openai, gemini, deepl, nllb-cloud, custom, local)")
	testModelCmd.Flags().StringVarP(&testModel, "model", "m", "", "Model name — for Ollama: gemma3:4b, qwen2.5:7b, etc. (auto-detected if omitted)")
	testModelCmd.Flags().StringVarP(&testAPIKey, "key", "k", "", "API key override")
	testModelCmd.Flags().StringVarP(&testTarget, "target", "t", "es", "Target language code (e.g. es, fr, ja, de, hi)")
	testModelCmd.Flags().StringVar(&testText, "text", "", "Custom text string to test translate")

	rootCmd.AddCommand(testModelCmd)
}
