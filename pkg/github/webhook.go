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
	EventIssueComment      WebhookEventType = "issue_comment"
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

// IssueCommentEvent represents a PR or Issue comment event from GitHub webhooks
type IssueCommentEvent struct {
	Action string `json:"action"` // "created", "edited"
	Issue  struct {
		Number      int `json:"number"`
		PullRequest *struct {
			URL string `json:"url"`
		} `json:"pull_request,omitempty"`
	} `json:"issue"`
	Comment struct {
		ID   int64  `json:"id"`
		Body string `json:"body"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"comment"`
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

// BotCommand represents a parsed @langpeanut interactive command in PR comments
type BotCommand struct {
	Action     string   `json:"action"` // "translate", "audit", "review", "prune", "doctor", "directive", "help"
	Locales    []string `json:"locales,omitempty"`
	Tone       string   `json:"tone,omitempty"`
	Provider   string   `json:"provider,omitempty"`
	Directive  string   `json:"directive,omitempty"`
	RawCommand string   `json:"raw_command"`
}

// ParseBotCommand inspects a comment body for @langpeanut or /langpeanut commands or natural language directives
func ParseBotCommand(body string) (*BotCommand, bool) {
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		var prefix string
		if strings.HasPrefix(strings.ToLower(trimmed), "@langpeanut") {
			prefix = "@langpeanut"
		} else if strings.HasPrefix(strings.ToLower(trimmed), "/langpeanut") {
			prefix = "/langpeanut"
		} else {
			continue
		}

		cleanInstruction := strings.TrimSpace(trimmed[len(prefix):])
		if cleanInstruction == "" {
			return &BotCommand{Action: "translate", RawCommand: trimmed}, true
		}

		parts := strings.Fields(trimmed)
		firstWord := strings.ToLower(parts[1])

		cmd := &BotCommand{
			Action:     firstWord,
			Directive:  cleanInstruction,
			RawCommand: trimmed,
		}

		// 1. Structured CLI flag parsing: --locales/-l es,ja --tone/-t formal --provider/-p claude
		for i := 2; i < len(parts); i++ {
			p := parts[i]
			if (p == "--locales" || p == "-l") && i+1 < len(parts) {
				locs := strings.Split(parts[i+1], ",")
				for _, loc := range locs {
					if clean := strings.TrimSpace(loc); clean != "" {
						cmd.Locales = append(cmd.Locales, clean)
					}
				}
				i++
			} else if (p == "--tone" || p == "-t") && i+1 < len(parts) {
				cmd.Tone = strings.TrimSpace(parts[i+1])
				i++
			} else if (p == "--provider" || p == "-p") && i+1 < len(parts) {
				cmd.Provider = strings.TrimSpace(parts[i+1])
				i++
			}
		}

		// 2. Natural Language Conversational Extraction:
		lowerInstr := strings.ToLower(cleanInstruction)
		if cmd.Tone == "" {
			if strings.Contains(lowerInstr, "casual") || strings.Contains(lowerInstr, "friendly") {
				cmd.Tone = "casual"
			} else if strings.Contains(lowerInstr, "formal") || strings.Contains(lowerInstr, "corporate") {
				cmd.Tone = "corporate"
			} else if strings.Contains(lowerInstr, "pirate") {
				cmd.Tone = "pirate"
			} else if strings.Contains(lowerInstr, "genz") || strings.Contains(lowerInstr, "gen-z") {
				cmd.Tone = "genz"
			}
		}

		if len(cmd.Locales) == 0 {
			langMap := map[string]string{
				"spanish": "es", "french": "fr", "german": "de", "japanese": "ja",
				"italian": "it", "portuguese": "pt", "chinese": "zh", "korean": "ko",
			}
			for name, code := range langMap {
				if strings.Contains(lowerInstr, name) {
					cmd.Locales = append(cmd.Locales, code)
				}
			}
		}

		return cmd, true
	}

	return nil, false
}

// ParseWebhook identifies the event type from the X-GitHub-Event header and
// decodes the payload into the matching typed struct.
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
	case "issue_comment":
		var evt IssueCommentEvent
		if err := json.Unmarshal(payload, &evt); err != nil {
			return EventUnhandled, nil, fmt.Errorf("decode issue_comment event: %w", err)
		}
		return EventIssueComment, &evt, nil
	default:
		return EventUnhandled, nil, nil
	}
}

// IsDefaultBranchPush reports whether a push event landed on the repo's
// default branch.
func (e *PushEvent) IsDefaultBranchPush() bool {
	return e.Ref == "refs/heads/"+e.Repository.DefaultBranch
}

// IsPullRequestComment reports whether an issue_comment event was created on a Pull Request
func (e *IssueCommentEvent) IsPullRequestComment() bool {
	return e.Issue.PullRequest != nil && e.Action == "created"
}
