package sync

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
	"github.com/eddiecarpenter/gh-agentic/internal/fsutil"
	"github.com/eddiecarpenter/gh-agentic/internal/tarball"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// backupSuffix is the directory name suffix used for the base/ backup.
const backupSuffix = ".agentic-sync-backup"

// syncPathPrefixes defines the path prefixes extracted from the template tarball
// during sync. These correspond to the directories that are managed by the template.
var syncPathPrefixes = []string{"base/", ".github/workflows/", ".goose/recipes/"}

// CloneTemplate clones the upstream template repo into tmpDir.
// Deprecated: Use FetchAndExtractTemplate instead. This function remains until
// RunSync is updated to use the tarball-based approach (task #332).
func CloneTemplate(repo, tmpDir string, run bootstrap.RunCommandFunc) error {
	url := fmt.Sprintf("https://github.com/%s.git", repo)
	out, err := run("git", "clone", "--depth", "1", url, tmpDir)
	if err != nil {
		return fmt.Errorf("git clone template: %w\n%s", err, strings.TrimSpace(out))
	}
	return nil
}

// fetchTarballFn is the tarball fetch function used by FetchAndExtractTemplate.
// Tests override this to inject fakes without making real HTTP requests.
var fetchTarballFn = tarball.DefaultFetch

// FetchAndExtractTemplate fetches the release tarball for the given repo and version,
// then extracts the template-managed directories (base/, .github/workflows/, .goose/recipes/)
// into the destination root. The tarballURL is validated before any fetch is attempted.
// Extraction is atomic: on failure, no partial extraction remains in destRoot.
func FetchAndExtractTemplate(tarballURL, repo, version, destRoot string) error {
	if tarballURL == "" {
		return fmt.Errorf("tarball URL is empty for release %s — cannot fetch template", version)
	}

	return tarball.ExtractFromTemplate(repo, version, destRoot, syncPathPrefixes, fetchTarballFn)
}

// BackupBase copies existing base/ and .github/workflows/ to a temp backup
// location. Returns the backup directory path. The caller is responsible for
// cleanup.
func BackupBase(repoRoot string) (string, error) {
	baseSrc := filepath.Join(repoRoot, "base")
	workflowsSrc := filepath.Join(repoRoot, ".github", "workflows")
	recipesSrc := filepath.Join(repoRoot, ".goose", "recipes")

	baseExists := false
	if _, err := os.Stat(baseSrc); err == nil {
		baseExists = true
	}

	workflowsExist := false
	if _, err := os.Stat(workflowsSrc); err == nil {
		workflowsExist = true
	}

	recipesExist := false
	if _, err := os.Stat(recipesSrc); err == nil {
		recipesExist = true
	}

	// Nothing to back up on first sync.
	if !baseExists && !workflowsExist && !recipesExist {
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

	if recipesExist {
		if err := fsutil.CopyDir(recipesSrc, filepath.Join(backupDir, "recipes")); err != nil {
			_ = os.RemoveAll(backupDir)
			return "", fmt.Errorf("backing up .goose/recipes/: %w", err)
		}
	}

	return backupDir, nil
}

// DeployWorkflows copies workflow files from the cloned template's
// base/.github/workflows/ into the local repo's .github/workflows/.
// Files in excludeFiles are skipped during copy and removed from the
// destination if they already exist (retroactive cleanup).
// Returns nil if the source directory does not exist (template has no workflows).
func DeployWorkflows(tmpDir, repoRoot string, excludeFiles []string) error {
	src := filepath.Join(tmpDir, "base", ".github", "workflows")
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil
	}

	dst := filepath.Join(repoRoot, ".github", "workflows")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return fmt.Errorf("creating .github/workflows/: %w", err)
	}

	// Build exclude set for O(1) lookups.
	excludeSet := make(map[string]bool, len(excludeFiles))
	for _, name := range excludeFiles {
		excludeSet[name] = true
	}

	// Copy files selectively, skipping excluded ones.
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("reading template workflows: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if excludeSet[entry.Name()] {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(src, entry.Name()))
		if readErr != nil {
			return fmt.Errorf("reading workflow %s: %w", entry.Name(), readErr)
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return fmt.Errorf("getting info for %s: %w", entry.Name(), infoErr)
		}
		if writeErr := os.WriteFile(filepath.Join(dst, entry.Name()), data, info.Mode()); writeErr != nil {
			return fmt.Errorf("writing workflow %s: %w", entry.Name(), writeErr)
		}
	}

	// Retroactive cleanup: remove excluded files if they already exist in the destination.
	for _, name := range excludeFiles {
		path := filepath.Join(dst, name)
		if _, statErr := os.Stat(path); statErr == nil {
			if removeErr := os.Remove(path); removeErr != nil {
				return fmt.Errorf("removing excluded workflow %s: %w", name, removeErr)
			}
		}
	}

	return nil
}

