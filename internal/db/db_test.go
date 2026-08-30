package db

import (
	"path/filepath"
	"testing"
)

func TestDB_MigrationsAndCRUD(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	// Create Team
	team, err := db.CreateTeam("Acme Engineering")
	if err != nil {
		t.Fatalf("CreateTeam: %v", err)
	}
	if team.ID == 0 {
		t.Errorf("expected non-zero team ID")
	}

	// Upsert GitHub Installation
	inst, err := db.UpsertInstallation(team.ID, 998877, "acme-corp")
	if err != nil {
		t.Fatalf("UpsertInstallation: %v", err)
	}

	// Upsert Repo
	repo, err := db.UpsertRepo(inst.ID, "acme-corp", "mobile-app", "main")
	if err != nil {
		t.Fatalf("UpsertRepo: %v", err)
	}

	// Upsert Repo Settings
	settings := &RepoSettings{
		RepoID:           repo.ID,
		Locales:          []string{"es", "fr", "de"},
		TonePreset:       "formal",
		Provider:         "openai",
		Model:            "gpt-4o-mini",
		SafetyMode:       true,
		ChunkWordBudget:  8000,
		ChunkKeyCeiling:  250,
		CustomInstallCmd: "pnpm install",
		CustomBuildCmd:   "pnpm typecheck",
		RootDir:          "apps/web",
	}
	if err := db.UpsertRepoSettings(settings); err != nil {
		t.Fatalf("UpsertRepoSettings: %v", err)
	}

	fetchedSettings, err := db.GetRepoSettings(repo.ID)
	if err != nil {
		t.Fatalf("GetRepoSettings: %v", err)
	}
	if fetchedSettings.TonePreset != "formal" || len(fetchedSettings.Locales) != 3 {
		t.Errorf("unexpected settings: %+v", fetchedSettings)
	}
	if fetchedSettings.CustomInstallCmd != "pnpm install" || fetchedSettings.CustomBuildCmd != "pnpm typecheck" || fetchedSettings.RootDir != "apps/web" {
		t.Errorf("expected custom commands and root dir to be persisted, got: %+v", fetchedSettings)
	}

	// Create Job
	job, err := db.CreateJob(repo.ID, "manual")
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if job.Status != "pending" {
		t.Errorf("job status = %q; want pending", job.Status)
	}

	// Atomically Claim Job
	claimed, err := db.ClaimNextPendingJob()
	if err != nil {
		t.Fatalf("ClaimNextPendingJob: %v", err)
	}
	if claimed == nil || claimed.ID != job.ID {
		t.Fatalf("claimed job mismatch: %+v", claimed)
	}
	if claimed.Status != "running" {
		t.Errorf("claimed job status = %q; want running", claimed.Status)
	}

	// Update Job Status
	err = db.UpdateJobStatus(job.ID, "succeeded", "langpeanut/i18n-123", "commit-sha-123", "settings-hash-123", "https://github.com/acme/pr/1", "")
	if err != nil {
		t.Fatalf("UpdateJobStatus: %v", err)
	}

	// Dedupe Check
	dup, err := db.HasDuplicateSuccessfulJob(repo.ID, "commit-sha-123", "settings-hash-123")
	if err != nil {
		t.Fatalf("HasDuplicateSuccessfulJob: %v", err)
	}
	if !dup {
		t.Errorf("expected duplicate check to return true")
	}
}
