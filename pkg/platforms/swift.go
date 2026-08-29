package platforms

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"

	treesitterswift "github.com/langPeanut/langPeanut/pkg/platforms/thirdparty/treesitterswift"
	"github.com/langPeanut/langPeanut/pkg/types"
)

// SwiftPlatform implements Platform for iOS / SwiftUI
type SwiftPlatform struct{}

func NewSwiftPlatform() *SwiftPlatform {
	return &SwiftPlatform{}
}

func (p *SwiftPlatform) Name() types.Framework {
	return types.FrameworkSwiftUI
}

func (p *SwiftPlatform) DisplayName() string {
	return "iOS / SwiftUI (Swift / .xcstrings)"
}

func (p *SwiftPlatform) FileExtensions() []string {
	return []string{".swift"}
}

func (p *SwiftPlatform) SkipDirs() []string {
	return []string{".build", "Pods", "DerivedData", ".git"}
}

func (p *SwiftPlatform) Detect(projectRoot string) (bool, float64) {
	if FileExists(projectRoot, "Package.swift") {
		return true, 0.95
	}
	if len(findFilesWithExt(projectRoot, ".xcodeproj")) > 0 || len(findFilesWithExt(projectRoot, ".xcworkspace")) > 0 {
		return true, 0.98
	}
	return false, 0
}

func (p *SwiftPlatform) DefaultLocaleDir(projectRoot string) string {
	return "Resources"
}

func (p *SwiftPlatform) DefaultSourceFile(projectRoot string, sourceLocale string) string {
	return filepath.Join(p.DefaultLocaleDir(projectRoot), "Localizable.xcstrings")
}

// Function/view names whose string argument is UI-facing text.
var swiftUICallees = map[string]bool{
	"Text": true, "Label": true, "Button": true,
}

// .method(...) suffixes (called via navigation_expression) whose argument is UI-facing text.
var swiftUINavigationSuffixes = map[string]bool{
	"navigationTitle": true, "alert": true, "tooltip": true,
}

func newSwiftParser() *sitter.Parser {
	parser := sitter.NewParser()
	parser.SetLanguage(sitter.NewLanguage(treesitterswift.Language()))
	return parser
}

func (p *SwiftPlatform) ExtractCandidates(filePath string, content []byte) ([]types.StringCandidate, error) {
	parser := newSwiftParser()
	defer parser.Close()

	tree := parser.Parse(content, nil)
	if tree == nil {
		return nil, fmt.Errorf("tree-sitter failed to parse Swift file %s", filePath)
	}
	defer tree.Close()

	ex := &swiftExtractor{
		filePath: filePath,
		src:      content,
		lines:    strings.Split(string(content), "\n"),
	}
	ex.walk(tree.RootNode())
	return ex.candidates, nil
}

type swiftExtractor struct {
	filePath   string
	src        []byte
	lines      []string
	candidates []types.StringCandidate
}

func (ex *swiftExtractor) walk(n *sitter.Node) {
	if n == nil {
		return
	}
	if n.Kind() == "line_string_literal" {
		ex.maybeExtractStringLiteral(n)
	}
	for i := uint(0); i < n.NamedChildCount(); i++ {
		ex.walk(n.NamedChild(i))
	}
}

// maybeExtractStringLiteral inspects the call surrounding a line_string_literal
// to decide whether it is UI-facing text: a direct call to Text/Label/Button,
// or the argument of a .navigationTitle(...)-style navigation suffix call.
func (ex *swiftExtractor) maybeExtractStringLiteral(strNode *sitter.Node) {
	valueArg := strNode.Parent()
	if valueArg == nil || valueArg.Kind() != "value_argument" {
		return
	}
	// Skip labeled arguments (format:, comment:, etc.) — only bare positional
	// string arguments to a UI call are treated as user-facing text.
	for i := uint(0); i < valueArg.NamedChildCount(); i++ {
		if valueArg.NamedChild(i).Kind() == "value_argument_label" {
			return
		}
	}

	valueArgs := valueArg.Parent() // value_arguments
	if valueArgs == nil || valueArgs.Kind() != "value_arguments" {
		return
	}
	callSuffix := valueArgs.Parent() // call_suffix
	if callSuffix == nil || callSuffix.Kind() != "call_suffix" {
		return
	}
	callExpr := callSuffix.Parent() // call_expression
	if callExpr == nil || callExpr.Kind() != "call_expression" {
		return
	}

	calleeNode := callExpr.NamedChild(0)
	if calleeNode == nil {
		return
	}

	var calleeName string
	switch calleeNode.Kind() {
	case "simple_identifier":
		calleeName = calleeNode.Utf8Text(ex.src)
		if !swiftUICallees[calleeName] {
			return
		}
	case "navigation_expression":
		// e.g. `.navigationTitle("Profile")` reached as base.navigationTitle(...)
		suffix := calleeNode.ChildByFieldName("suffix")
		if suffix == nil {
			// fall back: last named child is the navigation_suffix
			if calleeNode.NamedChildCount() == 0 {
				return
			}
			suffix = calleeNode.NamedChild(calleeNode.NamedChildCount() - 1)
		}
		if suffix.Kind() != "navigation_suffix" {
			return
		}
		if suffix.NamedChildCount() == 0 {
			return
		}
		calleeName = suffix.NamedChild(0).Utf8Text(ex.src)
		if !swiftUINavigationSuffixes[calleeName] {
			return
		}
	default:
		return
	}

	icuText, varNames, ok := renderSwiftStringAsICU(strNode, ex.src)
	if !ok {
		return
	}
	cleanVal := strings.TrimSpace(icuText)
	if !isValidUIString(cleanVal) {
		return
	}

	startByte := int(strNode.StartByte())
	endByte := int(strNode.EndByte())
	startLine, startCol := getLineAndCol(ex.src, startByte)
	endLine, endCol := getLineAndCol(ex.src, endByte)

	ex.candidates = append(ex.candidates, types.StringCandidate{
		ID:             fmt.Sprintf("%s:%d:%d", filepath.Base(ex.filePath), startLine, startCol),
		FilePath:       ex.filePath,
		StartByte:      startByte,
		EndByte:        endByte,
		StartLine:      startLine,
		StartCol:       startCol,
		EndLine:        endLine,
		EndCol:         endCol,
		RawValue:       string(ex.src[startByte:endByte]),
		CleanValue:     cleanVal,
		Key:            ToCamelCase(stripICUTags(cleanVal)),
		ParentNodeType: fmt.Sprintf("SwiftUICall(%s)", calleeName),
		ContextHint:    getSurroundingContext(ex.lines, startLine),
		Variables:      varNames,
		Classification: types.ClassLocalizable,
		Confidence:     0.96,
		Approved:       true,
	})
}

