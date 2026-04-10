package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewFakeRepo_CreatesStandardFiles(t *testing.T) {
	repo := NewFakeRepo(t)

	expectedFiles := []string{
		".ai/config.yml",
		".ai/RULEBOOK.md",
		"CLAUDE.md",
		"AGENTS.md",
		"LOCALRULES.md",
		"REPOS.md",
		"README.md",
	}

	for _, f := range expectedFiles {
		full := filepath.Join(repo.Root, f)
		if _, err := os.Stat(full); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", f)
		}
	}
}

func TestNewFakeRepo_IsGitRepo(t *testing.T) {
	repo := NewFakeRepo(t)

	// Check that .git directory exists.
	gitDir := filepath.Join(repo.Root, ".git")
	info, err := os.Stat(gitDir)
	if err != nil {
		t.Fatalf("expected .git directory: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected .git to be a directory")
	}
}

func TestNewFakeRepo_HasAtLeastOneCommit(t *testing.T) {
	repo := NewFakeRepo(t)

	cmd := exec.Command("git", "log", "--oneline")
	cmd.Dir = repo.Root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git log failed: %v\n%s", err, out)
	}
	if len(out) == 0 {
		t.Fatal("expected at least one commit")
	}
}

func TestNewFakeRepo_ConfigYMLContent(t *testing.T) {
	repo := NewFakeRepo(t)

	content, err := os.ReadFile(filepath.Join(repo.Root, ".ai", "config.yml"))
	if err != nil {
		t.Fatalf("read .ai/config.yml: %v", err)
	}
	if !strings.Contains(string(content), "eddiecarpenter/ai-native-delivery") {
		t.Fatalf("unexpected .ai/config.yml content: %q", string(content))
	}
	if !strings.Contains(string(content), "v1.0.0") {
		t.Fatalf(".ai/config.yml missing version: %q", string(content))
	}
}

func TestFakeRepo_Write_CreatesFile(t *testing.T) {
	repo := NewFakeRepo(t)

	repo.Write("new-file.txt", "hello")

	content, err := os.ReadFile(filepath.Join(repo.Root, "new-file.txt"))
	if err != nil {
		t.Fatalf("read new-file.txt: %v", err)
	}
	if string(content) != "hello" {
		t.Fatalf("expected %q, got %q", "hello", string(content))
	}
}

func TestFakeRepo_Write_CreatesNestedFile(t *testing.T) {
	repo := NewFakeRepo(t)

	repo.Write("deep/nested/dir/file.txt", "nested content")

	content, err := os.ReadFile(filepath.Join(repo.Root, "deep/nested/dir/file.txt"))
	if err != nil {
		t.Fatalf("read nested file: %v", err)
	}
	if string(content) != "nested content" {
		t.Fatalf("expected %q, got %q", "nested content", string(content))
	}
}

func TestFakeRepo_Remove_DeletesFile(t *testing.T) {
	repo := NewFakeRepo(t)

	// Write then remove.
	repo.Write("temp.txt", "temporary")
	repo.Remove("temp.txt")

	if _, err := os.Stat(filepath.Join(repo.Root, "temp.txt")); !os.IsNotExist(err) {
		t.Fatal("expected file to be removed")
	}
}

func TestFakeRepo_Cleanup_RemovesDirectory(t *testing.T) {
	// We create a separate sub-test so the t.TempDir cleanup doesn't
	// interfere with our assertion. We call Cleanup() explicitly and
	// check the directory is gone.
	root := ""
	t.Run("create and cleanup", func(t *testing.T) {
		repo := NewFakeRepo(t)
		root = repo.Root
		repo.Cleanup()
	})

	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("expected directory to be removed after Cleanup")
	}
}

func TestFakeRepo_Cleanup_Idempotent(t *testing.T) {
	repo := NewFakeRepo(t)

	// Calling Cleanup multiple times should not panic.
	repo.Cleanup()
	repo.Cleanup()
}
