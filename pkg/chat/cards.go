package chat

import (
	"fmt"
	"math"
	"strings"
)

// FormatMatrixCard constructs a visual Locale Coverage & Parity Matrix card
func FormatMatrixCard(data *MatrixCardData) UICard {
	var sb strings.Builder
	sb.WriteString("┌─── LOCALE COVERAGE & PARITY MATRIX ──────────────────────────────────┐\n")
	sb.WriteString(fmt.Sprintf("│ Project: %s (%s)  |  Total Keys: %d  |  Source: %s\n", data.ProjectRoot, data.Framework, data.TotalKeys, data.SourceLocale))
	sb.WriteString("│\n")

	for _, loc := range data.Locales {
		barLen := 20
		filled := int(math.Round((loc.Percentage / 100.0) * float64(barLen)))
		if filled > barLen {
			filled = barLen
		}
		if filled < 0 {
			filled = 0
		}
		empty := barLen - filled

		bar := strings.Repeat("=", filled) + strings.Repeat("-", empty)
		statusBadge := "[COMPLETE]"
		if loc.Status == "unlocalized" {
			statusBadge = "[UNLOCALIZED]"
		} else if loc.MissingCount > 0 {
			statusBadge = fmt.Sprintf("[%d MISSING]", loc.MissingCount)
		}

		sb.WriteString(fmt.Sprintf("│  %-14s [%s] %5.1f%% (%d/%d keys)  %s\n",
			fmt.Sprintf("%s (%s)", loc.LocaleName, loc.LocaleCode),
			bar,
			loc.Percentage,
			loc.Translated,
			loc.Total,
			statusBadge,
		))
	}
	sb.WriteString("└──────────────────────────────────────────────────────────────────────┘")

	return UICard{
		Type:         CardTypeMatrix,
		Title:        "Locale Coverage Matrix",
		Description:  fmt.Sprintf("%d keys across %d locales (%s)", data.TotalKeys, len(data.Locales), data.Framework),
		Data:         data,
		RenderedText: sb.String(),
	}
}

// FormatSERPCard constructs a Google Search visual preview card
func FormatSERPCard(data *SERPCardData) UICard {
	var sb strings.Builder
	safeStatus := "DESKTOP SAFE (<= 600px)"
	if !data.IsPixelSafe {
		safeStatus = "TRUNCATED (> 600px)"
	}

	sb.WriteString(fmt.Sprintf("┌─── SERP SIMULATION: %s ───────────────────────────────────────────┐\n", strings.ToUpper(data.Locale)))
	sb.WriteString(fmt.Sprintf("│ URL:     %s\n", data.DisplayURL))
	sb.WriteString(fmt.Sprintf("│ TITLE:   %s\n", data.Title))
	sb.WriteString(fmt.Sprintf("│ SNIPPET: %s\n", data.Snippet))
	sb.WriteString("│\n")
	sb.WriteString(fmt.Sprintf("│ WIDTH: %dpx / 600px [%s]\n", data.PixelWidth, safeStatus))
	sb.WriteString(fmt.Sprintf("│ METRICS: CTR Uplift +%.1f%%  |  Trust Score: %d/100  |  Keyword: '%s'\n",
		data.PredictedCTRGain, data.TrustScore, data.TargetKeyword))

	if len(data.FAQSchema) > 0 {
		sb.WriteString("│ SCHEMA:  " + fmt.Sprintf("%d interactive FAQ items attached", len(data.FAQSchema)) + "\n")
	}
	sb.WriteString("└──────────────────────────────────────────────────────────────────────┘")

	return UICard{
		Type:         CardTypeSERP,
		Title:        fmt.Sprintf("SERP Simulator (%s)", strings.ToUpper(data.Locale)),
		Description:  fmt.Sprintf("Title: %s (%dpx)", data.Title, data.PixelWidth),
		Data:         data,
		RenderedText: sb.String(),
	}
}

