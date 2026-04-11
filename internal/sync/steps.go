package sync

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
	"github.com/eddiecarpenter/gh-agentic/internal/fsutil"
	"github.com/eddiecarpenter/gh-agentic/internal/tarball"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// backupSuffix is the directory name suffix used for the .ai/ backup.
const backupSuffix = ".agentic-sync-backup"

// MigrateBaseToAI detects repos still using the old base/ layout and renames
// base/ to .ai/ using git mv to preserve history. Returns (true, nil) when
// migration occurred, (false, nil) when no migration was needed.
//
// Logic:
//   - .ai/ exists → no migration needed
//   - base/ exists and .ai/ does not → rename via git mv, return migrated=true
//   - Both exist → use .ai/, ignore base/
//   - Rename fails → return clear error
func MigrateBaseToAI(repoRoot string, run bootstrap.RunCommandFunc) (bool, error) {
	aiDir := filepath.Join(repoRoot, ".ai")
	baseDir := filepath.Join(repoRoot, "base")

	_, aiErr := os.Stat(aiDir)
	aiExists := aiErr == nil

	_, baseErr := os.Stat(baseDir)
	baseExists := baseErr == nil

	// .ai/ already exists — no migration needed regardless of base/.
	if aiExists {
		return false, nil
	}

	// Neither exists — nothing to migrate.
	if !baseExists {
		return false, nil
	}

	// base/ exists but .ai/ does not — rename via git mv.
	out, err := runInDir(run, repoRoot, "git", "mv", "base", ".ai")
	if err != nil {
		return false, fmt.Errorf("migrating base/ to .ai/: %w\n%s", err, strings.TrimSpace(out))
	}

	return true, nil
}

// fetchTarballFn is the tarball fetch function used by FetchAndExtractTemplate.
// Tests override this to inject fakes without making real HTTP requests.
var fetchTarballFn = tarball.DefaultFetch

// SetFetchTarballFn replaces the package-level fetch function used by
// FetchAndExtractTemplate. Returns the previous function so callers can
// restore it. Intended for use by tests in other packages (e.g. internal/cli).
func SetFetchTarballFn(fn tarball.FetchFunc) tarball.FetchFunc {
	prev := fetchTarballFn
	fetchTarballFn = fn
	return prev
}

