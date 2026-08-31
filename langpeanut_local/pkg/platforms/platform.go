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
	// DiscoverExistingLocales scans the project for already-present locale
	// catalog files (e.g. a Flutter project with lib/l10n/app_{en,de,es}.arb
	// already populated) and returns a map of locale code -> absolute file
	// path. Used so a run against an existing i18n setup translates only the
	// gap between source and target keys instead of starting from scratch.
	DiscoverExistingLocales(projectRoot string) (map[string]string, error)
	// ParseLocaleFileForLocale extracts one specific locale's entries from a
	// locale catalog file. For per-locale-file formats (ARB, strings.xml,
	// i18next JSON) this is equivalent to ParseLocaleFile. For formats that
	// hold every locale in a single shared file (Swift's .xcstrings), this
	// extracts only the requested locale's translations rather than flattening
	// all locales into one map.
	ParseLocaleFileForLocale(raw []byte, locale string) (*types.LocaleData, error)
	// CheckDependencies inspects project files and manifests for required localization libraries/packages.
	CheckDependencies(projectRoot string) (*types.DependencyStatus, error)
	// EnsureDependencies checks and injects missing localization dependencies into project manifests
	// (package.json, pubspec.yaml, etc.), creates required configuration bootstrap files (i18n.ts, l10n.yaml),
	// and executes the package manager installer (npm/pnpm/yarn/bun/flutter) if autoInstall is true.
	EnsureDependencies(projectRoot string, autoInstall bool) (*types.DependencyStatus, error)
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

// readFileBytes is a small os.ReadFile wrapper shared by platform
// DiscoverExistingLocales/config-parsing implementations.
func readFileBytes(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// osReadDir and fileExistsAbs are thin os wrappers shared by platform
// DiscoverExistingLocales implementations that scan directories by absolute path.
func osReadDir(path string) ([]os.DirEntry, error) {
	return os.ReadDir(path)
}

func fileExistsAbs(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
