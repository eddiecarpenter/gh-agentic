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
	withStubInstall(t, map[string]string{
		"RULEBOOK.md":            "# Rules",
		"skills/session-init.md": "# Session Init",
		"recipes/dev.yaml":       "recipe: dev",
		"standards/go.md":        "# Go Standards",
		"concepts/philosophy.md": "# Philosophy",
	})

	if err := DownloadFramework(root, "v2.0.0", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify framework files exist in .ai/ (the stub mimics the
	// post-install state of a real `git submodule add`).
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

	// Verify the dispatch wrote a .gitmodules entry — the durable
	// signal that this run actually went through the submodule path
	// (not a direct file-copy).
	gm, err := os.ReadFile(filepath.Join(root, ".gitmodules"))
	if err != nil {
		t.Fatalf("reading .gitmodules: %v", err)
	}
	if !strings.Contains(string(gm), `[submodule ".ai"]`) {
		t.Errorf(".gitmodules missing .ai entry: %s", gm)
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

func TestDownloadFramework_InstallError(t *testing.T) {
	root := t.TempDir()
	withStubInstallError(t, "network error")

	err := DownloadFramework(root, "v2.0.0", nil)
	if err == nil {
		t.Fatal("expected error on install failure")
	}
	if !strings.Contains(err.Error(), "network error") {
		t.Errorf("expected 'network error' in error, got: %v", err)
	}
}

func TestDownloadFramework_RefusesInconsistentExistingAI(t *testing.T) {
	// A pre-existing .ai/ that is neither a symlink, a submodule, nor a
	// gitignored legacy mount, AND that contains user-meaningful content
	// (i.e. not just an aborted-clone .git/ directory) is treated as
	// MountStateInconsistent — the dispatcher refuses rather than
	// silently overwriting.
	root := t.TempDir()
	aiDir := filepath.Join(root, ".ai")
	_ = os.MkdirAll(aiDir, 0o755)
	_ = os.WriteFile(filepath.Join(aiDir, "stale.txt"), []byte("stale"), 0o644)

	err := DownloadFramework(root, "v2.0.0", nil)
	if err == nil {
		t.Fatal("expected refusal on inconsistent state")
	}
	if !strings.Contains(err.Error(), "inconsistent") {
		t.Errorf("error should mention inconsistent state, got: %v", err)
	}
	// Stale file must remain — refusal must not modify the working tree.
	if _, err := os.Stat(filepath.Join(aiDir, "stale.txt")); err != nil {
		t.Error("expected stale.txt to remain after refusal")
	}
}

// The .ai-version flat-file readers and writer were removed in #585 —
// the mounted version lives in .ai/.git metadata, which ReadAIVersionFromGit
// reads directly. No unit test is kept for the deleted helpers.

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