// renderSwiftStringAsICU converts a Swift line_string_literal (with \(expr)
// interpolations) into ICU-style {var} placeholder text using the AST's own
// interpolated_expression boundaries.
func renderSwiftStringAsICU(strNode *sitter.Node, src []byte) (string, []string, bool) {
	var sb strings.Builder
	var varNames []string

	for i := uint(0); i < strNode.NamedChildCount(); i++ {
		child := strNode.NamedChild(i)
		switch child.Kind() {
		case "line_str_text":
			sb.WriteString(child.Utf8Text(src))
		case "interpolated_expression":
			inner := strings.TrimSpace(child.Utf8Text(src))
			inner = strings.TrimPrefix(inner, "\\(")
			inner = strings.TrimSuffix(inner, ")")
			varName := sanitizeVarName(inner, len(varNames))
			varNames = append(varNames, varName)
			sb.WriteString("{" + varName + "}")
		}
	}

	return sb.String(), varNames, true
}

func (p *SwiftPlatform) GenerateRefactorPlan(filePath string, content []byte, candidates []types.StringCandidate) (*types.FileRefactorPlan, error) {
	plan := &types.FileRefactorPlan{
		FilePath:        filePath,
		OriginalContent: string(content),
	}

	for _, c := range candidates {
		if !c.Approved || c.Classification != types.ClassLocalizable {
			continue
		}

		replacement := fmt.Sprintf("\"%s\"", c.Key)
		plan.Patches = append(plan.Patches, types.ByteRangePatch{
			FilePath:        filePath,
			StartByte:       c.StartByte,
			EndByte:         c.EndByte,
			ReplacementText: replacement,
			Description:     fmt.Sprintf("Replace string with LocalizedStringKey '%s'", c.Key),
		})
	}

	return plan, nil
}

func (p *SwiftPlatform) FormatLocaleFile(localeData types.LocaleData) ([]byte, error) {
	cat := map[string]any{
		"sourceLanguage": localeData.LocaleCode,
		"version":        "1.0",
		"strings":        make(map[string]any),
	}

	stringsMap := cat["strings"].(map[string]any)
	for k, v := range localeData.Entries {
		stringsMap[k] = map[string]any{
			"extractionState": "manual",
			"localizations": map[string]any{
				localeData.LocaleCode: map[string]any{
					"stringUnit": map[string]string{
						"state": "translated",
						"value": v,
					},
				},
			},
		}
	}

	return json.MarshalIndent(cat, "", "  ")
}

func (p *SwiftPlatform) ParseLocaleFile(raw []byte, format string) (*types.LocaleData, error) {
	var rawMap map[string]any
	if err := json.Unmarshal(raw, &rawMap); err != nil {
		return nil, err
	}

	entries := make(map[string]string)
	if stringsObj, ok := rawMap["strings"].(map[string]any); ok {
		for k, item := range stringsObj {
			if itemMap, ok := item.(map[string]any); ok {
				if locs, ok := itemMap["localizations"].(map[string]any); ok {
					for _, locItem := range locs {
						if locItemMap, ok := locItem.(map[string]any); ok {
							if su, ok := locItemMap["stringUnit"].(map[string]any); ok {
								if val, ok := su["value"].(string); ok {
									entries[k] = val
								}
							}
						}
					}
				}
			}
		}
	}

	return &types.LocaleData{
		Format:  "xcstrings",
		Entries: entries,
	}, nil
}

func findFilesWithExt(root, ext string) []string {
	var matches []string
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(path, ext) {
			matches = append(matches, path)
		}
		return nil
	})
	return matches
}