// FetchAndExtractTemplate fetches the release tarball for the given repo and version,
// extracts it to a temporary directory, and then deploys the template-managed
// directories (.ai/, .github/workflows/, .goose/recipes/) and root files
// (CLAUDE.md, AGENTS.md) into the repo root.
// The tarballURL is validated before any fetch is attempted.
//
// This replaces the old CloneTemplate + CopyAI + DeployWorkflows + DeployRecipes
// pipeline with a single atomic operation. On failure, no partial extraction
// remains in repoRoot (the backup/restore flow in RunSync handles rollback).
//
// workflowExcludes lists workflow filenames that should not be deployed (e.g.
// "sync-status-to-label.yml" for personal repos). Excluded files are also
// retroactively removed from the destination if they already exist.
func FetchAndExtractTemplate(tarballURL, repo, version, repoRoot string, workflowExcludes []string) error {
	if tarballURL == "" {
		return fmt.Errorf("tarball URL is empty for release %s — cannot fetch template", version)
	}

	// Extract the full tarball to a temp directory.
	tmpDir, err := os.MkdirTemp("", "agentic-sync-tarball-*")
	if err != nil {
		return fmt.Errorf("creating temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Extract .ai/ (includes .ai/.github/workflows/ and .ai/.github/actions/),
	// .goose/ (recipes), and root files (CLAUDE.md, AGENTS.md) so the subsequent
	// deploy steps can operate on the extracted content.
	extractPrefixes := []string{".ai/", ".goose/", "CLAUDE.md", "AGENTS.md"}
	if err := tarball.ExtractFromTemplate(repo, version, tmpDir, extractPrefixes, fetchTarballFn); err != nil {
		return err
	}

	// Deploy .ai/ from extracted tarball (full replace).
	if err := CopyAI(tmpDir, repoRoot); err != nil {
		return err
	}

	// Deploy root files (CLAUDE.md, AGENTS.md) from extracted tarball.
	if err := DeployRootFiles(tmpDir, repoRoot); err != nil {
		return err
	}

	// Deploy workflows from extracted tarball.
	if err := DeployWorkflows(tmpDir, repoRoot, workflowExcludes); err != nil {
		return err
	}

	// Deploy composite actions from extracted tarball.
	if err := DeployActions(tmpDir, repoRoot); err != nil {
		return err
	}

	// Deploy recipes from extracted tarball.
	if err := DeployRecipes(tmpDir, repoRoot); err != nil {
		return err
	}

	return nil
}

// BackupAI copies existing .ai/, .github/workflows/, .github/actions/, and
// .goose/recipes/ to a temp backup location. Returns the backup directory path.
// The caller is responsible for cleanup.
func BackupAI(repoRoot string) (string, error) {
	aiSrc := filepath.Join(repoRoot, ".ai")
	workflowsSrc := filepath.Join(repoRoot, ".github", "workflows")
	actionsSrc := filepath.Join(repoRoot, ".github", "actions")
	recipesSrc := filepath.Join(repoRoot, ".goose", "recipes")

	aiExists := false
	if _, err := os.Stat(aiSrc); err == nil {
		aiExists = true
	}

	workflowsExist := false
	if _, err := os.Stat(workflowsSrc); err == nil {
		workflowsExist = true
	}

	actionsExist := false
	if _, err := os.Stat(actionsSrc); err == nil {
		actionsExist = true
	}

	recipesExist := false
	if _, err := os.Stat(recipesSrc); err == nil {
		recipesExist = true
	}

	// Nothing to back up on first sync.
	if !aiExists && !workflowsExist && !actionsExist && !recipesExist {
		return "", nil
	}

	backupDir, err := os.MkdirTemp("", "agentic-ai"+backupSuffix)
	if err != nil {
		return "", fmt.Errorf("creating backup directory: %w", err)
	}

	if aiExists {
		if err := fsutil.CopyDir(aiSrc, filepath.Join(backupDir, "ai")); err != nil {
			_ = os.RemoveAll(backupDir)
			return "", fmt.Errorf("backing up .ai/: %w", err)
		}
	}

	if workflowsExist {
		if err := fsutil.CopyDir(workflowsSrc, filepath.Join(backupDir, "workflows")); err != nil {
			_ = os.RemoveAll(backupDir)
			return "", fmt.Errorf("backing up .github/workflows/: %w", err)
		}
	}

	if actionsExist {
		if err := fsutil.CopyDir(actionsSrc, filepath.Join(backupDir, "actions")); err != nil {
			_ = os.RemoveAll(backupDir)
			return "", fmt.Errorf("backing up .github/actions/: %w", err)
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

// DeployWorkflows copies workflow files from the extracted template's
// .ai/.github/workflows/ into the local repo's .github/workflows/.
// Files in excludeFiles are skipped during copy and removed from the
// destination if they already exist (retroactive cleanup).
// Returns nil if the source directory does not exist (template has no workflows).
func DeployWorkflows(tmpDir, repoRoot string, excludeFiles []string) error {
	src := filepath.Join(tmpDir, ".ai", ".github", "workflows")
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

// DeployActions copies composite action directories from the extracted template's
// .ai/.github/actions/ into the local repo's .github/actions/ using merge semantics:
// template files overwrite existing copies, project-owned files are preserved,
// no directories are deleted.
// Returns nil if the source directory does not exist (template has no actions).
func DeployActions(tmpDir, repoRoot string) error {
	src := filepath.Join(tmpDir, ".ai", ".github", "actions")
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil
	}

	dst := filepath.Join(repoRoot, ".github", "actions")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return fmt.Errorf("creating .github/actions/: %w", err)
	}

	// Walk the source tree and mirror it into the destination.
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(dstPath, 0o755)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading action file %s: %w", rel, err)
		}
		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("getting info for %s: %w", rel, err)
		}
		if err := os.WriteFile(dstPath, data, info.Mode()); err != nil {
			return fmt.Errorf("writing action file %s: %w", rel, err)
		}
		return nil
	})
}

