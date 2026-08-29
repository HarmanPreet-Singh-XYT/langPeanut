package platforms

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
	dart "github.com/UserNobody14/tree-sitter-dart/bindings/go"

	"github.com/langPeanut/langPeanut/pkg/types"
)

// FlutterPlatform implements Platform for Flutter & Dart
type FlutterPlatform struct{}

func NewFlutterPlatform() *FlutterPlatform {
	return &FlutterPlatform{}
}

func (p *FlutterPlatform) Name() types.Framework {
	return types.FrameworkFlutter
}

func (p *FlutterPlatform) DisplayName() string {
	return "Flutter (Dart / ARB)"
}

func (p *FlutterPlatform) FileExtensions() []string {
	return []string{".dart", ".arb"}
}

func (p *FlutterPlatform) SkipDirs() []string {
	return []string{"build", ".dart_tool", "android", "ios", "web", "linux", "macos", "windows", ".git"}
}

func (p *FlutterPlatform) Detect(projectRoot string) (bool, float64) {
	if FileExists(projectRoot, "pubspec.yaml") {
		if FileContains(projectRoot, "pubspec.yaml", "flutter:") {
			return true, 1.0
		}
		return true, 0.70
	}
	if len(findFilesWithExt(projectRoot, ".dart")) > 0 || len(findFilesWithExt(projectRoot, ".arb")) > 0 {
		return true, 0.85
	}
	return false, 0
}

func (p *FlutterPlatform) DefaultLocaleDir(projectRoot string) string {
	if DirExists(projectRoot, "lib/l10n") {
		return "lib/l10n"
	}
	return "lib/l10n"
}

func (p *FlutterPlatform) DefaultSourceFile(projectRoot string, sourceLocale string) string {
	return filepath.Join(p.DefaultLocaleDir(projectRoot), fmt.Sprintf("app_%s.arb", sourceLocale))
}

// Widget/parameter names whose string argument is UI-facing text.
var dartUIWidgets = map[string]bool{
	"Text": true, "TextSpan": true, "Tooltip": true, "SnackBar": true,
	"AlertDialog": true, "SimpleDialog": true, "AppBar": true,
}

// Named arguments (regardless of callee) that carry UI-facing text.
var dartUINamedArgs = map[string]bool{
	"message": true, "labelText": true, "hintText": true, "errorText": true,
	"helperText": true, "title": true, "tooltip": true, "semanticsLabel": true,
	"confirmText": true, "cancelText": true, "headerText": true, "actionText": true,
}

// Callees whose string arguments must never be localized.
var dartSkipCallees = map[string]bool{
	"debugPrint": true, "print": true, "log": true, "Key": true,
	"ValueKey": true, "Uri": true,
}

func newDartParser() *sitter.Parser {
	parser := sitter.NewParser()
	parser.SetLanguage(sitter.NewLanguage(dart.Language()))
	return parser
}

func (p *FlutterPlatform) ExtractCandidates(filePath string, content []byte) ([]types.StringCandidate, error) {
	// Handle ARB files directly (no AST — ARB is JSON, not Dart)
	if strings.HasSuffix(filePath, ".arb") {
		return extractARBCandidates(filePath, content)
	}

	parser := newDartParser()
	defer parser.Close()

	tree := parser.Parse(content, nil)
	if tree == nil {
		return nil, fmt.Errorf("tree-sitter failed to parse Dart file %s", filePath)
	}
	defer tree.Close()

	ex := &dartExtractor{
		filePath: filePath,
		src:      content,
		lines:    strings.Split(string(content), "\n"),
	}
	ex.walk(tree.RootNode())
	return ex.candidates, nil
}

func extractARBCandidates(filePath string, content []byte) ([]types.StringCandidate, error) {
	var candidates []types.StringCandidate
	var rawMap map[string]any
	if err := json.Unmarshal(content, &rawMap); err != nil {
		return nil, err
	}
	for k, v := range rawMap {
		if !strings.HasPrefix(k, "@") && !strings.HasPrefix(k, "@@") {
			if strVal, ok := v.(string); ok {
				candidates = append(candidates, types.StringCandidate{
					ID:             fmt.Sprintf("%s:%s", filepath.Base(filePath), k),
					FilePath:       filePath,
					RawValue:       strVal,
					CleanValue:     strVal,
					Key:            k,
					ParentNodeType: "ARBEntry",
					Classification: types.ClassLocalizable,
					Confidence:     1.0,
					Approved:       true,
				})
			}
		}
	}
	return candidates, nil
}

