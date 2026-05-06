package project

import (
	"os"
	"path/filepath"
	"testing"
)

// TestIsFrameworkSource covers the three observable states of .ai that
// IsFrameworkSource must distinguish: absent, directory, symlink. The
// symlink case is the only one that returns true — that is the single
// signal the CLI uses to recognise a framework-source repo.
func TestIsFrameworkSource(t *testing.T) {
	t.Run("no .ai entry at all — consumer before mount", func(t *testing.T) {
		root := t.TempDir()
		if IsFrameworkSource(root) {
			t.Fatal("expected false when .ai does not exist")
		}
	})

	t.Run(".ai is a regular directory — consumer after mount", func(t *testing.T) {
		root := t.TempDir()
		if err := os.Mkdir(filepath.Join(root, ".agents"), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if IsFrameworkSource(root) {
			t.Fatal("expected false when .ai is a directory")
		}
	})

	t.Run(".agents is a symlink — framework source", func(t *testing.T) {
		root := t.TempDir()
		// Target doesn't need to exist for the signal check — os.Lstat does
		// not follow the link. This mirrors the git-committed symlink case
		// in the framework repo where .ai → .
		if err := os.Symlink(".", filepath.Join(root, ".agents")); err != nil {
			t.Skipf("platform does not support symlinks in tempdir: %v", err)
		}
		if !IsFrameworkSource(root) {
			t.Fatal("expected true when .agents is a symlink")
		}
	})

	t.Run(".ai is a regular file — unexpected but must not false-positive", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, ".agents"), []byte("x"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if IsFrameworkSource(root) {
			t.Fatal("expected false when .ai is a regular file")
		}
	})
}
