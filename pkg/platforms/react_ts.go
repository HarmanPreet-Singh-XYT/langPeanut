package platforms

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
	tstypescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"

	"github.com/langPeanut/langPeanut/pkg/types"
)

// ReactPlatform implements Platform for React, Next.js, and React Native (TSX/JSX/TS/JS)
type ReactPlatform struct{}

func NewReactPlatform() *ReactPlatform {
	return &ReactPlatform{}
}

func (p *ReactPlatform) Name() types.Framework {
	return types.FrameworkReact
}

func (p *ReactPlatform) DisplayName() string {
	return "React / Next.js (TypeScript/JSX)"
}

func (p *ReactPlatform) FileExtensions() []string {
	return []string{".tsx", ".jsx", ".ts", ".js"}
}

func (p *ReactPlatform) SkipDirs() []string {
	return []string{"node_modules", ".next", "dist", "build", "coverage", ".git", "public"}
}

func (p *ReactPlatform) Detect(projectRoot string) (bool, float64) {
	hasPackage := FileExists(projectRoot, "package.json")
	if hasPackage {
		if FileContains(projectRoot, "package.json", "\"next\"") {
			return true, 0.98
		}
		if FileContains(projectRoot, "package.json", "\"react\"") {
			return true, 0.95
		}
		return true, 0.60
	}

	if len(findFilesWithExt(projectRoot, ".tsx")) > 0 || len(findFilesWithExt(projectRoot, ".jsx")) > 0 {
		return true, 0.85
	}
	return false, 0
}

func (p *ReactPlatform) DefaultLocaleDir(projectRoot string) string {
	if DirExists(projectRoot, "public/locales") {
		return "public/locales"
	}
	if DirExists(projectRoot, "src/locales") {
		return "src/locales"
	}
	if DirExists(projectRoot, "locales") {
		return "locales"
	}
	return "src/locales"
}

func (p *ReactPlatform) DefaultSourceFile(projectRoot string, sourceLocale string) string {
	return filepath.Join(p.DefaultLocaleDir(projectRoot), sourceLocale+".json")
}

var (
	// Non-UI JSX attributes to strictly ignore
	skipJSXAttributes = map[string]bool{
		"className": true, "class": true, "style": true, "key": true, "id": true,
		"type": true, "name": true, "src": true, "href": true, "target": true,
		"rel": true, "testID": true, "data-testid": true, "role": true,
		"method": true, "action": true, "color": true, "size": true,
		"width": true, "height": true, "viewBox": true, "fill": true, "stroke": true,
	}

	// UI JSX attributes to target
	uiJSXAttributes = map[string]bool{
		"placeholder": true, "title": true, "label": true, "alt": true,
		"aria-label": true, "helperText": true, "error": true, "tooltip": true,
		"header": true, "subtitle": true, "caption": true, "buttonText": true,
	}

	// Call expressions whose string/template-literal arguments are never UI-facing.
	skipCallCallees = map[string]bool{
		"console.log": true, "console.warn": true, "console.error": true, "console.debug": true,
		"console.info": true,
	}

	urlPattern    = regexp.MustCompile(`^https?://|^\/[a-zA-Z0-9_\-\/]+$|^\.\.?\/`)
	hexColorRegex = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)
)

// tsxParser lazily builds a tree-sitter parser configured for the TSX grammar.
// Tree-sitter parsers are cheap to construct but not goroutine-safe, so callers
// get a fresh one per file scan.
func newTSXParser() *sitter.Parser {
	parser := sitter.NewParser()
	parser.SetLanguage(sitter.NewLanguage(tstypescript.LanguageTSX()))
	return parser
}

// ExtractCandidates extracts translatable candidate string literals from TSX/JSX
// by walking the real tree-sitter AST rather than pattern-matching raw text.
func (p *ReactPlatform) ExtractCandidates(filePath string, content []byte) ([]types.StringCandidate, error) {
	parser := newTSXParser()
	defer parser.Close()

	tree := parser.Parse(content, nil)
	if tree == nil {
		return nil, fmt.Errorf("tree-sitter failed to parse %s", filePath)
	}
	defer tree.Close()
	root := tree.RootNode()

	src := content
	lines := strings.Split(string(content), "\n")

	ex := &reactExtractor{
		filePath: filePath,
		src:      src,
		lines:    lines,
	}
	ex.walk(root)
	return ex.candidates, nil
}

