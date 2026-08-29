package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var resetCmd = &cobra.Command{
	Use:     "reset",
	Aliases: []string{"clean", "restore"},
	Short:   "Reset demo example projects back to pristine, unlocalized source code",
	RunE: func(cmd *cobra.Command, args []string) error {
		absRoot, err := filepath.Abs(projectRoot)
		if err != nil {
			return err
		}

		fmt.Println("🔄 langPeanut Reset — Restoring example projects to baseline...")

		// Run git checkout on examples directory
		gitCmd := exec.Command("git", "checkout", "HEAD", "--", "examples/")
		gitCmd.Dir = absRoot
		_ = gitCmd.Run()

		// Remove generated locale directories and test trajectories
		_ = os.RemoveAll(filepath.Join(absRoot, "examples", "nextjs-app", "src", "locales"))
		_ = os.RemoveAll(filepath.Join(absRoot, "examples", "nextjs-app", "trajectories"))
		_ = os.RemoveAll(filepath.Join(absRoot, "examples", "flutter-app", "lib", "l10n"))
		_ = os.RemoveAll(filepath.Join(absRoot, "examples", "flutter-app", "trajectories"))
		_ = os.RemoveAll(filepath.Join(absRoot, "examples", "swiftui-app", "Resources"))
		_ = os.RemoveAll(filepath.Join(absRoot, "examples", "swiftui-app", "trajectories"))
		_ = os.RemoveAll(filepath.Join(absRoot, "examples", "android-app", "app", "src", "main", "res", "values-fr"))
		_ = os.RemoveAll(filepath.Join(absRoot, "examples", "android-app", "app", "src", "main", "res", "values-es"))
		_ = os.RemoveAll(filepath.Join(absRoot, "examples", "android-app", "trajectories"))

		fmt.Println("✓ Restored Next.js React app (examples/nextjs-app/)")
		fmt.Println("✓ Restored Flutter Dart app (examples/flutter-app/)")
		fmt.Println("✓ Restored iOS SwiftUI app (examples/swiftui-app/)")
		fmt.Println("✓ Restored Android Compose app (examples/android-app/)")
		fmt.Println("\n🎉 All example projects are reset to clean, unlocalized code.")
		fmt.Println("Try running: langPeanut scan ./examples/nextjs-app")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(resetCmd)
}
