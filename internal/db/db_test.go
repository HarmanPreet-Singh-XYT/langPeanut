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
		Provider:         "gemini",
		Model:            "gemini-3.5-flash",
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

	// ── Test SEO & Growth Studio Queries ──────────────────────────────────────
	seoStrategy := &RepoSEOStrategy{
		RepoID:             repo.ID,
		ProjectName:        "Acme Mobile",
		Category:           "Mobile Productivity SaaS",
		ProductDescription: "Enterprise grade mobile collaboration tool",
		TargetLocales:      []string{"ja", "de"},
		Goal:               "traffic",
		ScopeTier:          "high_impact",
		CompetitorURLs:     []string{"https://competitor.jp"},
	}
	if err := db.UpsertSEOStrategy(seoStrategy); err != nil {
		t.Fatalf("UpsertSEOStrategy: %v", err)
	}

	fetchedStrategy, err := db.GetSEOStrategy(repo.ID)
	if err != nil || fetchedStrategy == nil {
		t.Fatalf("GetSEOStrategy: %v", err)
	}
	if fetchedStrategy.Category != "Mobile Productivity SaaS" || len(fetchedStrategy.TargetLocales) != 2 {
		t.Errorf("unexpected fetched strategy: %+v", fetchedStrategy)
	}

	// Upsert Competitors
	comps := []RepoSEOCompetitor{
		{
			RepoID:          repo.ID,
			Locale:          "ja",
			Domain:          "competitor.jp",
			Rank:            1,
			Title:           "Top Platform in Japan",
			MetaDescription: "Leading solution",
			Keywords:        []string{"請求書作成", "インボイス"},
		},
	}
	if err := db.UpsertSEOCompetitors(repo.ID, "ja", comps); err != nil {
		t.Fatalf("UpsertSEOCompetitors: %v", err)
	}
	fetchedComps, err := db.GetSEOCompetitors(repo.ID, "ja")
	if err != nil || len(fetchedComps) != 1 {
		t.Fatalf("GetSEOCompetitors: %v", err)
	}
	if fetchedComps[0].Domain != "competitor.jp" || len(fetchedComps[0].Keywords) != 2 {
		t.Errorf("unexpected fetched competitor: %+v", fetchedComps[0])
	}

	// Upsert Keywords
	kws := []RepoSEOKeyword{
		{
			RepoID:           repo.ID,
			Locale:           "ja",
			Keyword:          "無料請求書作成ソフト",
			Intent:           "commercial",
			VolumeTier:       "high",
			EstMonthlyVolume: 22000,
			Difficulty:       32,
			Relevance:        98,
			IsPrimary:        true,
		},
	}
	if err := db.UpsertSEOKeywords(repo.ID, "ja", kws); err != nil {
		t.Fatalf("UpsertSEOKeywords: %v", err)
	}
	fetchedKws, err := db.GetSEOKeywords(repo.ID, "ja")
	if err != nil || len(fetchedKws) != 1 {
		t.Fatalf("GetSEOKeywords: %v", err)
	}
	if fetchedKws[0].Keyword != "無料請求書作成ソフト" || !fetchedKws[0].IsPrimary {
		t.Errorf("unexpected fetched keywords: %+v", fetchedKws[0])
	}

	// Upsert Optimizations
	opts := []RepoSEOOptimization{
		{
			RepoID:               repo.ID,
			Locale:               "ja",
			TranslationKey:       "home.hero.title",
			SourceEn:             "The fastest invoice app",
			BaselineTranslation:  "最速の請求書アプリ",
			OptimizedTranslation: "無料請求書作成ソフト・最速の請求書アプリ",
			InjectedKeywords:     []string{"無料請求書作成ソフト"},
			Rationale:            "Injected primary keyword",
			ImpactTier:           "high",
			ICUVariablesMatched:  true,
		},
	}
	if err := db.UpsertSEOOptimizations(repo.ID, "ja", opts); err != nil {
		t.Fatalf("UpsertSEOOptimizations: %v", err)
	}
	fetchedOpts, err := db.GetSEOOptimizations(repo.ID, "ja")
	if err != nil || len(fetchedOpts) != 1 {
		t.Fatalf("GetSEOOptimizations: %v", err)
	}
	if fetchedOpts[0].OptimizedTranslation != "無料請求書作成ソフト・最速の請求書アプリ" {
		t.Errorf("unexpected fetched optimization: %+v", fetchedOpts[0])
	}

	// Upsert Metrics
	metrics := &RepoSEOMetrics{
		RepoID:                repo.ID,
		Locale:                "ja",
		SearchVolumeBaseline:  500,
		SearchVolumeOptimized: 22000,
		SearchVolumeUpliftPct: 4300.0,
		ProjectedCTRBaseline:  1.8,
		ProjectedCTROptimized: 4.6,
		ProjectedCTRUpliftPct: 155.5,
		AvgKeywordDifficulty:  32,
		ReadabilityScore:      94,
		LocalTrustScore:       92,
		KeywordDensityPct:     2.4,
		DensitySafe:           true,
		EstimatedRankingDays:  45,
	}
	if err := db.UpsertSEOMetrics(metrics); err != nil {
		t.Fatalf("UpsertSEOMetrics: %v", err)
	}
	fetchedMetrics, err := db.GetSEOMetrics(repo.ID, "ja")
	if err != nil || fetchedMetrics == nil {
		t.Fatalf("GetSEOMetrics: %v", err)
	}
	if fetchedMetrics.SearchVolumeOptimized != 22000 || !fetchedMetrics.DensitySafe {
		t.Errorf("unexpected fetched metrics: %+v", fetchedMetrics)
	}

	// Test ResetRepoData
	if err := db.ResetRepoData(repo.ID); err != nil {
		t.Fatalf("ResetRepoData failed: %v", err)
	}
	matrixAfterReset, err := db.GetTranslationMatrix(repo.ID)
	if err != nil || len(matrixAfterReset) != 0 {
		t.Fatalf("expected empty matrix after reset, got: %v", matrixAfterReset)
	}
	seoOptsAfterReset, err := db.GetSEOOptimizations(repo.ID, "ja")
	if err != nil || len(seoOptsAfterReset) != 0 {
		t.Fatalf("expected empty seo optimizations after reset, got: %v", seoOptsAfterReset)
	}

	// Test DeleteRepo
	if err := db.DeleteRepo(repo.ID); err != nil {
		t.Fatalf("DeleteRepo failed: %v", err)
	}
	deletedRepo, err := db.GetRepoByID(repo.ID)
	if err != nil || deletedRepo != nil {
		t.Fatalf("expected repo to be deleted, got: %v", deletedRepo)
	}
}

