package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

// FakeRepo is a minimal agentic repo in a temp directory, suitable for
// integration tests. It includes standard agentic files (TEMPLATE_SOURCE,
// TEMPLATE_VERSION, CLAUDE.md, etc.) and is initialised as a git repository
// with at least one commit.
type FakeRepo struct {
	// Root is the absolute path to the temp directory.
	Root string
	// T is the test that owns this fake repo.
	T *testing.T

	cleanupOnce sync.Once
}

// NewFakeRepo creates a minimal agentic repo in a temp directory. The repo
// contains standard agentic files with sensible defaults and is initialised
// as a git repository with an initial commit. Cleanup is registered
// automatically via t.Cleanup.
func NewFakeRepo(t *testing.T) *FakeRepo {
	t.Helper()

	root := t.TempDir()

	r := &FakeRepo{
		Root: root,
		T:    t,
	}

	// Write standard agentic repo files.
	files := map[string]string{
		"TEMPLATE_SOURCE": "eddiecarpenter/agentic-development",
		"TEMPLATE_VERSION": "v1.0.0",
		"CLAUDE.md":        "# CLAUDE.md\n",
		"AGENTS.local.md":  "# AGENTS.local.md\n",
		"REPOS.md":         "# REPOS.md\n",
		"README.md":        "# README\n",
		"base/AGENTS.md":   "# AGENTS.md\n",
	}

	for path, content := range files {
		full := filepath.Join(root, path)
		dir := filepath.Dir(full)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("FakeRepo: create dir %s: %v", dir, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("FakeRepo: write %s: %v", path, err)
		}
	}

	// Initialise git repo.
	gitCmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "add", "."},
		{"git", "commit", "-m", "initial"},
	}
	for _, args := range gitCmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("FakeRepo: %v failed: %v\n%s", args, err, out)
		}
	}

	t.Cleanup(r.Cleanup)
	return r
}

// Write creates a file at the given relative path inside the repo with the
// given content. Parent directories are created as needed.
func (r *FakeRepo) Write(path, content string) {
	r.T.Helper()

	full := filepath.Join(r.Root, path)
	dir := filepath.Dir(full)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		r.T.Fatalf("FakeRepo.Write: create dir %s: %v", dir, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		r.T.Fatalf("FakeRepo.Write: write %s: %v", path, err)
	}
}

// Remove deletes the file at the given relative path inside the repo.
func (r *FakeRepo) Remove(path string) {
	r.T.Helper()

	full := filepath.Join(r.Root, path)
	if err := os.Remove(full); err != nil {
		r.T.Fatalf("FakeRepo.Remove: remove %s: %v", path, err)
	}
}

// Cleanup removes the temp directory. It is idempotent and safe to call
// multiple times. It is registered automatically via t.Cleanup when the
// FakeRepo is created.
func (r *FakeRepo) Cleanup() {
	r.cleanupOnce.Do(func() {
		// t.TempDir() handles cleanup automatically, but we provide this
		// method for explicit cleanup if needed. RemoveAll is safe even if
		// the directory has already been removed.
		os.RemoveAll(r.Root)
	})
}
