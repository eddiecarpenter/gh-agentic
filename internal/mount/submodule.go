package mount

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// MountState describes the working-tree state of `.agents/` in a domain repo
// at the time `gh agentic upgrade` is invoked. It is the input to the
// idempotent install/swap dispatch in DownloadFramework.
type MountState int

const (
	// MountStateNone means `.agents/` does not exist and `.gitmodules` has no
	// entry for it — the fresh-install case.
	MountStateNone MountState = iota
	// MountStateSymlink means `.agents` is a symlink (i.e. this is gh-agentic
	// itself, where `.ai -> .`). Upgrade refuses this case to preserve the
	// symlink — see refuseIfFrameworkSource at the CLI layer.
	MountStateSymlink
	// MountStateSubmodule means `.agents/` is already a tracked git submodule
	// recorded in `.gitmodules`. Upgrade swaps it to the new version.
	MountStateSubmodule
	// MountStateGitignoredMount means `.agents/` exists as a directory and is
	// listed in `.gitignore` (the legacy shallow-clone state). Upgrade
	// auto-migrates it to a submodule.
	MountStateGitignoredMount
	// MountStateInconsistent means the working tree is in a state the
	// dispatcher can't safely auto-recover from (e.g. `.agents/` exists but is
	// neither a submodule nor gitignored). Upgrade surfaces an error.
	MountStateInconsistent
)

// DetectMountState inspects the working tree at root and classifies the
// state of `.agents/`. The classification drives DownloadFramework's dispatch
// to InstallSubmodule / SwapSubmodule / MigrateGitignoredMount.
func DetectMountState(root string) (MountState, error) {
	aiPath := filepath.Join(root, ".agents")

	info, err := os.Lstat(aiPath)
	aiExists := err == nil
	aiIsSymlink := aiExists && info.Mode()&os.ModeSymlink != 0
	aiIsDir := aiExists && info.IsDir()

	if aiIsSymlink {
		return MountStateSymlink, nil
	}

	hasSubmoduleEntry, err := gitmodulesHasAI(root)
	if err != nil {
		return 0, fmt.Errorf("reading .gitmodules: %w", err)
	}

	if hasSubmoduleEntry {
		return MountStateSubmodule, nil
	}

	if !aiExists {
		return MountStateNone, nil
	}

	// `.agents` exists, is not a symlink, and there is no submodule entry.
	// If it's a directory and `.agents/` is listed in `.gitignore`, this is
	// the legacy shallow-clone state — eligible for auto-migration.
	if aiIsDir {
		gitignored, err := gitignoreContains(root, ".agents/")
		if err != nil {
			return 0, fmt.Errorf("reading .gitignore: %w", err)
		}
		if gitignored {
			return MountStateGitignoredMount, nil
		}

		// Empty .agents/ (or one containing only a stale .git/) is treated
		// as MountStateNone so the caller can recover from a previous
		// failed install. The fresh-install path defensively cleans
		// `.agents/` and `.git/modules/.agents/` before adding the submodule.
		if isEmptyOrAbortedClone(aiPath) {
			return MountStateNone, nil
		}
	}

	return MountStateInconsistent, nil
}

// isEmptyOrAbortedClone reports true when `.agents/` exists but contains no
// user-meaningful content beyond a `.git` directory left over from a
// previous failed `git submodule add` (the partial-clone state). Used
// by DetectMountState to classify aborted installs as MountStateNone
// so the install path can recover cleanly.
func isEmptyOrAbortedClone(aiPath string) bool {
	entries, err := os.ReadDir(aiPath)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.Name() == ".git" {
			continue
		}
		return false
	}
	return true
}

// gitmodulesHasAI returns true when `.gitmodules` at root contains a
// `[submodule ".agents"]` entry. Returns false (no error) when `.gitmodules`
// is missing or unreadable in the no-such-file sense.
func gitmodulesHasAI(root string) (bool, error) {
	data, err := os.ReadFile(filepath.Join(root, ".gitmodules"))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return strings.Contains(string(data), `[submodule ".agents"]`), nil
}