type dartExtractor struct {
	filePath   string
	src        []byte
	lines      []string
	candidates []types.StringCandidate
}

func (ex *dartExtractor) walk(n *sitter.Node) {
	if n == nil {
		return
	}

	if n.Kind() == "string_literal" {
		ex.maybeExtractStringLiteral(n)
	}

	for i := uint(0); i < n.NamedChildCount(); i++ {
		ex.walk(n.NamedChild(i))
	}
}

// maybeExtractStringLiteral inspects a string_literal node's syntactic position
// (positional argument to a UI widget constructor, or a named UI argument such
// as message:/labelText:) to decide whether it is user-facing text.
func (ex *dartExtractor) maybeExtractStringLiteral(strNode *sitter.Node) {
	curr := strNode.Parent()
	if curr == nil {
		return
	}

	// Unwrap conditional_expression (ternary: cond ? 'A' : 'B') or parenthesized_expression
	for curr != nil && (curr.Kind() == "conditional_expression" || curr.Kind() == "parenthesized_expression") {
		curr = curr.Parent()
	}
	if curr == nil {
		return
	}

	argNode := curr
	var attrLabel string
	switch argNode.Kind() {
	case "argument":
		// positional argument: Text("...") or Text(isLoggedIn ? 'Welcome' : 'Sign In')
	case "named_argument":
		attrLabel = namedArgumentLabel(argNode, ex.src)
	default:
		return
	}

	argsNode := argNode.Parent() // arguments
	if argsNode == nil || argsNode.Kind() != "arguments" {
		return
	}

	calleeName, constTok := resolveCalleeAndConst(argsNode, ex.src)
	if calleeName == "" {
		return
	}
	if dartSkipCallees[calleeName] {
		return
	}

	// Accept if: positional arg to a known UI widget, OR named arg matches a UI attribute name.
	if attrLabel == "" {
		if !dartUIWidgets[calleeName] {
			return
		}
	} else if !dartUINamedArgs[attrLabel] {
		return
	}

	icuText, varNames, ok := renderStringLiteralAsICU(strNode, ex.src)
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

	nodeType := "WidgetCall"
	if attrLabel != "" {
		nodeType = fmt.Sprintf("NamedArg(%s)", attrLabel)
	}

	var constRange *[2]int
	if constTok != nil {
		constRange = &[2]int{constTok.StartByte, constTok.EndByte}
	}

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
		ParentNodeType: nodeType,
		ContextHint:    getSurroundingContext(ex.lines, startLine),
		Variables:      varNames,
		IsConstContext: constTok != nil,
		ConstByteRange: constRange,
		Classification: types.ClassLocalizable,
		Confidence:     0.98,
		Approved:       true,
	})
}

// namedArgumentLabel returns the "label:" identifier of a named_argument node.
// Grammar shape: named_argument -> label -> identifier, ":".
func namedArgumentLabel(namedArg *sitter.Node, src []byte) string {
	for i := uint(0); i < namedArg.NamedChildCount(); i++ {
		c := namedArg.NamedChild(i)
		if c.Kind() == "label" {
			if c.NamedChildCount() > 0 {
				return c.NamedChild(0).Utf8Text(src)
			}
			return strings.TrimSuffix(strings.TrimSpace(c.Utf8Text(src)), ":")
		}
	}
	return ""
}

// constToStrip identifies a `const` keyword token whose removal is required
// once the widget call it governs becomes non-constant (an AppLocalizations
// method call is never a compile-time constant in Dart).
type constToStrip struct {
	StartByte, EndByte int
}

