package sync

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeRun returns a RunCommandFunc that records calls and returns preset output.
func fakeRun(output string, err error) func(name string, args ...string) (string, error) {
	return func(name string, args ...string) (string, error) {
		return output, err
	}
}

// fakeRunMulti returns a RunCommandFunc that returns different outputs based on call count.
func fakeRunMulti(responses []struct {
	output string
	err    error
}) func(name string, args ...string) (string, error) {
	idx := 0
	return func(name string, args ...string) (string, error) {
		if idx >= len(responses) {
			return "", fmt.Errorf("unexpected call %d", idx)
		}
		r := responses[idx]
		idx++
		return r.output, r.err
	}
}

func TestDisplayReleaseNotes(t *testing.T) {
	t.Run("displays tag and body", func(t *testing.T) {
		var buf bytes.Buffer
		release := Release{
			TagName: "v0.9.8",
			Name:    "Fix sync runner edge case",
			Body:    "Fixed a bug in the sync runner.\nMultiple lines of notes.",
		}
		DisplayReleaseNotes(&buf, release)

		output := buf.String()
		if !strings.Contains(output, "v0.9.8") {
			t.Errorf("expected tag in output, got: %s", output)
		}
		if !strings.Contains(output, "Fixed a bug") {
			t.Errorf("expected body in output, got: %s", output)
		}
		if !strings.Contains(output, "Multiple lines") {
			t.Errorf("expected multi-line body in output, got: %s", output)
		}
	})

	t.Run("handles empty body", func(t *testing.T) {
		var buf bytes.Buffer
		release := Release{
			TagName: "v0.9.7",
			Name:    "No notes",
			Body:    "",
		}
		DisplayReleaseNotes(&buf, release)

		output := buf.String()
		if !strings.Contains(output, "v0.9.7") {
			t.Errorf("expected tag in output, got: %s", output)
		}
		if !strings.Contains(output, "No release notes available") {
			t.Errorf("expected 'No release notes available' message, got: %s", output)
		}
	})

	t.Run("handles whitespace-only body", func(t *testing.T) {
		var buf bytes.Buffer
		release := Release{
			TagName: "v0.9.6",
			Name:    "Whitespace",
			Body:    "   \n  \n  ",
		}
		DisplayReleaseNotes(&buf, release)

		output := buf.String()
		if !strings.Contains(output, "No release notes available") {
			t.Errorf("expected 'No release notes available' for whitespace body, got: %s", output)
		}
	})
}

func TestDisplayReleaseList(t *testing.T) {
	t.Run("displays multiple releases with notes", func(t *testing.T) {
		var buf bytes.Buffer
		releases := []Release{
			{TagName: "v0.9.8", Name: "Fix sync runner", Body: "Fixed the sync runner bug."},
			{TagName: "v0.9.7", Name: "Add guards", Body: "Added execution guards."},
		}
		DisplayReleaseList(&buf, releases)

		output := buf.String()
		if !strings.Contains(output, "v0.9.8") {
			t.Errorf("expected v0.9.8 in output, got: %s", output)
		}
		if !strings.Contains(output, "v0.9.7") {
			t.Errorf("expected v0.9.7 in output, got: %s", output)
		}
		if !strings.Contains(output, "Fixed the sync runner") {
			t.Errorf("expected release notes in output, got: %s", output)
		}
		if !strings.Contains(output, "Added execution guards") {
			t.Errorf("expected release notes in output, got: %s", output)
		}
	})

	t.Run("displays release name alongside tag", func(t *testing.T) {
		var buf bytes.Buffer
		releases := []Release{
			{TagName: "v1.0.0", Name: "First stable", Body: "Notes"},
		}
		DisplayReleaseList(&buf, releases)

		output := buf.String()
		if !strings.Contains(output, "First stable") {
			t.Errorf("expected release name in output, got: %s", output)
		}
	})
}

