package platforms

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/langPeanut/langPeanut/pkg/types"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

// RunDiagnostics checks target files for AST syntax errors and native toolchain compiler diagnostics
func RunDiagnostics(projectRoot string, targetFiles []string) ([]types.CompilerDiagnostic, error) {
	return RunDiagnosticsWithCustom(projectRoot, targetFiles, "")
}

// RunDiagnosticsWithCustom checks target files for AST syntax errors, custom build/typecheck command, or native toolchain
func RunDiagnosticsWithCustom(projectRoot string, targetFiles []string, customBuildCmd string) ([]types.CompilerDiagnostic, error) {
	var diagnostics []types.CompilerDiagnostic

	// 1. In-Memory Tree-Sitter AST grammar checks
	for _, relPath := range targetFiles {
		fullPath := relPath
		if !filepath.IsAbs(fullPath) {
			fullPath = filepath.Join(projectRoot, relPath)
		}

		content, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}

		astErrors := checkASTErrors(fullPath, content)
		diagnostics = append(diagnostics, astErrors...)
	}

	// 2. Custom build / typecheck command or native toolchain
	if strings.TrimSpace(customBuildCmd) != "" {
		customErrors := runCustomBuildCheck(projectRoot, targetFiles, customBuildCmd)
		diagnostics = append(diagnostics, customErrors...)
	} else {
		toolchainErrors := runToolchainCheck(projectRoot, targetFiles)
		diagnostics = append(diagnostics, toolchainErrors...)
	}

	return deduplicateDiagnostics(diagnostics), nil
}

func runCustomBuildCheck(projectRoot string, targetFiles []string, customBuildCmd string) []types.CompilerDiagnostic {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if isWindows() {
		cmd = exec.CommandContext(ctx, "cmd.exe", "/c", customBuildCmd)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", customBuildCmd)
	}
	cmd.Dir = projectRoot
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf

	err := cmd.Run()
	if err == nil {
		return nil // Build succeeded with 0 errors
	}

	output := outBuf.String()
	// Try parsing standard TypeScript/Dart compiler formats first
	diags := parseTypeScriptOutput(output, targetFiles)
	if len(diags) == 0 {
		diags = parseDartOutput(output, targetFiles)
	}

	if len(diags) == 0 && len(output) > 0 {
		errMsg := strings.TrimSpace(output)
		if len(errMsg) > 200 {
			errMsg = errMsg[:197] + "..."
		}
		targetFile := "project"
		if len(targetFiles) > 0 {
			targetFile = targetFiles[0]
		}
		diags = append(diags, types.CompilerDiagnostic{
			FilePath: targetFile,
			Line:     1,
			Column:   1,
			Message:  fmt.Sprintf("[%s] %s", customBuildCmd, errMsg),
			Source:   "custom-build",
			Severity: "ERROR",
		})
	}

	return diags
}

// checkASTErrors locates specific error nodes in the tree-sitter AST
func checkASTErrors(filePath string, content []byte) []types.CompilerDiagnostic {
	var diags []types.CompilerDiagnostic
	ext := strings.ToLower(filepath.Ext(filePath))

	var parser *sitter.Parser
	switch ext {
	case ".tsx", ".jsx", ".ts", ".js":
		parser = newTSXParser()
	case ".dart":
		parser = newDartParser()
	case ".swift":
		parser = newSwiftParser()
	case ".kt":
		parser = newKotlinParser()
	default:
		return nil
	}
	defer parser.Close()

	tree := parser.Parse(content, nil)
	if tree == nil {
		return []types.CompilerDiagnostic{
			{
				FilePath: filePath,
				Line:     1,
				Column:   1,
				Message:  "Failed to initialize AST parse tree",
				Source:   "ast",
				Severity: "ERROR",
			},
		}
	}
	defer tree.Close()

	root := tree.RootNode()
	if !root.HasError() {
		return nil
	}

	// Traverse AST to locate error nodes
	walkErrorNodes(root, content, filePath, &diags)
	return diags
}

func walkErrorNodes(node *sitter.Node, content []byte, filePath string, diags *[]types.CompilerDiagnostic) {
	if node.IsError() || node.IsMissing() {
		startPoint := node.StartPosition()
		line := int(startPoint.Row) + 1
		col := int(startPoint.Column) + 1

		startByte := node.StartByte()
		endByte := node.EndByte()
		snippet := ""
		if int(endByte) <= len(content) && startByte < endByte {
			snippet = strings.TrimSpace(string(content[startByte:endByte]))
			if len(snippet) > 40 {
				snippet = snippet[:37] + "..."
			}
		}

		msg := "Syntax grammar error"
		if node.IsMissing() {
			msg = fmt.Sprintf("Missing syntax element '%s'", node.Kind())
		} else if snippet != "" {
			msg = fmt.Sprintf("Unexpected syntax token: '%s'", snippet)
		}

		*diags = append(*diags, types.CompilerDiagnostic{
			FilePath: filePath,
			Line:     line,
			Column:   col,
			Message:  msg,
			Source:   "ast",
			Severity: "ERROR",
		})
		return
	}

	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(uint(i))
		if child != nil && child.HasError() {
			walkErrorNodes(child, content, filePath, diags)
		}
	}
}

