package agents

import (
	"context"
	"encoding/json"
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

// DirectiveAgent executes post-localization coding directives (e.g. adding UI switchers,
// wiring root providers) using a self-healing Claude Code-style agentic harness.
// Built to handle large files (10,000+ lines) via AST outlines and focused windowing.
type DirectiveAgent struct {
	LLM llm.Client
}

func NewDirectiveAgent() *DirectiveAgent {
	return &DirectiveAgent{
		LLM: llm.AutoDetectClient(),
	}
}

func NewDirectiveAgentWithClient(client llm.Client) *DirectiveAgent {
	return &DirectiveAgent{
		LLM: client,
	}
}

// DirectiveAction defines an action requested by the LLM in the tool-calling loop
type DirectiveAction struct {
	Action         string `json:"action"` // "scan_outline", "read_window", "write_component", "apply_patch", "finish"
	FilePath       string `json:"file_path,omitempty"`
	StartLine      int    `json:"start_line,omitempty"`
	EndLine        int    `json:"end_line,omitempty"`
	Content        string `json:"content,omitempty"`
	SearchSnippet  string `json:"search_snippet,omitempty"`
	ReplaceSnippet string `json:"replace_snippet,omitempty"`
	Explanation    string `json:"explanation,omitempty"`
}

// ExecuteDirective runs the self-healing coding harness to fulfill developer directives
func (da *DirectiveAgent) ExecuteDirective(ctx context.Context, projectRoot, directive string, targetLocales []string, framework types.Framework) (*types.DirectiveResult, error) {
	result := &types.DirectiveResult{
		Directive: directive,
		Success:   false,
	}

	if strings.TrimSpace(directive) == "" {
		result.Success = true
		return result, nil
	}

	if da.LLM == nil {
		result.Explanation = "LLM client not configured for directive execution"
		return result, nil
	}

	logger.Get().Info("DIRECTIVE_AGENT", fmt.Sprintf("Executing custom directive: %s (Framework: %s)", directive, framework))

	// Find project files for framework context
	candidateFiles := da.discoverRelevantFiles(projectRoot, framework)

	// Conversation state
	var conversationHistory []string
	systemPrompt := da.buildSystemPrompt(framework, targetLocales, candidateFiles)

	conversationHistory = append(conversationHistory, fmt.Sprintf("USER DIRECTIVE: %s\nPROJECT ROOT: %s\nLOCALES CONFIGURED: %v", directive, projectRoot, targetLocales))

	var touchedFiles []string
	createdFilesMap := make(map[string]bool)
	patchedFilesMap := make(map[string]bool)

	// Autonomous ReAct Loop with strict cost guard (Up to 6 turns max)
	const maxDirectiveTurns = 6
	for turn := 1; turn <= maxDirectiveTurns; turn++ {
		result.Attempts = turn

		userPrompt := strings.Join(conversationHistory, "\n\n---\n\n")

		reqCtx, cancel := context.WithTimeout(ctx, 40*time.Second)
		resp, err := da.LLM.Complete(reqCtx, systemPrompt, userPrompt)
		cancel()

		if err != nil {
			logger.Get().Warn("DIRECTIVE_AGENT", fmt.Sprintf("LLM completion error on turn %d: %v", turn, err))
			result.Explanation = fmt.Sprintf("AI provider error: %v", err)
			break
		}

		action, parseErr := da.parseActionJSON(resp)
		if parseErr != nil {
			// If not strict JSON, prompt agent to format cleanly
			conversationHistory = append(conversationHistory, fmt.Sprintf("OBSERVATION: Could not parse response as valid JSON action. Please output valid JSON following the schema. Error: %v\nYour raw output was:\n%s", parseErr, resp))
			continue
		}

		if action.Explanation != "" {
			result.Explanation = action.Explanation
		}

		// Handle agent action
		switch action.Action {
		case "finish":
			// Ensure created components are linked if the model forgot
			da.autoLinkComponent(projectRoot, createdFilesMap, patchedFilesMap, framework)

			// Validate compiler diagnostics on all touched files
			diags, _ := platforms.RunDiagnostics(projectRoot, touchedFiles)
			if len(diags) == 0 {
				result.Success = true
				result.CompilerPassed = true
				logger.Get().Info("DIRECTIVE_AGENT", "Directive completed successfully with 0 compiler errors")
				return da.finalizeResult(result, createdFilesMap, patchedFilesMap), nil
			}

			// Report compiler errors back to agent for self-healing
			diagSummary := da.formatDiagnostics(projectRoot, diags)
			conversationHistory = append(conversationHistory, fmt.Sprintf("OBSERVATION: Compiler check reported errors:\n%s\nPlease apply a patch or edit to fix these errors.", diagSummary))

		case "scan_outline":
			fullPath := da.resolvePath(projectRoot, action.FilePath)
			outline := da.scanFileOutline(fullPath)
			conversationHistory = append(conversationHistory, fmt.Sprintf("OBSERVATION [scan_outline for %s]:\n%s", action.FilePath, outline))

		case "read_window":
			fullPath := da.resolvePath(projectRoot, action.FilePath)
			window := da.readCodeWindow(fullPath, action.StartLine, action.EndLine)
			conversationHistory = append(conversationHistory, fmt.Sprintf("OBSERVATION [read_window for %s:%d-%d]:\n%s", action.FilePath, action.StartLine, action.EndLine, window))

		case "write_component":
			fullPath := da.resolvePath(projectRoot, action.FilePath)
			if err := da.writeComponent(fullPath, action.Content); err != nil {
				conversationHistory = append(conversationHistory, fmt.Sprintf("OBSERVATION: write_component failed: %v", err))
			} else {
				createdFilesMap[action.FilePath] = true
				touchedFiles = append(touchedFiles, action.FilePath)
				// Check syntax
				if !platforms.ParsesCleanly(fullPath, []byte(action.Content)) {
					conversationHistory = append(conversationHistory, fmt.Sprintf("OBSERVATION: Created %s, but Tree-sitter detected syntax errors. Please fix.", action.FilePath))
				} else {
					conversationHistory = append(conversationHistory, fmt.Sprintf("OBSERVATION: Successfully created %s (%d bytes). Tree-Sitter AST syntax verified clean. NEXT STEP: Call read_window or apply_patch on parent Navbar/Header to import and mount this component!", action.FilePath, len(action.Content)))
				}
			}

		case "apply_patch":
			fullPath := da.resolvePath(projectRoot, action.FilePath)
			if err := da.applySurgicalPatch(fullPath, action.SearchSnippet, action.ReplaceSnippet); err != nil {
				conversationHistory = append(conversationHistory, fmt.Sprintf("OBSERVATION: apply_patch failed: %v", err))
			} else {
				patchedFilesMap[action.FilePath] = true
				touchedFiles = append(touchedFiles, action.FilePath)
				conversationHistory = append(conversationHistory, fmt.Sprintf("OBSERVATION: Successfully applied surgical patch to %s. In-memory Tree-Sitter AST syntax verified clean.", action.FilePath))
			}

		default:
			conversationHistory = append(conversationHistory, fmt.Sprintf("OBSERVATION: Unknown action '%s'. Valid actions are scan_outline, read_window, write_component, apply_patch, finish.", action.Action))
		}
	}

	// Safety fallback: ensure any newly created components are linked
	da.autoLinkComponent(projectRoot, createdFilesMap, patchedFilesMap, framework)

	// Final check
	diags, _ := platforms.RunDiagnostics(projectRoot, touchedFiles)
	result.CompilerPassed = (len(diags) == 0)
	if len(createdFilesMap) > 0 || len(patchedFilesMap) > 0 {
		result.Success = true
	}

	return da.finalizeResult(result, createdFilesMap, patchedFilesMap), nil
}

func (da *DirectiveAgent) finalizeResult(result *types.DirectiveResult, created, patched map[string]bool) *types.DirectiveResult {
	for f := range created {
		result.CreatedFiles = append(result.CreatedFiles, f)
	}
	for f := range patched {
		result.PatchedFiles = append(result.PatchedFiles, f)
	}
	return result
}

// autoLinkComponent ensures created UI switcher widgets are mounted in parent navbars if the LLM forgot step 2
func (da *DirectiveAgent) autoLinkComponent(projectRoot string, created, patched map[string]bool, framework types.Framework) {
	var switcherRel string
	for f := range created {
		base := strings.ToLower(filepath.Base(f))
		if strings.Contains(base, "switcher") || strings.Contains(base, "picker") || strings.Contains(base, "language") {
			switcherRel = f
			break
		}
	}
	if switcherRel == "" {
		return
	}

	componentName := strings.TrimSuffix(filepath.Base(switcherRel), filepath.Ext(switcherRel))

	// Check if already mounted in JSX
	jsxUsage := fmt.Sprintf("<%s", componentName)
	for f := range patched {
		if data, err := os.ReadFile(filepath.Join(projectRoot, f)); err == nil {
			if strings.Contains(string(data), jsxUsage) {
				return // Already mounted in JSX
			}
		}
	}

	// Find parent navbar/header
	navCandidates := []string{
		"components/layout/Navbar.tsx",
		"components/Navbar.tsx",
		"src/components/layout/Navbar.tsx",
		"src/components/Navbar.tsx",
		"components/Header.tsx",
		"src/components/Header.tsx",
		"app/layout.tsx",
	}

	for _, navRel := range navCandidates {
		navAbs := filepath.Join(projectRoot, navRel)
		data, err := os.ReadFile(navAbs)
		if err != nil {
			continue
		}
		content := string(data)
		if strings.Contains(content, jsxUsage) {
			return
		}

		// Calculate relative import
		switcherAbs := filepath.Join(projectRoot, switcherRel)
		relImport, err := filepath.Rel(filepath.Dir(navAbs), switcherAbs)
		if err != nil {
			continue
		}
		relImport = filepath.ToSlash(relImport)
		relImport = strings.TrimSuffix(relImport, ".tsx")
		relImport = strings.TrimSuffix(relImport, ".ts")
		relImport = strings.TrimSuffix(relImport, ".jsx")
		relImport = strings.TrimSuffix(relImport, ".js")
		if !strings.HasPrefix(relImport, ".") {
			relImport = "./" + relImport
		}

		importStmt := fmt.Sprintf("import %s from '%s';", componentName, relImport)
		fullJsxUsage := fmt.Sprintf("<%s />", componentName)

		// Inject import after 'use client' if present and not already imported
		var updated string
		if strings.Contains(content, componentName) && (strings.Contains(content, "import") || strings.Contains(content, "from")) {
			updated = content
		} else {
			trimmed := strings.TrimSpace(content)
			if strings.HasPrefix(trimmed, "'use client'") || strings.HasPrefix(trimmed, "\"use client\"") {
				idx := strings.Index(content, "\n")
				if idx != -1 {
					updated = content[:idx+1] + importStmt + "\n" + content[idx+1:]
				} else {
					updated = content + "\n\n" + importStmt + "\n"
				}
			} else {
				updated = importStmt + "\n" + content
			}
		}

		// Inject JSX strictly into the JSX render tree (after return)
		returnIdx := strings.Index(updated, "return (")
		if returnIdx == -1 {
			returnIdx = strings.Index(updated, "return")
		}
		if returnIdx == -1 {
			returnIdx = 0
		}

		jsxSection := updated[returnIdx:]
		insertPos := -1

		if btnIdx := strings.Index(jsxSection, "toggleTheme"); btnIdx != -1 {
			if bStart := strings.LastIndex(jsxSection[:btnIdx], "<button"); bStart != -1 {
				insertPos = returnIdx + bStart
			}
		} else if flexIdx := strings.Index(jsxSection, "className=\"flex items-center gap-2\""); flexIdx != -1 {
			tagEnd := strings.Index(jsxSection[flexIdx:], ">")
			if tagEnd != -1 {
				insertPos = returnIdx + flexIdx + tagEnd + 1
			}
		} else if flexIdx := strings.Index(jsxSection, "className=\"flex items-center gap-1\""); flexIdx != -1 {
			tagEnd := strings.Index(jsxSection[flexIdx:], ">")
			if tagEnd != -1 {
				insertPos = returnIdx + flexIdx + tagEnd + 1
			}
		} else if btnIdx := strings.Index(jsxSection, "<button"); btnIdx != -1 {
			insertPos = returnIdx + btnIdx
		}

		if insertPos != -1 {
			updated = updated[:insertPos] + fullJsxUsage + "\n          " + updated[insertPos:]
		} else {
			if idx := strings.LastIndex(updated, "</nav>"); idx != -1 {
				updated = updated[:idx] + "  " + fullJsxUsage + "\n    " + updated[idx:]
			} else if idx := strings.LastIndex(updated, "</header>"); idx != -1 {
				updated = updated[:idx] + "  " + fullJsxUsage + "\n    " + updated[idx:]
			}
		}

		if platforms.ParsesCleanly(navAbs, []byte(updated)) {
			_ = os.WriteFile(navAbs, []byte(updated), 0644)
			patched[navRel] = true
			logger.Get().Info("DIRECTIVE_AGENT", fmt.Sprintf("Auto-linked %s into %s", componentName, navRel))
			return
		}
	}
}

// buildSystemPrompt creates a prompt with surgical large-file instructions
func da_buildSystemPrompt(framework types.Framework, locales []string, files []string) string {
	localesJSON, _ := json.Marshal(locales)
	filesList := strings.Join(files, ", ")
	if len(filesList) > 2000 {
		filesList = filesList[:2000] + "..."
	}

	return fmt.Sprintf(`You are the langPeanut Autonomous App Integration Agent.
You specialize in implementing post-localization directives (e.g. adding a Language Switcher component, wiring navigation dropdowns, or adding language picker buttons) with surgical AST precision.

TARGET FRAMEWORK: %s
CONFIGURED LOCALES: %s
DISCOVERED PROJECT FILES: %s

CRITICAL COMPONENT INTEGRATION RULES:
1. MANDATORY TWO-STEP INTEGRATION:
   - Step A (Create Component): Use "write_component" to create the widget file (e.g. components/LanguageSwitcher.tsx or lib/widgets/language_picker.dart).
   - Step B (Link & Mount): Use "read_window" on the parent container (e.g. components/Navbar.tsx or components/layout/Navbar.tsx), and then use "apply_patch" to BOTH import the new component AND render it (<LanguageSwitcher />) in the parent JSX/widget tree!
   - NEVER call "finish" until the component is actually imported and rendered inside the parent navigation/header container!
2. In React / Next.js, always include 'use client'; at the very top of interactive client components.
3. In LanguageSwitcher components, use i18next / react-i18next (e.g. const { i18n } = useTranslation(); i18n.changeLanguage(lng)) or the framework's standard localization API.
4. When completely finished and verified, output action "finish".

RESPONSE FORMAT:
You must respond with ONLY a single JSON object conforming to this schema:
{
  "action": "scan_outline" | "read_window" | "write_component" | "apply_patch" | "finish",
  "file_path": "path/to/file.tsx",
  "start_line": 1,
  "end_line": 60,
  "content": "file contents for write_component",
  "search_snippet": "exact snippet to replace",
  "replace_snippet": "exact replacement",
  "explanation": "concise description of what you did"
}`, framework, string(localesJSON), filesList)
}

func (da *DirectiveAgent) buildSystemPrompt(framework types.Framework, locales []string, files []string) string {
	return da_buildSystemPrompt(framework, locales, files)
}

func (da *DirectiveAgent) parseActionJSON(resp string) (*DirectiveAction, error) {
	clean := strings.TrimSpace(resp)
	if strings.HasPrefix(clean, "```") {
		firstLine := strings.Index(clean, "\n")
		if firstLine != -1 {
			clean = clean[firstLine+1:]
		}
		if strings.HasSuffix(clean, "```") {
			clean = strings.TrimSuffix(clean, "```")
		}
		clean = strings.TrimSpace(clean)
	}

	// If there is surrounding text, find the first '{' and last '}'
	start := strings.Index(clean, "{")
	end := strings.LastIndex(clean, "}")
	if start != -1 && end != -1 && end > start {
		clean = clean[start : end+1]
	}

	var action DirectiveAction
	if err := json.Unmarshal([]byte(clean), &action); err != nil {
		return nil, err
	}
	return &action, nil
}

// scanFileOutline extracts a compact skeleton (< 60 lines) of a large file
func (da *DirectiveAgent) scanFileOutline(filePath string) string {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Sprintf("Error reading file: %v", err)
	}

	lines := strings.Split(string(data), "\n")
	totalLines := len(lines)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Total Lines: %d\n", totalLines))
	sb.WriteString("--- Top Imports & Header ---\n")

	importLimit := 35
	if totalLines < importLimit {
		importLimit = totalLines
	}
	for i := 0; i < importLimit; i++ {
		sb.WriteString(fmt.Sprintf("%4d: %s\n", i+1, lines[i]))
	}

	sb.WriteString("\n--- Detected Component & Export Signatures ---\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "export default") ||
			strings.HasPrefix(trimmed, "export function") ||
			strings.HasPrefix(trimmed, "export const") ||
			strings.HasPrefix(trimmed, "class ") ||
			strings.HasPrefix(trimmed, "struct ") ||
			strings.HasPrefix(trimmed, "Widget build(") ||
			strings.HasPrefix(trimmed, "return (") ||
			strings.HasPrefix(trimmed, "<nav") ||
			strings.HasPrefix(trimmed, "<header") {
			sb.WriteString(fmt.Sprintf("Line %4d: %s\n", i+1, trimmed))
		}
	}

	return sb.String()
}