// DeployRecipes copies .goose/recipes/ from the extracted template into the local
// repo's .goose/recipes/ using merge semantics: template files overwrite existing
// copies, project-owned files are preserved, no files are deleted.
// Returns nil if the source directory does not exist (template has no recipes).
func DeployRecipes(tmpDir, repoRoot string) error {
	src := filepath.Join(tmpDir, ".goose", "recipes")
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil
	}

	dst := filepath.Join(repoRoot, ".goose", "recipes")

	// Ensure destination directory exists.
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return fmt.Errorf("creating .goose/recipes/: %w", err)
	}

	// Copy each template recipe individually, overwriting if it exists
	// but preserving project-owned files that are not in the template.
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("reading template recipes: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(src, entry.Name()))
		if readErr != nil {
			return fmt.Errorf("reading recipe %s: %w", entry.Name(), readErr)
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return fmt.Errorf("getting info for %s: %w", entry.Name(), infoErr)
		}
		if writeErr := os.WriteFile(filepath.Join(dst, entry.Name()), data, info.Mode()); writeErr != nil {
			return fmt.Errorf("writing recipe %s: %w", entry.Name(), writeErr)
		}
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

// CopyAI copies .ai/ from the extracted template into the local repo,
// replacing the existing .ai/ directory (full replace semantics).
func CopyAI(tmpDir, repoRoot string) error {
	src := filepath.Join(tmpDir, ".ai")
	dst := filepath.Join(repoRoot, ".ai")

	if _, err := os.Stat(src); os.IsNotExist(err) {
		return fmt.Errorf("template does not contain a .ai/ directory")
	}

	// Remove existing .ai/ so we get a clean copy.
	if err := os.RemoveAll(dst); err != nil {
		return fmt.Errorf("removing existing .ai/: %w", err)
	}

	if err := fsutil.CopyDir(src, dst); err != nil {
		return fmt.Errorf("copying .ai/: %w", err)
	}

	return nil
}

// DeployRootFiles copies CLAUDE.md and AGENTS.md from the extracted template
// root into the local repo root, overwriting any existing copies.
func DeployRootFiles(tmpDir, repoRoot string) error {
	for _, name := range []string{"CLAUDE.md", "AGENTS.md"} {
		src := filepath.Join(tmpDir, name)
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue // File not in template — skip silently.
		}
		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("reading %s from template: %w", name, err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, name), data, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", name, err)
		}
	}
	return nil
}

// ShowDiff runs git diff on .ai/ and .github/workflows/ and returns the output.
func ShowDiff(repoRoot string, run bootstrap.RunCommandFunc) (string, error) {
	out, err := runInDir(run, repoRoot, "git", "diff", ".ai/")
	if err != nil {
		return "", fmt.Errorf("git diff: %w\n%s", err, strings.TrimSpace(out))
	}

	// Also check for untracked files in .ai/.
	untrackedOut, _ := runInDir(run, repoRoot, "git", "ls-files", "--others", "--exclude-standard", ".ai/")
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

	// Include .github/actions/ diffs.
	actionsDiff, _ := runInDir(run, repoRoot, "git", "diff", ".github/actions/")
	if strings.TrimSpace(actionsDiff) != "" {
		out += "\n" + actionsDiff
	}

	// Include untracked action files.
	actionsUntracked, _ := runInDir(run, repoRoot, "git", "ls-files", "--others", "--exclude-standard", ".github/actions/")
	if strings.TrimSpace(actionsUntracked) != "" {
		out += "\n--- New action files ---\n" + actionsUntracked
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

// UpdateVersion writes the new version into .ai/config.yml, preserving the
// existing template field. TEMPLATE_VERSION is no longer written — use
// DeleteLegacyVersionFiles to remove it after staging.
func UpdateVersion(repoRoot, version string) error {
	cfg, err := ReadAIConfig(repoRoot)
	if err != nil {
		// config.yml not yet present — fall back to reading the template source.
		// TODO(deprecated): remove TEMPLATE_SOURCE fallback in next major version.
		source, srcErr := ReadTemplateSource(repoRoot)
		if srcErr != nil {
			return fmt.Errorf("reading template source for config.yml: %w", srcErr)
		}
		cfg.Template = source
	}
	cfg.Version = version

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshalling .ai/config.yml: %w", err)
	}
	configPath := filepath.Join(repoRoot, ".ai", "config.yml")
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		return fmt.Errorf("writing .ai/config.yml: %w", err)
	}
	return nil
}

