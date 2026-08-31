package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/langPeanut/langPeanut/pkg/logger"
)

// ProviderType represents supported LLM providers
type ProviderType string

const (
	ProviderClaude    ProviderType = "claude"
	ProviderOpenAI    ProviderType = "openai"
	ProviderGemini    ProviderType = "gemini"
	ProviderDeepL     ProviderType = "deepl"
	ProviderNLLB      ProviderType = "nllb"       // Meta NLLB-200 (Auto Cloud / Local)
	ProviderNLLBCloud ProviderType = "nllb-cloud" // Meta NLLB-200 Cloud (Hugging Face Serverless)
	ProviderNLLBLocal ProviderType = "nllb-local" // Meta NLLB-200 Local (100% Offline Engine)
	ProviderOllama    ProviderType = "ollama"     // Ollama local inference (LLaMA, Qwen, Gemma…)
	ProviderCustom    ProviderType = "custom"     // OpenAI-compatible, Ollama, vLLM, fine-tuned models
	ProviderLocal     ProviderType = "local"
)

// Client is the universal multi-provider LLM interface
type Client interface {
	Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
	Name() ProviderType
	Description() string
}

// MultiProviderClient dispatches requests to OpenAI, Anthropic, Gemini, NLLB, Custom, or fallback
type MultiProviderClient struct {
	provider    ProviderType
	apiKey      string
	model       string
	endpoint    string
	description string
	http        *http.Client
}

func NewClient(provider ProviderType, model string) *MultiProviderClient {
	return NewClientWithConfig(provider, model, "", "")
}

func NewClientWithAPIKey(provider ProviderType, model string, apiKey string) *MultiProviderClient {
	c := NewClientWithConfig(provider, model, "", "")
	if apiKey != "" {
		c.apiKey = apiKey
	}
	return c
}

func (c *MultiProviderClient) SetAPIKey(apiKey string) {
	c.apiKey = apiKey
}

func NewClientWithConfig(provider ProviderType, model string, customDescription string, customEndpoint string) *MultiProviderClient {
	apiKey := ""
	desc := customDescription

	switch provider {
	case ProviderClaude:
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
		if model == "" {
			model = "claude-sonnet-5"
		}
		if desc == "" {
			desc = fmt.Sprintf("Anthropic Claude (%s) — Frontier reasoning & ICU syntax preservation", model)
		}
	case ProviderOpenAI:
		apiKey = os.Getenv("OPENAI_API_KEY")
		if model == "" {
			model = "gpt-5.4-mini"
		}
		if desc == "" {
			desc = fmt.Sprintf("OpenAI (%s) — High-speed multilingual translation", model)
		}
		if customEndpoint == "" {
			customEndpoint = "https://api.openai.com/v1/chat/completions"
		}
	case ProviderGemini:
		apiKey = os.Getenv("GEMINI_API_KEY")
		if model == "" {
			model = "gemini-3.7-flash"
		}
		if desc == "" {
			desc = fmt.Sprintf("Google Gemini (%s) — Ultra-low latency & token efficiency", model)
		}
	case ProviderNLLB, ProviderNLLBCloud:
		apiKey = os.Getenv("HF_TOKEN")
		if apiKey == "" {
			apiKey = os.Getenv("HUGGINGFACE_API_KEY")
		}
		if model == "" {
			model = "facebook/nllb-200-distilled-600M"
		}
		if desc == "" {
			desc = fmt.Sprintf("Meta NLLB-200 Cloud (%s) — 200+ languages direct translation", model)
		}
		if customEndpoint == "" {
			customEndpoint = "https://api-inference.huggingface.co/models/facebook/nllb-200-distilled-600M"
		}
	case ProviderNLLBLocal:
		if model == "" {
			model = "facebook/nllb-200-distilled-600M-local"
		}
		if desc == "" {
			desc = fmt.Sprintf("Meta NLLB-200 Local (%s) — 100%% offline zero-cost engine", model)
		}
		if customEndpoint == "" {
			customEndpoint = "http://localhost:8000/translate"
		}
	case ProviderOllama:
		baseURL := GetOllamaBaseURL()
		if customEndpoint == "" {
			customEndpoint = baseURL + "/v1/chat/completions"
		}
		if model == "" {
			// Try to auto-select a good translation model from running Ollama
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if status := CheckOllamaStatus(ctx); status.Running && len(status.Models) > 0 {
				model = BestOllamaModelForTranslation(status.Models)
			}
			if model == "" {
				model = "gemma3:4b" // sensible default
			}
		}
		apiKey = "ollama"
		if desc == "" {
			desc = fmt.Sprintf("Ollama Local (%s) — 100%% offline, no API key, runs on your Metal GPU", model)
		}
	case ProviderCustom:
		if endpoint := os.Getenv("OPENAI_BASE_URL"); endpoint != "" && customEndpoint == "" {
			customEndpoint = endpoint + "/chat/completions"
		}
		if customEndpoint == "" {
			customEndpoint = "http://localhost:11434/v1/chat/completions"
		}
		apiKey = os.Getenv("OPENAI_API_KEY")
		if model == "" {
			model = "qwen2.5:32b"
		}
		if desc == "" {
			desc = fmt.Sprintf("Custom Model (%s) at %s", model, customEndpoint)
		}
	default:
		provider = ProviderLocal
		if desc == "" {
			desc = "Local Deterministic Engine — Offline linguistic synthesizer (Zero API cost)"
		}
	}

	return &MultiProviderClient{
		provider:    provider,
		apiKey:      apiKey,
		model:       model,
		endpoint:    customEndpoint,
		description: desc,
		http:        &http.Client{Timeout: 60 * time.Second},
	}
}

