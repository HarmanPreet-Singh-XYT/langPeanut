package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/langPeanut/langpeanut-cloud/internal/auth"
	"github.com/langPeanut/langpeanut-cloud/internal/db"
)

func setupTestServer(t *testing.T) (*http.ServeMux, *db.DB, string) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	masterKey, _ := auth.GenerateMasterKey()
	mux := http.NewServeMux()
	h := &Handler{DB: database, MasterKey: masterKey, SessionSecret: masterKey}
	RegisterRoutes(mux, h)

	return mux, database, masterKey
}

// sessionCookie builds a valid signed session cookie for the given
// user/team, standing in for what handleGitHubOAuthCallback would set after
// a real GitHub login.
func sessionCookie(t *testing.T, sessionSecret string, userID, teamID int64) *http.Cookie {
	t.Helper()
	tok, err := auth.NewSessionToken(sessionSecret, userID, teamID)
	if err != nil {
		t.Fatalf("NewSessionToken: %v", err)
	}
	return &http.Cookie{Name: auth.SessionCookieName, Value: tok}
}

func TestAPI_Health(t *testing.T) {
	mux, database, _ := setupTestServer(t)
	defer database.Close()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d; want 200", w.Code)
	}
}

func TestAPI_RepoFlow(t *testing.T) {
	mux, database, sessionSecret := setupTestServer(t)
	defer database.Close()

	team, _ := database.CreateTeam("Team Alpha")
	inst, _ := database.UpsertInstallation(team.ID, 12345, "alpha-org")
	user, _ := database.UpsertUserByGithubID(team.ID, 999, "dev@example.com", "Dev", "alpha-dev", "")
	cookie := sessionCookie(t, sessionSecret, user.ID, team.ID)

	// 1. Upsert Repo
	repoPayload, _ := json.Marshal(map[string]any{
		"installation_id": inst.ID,
		"owner":           "alpha-org",
		"name":            "web-app",
		"default_branch":  "main",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/repos", bytes.NewReader(repoPayload))
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/repos status = %d, body = %s", w.Code, w.Body.String())
	}

	var repo db.Repo
	_ = json.NewDecoder(w.Body).Decode(&repo)

	// 2. Configure Settings
	settingsPayload, _ := json.Marshal(map[string]any{
		"locales":     []string{"ja", "ko"},
		"tone_preset": "casual",
		"provider":    "gemini",
		"model":       "gemini-3.5-flash",
	})
	req = httptest.NewRequest(http.MethodPut, "/api/repos/1/settings", bytes.NewReader(settingsPayload))
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT /api/repos/1/settings status = %d, body = %s", w.Code, w.Body.String())
	}

	// 3. Set API Key Credential for Gemini
	credPayload, _ := json.Marshal(map[string]any{
		"api_key": "test-gemini-api-key",
	})
	req = httptest.NewRequest(http.MethodPut, "/api/credentials/gemini", bytes.NewReader(credPayload))
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT /api/credentials/gemini status = %d, body = %s", w.Code, w.Body.String())
	}

	// 4. Trigger Job
	req = httptest.NewRequest(http.MethodPost, "/api/repos/1/jobs", nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("POST /api/repos/1/jobs status = %d, body = %s", w.Code, w.Body.String())
	}

	// 5. Reset Repo Data
	req = httptest.NewRequest(http.MethodPost, "/api/repos/1/reset", nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/repos/1/reset status = %d, body = %s", w.Code, w.Body.String())
	}

	// 6. Delete Repo Completely
	req = httptest.NewRequest(http.MethodDelete, "/api/repos/1", nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("DELETE /api/repos/1 status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestAPI_GenkitEndpoints(t *testing.T) {
	mux, database, sessionSecret := setupTestServer(t)
	defer database.Close()

	team, _ := database.CreateTeam("Team Genkit")
	inst, _ := database.UpsertInstallation(team.ID, 12345, "genkit-org")
	repo, _ := database.UpsertRepo(inst.ID, "genkit-org", "cloud-app", "main")
	user, _ := database.UpsertUserByGithubID(team.ID, 888, "genkit@example.com", "Genkit User", "gk-dev", "")
	cookie := sessionCookie(t, sessionSecret, user.ID, team.ID)

	// 1. GET /api/repos/{repoID}/genkit/runtime
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/repos/%d/genkit/runtime", repo.ID), nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /genkit/runtime status = %d; body = %s", w.Code, w.Body.String())
	}

	var runtimeInfo map[string]any
	_ = json.NewDecoder(w.Body).Decode(&runtimeInfo)
	if runtimeInfo["framework"] != "Google Genkit Go" {
		t.Errorf("expected framework 'Google Genkit Go', got %v", runtimeInfo["framework"])
	}

	// 2. GET /api/repos/{repoID}/genkit/flows
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/repos/%d/genkit/flows", repo.ID), nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /genkit/flows status = %d; body = %s", w.Code, w.Body.String())
	}

	// 3. POST /api/repos/{repoID}/genkit/flow/verifyTranslationsFlow
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/repos/%d/genkit/flow/verifyTranslationsFlow", repo.ID), bytes.NewReader([]byte(`{}`)))
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /genkit/flow/verifyTranslationsFlow status = %d; body = %s", w.Code, w.Body.String())
	}

	// 4. POST /api/repos/{repoID}/chat (Genkit SSE stream)
	chatPayload, _ := json.Marshal(map[string]any{
		"message": "Scan repository and show coverage matrix",
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/repos/%d/chat", repo.ID), bytes.NewReader(chatPayload))
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/repos/{repoID}/chat status = %d; body = %s", w.Code, w.Body.String())
	}

	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("expected text/event-stream content type, got %s", w.Header().Get("Content-Type"))
	}
	if w.Header().Get("X-Genkit-Framework") != "Google Genkit Go" {
		t.Errorf("expected X-Genkit-Framework 'Google Genkit Go', got %s", w.Header().Get("X-Genkit-Framework"))
	}
}