func TestFetchAndExtractTemplate(t *testing.T) {
	t.Run("success extracts template files", func(t *testing.T) {
		orig := fetchTarballFn
		defer func() { fetchTarballFn = orig }()

		destRoot := t.TempDir()
		fetchTarballFn = createFakeTarballFetch(t, map[string]string{
			"base/AGENTS.md":                    "# Agents",
			"base/standards/go.md":              "Go standards",
			".github/workflows/ci.yml":          "ci workflow",
			".goose/recipes/dev.yaml":           "dev recipe",
		})

		err := FetchAndExtractTemplate("https://api.github.com/repos/owner/repo/tarball/v1.0.0", "owner/repo", "v1.0.0", destRoot)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify extracted files.
		for _, check := range []struct {
			path    string
			content string
		}{
			{"base/AGENTS.md", "# Agents"},
			{"base/standards/go.md", "Go standards"},
			{".github/workflows/ci.yml", "ci workflow"},
			{".goose/recipes/dev.yaml", "dev recipe"},
		} {
			data, err := os.ReadFile(filepath.Join(destRoot, check.path))
			if err != nil {
				t.Errorf("expected file %s to exist: %v", check.path, err)
				continue
			}
			if string(data) != check.content {
				t.Errorf("%s content = %q, want %q", check.path, string(data), check.content)
			}
		}
	})

	t.Run("empty tarball URL returns error before fetch", func(t *testing.T) {
		orig := fetchTarballFn
		defer func() { fetchTarballFn = orig }()

		fetchCalled := false
		fetchTarballFn = func(_, _ string) (io.ReadCloser, error) {
			fetchCalled = true
			return nil, fmt.Errorf("should not be called")
		}

		err := FetchAndExtractTemplate("", "owner/repo", "v1.0.0", t.TempDir())
		if err == nil {
			t.Fatal("expected error for empty tarball URL")
		}
		if !strings.Contains(err.Error(), "tarball URL is empty") {
			t.Errorf("error should mention empty tarball URL: %v", err)
		}
		if fetchCalled {
			t.Error("fetch function should not have been called for empty tarball URL")
		}
	})

	t.Run("fetch failure returns descriptive error and leaves repo unchanged", func(t *testing.T) {
		orig := fetchTarballFn
		defer func() { fetchTarballFn = orig }()

		destRoot := t.TempDir()

		// Create a pre-existing file to verify it's not modified.
		existingFile := filepath.Join(destRoot, "existing.txt")
		if err := os.WriteFile(existingFile, []byte("original"), 0o644); err != nil {
			t.Fatal(err)
		}

		fetchTarballFn = func(_, _ string) (io.ReadCloser, error) {
			return nil, fmt.Errorf("network timeout")
		}

		err := FetchAndExtractTemplate("https://api.github.com/repos/owner/repo/tarball/v1.0.0", "owner/repo", "v1.0.0", destRoot)
		if err == nil {
			t.Fatal("expected error for fetch failure")
		}
		if !strings.Contains(err.Error(), "network timeout") {
			t.Errorf("error should contain fetch error detail: %v", err)
		}

		// Verify pre-existing file is unchanged.
		data, readErr := os.ReadFile(existingFile)
		if readErr != nil {
			t.Fatalf("pre-existing file should still exist: %v", readErr)
		}
		if string(data) != "original" {
			t.Errorf("pre-existing file content changed: %q", string(data))
		}
	})
}

