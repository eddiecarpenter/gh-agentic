package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRefuseIfFrameworkSource covers the helper every guarded command
// calls at entry. When .ai is a symlink, the helper produces the
// canonical refusal error; otherwise it returns nil so the command
// proceeds normally.
func TestRefuseIfFrameworkSource(t *testing.T) {
	t.Run("not a framework source — returns nil", func(t *testing.T) {
		root := t.TempDir()
		if err := refuseIfFrameworkSource(nil, root, "mount"); err != nil {
			t.Fatalf("expected nil, got: %v", err)
		}
	})

	t.Run("framework source — returns canonical refusal", func(t *testing.T) {
		root := t.TempDir()
		if err := os.Symlink(".", filepath.Join(root, ".ai")); err != nil {
			t.Skipf("platform does not support symlinks in tempdir: %v", err)
		}
		err := refuseIfFrameworkSource(nil, root, "mount")
		if err == nil {
			t.Fatal("expected refusal error, got nil")
		}
		msg := err.Error()
		// Shape checks — the message must name the detection signal,
		// the command that was refused, and the commands that ARE
		// supported so the user has a path forward.
		if !strings.Contains(msg, "symlink") {
			t.Errorf("error message should name the symlink detection signal, got: %s", msg)
		}
		if !strings.Contains(msg, "framework source") {
			t.Errorf("error message should name the framework-source diagnosis, got: %s", msg)
		}
		if !strings.Contains(msg, "mount") {
			t.Errorf("error message should name the refused command, got: %s", msg)
		}
		if !strings.Contains(msg, "status") || !strings.Contains(msg, "info") {
			t.Errorf("error message should list supported commands (status, info, ...), got: %s", msg)
		}
	})
}