// resolveCalleeAndConst finds the identifier being called (e.g. Text, Tooltip)
// and any `const` keyword — on the call itself, or on an enclosing const list
// literal that directly contains it — that must be stripped once the call's
// string argument is replaced with a non-const AppLocalizations lookup.
//
// Two grammar shapes reach a call's arguments:
//  1. `Tooltip(message: "...")` — expression_statement/named_argument value ->
//     identifier, selector -> argument_part -> arguments
//  2. `const Text("...")` — const_object_expression -> const_builtin, type_identifier, arguments
//
// A call can also appear as a bare element of a `const [ ... ]` list literal
// (Flutter's `children: const [Text(...), ...]` pattern). Dart requires every
// element of a const list to itself be a compile-time constant, so making one
// element non-const forces the list's own `const` to be removed too.
func resolveCalleeAndConst(argsNode *sitter.Node, src []byte) (callee string, constTok *constToStrip) {
	parent := argsNode.Parent()
	if parent == nil {
		return "", nil
	}

	switch parent.Kind() {
	case "const_object_expression":
		for i := uint(0); i < parent.NamedChildCount(); i++ {
			c := parent.NamedChild(i)
			if c.Kind() == "type_identifier" || c.Kind() == "identifier" {
				callee = c.Utf8Text(src)
				break
			}
		}
		for i := uint(0); i < parent.ChildCount(); i++ {
			if c := parent.Child(i); c.Kind() == "const_builtin" {
				constTok = &constToStrip{StartByte: int(c.StartByte()), EndByte: int(c.EndByte())}
				break
			}
		}
		return callee, constTok
	case "argument_part":
		selector := parent.Parent() // selector
		if selector != nil {
			caller := selector.PrevSibling()
			if caller != nil && (caller.Kind() == "identifier" || caller.Kind() == "type_identifier") {
				callee = caller.Utf8Text(src)
			}
		}
	default:
		return "", nil
	}

	// Not itself const — check whether it's a direct element of a const list
	// literal (`const [Text(...), ...]`), which also needs its const stripped.
	callExpr := parent.Parent() // expression_statement-equivalent wrapping identifier+selector
	if callExpr != nil {
		if listLit := callExpr.Parent(); listLit != nil && listLit.Kind() == "list_literal" {
			for i := uint(0); i < listLit.ChildCount(); i++ {
				if c := listLit.Child(i); c.Kind() == "const_builtin" {
					constTok = &constToStrip{StartByte: int(c.StartByte()), EndByte: int(c.EndByte())}
					break
				}
			}
		}
	}

	return callee, constTok
}

// renderStringLiteralAsICU converts a Dart string_literal (with $var / ${expr}
// interpolations) into ICU-style {var} placeholder text using the AST's own
// template_substitution boundaries rather than regex matching.
func renderStringLiteralAsICU(strNode *sitter.Node, src []byte) (string, []string, bool) {
	full := strNode.Utf8Text(src)
	if strNode.NamedChildCount() == 0 {
		// No interpolation — strip surrounding quotes.
		return unquoteDartString(full), nil, true
	}

	var sb strings.Builder
	var varNames []string
	cursor := int(strNode.StartByte())

	for i := uint(0); i < strNode.NamedChildCount(); i++ {
		child := strNode.NamedChild(i)
		if child.Kind() != "template_substitution" {
			continue
		}
		// Append literal text between cursor and this substitution, minus quote chars.
		betweenStart := cursor
		betweenEnd := int(child.StartByte())
		if betweenStart < betweenEnd {
			sb.WriteString(string(src[betweenStart:betweenEnd]))
		}

		inner := strings.TrimSpace(child.Utf8Text(src))
		inner = strings.TrimPrefix(inner, "${")
		inner = strings.TrimPrefix(inner, "$")
		inner = strings.TrimSuffix(inner, "}")
		varName := sanitizeVarName(inner, len(varNames))
		varNames = append(varNames, varName)
		sb.WriteString("{" + varName + "}")

		cursor = int(child.EndByte())
	}
	// Trailing literal text after the last substitution.
	sb.WriteString(string(src[cursor:strNode.EndByte()]))

	result := unquoteDartString(sb.String())
	return result, varNames, true
}

