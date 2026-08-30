package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/langPeanut/langPeanut/pkg/logger"
)

// OllamaModel represents a model available in a running Ollama instance
type OllamaModel struct {
	Name       string `json:"name"`
	Model      string `json:"model"`
	SizeBytes  int64  `json:"size"`
	Family     string `json:"family"`
	ParamSize  string `json:"parameter_size"`
	QuantLevel string `json:"quantization_level"`
}

// OllamaStatus holds the result of querying the Ollama daemon
type OllamaStatus struct {
	Running bool
	BaseURL string
	Models  []OllamaModel
	Error   string
}

// GetOllamaBaseURL returns the configured Ollama base URL (default localhost:11434)
func GetOllamaBaseURL() string {
	if url := os.Getenv("OLLAMA_HOST"); url != "" {
		if !strings.HasPrefix(url, "http") {
			url = "http://" + url
		}
		return strings.TrimSuffix(url, "/")
	}
	return "http://localhost:11434"
}

// CheckOllamaStatus queries the Ollama daemon and returns available models
func CheckOllamaStatus(ctx context.Context) *OllamaStatus {
	baseURL := GetOllamaBaseURL()
	status := &OllamaStatus{BaseURL: baseURL}

	client := &http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/tags", nil)
	if err != nil {
		status.Error = err.Error()
		return status
	}

	resp, err := client.Do(req)
	if err != nil {
		status.Error = fmt.Sprintf("Ollama not running at %s: %v", baseURL, err)
		return status
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var tagsResp struct {
		Models []struct {
			Name    string `json:"name"`
			Model   string `json:"model"`
			Size    int64  `json:"size"`
			Details struct {
				Family            string `json:"family"`
				ParameterSize     string `json:"parameter_size"`
				QuantizationLevel string `json:"quantization_level"`
			} `json:"details"`
		} `json:"models"`
	}

	if err := json.Unmarshal(body, &tagsResp); err != nil {
		status.Error = fmt.Sprintf("Failed to parse Ollama response: %v", err)
		return status
	}

	status.Running = true
	for _, m := range tagsResp.Models {
		status.Models = append(status.Models, OllamaModel{
			Name:       m.Name,
			Model:      m.Model,
			SizeBytes:  m.Size,
			Family:     m.Details.Family,
			ParamSize:  m.Details.ParameterSize,
			QuantLevel: m.Details.QuantizationLevel,
		})
	}

	logger.Get().Info("OLLAMA", fmt.Sprintf("Detected %d model(s) at %s", len(status.Models), baseURL))
	return status
}

// BestOllamaModelForTranslation picks the most capable available model for multilingual translation
// Prefers larger multilingual models over code-specific ones
func BestOllamaModelForTranslation(models []OllamaModel) string {
	// Priority: multilingual general models > code models > any
	priority := []string{"qwen2.5", "llama3", "gemma3", "mistral", "aya", "command-r", "phi3"}
	avoid := []string{"coder", "embed", "embedding", "vision"}

	var best string
	for _, pref := range priority {
		for _, m := range models {
			name := strings.ToLower(m.Name)
			if !strings.Contains(name, pref) {
				continue
			}
			skip := false
			for _, bad := range avoid {
				if strings.Contains(name, bad) {
					skip = true
					break
				}
			}
			if !skip {
				return m.Name
			}
		}
	}

	// Fall back to any non-embedding model
	for _, m := range models {
		name := strings.ToLower(m.Name)
		isEmbed := strings.Contains(name, "embed") || strings.Contains(name, "embedding")
		if !isEmbed {
			best = m.Name
			break
		}
	}
	return best
}

// OllamaComplete sends a translation prompt to the Ollama chat completions endpoint
func OllamaComplete(ctx context.Context, baseURL, model, systemPrompt, userPrompt string) (string, error) {
	endpoint := baseURL + "/v1/chat/completions"

	reqBody := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.1,
		"stream":      false,
	}

	data, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(string(data)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	// Ollama's v1 endpoint accepts an optional Authorization header
	req.Header.Set("Authorization", "Bearer ollama")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Ollama request failed (is 'ollama serve' running?): %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Ollama returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	body, _ := io.ReadAll(resp.Body)
	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("failed to parse Ollama response: %w", err)
	}
	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("Ollama returned empty choices")
	}
	return strings.TrimSpace(chatResp.Choices[0].Message.Content), nil
}

// IsModelInOllamaList checks if a model name is in the list of available Ollama models
func IsModelInOllamaList(models []OllamaModel, name string) bool {
	for _, m := range models {
		if strings.EqualFold(m.Name, name) || strings.EqualFold(m.Model, name) {
			return true
		}
	}
	return false
}

// GetNextOllamaModel returns the next model in the list after the current one, wrapping around
func GetNextOllamaModel(models []OllamaModel, current string) string {
	if len(models) == 0 {
		return current
	}
	var valid []string
	for _, m := range models {
		name := strings.ToLower(m.Name)
		if !strings.Contains(name, "embed") && !strings.Contains(name, "embedding") {
			valid = append(valid, m.Name)
		}
	}
	if len(valid) == 0 {
		for _, m := range models {
			valid = append(valid, m.Name)
		}
	}
	for i, v := range valid {
		if strings.EqualFold(v, current) {
			return valid[(i+1)%len(valid)]
		}
	}
	return valid[0]
}
