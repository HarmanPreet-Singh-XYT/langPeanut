package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	resetHard bool
)

var resetCmd = &cobra.Command{
	Use:     "reset [path]",
	Aliases: []string{"clean", "restore"},
	Short:   "Reset target repository or demo projects back to pristine, unlocalized source code",
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir := projectRoot
		if len(args) > 0 {
			targetDir = args[0]
		}
		absRoot, err := filepath.Abs(targetDir)
		if err != nil {
			return err
		}

		// If resetting specific project directory
		if targetDir != "" && targetDir != "." && targetDir != "./" {
			fmt.Printf("[RESET] Restoring project %s to clean baseline...\n", absRoot)
			_ = os.RemoveAll(filepath.Join(absRoot, ".langPeanut"))
			_ = os.RemoveAll(filepath.Join(absRoot, "trajectories"))
			_ = os.RemoveAll(filepath.Join(absRoot, ".trajectories"))

			if resetHard {
				gitCmd := exec.Command("git", "checkout", "HEAD", "--", ".")
				gitCmd.Dir = absRoot
				_ = gitCmd.Run()
			}
			fmt.Printf("[OK] Successfully cleared .langPeanut cache and localization state for %s\n", absRoot)
			return nil
		}

		fmt.Println("[RESET] langPeanut Reset — Restoring projects and caches to baseline...")

		// Run git checkout on examples directory
		gitCmd := exec.Command("git", "checkout", "HEAD", "--", "examples/")
		gitCmd.Dir = absRoot
		_ = gitCmd.Run()

		// Remove generated locale directories and test trajectories
		_ = os.RemoveAll(filepath.Join(absRoot, "examples", "nextjs-app", "src", "locales"))
		_ = os.RemoveAll(filepath.Join(absRoot, "examples", "nextjs-app", "trajectories"))
		_ = os.RemoveAll(filepath.Join(absRoot, "examples", "nextjs-app", ".langPeanut"))
		_ = os.RemoveAll(filepath.Join(absRoot, "examples", "flutter-app", "lib", "l10n"))
		_ = os.RemoveAll(filepath.Join(absRoot, "examples", "flutter-app", "trajectories"))
		_ = os.RemoveAll(filepath.Join(absRoot, "examples", "flutter-app", ".langPeanut"))
		_ = os.RemoveAll(filepath.Join(absRoot, "examples", "swiftui-app", "Resources"))
		_ = os.RemoveAll(filepath.Join(absRoot, "examples", "swiftui-app", "trajectories"))
		_ = os.RemoveAll(filepath.Join(absRoot, "examples", "swiftui-app", ".langPeanut"))
		_ = os.RemoveAll(filepath.Join(absRoot, "examples", "android-app", "app", "src", "main", "res", "values-fr"))
		_ = os.RemoveAll(filepath.Join(absRoot, "examples", "android-app", "app", "src", "main", "res", "values-es"))
		_ = os.RemoveAll(filepath.Join(absRoot, "examples", "android-app", "trajectories"))
		_ = os.RemoveAll(filepath.Join(absRoot, "examples", "android-app", ".langPeanut"))

		fmt.Println("[OK] Restored Next.js React app (examples/nextjs-app/)")
		fmt.Println("[OK] Restored Flutter Dart app (examples/flutter-app/)")
		fmt.Println("[OK] Restored iOS SwiftUI app (examples/swiftui-app/)")
		fmt.Println("[OK] Restored Android Compose app (examples/android-app/)")
		fmt.Println("\n[OK] All example projects are reset to clean, unlocalized code.")
		fmt.Println("Try running: langPeanut scan ./examples/nextjs-app")
		return nil
	},
}

func init() {
	resetCmd.Flags().BoolVar(&resetHard, "hard", false, "Discard all uncommitted git changes in target directory")
	rootCmd.AddCommand(resetCmd)
}
