package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
	"github.com/eddiecarpenter/gh-agentic/internal/fsutil"
)

// backupSuffix is the directory name suffix used for the base/ backup.
const backupSuffix = ".agentic-sync-backup"

// CloneTemplate clones the upstream template repo into tmpDir.
func CloneTemplate(repo, tmpDir string, run bootstrap.RunCommandFunc) error {
	url := fmt.Sprintf("https://github.com/%s.git", repo)
	out, err := run("git", "clone", "--depth", "1", url, tmpDir)
	if err != nil {
		return fmt.Errorf("git clone template: %w\n%s", err, strings.TrimSpace(out))
	}
	return nil
}

// BackupBase copies existing base/ and .github/workflows/ to a temp backup
// location. Returns the backup directory path. The caller is responsible for
// cleanup.
func BackupBase(repoRoot string) (string, error) {
	baseSrc := filepath.Join(repoRoot, "base")
	workflowsSrc := filepath.Join(repoRoot, ".github", "workflows")

	baseExists := false
	if _, err := os.Stat(baseSrc); err == nil {
		baseExists = true
	}

	workflowsExist := false
	if _, err := os.Stat(workflowsSrc); err == nil {
		workflowsExist = true
	}

	// Nothing to back up on first sync.
	if !baseExists && !workflowsExist {
		return "", nil
	}

	backupDir, err := os.MkdirTemp("", "agentic-base"+backupSuffix)
	if err != nil {
		return "", fmt.Errorf("creating backup directory: %w", err)
	}

	if baseExists {
		if err := fsutil.CopyDir(baseSrc, filepath.Join(backupDir, "base")); err != nil {
			_ = os.RemoveAll(backupDir)
			return "", fmt.Errorf("backing up base/: %w", err)
		}
	}

	if workflowsExist {
		if err := fsutil.CopyDir(workflowsSrc, filepath.Join(backupDir, "workflows")); err != nil {
			_ = os.RemoveAll(backupDir)
			return "", fmt.Errorf("backing up .github/workflows/: %w", err)
		}
	}

	return backupDir, nil
}

// CopyBase copies base/ from the cloned template into the local repo,
// replacing the existing base/ directory.
func CopyBase(tmpDir, repoRoot string) error {
	src := filepath.Join(tmpDir, "base")
	dst := filepath.Join(repoRoot, "base")

	if _, err := os.Stat(src); os.IsNotExist(err) {
		return fmt.Errorf("template does not contain a base/ directory")
	}

	// Remove existing base/ so we get a clean copy.
	if err := os.RemoveAll(dst); err != nil {
		return fmt.Errorf("removing existing base/: %w", err)
	}

	if err := fsutil.CopyDir(src, dst); err != nil {
		return fmt.Errorf("copying base/: %w", err)
	}

	return nil
}

// ShowDiff runs git diff on base/ and returns the output.
func ShowDiff(repoRoot string, run bootstrap.RunCommandFunc) (string, error) {
	out, err := runInDir(run, repoRoot, "git", "diff", "base/")
	if err != nil {
		return "", fmt.Errorf("git diff: %w\n%s", err, strings.TrimSpace(out))
	}

	// Also check for untracked files in base/.
	untrackedOut, _ := runInDir(run, repoRoot, "git", "ls-files", "--others", "--exclude-standard", "base/")
	if strings.TrimSpace(untrackedOut) != "" {
		out += "\n--- New files ---\n" + untrackedOut
	}

	return out, nil
}

// UpdateVersion writes the new version string to TEMPLATE_VERSION.
func UpdateVersion(repoRoot, version string) error {
	path := filepath.Join(repoRoot, "TEMPLATE_VERSION")
	if err := os.WriteFile(path, []byte(version+"\n"), 0o644); err != nil {
		return fmt.Errorf("writing TEMPLATE_VERSION: %w", err)
	}
	return nil
}

// CommitSync stages base/ and TEMPLATE_VERSION, then commits with a descriptive message.
// If nothing changed after staging, the commit is skipped cleanly.
func CommitSync(repoRoot, repo, version string, run bootstrap.RunCommandFunc) error {
	// Stage changes.
	if out, err := runInDir(run, repoRoot, "git", "add", "base/", "TEMPLATE_VERSION"); err != nil {
		return fmt.Errorf("git add: %w\n%s", err, strings.TrimSpace(out))
	}

	// Check if anything is actually staged — exit 0 means no diff (nothing to commit).
	if _, err := runInDir(run, repoRoot, "git", "diff", "--cached", "--quiet"); err == nil {
		return nil
	}

	// Commit.
	msg := fmt.Sprintf("chore: sync base/ from %s %s", repo, version)
	if out, err := runInDir(run, repoRoot, "git", "commit", "-m", msg); err != nil {
		return fmt.Errorf("git commit: %w\n%s", err, strings.TrimSpace(out))
	}

	return nil
}

// RestoreBase restores base/ and .github/workflows/ from a backup created by
// BackupBase. If backupDir is empty, there was nothing to restore.
func RestoreBase(repoRoot, backupDir string) error {
	if backupDir == "" {
		return nil
	}

	// Restore base/ if it was backed up.
	baseSrc := filepath.Join(backupDir, "base")
	if _, err := os.Stat(baseSrc); err == nil {
		baseDst := filepath.Join(repoRoot, "base")
		if err := os.RemoveAll(baseDst); err != nil {
			return fmt.Errorf("removing base/ for restore: %w", err)
		}
		if err := fsutil.CopyDir(baseSrc, baseDst); err != nil {
			return fmt.Errorf("restoring base/: %w", err)
		}
	}

	// Restore .github/workflows/ if it was backed up.
	workflowsSrc := filepath.Join(backupDir, "workflows")
	if _, err := os.Stat(workflowsSrc); err == nil {
		workflowsDst := filepath.Join(repoRoot, ".github", "workflows")
		if err := os.RemoveAll(workflowsDst); err != nil {
			return fmt.Errorf("removing .github/workflows/ for restore: %w", err)
		}
		if err := fsutil.CopyDir(workflowsSrc, workflowsDst); err != nil {
			return fmt.Errorf("restoring .github/workflows/: %w", err)
		}
	}

	return nil
}

// CleanupTemp removes the temporary directory. Safe to call with empty string.
func CleanupTemp(tmpDir string) error {
	if tmpDir == "" {
		return nil
	}
	return os.RemoveAll(tmpDir)
}

// ──────────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────────

// runInDir runs a command in the given directory via bash -c.
func runInDir(run bootstrap.RunCommandFunc, dir string, name string, args ...string) (string, error) {
	quotedDir := "'" + strings.ReplaceAll(dir, "'", "'\\''") + "'"
	inner := "cd " + quotedDir + " && " + shellJoin(name, args...)
	return run("bash", "-c", inner)
}

// shellJoin single-quotes each token and joins them with spaces.
func shellJoin(name string, args ...string) string {
	parts := make([]string, 0, 1+len(args))
	parts = append(parts, shellQuote(name))
	for _, a := range args {
		parts = append(parts, shellQuote(a))
	}
	return strings.Join(parts, " ")
}

// shellQuote wraps s in single quotes, escaping embedded single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