// NewCustomClient initializes a custom model (e.g. Ollama, vLLM, fine-tuned endpoint)
func NewCustomClient(model, endpoint, apiKey, description string) *MultiProviderClient {
	if endpoint == "" {
		endpoint = "http://localhost:11434/v1/chat/completions" // Default local Ollama
	}
	if description == "" {
		description = fmt.Sprintf("Custom Model: %s at %s", model, endpoint)
	}

	return &MultiProviderClient{
		provider:    ProviderCustom,
		apiKey:      apiKey,
		model:       model,
		endpoint:    endpoint,
		description: description,
		http:        &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *MultiProviderClient) Name() ProviderType {
	return c.provider
}

func (c *MultiProviderClient) Description() string {
	return c.description
}

// Complete sends prompt to the selected LLM provider
func (c *MultiProviderClient) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if c.provider == ProviderLocal {
		return "", fmt.Errorf("local engine uses deterministic dictionary")
	}
	if c.apiKey == "" && c.provider != ProviderLocal && c.provider != ProviderOllama && c.provider != ProviderNLLBLocal {
		return "", fmt.Errorf("missing API key for %s", c.provider)
	}

	logger.Get().Info("MODEL:LLM", fmt.Sprintf("Dispatching completion request to %s (%s)", c.provider, c.model))

	switch c.provider {
	case ProviderClaude:
		return c.callClaude(ctx, systemPrompt, userPrompt)
	case ProviderOpenAI:
		return c.callOpenAI(ctx, systemPrompt, userPrompt)
	case ProviderGemini:
		return c.callGemini(ctx, systemPrompt, userPrompt)
	case ProviderDeepL:
		return c.callDeepL(ctx, systemPrompt, userPrompt)
	case ProviderNLLB, ProviderNLLBCloud, ProviderNLLBLocal:
		return c.callNLLB(ctx, systemPrompt, userPrompt)
	case ProviderOllama:
		return OllamaComplete(ctx, GetOllamaBaseURL(), c.model, systemPrompt, userPrompt)
	case ProviderCustom:
		return c.callCustom(ctx, systemPrompt, userPrompt)
	default:
		return "", fmt.Errorf("unsupported provider %s", c.provider)
	}
}

