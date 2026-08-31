package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/langPeanut/langPeanut/pkg/agents"
)

// PullRequest is the subset of GitHub's PR response the caller needs back —
// primarily the URL to surface in the web UI and persist on the job row.
type PullRequest struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
	State   string `json:"state"`
}

type createPullRequestBody struct {
	Title string `json:"title"`
	Head  string `json:"head"`
	Base  string `json:"base"`
	Body  string `json:"body"`
}

// CreatePullRequest opens a PR from headBranch into baseBranch with the given
// title/body. It does not apply labels or comments itself — those are
// separate calls (AddLabels, PostComment) so OpenLocalizationPR can sequence
// them and so each step's failure is independently reportable.
func CreatePullRequest(ctx httpContext, installationToken, owner, repo, headBranch, baseBranch, title, body string) (*PullRequest, error) {
	payload, err := json.Marshal(createPullRequestBody{
		Title: title,
		Head:  headBranch,
		Base:  baseBranch,
		Body:  body,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal pull request payload: %w", err)
	}

	url := fmt.Sprintf("%s/repos/%s/%s/pulls", apiBaseURL, owner, repo)
	req, err := newRequest(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+installationToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := doRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("create pull request: unexpected status %d: %s", resp.StatusCode, readBody(resp))
	}

	var pr PullRequest
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, fmt.Errorf("decode pull request response: %w", err)
	}
	return &pr, nil
}

// AddLabels applies labels to an existing PR (PRs are issues under the hood
// in GitHub's API, so this is the issues-labels endpoint).
func AddLabels(ctx httpContext, installationToken, owner, repo string, prNumber int, labels []string) error {
	if len(labels) == 0 {
		return nil
	}
	payload, err := json.Marshal(map[string][]string{"labels": labels})
	if err != nil {
		return fmt.Errorf("marshal labels payload: %w", err)
	}

	url := fmt.Sprintf("%s/repos/%s/%s/issues/%d/labels", apiBaseURL, owner, repo, prNumber)
	req, err := newRequest(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+installationToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := doRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("add labels: unexpected status %d: %s", resp.StatusCode, readBody(resp))
	}
	return nil
}

// PostComment posts an issue comment on the PR — used to surface the
// needs-manual-review detail as a standalone, easily-notified comment in
// addition to the body section BuildPullRequest already writes, since GitHub
// notifies watchers on new comments but not on the initial PR body.
func PostComment(ctx httpContext, installationToken, owner, repo string, prNumber int, commentBody string) error {
	payload, err := json.Marshal(map[string]string{"body": commentBody})
	if err != nil {
		return fmt.Errorf("marshal comment payload: %w", err)
	}

	url := fmt.Sprintf("%s/repos/%s/%s/issues/%d/comments", apiBaseURL, owner, repo, prNumber)
	req, err := newRequest(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+installationToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := doRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("post comment: unexpected status %d: %s", resp.StatusCode, readBody(resp))
	}
	return nil
}

// OpenLocalizationPR is the single entry point the job worker calls after a
// pipeline run: it formats the PR via the deterministic BuildPullRequest
// template, creates the PR, applies labels, and — only when the repair agent
// left unresolved errors — posts a standalone review-request comment so the
// failure surfaces as a notification, not just body text a reviewer has to
// scroll to. The PR is always created; a failure in the labels or comment
// step is returned as an error to the caller but never un-does the PR itself,
// since a mislabeled or uncommented PR is still strictly better than no PR.
func OpenLocalizationPR(ctx httpContext, installationToken, owner, repo, headBranch, baseBranch string, result *agents.PipelineResult, meta RunMetadata) (*PullRequest, error) {
	title, body, labels := BuildPullRequest(result, meta)
	if title == "" {
		return nil, fmt.Errorf("open localization pr: BuildPullRequest returned empty title (nil result?)")
	}

	pr, err := CreatePullRequest(ctx, installationToken, owner, repo, headBranch, baseBranch, title, body)
	if err != nil {
		return nil, fmt.Errorf("create pull request: %w", err)
	}

	if err := AddLabels(ctx, installationToken, owner, repo, pr.Number, labels); err != nil {
		return pr, fmt.Errorf("pull request #%d created but failed to add labels: %w", pr.Number, err)
	}

	if len(result.UnresolvedErrors) > 0 {
		comment := buildNeedsReviewComment(result)
		if err := PostComment(ctx, installationToken, owner, repo, pr.Number, comment); err != nil {
			return pr, fmt.Errorf("pull request #%d created but failed to post review comment: %w", pr.Number, err)
		}
	}

	return pr, nil
}

// buildNeedsReviewComment produces a short, deterministic standalone comment
// (distinct from the PR body's own review section) so watchers get a
// notification, not just body text they have to scroll to find.
func buildNeedsReviewComment(result *agents.PipelineResult) string {
	comment := "⚠️ **This localization PR was opened automatically, but the code-repair agent could not resolve every issue it introduced.** Manual review is needed before merging:\n\n"
	for _, d := range result.UnresolvedErrors {
		comment += fmt.Sprintf("- `%s:%d` — %s (%s)\n", d.FilePath, d.Line, d.Message, d.Source)
	}
	return comment
}
