package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestWebStudio_APIRoutes(t *testing.T) {
	demoPath := filepath.Join("..", "..", "examples", "nextjs-app")
	studio := NewStudioServer(demoPath)

	// Test 1: GET /api/project
	req := httptest.NewRequest(http.MethodGet, "/api/project", nil)
	rec := httptest.NewRecorder()
	studio.handleGetProject(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200 for /api/project, got %d", rec.Code)
	}

	var projData map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&projData); err != nil {
		t.Fatalf("Failed to decode /api/project response: %v", err)
	}
	if projData["framework"] != "react" {
		t.Errorf("Expected framework 'react', got %v", projData["framework"])
	}

	// Test 2: GET /api/candidates
	req = httptest.NewRequest(http.MethodGet, "/api/candidates", nil)
	rec = httptest.NewRecorder()
	studio.handleGetCandidates(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200 for /api/candidates, got %d", rec.Code)
	}

	// Test 3: POST /api/candidates/batch (Approve All)
	batchPayload := map[string]string{"action": "approve_all"}
	batchBody, _ := json.Marshal(batchPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/candidates/batch", bytes.NewReader(batchBody))
	rec = httptest.NewRecorder()
	studio.handleBatchCandidates(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200 for /api/candidates/batch, got %d", rec.Code)
	}

	// Test 4: GET /api/stats
	req = httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	rec = httptest.NewRecorder()
	studio.handleGetStats(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200 for /api/stats, got %d", rec.Code)
	}

	// Test 5: GET /api/settings
	req = httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	rec = httptest.NewRecorder()
	studio.handleGetSettings(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200 for /api/settings, got %d", rec.Code)
	}

	// Test 6: GET /api/tree
	req = httptest.NewRequest(http.MethodGet, "/api/tree", nil)
	rec = httptest.NewRecorder()
	studio.handleGetTree(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200 for /api/tree, got %d", rec.Code)
	}

	// Test 7: GET /api/matrix
	req = httptest.NewRequest(http.MethodGet, "/api/matrix", nil)
	rec = httptest.NewRecorder()
	studio.handleGetMatrix(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200 for /api/matrix, got %d", rec.Code)
	}

	// Test 8: GET /api/git
	req = httptest.NewRequest(http.MethodGet, "/api/git", nil)
	rec = httptest.NewRecorder()
	studio.handleGetGitStatus(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200 for /api/git, got %d", rec.Code)
	}

	// Test 9: GET / (HTML app root)
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = httptest.NewRecorder()
	studio.handleHome(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200 for /, got %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("langPeanut — Localization Engineering Studio")) {
		t.Errorf("Expected HTML response to contain Studio title")
	}

	// Test 10: GET /api/chat/history & POST /api/chat/reset
	req = httptest.NewRequest(http.MethodGet, "/api/chat/history", nil)
	rec = httptest.NewRecorder()
	studio.handleChatHistory(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200 for /api/chat/history, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/chat/reset", nil)
	rec = httptest.NewRecorder()
	studio.handleChatReset(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200 for /api/chat/reset, got %d", rec.Code)
	}
}

func TestWebStudio_SEORoutes(t *testing.T) {
	demoPath := filepath.Join("..", "..", "examples", "nextjs-app")
	studio := NewStudioServer(demoPath)

	// 1. GET /api/seo initial overview
	req := httptest.NewRequest(http.MethodGet, "/api/seo", nil)
	rec := httptest.NewRecorder()
	studio.handleGetSEO(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 for /api/seo, got %d", rec.Code)
	}
	var seoData map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&seoData); err != nil {
		t.Fatalf("Failed to decode /api/seo: %v", err)
	}
	if seoData["configured"] != true {
		t.Errorf("Expected configured true, got %v", seoData["configured"])
	}
	if keyCount, ok := seoData["extracted_keys_count"].(float64); !ok || keyCount <= 0 {
		t.Errorf("Expected positive extracted_keys_count, got %v", seoData["extracted_keys_count"])
	}

	// 2. POST /api/seo/strategy
	stratPayload := map[string]any{
		"category":            "Developer Tool",
		"product_description": "Next.js translation platform",
		"goal":                "traffic",
		"scope_tier":          "high_impact",
		"target_locales":      []string{"en", "ja", "de"},
		"competitor_urls":     []string{"example.com"},
	}
	body, _ := json.Marshal(stratPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/seo/strategy", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	studio.handleSaveSEOStrategy(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 for /api/seo/strategy, got %d: %s", rec.Code, rec.Body.String())
	}

	// 3. POST /api/seo/analyze-domain
	req = httptest.NewRequest(http.MethodPost, "/api/seo/analyze-domain", nil)
	rec = httptest.NewRecorder()
	studio.handleAnalyzeSEODomain(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 for /api/seo/analyze-domain, got %d: %s", rec.Code, rec.Body.String())
	}
	var domainData map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&domainData); err != nil {
		t.Fatalf("Failed to decode domain data: %v", err)
	}
	if domainData["category"] == "" || domainData["category"] == "Software Platform" {
		t.Errorf("Expected specific category inferred from strings, got %v", domainData["category"])
	}

	// 4. POST /api/seo/scout (English)
	scoutPayload := map[string]string{"locale": "en"}
	scoutBody, _ := json.Marshal(scoutPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/seo/scout", bytes.NewReader(scoutBody))
	rec = httptest.NewRecorder()
	studio.handleRunSEOScout(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 for /api/seo/scout, got %d: %s", rec.Code, rec.Body.String())
	}

	// 5. POST /api/seo/optimize (English)
	optPayload := map[string]string{"locale": "en"}
	optBody, _ := json.Marshal(optPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/seo/optimize", bytes.NewReader(optBody))
	rec = httptest.NewRecorder()
	studio.handleRunSEOOptimize(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 for /api/seo/optimize, got %d: %s", rec.Code, rec.Body.String())
	}

	// 6. POST /api/seo/apply
	applyPayload := map[string]string{"locale": "en"}
	applyBody, _ := json.Marshal(applyPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/seo/apply", bytes.NewReader(applyBody))
	rec = httptest.NewRecorder()
	studio.handleApplySEO(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 for /api/seo/apply, got %d: %s", rec.Code, rec.Body.String())
	}
}