// DeployRecipes copies .goose/recipes/ from the cloned template into the local
// repo's .goose/recipes/. Returns nil if the source directory does not exist
// (template has no recipes).
func DeployRecipes(tmpDir, repoRoot string) error {
	src := filepath.Join(tmpDir, ".goose", "recipes")
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil
	}

	dst := filepath.Join(repoRoot, ".goose", "recipes")

	// Remove existing destination before copying, matching CopyBase pattern.
	if err := os.RemoveAll(dst); err != nil {
		return fmt.Errorf("removing existing .goose/recipes/: %w", err)
	}

	// Ensure parent directory exists.
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("creating .goose/: %w", err)
	}

	if err := fsutil.CopyDir(src, dst); err != nil {
		return fmt.Errorf("copying .goose/recipes/: %w", err)
	}

	return nil
}

// extractOwnerFromAgentsLocal parses AGENTS.local.md looking for the GitHub owner.
// It looks for lines like "- **GitHub:** https://github.com/<owner>/<repo>" or
// "- **Owner:** <owner>".
func extractOwnerFromAgentsLocal(root string) string {
	path := filepath.Join(root, "AGENTS.local.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	for _, line := range strings.Split(string(data), "\n") {
		// Try "- **GitHub:** https://github.com/<owner>/<repo>"
		if strings.Contains(line, "**GitHub:**") && strings.Contains(line, "github.com") {
			parts := strings.Split(line, "github.com/")
			if len(parts) >= 2 {
				rest := strings.TrimSpace(parts[1])
				ownerRepo := strings.SplitN(rest, "/", 2)
				if len(ownerRepo) >= 1 && ownerRepo[0] != "" {
					return ownerRepo[0]
				}
			}
		}

		// Try "- **Owner:** <owner>"
		if strings.Contains(line, "**Owner:**") {
			parts := strings.SplitN(line, "**Owner:**", 2)
			if len(parts) == 2 {
				owner := strings.TrimSpace(parts[1])
				if owner != "" {
					return owner
				}
			}
		}
	}

	return ""
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

// ShowDiff runs git diff on base/ and .github/workflows/ and returns the output.
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

	// Include .github/workflows/ diffs.
	wfDiff, _ := runInDir(run, repoRoot, "git", "diff", ".github/workflows/")
	if strings.TrimSpace(wfDiff) != "" {
		out += "\n" + wfDiff
	}

	// Include untracked workflow files.
	wfUntracked, _ := runInDir(run, repoRoot, "git", "ls-files", "--others", "--exclude-standard", ".github/workflows/")
	if strings.TrimSpace(wfUntracked) != "" {
		out += "\n--- New workflow files ---\n" + wfUntracked
	}

	// Include .goose/recipes/ diffs.
	recipesDiff, _ := runInDir(run, repoRoot, "git", "diff", ".goose/recipes/")
	if strings.TrimSpace(recipesDiff) != "" {
		out += "\n" + recipesDiff
	}

	// Include untracked recipe files.
	recipesUntracked, _ := runInDir(run, repoRoot, "git", "ls-files", "--others", "--exclude-standard", ".goose/recipes/")
	if strings.TrimSpace(recipesUntracked) != "" {
		out += "\n--- New recipe files ---\n" + recipesUntracked
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

// WritePostSyncMD writes the release body to POST_SYNC.md in the repo root.
// An empty releaseBody is valid — the file is still created (a release may have no notes).
func WritePostSyncMD(repoRoot string, releaseBody string) error {
	path := filepath.Join(repoRoot, "POST_SYNC.md")
	if err := os.WriteFile(path, []byte(releaseBody), 0o644); err != nil {
		return fmt.Errorf("writing POST_SYNC.md: %w", err)
	}
	return nil
}

// StageSync stages base/, .github/workflows/, TEMPLATE_VERSION, and POST_SYNC.md for commit.
func StageSync(repoRoot string, run bootstrap.RunCommandFunc) error {
	if out, err := runInDir(run, repoRoot, "git", "add", "base/", "TEMPLATE_VERSION", ".github/workflows/", ".goose/recipes/", "POST_SYNC.md"); err != nil {
		return fmt.Errorf("git add: %w\n%s", err, strings.TrimSpace(out))
	}
	return nil
}

// CommitSync stages base/, .github/workflows/, and TEMPLATE_VERSION, then
// commits with a descriptive message. If nothing changed after staging, the
// commit is skipped cleanly.
func CommitSync(repoRoot, repo, version string, run bootstrap.RunCommandFunc) error {
	// Stage changes.
	if err := StageSync(repoRoot, run); err != nil {
		return err
	}

	// Check if anything is actually staged — exit 0 means no diff (nothing to commit).
	if _, err := runInDir(run, repoRoot, "git", "diff", "--cached", "--quiet"); err == nil {
		return nil
	}

	// Commit.
	msg := fmt.Sprintf("chore: sync base/, workflows, and recipes from %s %s", repo, version)
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

	// Restore .goose/recipes/ if it was backed up.
	recipesSrc := filepath.Join(backupDir, "recipes")
	if _, err := os.Stat(recipesSrc); err == nil {
		recipesDst := filepath.Join(repoRoot, ".goose", "recipes")
		if err := os.RemoveAll(recipesDst); err != nil {
			return fmt.Errorf("removing .goose/recipes/ for restore: %w", err)
		}
		if err := fsutil.CopyDir(recipesSrc, recipesDst); err != nil {
			return fmt.Errorf("restoring .goose/recipes/: %w", err)
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

// DisplayReleaseNotes renders a styled release notes block to the given writer.
// The output includes a heading line with the tag name and the release body.
func DisplayReleaseNotes(w io.Writer, release Release) {
	divider := ui.Muted.Render(strings.Repeat("─", 50))

	fmt.Fprintln(w, "  "+ui.Muted.Render("── Release notes: "+release.TagName+" ")+""+divider)
	if strings.TrimSpace(release.Body) != "" {
		for _, line := range strings.Split(strings.TrimSpace(release.Body), "\n") {
			fmt.Fprintln(w, "  "+line)
		}
	} else {
		fmt.Fprintln(w, "  "+ui.Muted.Render("No release notes available"))
	}
	fmt.Fprintln(w, "  "+divider)
}

// DisplayReleaseList renders all releases with their version tags and notes.
// Used by the --list flag to display available releases without performing a sync.
func DisplayReleaseList(w io.Writer, releases []Release) {
	fmt.Fprintln(w, "  "+ui.SectionHeading.Render("Available releases:"))
	fmt.Fprintln(w)
	for i, r := range releases {
		label := r.TagName
		if r.Name != "" {
			label += "  — " + r.Name
		}
		fmt.Fprintln(w, "  "+ui.Value.Render(label))
		if strings.TrimSpace(r.Body) != "" {
			divider := ui.Muted.Render(strings.Repeat("─", 40))
			fmt.Fprintln(w, "  "+ui.Muted.Render("── Release notes ──"))
			for _, line := range strings.Split(strings.TrimSpace(r.Body), "\n") {
				fmt.Fprintln(w, "  "+line)
			}
			fmt.Fprintln(w, "  "+divider)
		}
		if i < len(releases)-1 {
			fmt.Fprintln(w)
		}
	}
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