func TestBackupBase(t *testing.T) {
	t.Run("existing base directory", func(t *testing.T) {
		root := t.TempDir()
		baseDir := filepath.Join(root, "base")
		if err := os.MkdirAll(filepath.Join(baseDir, "standards"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(baseDir, "AGENTS.md"), []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(baseDir, "standards", "go.md"), []byte("go standards"), 0o644); err != nil {
			t.Fatal(err)
		}

		backupDir, err := BackupBase(root)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer os.RemoveAll(backupDir)

		if backupDir == "" {
			t.Fatal("expected non-empty backup dir")
		}

		// Verify files were copied.
		data, err := os.ReadFile(filepath.Join(backupDir, "base", "AGENTS.md"))
		if err != nil {
			t.Fatalf("backup file missing: %v", err)
		}
		if string(data) != "content" {
			t.Errorf("backup content mismatch: %s", data)
		}

		data, err = os.ReadFile(filepath.Join(backupDir, "base", "standards", "go.md"))
		if err != nil {
			t.Fatalf("nested backup file missing: %v", err)
		}
		if string(data) != "go standards" {
			t.Errorf("nested backup content mismatch: %s", data)
		}
	})

	t.Run("no base directory", func(t *testing.T) {
		root := t.TempDir()
		backupDir, err := BackupBase(root)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if backupDir != "" {
			t.Errorf("expected empty backup dir, got %q", backupDir)
		}
	})
}

func TestBackupBase_WithWorkflows(t *testing.T) {
	t.Run("backs up both base and workflows", func(t *testing.T) {
		root := t.TempDir()
		// Create base/.
		if err := os.MkdirAll(filepath.Join(root, "base"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "base", "AGENTS.md"), []byte("agents"), 0o644); err != nil {
			t.Fatal(err)
		}
		// Create .github/workflows/.
		wfDir := filepath.Join(root, ".github", "workflows")
		if err := os.MkdirAll(wfDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(wfDir, "pipeline.yml"), []byte("pipeline"), 0o644); err != nil {
			t.Fatal(err)
		}

		backupDir, err := BackupBase(root)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer os.RemoveAll(backupDir)

		// Verify base backup.
		data, err := os.ReadFile(filepath.Join(backupDir, "base", "AGENTS.md"))
		if err != nil {
			t.Fatalf("base backup missing: %v", err)
		}
		if string(data) != "agents" {
			t.Errorf("base content mismatch: %s", data)
		}

		// Verify workflows backup.
		data, err = os.ReadFile(filepath.Join(backupDir, "workflows", "pipeline.yml"))
		if err != nil {
			t.Fatalf("workflows backup missing: %v", err)
		}
		if string(data) != "pipeline" {
			t.Errorf("workflow content mismatch: %s", data)
		}
	})

	t.Run("no workflows directory is not an error", func(t *testing.T) {
		root := t.TempDir()
		// Create only base/.
		if err := os.MkdirAll(filepath.Join(root, "base"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "base", "AGENTS.md"), []byte("agents"), 0o644); err != nil {
			t.Fatal(err)
		}

		backupDir, err := BackupBase(root)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer os.RemoveAll(backupDir)

		if backupDir == "" {
			t.Fatal("expected non-empty backup dir")
		}

		// Verify workflows backup dir does NOT exist.
		if _, err := os.Stat(filepath.Join(backupDir, "workflows")); !os.IsNotExist(err) {
			t.Error("expected no workflows backup when dir is absent")
		}
	})

	t.Run("only workflows exist returns backup", func(t *testing.T) {
		root := t.TempDir()
		wfDir := filepath.Join(root, ".github", "workflows")
		if err := os.MkdirAll(wfDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(wfDir, "ci.yml"), []byte("ci"), 0o644); err != nil {
			t.Fatal(err)
		}

		backupDir, err := BackupBase(root)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer os.RemoveAll(backupDir)

		if backupDir == "" {
			t.Fatal("expected non-empty backup dir when only workflows exist")
		}

		// Verify workflows backup.
		data, err := os.ReadFile(filepath.Join(backupDir, "workflows", "ci.yml"))
		if err != nil {
			t.Fatalf("workflows backup missing: %v", err)
		}
		if string(data) != "ci" {
			t.Errorf("workflow content mismatch: %s", data)
		}
	})
}

func TestRestoreBase_WithWorkflows(t *testing.T) {
	t.Run("restores both base and workflows", func(t *testing.T) {
		root := t.TempDir()
		backupDir := t.TempDir()

		// Create backup content.
		if err := os.MkdirAll(filepath.Join(backupDir, "base"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(backupDir, "base", "AGENTS.md"), []byte("original"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(backupDir, "workflows"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(backupDir, "workflows", "pipeline.yml"), []byte("original-wf"), 0o644); err != nil {
			t.Fatal(err)
		}

		// Create modified content in repo.
		if err := os.MkdirAll(filepath.Join(root, "base"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "base", "AGENTS.md"), []byte("modified"), 0o644); err != nil {
			t.Fatal(err)
		}
		wfDir := filepath.Join(root, ".github", "workflows")
		if err := os.MkdirAll(wfDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(wfDir, "pipeline.yml"), []byte("modified-wf"), 0o644); err != nil {
			t.Fatal(err)
		}

		err := RestoreBase(root, backupDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify base restored.
		data, err := os.ReadFile(filepath.Join(root, "base", "AGENTS.md"))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "original" {
			t.Errorf("base not restored: %q", data)
		}

		// Verify workflows restored.
		data, err = os.ReadFile(filepath.Join(root, ".github", "workflows", "pipeline.yml"))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "original-wf" {
			t.Errorf("workflows not restored: %q", data)
		}
	})

	t.Run("no workflows backup skips workflow restore", func(t *testing.T) {
		root := t.TempDir()
		backupDir := t.TempDir()

		// Only base backup.
		if err := os.MkdirAll(filepath.Join(backupDir, "base"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(backupDir, "base", "AGENTS.md"), []byte("original"), 0o644); err != nil {
			t.Fatal(err)
		}

		// Create modified base in repo.
		if err := os.MkdirAll(filepath.Join(root, "base"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "base", "AGENTS.md"), []byte("modified"), 0o644); err != nil {
			t.Fatal(err)
		}

		err := RestoreBase(root, backupDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(filepath.Join(root, "base", "AGENTS.md"))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "original" {
			t.Errorf("base not restored: %q", data)
		}
	})
}

func TestBackupBase_WithRecipes(t *testing.T) {
	t.Run("backs up recipes when they exist", func(t *testing.T) {
		root := t.TempDir()
		// Create .goose/recipes/.
		recipesDir := filepath.Join(root, ".goose", "recipes")
		if err := os.MkdirAll(recipesDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(recipesDir, "dev.yaml"), []byte("dev-recipe"), 0o644); err != nil {
			t.Fatal(err)
		}

		backupDir, err := BackupBase(root)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer os.RemoveAll(backupDir)

		if backupDir == "" {
			t.Fatal("expected non-empty backup dir")
		}

		// Verify recipes backup.
		data, err := os.ReadFile(filepath.Join(backupDir, "recipes", "dev.yaml"))
		if err != nil {
			t.Fatalf("recipes backup missing: %v", err)
		}
		if string(data) != "dev-recipe" {
			t.Errorf("recipe content mismatch: %s", data)
		}
	})

	t.Run("no recipes directory is not an error", func(t *testing.T) {
		root := t.TempDir()
		// Create only base/.
		if err := os.MkdirAll(filepath.Join(root, "base"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "base", "AGENTS.md"), []byte("agents"), 0o644); err != nil {
			t.Fatal(err)
		}

		backupDir, err := BackupBase(root)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer os.RemoveAll(backupDir)

		// Verify recipes backup dir does NOT exist.
		if _, err := os.Stat(filepath.Join(backupDir, "recipes")); !os.IsNotExist(err) {
			t.Error("expected no recipes backup when dir is absent")
		}
	})

	t.Run("backs up all three directories", func(t *testing.T) {
		root := t.TempDir()
		// Create base/.
		if err := os.MkdirAll(filepath.Join(root, "base"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "base", "AGENTS.md"), []byte("agents"), 0o644); err != nil {
			t.Fatal(err)
		}
		// Create .github/workflows/.
		wfDir := filepath.Join(root, ".github", "workflows")
		if err := os.MkdirAll(wfDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(wfDir, "ci.yml"), []byte("ci"), 0o644); err != nil {
			t.Fatal(err)
		}
		// Create .goose/recipes/.
		recipesDir := filepath.Join(root, ".goose", "recipes")
		if err := os.MkdirAll(recipesDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(recipesDir, "dev.yaml"), []byte("dev"), 0o644); err != nil {
			t.Fatal(err)
		}

		backupDir, err := BackupBase(root)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer os.RemoveAll(backupDir)

		// Verify all three backed up.
		if _, err := os.Stat(filepath.Join(backupDir, "base", "AGENTS.md")); err != nil {
			t.Error("base backup missing")
		}
		if _, err := os.Stat(filepath.Join(backupDir, "workflows", "ci.yml")); err != nil {
			t.Error("workflows backup missing")
		}
		if _, err := os.Stat(filepath.Join(backupDir, "recipes", "dev.yaml")); err != nil {
			t.Error("recipes backup missing")
		}
	})
}

func TestRestoreBase_WithRecipes(t *testing.T) {
	t.Run("restores recipes from backup", func(t *testing.T) {
		root := t.TempDir()
		backupDir := t.TempDir()

		// Create recipes backup.
		if err := os.MkdirAll(filepath.Join(backupDir, "recipes"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(backupDir, "recipes", "dev.yaml"), []byte("original-recipe"), 0o644); err != nil {
			t.Fatal(err)
		}

		// Create modified recipes in repo.
		recipesDir := filepath.Join(root, ".goose", "recipes")
		if err := os.MkdirAll(recipesDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(recipesDir, "dev.yaml"), []byte("modified-recipe"), 0o644); err != nil {
			t.Fatal(err)
		}

		err := RestoreBase(root, backupDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify recipes restored.
		data, err := os.ReadFile(filepath.Join(root, ".goose", "recipes", "dev.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "original-recipe" {
			t.Errorf("recipes not restored: %q", data)
		}
	})

	t.Run("no recipes backup skips recipe restore", func(t *testing.T) {
		root := t.TempDir()
		backupDir := t.TempDir()

		// Only base backup.
		if err := os.MkdirAll(filepath.Join(backupDir, "base"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(backupDir, "base", "AGENTS.md"), []byte("original"), 0o644); err != nil {
			t.Fatal(err)
		}

		// Create modified base in repo.
		if err := os.MkdirAll(filepath.Join(root, "base"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "base", "AGENTS.md"), []byte("modified"), 0o644); err != nil {
			t.Fatal(err)
		}

		err := RestoreBase(root, backupDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// base should be restored.
		data, err := os.ReadFile(filepath.Join(root, "base", "AGENTS.md"))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "original" {
			t.Errorf("base not restored: %q", data)
		}
	})
}

func TestDeployRecipes(t *testing.T) {
	t.Run("copies recipes from template to repo", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoRoot := t.TempDir()

		// Create source recipes in template.
		srcRecipes := filepath.Join(tmpDir, ".goose", "recipes")
		if err := os.MkdirAll(srcRecipes, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(srcRecipes, "dev.yaml"), []byte("dev-content"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(srcRecipes, "review.yaml"), []byte("review-content"), 0o644); err != nil {
			t.Fatal(err)
		}

		err := DeployRecipes(tmpDir, repoRoot)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify files were copied.
		data, err := os.ReadFile(filepath.Join(repoRoot, ".goose", "recipes", "dev.yaml"))
		if err != nil {
			t.Fatalf("recipe file missing: %v", err)
		}
		if string(data) != "dev-content" {
			t.Errorf("content mismatch: %s", data)
		}

		data, err = os.ReadFile(filepath.Join(repoRoot, ".goose", "recipes", "review.yaml"))
		if err != nil {
			t.Fatalf("review recipe file missing: %v", err)
		}
		if string(data) != "review-content" {
			t.Errorf("review content mismatch: %s", data)
		}
	})

	t.Run("returns nil when source absent", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoRoot := t.TempDir()

		err := DeployRecipes(tmpDir, repoRoot)
		if err != nil {
			t.Fatalf("expected nil for absent source, got: %v", err)
		}
	})

	t.Run("replaces existing recipes", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoRoot := t.TempDir()

		// Create source recipes.
		srcRecipes := filepath.Join(tmpDir, ".goose", "recipes")
		if err := os.MkdirAll(srcRecipes, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(srcRecipes, "new.yaml"), []byte("new-content"), 0o644); err != nil {
			t.Fatal(err)
		}

		// Create existing recipes in repo.
		dstRecipes := filepath.Join(repoRoot, ".goose", "recipes")
		if err := os.MkdirAll(dstRecipes, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dstRecipes, "old.yaml"), []byte("old-content"), 0o644); err != nil {
			t.Fatal(err)
		}

		err := DeployRecipes(tmpDir, repoRoot)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify new file exists.
		if _, err := os.Stat(filepath.Join(dstRecipes, "new.yaml")); os.IsNotExist(err) {
			t.Error("new.yaml should exist")
		}

		// Verify old file was removed.
		if _, err := os.Stat(filepath.Join(dstRecipes, "old.yaml")); !os.IsNotExist(err) {
			t.Error("old.yaml should have been removed")
		}
	})
}

func TestCopyBase(t *testing.T) {
	t.Run("success with nested dirs", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoRoot := t.TempDir()

		// Create source base/ in tmpDir.
		srcBase := filepath.Join(tmpDir, "base")
		if err := os.MkdirAll(filepath.Join(srcBase, "standards"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(srcBase, "AGENTS.md"), []byte("new content"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(srcBase, "standards", "go.md"), []byte("new go"), 0o644); err != nil {
			t.Fatal(err)
		}

		// Create existing base/ in repoRoot.
		if err := os.MkdirAll(filepath.Join(repoRoot, "base"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, "base", "old.md"), []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}

		err := CopyBase(tmpDir, repoRoot)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify new content.
		data, err := os.ReadFile(filepath.Join(repoRoot, "base", "AGENTS.md"))
		if err != nil {
			t.Fatalf("file missing: %v", err)
		}
		if string(data) != "new content" {
			t.Errorf("content mismatch: %s", data)
		}

		// Verify old file was removed.
		if _, err := os.Stat(filepath.Join(repoRoot, "base", "old.md")); !os.IsNotExist(err) {
			t.Error("old file should have been removed")
		}
	})

	t.Run("missing base in template", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoRoot := t.TempDir()

		err := CopyBase(tmpDir, repoRoot)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "does not contain a base/") {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestDeployWorkflows(t *testing.T) {
	t.Run("copies workflow files", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoRoot := t.TempDir()

		// Create source workflows in template.
		srcWf := filepath.Join(tmpDir, "base", ".github", "workflows")
		if err := os.MkdirAll(srcWf, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(srcWf, "pipeline.yml"), []byte("pipeline-content"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(srcWf, "ci.yml"), []byte("ci-content"), 0o644); err != nil {
			t.Fatal(err)
		}

		err := DeployWorkflows(tmpDir, repoRoot, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify files were copied.
		data, err := os.ReadFile(filepath.Join(repoRoot, ".github", "workflows", "pipeline.yml"))
		if err != nil {
			t.Fatalf("workflow file missing: %v", err)
		}
		if string(data) != "pipeline-content" {
			t.Errorf("content mismatch: %s", data)
		}

		data, err = os.ReadFile(filepath.Join(repoRoot, ".github", "workflows", "ci.yml"))
		if err != nil {
			t.Fatalf("ci workflow file missing: %v", err)
		}
		if string(data) != "ci-content" {
			t.Errorf("ci content mismatch: %s", data)
		}
	})

	t.Run("returns nil when source absent", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoRoot := t.TempDir()

		err := DeployWorkflows(tmpDir, repoRoot, nil)
		if err != nil {
			t.Fatalf("expected nil for absent source, got: %v", err)
		}
	})

	t.Run("creates destination directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoRoot := t.TempDir()

		srcWf := filepath.Join(tmpDir, "base", ".github", "workflows")
		if err := os.MkdirAll(srcWf, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(srcWf, "test.yml"), []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}

		err := DeployWorkflows(tmpDir, repoRoot, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		info, err := os.Stat(filepath.Join(repoRoot, ".github", "workflows"))
		if err != nil {
			t.Fatalf("destination dir not created: %v", err)
		}
		if !info.IsDir() {
			t.Error("expected directory")
		}
	})

	t.Run("excludes specified files", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoRoot := t.TempDir()

		srcWf := filepath.Join(tmpDir, "base", ".github", "workflows")
		if err := os.MkdirAll(srcWf, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(srcWf, "pipeline.yml"), []byte("pipeline"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(srcWf, "sync-status-to-label.yml"), []byte("sync"), 0o644); err != nil {
			t.Fatal(err)
		}

		err := DeployWorkflows(tmpDir, repoRoot, []string{"sync-status-to-label.yml"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify pipeline was copied.
		if _, err := os.Stat(filepath.Join(repoRoot, ".github", "workflows", "pipeline.yml")); os.IsNotExist(err) {
			t.Error("pipeline.yml should be copied")
		}

		// Verify excluded file was NOT copied.
		if _, err := os.Stat(filepath.Join(repoRoot, ".github", "workflows", "sync-status-to-label.yml")); err == nil {
			t.Error("sync-status-to-label.yml should NOT be copied when excluded")
		}
	})

	t.Run("removes pre-existing excluded files", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoRoot := t.TempDir()

		srcWf := filepath.Join(tmpDir, "base", ".github", "workflows")
		if err := os.MkdirAll(srcWf, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(srcWf, "pipeline.yml"), []byte("pipeline"), 0o644); err != nil {
			t.Fatal(err)
		}

		// Pre-create the excluded file in the destination.
		dstWf := filepath.Join(repoRoot, ".github", "workflows")
		if err := os.MkdirAll(dstWf, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dstWf, "sync-status-to-label.yml"), []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}

		err := DeployWorkflows(tmpDir, repoRoot, []string{"sync-status-to-label.yml"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify pre-existing excluded file was removed.
		if _, err := os.Stat(filepath.Join(dstWf, "sync-status-to-label.yml")); err == nil {
			t.Error("pre-existing sync-status-to-label.yml should be removed when excluded")
		}

		// Verify pipeline was still copied.
		if _, err := os.Stat(filepath.Join(dstWf, "pipeline.yml")); os.IsNotExist(err) {
			t.Error("pipeline.yml should be copied")
		}
	})
}

func TestExtractOwnerFromAgentsLocal_Sync(t *testing.T) {
	t.Run("extracts from GitHub URL", func(t *testing.T) {
		dir := t.TempDir()
		content := "## Repo\n\n- **GitHub:** https://github.com/myorg/my-project\n"
		if err := os.WriteFile(filepath.Join(dir, "AGENTS.local.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		got := extractOwnerFromAgentsLocal(dir)
		if got != "myorg" {
			t.Errorf("got %q, want %q", got, "myorg")
		}
	})

	t.Run("extracts from Owner field", func(t *testing.T) {
		dir := t.TempDir()
		content := "## Repo\n\n- **Owner:** alice\n"
		if err := os.WriteFile(filepath.Join(dir, "AGENTS.local.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		got := extractOwnerFromAgentsLocal(dir)
		if got != "alice" {
			t.Errorf("got %q, want %q", got, "alice")
		}
	})

	t.Run("returns empty when no file", func(t *testing.T) {
		dir := t.TempDir()
		got := extractOwnerFromAgentsLocal(dir)
		if got != "" {
			t.Errorf("got %q, want empty string", got)
		}
	})
}

func TestShowDiff(t *testing.T) {
	t.Run("returns diff output", func(t *testing.T) {
		responses := []struct {
			output string
			err    error
		}{
			{output: "diff --git a/base/AGENTS.md", err: nil}, // git diff base/
			{output: "", err: nil},                             // ls-files base/
			{output: "", err: nil},                             // git diff .github/workflows/
			{output: "", err: nil},                             // ls-files .github/workflows/
			{output: "", err: nil},                             // git diff .goose/recipes/
			{output: "", err: nil},                             // ls-files .goose/recipes/
		}
		run := fakeRunMulti(responses)

		diff, err := ShowDiff("/repo", run)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(diff, "diff --git") {
			t.Errorf("expected diff output, got %q", diff)
		}
	})

	t.Run("includes untracked files", func(t *testing.T) {
		responses := []struct {
			output string
			err    error
		}{
			{output: "", err: nil},                   // git diff base/
			{output: "base/new-file.md\n", err: nil}, // ls-files base/
			{output: "", err: nil},                   // git diff .github/workflows/
			{output: "", err: nil},                   // ls-files .github/workflows/
			{output: "", err: nil},                   // git diff .goose/recipes/
			{output: "", err: nil},                   // ls-files .goose/recipes/
		}
		run := fakeRunMulti(responses)

		diff, err := ShowDiff("/repo", run)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(diff, "New files") {
			t.Errorf("expected new files section, got %q", diff)
		}
	})

	t.Run("includes workflow diffs", func(t *testing.T) {
		responses := []struct {
			output string
			err    error
		}{
			{output: "", err: nil},                                      // git diff base/
			{output: "", err: nil},                                      // ls-files base/
			{output: "diff --git a/.github/workflows/ci.yml", err: nil}, // git diff .github/workflows/
			{output: "", err: nil},                                      // ls-files .github/workflows/
			{output: "", err: nil},                                      // git diff .goose/recipes/
			{output: "", err: nil},                                      // ls-files .goose/recipes/
		}
		run := fakeRunMulti(responses)

		diff, err := ShowDiff("/repo", run)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(diff, ".github/workflows/ci.yml") {
			t.Errorf("expected workflow diff, got %q", diff)
		}
	})

	t.Run("includes new workflow files", func(t *testing.T) {
		responses := []struct {
			output string
			err    error
		}{
			{output: "", err: nil},                                     // git diff base/
			{output: "", err: nil},                                     // ls-files base/
			{output: "", err: nil},                                     // git diff .github/workflows/
			{output: ".github/workflows/new-pipeline.yml\n", err: nil}, // ls-files .github/workflows/
			{output: "", err: nil},                                     // git diff .goose/recipes/
			{output: "", err: nil},                                     // ls-files .goose/recipes/
		}
		run := fakeRunMulti(responses)

		diff, err := ShowDiff("/repo", run)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(diff, "New workflow files") {
			t.Errorf("expected new workflow files section, got %q", diff)
		}
	})

	t.Run("includes recipe diffs", func(t *testing.T) {
		responses := []struct {
			output string
			err    error
		}{
			{output: "", err: nil},                                              // git diff base/
			{output: "", err: nil},                                              // ls-files base/
			{output: "", err: nil},                                              // git diff .github/workflows/
			{output: "", err: nil},                                              // ls-files .github/workflows/
			{output: "diff --git a/.goose/recipes/dev.yaml", err: nil},          // git diff .goose/recipes/
			{output: "", err: nil},                                              // ls-files .goose/recipes/
		}
		run := fakeRunMulti(responses)

		diff, err := ShowDiff("/repo", run)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(diff, ".goose/recipes/dev.yaml") {
			t.Errorf("expected recipe diff, got %q", diff)
		}
	})

	t.Run("includes new recipe files", func(t *testing.T) {
		responses := []struct {
			output string
			err    error
		}{
			{output: "", err: nil},                                  // git diff base/
			{output: "", err: nil},                                  // ls-files base/
			{output: "", err: nil},                                  // git diff .github/workflows/
			{output: "", err: nil},                                  // ls-files .github/workflows/
			{output: "", err: nil},                                  // git diff .goose/recipes/
			{output: ".goose/recipes/new-recipe.yaml\n", err: nil},  // ls-files .goose/recipes/
		}
		run := fakeRunMulti(responses)

		diff, err := ShowDiff("/repo", run)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(diff, "New recipe files") {
			t.Errorf("expected new recipe files section, got %q", diff)
		}
	})

	t.Run("git diff error", func(t *testing.T) {
		run := fakeRun("error", fmt.Errorf("exit 1"))
		_, err := ShowDiff("/repo", run)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestUpdateVersion(t *testing.T) {
	t.Run("writes version", func(t *testing.T) {
		root := t.TempDir()
		err := UpdateVersion(root, "v0.2.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(filepath.Join(root, "TEMPLATE_VERSION"))
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(string(data)) != "v0.2.0" {
			t.Errorf("got %q, want %q", string(data), "v0.2.0\n")
		}
	})
}

func TestWritePostSyncMD(t *testing.T) {
	t.Run("writes release body to POST_SYNC.md", func(t *testing.T) {
		root := t.TempDir()
		body := "## What's Changed\n\n- Fixed sync runner\n- Added tarball support"

		err := WritePostSyncMD(root, body)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, readErr := os.ReadFile(filepath.Join(root, "POST_SYNC.md"))
		if readErr != nil {
			t.Fatalf("POST_SYNC.md should exist: %v", readErr)
		}
		if string(data) != body {
			t.Errorf("content = %q, want %q", string(data), body)
		}
	})

	t.Run("writes file with empty body", func(t *testing.T) {
		root := t.TempDir()

		err := WritePostSyncMD(root, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, readErr := os.ReadFile(filepath.Join(root, "POST_SYNC.md"))
		if readErr != nil {
			t.Fatalf("POST_SYNC.md should exist even with empty body: %v", readErr)
		}
		if string(data) != "" {
			t.Errorf("content should be empty, got %q", string(data))
		}
	})

	t.Run("overwrites existing file", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "POST_SYNC.md")
		if err := os.WriteFile(path, []byte("old content"), 0o644); err != nil {
			t.Fatal(err)
		}

		err := WritePostSyncMD(root, "new content")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(path)
		if string(data) != "new content" {
			t.Errorf("content = %q, want %q", string(data), "new content")
		}
	})
}

func TestStageSync(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var calls []string
		run := func(name string, args ...string) (string, error) {
			joined := name + " " + strings.Join(args, " ")
			calls = append(calls, joined)
			return "", nil
		}

		err := StageSync("/repo", run)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(calls) != 1 {
			t.Fatalf("expected 1 call, got %d: %v", len(calls), calls)
		}

		// Verify git add was called with the right paths.
		if !strings.Contains(calls[0], "git") || !strings.Contains(calls[0], "add") {
			t.Errorf("expected git add call: %s", calls[0])
		}
		if !strings.Contains(calls[0], "base/") {
			t.Errorf("expected base/ in git add: %s", calls[0])
		}
		if !strings.Contains(calls[0], "TEMPLATE_VERSION") {
			t.Errorf("expected TEMPLATE_VERSION in git add: %s", calls[0])
		}
		if !strings.Contains(calls[0], ".github/workflows/") {
			t.Errorf("expected .github/workflows/ in git add: %s", calls[0])
		}
		if !strings.Contains(calls[0], ".goose/recipes/") {
			t.Errorf("expected .goose/recipes/ in git add: %s", calls[0])
		}
		if !strings.Contains(calls[0], "POST_SYNC.md") {
			t.Errorf("expected POST_SYNC.md in git add: %s", calls[0])
		}
	})

	t.Run("add error", func(t *testing.T) {
		run := fakeRun("error", fmt.Errorf("exit 1"))
		err := StageSync("/repo", run)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "git add") {
			t.Errorf("error should mention git add: %v", err)
		}
	})
}

func TestCommitSync(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var calls []string
		run := func(name string, args ...string) (string, error) {
			joined := name + " " + strings.Join(args, " ")
			calls = append(calls, joined)
			// Simulate staged changes: git diff --cached --quiet exits non-zero.
			if strings.Contains(joined, "diff") && strings.Contains(joined, "--cached") {
				return "", fmt.Errorf("exit 1")
			}
			return "", nil
		}

		err := CommitSync("/repo", "owner/template", "v0.2.0", run)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(calls) != 3 {
			t.Fatalf("expected 3 calls, got %d: %v", len(calls), calls)
		}

		// Verify git add was called.
		if !strings.Contains(calls[0], "git") || !strings.Contains(calls[0], "add") {
			t.Errorf("first call should be git add: %s", calls[0])
		}

		// Verify git diff --cached --quiet was called.
		if !strings.Contains(calls[1], "diff") || !strings.Contains(calls[1], "--cached") {
			t.Errorf("second call should be git diff --cached: %s", calls[1])
		}

		// Verify git commit was called with correct message.
		if !strings.Contains(calls[2], "commit") || !strings.Contains(calls[2], "sync base/, workflows, and recipes") {
			t.Errorf("third call should be git commit with updated message: %s", calls[2])
		}
	})

	t.Run("nothing to commit", func(t *testing.T) {
		var calls []string
		run := func(name string, args ...string) (string, error) {
			joined := name + " " + strings.Join(args, " ")
			calls = append(calls, joined)
			// git diff --cached --quiet exits 0 = nothing staged.
			return "", nil
		}

		err := CommitSync("/repo", "owner/template", "v0.2.0", run)
		if err != nil {
			t.Fatalf("expected nil when nothing to commit, got: %v", err)
		}

		if len(calls) != 2 {
			t.Fatalf("expected 2 calls (add + diff check), got %d: %v", len(calls), calls)
		}
	})

	t.Run("add error", func(t *testing.T) {
		run := fakeRun("error", fmt.Errorf("exit 1"))
		err := CommitSync("/repo", "owner/template", "v0.2.0", run)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestRestoreBase(t *testing.T) {
	t.Run("restores from backup", func(t *testing.T) {
		root := t.TempDir()
		backupDir := t.TempDir()

		// Create backup content.
		if err := os.MkdirAll(filepath.Join(backupDir, "base"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(backupDir, "base", "AGENTS.md"), []byte("original"), 0o644); err != nil {
			t.Fatal(err)
		}

		// Create modified base/ in repo.
		if err := os.MkdirAll(filepath.Join(root, "base"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "base", "AGENTS.md"), []byte("modified"), 0o644); err != nil {
			t.Fatal(err)
		}

		err := RestoreBase(root, backupDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(filepath.Join(root, "base", "AGENTS.md"))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "original" {
			t.Errorf("expected original content, got %q", data)
		}
	})

	t.Run("empty backup dir is no-op", func(t *testing.T) {
		err := RestoreBase("/repo", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestCleanupTemp(t *testing.T) {
	t.Run("removes directory", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte("data"), 0o644); err != nil {
			t.Fatal(err)
		}

		err := CleanupTemp(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Error("directory should have been removed")
		}
	})

	t.Run("empty string is no-op", func(t *testing.T) {
		err := CleanupTemp("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// createFakeTarballFetch returns a tarball.FetchFunc that produces a gzipped tar
// archive containing the given files. The archive includes a top-level prefix
// directory ("repo-v1.0.0/") to simulate GitHub's tarball format.
func createFakeTarballFetch(t *testing.T, files map[string]string) func(repo, version string) (io.ReadCloser, error) {
	t.Helper()
	return func(repo, version string) (io.ReadCloser, error) {
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gw)

		// Add top-level prefix directory.
		prefix := "repo-" + version + "/"
		if err := tw.WriteHeader(&tar.Header{
			Name:     prefix,
			Typeflag: tar.TypeDir,
			Mode:     0o755,
		}); err != nil {
			t.Fatalf("writing prefix dir header: %v", err)
		}

		for path, content := range files {
			fullPath := prefix + path

			// Create parent directories.
			dir := filepath.Dir(fullPath)
			if dir != prefix && dir != "." {
				_ = tw.WriteHeader(&tar.Header{
					Name:     dir + "/",
					Typeflag: tar.TypeDir,
					Mode:     0o755,
				})
			}

			if err := tw.WriteHeader(&tar.Header{
				Name:     fullPath,
				Size:     int64(len(content)),
				Mode:     0o644,
				Typeflag: tar.TypeReg,
			}); err != nil {
				t.Fatalf("writing header for %s: %v", path, err)
			}
			if _, err := tw.Write([]byte(content)); err != nil {
				t.Fatalf("writing content for %s: %v", path, err)
			}
		}

		if err := tw.Close(); err != nil {
			t.Fatalf("closing tar writer: %v", err)
		}
		if err := gw.Close(); err != nil {
			t.Fatalf("closing gzip writer: %v", err)
		}

		return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
	}
}
