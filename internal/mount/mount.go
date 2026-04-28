package mount

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// ValidateTag checks that the requested version tag exists among available
// releases. If the tag is not found, returns an error that includes the
// latest available version for guidance.
func ValidateTag(version string, releases []Release) error {
	_, found := FindReleaseByTag(releases, version)
	if found {
		return nil
	}

	latest := "unknown"
	if len(releases) > 0 {
		latest = releases[0].TagName
	}

	return fmt.Errorf("version %s not found — latest available version is %s", version, latest)
}

// DefaultClone performs a shallow git clone of repoURL at the given tag into
// destDir. The .git/ directory is retained so that the mounted version can be
// read from git metadata and the clone is left in detached HEAD state (no branch
// to push to).
func DefaultClone(repoURL, tag, destDir string) error {
	cmd := exec.Command("git", "clone", "--depth", "1", "--branch", tag, repoURL, destDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone failed: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ReadAIVersionFromGit reads the mounted framework version from the .git
// metadata inside .ai/. This is the authoritative source of truth — it reflects
// exactly what was cloned, not what .ai-version says.
func ReadAIVersionFromGit(root string) (string, error) {
	aiDir := filepath.Join(root, ".ai")
	cmd := exec.Command("git", "-C", aiDir, "describe", "--tags", "--exact-match")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("reading version from .ai/.git: %w", err)
	}
	v := strings.TrimSpace(string(out))
	if v == "" {
		return "", fmt.Errorf(".ai/.git has no version tag")
	}
	return v, nil
}

// DownloadFramework installs or updates the framework mount at
// destRoot/.ai/ as a tracked git submodule pointing at the framework
// repo at the given version tag. The dispatch is idempotent over four
// working-tree states (see DetectMountState):
//
//   - MountStateNone           → InstallSubmodule (fresh install)
//   - MountStateSubmodule      → SwapSubmodule (version swap)
//   - MountStateGitignoredMount → MigrateGitignoredMount (legacy → submodule)
//   - MountStateSymlink        → refused (gh-agentic itself)
//   - MountStateInconsistent   → refused (working tree is in an unsafe state)
//
// Production callers pass `clone = nil` — submodule operations rely on
// the system `git` binary acting on the parent repo and need no
// injectable clone. Tests may pass a stub `CloneFunc` to take a
// shortcut path that bypasses real submodule operations: it removes
// any existing `.ai/`, invokes the stub to populate `.ai/`, and stops.
// This preserves the long-standing test paradigm in which a stub
// CloneFunc fakes the framework checkout, without forcing every test
// to scaffold a real git repo.
func DownloadFramework(destRoot, version string, clone CloneFunc) error {
	if version == "" {
		return fmt.Errorf("version is empty — cannot install framework")
	}

	if clone != nil {
		aiDir := filepath.Join(destRoot, ".ai")
		if err := os.RemoveAll(aiDir); err != nil {
			return fmt.Errorf("removing existing .ai/: %w", err)
		}
		if err := clone(FrameworkRepoURL, version, aiDir); err != nil {
			return fmt.Errorf("cloning framework: %w", err)
		}
		return nil
	}

	state, err := DetectMountState(destRoot)
	if err != nil {
		return fmt.Errorf("detecting mount state: %w", err)
	}

	switch state {
	case MountStateSymlink:
		return fmt.Errorf(".ai is a symlink (this is gh-agentic itself); refusing to overwrite the framework source")
	case MountStateNone:
		return InstallSubmodule(destRoot, version)
	case MountStateSubmodule:
		return SwapSubmodule(destRoot, version)
	case MountStateGitignoredMount:
		return MigrateGitignoredMount(destRoot, version)
	case MountStateInconsistent:
		return fmt.Errorf(".ai/ exists but is neither a symlink, a tracked submodule, nor a gitignored legacy mount — working tree is inconsistent. Resolve manually before running upgrade")
	default:
		return fmt.Errorf("unknown mount state: %d", state)
	}
}

// EnsureGitignore ensures that ".ai/" is listed in .gitignore at root.
func EnsureGitignore(root string) error {
	return ensureGitignoreEntry(root, ".ai/")
}

// ensureGitignoreEntry appends entry to .gitignore if it is not already
// present. The entry is matched line-for-line after trimming whitespace.
func ensureGitignoreEntry(root, entry string) error {
	gitignorePath := filepath.Join(root, ".gitignore")

	var content string
	data, err := os.ReadFile(gitignorePath)
	if err == nil {
		content = string(data)
	}

	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == entry {
			return nil
		}
	}

	toAppend := entry + "\n"
	if content != "" && !strings.HasSuffix(content, "\n") {
		toAppend = "\n" + toAppend
	}

	return os.WriteFile(gitignorePath, []byte(content+toAppend), 0o644)
}

// versionTagPattern matches @vX.Y.Z version tags in uses: lines for
// the gh-agentic reusable workflows.
var versionTagPattern = regexp.MustCompile(`(eddiecarpenter/gh-agentic/\.github/workflows/[^@]+)@(v[0-9]+\.[0-9]+\.[0-9]+[^\s]*)`)

// UpdateWorkflowVersions scans all .yml files in .github/workflows/ and
// replaces gh-agentic version tags with the new version.
func UpdateWorkflowVersions(root, newVersion string) error {
	_, err := UpdateWorkflowVersionsCount(root, newVersion)
	return err
}

// UpdateWorkflowVersionsCount is like UpdateWorkflowVersions but reports the
// number of files actually rewritten. Returns 0 when the workflows in
// .github/workflows/ don't reference gh-agentic reusable workflows at all
// (the current framework template uses inlined steps, not @vX.Y.Z refs).
func UpdateWorkflowVersionsCount(root, newVersion string) (int, error) {
	workflowsDir := filepath.Join(root, ".github", "workflows")

	entries, err := os.ReadDir(workflowsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("reading workflows directory: %w", err)
	}

	rewrites := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) != ".yml" && filepath.Ext(name) != ".yaml" {
			continue
		}

		path := filepath.Join(workflowsDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			return rewrites, fmt.Errorf("reading %s: %w", name, err)
		}

		updated := versionTagPattern.ReplaceAllString(string(data), "${1}@"+newVersion)
		if updated != string(data) {
			if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
				return rewrites, fmt.Errorf("writing %s: %w", name, err)
			}
			rewrites++
		}
	}

	return rewrites, nil
}