func formatProviderError(provider ProviderType, statusCode int, rawBody []byte) error {
	bodyStr := strings.TrimSpace(string(rawBody))

	// Try extracting message from structured JSON errors
	var genericErr struct {
		Error any `json:"error"`
		Message string `json:"message"`
		Detail string `json:"detail"`
	}
	if json.Unmarshal(rawBody, &genericErr) == nil {
		if genericErr.Message != "" {
			bodyStr = genericErr.Message
		} else if genericErr.Detail != "" {
			bodyStr = genericErr.Detail
		} else if str, ok := genericErr.Error.(string); ok && str != "" {
			bodyStr = str
		} else if m, ok := genericErr.Error.(map[string]any); ok {
			if msg, ok := m["message"].(string); ok && msg != "" {
				bodyStr = msg
			}
		}
	}

	switch provider {
	case ProviderClaude:
		if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden {
			return fmt.Errorf("Anthropic Claude API Error (%d): Invalid or missing ANTHROPIC_API_KEY. %s", statusCode, bodyStr)
		}
		if statusCode == http.StatusTooManyRequests {
			return fmt.Errorf("Anthropic Claude API Rate Limit (429): %s", bodyStr)
		}
		return fmt.Errorf("Anthropic Claude API Error (%d): %s", statusCode, bodyStr)

	case ProviderOpenAI:
		if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden {
			return fmt.Errorf("OpenAI API Error (%d): Incorrect or missing OPENAI_API_KEY. %s", statusCode, bodyStr)
		}
		if statusCode == http.StatusTooManyRequests {
			return fmt.Errorf("OpenAI API Rate Limit / Quota Exhausted (429): %s", bodyStr)
		}
		return fmt.Errorf("OpenAI API Error (%d): %s", statusCode, bodyStr)

	case ProviderGemini:
		if statusCode == http.StatusBadRequest || statusCode == http.StatusForbidden {
			return fmt.Errorf("Google Gemini API Error (%d): Invalid GEMINI_API_KEY. %s", statusCode, bodyStr)
		}
		return fmt.Errorf("Google Gemini API Error (%d): %s", statusCode, bodyStr)

	case ProviderDeepL:
		if statusCode == http.StatusForbidden {
			return fmt.Errorf("DeepL API Error (403): Invalid DEEPL_API_KEY or wrong endpoint. %s", bodyStr)
		}
		if statusCode == 456 {
			return fmt.Errorf("DeepL API Quota Limit (456): Monthly character limit reached. %s", bodyStr)
		}
		return fmt.Errorf("DeepL API Error (%d): %s", statusCode, bodyStr)

	case ProviderCustom:
		if statusCode == http.StatusNotFound {
			return fmt.Errorf("Custom Endpoint Error (404): Model or chat route not found at endpoint. %s", bodyStr)
		}
		return fmt.Errorf("Custom Endpoint Error (%d): %s", statusCode, bodyStr)

	default:
		return fmt.Errorf("%s API Error (%d): %s", provider, statusCode, bodyStr)
	}
}

func (c *MultiProviderClient) executeHTTPRequestWithRetry(ctx context.Context, makeReq func() (*http.Request, error)) ([]byte, error) {
	var lastErr error
	var body []byte

	const maxAttempts = 3
	for attempt := 0; attempt < maxAttempts; attempt++ {
		req, err := makeReq()
		if err != nil {
			return nil, err
		}

		startT := time.Now()
		resp, err := c.http.Do(req)
		if err == nil {
			body, _ = io.ReadAll(resp.Body)
			_ = resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				logger.Get().Debug("MODEL:LLM", fmt.Sprintf("HTTP 200 OK from %s in %v", c.provider, time.Since(startT)))
				return body, nil
			}

			formattedErr := formatProviderError(c.provider, resp.StatusCode, body)

			// Non-retriable authentication or schema client errors
			if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusBadRequest {
				logger.Get().Error("MODEL:LLM", fmt.Sprintf("Non-retriable auth/client error (%d)", resp.StatusCode), formattedErr)
				return nil, formattedErr
			}

			lastErr = formattedErr
			logger.Get().Warn("MODEL:LLM", fmt.Sprintf("API returned status %d on attempt %d/%d: %v", resp.StatusCode, attempt+1, maxAttempts, formattedErr))
		} else {
			lastErr = err
			logger.Get().Warn("MODEL:LLM", fmt.Sprintf("Network connection error on attempt %d/%d: %v", attempt+1, maxAttempts, err))
		}

		if attempt < maxAttempts-1 {
			backoff := time.Duration(500*(1<<attempt)) * time.Millisecond
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}
	}

	logger.Get().Error("MODEL:LLM", fmt.Sprintf("All %d API attempts failed for %s", maxAttempts, c.provider), lastErr)
	return nil, fmt.Errorf("request failed after %d attempts: %w", maxAttempts, lastErr)
}

