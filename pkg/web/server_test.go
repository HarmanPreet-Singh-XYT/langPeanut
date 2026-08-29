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
}
