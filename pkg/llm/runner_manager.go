package llm

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/langPeanut/langPeanut/pkg/logger"
)

// GetUserBinDir returns ~/.langPeanut/bin for user-space static binaries
func GetUserBinDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".langPeanut", "bin")
}

// IsLlamaCLIInstalled checks whether llama-cli is available in PATH or user directories
func IsLlamaCLIInstalled() (bool, string) {
	candidates := []string{
		filepath.Join(GetUserBinDir(), "llama-cli"),
		"/opt/homebrew/bin/llama-cli",
		"/usr/local/bin/llama-cli",
		"llama-cli",
		"llama",
	}

	for _, c := range candidates {
		if path, err := exec.LookPath(c); err == nil {
			return true, path
		}
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return true, c
		}
	}
	return false, ""
}

// InstallLlamaCLIViaBrew attempts a user-space installation of llama.cpp using Homebrew
func InstallLlamaCLIViaBrew(ctx context.Context, progressFn func(line string)) (string, error) {
	brewPath, err := exec.LookPath("brew")
	if err != nil {
		// Check common macOS brew location
		if _, statErr := os.Stat("/opt/homebrew/bin/brew"); statErr == nil {
			brewPath = "/opt/homebrew/bin/brew"
		} else if _, statErr := os.Stat("/usr/local/bin/brew"); statErr == nil {
			brewPath = "/usr/local/bin/brew"
		} else {
			return "", fmt.Errorf("Homebrew ('brew') not found. Please install Homebrew (https://brew.sh) or compile llama.cpp manually")
		}
	}

	logger.Get().Info("RUNNER:INSTALL", fmt.Sprintf("Installing llama.cpp via Homebrew (%s install llama.cpp)...", brewPath))
	if progressFn != nil {
		progressFn("Running: brew install llama.cpp (user-space, zero root/sudo required)...")
	}

	cmd := exec.CommandContext(ctx, brewPath, "install", "llama.cpp")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start brew install: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		text := scanner.Text()
		logger.Get().Debug("RUNNER:BREW", text)
		if progressFn != nil {
			progressFn(text)
		}
	}

	if err := cmd.Wait(); err != nil {
		return "", fmt.Errorf("brew install llama.cpp exited with error: %w", err)
	}

	// Verify installation
	installed, path := IsLlamaCLIInstalled()
	if !installed {
		return "", fmt.Errorf("brew install finished but llama-cli could not be located in standard PATH")
	}

	logger.Get().Success("RUNNER:INSTALL", fmt.Sprintf("Successfully installed llama-cli at %s", path))
	return path, nil
}