type reactExtractor struct {
	filePath   string
	src        []byte
	lines      []string
	candidates []types.StringCandidate
}

func (ex *reactExtractor) walk(n *sitter.Node) {
	if n == nil {
		return
	}

	switch n.Kind() {
	case "jsx_element", "jsx_fragment":
		ex.extractJSXChildren(n)
	case "jsx_attribute":
		ex.extractJSXAttribute(n)
	case "template_string":
		if ex.isInsideJSX(n) && !ex.isInsideSkippedCall(n) && !ex.isInsideJSXAttribute(n) {
			ex.extractTemplateString(n)
		}
	}

	for i := uint(0); i < n.NamedChildCount(); i++ {
		ex.walk(n.NamedChild(i))
	}
}

func (ex *reactExtractor) isInsideJSX(n *sitter.Node) bool {
	p := n.Parent()
	for p != nil {
		if p.Kind() == "jsx_element" || p.Kind() == "jsx_fragment" || p.Kind() == "jsx_attribute" || p.Kind() == "jsx_expression" {
			return true
		}
		p = p.Parent()
	}
	return false
}

// extractJSXChildren scans the direct children of a JSX element/fragment for
// jsx_text runs interleaved with {expression} interpolations, and merges
// adjacent text+expression+text sequences into a single ICU-style candidate
// (e.g. "Welcome back, {user.name}!").
func (ex *reactExtractor) extractJSXChildren(n *sitter.Node) {
	childCount := n.ChildCount()

	var run []jsxSegment
	flush := func() {
		if len(run) == 0 {
			return
		}
		ex.emitJSXRun(run2Candidates(run))
		run = nil
	}

	for i := uint(0); i < childCount; i++ {
		child := n.Child(i)
		switch child.Kind() {
		case "jsx_text":
			run = append(run, jsxSegment{isText: true, node: child, text: child.Utf8Text(ex.src)})
		case "html_character_reference":
			ent := child.Utf8Text(ex.src)
			decoded := decodeHTMLEntity(ent)
			run = append(run, jsxSegment{isText: true, node: child, text: decoded})
		case "ERROR":
			// Tree-sitter flags raw unescaped '&' in JSX as an ERROR node; treat its text as literal JSX text
			errText := child.Utf8Text(ex.src)
			run = append(run, jsxSegment{isText: true, node: child, text: errText})
		case "jsx_expression":
			// A simple {identifier} or {a.b.c} substitution can be inlined into
			// the surrounding text as an ICU placeholder. Anything more complex
			// (function calls, ternaries, JSX) breaks the run — it isn't safely
			// representable as a single translated string.
			inner := firstNamedChild(child)
			if inner != nil && (inner.Kind() == "identifier" || inner.Kind() == "member_expression") {
				run = append(run, jsxSegment{isText: false, node: child, text: inner.Utf8Text(ex.src)})
			} else {
				flush()
			}
		default:
			flush()
		}
	}
	flush()
}

func decodeHTMLEntity(ent string) string {
	switch ent {
	case "&apos;", "&#39;":
		return "'"
	case "&quot;", "&#34;":
		return "\""
	case "&amp;", "&#38;":
		return "&"
	case "&lt;", "&#60;":
		return "<"
	case "&gt;", "&#62;":
		return ">"
	case "&nbsp;", "&#160;":
		return " "
	default:
		return ent
	}
}

type jsxSegment struct {
	isText bool
	node   *sitter.Node
	text   string // for text segments: raw text; for expr segments: variable name
}

type jsxRunResult struct {
	rawStart, rawEnd int
	icuText          string
	varNames         []string
	hasContent       bool
}

func run2Candidates(run []jsxSegment) jsxRunResult {
	var res jsxRunResult
	if len(run) == 0 {
		return res
	}
	res.rawStart = int(run[0].node.StartByte())
	res.rawEnd = int(run[len(run)-1].node.EndByte())

	var sb strings.Builder
	for _, seg := range run {
		if seg.isText {
			sb.WriteString(seg.text)
		} else {
			varName := sanitizeVarName(seg.text, len(res.varNames))
			res.varNames = append(res.varNames, varName)
			sb.WriteString("{" + varName + "}")
		}
	}
	res.icuText = sb.String()
	res.hasContent = strings.TrimSpace(stripICUTags(res.icuText)) != ""
	return res
}

