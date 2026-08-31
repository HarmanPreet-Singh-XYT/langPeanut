package github

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func signPayload(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestVerifySignature_Valid(t *testing.T) {
	payload := []byte(`{"ref":"refs/heads/main"}`)
	secret := "top-secret"
	sig := signPayload(payload, secret)

	if !VerifySignature(payload, sig, secret) {
		t.Fatal("expected valid signature to verify")
	}
}

func TestVerifySignature_WrongSecret(t *testing.T) {
	payload := []byte(`{"ref":"refs/heads/main"}`)
	sig := signPayload(payload, "correct-secret")

	if VerifySignature(payload, sig, "wrong-secret") {
		t.Fatal("expected signature with wrong secret to fail verification")
	}
}

func TestVerifySignature_TamperedPayload(t *testing.T) {
	secret := "top-secret"
	sig := signPayload([]byte(`{"ref":"refs/heads/main"}`), secret)
	tampered := []byte(`{"ref":"refs/heads/evil"}`)

	if VerifySignature(tampered, sig, secret) {
		t.Fatal("expected tampered payload to fail verification")
	}
}

func TestVerifySignature_MissingPrefix(t *testing.T) {
	if VerifySignature([]byte("data"), "not-a-valid-header", "secret") {
		t.Fatal("expected malformed signature header to fail verification")
	}
}

func TestVerifySignature_InvalidHex(t *testing.T) {
	if VerifySignature([]byte("data"), "sha256=not-hex!!", "secret") {
		t.Fatal("expected non-hex signature to fail verification")
	}
}

func TestParseWebhook_Push(t *testing.T) {
	payload := []byte(`{
		"ref": "refs/heads/main",
		"after": "abc123",
		"repository": {"name": "widgets", "full_name": "acme/widgets", "default_branch": "main", "owner": {"login": "acme"}},
		"installation": {"id": 42}
	}`)

	eventType, decoded, err := ParseWebhook("push", payload)
	if err != nil {
		t.Fatalf("ParseWebhook: %v", err)
	}
	if eventType != EventPush {
		t.Fatalf("eventType = %v, want %v", eventType, EventPush)
	}
	evt, ok := decoded.(*PushEvent)
	if !ok {
		t.Fatalf("decoded type = %T, want *PushEvent", decoded)
	}
	if evt.After != "abc123" || evt.Repository.FullName != "acme/widgets" || evt.Installation.ID != 42 {
		t.Errorf("unexpected decoded push event: %+v", evt)
	}
	if !evt.IsDefaultBranchPush() {
		t.Error("expected push to refs/heads/main (default branch) to report true")
	}
}

func TestPushEvent_IsDefaultBranchPush_FeatureBranch(t *testing.T) {
	evt := PushEvent{Ref: "refs/heads/feature-x"}
	evt.Repository.DefaultBranch = "main"

	if evt.IsDefaultBranchPush() {
		t.Error("expected feature branch push to report false")
	}
}

func TestParseWebhook_Installation(t *testing.T) {
	payload := []byte(`{"action":"created","installation":{"id":7,"account":{"login":"acme"}}}`)

	eventType, decoded, err := ParseWebhook("installation", payload)
	if err != nil {
		t.Fatalf("ParseWebhook: %v", err)
	}
	if eventType != EventInstallation {
		t.Fatalf("eventType = %v, want %v", eventType, EventInstallation)
	}
	evt, ok := decoded.(*InstallationEvent)
	if !ok {
		t.Fatalf("decoded type = %T, want *InstallationEvent", decoded)
	}
	if evt.Action != "created" || evt.Installation.ID != 7 {
		t.Errorf("unexpected decoded installation event: %+v", evt)
	}
}

func TestParseWebhook_Unhandled(t *testing.T) {
	eventType, decoded, err := ParseWebhook("star", []byte(`{}`))
	if err != nil {
		t.Fatalf("ParseWebhook should not error on unhandled event types: %v", err)
	}
	if eventType != EventUnhandled || decoded != nil {
		t.Errorf("expected (EventUnhandled, nil) for unrecognized event, got (%v, %v)", eventType, decoded)
	}
}

func TestParseWebhook_IssueComment(t *testing.T) {
	payload := []byte(`{
		"action": "created",
		"issue": {
			"number": 12,
			"pull_request": {"url": "https://api.github.com/repos/acme/widgets/pulls/12"}
		},
		"comment": {
			"id": 999,
			"body": "@langpeanut translate --locales es,ja --tone formal",
			"user": {"login": "octocat"}
		},
		"repository": {"name": "widgets", "full_name": "acme/widgets", "default_branch": "main", "owner": {"login": "acme"}},
		"installation": {"id": 42}
	}`)

	eventType, decoded, err := ParseWebhook("issue_comment", payload)
	if err != nil {
		t.Fatalf("ParseWebhook: %v", err)
	}
	if eventType != EventIssueComment {
		t.Fatalf("eventType = %v, want %v", eventType, EventIssueComment)
	}
	evt, ok := decoded.(*IssueCommentEvent)
	if !ok {
		t.Fatalf("decoded type = %T, want *IssueCommentEvent", decoded)
	}
	if !evt.IsPullRequestComment() {
		t.Error("expected IsPullRequestComment to be true")
	}

	cmd, hasCmd := ParseBotCommand(evt.Comment.Body)
	if !hasCmd || cmd == nil {
		t.Fatal("expected @langpeanut bot command to be detected")
	}
	if cmd.Action != "translate" {
		t.Errorf("expected action 'translate', got %s", cmd.Action)
	}
	if len(cmd.Locales) != 2 || cmd.Locales[0] != "es" || cmd.Locales[1] != "ja" {
		t.Errorf("expected locales [es, ja], got %v", cmd.Locales)
	}
	if cmd.Tone != "formal" {
		t.Errorf("expected tone 'formal', got %s", cmd.Tone)
	}
}

func TestParseWebhook_MalformedJSON(t *testing.T) {
	_, _, err := ParseWebhook("push", []byte(`not json`))
	if err == nil {
		t.Fatal("expected error for malformed push payload")
	}
}

