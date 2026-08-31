package main

import (
	"fmt"
	"sort"

	"github.com/langPeanut/langPeanut/pkg/llm"
	"github.com/spf13/cobra"
)

var resetStatsFlag bool

var statsCmd = &cobra.Command{
	Use:     "stats",
	Aliases: []string{"tokens", "usage", "metrics", "cost"},
	Short:   "View AI token consumption, model breakdowns, and estimated API costs",
	Run: func(cmd *cobra.Command, args []string) {
		tracker := llm.GetGlobalTracker()

		if resetStatsFlag {
			tracker.Reset()
			fmt.Println("✓ AI token usage metrics and history successfully reset to 0.")
			return
		}

		stats := tracker.GetStats()

		fmt.Println("langPeanut — AI Token Consumption & Cost Analytics")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Printf("  • Total API Requests:     %d\n", stats.TotalRequests)
		fmt.Printf("  • Total Input Tokens:     %s\n", formatNumber(stats.TotalInputTokens))
		fmt.Printf("  • Total Output Tokens:    %s\n", formatNumber(stats.TotalOutputTokens))
		fmt.Printf("  • Combined Tokens:        %s\n", formatNumber(stats.TotalTokens))
		fmt.Printf("  • Total Estimated Cost:   $%.4f USD\n", stats.TotalEstimatedCostUSD)
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println()

		if len(stats.ByModel) == 0 {
			fmt.Println("No AI token usage recorded yet. Run a translation or audit command to track tokens.")
			return
		}

		fmt.Printf("%-32s %-12s %-14s %-14s %-14s %-10s %-12s\n", "MODEL", "PROVIDER", "INPUT TOKENS", "OUTPUT TOKENS", "TOTAL TOKENS", "CALLS", "EST. COST")
		fmt.Println("────────────────────────────────────────────────────────────────────────────────────────────────────────────")

		// Sort models alphabetically
		var models []string
		for m := range stats.ByModel {
			models = append(models, m)
		}
		sort.Strings(models)

		for _, m := range models {
			u := stats.ByModel[m]
			fmt.Printf("%-32s %-12s %-14s %-14s %-14s %-10d $%.4f\n",
				truncateStr(u.Model, 30),
				u.Provider,
				formatNumber(u.InputTokens),
				formatNumber(u.OutputTokens),
				formatNumber(u.TotalTokens),
				u.Requests,
				u.EstimatedCostUSD,
			)
		}
		fmt.Println("────────────────────────────────────────────────────────────────────────────────────────────────────────────")
		fmt.Println("\nTip: Run `langPeanut stats --reset` to clear historical token metrics.")
	},
}

func init() {
	statsCmd.Flags().BoolVar(&resetStatsFlag, "reset", false, "Reset cumulative token tracking metrics to 0")
	rootCmd.AddCommand(statsCmd)
}

func formatNumber(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1_000_000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000.0)
	}
	return fmt.Sprintf("%.2fM", float64(n)/1_000_000.0)
}

func truncateStr(s string, max int) string {
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}
