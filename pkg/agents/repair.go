package agents

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/langPeanut/langPeanut/pkg/llm"
	"github.com/langPeanut/langPeanut/pkg/logger"
	"github.com/langPeanut/langPeanut/pkg/platforms"
	"github.com/langPeanut/langPeanut/pkg/types"
)

// CodeRepairAgent specializes in diagnosing and automatically healing syntax & typecheck regressions
type CodeRepairAgent struct {
	LLM llm.Client
}

func NewCodeRepairAgent() *CodeRepairAgent {
	return &CodeRepairAgent{
		LLM: llm.AutoDetectClient(),
	}
}

func NewCodeRepairAgentWithClient(client llm.Client) *CodeRepairAgent {
	return &CodeRepairAgent{
		LLM: client,
	}
}

// RepairFile attempts to automatically fix compiler and AST syntax errors introduced into a file
func (cra *CodeRepairAgent) RepairFile(ctx context.Context, projectRoot, filePath string, errors []types.CompilerDiagnostic, framework types.Framework) (*types.CodeRepairResult, error) {
	result := &types.CodeRepairResult{
		FilePath:       filePath,
		OriginalErrors: errors,
		Repaired:       false,
	}

	if len(errors) == 0 {
		result.Repaired = true
		return result, nil
	}

	fullPath := filePath
	if !filepath.IsAbs(fullPath) {
		fullPath = filepath.Join(projectRoot, filePath)
	}

	contentBytes, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file for repair: %w", err)
	}
	currentContent := string(contentBytes)

	// Try deterministic heuristic repair first (e.g. missing import or basic bracket mismatch)
	healed, heuristicFixed := tryHeuristicRepair(currentContent, errors, framework)
	if heuristicFixed && platforms.ParsesCleanly(fullPath, []byte(healed)) {
		_ = os.WriteFile(fullPath, []byte(healed), 0644)
		newDiags, _ := platforms.RunDiagnostics(projectRoot, []string{filePath})
		if len(newDiags) == 0 {
			result.Repaired = true
			result.Attempts = 1
			result.Explanation = "Auto-repaired missing imports and JSX syntax using AST heuristics"
			return result, nil
		}
		currentContent = healed
	}

	// If live LLM is available, perform AI-powered surgical code repair with strict circuit breaker (max 2 attempts per file)
	if cra.LLM != nil && cra.LLM.Name() != llm.ProviderLocal {
		const maxAttempts = 2
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			result.Attempts = attempt

			repairedCode, repairErr := cra.callLLMRepair(ctx, fullPath, currentContent, errors, framework)
			if repairErr != nil {
				logger.Get().Warn("CODE_REPAIR", fmt.Sprintf("Repair attempt %d/%d failed for %s: %v", attempt, maxAttempts, filepath.Base(filePath), repairErr))
				continue
			}

			// Validate with in-memory tree-sitter parser before touching disk
			if platforms.ParsesCleanly(fullPath, []byte(repairedCode)) {
				// Save and re-run compiler diagnostics to confirm fix
				_ = os.WriteFile(fullPath, []byte(repairedCode), 0644)
				remainingDiags, _ := platforms.RunDiagnostics(projectRoot, []string{filePath})
				if len(remainingDiags) == 0 {
					result.Repaired = true
					result.Explanation = fmt.Sprintf("AI Code Repair Agent successfully healed %d compiler error(s) on attempt %d", len(errors), attempt)
					return result, nil
				}
				// Update currentContent for next attempt if partial progress made
				currentContent = repairedCode
				errors = remainingDiags
			}
		}
	}

	// Final verification check
	finalDiags, _ := platforms.RunDiagnostics(projectRoot, []string{filePath})
	if len(finalDiags) == 0 {
		result.Repaired = true
	} else {
		result.RemainingErrors = finalDiags
		result.Repaired = false
		result.Explanation = fmt.Sprintf("Cost circuit breaker triggered: Ceased repair after %d attempts (%d diagnostic(s) flagged for human review)", result.Attempts, len(finalDiags))
		logger.Get().Warn("CODE_REPAIR", fmt.Sprintf("Ceased repair after %d attempts for %s to prevent cost spikes (%d error(s) flagged for developer)", result.Attempts, filepath.Base(filePath), len(finalDiags)))
	}

	return result, nil
}

