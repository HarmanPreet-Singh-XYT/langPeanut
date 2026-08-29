package main

import (
	"fmt"
	"path/filepath"

	"github.com/langPeanut/langPeanut/benchmark"
	"github.com/spf13/cobra"
)

var benchmarkCmd = &cobra.Command{
	Use:   "benchmark",
	Short: "Run the 10-Case Adversarial Benchmark Suite comparing baseline vs multi-agent workflow",
	RunE: func(cmd *cobra.Command, args []string) error {
		absRoot, err := filepath.Abs(projectRoot)
		if err != nil {
			return err
		}

		benchDir := filepath.Join(absRoot, "benchmark", "workspace")
		fmt.Printf("🚀 micro1 Hackathon — Running 10-Case Adversarial Benchmark Suite...\n\n")

		results, err := benchmark.RunBenchmark(benchDir)
		if err != nil {
			return err
		}

		fmt.Println("┌────┬─────────────────────────────┬───────────┬──────────────┬──────────────┬──────────────┬──────────────┐")
		fmt.Println("│ #  │ Test Case Name              │ Framework │ Baseline Win │ Regex Tool   │ langPeanut   │ Token Saved  │")
		fmt.Println("├────┼─────────────────────────────┼───────────┼──────────────┼──────────────┼──────────────┼──────────────┤")

		totalPass, totalBaseline, totalRegex := 0.0, 0.0, 0.0
		liveBaseline := false
		baselineProvider := ""
		for _, r := range results {
			totalPass += r.AgenticPassRate
			totalBaseline += r.BaselinePassRate
			totalRegex += r.RegexPassRate
			if r.BaselineIsLive {
				liveBaseline = true
				baselineProvider = r.BaselineProvider
			}
			fmt.Printf("│ %-2d │ %-27s │ %-9s │ %-12.1f%%│ %-12.1f%%│ %-12.1f%%│ %-12.1f%%│\n",
				r.CaseID, r.CaseName, r.Framework, r.BaselinePassRate, r.RegexPassRate, r.AgenticPassRate, r.TokenSavingsPct)
		}

		fmt.Println("└────┴─────────────────────────────┴───────────┴──────────────┴──────────────┴──────────────┴──────────────┘")

		n := float64(len(results))
		avgPass, avgBaseline, avgRegex := totalPass/n, totalBaseline/n, totalRegex/n

		baselineLabel := "historical estimate — set GEMINI_API_KEY to measure live"
		if liveBaseline {
			baselineLabel = fmt.Sprintf("measured live via %s", baselineProvider)
		}

		fmt.Printf("\n🏆 Overall Multi-Agent Pass Rate: %.1f%% (Zero-Shot Baseline: %.1f%% [%s] | Naive Regex: %.1f%% [measured live])\n",
			avgPass, avgBaseline, baselineLabel, avgRegex)
		fmt.Println("✓ Trajectories exported to `/trajectories/` for micro1 Hackathon Deliverable 04.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(benchmarkCmd)
}
