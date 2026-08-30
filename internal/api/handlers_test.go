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
