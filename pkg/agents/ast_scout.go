package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/langPeanut/langPeanut/pkg/platforms"
	"github.com/langPeanut/langPeanut/pkg/types"
)

// ASTScoutAgent is responsible for scanning the codebase and extracting candidate strings
type ASTScoutAgent struct {
	Platform platforms.Platform
}

func NewASTScoutAgent(p platforms.Platform) *ASTScoutAgent {
	return &ASTScoutAgent{Platform: p}
}

type ScanReport struct {
	TotalFilesScanned int                     `json:"total_files_scanned"`
	TotalCandidates   int                     `json:"total_candidates"`
	LocalizableCount  int                     `json:"localizable_count"`
	SkipCount         int                     `json:"skip_count"`
	UncertainCount    int                     `json:"uncertain_count"`
	Candidates        []types.StringCandidate `json:"candidates"`
}

// ScanProject crawls the project directory and extracts all candidate strings
func (a *ASTScoutAgent) ScanProject(projectRoot string, specificFile string) (*ScanReport, error) {
	report := &ScanReport{}
	var filesToScan []string

	if specificFile != "" {
		fullPath := filepath.Join(projectRoot, specificFile)
		if _, err := os.Stat(fullPath); err == nil {
			filesToScan = append(filesToScan, fullPath)
		} else if _, err := os.Stat(specificFile); err == nil {
			filesToScan = append(filesToScan, specificFile)
		} else {
			return nil, fmt.Errorf("specified file not found: %s", specificFile)
		}
	} else {
		extMap := make(map[string]bool)
		for _, ext := range a.Platform.FileExtensions() {
			extMap[ext] = true
		}

		skipDirMap := make(map[string]bool)
		for _, d := range a.Platform.SkipDirs() {
			skipDirMap[d] = true
		}

		err := filepath.Walk(projectRoot, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				if skipDirMap[info.Name()] || strings.HasPrefix(info.Name(), ".") {
					return filepath.SkipDir
				}
				return nil
			}

			ext := filepath.Ext(path)
			if extMap[ext] {
				filesToScan = append(filesToScan, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	report.TotalFilesScanned = len(filesToScan)

	for _, file := range filesToScan {
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		candidates, err := a.Platform.ExtractCandidates(file, content)
		if err != nil {
			continue
		}

		for _, c := range candidates {
			report.TotalCandidates++
			switch c.Classification {
			case types.ClassLocalizable:
				report.LocalizableCount++
			case types.ClassSkip:
				report.SkipCount++
			case types.ClassUncertain:
				report.UncertainCount++
			}
			report.Candidates = append(report.Candidates, c)
		}
	}

	return report, nil
}