func TestAPI_RepoFlow_RejectsOtherTeam(t *testing.T) {
	mux, database, sessionSecret := setupTestServer(t)
	defer database.Close()

	teamA, _ := database.CreateTeam("Team A")
	instA, _ := database.UpsertInstallation(teamA.ID, 111, "org-a")
	repo, _ := database.UpsertRepo(instA.ID, "org-a", "web-app", "main")

	teamB, _ := database.CreateTeam("Team B")
	userB, _ := database.UpsertUserByGithubID(teamB.ID, 222, "b@example.com", "B", "b-dev", "")
	cookieB := sessionCookie(t, sessionSecret, userB.ID, teamB.ID)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/repos/%d/settings", repo.ID), nil)
	req.AddCookie(cookieB)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Fatalf("expected team B to be denied access to team A's repo, got 200: %s", w.Body.String())
	}
}

func TestAPI_WebhookBranchFilterAndSettings(t *testing.T) {
	mux, database, sessionSecret := setupTestServer(t)
	defer database.Close()

	team, _ := database.CreateTeam("Team Webhook")
	inst, _ := database.UpsertInstallation(team.ID, 12345, "webhook-org")
	repo, _ := database.UpsertRepo(inst.ID, "webhook-org", "app-webhook", "main")
	user, _ := database.UpsertUserByGithubID(team.ID, 555, "wh@example.com", "WH User", "wh-dev", "")
	cookie := sessionCookie(t, sessionSecret, user.ID, team.ID)

	// 1. Configure settings with branch filter = "default_branch" and push enabled
	pushEnabled := true
	settingsPayload, _ := json.Marshal(map[string]any{
		"locales":                      []string{"ja", "de"},
		"tone_preset":                  "formal",
		"provider":                     "gemini",
		"model":                        "gemini-3.5-flash",
		"webhook_push_enabled":         &pushEnabled,
		"webhook_branch_filter":        "default_branch",
		"webhook_custom_branch_prefix": "l10n/auto-",
	})
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/repos/%d/settings", repo.ID), bytes.NewReader(settingsPayload))
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT settings failed: %d, body: %s", w.Code, w.Body.String())
	}

	// 2. Verify GET settings returns webhook fields
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/repos/%d/settings", repo.ID), nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET settings failed: %d", w.Code)
	}
	var resMap map[string]any
	_ = json.NewDecoder(w.Body).Decode(&resMap)
	if resMap["webhook_push_enabled"] != true {
		t.Errorf("expected webhook_push_enabled=true, got %v", resMap["webhook_push_enabled"])
	}
	if resMap["webhook_branch_filter"] != "default_branch" {
		t.Errorf("expected webhook_branch_filter='default_branch', got %v", resMap["webhook_branch_filter"])
	}
	if resMap["webhook_custom_branch_prefix"] != "l10n/auto-" {
		t.Errorf("expected webhook_custom_branch_prefix='l10n/auto-', got %v", resMap["webhook_custom_branch_prefix"])
	}

	// 3. Test Simulation Push Endpoint: Matching Default Branch
	simPayload, _ := json.Marshal(map[string]any{
		"branch":  "main",
		"dry_run": true,
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/repos/%d/webhook/test-push", repo.ID), bytes.NewReader(simPayload))
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("test-push failed: %d, body: %s", w.Code, w.Body.String())
	}
	var simRes map[string]any
	_ = json.NewDecoder(w.Body).Decode(&simRes)
	if simRes["matched"] != true {
		t.Errorf("expected matched=true for default branch, got %v", simRes)
	}

	// 4. Test Simulation Push Endpoint: Non-matching feature branch
	simPayloadNonMatch, _ := json.Marshal(map[string]any{
		"branch":  "feature/unmatched",
		"dry_run": true,
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/repos/%d/webhook/test-push", repo.ID), bytes.NewReader(simPayloadNonMatch))
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	_ = json.NewDecoder(w.Body).Decode(&simRes)
	if simRes["matched"] == true {
		t.Errorf("expected matched=false for non-matching branch, got %v", simRes)
	}

	// 5. Test Bot command simulation
	botPayload, _ := json.Marshal(map[string]any{
		"command": "@langpeanut translate --locales es,fr --tone casual",
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/repos/%d/webhook/test-bot", repo.ID), bytes.NewReader(botPayload))
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("test-bot failed: %d", w.Code)
	}
	var botRes map[string]any
	_ = json.NewDecoder(w.Body).Decode(&botRes)
	if botRes["valid"] != true || botRes["action"] != "translate" {
		t.Errorf("expected valid=true, action='translate', got %v", botRes)
	}
}
