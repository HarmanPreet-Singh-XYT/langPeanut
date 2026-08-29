package platforms

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/langPeanut/langPeanut/pkg/types"
)

// GenericPlatform serves as a fallback for standard JSON-based frameworks
type GenericPlatform struct{}

func NewGenericPlatform() *GenericPlatform {
	return &GenericPlatform{}
}

func (p *GenericPlatform) Name() types.Framework {
	return types.FrameworkGeneric
}

func (p *GenericPlatform) DisplayName() string {
	return "Generic / Web / JSON"
}

func (p *GenericPlatform) FileExtensions() []string {
	return []string{".js", ".ts", ".html", ".vue", ".py"}
}

func (p *GenericPlatform) SkipDirs() []string {
	return []string{"node_modules", ".git", "dist", "build", "vendor"}
}

func (p *GenericPlatform) Detect(projectRoot string) (bool, float64) {
	return true, 0.1
}

func (p *GenericPlatform) DefaultLocaleDir(projectRoot string) string {
	return "locales"
}

func (p *GenericPlatform) DefaultSourceFile(projectRoot string, sourceLocale string) string {
	return filepath.Join(p.DefaultLocaleDir(projectRoot), sourceLocale+".json")
}

var genericStringRegex = regexp.MustCompile(`["']([^"'\n]{3,80})["']`)

func (p *GenericPlatform) ExtractCandidates(filePath string, content []byte) ([]types.StringCandidate, error) {
	var candidates []types.StringCandidate
	src := string(content)
	lines := strings.Split(src, "\n")

	matches := genericStringRegex.FindAllStringSubmatchIndex(src, -1)
	for _, m := range matches {
		if len(m) >= 4 {
			valStart := m[2]
			valEnd := m[3]
			val := src[valStart:valEnd]
			cleanVal := strings.TrimSpace(val)

			if !isValidUIString(cleanVal) {
				continue
			}

			startLine, startCol := getLineAndCol(content, valStart)
			endLine, endCol := getLineAndCol(content, valEnd)

			candidates = append(candidates, types.StringCandidate{
				ID:             fmt.Sprintf("%s:%d:%d", filepath.Base(filePath), startLine, startCol),
				FilePath:       filePath,
				StartByte:      valStart - 1,
				EndByte:        valEnd + 1,
				StartLine:      startLine,
				StartCol:       startCol,
				EndLine:        endLine,
				EndCol:         endCol,
				RawValue:       src[valStart-1 : valEnd+1],
				CleanValue:     cleanVal,
				Key:            ToCamelCase(cleanVal),
				ParentNodeType: "Literal",
				ContextHint:    getSurroundingContext(lines, startLine),
				Classification: types.ClassUncertain,
				Confidence:     0.70,
				Approved:       false,
			})
		}
	}

	return candidates, nil
}

func (p *GenericPlatform) GenerateRefactorPlan(filePath string, content []byte, candidates []types.StringCandidate) (*types.FileRefactorPlan, error) {
	plan := &types.FileRefactorPlan{
		FilePath:        filePath,
		OriginalContent: string(content),
	}

	for _, c := range candidates {
		if !c.Approved || c.Classification != types.ClassLocalizable {
			continue
		}

		replacement := fmt.Sprintf("t('%s')", c.Key)
		plan.Patches = append(plan.Patches, types.ByteRangePatch{
			FilePath:        filePath,
			StartByte:       c.StartByte,
			EndByte:         c.EndByte,
			ReplacementText: replacement,
			Description:     fmt.Sprintf("Replace with t('%s')", c.Key),
		})
	}

	return plan, nil
}

func (p *GenericPlatform) FormatLocaleFile(localeData types.LocaleData) ([]byte, error) {
	return json.MarshalIndent(localeData.Entries, "", "  ")
}

func (p *GenericPlatform) ParseLocaleFile(raw []byte, format string) (*types.LocaleData, error) {
	var entries map[string]string
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, err
	}
	return &types.LocaleData{
		Format:  "json",
		Entries: entries,
	}, nil
}