func (ex *reactExtractor) emitJSXRun(res jsxRunResult) {
	clean := strings.TrimSpace(res.icuText)
	if !res.hasContent || !isValidUIString(clean) {
		return
	}

	startLine, startCol := getLineAndCol(ex.src, res.rawStart)
	endLine, endCol := getLineAndCol(ex.src, res.rawEnd)
	rawValue := string(ex.src[res.rawStart:res.rawEnd])

	nodeType := "JSXElement"
	confidence := 0.98
	if len(res.varNames) > 0 {
		nodeType = "JSXInterpolatedText"
		confidence = 0.93
	}

	ex.candidates = append(ex.candidates, types.StringCandidate{
		ID:             fmt.Sprintf("%s:%d:%d", filepath.Base(ex.filePath), startLine, startCol),
		FilePath:       ex.filePath,
		StartByte:      res.rawStart,
		EndByte:        res.rawEnd,
		StartLine:      startLine,
		StartCol:       startCol,
		EndLine:        endLine,
		EndCol:         endCol,
		RawValue:       rawValue,
		CleanValue:     clean,
		Key:            ToCamelCase(stripICUTags(clean)),
		ParentNodeType: nodeType,
		ContextHint:    getSurroundingContext(ex.lines, startLine),
		Variables:      res.varNames,
		Classification: types.ClassLocalizable,
		Confidence:     confidence,
		Approved:       true,
	})
}

// extractJSXAttribute handles attribute="literal string" cases (placeholder, title, alt, ...).
func (ex *reactExtractor) extractJSXAttribute(n *sitter.Node) {
	nameNode := n.Child(0)
	if nameNode == nil {
		return
	}
	attrName := nameNode.Utf8Text(ex.src)
	if skipJSXAttributes[attrName] || !uiJSXAttributes[attrName] {
		return
	}

	// jsx_attribute -> property_identifier, "=", string | jsx_expression
	valueNode := n.NamedChild(1)
	if valueNode == nil || valueNode.Kind() != "string" {
		return
	}
	fragment := stringFragment(valueNode)
	if fragment == nil {
		return
	}

	cleanVal := strings.TrimSpace(fragment.Utf8Text(ex.src))
	if !isValidUIString(cleanVal) {
		return
	}

	startByte := int(valueNode.StartByte())
	endByte := int(valueNode.EndByte())
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
		Key:            ToCamelCase(attrName + " " + cleanVal),
		ParentNodeType: fmt.Sprintf("JSXAttribute(%s)", attrName),
		ContextHint:    getSurroundingContext(ex.lines, startLine),
		Classification: types.ClassLocalizable,
		Confidence:     0.95,
		Approved:       true,
	})
}

// extractTemplateString handles standalone `Welcome ${user.name}!` template literals
// that appear outside JSX (e.g. passed to a translated alert/toast call).
func (ex *reactExtractor) extractTemplateString(n *sitter.Node) {
	var sb strings.Builder
	var varNames []string

	for i := uint(0); i < n.NamedChildCount(); i++ {
		child := n.NamedChild(i)
		switch child.Kind() {
		case "string_fragment":
			sb.WriteString(child.Utf8Text(ex.src))
		case "template_substitution":
			inner := firstNamedChild(child)
			if inner == nil {
				return // unsupported substitution shape; skip this literal entirely
			}
			varName := sanitizeVarName(inner.Utf8Text(ex.src), len(varNames))
			varNames = append(varNames, varName)
			sb.WriteString("{" + varName + "}")
		}
	}

	icuString := sb.String()
	if !isValidUIString(icuString) {
		return
	}

	startByte := int(n.StartByte())
	endByte := int(n.EndByte())
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
		CleanValue:     icuString,
		Key:            ToCamelCase(stripICUTags(icuString)),
		ParentNodeType: "TemplateLiteral",
		ContextHint:    getSurroundingContext(ex.lines, startLine),
		Variables:      varNames,
		Classification: types.ClassLocalizable,
		Confidence:     0.90,
		Approved:       true,
	})
}

// isInsideSkippedCall walks up from a template_string/string node to see whether
// it is a direct argument of a call like console.log(...).
func (ex *reactExtractor) isInsideSkippedCall(n *sitter.Node) bool {
	args := n.Parent()
	if args == nil || args.Kind() != "arguments" {
		return false
	}
	call := args.Parent()
	if call == nil || call.Kind() != "call_expression" {
		return false
	}
	callee := call.ChildByFieldName("function")
	if callee == nil {
		return false
	}
	return skipCallCallees[callee.Utf8Text(ex.src)]
}

