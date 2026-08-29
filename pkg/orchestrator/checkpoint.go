package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CheckpointManifest records metadata about a snapshot
type CheckpointManifest struct {
	ID                 string    `json:"id"`
	Stage              string    `json:"stage"`
	CreatedAt          time.Time `json:"created_at"`
	Summary            string    `json:"summary"`
	FilesRestoredCount int       `json:"files_restored_count"`
	Files              []string  `json:"files"`
}

// CheckpointManager manages pre-run snapshots and atomic rollbacks
type CheckpointManager struct {
	rootDir string
}

func NewCheckpointManager(projectRoot string) (*CheckpointManager, error) {
	ckptDir := filepath.Join(projectRoot, ".langPeanut", "checkpoints")
	if err := os.MkdirAll(ckptDir, 0755); err != nil {
		return nil, err
	}
	return &CheckpointManager{rootDir: ckptDir}, nil
}

// CreateCheckpoint takes a snapshot of the specified files
func (cm *CheckpointManager) CreateCheckpoint(stage, summary string, files []string) (*CheckpointManifest, error) {
	id := fmt.Sprintf("%s-%s", stage, time.Now().Format("20060102-150405"))
	ckptPath := filepath.Join(cm.rootDir, id)

	if err := os.MkdirAll(ckptPath, 0755); err != nil {
		return nil, err
	}

	manifest := &CheckpointManifest{
		ID:                 id,
		Stage:              stage,
		CreatedAt:          time.Now(),
		Summary:            summary,
		FilesRestoredCount: len(files),
		Files:              files,
	}

	// Copy each file into checkpoint dir
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		relPath := filepath.Base(file)
		dstPath := filepath.Join(ckptPath, relPath)
		_ = os.WriteFile(dstPath, data, 0644)
	}

	// Save manifest
	mData, _ := json.MarshalIndent(manifest, "", "  ")
	_ = os.WriteFile(filepath.Join(ckptPath, "manifest.json"), mData, 0644)

	return manifest, nil
}

// ListCheckpoints returns all available checkpoints
func (cm *CheckpointManager) ListCheckpoints() ([]CheckpointManifest, error) {
	entries, err := os.ReadDir(cm.rootDir)
	if err != nil {
		return nil, err
	}

	var manifests []CheckpointManifest
	for _, e := range entries {
		if e.IsDir() {
			mPath := filepath.Join(cm.rootDir, e.Name(), "manifest.json")
			mData, err := os.ReadFile(mPath)
			if err == nil {
				var m CheckpointManifest
				if json.Unmarshal(mData, &m) == nil {
					manifests = append(manifests, m)
				}
			}
		}
	}
	return manifests, nil
}

// RestoreCheckpoint reverts files to the state captured in the checkpoint
func (cm *CheckpointManager) RestoreCheckpoint(id string) error {
	ckptPath := filepath.Join(cm.rootDir, id)
	mPath := filepath.Join(ckptPath, "manifest.json")

	mData, err := os.ReadFile(mPath)
	if err != nil {
		return fmt.Errorf("checkpoint %s not found: %w", id, err)
	}

	var manifest CheckpointManifest
	if err := json.Unmarshal(mData, &manifest); err != nil {
		return err
	}

	for _, file := range manifest.Files {
		relPath := filepath.Base(file)
		backupPath := filepath.Join(ckptPath, relPath)
		data, err := os.ReadFile(backupPath)
		if err == nil {
			_ = os.WriteFile(file, data, 0644)
		}
	}

	return nil
}
