package github

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// RepoClient drives git operations for one job against one cloned repo,
// shelling out to the system `git` binary (available on the deploy VPS
// alongside CGO/tree-sitter tooling) rather than a Go git library — simpler
// auth-token injection and battle-tested push behavior.
type RepoClient struct {
	WorkDir string // ephemeral scratch directory this clone lives in
}

// CloneForJob performs a shallow clone of the repo into a fresh temp directory
// under baseDir, authenticating via the installation token embedded in the
// remote URL (GitHub's documented pattern: https://x-access-token:<token>@github.com/owner/repo.git).
func CloneForJob(ctx context.Context, baseDir, owner, repo, installationToken string) (*RepoClient, error) {
	workDir, err := os.MkdirTemp(baseDir, fmt.Sprintf("langpeanut-%s-%s-*", owner, repo))
	if err != nil {
		return nil, fmt.Errorf("create scratch dir: %w", err)
	}

	authenticatedURL := fmt.Sprintf("https://x-access-token:%s@github.com/%s/%s.git", installationToken, owner, repo)

	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", authenticatedURL, workDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(workDir)
		return nil, fmt.Errorf("git clone failed: %w: %s", err, redactToken(string(out), installationToken))
	}

	return &RepoClient{WorkDir: workDir}, nil
}

// Cleanup removes the scratch clone. Always call via defer after CloneForJob succeeds.
func (c *RepoClient) Cleanup() error {
	return os.RemoveAll(c.WorkDir)
}

// CreateBranch checks out a new branch named for this job, so the pipeline's
// file writes land isolated from the default branch.
func (c *RepoClient) CreateBranch(ctx context.Context, branchName string) error {
	return c.run(ctx, "checkout", "-b", branchName)
}

// DefaultBranchName generates the per-run branch name convention used across
// the cloud unit: langpeanut/i18n-<unix-timestamp>.
func DefaultBranchName() string {
	return fmt.Sprintf("langpeanut/i18n-%d", time.Now().Unix())
}

// CommitAll stages every change the pipeline made and commits with a
// deterministic message (no LLM-generated commit prose, consistent with the
// Zero-Generation Principle applied to PR titles/bodies).
func (c *RepoClient) CommitAll(ctx context.Context, message string) error {
	if err := c.run(ctx, "add", "-A"); err != nil {
		return err
	}
	if err := c.run(ctx, "-c", "user.name=langPeanut Bot", "-c", "user.email=bot@langpeanut.dev", "commit", "-m", message); err != nil {
		return err
	}
	return nil
}

// HasChanges reports whether the working tree has anything staged/unstaged to
// commit — used to skip opening an empty PR when the pipeline found nothing
// to localize.
func (c *RepoClient) HasChanges(ctx context.Context) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", c.WorkDir, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git status failed: %w", err)
	}
	return len(bytes.TrimSpace(out)) > 0, nil
}

// Push pushes the current branch to origin, using the same installation-token
// authentication baked into the clone URL by CloneForJob.
func (c *RepoClient) Push(ctx context.Context, branchName, installationToken string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", c.WorkDir, "push", "origin", branchName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git push failed: %w: %s", err, redactToken(string(out), installationToken))
	}
	return nil
}

func (c *RepoClient) run(ctx context.Context, args ...string) error {
	fullArgs := append([]string{"-C", c.WorkDir}, args...)
	cmd := exec.CommandContext(ctx, "git", fullArgs...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git %s failed: %w: %s", strings.Join(args, " "), err, string(out))
	}
	return nil
}

// redactToken strips the installation token out of git command output before
// it's wrapped into an error — clone/push failures often echo the remote URL,
// and that URL contains the live credential.
func redactToken(output, token string) string {
	if token == "" {
		return output
	}
	return strings.ReplaceAll(output, token, "***REDACTED***")
}
