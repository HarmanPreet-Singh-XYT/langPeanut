package platforms

import (
	"encoding/xml"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"

	treesitterkotlin "github.com/langPeanut/langPeanut/pkg/platforms/thirdparty/treesitterkotlin"
	"github.com/langPeanut/langPeanut/pkg/types"
)

// AndroidPlatform implements Platform for Android & Jetpack Compose
type AndroidPlatform struct{}

func NewAndroidPlatform() *AndroidPlatform {
	return &AndroidPlatform{}
}

func (p *AndroidPlatform) Name() types.Framework {
	return types.FrameworkAndroid
}

func (p *AndroidPlatform) DisplayName() string {
	return "Android / Kotlin (Jetpack Compose / strings.xml)"
}

func (p *AndroidPlatform) FileExtensions() []string {
	return []string{".kt", ".java"}
}

func (p *AndroidPlatform) SkipDirs() []string {
	return []string{"build", ".gradle", ".git"}
}

func (p *AndroidPlatform) Detect(projectRoot string) (bool, float64) {
	if FileExists(projectRoot, "build.gradle") || FileExists(projectRoot, "build.gradle.kts") {
		return true, 0.95
	}
	if FileExists(projectRoot, "AndroidManifest.xml") || FileExists(projectRoot, "app/src/main/AndroidManifest.xml") {
		return true, 0.98
	}
	return false, 0
}

func (p *AndroidPlatform) DefaultLocaleDir(projectRoot string) string {
	return "app/src/main/res"
}

func (p *AndroidPlatform) DefaultSourceFile(projectRoot string, sourceLocale string) string {
	if sourceLocale == "en" {
		return filepath.Join(p.DefaultLocaleDir(projectRoot), "values/strings.xml")
	}
	return filepath.Join(p.DefaultLocaleDir(projectRoot), fmt.Sprintf("values-%s/strings.xml", sourceLocale))
}

// Composable function names whose string argument is UI-facing text.
var kotlinComposeCallees = map[string]bool{
	"Text": true, "Button": true, "OutlinedTextField": true, "Tooltip": true,
}

// Named arguments (text = "...", label = "...") that carry UI-facing text
// regardless of which composable they appear on.
var kotlinUINamedArgs = map[string]bool{
	"text": true, "label": true, "placeholder": true, "hint": true,
}

func newKotlinParser() *sitter.Parser {
	parser := sitter.NewParser()
	parser.SetLanguage(sitter.NewLanguage(treesitterkotlin.Language()))
	return parser
}

func (p *AndroidPlatform) ExtractCandidates(filePath string, content []byte) ([]types.StringCandidate, error) {
	if strings.HasSuffix(filePath, ".java") {
		// Java Compose interop is out of scope for AST extraction; Kotlin is
		// the primary Jetpack Compose language and covered below.
		return nil, nil
	}

	parser := newKotlinParser()
	defer parser.Close()

	tree := parser.Parse(content, nil)
	if tree == nil {
		return nil, fmt.Errorf("tree-sitter failed to parse Kotlin file %s", filePath)
	}
	defer tree.Close()

	ex := &kotlinExtractor{
		filePath: filePath,
		src:      content,
		lines:    strings.Split(string(content), "\n"),
	}
	ex.walk(tree.RootNode())
	return ex.candidates, nil
}

type kotlinExtractor struct {
	filePath   string
	src        []byte
	lines      []string
	candidates []types.StringCandidate
}