// gitignoreContains returns true when `.gitignore` at root contains a
// line matching entry (after trimming whitespace). Returns false (no
// error) when `.gitignore` is missing.
func gitignoreContains(root, entry string) (bool, error) {
	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == entry {
			return true, nil
		}
	}
	return false, nil
}

// InstallSubmodule is the package-level seam for the fresh-install
// operation. Production binds it to installSubmoduleViaGit; tests may
// rebind it to a stub that fakes the submodule state in the test
// directory without invoking real git operations.
var InstallSubmodule = installSubmoduleViaGit

// installSubmoduleViaGit performs a fresh `git submodule add` of the
// framework at version `tag` into `.agents/` of the parent repo at root,
// then checks out the tag inside the new submodule so the gitlink
// pins to the tag's resolved commit. The resulting `.gitmodules` and
// gitlink change are staged for commit; this function does NOT create
// a commit.
//
// Pre-conditions: `.agents/` does not exist as a tracked submodule;
// `.gitmodules` has no `.agents` entry. Caller is responsible for these
// guarantees (DetectMountState returned MountStateNone). A leftover
// `.git/modules/.agents/` directory from a previous failed run is handled
// defensively below — it is removed before `git submodule add` runs.
func installSubmoduleViaGit(root, tag string) error {
	if tag == "" {
		return fmt.Errorf("tag is empty — cannot install submodule")
	}

	// Clean up any orphan submodule git-dir from a previous run that
	// failed mid-add. `git submodule add` refuses to proceed when
	// `.git/modules/<path>/` already exists (with a "found locally"
	// error), even though the parent's `.gitmodules` has no entry.
	if gitDir, err := resolveGitDir(root); err == nil {
		_ = os.RemoveAll(filepath.Join(gitDir, "modules", ".agents"))
	}

	// Clean up any orphan `.agents/` directory from a previous run that
	// failed after the clone but before the gitmodules registration.
	// DetectMountState classifies this case as MountStateNone (via
	// isEmptyOrAbortedClone), but git submodule add still refuses to
	// add into an existing path.
	_ = os.RemoveAll(filepath.Join(root, ".agents"))

	// Note on the missing -b flag: `git submodule add -b <ref>` treats
	// <ref> as a branch name. Pinning to a tag therefore must NOT use
	// -b; we clone from the default branch and then check out the tag
	// inside the submodule. The gitlink the parent records is whatever
	// HEAD points at after the checkout, which is exactly the tag's
	// commit SHA.
	if err := runGit(root, "submodule", "add", FrameworkRepoURL, ".agents"); err != nil {
		return fmt.Errorf("git submodule add: %w", err)
	}

	aiPath := filepath.Join(root, ".agents")
	if err := runGit(aiPath, "fetch", "--tags", "--quiet"); err != nil {
		return fmt.Errorf("fetching tags inside .ai: %w", err)
	}
	if err := runGit(aiPath, "checkout", "--quiet", tag); err != nil {
		return fmt.Errorf("pinning .ai to %s: %w", tag, err)
	}

	// Stage the submodule pointer change so the gitlink update is part
	// of the human's next commit.
	if err := runGit(root, "add", ".agents"); err != nil {
		return fmt.Errorf("staging .ai gitlink: %w", err)
	}

	return nil
}

// SwapSubmodule is the package-level seam for the version-swap
// operation. Production binds it to swapSubmoduleViaGit; tests may
// rebind it to a stub.
var SwapSubmodule = swapSubmoduleViaGit

