package github

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupBareRemote creates a local bare git repo to stand in for a GitHub
// remote, plus an initial commit on its default branch, so CloneForJob-style
// operations can be exercised end-to-end without any network access.
func setupBareRemote(t *testing.T) (remoteDir string) {
	t.Helper()
	base := t.TempDir()

	bareDir := filepath.Join(base, "remote.git")
	if out, err := exec.Command("git", "init", "--bare", "-b", "main", bareDir).CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v: %s", err, out)
	}

	seedDir := filepath.Join(base, "seed")
	if err := os.MkdirAll(seedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, seedDir, "init", "-b", "main")
	runGit(t, seedDir, "config", "user.email", "seed@test.local")
	runGit(t, seedDir, "config", "user.name", "seed")
	if err := os.WriteFile(filepath.Join(seedDir, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, seedDir, "add", "-A")
	runGit(t, seedDir, "commit", "-m", "initial commit")
	runGit(t, seedDir, "remote", "add", "origin", bareDir)
	runGit(t, seedDir, "push", "origin", "main")

	return bareDir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s failed: %v: %s", strings.Join(args, " "), err, out)
	}
}

// cloneLocal bypasses CloneForJob's GitHub-token URL construction (there's no
// real GitHub endpoint in this test) and clones the bare remote directly,
// returning a RepoClient wired to the result — everything downstream
// (CreateBranch, CommitAll, HasChanges, Push) is exercised identically to how
// CloneForJob's caller would use it.
func cloneLocal(t *testing.T, remoteDir string) *RepoClient {
	t.Helper()
	workDir := t.TempDir()
	if out, err := exec.Command("git", "clone", remoteDir, workDir).CombinedOutput(); err != nil {
		t.Fatalf("git clone: %v: %s", err, out)
	}
	runGit(t, workDir, "config", "user.email", "bot@test.local")
	runGit(t, workDir, "config", "user.name", "bot")
	return &RepoClient{WorkDir: workDir}
}

func TestRepoClient_BranchCommitPushFlow(t *testing.T) {
	ctx := context.Background()
	remote := setupBareRemote(t)
	client := cloneLocal(t, remote)

	branch := DefaultBranchName()
	if !strings.HasPrefix(branch, "langpeanut/i18n-") {
		t.Fatalf("unexpected branch name shape: %s", branch)
	}

	if err := client.CreateBranch(ctx, branch); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}

	if err := os.WriteFile(filepath.Join(client.WorkDir, "locales", "fr.json"), nil, 0o644); err == nil {
		t.Fatal("expected write to nonexistent dir to fail without mkdir")
	}
	if err := os.MkdirAll(filepath.Join(client.WorkDir, "locales"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(client.WorkDir, "locales", "fr.json"), []byte(`{"hello":"bonjour"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	hasChanges, err := client.HasChanges(ctx)
	if err != nil {
		t.Fatalf("HasChanges: %v", err)
	}
	if !hasChanges {
		t.Fatal("expected HasChanges to report true after adding a new file")
	}

	if err := client.CommitAll(ctx, "i18n: add French locale"); err != nil {
		t.Fatalf("CommitAll: %v", err)
	}

	hasChanges, err = client.HasChanges(ctx)
	if err != nil {
		t.Fatalf("HasChanges after commit: %v", err)
	}
	if hasChanges {
		t.Fatal("expected HasChanges to report false after commit")
	}

	if err := client.Push(ctx, branch, ""); err != nil {
		t.Fatalf("Push: %v", err)
	}

	out, err := exec.Command("git", "-C", remote, "branch", "--list", branch).CombinedOutput()
	if err != nil {
		t.Fatalf("verify branch on remote: %v: %s", err, out)
	}
	if !strings.Contains(string(out), branch) {
		t.Fatalf("expected branch %q to exist on remote after push, git branch output: %s", branch, out)
	}
}

func TestRepoClient_Cleanup(t *testing.T) {
	remote := setupBareRemote(t)
	client := cloneLocal(t, remote)

	if _, err := os.Stat(client.WorkDir); err != nil {
		t.Fatalf("expected workdir to exist before cleanup: %v", err)
	}
	if err := client.Cleanup(); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if _, err := os.Stat(client.WorkDir); !os.IsNotExist(err) {
		t.Fatalf("expected workdir to be removed after cleanup, stat err = %v", err)
	}
}

func TestRedactToken(t *testing.T) {
	out := redactToken("remote: fatal https://x-access-token:ghs_secret123@github.com/x/y.git", "ghs_secret123")
	if strings.Contains(out, "ghs_secret123") {
		t.Fatalf("token was not redacted: %s", out)
	}
	if !strings.Contains(out, "REDACTED") {
		t.Fatalf("expected redaction marker in output: %s", out)
	}
}
