package mount

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)


// MountState describes the working-tree state of `.ai/` in a domain repo
// at the time `gh agentic upgrade` is invoked. It is the input to the
// idempotent install/swap dispatch in DownloadFramework.
type MountState int

const (
	// MountStateNone means `.ai/` does not exist and `.gitmodules` has no
	// entry for it — the fresh-install case.
	MountStateNone MountState = iota
	// MountStateSymlink means `.ai` is a symlink (i.e. this is gh-agentic
	// itself, where `.ai -> .`). Upgrade refuses this case to preserve the
	// symlink — see refuseIfFrameworkSource at the CLI layer.
	MountStateSymlink
	// MountStateSubmodule means `.ai/` is already a tracked git submodule
	// recorded in `.gitmodules`. Upgrade swaps it to the new version.
	MountStateSubmodule
	// MountStateGitignoredMount means `.ai/` exists as a directory and is
	// listed in `.gitignore` (the legacy shallow-clone state). Upgrade
	// auto-migrates it to a submodule.
	MountStateGitignoredMount
	// MountStateInconsistent means the working tree is in a state the
	// dispatcher can't safely auto-recover from (e.g. `.ai/` exists but is
	// neither a submodule nor gitignored). Upgrade surfaces an error.
	MountStateInconsistent
)

// DetectMountState inspects the working tree at root and classifies the
// state of `.ai/`. The classification drives DownloadFramework's dispatch
// to InstallSubmodule / SwapSubmodule / MigrateGitignoredMount.
func DetectMountState(root string) (MountState, error) {
	aiPath := filepath.Join(root, ".ai")

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

	// `.ai` exists, is not a symlink, and there is no submodule entry.
	// If it's a directory and `.ai/` is listed in `.gitignore`, this is
	// the legacy shallow-clone state — eligible for auto-migration.
	if aiIsDir {
		gitignored, err := gitignoreContains(root, ".ai/")
		if err != nil {
			return 0, fmt.Errorf("reading .gitignore: %w", err)
		}
		if gitignored {
			return MountStateGitignoredMount, nil
		}
	}

	return MountStateInconsistent, nil
}

// gitmodulesHasAI returns true when `.gitmodules` at root contains a
// `[submodule ".ai"]` entry. Returns false (no error) when `.gitmodules`
// is missing or unreadable in the no-such-file sense.
func gitmodulesHasAI(root string) (bool, error) {
	data, err := os.ReadFile(filepath.Join(root, ".gitmodules"))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return strings.Contains(string(data), `[submodule ".ai"]`), nil
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
// framework at version `tag` into `.ai/` of the parent repo at root.
// The resulting `.gitmodules` and gitlink change are staged for commit;
// this function does NOT create a commit.
//
// Pre-conditions: `.ai/` does not exist; `.gitmodules` has no `.ai`
// entry. Caller is responsible for these guarantees (DetectMountState
// returned MountStateNone).
func installSubmoduleViaGit(root, tag string) error {
	if tag == "" {
		return fmt.Errorf("tag is empty — cannot install submodule")
	}

	if err := runGit(root, "submodule", "add", "-b", tag, FrameworkRepoURL, ".ai"); err != nil {
		return fmt.Errorf("git submodule add: %w", err)
	}

	// Pin the gitlink to the tag's resolved commit, not just the branch
	// tip — `submodule add -b <tag>` records the branch ref but leaves
	// the gitlink at whatever HEAD pointed to at clone time. For tags
	// (which are detached refs), this usually coincides; explicitly
	// checking out the tag inside `.ai/` makes the pinning explicit.
	aiPath := filepath.Join(root, ".ai")
	if err := runGit(aiPath, "checkout", "--quiet", tag); err != nil {
		return fmt.Errorf("pinning .ai to %s: %w", tag, err)
	}

	// Stage the submodule pointer change so the gitlink update is part
	// of the human's next commit.
	if err := runGit(root, "add", ".ai"); err != nil {
		return fmt.Errorf("staging .ai gitlink: %w", err)
	}

	return nil
}

// SwapSubmodule is the package-level seam for the version-swap
// operation. Production binds it to swapSubmoduleViaGit; tests may
// rebind it to a stub.
var SwapSubmodule = swapSubmoduleViaGit

// swapSubmoduleViaGit transitions `.ai/` from a submodule pinned at the
// current version to a submodule pinned at `tag`. The dance is
// deinit → rm → remove `.git/modules/.ai` → add at new tag → pin to
// the tag's resolved commit SHA. The resulting `.gitmodules` and
// gitlink change are staged for commit.
//
// Pre-conditions: `.gitmodules` has a `.ai` entry pointing at the
// framework. Caller is responsible (DetectMountState returned
// MountStateSubmodule).
func swapSubmoduleViaGit(root, tag string) error {
	if tag == "" {
		return fmt.Errorf("tag is empty — cannot swap submodule")
	}

	// `submodule deinit -f` is idempotent; tolerate "already deinited".
	_ = runGit(root, "submodule", "deinit", "-f", ".ai")

	// `git rm` removes the gitlink and clears the submodule's
	// `.gitmodules` entry. The directory itself is also removed.
	if err := runGit(root, "rm", "-f", ".ai"); err != nil {
		return fmt.Errorf("git rm .ai: %w", err)
	}

	// Clear the per-submodule git metadata so `submodule add` of the new
	// version doesn't error out on an existing module directory.
	gitDir, err := resolveGitDir(root)
	if err != nil {
		return fmt.Errorf("resolving .git dir: %w", err)
	}
	moduleDir := filepath.Join(gitDir, "modules", ".ai")
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

// migrateGitignoredMountViaGit converts a legacy shallow-clone `.ai/`
// (a directory listed in `.gitignore`) into a tracked submodule at the
// given tag. It removes the gitignored directory, removes the `.ai/`
// line from `.gitignore`, then installs the submodule. The resulting
// changes (`.gitignore`, `.gitmodules`, `.ai` gitlink) are staged for
// commit.
//
// Pre-conditions: `.ai/` exists as a directory; `.gitignore` lists
// `.ai/`; `.gitmodules` does not have a `.ai` entry. Caller is
// responsible (DetectMountState returned MountStateGitignoredMount).
func migrateGitignoredMountViaGit(root, tag string) error {
	aiPath := filepath.Join(root, ".ai")
	if err := os.RemoveAll(aiPath); err != nil {
		return fmt.Errorf("removing legacy %s: %w", aiPath, err)
	}

	if err := removeFromGitignore(root, ".ai/"); err != nil {
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

