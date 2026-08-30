package llm

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	// DefaultNLLBModelURL is the verified public Hugging Face URL for the 4-bit quantized GGUF NLLB-200 distilled 600M model
	DefaultNLLBModelURL = "https://huggingface.co/keisuke-miyako/nllb-200-distilled-600M-gguf-q4_k_m/resolve/main/nllb-200-distilled-600M-q4_k_m.gguf"
	// SecondaryNLLBModelURL is a backup public mirror
	SecondaryNLLBModelURL = "https://huggingface.co/JosephTu/nllb-200-distilled-600M-GGUF/resolve/main/nllb-600m.gguf"
	// DefaultNLLBModelFileName is the local disk filename
	DefaultNLLBModelFileName = "nllb-200-600M-q4_k_m.gguf"
)

// GetNLLBModelDir returns the directory path where local translation models are stored (~/.langPeanut/models)
func GetNLLBModelDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".langPeanut", "models")
}

// GetNLLBModelPath returns the full path to the local NLLB model artifact
func GetNLLBModelPath() string {
	if custom := os.Getenv("NLLB_MODEL_PATH"); custom != "" {
		return custom
	}
	return filepath.Join(GetNLLBModelDir(), DefaultNLLBModelFileName)
}

// IsNLLBModelDownloaded checks if the local NLLB model exists and returns its size in bytes
func IsNLLBModelDownloaded() (bool, string, int64) {
	path := GetNLLBModelPath()
	info, err := os.Stat(path)
	if err == nil && info.Size() > 1024*1024 { // Valid if > 1MB
		return true, path, info.Size()
	}
	return false, path, 0
}

// EnsureNLLBModel checks if the NLLB model exists locally, and downloads it from Hugging Face if missing.
// It invokes progressFn with (downloadedBytes, totalBytes, percentFloat) during streaming.
func EnsureNLLBModel(ctx context.Context, progressFn func(downloaded, total int64, percent float64)) (string, error) {
	downloaded, path, _ := IsNLLBModelDownloaded()
	if downloaded {
		return path, nil
	}

	modelDir := GetNLLBModelDir()
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create model directory: %w", err)
	}

	urls := []string{
		os.Getenv("NLLB_MODEL_URL"),
		DefaultNLLBModelURL,
		SecondaryNLLBModelURL,
	}

	client := &http.Client{Timeout: 30 * time.Minute}
	var lastErr error

	for _, downloadURL := range urls {
		if downloadURL == "" {
			continue
		}

		req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
		if err != nil {
			lastErr = err
			continue
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("Hugging Face download from %s failed with HTTP status %d: %s", downloadURL, resp.StatusCode, resp.Status)
			continue
		}

		totalSize := resp.ContentLength
		tempPath := path + ".downloading"
		out, err := os.Create(tempPath)
		if err != nil {
			resp.Body.Close()
			return "", fmt.Errorf("failed to create model file: %w", err)
		}

		buf := make([]byte, 64*1024) // 64KB chunks
		var totalDownloaded int64
		var streamErr error

		for {
			select {
			case <-ctx.Done():
				_ = out.Close()
				_ = os.Remove(tempPath)
				resp.Body.Close()
				return "", ctx.Err()
			default:
			}

			n, readErr := resp.Body.Read(buf)
			if n > 0 {
				if _, writeErr := out.Write(buf[:n]); writeErr != nil {
					streamErr = fmt.Errorf("failed to write to model file: %w", writeErr)
					break
				}
				totalDownloaded += int64(n)

				if progressFn != nil {
					percent := 0.0
					if totalSize > 0 {
						percent = (float64(totalDownloaded) / float64(totalSize)) * 100.0
					}
					progressFn(totalDownloaded, totalSize, percent)
				}
			}

			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				streamErr = fmt.Errorf("error streaming model: %w", readErr)
				break
			}
		}

		_ = out.Close()
		resp.Body.Close()

		if streamErr != nil {
			_ = os.Remove(tempPath)
			lastErr = streamErr
			continue
		}

		if err := os.Rename(tempPath, path); err != nil {
			_ = os.Remove(tempPath)
			return "", fmt.Errorf("failed to finalize downloaded model: %w", err)
		}

		return path, nil
	}

	return "", lastErr
}
