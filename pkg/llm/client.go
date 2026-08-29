package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// ProviderType represents supported LLM providers
type ProviderType string

const (
	ProviderClaude ProviderType = "claude"
	ProviderOpenAI ProviderType = "openai"
	ProviderGemini ProviderType = "gemini"
	ProviderDeepL  ProviderType = "deepl"
	ProviderCustom ProviderType = "custom" // OpenAI-compatible, Ollama, vLLM, fine-tuned models
	ProviderLocal  ProviderType = "local"
)

// Client is the universal multi-provider LLM interface
type Client interface {
	Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
	Name() ProviderType
	Description() string
}

// MultiProviderClient dispatches requests to OpenAI, Anthropic, Gemini, Custom, or fallback
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
			model = "claude-3-7-sonnet-20250219"
		}
		if desc == "" {
			desc = fmt.Sprintf("Custom Claude (%s) — Frontier reasoning & ICU syntax preservation", model)
		}
	case ProviderOpenAI:
		apiKey = os.Getenv("OPENAI_API_KEY")
		if model == "" {
			model = "gpt-5.4-mini-2026-03-17"
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
			model = "gemini-2.5-flash"
		}
		if desc == "" {
			desc = fmt.Sprintf("Custom Gemini (%s) — Ultra-low latency & token efficiency", model)
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

	switch c.provider {
	case ProviderClaude:
		return c.callClaude(ctx, systemPrompt, userPrompt)
	case ProviderOpenAI:
		return c.callOpenAI(ctx, systemPrompt, userPrompt)
	case ProviderGemini:
		return c.callGemini(ctx, systemPrompt, userPrompt)
	case ProviderCustom:
		return c.callCustom(ctx, systemPrompt, userPrompt)
	default:
		return "", fmt.Errorf("unsupported provider %s", c.provider)
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

		resp, err := c.http.Do(req)
		if err == nil {
			body, _ = io.ReadAll(resp.Body)
			_ = resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				return body, nil
			}

			// Non-retriable authentication or schema client errors
			if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusBadRequest {
				return nil, fmt.Errorf("API auth/client error (%d): %s", resp.StatusCode, string(body))
			}

			lastErr = fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
		} else {
			lastErr = err
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

// AutoDetectClient instantiates the best available LLM provider from environment variables
func AutoDetectClient() *MultiProviderClient {
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		return NewClient(ProviderClaude, "")
	}
	if os.Getenv("OPENAI_API_KEY") != "" {
		return NewClient(ProviderOpenAI, "")
	}
	if os.Getenv("GEMINI_API_KEY") != "" {
		return NewClient(ProviderGemini, "")
	}
	if os.Getenv("OPENAI_BASE_URL") != "" {
		return NewClient(ProviderCustom, "")
	}
	return NewClient(ProviderLocal, "")
}
