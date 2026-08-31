package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/langPeanut/langPeanut/pkg/logger"
	"github.com/langPeanut/langPeanut/pkg/memory"
)

// TestModelResult holds the diagnostic outcome of testing an AI model
type TestModelResult struct {
	Provider       ProviderType            `json:"provider"`
	Model          string                  `json:"model"`
	SourceLang     string                  `json:"source_lang"`
	TargetLang     string                  `json:"target_lang"`
	SourceText     string                  `json:"source_text"`
	TranslatedText string                  `json:"translated_text"`
	LatencyMs      int64                   `json:"latency_ms"`
	InputTokens    int64                   `json:"input_tokens"`
	OutputTokens   int64                   `json:"output_tokens"`
	EstimatedCost  float64                 `json:"estimated_cost"`
	Success        bool                    `json:"success"`
	ErrorMessage   string                  `json:"error_message,omitempty"`
	Diagnostic     *logger.DiagnosticAdvice `json:"diagnostic,omitempty"`
}

// TestModelConnection executes a live translation probe against any AI provider
func TestModelConnection(ctx context.Context, provider ProviderType, model, apiKey, targetLang, sampleText string) (*TestModelResult, error) {
	cfg := memory.LoadConfig("")

	if provider == "" {
		if cfg.ActiveProvider != "" {
			provider = ProviderType(cfg.ActiveProvider)
		} else {
			provider = ProviderLocal
		}
	}
	if model == "" {
		// For Ollama, skip stored config model — it may be an NLLB/other filename.
		// Let the Ollama probe block auto-select from running models.
		if provider != ProviderOllama {
			if cfg.ActiveModel != "" {
				model = cfg.ActiveModel
			} else {
				model = "default"
			}
		}
	}
	if apiKey == "" {
		apiKey = cfg.GetAPIKey(string(provider))
	}

	if targetLang == "" {
		targetLang = "es"
	}
	if sampleText == "" {
		sampleText = "Welcome to langPeanut! Effortless multi-agent software localization."
	}

	res := &TestModelResult{
		Provider:   provider,
		Model:      model,
		SourceLang: "en",
		TargetLang: targetLang,
		SourceText: sampleText,
	}

	logger.Get().Info("MODEL:TEST", fmt.Sprintf("Starting model connectivity test for %s (%s) -> %s", provider, model, targetLang))

	// Pre-check for local NLLB offline model
	if provider == ProviderNLLBLocal {
		downloaded, path, _ := IsNLLBModelDownloaded()
		if !downloaded {
			err := fmt.Errorf("Meta NLLB-200 offline model (380MB GGUF) is not downloaded. Run 'langPeanut models download' or click download in Settings")
			res.Success = false
			res.ErrorMessage = err.Error()
			res.Diagnostic = logger.ExplainError(err)
			logger.Get().Error("MODEL:TEST", "Local model missing", err)
			return res, err
		}
		logger.Get().Debug("MODEL:TEST", fmt.Sprintf("Found local NLLB model at %s", path))

		// Route directly through NLLBEngine (llama-cli → GGUF on Metal/CPU)
		start := time.Now()
		engine := NewNLLBLocalEngine("")
		results, err := engine.TranslateStringsBatch(ctx, map[string]string{"probe": sampleText}, "en", targetLang)
		res.LatencyMs = time.Since(start).Milliseconds()
		if err != nil {
			res.Success = false
			res.ErrorMessage = err.Error()
			res.Diagnostic = logger.ExplainError(err)
			logger.Get().Error("MODEL:TEST", fmt.Sprintf("NLLB local probe failed after %dms", res.LatencyMs), err)
			return res, err
		}
		res.TranslatedText = results["probe"]
		res.InputTokens = EstimateTokens(sampleText)
		res.OutputTokens = EstimateTokens(res.TranslatedText)
		res.EstimatedCost = 0.00
		res.Success = true
		logger.Get().Success("MODEL:TEST", fmt.Sprintf("NLLB local on-device probe succeeded in %dms! Output: %q", res.LatencyMs, res.TranslatedText))
		return res, nil
	}

	start := time.Now()

	// Ollama — first-class local inference via /v1/chat/completions
	if provider == ProviderOllama {
		ollamaStatus := CheckOllamaStatus(ctx)
		if !ollamaStatus.Running {
			err := fmt.Errorf("Ollama is not running. Start it with: ollama serve\nThen pull a model: ollama pull gemma3:4b")
			res.Success = false
			res.ErrorMessage = err.Error()
			res.Diagnostic = logger.ExplainError(err)
			logger.Get().Error("MODEL:TEST", "Ollama daemon not reachable", err)
			return res, err
		}
		if len(ollamaStatus.Models) == 0 {
			err := fmt.Errorf("Ollama is running but has no models. Pull one: ollama pull gemma3:4b")
			res.Success = false
			res.ErrorMessage = err.Error()
			res.Diagnostic = logger.ExplainError(err)
			return res, err
		}
		// Pick model: use explicit flag value, or auto-select
		if model == "" || model == "default" {
			model = BestOllamaModelForTranslation(ollamaStatus.Models)
		}
		res.Model = model

		systemPrompt := fmt.Sprintf(`You are a multilingual translation engine. Translate the text below from English to %s.
Preserve any brand names (e.g. langPeanut), code placeholders, or ICU syntax ({count, plural, ...}).
Output ONLY the translated text, nothing else.`, targetLang)

		translated, err := OllamaComplete(ctx, ollamaStatus.BaseURL, model, systemPrompt, sampleText)
		res.LatencyMs = time.Since(start).Milliseconds()
		if err != nil {
			res.Success = false
			res.ErrorMessage = err.Error()
			res.Diagnostic = logger.ExplainError(err)
			logger.Get().Error("MODEL:TEST", fmt.Sprintf("Ollama probe failed after %dms", res.LatencyMs), err)
			return res, err
		}
		res.TranslatedText = translated
		res.InputTokens = EstimateTokens(systemPrompt + " " + sampleText)
		res.OutputTokens = EstimateTokens(translated)
		res.EstimatedCost = 0.00
		res.Success = true
		logger.Get().Success("MODEL:TEST", fmt.Sprintf("Ollama probe succeeded in %dms with %s! Output: %q", res.LatencyMs, model, translated))
		return res, nil
	}

	if provider == ProviderLocal {
		res.LatencyMs = time.Since(start).Milliseconds()
		res.TranslatedText = fmt.Sprintf("[%s] Bienvenido a langPeanut! Localización de software multiagente sin esfuerzo.", strings.ToUpper(targetLang))
		res.InputTokens = EstimateTokens(sampleText)
		res.OutputTokens = EstimateTokens(res.TranslatedText)
		res.EstimatedCost = 0.00
		res.Success = true
		logger.Get().Success("MODEL:TEST", fmt.Sprintf("Deterministic Local probe succeeded in %dms", res.LatencyMs))
		return res, nil
	}

	// Build translation probe client
	client := NewClientWithAPIKey(provider, model, apiKey)
	if client == nil {
		err := fmt.Errorf("failed to initialize client for provider %s", provider)
		res.Success = false
		res.ErrorMessage = err.Error()
		res.Diagnostic = logger.ExplainError(err)
		return res, err
	}

	systemPrompt := fmt.Sprintf(`You are the Cultural Translator Agent in langPeanut.
Translate the provided JSON map of UI text from source locale "en" into target locale "%s".
Maintain ICU syntax placeholders and preserve brand names.
Output ONLY a raw, valid JSON object mapping keys to translated strings.`, targetLang)

	userMap := map[string]string{
		"welcome_message": sampleText,
	}
	userBytes, _ := json.Marshal(userMap)

	rawOutput, err := client.Complete(ctx, systemPrompt, string(userBytes))
	res.LatencyMs = time.Since(start).Milliseconds()

	if err != nil {
		res.Success = false
		res.ErrorMessage = err.Error()
		res.Diagnostic = logger.ExplainError(err)
		logger.Get().Error("MODEL:TEST", fmt.Sprintf("Model probe failed after %dms: %v", res.LatencyMs, err), err)
		return res, err
	}

	// Parse translated output
	var transMap map[string]string
	cleanJSON := strings.TrimSpace(rawOutput)
	if strings.HasPrefix(cleanJSON, "```") {
		lines := strings.Split(cleanJSON, "\n")
		if len(lines) >= 2 {
			cleanJSON = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}

	if err := json.Unmarshal([]byte(cleanJSON), &transMap); err == nil && transMap["welcome_message"] != "" {
		res.TranslatedText = transMap["welcome_message"]
	} else {
		res.TranslatedText = cleanJSON
	}

	res.InputTokens = EstimateTokens(systemPrompt + " " + string(userBytes))
	res.OutputTokens = EstimateTokens(res.TranslatedText)
	res.EstimatedCost = estimateCost(model, res.InputTokens, res.OutputTokens)
	res.Success = true

	logger.Get().Success("MODEL:TEST", fmt.Sprintf("Model probe succeeded in %dms! Output: %q", res.LatencyMs, res.TranslatedText))
	return res, nil
}
