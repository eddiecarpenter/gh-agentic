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

// DefaultClone is retained as a no-op for source compatibility with
// the Clone field on the various Deps structs that wired it in.
// Production no longer consults the value — DownloadFramework dispatches
// via DetectMountState to InstallSubmodule / SwapSubmodule /
// MigrateGitignoredMount, none of which call back into the Clone func.
// The symbol will be removed in a follow-up that prunes the now-vestigial
// Clone field from the Deps shape.
func DefaultClone(repoURL, tag, destDir string) error {
	return nil
}

// ReadAIVersionFromGit reads the mounted framework version from the .git
// metadata inside .agents/. This is the authoritative source of truth — it reflects
// exactly what was cloned, not what .ai-version says.
func ReadAIVersionFromGit(root string) (string, error) {
	aiDir := filepath.Join(root, ".agents")
	cmd := exec.Command("git", "-C", aiDir, "describe", "--tags", "--exact-match")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("reading version from .agents/.git: %w", err)
	}
	v := strings.TrimSpace(string(out))
	if v == "" {
		return "", fmt.Errorf(".agents/.git has no version tag")
	}
	return v, nil
}

// DownloadFramework installs or updates the framework mount at
// destRoot/.agents/ as a tracked git submodule pointing at the framework
// repo at the given version tag. The dispatch is idempotent over five
// working-tree states (see DetectMountState):
//
//   - MountStateNone           → InstallSubmodule (fresh install)
//   - MountStateSubmodule      → SwapSubmodule (version swap)
//   - MountStateGitignoredMount → MigrateGitignoredMount (legacy → submodule)
//   - MountStateSymlink        → refused (gh-agentic itself)
//   - MountStateInconsistent   → refused (working tree is in an unsafe state)
//
// The `clone` parameter is retained on the signature for source
// compatibility with existing callers, but is no longer consulted —
// submodule operations rely on the system `git` binary acting on the
// parent repo. Tests stub the install/swap/migrate primitives via the
// package-level vars (InstallSubmodule, SwapSubmodule,
// MigrateGitignoredMount) rather than supplying a CloneFunc.
func DownloadFramework(destRoot, version string, _ CloneFunc) error {
	if version == "" {
		return fmt.Errorf("version is empty — cannot install framework")
	}

	state, err := DetectMountState(destRoot)
	if err != nil {
		return fmt.Errorf("detecting mount state: %w", err)
	}

	switch state {
	case MountStateSymlink:
		return fmt.Errorf(".agents is a symlink (this is gh-agentic itself); refusing to overwrite the framework source")
	case MountStateNone:
		return InstallSubmodule(destRoot, version)
	case MountStateSubmodule:
		return SwapSubmodule(destRoot, version)
	case MountStateGitignoredMount:
		return MigrateGitignoredMount(destRoot, version)
	case MountStateInconsistent:
		return fmt.Errorf(".agents/ exists but is neither a symlink, a tracked submodule, nor a gitignored legacy mount — working tree is inconsistent. Resolve manually before running upgrade")
	default:
		return fmt.Errorf("unknown mount state: %d", state)
	}
}

// EnsureGitignore is retained for control-plane / control-plane-mirror
// callers that still need to add a path to .gitignore. The .agents/ mount
// no longer uses it — submodules are tracked, not gitignored.
func EnsureGitignore(root string) error {
	return ensureGitignoreEntry(root, ".agents/")
}

// RemoveAIFromGitignore strips a `.agents/` line from the parent repo's
// .gitignore, if present. The doctor repair calls this to clean up the
// legacy shallow-clone state during migration to the submodule mount.
// Other lines in .gitignore are preserved verbatim. Idempotent: a
// missing `.gitignore` or a missing `.agents/` line is a no-op.
func RemoveAIFromGitignore(root string) error {
	return removeFromGitignore(root, ".agents/")
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