func (c *MultiProviderClient) callCustom(ctx context.Context, system, user string) (string, error) {
	reqBody := map[string]any{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"response_format": map[string]string{"type": "json_object"},
	}

	data, _ := json.Marshal(reqBody)
	makeReq := func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, bytes.NewBuffer(data))
		if err != nil {
			return nil, err
		}
		if c.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+c.apiKey)
		}
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	}

	body, err := c.executeHTTPRequestWithRetry(ctx, makeReq)
	if err != nil {
		return "", err
	}

	var res struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &res); err != nil || len(res.Choices) == 0 {
		return "", fmt.Errorf("failed to parse custom model response: %w", err)
	}

	content := res.Choices[0].Message.Content
	inTokens := res.Usage.PromptTokens
	outTokens := res.Usage.CompletionTokens
	if inTokens == 0 {
		inTokens = EstimateTokens(system + " " + user)
	}
	if outTokens == 0 {
		outTokens = EstimateTokens(content)
	}
	RecordUsage(string(c.provider), c.model, inTokens, outTokens)

	return content, nil
}

func (c *MultiProviderClient) callClaude(ctx context.Context, system, user string) (string, error) {
	url := "https://api.anthropic.com/v1/messages"

	reqBody := map[string]any{
		"model":      c.model,
		"max_tokens": 16384,
		"system":     system,
		"messages": []map[string]string{
			{"role": "user", "content": user},
		},
	}

	data, _ := json.Marshal(reqBody)
	makeReq := func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
		if err != nil {
			return nil, err
		}
		req.Header.Set("x-api-key", c.apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
		req.Header.Set("content-type", "application/json")
		return req, nil
	}

	body, err := c.executeHTTPRequestWithRetry(ctx, makeReq)
	if err != nil {
		return "", err
	}

	var res struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int64 `json:"input_tokens"`
			OutputTokens int64 `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &res); err != nil || len(res.Content) == 0 {
		return "", fmt.Errorf("failed to parse claude response: %w", err)
	}

	content := res.Content[0].Text
	inTokens := res.Usage.InputTokens
	outTokens := res.Usage.OutputTokens
	if inTokens == 0 {
		inTokens = EstimateTokens(system + " " + user)
	}
	if outTokens == 0 {
		outTokens = EstimateTokens(content)
	}
	RecordUsage(string(c.provider), c.model, inTokens, outTokens)

	return content, nil
}

func (c *MultiProviderClient) callOpenAI(ctx context.Context, system, user string) (string, error) {
	url := "https://api.openai.com/v1/chat/completions"

	reqBody := map[string]any{
		"model":                 c.model,
		"max_completion_tokens": 16384,
		"response_format":       map[string]string{"type": "json_object"},
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
	}

	data, _ := json.Marshal(reqBody)
	makeReq := func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	}

	body, err := c.executeHTTPRequestWithRetry(ctx, makeReq)
	if err != nil {
		// Fallback for endpoints without response_format or max_completion_tokens support
		if strings.Contains(err.Error(), "response_format") || strings.Contains(err.Error(), "max_completion_tokens") {
			fallbackBody := map[string]any{
				"model":      c.model,
				"max_tokens": 16384,
				"messages": []map[string]string{
					{"role": "system", "content": system},
					{"role": "user", "content": user},
				},
			}
			fbData, _ := json.Marshal(fallbackBody)
			makeFbReq := func() (*http.Request, error) {
				req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(fbData))
				if err != nil {
					return nil, err
				}
				req.Header.Set("Authorization", "Bearer "+c.apiKey)
				req.Header.Set("Content-Type", "application/json")
				return req, nil
			}
			body, err = c.executeHTTPRequestWithRetry(ctx, makeFbReq)
		}
		if err != nil {
			return "", err
		}
	}

	var res struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &res); err != nil || len(res.Choices) == 0 {
		return "", fmt.Errorf("failed to parse openai response: %w", err)
	}

	content := res.Choices[0].Message.Content
	inTokens := res.Usage.PromptTokens
	outTokens := res.Usage.CompletionTokens
	if inTokens == 0 {
		inTokens = EstimateTokens(system + " " + user)
	}
	if outTokens == 0 {
		outTokens = EstimateTokens(content)
	}
	RecordUsage(string(c.provider), c.model, inTokens, outTokens)

	return content, nil
}

func (c *MultiProviderClient) callGemini(ctx context.Context, system, user string) (string, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", c.model, c.apiKey)

	reqBody := map[string]any{
		"contents": []map[string]any{
			{
				"parts": []map[string]string{
					{"text": system + "\n\n" + user},
				},
			},
		},
		"generationConfig": map[string]any{
			"maxOutputTokens":  16384,
			"responseMimeType": "application/json",
		},
	}

	data, _ := json.Marshal(reqBody)
	makeReq := func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	}

	body, err := c.executeHTTPRequestWithRetry(ctx, makeReq)
	if err != nil {
		return "", err
	}

	var res struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int64 `json:"promptTokenCount"`
			CandidatesTokenCount int64 `json:"candidatesTokenCount"`
		} `json:"usageMetadata"`
	}
	if err := json.Unmarshal(body, &res); err != nil || len(res.Candidates) == 0 || len(res.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("failed to parse gemini response: %w", err)
	}

	content := res.Candidates[0].Content.Parts[0].Text
	inTokens := res.UsageMetadata.PromptTokenCount
	outTokens := res.UsageMetadata.CandidatesTokenCount
	if inTokens == 0 {
		inTokens = EstimateTokens(system + " " + user)
	}
	if outTokens == 0 {
		outTokens = EstimateTokens(content)
	}
	RecordUsage(string(c.provider), c.model, inTokens, outTokens)

	return content, nil
}

func (c *MultiProviderClient) callNLLB(ctx context.Context, system, user string) (string, error) {
	// Parse user JSON map of strings: { "key": "source text" }
	var sourceMap map[string]string
	user = strings.TrimSpace(user)
	if err := json.Unmarshal([]byte(user), &sourceMap); err != nil {
		sourceMap = map[string]string{"value": user}
	}

	// Extract target locale from system prompt (e.g. target locale "fr")
	targetLocale := "es"
	re := regexp.MustCompile(`target locale ["']?([a-zA-Z0-9_-]+)["']?`)
	if match := re.FindStringSubmatch(system); len(match) > 1 {
		targetLocale = match[1]
	}

	sourceLocale := "en"
	if matchSrc := regexp.MustCompile(`source locale ["']?([a-zA-Z0-9_-]+)["']?`).FindStringSubmatch(system); len(matchSrc) > 1 {
		sourceLocale = matchSrc[1]
	}

	var engine *NLLBEngine
	if c.provider == ProviderNLLBLocal {
		engine = NewNLLBLocalEngine(c.endpoint)
	} else {
		engine = NewNLLBCloudEngine(c.apiKey)
		if c.endpoint != "" {
			engine.endpoint = c.endpoint
		}
	}

	translatedMap, err := engine.TranslateStringsBatch(ctx, sourceMap, sourceLocale, targetLocale)
	if err != nil {
		return "", err
	}

	resBytes, err := json.MarshalIndent(translatedMap, "", "  ")
	if err != nil {
		return "", err
	}

	inTokens := EstimateTokens(system + " " + user)
	outTokens := EstimateTokens(string(resBytes))
	RecordUsage(string(c.provider), c.model, inTokens, outTokens)

	return string(resBytes), nil
}

func (c *MultiProviderClient) callDeepL(ctx context.Context, system, user string) (string, error) {
	// Parse user JSON map: { "key": "source text" }
	var sourceMap map[string]string
	user = strings.TrimSpace(user)
	if err := json.Unmarshal([]byte(user), &sourceMap); err != nil {
		sourceMap = map[string]string{"value": user}
	}

	targetLocale := "ES"
	re := regexp.MustCompile(`target locale ["']?([a-zA-Z0-9_-]+)["']?`)
	if match := re.FindStringSubmatch(system); len(match) > 1 {
		targetLocale = strings.ToUpper(match[1])
	}
	if targetLocale == "EN" {
		targetLocale = "EN-US"
	} else if targetLocale == "PT" {
		targetLocale = "PT-PT"
	}

	endpoint := "https://api-free.deepl.com/v2/translate"
	if !strings.HasSuffix(c.apiKey, ":fx") && c.apiKey != "" {
		endpoint = "https://api.deepl.com/v2/translate"
	}

	logger.Get().Info("MODEL:DEEPL", fmt.Sprintf("Translating %d strings to %s via DeepL API", len(sourceMap), targetLocale))

	translatedMap := make(map[string]string, len(sourceMap))
	for k, text := range sourceMap {
		reqData := map[string]any{
			"text":        []string{text},
			"target_lang": targetLocale,
		}
		data, _ := json.Marshal(reqData)

		makeReq := func() (*http.Request, error) {
			req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(data))
			if err != nil {
				return nil, err
			}
			req.Header.Set("Authorization", "DeepL-Auth-Key "+c.apiKey)
			req.Header.Set("Content-Type", "application/json")
			return req, nil
		}

		body, err := c.executeHTTPRequestWithRetry(ctx, makeReq)
		if err != nil {
			logger.Get().Error("MODEL:DEEPL", fmt.Sprintf("DeepL translation failed for key %s: %v", k, err), err)
			return "", err
		}

		var res struct {
			Translations []struct {
				Text string `json:"text"`
			} `json:"translations"`
		}
		if err := json.Unmarshal(body, &res); err == nil && len(res.Translations) > 0 {
			translatedMap[k] = res.Translations[0].Text
		} else {
			translatedMap[k] = text
		}
	}

	resBytes, err := json.MarshalIndent(translatedMap, "", "  ")
	if err != nil {
		return "", err
	}

	RecordUsage("deepl", c.model, EstimateTokens(user), EstimateTokens(string(resBytes)))
	logger.Get().Success("MODEL:DEEPL", fmt.Sprintf("Successfully translated %d strings to %s", len(translatedMap), targetLocale))
	return string(resBytes), nil
}

// AutoDetectClient instantiates the best available LLM provider from environment variables
func AutoDetectClient() *MultiProviderClient {
	if os.Getenv("GEMINI_API_KEY") != "" {
		return NewClient(ProviderGemini, "gemini-3.7-flash")
	}
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		return NewClient(ProviderClaude, "claude-sonnet-5")
	}
	if os.Getenv("OPENAI_API_KEY") != "" {
		return NewClient(ProviderOpenAI, "gpt-5.4-mini")
	}
	if os.Getenv("HF_TOKEN") != "" || os.Getenv("HUGGINGFACE_API_KEY") != "" {
		return NewClient(ProviderNLLBCloud, "")
	}
	if os.Getenv("OPENAI_BASE_URL") != "" {
		return NewClient(ProviderCustom, "")
	}
	return NewClient(ProviderGemini, "gemini-3.7-flash")
}