// isInsideJSXAttribute prevents double-extraction when a template string is used
// as an attribute value expression (handled separately, if at all).
func (ex *reactExtractor) isInsideJSXAttribute(n *sitter.Node) bool {
	p := n.Parent()
	for p != nil {
		if p.Kind() == "jsx_attribute" {
			return true
		}
		if p.Kind() == "jsx_element" || p.Kind() == "statement_block" {
			return false
		}
		p = p.Parent()
	}
	return false
}

func firstNamedChild(n *sitter.Node) *sitter.Node {
	if n.NamedChildCount() == 0 {
		return nil
	}
	return n.NamedChild(0)
}

func stringFragment(stringNode *sitter.Node) *sitter.Node {
	for i := uint(0); i < stringNode.NamedChildCount(); i++ {
		c := stringNode.NamedChild(i)
		if c.Kind() == "string_fragment" {
			return c
		}
	}
	return nil
}

// GenerateRefactorPlan produces the exact byte range patches and imports for TSX/React
func (p *ReactPlatform) GenerateRefactorPlan(filePath string, content []byte, candidates []types.StringCandidate) (*types.FileRefactorPlan, error) {
	plan := &types.FileRefactorPlan{
		FilePath:        filePath,
		OriginalContent: string(content),
		RequiredImports: []string{"import { useTranslation } from 'react-i18next';"},
		RequiredHooks:   []string{"const { t } = useTranslation();"},
	}

	for _, c := range candidates {
		if !c.Approved || c.Classification != types.ClassLocalizable {
			continue
		}

		var replacement string
		if strings.HasPrefix(c.ParentNodeType, "JSXAttribute") {
			if len(c.Variables) > 0 {
				varsMap := formatTSXVars(c.Variables)
				replacement = fmt.Sprintf("{t('%s', %s)}", c.Key, varsMap)
			} else {
				replacement = fmt.Sprintf("{t('%s')}", c.Key)
			}
		} else if c.ParentNodeType == "TemplateLiteral" {
			if len(c.Variables) > 0 {
				varsMap := formatTSXVars(c.Variables)
				replacement = fmt.Sprintf("t('%s', %s)", c.Key, varsMap)
			} else {
				replacement = fmt.Sprintf("t('%s')", c.Key)
			}
		} else { // JSXElement text / JSXInterpolatedText
			if len(c.Variables) > 0 {
				varsMap := formatTSXVars(c.Variables)
				replacement = fmt.Sprintf("{t('%s', %s)}", c.Key, varsMap)
			} else {
				replacement = fmt.Sprintf("{t('%s')}", c.Key)
			}
		}

		plan.Patches = append(plan.Patches, types.ByteRangePatch{
			FilePath:        filePath,
			StartByte:       c.StartByte,
			EndByte:         c.EndByte,
			ReplacementText: replacement,
			Description:     fmt.Sprintf("Replace '%s' with localization token '%s'", c.CleanValue, c.Key),
		})
	}

	// If patches were generated, ensure component body has the hook
	if len(plan.Patches) > 0 {
		plan.Patches = injectComponentHooks(content, plan.Patches)
	}

	return plan, nil
}

func injectComponentHooks(content []byte, patches []types.ByteRangePatch) []types.ByteRangePatch {
	srcStr := string(content)
	if strings.Contains(srcStr, "useTranslation()") || strings.Contains(srcStr, "useTranslation(") {
		return patches
	}

	parser := newTSXParser()
	defer parser.Close()
	tree := parser.Parse(content, nil)
	if tree == nil {
		return patches
	}
	defer tree.Close()

	root := tree.RootNode()
	var hookPatch *types.ByteRangePatch

	var findComponent func(n *sitter.Node)
	findComponent = func(n *sitter.Node) {
		if n == nil || hookPatch != nil {
			return
		}

		if n.Kind() == "function_declaration" || n.Kind() == "arrow_function" || n.Kind() == "function_expression" {
			body := n.ChildByFieldName("body")
			if body != nil && body.Kind() == "statement_block" {
				if hasJSX(body) {
					openBrace := body.Child(0)
					if openBrace != nil && openBrace.Utf8Text(content) == "{" {
						insertPos := int(openBrace.EndByte())
						hookPatch = &types.ByteRangePatch{
							StartByte:       insertPos,
							EndByte:         insertPos,
							ReplacementText: "\n  const { t } = useTranslation();",
							Description:     "Inject useTranslation hook",
						}
						return
					}
				}
			}
		}

		for i := uint(0); i < n.NamedChildCount(); i++ {
			findComponent(n.NamedChild(i))
		}
	}

	findComponent(root)
	if hookPatch != nil {
		patches = append(patches, *hookPatch)
	}

	return patches
}

