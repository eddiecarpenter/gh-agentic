package mount

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

)

// fakeClone returns a CloneFunc that creates the given files in destDir,
// simulating a shallow git clone of the framework.
func fakeClone(files map[string]string) CloneFunc {
	return func(repoURL, tag, destDir string) error {
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return err
		}
		for path, content := range files {
			full := filepath.Join(destDir, path)
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
				return err
			}
		}
		return nil
	}
}

// fakeCloneError returns a CloneFunc that always returns an error.
func fakeCloneError(errMsg string) CloneFunc {
	return func(repoURL, tag, destDir string) error {
		return fmt.Errorf("%s", errMsg)
	}
}

func sampleReleases() []Release {
	return []Release{
		{TagName: "v2.1.0", Name: "v2.1.0", Body: "Latest", TarballURL: "https://example.com/v2.1.0.tar.gz"},
		{TagName: "v2.0.0", Name: "v2.0.0", Body: "Stable", TarballURL: "https://example.com/v2.0.0.tar.gz"},
		{TagName: "v1.5.0", Name: "v1.5.0", Body: "Old", TarballURL: "https://example.com/v1.5.0.tar.gz"},
	}
}

func TestValidateTag_ValidTag(t *testing.T) {
	releases := sampleReleases()
	err := ValidateTag("v2.0.0", releases)
	if err != nil {
		t.Errorf("expected nil error for valid tag, got: %v", err)
	}
}

func TestValidateTag_InvalidTag(t *testing.T) {
	releases := sampleReleases()
	err := ValidateTag("v9.9.9", releases)
	if err == nil {
		t.Fatal("expected error for invalid tag")
	}
	if !strings.Contains(err.Error(), "v9.9.9 not found") {
		t.Errorf("error should mention the invalid tag, got: %v", err)
	}
	if !strings.Contains(err.Error(), "v2.1.0") {
		t.Errorf("error should mention latest available version, got: %v", err)
	}
}

func TestValidateTag_EmptyReleases(t *testing.T) {
	err := ValidateTag("v1.0.0", nil)
	if err == nil {
		t.Fatal("expected error when no releases available")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Errorf("error should mention 'unknown' when no releases, got: %v", err)
	}
}

func TestDownloadFramework_Success(t *testing.T) {
	root := t.TempDir()

	clone := fakeClone(map[string]string{
		"RULEBOOK.md":            "# Rules",
		"skills/session-init.md": "# Session Init",
		"recipes/dev.yaml":       "recipe: dev",
		"standards/go.md":        "# Go Standards",
		"concepts/philosophy.md": "# Philosophy",
		"cmd/gh-agentic/main.go": "package main", // Also cloned — all files land in .ai/
		"internal/cli/root.go":   "package cli",
	})

	err := DownloadFramework(root, "v2.0.0", clone)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify framework files exist in .ai/.
	expectedFiles := []string{
		".ai/RULEBOOK.md",
		".ai/skills/session-init.md",
		".ai/recipes/dev.yaml",
		".ai/standards/go.md",
		".ai/concepts/philosophy.md",
	}
	for _, f := range expectedFiles {
		path := filepath.Join(root, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected %s to exist", f)
		}
	}

	// Verify content.
	data, err := os.ReadFile(filepath.Join(root, ".ai/RULEBOOK.md"))
	if err != nil {
		t.Fatalf("reading RULEBOOK.md: %v", err)
	}
	if string(data) != "# Rules" {
		t.Errorf("unexpected RULEBOOK.md content: %s", data)
	}
}

func TestDownloadFramework_EmptyVersion(t *testing.T) {
	root := t.TempDir()
	err := DownloadFramework(root, "", nil)
	if err == nil {
		t.Fatal("expected error for empty version")
	}
	if !strings.Contains(err.Error(), "version is empty") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDownloadFramework_CloneError(t *testing.T) {
	root := t.TempDir()
	err := DownloadFramework(root, "v2.0.0", fakeCloneError("network error"))
	if err == nil {
		t.Fatal("expected error on clone failure")
	}
	if !strings.Contains(err.Error(), "network error") {
		t.Errorf("expected 'network error' in error, got: %v", err)
	}
}

func TestDownloadFramework_CleansExistingAI(t *testing.T) {
	root := t.TempDir()

	// Create existing .ai/ with a stale file.
	aiDir := filepath.Join(root, ".ai")
	_ = os.MkdirAll(aiDir, 0o755)
	_ = os.WriteFile(filepath.Join(aiDir, "stale.txt"), []byte("stale"), 0o644)

	clone := fakeClone(map[string]string{
		"RULEBOOK.md": "# Fresh Rules",
	})

	err := DownloadFramework(root, "v2.0.0", clone)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Stale file should be gone.
	if _, err := os.Stat(filepath.Join(aiDir, "stale.txt")); err == nil {
		t.Error("expected stale.txt to be removed")
	}

	// Fresh file should exist.
	if _, err := os.Stat(filepath.Join(aiDir, "RULEBOOK.md")); os.IsNotExist(err) {
		t.Error("expected RULEBOOK.md to exist")
	}
}

func TestReadAIVersion_Exists(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, ".ai-version"), []byte("v2.0.0\n"), 0o644)

	v, err := ReadAIVersion(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "v2.0.0" {
		t.Errorf("expected v2.0.0, got %q", v)
	}
}

func TestReadAIVersion_NotExists(t *testing.T) {
	root := t.TempDir()
	_, err := ReadAIVersion(root)
	if err == nil {
		t.Fatal("expected error when .ai-version does not exist")
	}
}

func TestReadAIVersion_Empty(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, ".ai-version"), []byte("  \n"), 0o644)

	_, err := ReadAIVersion(root)
	if err == nil {
		t.Fatal("expected error for empty .ai-version")
	}
}

func TestWriteAIVersion(t *testing.T) {
	root := t.TempDir()
	err := WriteAIVersion(root, "v2.1.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".ai-version"))
	if err != nil {
		t.Fatalf("reading .ai-version: %v", err)
	}
	if strings.TrimSpace(string(data)) != "v2.1.0" {
		t.Errorf("expected v2.1.0, got %q", string(data))
	}
}

func TestEnsureGitignore_Creates(t *testing.T) {
	root := t.TempDir()
	err := EnsureGitignore(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatalf("reading .gitignore: %v", err)
	}
	if !strings.Contains(string(data), ".ai/") {
		t.Errorf("expected .ai/ in .gitignore, got: %s", data)
	}
}

func TestEnsureGitignore_AlreadyPresent(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, ".gitignore"), []byte("node_modules/\n.ai/\n"), 0o644)

	err := EnsureGitignore(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	count := strings.Count(string(data), ".ai/")
	if count != 1 {
		t.Errorf("expected exactly 1 .ai/ entry, got %d in: %s", count, data)
	}
}

func TestEnsureGitignore_Appends(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, ".gitignore"), []byte("node_modules/\n"), 0o644)

	err := EnsureGitignore(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	if !strings.Contains(string(data), "node_modules/") {
		t.Error("expected existing entries to be preserved")
	}
	if !strings.Contains(string(data), ".ai/") {
		t.Errorf("expected .ai/ to be appended, got: %s", data)
	}
}

func TestEnsureGitignore_NoTrailingNewline(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, ".gitignore"), []byte("node_modules/"), 0o644)

	err := EnsureGitignore(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	content := string(data)
	if !strings.Contains(content, "node_modules/") {
		t.Error("expected existing entries preserved")
	}
	if !strings.Contains(content, "\n.ai/") {
		t.Errorf("expected newline before .ai/ entry, got: %q", content)
	}
}