// DeleteLegacyVersionFiles removes TEMPLATE_SOURCE and TEMPLATE_VERSION from
// the repo root if they exist. These files were superseded by .ai/config.yml
// in v1.5.0.
func DeleteLegacyVersionFiles(repoRoot string) error {
	for _, name := range []string{"TEMPLATE_SOURCE", "TEMPLATE_VERSION"} {
		path := filepath.Join(repoRoot, name)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing %s: %w", name, err)
		}
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

// StageSync stages all sync changes for commit. Removes legacy TEMPLATE_SOURCE
// and TEMPLATE_VERSION from git tracking (they are superseded by .ai/config.yml),
// then stages .ai/, .github/workflows/, .github/actions/, CLAUDE.md, AGENTS.md,
// .goose/recipes/, and POST_SYNC.md.
func StageSync(repoRoot string, run bootstrap.RunCommandFunc) error {
	// Remove legacy files from git tracking — ignore if already gone.
	if out, err := runInDir(run, repoRoot, "git", "rm", "--ignore-unmatch", "--cached", "TEMPLATE_SOURCE", "TEMPLATE_VERSION"); err != nil {
		return fmt.Errorf("git rm legacy files: %w\n%s", err, strings.TrimSpace(out))
	}
	if out, err := runInDir(run, repoRoot, "git", "add", ".ai/", "CLAUDE.md", "AGENTS.md", ".github/workflows/", ".github/actions/", ".goose/recipes/", "POST_SYNC.md"); err != nil {
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
	msg := fmt.Sprintf("chore: sync .ai/, workflows, and recipes from %s %s", repo, version)
	if out, err := runInDir(run, repoRoot, "git", "commit", "-m", msg); err != nil {
		return fmt.Errorf("git commit: %w\n%s", err, strings.TrimSpace(out))
	}

	return nil
}

// RestoreAI restores .ai/, .github/workflows/, .github/actions/, and .goose/recipes/
// from a backup created by BackupAI. If backupDir is empty, there was nothing to restore.
func RestoreAI(repoRoot, backupDir string) error {
	if backupDir == "" {
		return nil
	}

	// Restore .ai/ if it was backed up.
	aiSrc := filepath.Join(backupDir, "ai")
	if _, err := os.Stat(aiSrc); err == nil {
		aiDst := filepath.Join(repoRoot, ".ai")
		if err := os.RemoveAll(aiDst); err != nil {
			return fmt.Errorf("removing .ai/ for restore: %w", err)
		}
		if err := fsutil.CopyDir(aiSrc, aiDst); err != nil {
			return fmt.Errorf("restoring .ai/: %w", err)
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

	// Restore .github/actions/ if it was backed up.
	actionsSrc := filepath.Join(backupDir, "actions")
	if _, err := os.Stat(actionsSrc); err == nil {
		actionsDst := filepath.Join(repoRoot, ".github", "actions")
		if err := os.RemoveAll(actionsDst); err != nil {
			return fmt.Errorf("removing .github/actions/ for restore: %w", err)
		}
		if err := fsutil.CopyDir(actionsSrc, actionsDst); err != nil {
			return fmt.Errorf("restoring .github/actions/: %w", err)
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
