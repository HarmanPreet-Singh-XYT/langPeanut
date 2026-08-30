// Package mirror manages persistent bare git mirrors for connected repos.
// One mirror per repo lives at data/mirrors/{repo_id}.git (a bare clone).
// Before each job the mirror is refreshed via `git fetch`; the job's working
// copy then clones from the local mirror — avoiding a fresh GitHub clone every run.
package mirror

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Manager handles bare git mirror creation and refresh.
type Manager struct {
	mirrorsDir string // absolute path to data/mirrors/
}

// New creates a Manager rooted at mirrorsDir (e.g. "/data/mirrors").
func New(mirrorsDir string) (*Manager, error) {
	if err := os.MkdirAll(mirrorsDir, 0o750); err != nil {
		return nil, fmt.Errorf("create mirrors dir: %w", err)
	}
	return &Manager{mirrorsDir: mirrorsDir}, nil
}

// MirrorPath returns the absolute path to the bare mirror for the given repo ID.
func (m *Manager) MirrorPath(repoID int64) string {
	return filepath.Join(m.mirrorsDir, fmt.Sprintf("%d.git", repoID))
}

// EnsureMirror creates the bare mirror if it doesn't exist, or fetches updates
// if it does.  authURL is the authenticated clone URL
// (https://x-access-token:<token>@github.com/owner/repo.git).
// Returns the bare mirror path for use as the clone source.
func (m *Manager) EnsureMirror(repoID int64, authURL string) (string, error) {
	mirrorPath := m.MirrorPath(repoID)

	if _, err := os.Stat(mirrorPath); os.IsNotExist(err) {
		// First time: create the bare mirror.
		if err := gitRun("", "clone", "--mirror", redactURL(authURL), mirrorPath); err != nil {
			// Re-run with real URL but redact it in any returned error.
			cmd := exec.Command("git", "clone", "--mirror", authURL, mirrorPath)
			out, runErr := cmd.CombinedOutput()
			if runErr != nil {
				return "", fmt.Errorf("git clone --mirror: %s", redactOutput(string(out), authURL))
			}
		}
	} else {
		// Already exists: fetch updates using the stored remote.
		// We need to update the remote URL so it has a fresh token.
		if err := gitRun(mirrorPath, "remote", "set-url", "origin", authURL); err != nil {
			cmd := exec.Command("git", "-C", mirrorPath, "remote", "set-url", "origin", authURL)
			if out, runErr := cmd.CombinedOutput(); runErr != nil {
				return "", fmt.Errorf("git remote set-url: %s", redactOutput(string(out), authURL))
			}
		}
		cmd := exec.Command("git", "-C", mirrorPath, "fetch", "--prune", "origin")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("git fetch: %s", redactOutput(string(out), authURL))
		}
	}
	return mirrorPath, nil
}

// CloneFromMirror creates a working-tree clone from the bare local mirror into destDir,
// then rewrites the remote URL to authURL so push goes directly to GitHub.
func (m *Manager) CloneFromMirror(mirrorPath, destDir, authURL string) error {
	if err := os.MkdirAll(filepath.Dir(destDir), 0o750); err != nil {
		return err
	}
	// Clone from local mirror (fast, no network).
	cmd := exec.Command("git", "clone", mirrorPath, destDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone from mirror: %s", string(out))
	}
	// Point origin at GitHub with the authenticated URL for push.
	cmd = exec.Command("git", "-C", destDir, "remote", "set-url", "origin", authURL)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git remote set-url: %s", redactOutput(string(out), authURL))
	}
	return nil
}

// HeadCommitSHA returns the current HEAD commit SHA of the given branch in the mirror.
func (m *Manager) HeadCommitSHA(mirrorPath, branch string) (string, error) {
	cmd := exec.Command("git", "-C", mirrorPath, "rev-parse", "refs/heads/"+branch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("rev-parse %s: %s", branch, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func gitRun(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", strings.Join(args, " "), string(out))
	}
	return nil
}

// redactURL replaces the token portion of an authenticated GitHub URL with ***
// so it's safe to include in logs / error messages.
func redactURL(u string) string {
	// https://x-access-token:<TOKEN>@github.com/...
	if idx := strings.Index(u, "@"); idx != -1 {
		return "https://x-access-token:***@github.com" + u[idx+len("@github.com"):]
	}
	return u
}

func redactOutput(s, authURL string) string {
	if idx := strings.Index(authURL, "@"); idx != -1 {
		token := authURL[len("https://x-access-token:"):idx]
		return strings.ReplaceAll(s, token, "***")
	}
	return s
}