func hasJSX(n *sitter.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind() == "jsx_element" || n.Kind() == "jsx_fragment" || n.Kind() == "jsx_self_closing_element" {
		return true
	}
	for i := uint(0); i < n.NamedChildCount(); i++ {
		if hasJSX(n.NamedChild(i)) {
			return true
		}
	}
	return false
}

func (p *ReactPlatform) FormatLocaleFile(localeData types.LocaleData) ([]byte, error) {
	return json.MarshalIndent(localeData.Entries, "", "  ")
}

func (p *ReactPlatform) ParseLocaleFile(raw []byte, format string) (*types.LocaleData, error) {
	var entries map[string]string
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, err
	}
	return &types.LocaleData{
		Format:  "json_i18next",
		Entries: entries,
	}, nil
}

// Helpers
func isValidUIString(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return false
	}

	// Must have at least one alphabetic letter
	stripped := stripICUTags(s)
	hasLetter := false
	for _, r := range stripped {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			hasLetter = true
			break
		}
	}
	if !hasLetter {
		return false
	}

	if urlPattern.MatchString(s) || hexColorRegex.MatchString(s) {
		return false
	}
	if strings.Contains(s, "http://") || strings.Contains(s, "https://") || strings.Contains(s, "mailto:") {
		return false
	}
	if strings.HasSuffix(s, ".png") || strings.HasSuffix(s, ".jpg") || strings.HasSuffix(s, ".svg") || strings.HasSuffix(s, ".webp") || strings.HasSuffix(s, ".ico") {
		return false
	}
	if strings.Contains(s, "/issues/") || strings.Contains(s, "/blob/") || strings.Contains(s, "/api/") {
		return false
	}
	// Skip SVG path syntax
	if strings.Contains(s, " L ") || strings.Contains(s, " Z") || strings.Contains(s, " M ") {
		return false
	}
	// Skip markdown templates
	if strings.HasPrefix(s, "### ") || strings.HasPrefix(s, "## ") || strings.HasPrefix(s, "# ") {
		return false
	}
	// Skip common code artifacts
	if strings.HasPrefix(s, "var(") || strings.HasPrefix(s, "calc(") || strings.HasPrefix(s, "rgba(") {
		return false
	}
	if strings.Contains(s, "=>") || strings.Contains(s, "function(") || strings.Contains(s, "SELECT ") {
		return false
	}
	return true
}

func getLineAndCol(content []byte, byteOffset int) (int, int) {
	line := 1
	col := 1
	for i := 0; i < byteOffset && i < len(content); i++ {
		if content[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return line, col
}

func getSurroundingContext(lines []string, targetLine int) string {
	start := max(0, targetLine-2)
	end := min(len(lines), targetLine+1)
	return strings.Join(lines[start:end], " | ")
}

func ToCamelCase(input string) string {
	// Strip special characters
	reg := regexp.MustCompile(`[^a-zA-Z0-9\s]+`)
	clean := reg.ReplaceAllString(input, " ")
	words := strings.Fields(clean)
	if len(words) == 0 {
		return "textKey"
	}

	// Limit to max 5 words for key
	if len(words) > 5 {
		words = words[:5]
	}

	var sb strings.Builder
	for i, w := range words {
		w = strings.ToLower(w)
		if i == 0 {
			sb.WriteString(w)
		} else {
			sb.WriteString(strings.Title(w))
		}
	}
	return sb.String()
}

func sanitizeVarName(expr string, idx int) string {
	parts := strings.Split(expr, ".")
	last := parts[len(parts)-1]
	reg := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	clean := reg.ReplaceAllString(last, "")
	if clean == "" {
		return fmt.Sprintf("var%d", idx+1)
	}
	return clean
}

func stripICUTags(s string) string {
	reg := regexp.MustCompile(`\{[^}]+\}`)
	return reg.ReplaceAllString(s, "")
}

func formatTSXVars(vars []string) string {
	var pairs []string
	for _, v := range vars {
		pairs = append(pairs, fmt.Sprintf("%s", v))
	}
	return fmt.Sprintf("{ %s }", strings.Join(pairs, ", "))
}