// FormatDiffCard constructs an AST patch code difference preview
func FormatDiffCard(data *DiffCardData) UICard {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("┌─── AST PATCH: %s ─────────────────────────────────────────┐\n", data.FilePath))
	if len(data.RequiredImports) > 0 {
		sb.WriteString(fmt.Sprintf("│ Imports: %s\n", strings.Join(data.RequiredImports, ", ")))
	}
	if len(data.RequiredHooks) > 0 {
		sb.WriteString(fmt.Sprintf("│ Hooks:   %s\n", strings.Join(data.RequiredHooks, ", ")))
	}
	if data.RemovedConstCount > 0 {
		sb.WriteString(fmt.Sprintf("│ Removed %d const declarations for dynamic translation\n", data.RemovedConstCount))
	}
	sb.WriteString("│\n")

	for _, hunk := range data.DiffHunks {
		lines := strings.Split(hunk, "\n")
		for _, l := range lines {
			if strings.HasPrefix(l, "+") {
				sb.WriteString(fmt.Sprintf("│ \033[32m%s\033[0m\n", l))
			} else if strings.HasPrefix(l, "-") {
				sb.WriteString(fmt.Sprintf("│ \033[31m%s\033[0m\n", l))
			} else {
				sb.WriteString(fmt.Sprintf("│ %s\n", l))
			}
		}
	}
	sb.WriteString("└──────────────────────────────────────────────────────────────────────┘")

	return UICard{
		Type:         CardTypeDiff,
		Title:        fmt.Sprintf("AST Patch: %s", data.FilePath),
		Description:  fmt.Sprintf("Framework: %s | %d diff hunks", data.Framework, len(data.DiffHunks)),
		Data:         data,
		RenderedText: sb.String(),
	}
}

// FormatCriticCard constructs a 4-Tier Critic Scorecard
func FormatCriticCard(data *CriticCardData) UICard {
	var sb strings.Builder
	overall := "ALL TIERS PASSED (100% Verified)"
	if !data.OverallPassed {
		overall = "VERIFICATION ISSUES DETECTED"
	}

	sb.WriteString("┌─── 4-TIER CRITIC VERIFICATION REPORT ────────────────────────────────┐\n")
	sb.WriteString(fmt.Sprintf("│ Overall Status: [%s]\n│\n", overall))

	tiers := []TierStatus{data.Tier1Syntax, data.Tier2ICU, data.Tier3Expansion, data.Tier4Parity}
	for _, t := range tiers {
		badge := "[PASS]"
		if !t.Passed {
			badge = "[FAIL]"
		} else if t.WarningCount > 0 {
			badge = fmt.Sprintf("[%d WARN]", t.WarningCount)
		}
		sb.WriteString(fmt.Sprintf("│  %-10s %-28s  %s\n", badge, t.TierName, t.Summary))
	}

	if len(data.Diagnostics) > 0 {
		sb.WriteString("│\n│ Diagnostics:\n")
		for i, d := range data.Diagnostics {
			if i >= 4 {
				sb.WriteString(fmt.Sprintf("│   ... and %d more items\n", len(data.Diagnostics)-4))
				break
			}
			sb.WriteString(fmt.Sprintf("│   * [%s] %s: %s\n", d.Severity, d.Key, d.Message))
		}
	}
	sb.WriteString("└──────────────────────────────────────────────────────────────────────┘")

	return UICard{
		Type:         CardTypeCritic,
		Title:        "4-Tier Critic Report",
		Description:  overall,
		Data:         data,
		RenderedText: sb.String(),
	}
}

// FormatCostCard constructs a token analytics and pricing card
func FormatCostCard(data *CostCardData) UICard {
	var sb strings.Builder
	sb.WriteString("┌─── TOKEN TELEMETRY & COST ESTIMATE ──────────────────────────────────┐\n")
	sb.WriteString(fmt.Sprintf("│ Provider: %-16s  |  Model: %s\n", data.Provider, data.ModelID))
	sb.WriteString(fmt.Sprintf("│ Input Tokens:       %8d   ($0.75-$2.00 / 1M)\n", data.InputTokens))
	sb.WriteString(fmt.Sprintf("│ Cached Input Reads: %8d   (Up to 90%% savings)\n", data.CachedReadTokens))
	sb.WriteString(fmt.Sprintf("│ Output Tokens:      %8d   ($4.50-$10.00 / 1M)\n", data.OutputTokens))
	sb.WriteString(fmt.Sprintf("│ Total Tokens:       %8d\n", data.TotalTokens))
	sb.WriteString("│\n")
	sb.WriteString(fmt.Sprintf("│ Net Estimated Cost: $%.4f USD  (Saved ~%.1f%% via TM & Cache)\n", data.EstimatedCostUSD, data.SavingsPercent))
	sb.WriteString("└──────────────────────────────────────────────────────────────────────┘")

	return UICard{
		Type:         CardTypeCost,
		Title:        "Token Analytics & Cost",
		Description:  fmt.Sprintf("%d total tokens ($%.4f USD)", data.TotalTokens, data.EstimatedCostUSD),
		Data:         data,
		RenderedText: sb.String(),
	}
}

