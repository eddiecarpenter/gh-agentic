package sync

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
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

// BackupBase copies existing base/ to a temp backup location alongside the repo root.
// Returns the backup directory path. The caller is responsible for cleanup.
func BackupBase(repoRoot string) (string, error) {
	src := filepath.Join(repoRoot, "base")
	if _, err := os.Stat(src); os.IsNotExist(err) {
		// No base/ to back up — that's fine on first sync.
		return "", nil
	}

	backupDir, err := os.MkdirTemp("", "agentic-base"+backupSuffix)
	if err != nil {
		return "", fmt.Errorf("creating backup directory: %w", err)
	}

	if err := copyDir(src, filepath.Join(backupDir, "base")); err != nil {
		_ = os.RemoveAll(backupDir)
		return "", fmt.Errorf("backing up base/: %w", err)
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

	if err := copyDir(src, dst); err != nil {
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
func CommitSync(repoRoot, repo, version string, run bootstrap.RunCommandFunc) error {
	// Stage changes.
	if out, err := runInDir(run, repoRoot, "git", "add", "base/", "TEMPLATE_VERSION"); err != nil {
		return fmt.Errorf("git add: %w\n%s", err, strings.TrimSpace(out))
	}

	// Commit.
	msg := fmt.Sprintf("chore: sync base/ from %s %s", repo, version)
	if out, err := runInDir(run, repoRoot, "git", "commit", "-m", msg); err != nil {
		return fmt.Errorf("git commit: %w\n%s", err, strings.TrimSpace(out))
	}

	return nil
}

// RestoreBase restores base/ from a backup created by BackupBase.
// If backupDir is empty, there was nothing to restore.
func RestoreBase(repoRoot, backupDir string) error {
	if backupDir == "" {
		return nil
	}

	dst := filepath.Join(repoRoot, "base")
	src := filepath.Join(backupDir, "base")

	if err := os.RemoveAll(dst); err != nil {
		return fmt.Errorf("removing base/ for restore: %w", err)
	}

	if err := copyDir(src, dst); err != nil {
		return fmt.Errorf("restoring base/: %w", err)
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

// copyDir recursively copies src to dst, preserving file permissions.
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		return os.WriteFile(target, data, info.Mode())
	})
}

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
