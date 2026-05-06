package mount

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeCPSync writes a marker file at destDir on first call and records calls.
func fakeCPSync(t *testing.T, calls *[]string) CPSyncFunc {
	t.Helper()
	return func(cpNameWithOwner, destDir string) error {
		*calls = append(*calls, cpNameWithOwner+" -> "+destDir)
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return err
		}
		marker := filepath.Join(destDir, "marker")
		return os.WriteFile(marker, []byte(cpNameWithOwner), 0o644)
	}
}

func TestMountControlPlane_FirstTime(t *testing.T) {
	root := t.TempDir()
	var buf bytes.Buffer
	var calls []string

	err := MountControlPlane(&buf, root, "org/cp-repo", fakeCPSync(t, &calls))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(calls) != 1 {
		t.Fatalf("expected 1 sync call, got %d", len(calls))
	}
	if !strings.Contains(calls[0], "org/cp-repo") {
		t.Errorf("sync call missing repo name: %s", calls[0])
	}

	if _, err := os.Stat(filepath.Join(root, ".cp", "marker")); err != nil {
		t.Errorf(".cp/marker should exist: %v", err)
	}

	gitignore, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	if !strings.Contains(string(gitignore), ".cp/") {
		t.Errorf(".gitignore should contain .cp/, got: %s", gitignore)
	}

	out := buf.String()
	if !strings.Contains(out, "Mounting control plane knowledge") {
		t.Errorf("output should mention mounting, got: %s", out)
	}
}

func TestMountControlPlane_Refresh(t *testing.T) {
	root := t.TempDir()
	// Pre-create .cp/ to simulate an existing mount.
	if err := os.MkdirAll(filepath.Join(root, ".cp"), 0o755); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	var calls []string

	if err := MountControlPlane(&buf, root, "org/cp-repo", fakeCPSync(t, &calls)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Refreshing control plane knowledge") {
		t.Errorf("output should mention refreshing for existing .cp/, got: %s", out)
	}

	if len(calls) != 1 {
		t.Fatalf("expected 1 sync call, got %d", len(calls))
	}
}

func TestMountControlPlane_SyncFailureIsNonFatal(t *testing.T) {
	root := t.TempDir()
	var buf bytes.Buffer

	failing := func(cpNameWithOwner, destDir string) error {
		return fmt.Errorf("network unreachable")
	}

	if err := MountControlPlane(&buf, root, "org/cp-repo", failing); err != nil {
		t.Fatalf("sync failure should be non-fatal, got error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "⚠") || !strings.Contains(out, "network unreachable") {
		t.Errorf("output should warn about sync failure, got: %s", out)
	}
}

func TestEnsureCPGitignore_AddsEntry(t *testing.T) {
	root := t.TempDir()

	if err := EnsureCPGitignore(root); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	if !strings.Contains(string(data), ".cp/") {
		t.Errorf(".gitignore should contain .cp/, got: %s", data)
	}
}

func TestEnsureCPGitignore_Idempotent(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".gitignore")

	if err := os.WriteFile(path, []byte(".agents/\n.cp/\nother\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := EnsureCPGitignore(root); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(path)
	if count := strings.Count(string(data), ".cp/"); count != 1 {
		t.Errorf("expected exactly one .cp/ entry, got %d: %s", count, data)
	}
}

func TestEnsureCPGitignore_CoexistsWithAIEntry(t *testing.T) {
	root := t.TempDir()

	if err := EnsureGitignore(root); err != nil {
		t.Fatal(err)
	}
	if err := EnsureCPGitignore(root); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	s := string(data)
	if !strings.Contains(s, ".agents/") || !strings.Contains(s, ".cp/") {
		t.Errorf(".gitignore should contain both entries, got: %s", s)
	}
}

func TestDefaultCPSync_EmptyRepoErrors(t *testing.T) {
	if err := DefaultCPSync("", t.TempDir()); err == nil {
		t.Fatal("expected error for empty cpNameWithOwner")
	}
}