// swapSubmoduleViaGit transitions `.agents/` from a submodule pinned at the
// current version to a submodule pinned at `tag`. The dance is
// deinit → rm → remove `.git/modules/.ai` → add at new tag → pin to
// the tag's resolved commit SHA. The resulting `.gitmodules` and
// gitlink change are staged for commit.
//
// Pre-conditions: `.gitmodules` has a `.agents` entry pointing at the
// framework. Caller is responsible (DetectMountState returned
// MountStateSubmodule).
func swapSubmoduleViaGit(root, tag string) error {
	if tag == "" {
		return fmt.Errorf("tag is empty — cannot swap submodule")
	}

	// `submodule deinit -f` is idempotent; tolerate "already deinited".
	_ = runGit(root, "submodule", "deinit", "-f", ".agents")

	// `git rm` removes the gitlink and clears the submodule's
	// `.gitmodules` entry. The directory itself is also removed.
	if err := runGit(root, "rm", "-f", ".agents"); err != nil {
		return fmt.Errorf("git rm .ai: %w", err)
	}

	// Clear the per-submodule git metadata so `submodule add` of the new
	// version doesn't error out on an existing module directory.
	gitDir, err := resolveGitDir(root)
	if err != nil {
		return fmt.Errorf("resolving .git dir: %w", err)
	}
	moduleDir := filepath.Join(gitDir, "modules", ".agents")
	if err := os.RemoveAll(moduleDir); err != nil {
		return fmt.Errorf("removing %s: %w", moduleDir, err)
	}

	// Re-add at the new tag using the install primitive.
	return InstallSubmodule(root, tag)
}

// MigrateGitignoredMount is the package-level seam for the legacy
// gitignored-mount migration. Production binds it to
// migrateGitignoredMountViaGit; tests may rebind it to a stub.
var MigrateGitignoredMount = migrateGitignoredMountViaGit

// migrateGitignoredMountViaGit converts a legacy shallow-clone `.agents/`
// (a directory listed in `.gitignore`) into a tracked submodule at the
// given tag. It removes the gitignored directory, removes the `.agents/`
// line from `.gitignore`, then installs the submodule. The resulting
// changes (`.gitignore`, `.gitmodules`, `.agents` gitlink) are staged for
// commit.
//
// Pre-conditions: `.agents/` exists as a directory; `.gitignore` lists
// `.agents/`; `.gitmodules` does not have a `.agents` entry. Caller is
// responsible (DetectMountState returned MountStateGitignoredMount).
func migrateGitignoredMountViaGit(root, tag string) error {
	aiPath := filepath.Join(root, ".agents")
	if err := os.RemoveAll(aiPath); err != nil {
		return fmt.Errorf("removing legacy %s: %w", aiPath, err)
	}

	if err := removeFromGitignore(root, ".agents/"); err != nil {
		return fmt.Errorf("updating .gitignore: %w", err)
	}

	if err := runGit(root, "add", ".gitignore"); err != nil {
		return fmt.Errorf("staging .gitignore: %w", err)
	}

	return InstallSubmodule(root, tag)
}

// removeFromGitignore strips the line matching entry from `.gitignore`,
// preserving every other line and the file's trailing newline behaviour.
// If the entry is not present, the file is untouched.
func removeFromGitignore(root, entry string) error {
	gitignorePath := filepath.Join(root, ".gitignore")
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	lines := strings.Split(string(data), "\n")
	out := make([]string, 0, len(lines))
	removed := false
	for _, line := range lines {
		if strings.TrimSpace(line) == entry {
			removed = true
			continue
		}
		out = append(out, line)
	}
	if !removed {
		return nil
	}

	return os.WriteFile(gitignorePath, []byte(strings.Join(out, "\n")), 0o644)
}

// resolveGitDir returns the absolute path to the `.git` directory for
// the working tree at root. Handles both the regular case (`.git` is a
// directory) and the worktree case (`.git` is a file that contains
// `gitdir: <path>`).
func resolveGitDir(root string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	gitDir := strings.TrimSpace(string(out))
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(root, gitDir)
	}
	return gitDir, nil
}
