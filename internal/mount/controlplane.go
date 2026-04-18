package mount

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CPMountDir is the directory under the repo root where the control plane
// knowledge is mounted.
const CPMountDir = ".cp"

// CPBranch is the branch tracked by the .cp/ mount. Knowledge updates on the
// control plane propagate to every domain repo as soon as they merge to main,
// so the mount always tracks main rather than a pinned version.
const CPBranch = "main"

// CPSparsePath is the directory inside the control plane repo whose contents
// populate the .cp/ mount.
const CPSparsePath = "docs"

// CPSyncFunc syncs the control plane knowledge mount at destDir from the
// repo identified by cpNameWithOwner (owner/repo). When destDir does not
// exist it performs the initial sparse clone; otherwise it refreshes in place.
type CPSyncFunc func(cpNameWithOwner, destDir string) error

// DefaultCPSync is the production CPSyncFunc. It clones docs/ from the CP
// repo via a filtered sparse checkout on first run, and fast-forwards main
// on subsequent runs.
func DefaultCPSync(cpNameWithOwner, destDir string) error {
	if cpNameWithOwner == "" {
		return fmt.Errorf("control plane repo is empty")
	}

	if _, err := os.Stat(destDir); os.IsNotExist(err) {
		return sparseCloneCP(cpNameWithOwner, destDir)
	} else if err != nil {
		return fmt.Errorf("stat %s: %w", destDir, err)
	}

	return pullCP(destDir)
}

// sparseCloneCP creates a new sparse checkout of docs/ at destDir.
func sparseCloneCP(cpNameWithOwner, destDir string) error {
	url := "https://github.com/" + cpNameWithOwner + ".git"
	cmd := exec.Command("git", "clone",
		"--depth", "1",
		"--branch", CPBranch,
		"--filter=blob:none",
		"--sparse",
		url, destDir,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone failed: %w\n%s", err, strings.TrimSpace(string(out)))
	}

	if err := runGit(destDir, "sparse-checkout", "set", CPSparsePath); err != nil {
		return fmt.Errorf("git sparse-checkout set %s: %w", CPSparsePath, err)
	}
	return nil
}

// pullCP refreshes an existing .cp/ checkout by fast-forwarding main.
func pullCP(destDir string) error {
	return runGit(destDir, "pull", "--ff-only", "origin", CPBranch)
}

func runGit(dir string, args ...string) error {
	full := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", full...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git %s failed: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// MountControlPlane establishes or refreshes the .cp/ mount for a federated
// domain repo. Network failures are reported as warnings and do not fail the
// enclosing mount — sessions remain usable offline with slightly-stale CP
// knowledge.
func MountControlPlane(w io.Writer, root, cpNameWithOwner string, sync CPSyncFunc) error {
	if sync == nil {
		sync = DefaultCPSync
	}

	destDir := filepath.Join(root, CPMountDir)
	existed := true
	if _, err := os.Stat(destDir); os.IsNotExist(err) {
		existed = false
	}

	if existed {
		fmt.Fprintf(w, "  Refreshing control plane knowledge (%s) at .cp/\n", cpNameWithOwner)
	} else {
		fmt.Fprintf(w, "  Mounting control plane knowledge (%s) at .cp/\n", cpNameWithOwner)
	}

	if err := sync(cpNameWithOwner, destDir); err != nil {
		fmt.Fprintf(w, "  ⚠  .cp/ sync failed — proceeding with existing state: %v\n", err)
		return nil
	}

	if err := EnsureCPGitignore(root); err != nil {
		return fmt.Errorf("updating .gitignore: %w", err)
	}

	fmt.Fprintln(w, "  ✓ Control plane knowledge mounted at .cp/")
	return nil
}

// EnsureCPGitignore ensures that ".cp/" is listed in .gitignore at root.
func EnsureCPGitignore(root string) error {
	return ensureGitignoreEntry(root, ".cp/")
}
