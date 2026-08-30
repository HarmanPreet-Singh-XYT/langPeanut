package main

import (
	"context"
	"fmt"

	"github.com/langPeanut/langPeanut/pkg/llm"
	"github.com/spf13/cobra"
)

var modelsCmd = &cobra.Command{
	Use:     "models",
	Aliases: []string{"model"},
	Short:   "Manage local offline AI translation models and runners",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runListModels()
	},
}

var modelsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List local model artifacts and runtime runner status",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runListModels()
	},
}

var modelsDownloadCmd = &cobra.Command{
	Use:   "download [model_name]",
	Short: "Download local NLLB-200 offline model (380MB GGUF) from Hugging Face",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("langPeanut Models — Checking local model cache directory: %s\n", llm.GetNLLBModelDir())
		downloaded, path, size := llm.IsNLLBModelDownloaded()
		if downloaded {
			fmt.Printf("✓ Model weights already downloaded: %s (%.1f MB)\n", path, float64(size)/(1024*1024))
		} else {
			fmt.Printf("Downloading Meta NLLB-200 Distilled 600M (4-bit GGUF, ~380MB)...\n")
			var lastReported int = -1
			savedPath, err := llm.EnsureNLLBModel(context.Background(), func(downloaded, total int64, percent float64) {
				curr := int(percent)
				if curr%10 == 0 && curr != lastReported {
					lastReported = curr
					fmt.Printf("  ↳ Progress: %3d%%  (%d MB / %d MB)\n", curr, downloaded/(1024*1024), total/(1024*1024))
				}
			})
			if err != nil {
				return fmt.Errorf("model download failed: %w", err)
			}
			fmt.Printf("\n✓ Successfully downloaded Meta NLLB-200 weights to %s\n", savedPath)
		}

		// Check runner status
		runnerInstalled, runnerPath := llm.IsLlamaCLIInstalled()
		if runnerInstalled {
			fmt.Printf("✓ Local GGUF Runner ready: %s\n", runnerPath)
		} else {
			fmt.Println("\n⚠️  Local GGUF execution requires llama.cpp to run on your CPU/Metal GPU.")
			fmt.Println("To install the runner without root/sudo permissions, run:")
			fmt.Println("  langPeanut models install-runner")
			fmt.Println("Or install via Homebrew directly:")
			fmt.Println("  brew install llama.cpp")
		}

		return nil
	},
}

var modelsInstallRunnerCmd = &cobra.Command{
	Use:     "install-runner",
	Aliases: []string{"runner", "setup-runner"},
	Short:   "Install local llama.cpp runner via Homebrew (zero root/sudo required)",
	RunE: func(cmd *cobra.Command, args []string) error {
		installed, path := llm.IsLlamaCLIInstalled()
		if installed {
			fmt.Printf("✓ llama.cpp runner is already installed and operational at: %s\n", path)
			return nil
		}

		fmt.Println("Installing llama.cpp via Homebrew (user-space, zero root/sudo permissions needed)...")
		installedPath, err := llm.InstallLlamaCLIViaBrew(context.Background(), func(line string) {
			fmt.Println("  ↳", line)
		})
		if err != nil {
			return fmt.Errorf("failed to install llama.cpp: %w\nYou can also install manually by running: brew install llama.cpp", err)
		}

		fmt.Printf("\n✓ Successfully installed llama.cpp runner to: %s\n", installedPath)
		fmt.Println("You can now test local GGUF translation:")
		fmt.Println("  langPeanut test-model --provider nllb-local --target es")
		return nil
	},
}

var modelsPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print local model directory path",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(llm.GetNLLBModelDir())
	},
}

func runListModels() error {
	fmt.Println("Local AI Translation Models & Runners:")
	fmt.Println("──────────────────────────────────────────────────────────────────────────")

	downloaded, path, size := llm.IsNLLBModelDownloaded()
	status := "[NOT DOWNLOADED]"
	sizeStr := "380 MB"
	if downloaded {
		status = "[INSTALLED]"
		sizeStr = fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
	}

	fmt.Printf(" • Meta NLLB-200 (600M Distilled Q4_K_M GGUF)\n")
	fmt.Printf("   Weights:   %-20s\n", status)
	fmt.Printf("   Disk Size: %s\n", sizeStr)
	fmt.Printf("   Path:      %s\n", path)

	runnerInstalled, runnerPath := llm.IsLlamaCLIInstalled()
	runnerStatus := "\033[33m[NOT INSTALLED — run 'langPeanut models install-runner']\033[0m"
	if runnerInstalled {
		runnerStatus = fmt.Sprintf("\033[32m[READY: %s]\033[0m", runnerPath)
	}
	fmt.Printf("   Runner:    %s\n", runnerStatus)
	fmt.Printf("   Languages: 200+ world languages (100%% offline, zero API token cost)\n")
	fmt.Println("──────────────────────────────────────────────────────────────────────────")

	if !downloaded {
		fmt.Println("\nTo download model weights:")
		fmt.Println("  langPeanut models download")
	}
	if !runnerInstalled {
		fmt.Println("\nTo install local runner (zero root/sudo needed):")
		fmt.Println("  langPeanut models install-runner   (or 'brew install llama.cpp')")
	}
	if downloaded && runnerInstalled {
		fmt.Println("\n✓ Everything is ready for offline translation:")
		fmt.Println("  langPeanut test-model --provider nllb-local --target es")
	}
	return nil
}

func init() {
	modelsCmd.AddCommand(modelsListCmd)
	modelsCmd.AddCommand(modelsDownloadCmd)
	modelsCmd.AddCommand(modelsInstallRunnerCmd)
	modelsCmd.AddCommand(modelsPathCmd)
	rootCmd.AddCommand(modelsCmd)
}
