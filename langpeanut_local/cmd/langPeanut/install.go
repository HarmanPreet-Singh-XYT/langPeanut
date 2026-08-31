package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/langPeanut/langPeanut/pkg/platforms"
	"github.com/langPeanut/langPeanut/pkg/types"
	"github.com/spf13/cobra"
)

var (
	noInstallFlag        bool
	installCmdCustomFlag string
)

var installCmd = &cobra.Command{
	Use:     "install [directory]",
	Aliases: []string{"deps", "setup", "add-deps", "get"},
	Short:   "Check and install framework localization dependencies (e.g. react-i18next, flutter_localizations)",
	Long: `langPeanut Install — Autonomous Framework Localization Dependency Manager

Inspects your project framework (React/Next.js, Flutter, Android, SwiftUI, etc.), detects missing i18n
packages and configuration files, updates project manifests (package.json, pubspec.yaml),
creates necessary bootstrap setup files (e.g. i18n.ts, l10n.yaml), and runs package manager
installation (npm/pnpm/yarn/bun/flutter or custom install command).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir := projectRoot
		if len(args) > 0 {
			targetDir = args[0]
		}
		absRoot, err := filepath.Abs(targetDir)
		if err != nil {
			return err
		}

		registry := platforms.NewRegistry()
		platform, _ := registry.AutoDetect(absRoot)

		fmt.Println("┌────────────────────────────────────────────────────────────────────────┐")
		fmt.Printf("│ langPeanut Dependency Manager                                         │\n")
		fmt.Printf("│ Target:    %-59s │\n", truncatePath(absRoot, 59))
		fmt.Printf("│ Framework: %-59s │\n", platform.DisplayName())
		if installCmdCustomFlag != "" {
			fmt.Printf("│ Command:   %-59s │\n", installCmdCustomFlag)
		}
		fmt.Println("└────────────────────────────────────────────────────────────────────────┘")

		var status *types.DependencyStatus
		if installCmdCustomFlag != "" && !noInstallFlag {
			cmdStr, out, execErr := platforms.ExecuteCustomCommand(absRoot, installCmdCustomFlag)
			status, err = platform.EnsureDependencies(absRoot, false)
			if err != nil {
				return fmt.Errorf("failed to update manifests: %w", err)
			}
			if status != nil {
				status.CommandExecuted = cmdStr
				status.CommandOutput = out
				if execErr != nil {
					status.Message = fmt.Sprintf("Custom command executed with warning: %v", execErr)
				} else {
					status.Message = fmt.Sprintf("Custom command '%s' completed successfully", installCmdCustomFlag)
				}
			}
		} else {
			status, err = platform.EnsureDependencies(absRoot, !noInstallFlag)
			if err != nil {
				return fmt.Errorf("failed to ensure dependencies: %w", err)
			}
		}

		fmt.Println("\nDependency Status Report:")
		fmt.Println("──────────────────────────────────────────────────────────────────────────")
		fmt.Printf("  • Manifest:         %s\n", status.ManifestFile)
		if len(status.MissingDeps) > 0 {
			fmt.Printf("  • Detected Missing: %s\n", strings.Join(status.MissingDeps, ", "))
		}
		if status.ManifestUpdated {
			fmt.Printf("  • Manifest Updated: ✓ Injected missing dependencies into %s\n", status.ManifestFile)
		}
		if len(status.ConfigCreated) > 0 {
			fmt.Printf("  • Configs Created:  ✓ %s\n", strings.Join(status.ConfigCreated, ", "))
		}
		if status.CommandExecuted != "" {
			fmt.Printf("  • Command Run:      %s\n", status.CommandExecuted)
			if status.CommandOutput != "" {
				lines := strings.Split(status.CommandOutput, "\n")
				if len(lines) > 5 {
					lines = lines[len(lines)-5:]
				}
				for _, l := range lines {
					if strings.TrimSpace(l) != "" {
						fmt.Printf("    │ %s\n", l)
					}
				}
			}
		}

		if status.Success {
			fmt.Printf("\n✓ %s\n", status.Message)
		} else {
			fmt.Printf("\n⚠️ %s\n", status.Message)
		}

		return nil
	},
}

func init() {
	installCmd.Flags().BoolVar(&noInstallFlag, "no-install", false, "Update manifests without executing the package manager install command")
	installCmd.Flags().StringVar(&installCmdCustomFlag, "cmd", "", "Custom shell command to execute (e.g. 'pnpm install', 'yarn add react-i18next i18next')")
	installCmd.Flags().StringVar(&installCmdCustomFlag, "custom-cmd", "", "Alias for --cmd")
	rootCmd.AddCommand(installCmd)
}