func (ex *kotlinExtractor) walk(n *sitter.Node) {
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

// maybeExtractStringLiteral inspects a string_literal's syntactic position —
// positional argument to a known Compose callee, or a named UI argument
// (text =, label =, ...) — to decide whether it is user-facing text.
func (ex *kotlinExtractor) maybeExtractStringLiteral(strNode *sitter.Node) {
	valueArg := strNode.Parent()
	if valueArg == nil || valueArg.Kind() != "value_argument" {
		return
	}

	var attrLabel string
	if valueArg.NamedChildCount() >= 2 {
		// named_argument shape: simple_identifier "=" string_literal
		first := valueArg.NamedChild(0)
		if first.Kind() == "simple_identifier" {
			attrLabel = first.Utf8Text(ex.src)
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
	if calleeNode == nil || calleeNode.Kind() != "simple_identifier" {
		return
	}
	calleeName := calleeNode.Utf8Text(ex.src)

	if attrLabel == "" {
		if !kotlinComposeCallees[calleeName] {
			return
		}
	} else if !kotlinUINamedArgs[attrLabel] {
		return
	}

	icuText, varNames, ok := renderKotlinStringAsICU(strNode, ex.src)
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

	nodeType := fmt.Sprintf("ComposeCall(%s)", calleeName)
	if attrLabel != "" {
		nodeType = fmt.Sprintf("NamedArg(%s)", attrLabel)
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
		Key:            ToSnakeCase(stripICUTags(cleanVal)),
		ParentNodeType: nodeType,
		ContextHint:    getSurroundingContext(ex.lines, startLine),
		Variables:      varNames,
		Classification: types.ClassLocalizable,
		Confidence:     0.96,
		Approved:       true,
	})
}

// renderKotlinStringAsICU converts a Kotlin string_literal (with $var /
// ${expr} interpolations) into ICU-style {var} placeholder text using the
// AST's own interpolated_identifier/interpolated_expression boundaries.
func renderKotlinStringAsICU(strNode *sitter.Node, src []byte) (string, []string, bool) {
	full := strNode.Utf8Text(src)
	if strNode.NamedChildCount() == 0 {
		return unquoteKotlinString(full), nil, true
	}

	var sb strings.Builder
	var varNames []string
	cursor := int(strNode.StartByte())

	for i := uint(0); i < strNode.NamedChildCount(); i++ {
		child := strNode.NamedChild(i)
		if child.Kind() != "interpolated_identifier" && child.Kind() != "interpolated_expression" {
			continue
		}
		betweenStart, betweenEnd := cursor, int(child.StartByte())
		if betweenStart < betweenEnd {
			sb.WriteString(string(src[betweenStart:betweenEnd]))
		}

		varName := sanitizeVarName(child.Utf8Text(src), len(varNames))
		varNames = append(varNames, varName)
		sb.WriteString("{" + varName + "}")

		cursor = int(child.EndByte())
	}
	sb.WriteString(string(src[cursor:strNode.EndByte()]))

	return unquoteKotlinString(sb.String()), varNames, true
}

func unquoteKotlinString(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

func (p *AndroidPlatform) GenerateRefactorPlan(filePath string, content []byte, candidates []types.StringCandidate) (*types.FileRefactorPlan, error) {
	plan := &types.FileRefactorPlan{
		FilePath:        filePath,
		OriginalContent: string(content),
		RequiredImports: []string{"import androidx.compose.ui.res.stringResource"},
	}

	for _, c := range candidates {
		if !c.Approved || c.Classification != types.ClassLocalizable {
			continue
		}

		replacement := fmt.Sprintf("stringResource(R.string.%s)", c.Key)
		plan.Patches = append(plan.Patches, types.ByteRangePatch{
			FilePath:        filePath,
			StartByte:       c.StartByte,
			EndByte:         c.EndByte,
			ReplacementText: replacement,
			Description:     fmt.Sprintf("Replace string with stringResource(R.string.%s)", c.Key),
		})
	}

	return plan, nil
}

type XMLResources struct {
	XMLName xml.Name    `xml:"resources"`
	Strings []XMLString `xml:"string"`
}

type XMLString struct {
	Name  string `xml:"name,attr"`
	Value string `xml:",chardata"`
}

func (p *AndroidPlatform) FormatLocaleFile(localeData types.LocaleData) ([]byte, error) {
	res := XMLResources{}
	for k, v := range localeData.Entries {
		escapedVal := strings.ReplaceAll(v, "&", "&amp;")
		escapedVal = strings.ReplaceAll(escapedVal, "<", "&lt;")
		escapedVal = strings.ReplaceAll(escapedVal, ">", "&gt;")
		escapedVal = strings.ReplaceAll(escapedVal, "'", "\\'")

		res.Strings = append(res.Strings, XMLString{
			Name:  k,
			Value: escapedVal,
		})
	}

	out, err := xml.MarshalIndent(res, "", "    ")
	if err != nil {
		return nil, err
	}
	xmlHeader := []byte("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n")
	return append(xmlHeader, out...), nil
}

func (p *AndroidPlatform) ParseLocaleFile(raw []byte, format string) (*types.LocaleData, error) {
	var res XMLResources
	if err := xml.Unmarshal(raw, &res); err != nil {
		return nil, err
	}

	entries := make(map[string]string)
	for _, s := range res.Strings {
		val := strings.ReplaceAll(s.Value, "\\'", "'")
		entries[s.Name] = val
	}

	return &types.LocaleData{
		Format:  "strings_xml",
		Entries: entries,
	}, nil
}

func ToSnakeCase(input string) string {
	reg := regexp.MustCompile(`[^a-zA-Z0-9\s]+`)
	clean := reg.ReplaceAllString(input, " ")
	words := strings.Fields(clean)
	if len(words) == 0 {
		return "text_key"
	}
	if len(words) > 5 {
		words = words[:5]
	}
	for i, w := range words {
		words[i] = strings.ToLower(w)
	}
	return strings.Join(words, "_")
}