// runToolchainCheck executes fast non-emitting compiler checks (e.g. tsc --noEmit, dart analyze)
func runToolchainCheck(projectRoot string, targetFiles []string) []types.CompilerDiagnostic {
	var diags []types.CompilerDiagnostic

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	// TypeScript / Next.js
	if FileExists(projectRoot, "tsconfig.json") || FileExists(projectRoot, "package.json") {
		cmd := exec.CommandContext(ctx, "npx", "--no-install", "tsc", "--noEmit", "--pretty", "false")
		cmd.Dir = projectRoot
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		_ = cmd.Run()

		tsDiags := parseTypeScriptOutput(out.String(), targetFiles)
		diags = append(diags, tsDiags...)
	}

	// Dart / Flutter
	if FileExists(projectRoot, "pubspec.yaml") {
		cmd := exec.CommandContext(ctx, "dart", "analyze")
		cmd.Dir = projectRoot
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		_ = cmd.Run()

		dartDiags := parseDartOutput(out.String(), targetFiles)
		diags = append(diags, dartDiags...)
	}

	return diags
}

// Regex for TypeScript errors: path/to/file.tsx(12,5): error TS2304: Cannot find name 't'.
var tsErrorRegex = regexp.MustCompile(`^(.+)\((\d+),(\d+)\):\s+(error|warning)\s+(TS\d+):\s+(.+)$`)

func parseTypeScriptOutput(output string, filterFiles []string) []types.CompilerDiagnostic {
	var diags []types.CompilerDiagnostic
	filterMap := make(map[string]bool)
	for _, f := range filterFiles {
		filterMap[filepath.Clean(f)] = true
		filterMap[filepath.Base(f)] = true
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		matches := tsErrorRegex.FindStringSubmatch(line)
		if len(matches) == 7 {
			file := matches[1]
			lineNum, _ := strconv.Atoi(matches[2])
			colNum, _ := strconv.Atoi(matches[3])
			severity := strings.ToUpper(matches[4])
			code := matches[5]
			msg := matches[6]

			// Only report errors matching the modified target files
			if len(filterFiles) > 0 && !filterMap[filepath.Clean(file)] && !filterMap[filepath.Base(file)] {
				continue
			}

			diags = append(diags, types.CompilerDiagnostic{
				FilePath: file,
				Line:     lineNum,
				Column:   colNum,
				Message:  fmt.Sprintf("[%s] %s", code, msg),
				Source:   "tsc",
				Severity: severity,
			})
		}
	}
	return diags
}

// Regex for Dart errors: info • Undefined name 't' • lib/main.dart:42:5 • undefined_identifier
var dartErrorRegex = regexp.MustCompile(`^(error|warning|info)\s+•\s+(.+)\s+•\s+(.+):(\d+):(\d+)\s+•\s+(.+)$`)

func parseDartOutput(output string, filterFiles []string) []types.CompilerDiagnostic {
	var diags []types.CompilerDiagnostic
	filterMap := make(map[string]bool)
	for _, f := range filterFiles {
		filterMap[filepath.Clean(f)] = true
		filterMap[filepath.Base(f)] = true
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		matches := dartErrorRegex.FindStringSubmatch(line)
		if len(matches) == 7 {
			severity := strings.ToUpper(matches[1])
			msg := matches[2]
			file := matches[3]
			lineNum, _ := strconv.Atoi(matches[4])
			colNum, _ := strconv.Atoi(matches[5])

			if len(filterFiles) > 0 && !filterMap[filepath.Clean(file)] && !filterMap[filepath.Base(file)] {
				continue
			}

			diags = append(diags, types.CompilerDiagnostic{
				FilePath: file,
				Line:     lineNum,
				Column:   colNum,
				Message:  msg,
				Source:   "dart",
				Severity: severity,
			})
		}
	}
	return diags
}

func deduplicateDiagnostics(diags []types.CompilerDiagnostic) []types.CompilerDiagnostic {
	seen := make(map[string]bool)
	var unique []types.CompilerDiagnostic

	for _, d := range diags {
		key := fmt.Sprintf("%s:%d:%d:%s", filepath.Clean(d.FilePath), d.Line, d.Column, d.Message)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, d)
		}
	}
	return unique
}