func (cra *CodeRepairAgent) callLLMRepair(ctx context.Context, filePath, content string, errors []types.CompilerDiagnostic, framework types.Framework) (string, error) {
	var errSummary strings.Builder
	for i, e := range errors {
		errSummary.WriteString(fmt.Sprintf("%d. [%s] Line %d:%d — %s\n", i+1, e.Source, e.Line, e.Column, e.Message))
	}

	systemPrompt := fmt.Sprintf(`You are the langPeanut Autonomous Code Repair Specialist.
A surgical localization refactor introduced a syntax or typecheck regression into a %s source file.

YOUR TASK:
Fix the syntax / missing imports / unclosed tags / mismatched brackets in the code below while preserving all localization hooks (e.g. t('key') or AppLocalizations.of(context)!.key).

STRICT RULES:
1. Return ONLY the complete, 100%% syntactically valid source code for this file.
2. DO NOT wrap with conversational preamble or commentary.
3. DO NOT hallucinate unrelated changes or rewrite working components.`, framework)

	userPrompt := fmt.Sprintf(`FILE: %s

COMPILER / AST DIAGNOSTICS:
%s

CURRENT SOURCE CODE:
%s`, filepath.Base(filePath), errSummary.String(), content)

	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := cra.LLM.Complete(reqCtx, systemPrompt, userPrompt)
	if err != nil {
		return "", err
	}

	// Clean code block wrappers if model wrapped in ```
	resp = strings.TrimSpace(resp)
	if strings.HasPrefix(resp, "```") {
		firstLineEnd := strings.Index(resp, "\n")
		if firstLineEnd != -1 {
			resp = resp[firstLineEnd+1:]
		}
		if strings.HasSuffix(resp, "```") {
			resp = strings.TrimSuffix(resp, "```")
		}
		resp = strings.TrimSpace(resp)
	}

	return resp, nil
}

// tryHeuristicRepair applies deterministic syntax fixes for common localization refactor errors
func tryHeuristicRepair(content string, errors []types.CompilerDiagnostic, framework types.Framework) (string, bool) {
	modified := false
	result := content

	for _, e := range errors {
		msg := strings.ToLower(e.Message)

		// 1. Missing useTranslation import in React / Next.js
		// Detects: file uses useTranslation() but the import statement is absent
		if (framework == types.FrameworkReact || framework == types.FrameworkNextJS) &&
			(strings.Contains(msg, "cannot find name 't'") || strings.Contains(msg, "usetranslation") ||
				strings.Contains(msg, "cannot find name 'usetranslation'")) {
			hasImport := strings.Contains(result, "from 'react-i18next'") || strings.Contains(result, "from \"react-i18next\"")
			if !hasImport {
				importStmt := "import { useTranslation } from 'react-i18next';\n"
				firstImport := strings.Index(result, "import ")
				if firstImport != -1 {
					result = result[:firstImport] + importStmt + result[firstImport:]
				} else {
					result = importStmt + result
				}
				modified = true
			}
		}

		// 2. Missing AppLocalizations import in Flutter
		if framework == types.FrameworkFlutter && strings.Contains(msg, "applocalizations") {
			if !strings.Contains(result, "app_localizations.dart") {
				importStmt := "import 'package:flutter_gen/gen_l10n/app_localizations.dart';\n"
				firstImport := strings.Index(result, "import ")
				if firstImport != -1 {
					result = result[:firstImport] + importStmt + result[firstImport:]
				} else {
					result = importStmt + result
				}
				modified = true
			}
		}

		// 3. Next.js App Router RSC createContext / "use client" error
		// Detects: "createContext only works in Client Components. Add the "use client" directive at the top of the file to use it"
		if (framework == types.FrameworkReact || framework == types.FrameworkNextJS) &&
			(strings.Contains(msg, "createcontext only works in client components") ||
				strings.Contains(msg, "add the \"use client\" directive") ||
				strings.Contains(msg, "add the 'use client' directive") ||
				strings.Contains(msg, "context-in-server-component") ||
				strings.Contains(msg, "you're importing a component that needs usestate") ||
				strings.Contains(msg, "client components")) {
			repaired := EnsureUseClientDirective(result)
			if repaired != result {
				result = repaired
				modified = true
			}
		}
	}

	return result, modified
}