func unquoteDartString(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		first, last := s[0], s[len(s)-1]
		if (first == '\'' || first == '"') && first == last {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func (p *FlutterPlatform) GenerateRefactorPlan(filePath string, content []byte, candidates []types.StringCandidate) (*types.FileRefactorPlan, error) {
	if strings.HasSuffix(filePath, ".arb") {
		return &types.FileRefactorPlan{
			FilePath:          filePath,
			OriginalContent:   string(content),
			RefactoredContent: string(content),
		}, nil
	}

	plan := &types.FileRefactorPlan{
		FilePath:        filePath,
		OriginalContent: string(content),
		RequiredImports: []string{"import 'package:flutter_gen/gen_l10n/app_localizations.dart';"},
	}

	for _, c := range candidates {
		if !c.Approved || c.Classification != types.ClassLocalizable {
			continue
		}

		var replacement string
		if len(c.Variables) > 0 {
			varArgs := strings.Join(c.Variables, ", ")
			replacement = fmt.Sprintf("AppLocalizations.of(context)!.%s(%s)", c.Key, varArgs)
		} else {
			replacement = fmt.Sprintf("AppLocalizations.of(context)!.%s", c.Key)
		}

		targetStart := c.StartByte
		targetEnd := c.EndByte

		// If the AST identified a `const` keyword governing this call (on the
		// call itself, or on an enclosing const list literal), strip it —
		// otherwise the widget expression is no longer a valid compile-time
		// constant once it contains an AppLocalizations method call.
		if c.ConstByteRange != nil {
			constStart, constEnd := c.ConstByteRange[0], c.ConstByteRange[1]

			alreadyAdded := false
			for _, existing := range plan.Patches {
				if existing.StartByte == constStart && existing.EndByte == constEnd {
					alreadyAdded = true
					break
				}
			}
			if !alreadyAdded {
				plan.Patches = append(plan.Patches, types.ByteRangePatch{
					FilePath:        filePath,
					StartByte:       constStart,
					EndByte:         constEnd,
					ReplacementText: "",
					Description:     "Strip incompatible const keyword",
				})
			}
		}

		plan.Patches = append(plan.Patches, types.ByteRangePatch{
			FilePath:        filePath,
			StartByte:       targetStart,
			EndByte:         targetEnd,
			ReplacementText: replacement,
			Description:     fmt.Sprintf("Replace '%s' with AppLocalizations call", c.CleanValue),
		})
	}

	return plan, nil
}

func (p *FlutterPlatform) FormatLocaleFile(localeData types.LocaleData) ([]byte, error) {
	outputMap := make(map[string]any)

	// In ARB, @@locale specifies the locale code
	if localeData.LocaleCode != "" {
		outputMap["@@locale"] = localeData.LocaleCode
	}

	for k, v := range localeData.Entries {
		outputMap[k] = v
		if meta, ok := localeData.Metadata[k]; ok {
			outputMap["@"+k] = meta
		} else {
			// Auto generate @key metadata for placeholders if any
			if placeholders := extractICUPlaceholders(v); len(placeholders) > 0 {
				metaMap := map[string]any{
					"description":  fmt.Sprintf("Translation for %s", k),
					"placeholders": placeholders,
				}
				outputMap["@"+k] = metaMap
			}
		}
	}

	return json.MarshalIndent(outputMap, "", "  ")
}

func (p *FlutterPlatform) ParseLocaleFile(raw []byte, format string) (*types.LocaleData, error) {
	var rawMap map[string]any
	if err := json.Unmarshal(raw, &rawMap); err != nil {
		return nil, err
	}

	entries := make(map[string]string)
	metadata := make(map[string]any)
	localeCode := ""

	for k, v := range rawMap {
		if k == "@@locale" {
			if strVal, ok := v.(string); ok {
				localeCode = strVal
			}
		} else if strings.HasPrefix(k, "@") {
			metadata[strings.TrimPrefix(k, "@")] = v
		} else if strVal, ok := v.(string); ok {
			entries[k] = strVal
		}
	}

	return &types.LocaleData{
		LocaleCode: localeCode,
		Format:     "arb",
		Entries:    entries,
		Metadata:   metadata,
	}, nil
}

func extractICUPlaceholders(s string) map[string]any {
	reg := regexp.MustCompile(`\{([a-zA-Z0-9_]+)\}`)
	matches := reg.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return nil
	}

	placeholders := make(map[string]any)
	for _, m := range matches {
		if len(m) > 1 {
			placeholders[m[1]] = map[string]string{
				"type": "String",
			}
		}
	}
	return placeholders
}
