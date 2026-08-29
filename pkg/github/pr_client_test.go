package github

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/langPeanut/langPeanut/pkg/agents"
	"github.com/langPeanut/langPeanut/pkg/types"
)

func TestCreatePullRequest_Success(t *testing.T) {
	var gotBody createPullRequestBody
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/repos/acme/widgets/pulls") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"number": 7, "html_url": "https://github.com/acme/widgets/pull/7", "state": "open"})
	}))
	defer srv.Close()
	restore := apiBaseURL
	setAPIBaseURLForTest(srv.URL)
	defer setAPIBaseURLForTest(restore)

	pr, err := CreatePullRequest(newTestCtx(), "tok", "acme", "widgets", "langpeanut/i18n-1", "main", "i18n: localize 3 strings", "body text")
	if err != nil {
		t.Fatalf("CreatePullRequest: %v", err)
	}
	if pr.Number != 7 || pr.HTMLURL != "https://github.com/acme/widgets/pull/7" {
		t.Errorf("unexpected PR result: %+v", pr)
	}
	if gotBody.Head != "langpeanut/i18n-1" || gotBody.Base != "main" {
		t.Errorf("unexpected request body: %+v", gotBody)
	}
}

func TestCreatePullRequest_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(`{"message":"Validation Failed"}`))
	}))
	defer srv.Close()
	restore := apiBaseURL
	setAPIBaseURLForTest(srv.URL)
	defer setAPIBaseURLForTest(restore)

	_, err := CreatePullRequest(newTestCtx(), "tok", "acme", "widgets", "branch", "main", "title", "body")
	if err == nil {
		t.Fatal("expected error for non-201 status")
	}
	if !strings.Contains(err.Error(), "422") {
		t.Errorf("expected error to mention status code, got: %v", err)
	}
}

func TestAddLabels_EmptyIsNoop(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	restore := apiBaseURL
	setAPIBaseURLForTest(srv.URL)
	defer setAPIBaseURLForTest(restore)

	if err := AddLabels(newTestCtx(), "tok", "acme", "widgets", 1, nil); err != nil {
		t.Fatalf("AddLabels with no labels should be a no-op, got err: %v", err)
	}
	if called {
		t.Error("expected no HTTP call when labels is empty")
	}
}

func TestAddLabels_Success(t *testing.T) {
	var gotLabels map[string][]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/issues/7/labels") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&gotLabels)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	restore := apiBaseURL
	setAPIBaseURLForTest(srv.URL)
	defer setAPIBaseURLForTest(restore)

	err := AddLabels(newTestCtx(), "tok", "acme", "widgets", 7, []string{LabelAutomation, LabelNeedsReview})
	if err != nil {
		t.Fatalf("AddLabels: %v", err)
	}
	if len(gotLabels["labels"]) != 2 {
		t.Errorf("expected 2 labels sent, got %v", gotLabels["labels"])
	}
}

func TestPostComment_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/issues/3/comments") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()
	restore := apiBaseURL
	setAPIBaseURLForTest(srv.URL)
	defer setAPIBaseURLForTest(restore)

	if err := PostComment(newTestCtx(), "tok", "acme", "widgets", 3, "needs review"); err != nil {
		t.Fatalf("PostComment: %v", err)
	}
}

func TestOpenLocalizationPR_CleanSuccess_NoCommentPosted(t *testing.T) {
	commentPosted := false
	var labelsApplied []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/pulls"):
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"number": 5, "html_url": "https://github.com/acme/widgets/pull/5", "state": "open"})
		case strings.HasSuffix(r.URL.Path, "/labels"):
			var body map[string][]string
			json.NewDecoder(r.Body).Decode(&body)
			labelsApplied = body["labels"]
			w.WriteHeader(http.StatusOK)
		case strings.HasSuffix(r.URL.Path, "/comments"):
			commentPosted = true
			w.WriteHeader(http.StatusCreated)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	restore := apiBaseURL
	setAPIBaseURLForTest(srv.URL)
	defer setAPIBaseURLForTest(restore)

	result := &agents.PipelineResult{
		ExtractedCandidates: 4,
		RefactoredFiles:     []string{"src/App.tsx"},
		VerificationReport:  &types.VerificationReport{Passed: true},
	}
	meta := RunMetadata{Locales: []string{"fr"}}

	pr, err := OpenLocalizationPR(newTestCtx(), "tok", "acme", "widgets", "langpeanut/i18n-1", "main", result, meta)
	if err != nil {
		t.Fatalf("OpenLocalizationPR: %v", err)
	}
	if pr.Number != 5 {
		t.Errorf("pr.Number = %d, want 5", pr.Number)
	}
	if commentPosted {
		t.Error("should not post a review comment on clean success")
	}
	if len(labelsApplied) != 1 || labelsApplied[0] != LabelAutomation {
		t.Errorf("labelsApplied = %v, want [%s]", labelsApplied, LabelAutomation)
	}
}

func TestOpenLocalizationPR_NeedsReview_PostsComment(t *testing.T) {
	commentPosted := false
	var labelsApplied []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/pulls"):
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"number": 9, "html_url": "https://github.com/acme/widgets/pull/9", "state": "open"})
		case strings.HasSuffix(r.URL.Path, "/labels"):
			var body map[string][]string
			json.NewDecoder(r.Body).Decode(&body)
			labelsApplied = body["labels"]
			w.WriteHeader(http.StatusOK)
		case strings.HasSuffix(r.URL.Path, "/comments"):
			commentPosted = true
			w.WriteHeader(http.StatusCreated)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	restore := apiBaseURL
	setAPIBaseURLForTest(srv.URL)
	defer setAPIBaseURLForTest(restore)

	result := &agents.PipelineResult{
		ExtractedCandidates: 4,
		RefactoredFiles:     []string{"src/App.tsx"},
		VerificationReport:  &types.VerificationReport{Passed: false, ErrorCount: 1},
		UnresolvedErrors: []types.CompilerDiagnostic{
			{FilePath: "src/App.tsx", Line: 10, Message: "boom", Source: "tsc"},
		},
	}
	meta := RunMetadata{Locales: []string{"fr"}}

	pr, err := OpenLocalizationPR(newTestCtx(), "tok", "acme", "widgets", "langpeanut/i18n-2", "main", result, meta)
	if err != nil {
		t.Fatalf("OpenLocalizationPR: %v", err)
	}
	if pr.Number != 9 {
		t.Errorf("pr.Number = %d, want 9", pr.Number)
	}
	if !commentPosted {
		t.Error("expected a review comment to be posted when UnresolvedErrors is non-empty")
	}
	if len(labelsApplied) != 2 || labelsApplied[1] != LabelNeedsReview {
		t.Errorf("labelsApplied = %v, want [%s %s]", labelsApplied, LabelAutomation, LabelNeedsReview)
	}
}

func TestOpenLocalizationPR_NilResult(t *testing.T) {
	_, err := OpenLocalizationPR(newTestCtx(), "tok", "acme", "widgets", "branch", "main", nil, RunMetadata{})
	if err == nil {
		t.Fatal("expected error for nil PipelineResult")
	}
}