// readCodeWindow reads only a slice of lines (max 120 lines)
func (da *DirectiveAgent) readCodeWindow(filePath string, startLine, endLine int) string {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Sprintf("Error reading file: %v", err)
	}

	lines := strings.Split(string(data), "\n")
	total := len(lines)

	if startLine < 1 {
		startLine = 1
	}
	if endLine > total || endLine == 0 {
		endLine = total
	}
	if endLine-startLine > 120 {
		endLine = startLine + 120
	}

	var sb strings.Builder
	for i := startLine - 1; i < endLine && i < total; i++ {
		sb.WriteString(fmt.Sprintf("%4d: %s\n", i+1, lines[i]))
	}
	return sb.String()
}

// writeComponent creates a new file and parent directories
func (da *DirectiveAgent) writeComponent(filePath, content string) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	ext := strings.ToLower(filepath.Ext(filePath))
	if (ext == ".tsx" || ext == ".jsx") && (strings.Contains(content, "useTranslation") || strings.Contains(content, "useState") || strings.Contains(content, "useEffect")) {
		content = EnsureUseClientDirective(content)
	}
	return os.WriteFile(filePath, []byte(content), 0644)
}

// applySurgicalPatch replaces a specific text snippet in a file with Tree-Sitter syntax checks
func (da *DirectiveAgent) applySurgicalPatch(filePath, searchSnippet, replaceSnippet string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	content := string(data)
	if !strings.Contains(content, searchSnippet) {
		// Try normalized whitespace match
		cleanSearch := strings.TrimSpace(searchSnippet)
		if !strings.Contains(content, cleanSearch) {
			return fmt.Errorf("search_snippet not found in %s", filepath.Base(filePath))
		}
		searchSnippet = cleanSearch
	}

	patched := strings.Replace(content, searchSnippet, replaceSnippet, 1)

	// Validate in-memory with Tree-sitter
	if !platforms.ParsesCleanly(filePath, []byte(patched)) {
		return fmt.Errorf("patched result on %s failed Tree-Sitter AST syntax check", filepath.Base(filePath))
	}

	return os.WriteFile(filePath, []byte(patched), 0644)
}