// FormatCheckpointsCard constructs a rollback snapshots card
func FormatCheckpointsCard(data *CheckpointCardData) UICard {
	var sb strings.Builder
	sb.WriteString("┌─── ROLLBACK CHECKPOINTS ─────────────────────────────────────────────┐\n")
	if len(data.Checkpoints) == 0 {
		sb.WriteString("│ No previous checkpoints recorded for this workspace.\n")
	} else {
		for i, ck := range data.Checkpoints {
			activeMark := " "
			if ck.ID == data.ActiveID || (data.ActiveID == "" && i == 0) {
				activeMark = "*"
			}
			sb.WriteString(fmt.Sprintf("│ %s [%s] %-20s  %d files  %s\n",
				activeMark,
				ck.CreatedAt.Format("15:04:05"),
				ck.ID,
				ck.FileCount,
				ck.Summary,
			))
		}
	}
	sb.WriteString("│\n│ Execute 'rollback <id>' or 'undo' to revert changes.\n")
	sb.WriteString("└──────────────────────────────────────────────────────────────────────┘")

	return UICard{
		Type:         CardTypeCheckpoints,
		Title:        "Rollback Checkpoints",
		Description:  fmt.Sprintf("%d snapshots available", len(data.Checkpoints)),
		Data:         data,
		RenderedText: sb.String(),
	}
}

// FormatConfigCard constructs a settings and configuration inspector card
func FormatConfigCard(data *ConfigCardData) UICard {
	var sb strings.Builder
	sb.WriteString("┌─── SYSTEM CONFIGURATION & MODEL PARAMETERS ──────────────────────────┐\n")
	sb.WriteString(fmt.Sprintf("│ Active Provider:    %-20s\n", data.ActiveProvider))
	sb.WriteString(fmt.Sprintf("│ Active Model:       %-20s\n", data.ActiveModel))
	sb.WriteString(fmt.Sprintf("│ Tone Persona:       %-20s\n", data.StylePreset))
	sb.WriteString(fmt.Sprintf("│ Concurrency:        %-2d parallel workers\n", data.Concurrency))
	sb.WriteString(fmt.Sprintf("│ Chunk Word Budget:  %-5d words per batch\n", data.ChunkWords))
	sb.WriteString(fmt.Sprintf("│ Chunk Key Ceiling:  %-5d keys per batch\n", data.ChunkKeys))
	sb.WriteString(fmt.Sprintf("│ Auto-Gitignore:     %-5t\n", data.AutoGitignore))
	sb.WriteString("│\n│ API Key Status:\n")
	for p, ok := range data.APIKeyConfig {
		badge := "[MISSING]"
		if ok {
			badge = "[CONFIGURED]"
		}
		sb.WriteString(fmt.Sprintf("│   * %-16s %s\n", p, badge))
	}
	sb.WriteString("└──────────────────────────────────────────────────────────────────────┘")

	return UICard{
		Type:         CardTypeConfig,
		Title:        "System Configuration",
		Description:  fmt.Sprintf("Provider: %s | Model: %s", data.ActiveProvider, data.ActiveModel),
		Data:         data,
		RenderedText: sb.String(),
	}
}

// FormatActionButtons constructs interactive triggers
func FormatActionButtons(title string, buttons []ActionButton) UICard {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("┌─── ACTIONS: %s ──────────────────────────────────┐\n│ ", title))
	for i, b := range buttons {
		sb.WriteString(fmt.Sprintf("[%s] ", b.Label))
		if i > 0 && i%3 == 0 {
			sb.WriteString("\n│ ")
		}
	}
	sb.WriteString("\n└──────────────────────────────────────────────────────────────────────┘")

	return UICard{
		Type:         CardTypeActionButton,
		Title:        title,
		Data:         buttons,
		RenderedText: sb.String(),
	}
}
