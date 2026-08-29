package platforms

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/langPeanut/langPeanut/pkg/types"
)

// Platform defines the interface every framework plugin implements
type Platform interface {
	Name() types.Framework
	DisplayName() string
	FileExtensions() []string
	SkipDirs() []string
	Detect(projectRoot string) (bool, float64)
	ExtractCandidates(filePath string, content []byte) ([]types.StringCandidate, error)
	GenerateRefactorPlan(filePath string, content []byte, candidates []types.StringCandidate) (*types.FileRefactorPlan, error)
	DefaultLocaleDir(projectRoot string) string
	DefaultSourceFile(projectRoot string, sourceLocale string) string
	FormatLocaleFile(localeData types.LocaleData) ([]byte, error)
	ParseLocaleFile(raw []byte, format string) (*types.LocaleData, error)
}

// Registry manages all registered platforms
type Registry struct {
	platforms map[types.Framework]Platform
}

// NewRegistry creates a new platform registry with built-in platform plugins
func NewRegistry() *Registry {
	r := &Registry{
		platforms: make(map[types.Framework]Platform),
	}
	r.Register(NewReactPlatform())
	r.Register(NewFlutterPlatform())
	r.Register(NewSwiftPlatform())
	r.Register(NewAndroidPlatform())
	r.Register(NewGenericPlatform())
	return r
}

// Register registers a platform plugin
func (r *Registry) Register(p Platform) {
	r.platforms[p.Name()] = p
}

// Get retrieves a platform by framework type
func (r *Registry) Get(name types.Framework) (Platform, error) {
	p, ok := r.platforms[name]
	if !ok {
		return nil, fmt.Errorf("platform %s not supported", name)
	}
	return p, nil
}

// AutoDetect attempts to detect the project platform from files in the root
func (r *Registry) AutoDetect(projectRoot string) (Platform, float64) {
	var bestPlatform Platform
	var bestScore float64

	for _, p := range r.platforms {
		detected, score := p.Detect(projectRoot)
		if detected && score > bestScore {
			bestScore = score
			bestPlatform = p
		}
	}

	if bestPlatform == nil {
		p, _ := r.Get(types.FrameworkGeneric)
		return p, 0.1
	}
	return bestPlatform, bestScore
}

// FileExists checks if a file exists in the directory
func FileExists(root, relPath string) bool {
	info, err := os.Stat(filepath.Join(root, relPath))
	return err == nil && !info.IsDir()
}

// DirExists checks if a directory exists
func DirExists(root, relPath string) bool {
	info, err := os.Stat(filepath.Join(root, relPath))
	return err == nil && info.IsDir()
}

// FileContains checks if a file contains a given substring
func FileContains(root, relPath, substring string) bool {
	b, err := os.ReadFile(filepath.Join(root, relPath))
	if err != nil {
		return false
	}
	return strings.Contains(string(b), substring)
}