func (da *DirectiveAgent) resolvePath(projectRoot, relPath string) string {
	if filepath.IsAbs(relPath) {
		return relPath
	}
	return filepath.Join(projectRoot, relPath)
}

func (da *DirectiveAgent) discoverRelevantFiles(projectRoot string, framework types.Framework) []string {
	var priorityFiles []string
	var otherFiles []string

	_ = filepath.Walk(projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			name := info.Name()
			if name == "node_modules" || name == ".git" || name == ".langPeanut" || name == "dist" || name == "build" || name == ".next" {
				return filepath.SkipDir
			}
			return nil
		}

		rel, _ := filepath.Rel(projectRoot, path)
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".tsx" || ext == ".jsx" || ext == ".dart" || ext == ".swift" || ext == ".vue" || ext == ".kt" {
			lowerRel := strings.ToLower(rel)
			if strings.Contains(lowerRel, "nav") || strings.Contains(lowerRel, "header") || strings.Contains(lowerRel, "layout") ||
				strings.Contains(lowerRel, "app") || strings.Contains(lowerRel, "bar") || strings.Contains(lowerRel, "main") ||
				strings.Contains(lowerRel, "switch") || strings.Contains(lowerRel, "picker") {
				priorityFiles = append(priorityFiles, rel)
			} else {
				otherFiles = append(otherFiles, rel)
			}
		}
		return nil
	})

	combined := append(priorityFiles, otherFiles...)
	if len(combined) > 60 {
		combined = combined[:60]
	}
	return combined
}

func (da *DirectiveAgent) formatDiagnostics(projectRoot string, diags []types.CompilerDiagnostic) string {
	var sb strings.Builder
	for i, d := range diags {
		rel, _ := filepath.Rel(projectRoot, d.FilePath)
		if rel == "" {
			rel = d.FilePath
		}
		sb.WriteString(fmt.Sprintf("%d. [%s] %s:%d:%d — %s\n", i+1, d.Source, rel, d.Line, d.Column, d.Message))
	}
	return sb.String()
}
