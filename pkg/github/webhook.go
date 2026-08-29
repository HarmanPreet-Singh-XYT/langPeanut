package github

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// VerifySignature checks a GitHub webhook payload against the
// X-Hub-Signature-256 header value using the App's configured webhook
// secret. GitHub signs the raw request body with HMAC-SHA256; this must run
// against the body bytes exactly as received; before any JSON decoding.
func VerifySignature(payload []byte, signatureHeader, secret string) bool {
	const prefix = "sha256="
	if !strings.HasPrefix(signatureHeader, prefix) {
		return false
	}
	expectedHex := strings.TrimPrefix(signatureHeader, prefix)
	expectedSig, err := hex.DecodeString(expectedHex)
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	computedSig := mac.Sum(nil)

	return hmac.Equal(expectedSig, computedSig)
}

// WebhookEventType identifies the GitHub webhook events this service acts on.
type WebhookEventType string

const (
	EventPush              WebhookEventType = "push"
	EventInstallation      WebhookEventType = "installation"
	EventInstallationRepos WebhookEventType = "installation_repositories"
	EventUnhandled         WebhookEventType = "unhandled"
)

// PushEvent is the subset of GitHub's push webhook payload the job trigger
// needs: which repo, which branch (ref), and the resulting commit SHA — the
// same head_commit_sha used for the dedupe check (cloud_plan.md §6.2).
type PushEvent struct {
	Ref        string `json:"ref"`
	After      string `json:"after"` // new HEAD commit SHA after the push
	Repository struct {
		Name          string `json:"name"`
		FullName      string `json:"full_name"`
		DefaultBranch string `json:"default_branch"`
		Owner         struct {
			Login string `json:"login"`
		} `json:"owner"`
	} `json:"repository"`
	Installation struct {
		ID int64 `json:"id"`
	} `json:"installation"`
}

// InstallationEvent covers both the "installation" event (App installed/
// uninstalled/repos added at install time) and "installation_repositories"
// (repos added/removed from an existing installation).
type InstallationEvent struct {
	Action       string `json:"action"` // "created", "deleted", "added", "removed", ...
	Installation struct {
		ID      int64 `json:"id"`
		Account struct {
			Login string `json:"login"`
		} `json:"account"`
	} `json:"installation"`
}

// ParseWebhook identifies the event type from the X-GitHub-Event header and
// decodes the payload into the matching typed struct. The caller (the
// langpeanut-cloud API handler) is expected to call VerifySignature first —
// this function does no signature checking itself, keeping the two concerns
// (authenticity vs. parsing) independently testable.
func ParseWebhook(eventHeader string, payload []byte) (WebhookEventType, any, error) {
	switch eventHeader {
	case "push":
		var evt PushEvent
		if err := json.Unmarshal(payload, &evt); err != nil {
			return EventUnhandled, nil, fmt.Errorf("decode push event: %w", err)
		}
		return EventPush, &evt, nil
	case "installation":
		var evt InstallationEvent
		if err := json.Unmarshal(payload, &evt); err != nil {
			return EventUnhandled, nil, fmt.Errorf("decode installation event: %w", err)
		}
		return EventInstallation, &evt, nil
	case "installation_repositories":
		var evt InstallationEvent
		if err := json.Unmarshal(payload, &evt); err != nil {
			return EventUnhandled, nil, fmt.Errorf("decode installation_repositories event: %w", err)
		}
		return EventInstallationRepos, &evt, nil
	default:
		return EventUnhandled, nil, nil
	}
}

// IsDefaultBranchPush reports whether a push event landed on the repo's
// default branch — the only push events worth triggering an automatic
// localization job for for now (per cloud_plan.md §9's still-open question on
// trigger model; feature-branch pushes are ignored until that's decided).
func (e *PushEvent) IsDefaultBranchPush() bool {
	return e.Ref == "refs/heads/"+e.Repository.DefaultBranch
}
