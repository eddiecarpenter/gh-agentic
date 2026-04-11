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
	t.Run("success extracts template files to correct locations", func(t *testing.T) {
		orig := fetchTarballFn
		defer func() { fetchTarballFn = orig }()

		destRoot := t.TempDir()
		// Create existing .ai/ so CopyAI can replace it.
		if err := os.MkdirAll(filepath.Join(destRoot, ".ai"), 0o755); err != nil {
			t.Fatal(err)
		}

		fetchTarballFn = createFakeTarballFetch(t, map[string]string{
			".ai/RULEBOOK.md":                        "# Rulebook",
			".ai/standards/go.md":                    "Go standards",
			".ai/.github/workflows/ci.yml":           "ci workflow",
			".goose/recipes/dev.yaml":                "dev recipe",
			"CLAUDE.md":                              "# CLAUDE",
			"AGENTS.md":                              "# AGENTS",
		})

		err := FetchAndExtractTemplate("https://api.github.com/repos/owner/repo/tarball/v1.0.0", "owner/repo", "v1.0.0", destRoot, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify extracted files are in correct locations.
		for _, check := range []struct {
			path    string
			content string
		}{
			{".ai/RULEBOOK.md", "# Rulebook"},
			{".ai/standards/go.md", "Go standards"},
			{".github/workflows/ci.yml", "ci workflow"},
			{".goose/recipes/dev.yaml", "dev recipe"},
			{"CLAUDE.md", "# CLAUDE"},
			{"AGENTS.md", "# AGENTS"},
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

		err := FetchAndExtractTemplate("", "owner/repo", "v1.0.0", t.TempDir(), nil)
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

		err := FetchAndExtractTemplate("https://api.github.com/repos/owner/repo/tarball/v1.0.0", "owner/repo", "v1.0.0", destRoot, nil)
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

func TestBackupAI(t *testing.T) {
	t.Run("existing base directory", func(t *testing.T) {
		root := t.TempDir()
		baseDir := filepath.Join(root, ".ai")
		if err := os.MkdirAll(filepath.Join(baseDir, "standards"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(baseDir, "AGENTS.md"), []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(baseDir, "standards", "go.md"), []byte("go standards"), 0o644); err != nil {
			t.Fatal(err)
		}

		backupDir, err := BackupAI(root)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer os.RemoveAll(backupDir)

		if backupDir == "" {
			t.Fatal("expected non-empty backup dir")
		}

		// Verify files were copied.
		data, err := os.ReadFile(filepath.Join(backupDir, "ai", "AGENTS.md"))
		if err != nil {
			t.Fatalf("backup file missing: %v", err)
		}
		if string(data) != "content" {
			t.Errorf("backup content mismatch: %s", data)
		}

		data, err = os.ReadFile(filepath.Join(backupDir, "ai", "standards", "go.md"))
		if err != nil {
			t.Fatalf("nested backup file missing: %v", err)
		}
		if string(data) != "go standards" {
			t.Errorf("nested backup content mismatch: %s", data)
		}
	})

	t.Run("no base directory", func(t *testing.T) {
		root := t.TempDir()
		backupDir, err := BackupAI(root)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if backupDir != "" {
			t.Errorf("expected empty backup dir, got %q", backupDir)
		}
	})
}

func TestBackupAI_WithWorkflows(t *testing.T) {
	t.Run("backs up both base and workflows", func(t *testing.T) {
		root := t.TempDir()
		// Create .ai/.
		if err := os.MkdirAll(filepath.Join(root, ".ai"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".ai", "AGENTS.md"), []byte("agents"), 0o644); err != nil {
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

		backupDir, err := BackupAI(root)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer os.RemoveAll(backupDir)

		// Verify base backup.
		data, err := os.ReadFile(filepath.Join(backupDir, "ai", "AGENTS.md"))
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
		// Create only .ai/.
		if err := os.MkdirAll(filepath.Join(root, ".ai"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".ai", "AGENTS.md"), []byte("agents"), 0o644); err != nil {
			t.Fatal(err)
		}

		backupDir, err := BackupAI(root)
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

		backupDir, err := BackupAI(root)
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

func TestRestoreAI_WithWorkflows(t *testing.T) {
	t.Run("restores both base and workflows", func(t *testing.T) {
		root := t.TempDir()
		backupDir := t.TempDir()

		// Create backup content.
		if err := os.MkdirAll(filepath.Join(backupDir, "ai"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(backupDir, "ai", "AGENTS.md"), []byte("original"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(backupDir, "workflows"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(backupDir, "workflows", "pipeline.yml"), []byte("original-wf"), 0o644); err != nil {
			t.Fatal(err)
		}

		// Create modified content in repo.
		if err := os.MkdirAll(filepath.Join(root, ".ai"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".ai", "AGENTS.md"), []byte("modified"), 0o644); err != nil {
			t.Fatal(err)
		}
		wfDir := filepath.Join(root, ".github", "workflows")
		if err := os.MkdirAll(wfDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(wfDir, "pipeline.yml"), []byte("modified-wf"), 0o644); err != nil {
			t.Fatal(err)
		}

		err := RestoreAI(root, backupDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify base restored.
		data, err := os.ReadFile(filepath.Join(root, ".ai", "AGENTS.md"))
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
		if err := os.MkdirAll(filepath.Join(backupDir, "ai"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(backupDir, "ai", "AGENTS.md"), []byte("original"), 0o644); err != nil {
			t.Fatal(err)
		}

		// Create modified base in repo.
		if err := os.MkdirAll(filepath.Join(root, ".ai"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".ai", "AGENTS.md"), []byte("modified"), 0o644); err != nil {
			t.Fatal(err)
		}

		err := RestoreAI(root, backupDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(filepath.Join(root, ".ai", "AGENTS.md"))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "original" {
			t.Errorf("base not restored: %q", data)
		}
	})
}

func TestBackupAI_WithRecipes(t *testing.T) {
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

		backupDir, err := BackupAI(root)
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
		// Create only .ai/.
		if err := os.MkdirAll(filepath.Join(root, ".ai"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".ai", "AGENTS.md"), []byte("agents"), 0o644); err != nil {
			t.Fatal(err)
		}

		backupDir, err := BackupAI(root)
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
		// Create .ai/.
		if err := os.MkdirAll(filepath.Join(root, ".ai"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".ai", "AGENTS.md"), []byte("agents"), 0o644); err != nil {
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

		backupDir, err := BackupAI(root)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer os.RemoveAll(backupDir)

		// Verify all three backed up.
		if _, err := os.Stat(filepath.Join(backupDir, "ai", "AGENTS.md")); err != nil {
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

func TestRestoreAI_WithRecipes(t *testing.T) {
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

		err := RestoreAI(root, backupDir)
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
		if err := os.MkdirAll(filepath.Join(backupDir, "ai"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(backupDir, "ai", "AGENTS.md"), []byte("original"), 0o644); err != nil {
			t.Fatal(err)
		}

		// Create modified base in repo.
		if err := os.MkdirAll(filepath.Join(root, ".ai"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".ai", "AGENTS.md"), []byte("modified"), 0o644); err != nil {
			t.Fatal(err)
		}

		err := RestoreAI(root, backupDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// base should be restored.
		data, err := os.ReadFile(filepath.Join(root, ".ai", "AGENTS.md"))
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

	t.Run("merge semantics — template overwrites, project files preserved", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoRoot := t.TempDir()

		// Create source recipes (template).
		srcRecipes := filepath.Join(tmpDir, ".goose", "recipes")
		if err := os.MkdirAll(srcRecipes, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(srcRecipes, "template.yaml"), []byte("new-content"), 0o644); err != nil {
			t.Fatal(err)
		}

		// Create existing project-owned recipe in repo.
		dstRecipes := filepath.Join(repoRoot, ".goose", "recipes")
		if err := os.MkdirAll(dstRecipes, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dstRecipes, "my-custom.yaml"), []byte("custom-content"), 0o644); err != nil {
			t.Fatal(err)
		}

		err := DeployRecipes(tmpDir, repoRoot)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify template file exists.
		if _, err := os.Stat(filepath.Join(dstRecipes, "template.yaml")); os.IsNotExist(err) {
			t.Error("template.yaml should exist")
		}

		// Verify project-owned file is preserved (merge semantics).
		if _, err := os.Stat(filepath.Join(dstRecipes, "my-custom.yaml")); os.IsNotExist(err) {
			t.Error("my-custom.yaml should be preserved (merge semantics)")
		}
		data, _ := os.ReadFile(filepath.Join(dstRecipes, "my-custom.yaml"))
		if string(data) != "custom-content" {
			t.Errorf("my-custom.yaml content changed: %q", data)
		}
	})

	t.Run("template recipe overwrites existing version", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoRoot := t.TempDir()

		// Create source recipe (template) with updated content.
		srcRecipes := filepath.Join(tmpDir, ".goose", "recipes")
		if err := os.MkdirAll(srcRecipes, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(srcRecipes, "dev.yaml"), []byte("updated-dev"), 0o644); err != nil {
			t.Fatal(err)
		}

		// Create existing dev.yaml in repo with old content.
		dstRecipes := filepath.Join(repoRoot, ".goose", "recipes")
		if err := os.MkdirAll(dstRecipes, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dstRecipes, "dev.yaml"), []byte("old-dev"), 0o644); err != nil {
			t.Fatal(err)
		}

		err := DeployRecipes(tmpDir, repoRoot)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(dstRecipes, "dev.yaml"))
		if string(data) != "updated-dev" {
			t.Errorf("template recipe should overwrite existing: got %q", data)
		}
	})

	t.Run("template removal does not delete repo copy", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoRoot := t.TempDir()

		// Empty template recipes directory (template removed a recipe).
		srcRecipes := filepath.Join(tmpDir, ".goose", "recipes")
		if err := os.MkdirAll(srcRecipes, 0o755); err != nil {
			t.Fatal(err)
		}

		// Repo has a recipe that used to be in the template.
		dstRecipes := filepath.Join(repoRoot, ".goose", "recipes")
		if err := os.MkdirAll(dstRecipes, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dstRecipes, "removed.yaml"), []byte("still-here"), 0o644); err != nil {
			t.Fatal(err)
		}

		err := DeployRecipes(tmpDir, repoRoot)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify file still exists — merge does not delete.
		if _, err := os.Stat(filepath.Join(dstRecipes, "removed.yaml")); os.IsNotExist(err) {
			t.Error("removed.yaml should be preserved — merge does not delete")
		}
	})
}

func TestDeployWorkflows_MergeSemantics(t *testing.T) {
	t.Run("project-owned workflow preserved", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoRoot := t.TempDir()

		// Template provides one workflow.
		srcWf := filepath.Join(tmpDir, ".ai", ".github", "workflows")
		if err := os.MkdirAll(srcWf, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(srcWf, "agentic-pipeline.yml"), []byte("template"), 0o644); err != nil {
			t.Fatal(err)
		}

		// Repo has a project-owned workflow.
		dstWf := filepath.Join(repoRoot, ".github", "workflows")
		if err := os.MkdirAll(dstWf, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dstWf, "publish-release.yml"), []byte("project-owned"), 0o644); err != nil {
			t.Fatal(err)
		}

		err := DeployWorkflows(tmpDir, repoRoot, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Template workflow deployed.
		data, _ := os.ReadFile(filepath.Join(dstWf, "agentic-pipeline.yml"))
		if string(data) != "template" {
			t.Errorf("template workflow should be deployed: got %q", data)
		}

		// Project-owned workflow preserved.
		data, _ = os.ReadFile(filepath.Join(dstWf, "publish-release.yml"))
		if string(data) != "project-owned" {
			t.Errorf("project-owned workflow should be preserved: got %q", data)
		}
	})

	t.Run("template removal does not delete repo workflow", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoRoot := t.TempDir()

		// Empty template workflows (template removed a workflow).
		srcWf := filepath.Join(tmpDir, ".ai", ".github", "workflows")
		if err := os.MkdirAll(srcWf, 0o755); err != nil {
			t.Fatal(err)
		}

		// Repo has a workflow that used to be in the template.
		dstWf := filepath.Join(repoRoot, ".github", "workflows")
		if err := os.MkdirAll(dstWf, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dstWf, "old-template.yml"), []byte("still-here"), 0o644); err != nil {
			t.Fatal(err)
		}

		err := DeployWorkflows(tmpDir, repoRoot, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// File should still exist — merge does not delete.
		if _, err := os.Stat(filepath.Join(dstWf, "old-template.yml")); os.IsNotExist(err) {
			t.Error("old-template.yml should be preserved — merge does not delete")
		}
	})
}

func TestCopyAI(t *testing.T) {
	t.Run("success with nested dirs", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoRoot := t.TempDir()

		// Create source .ai/ in tmpDir.
		srcBase := filepath.Join(tmpDir, ".ai")
		if err := os.MkdirAll(filepath.Join(srcBase, "standards"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(srcBase, "AGENTS.md"), []byte("new content"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(srcBase, "standards", "go.md"), []byte("new go"), 0o644); err != nil {
			t.Fatal(err)
		}

		// Create existing .ai/ in repoRoot.
		if err := os.MkdirAll(filepath.Join(repoRoot, ".ai"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, ".ai", "old.md"), []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}

		err := CopyAI(tmpDir, repoRoot)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify new content.
		data, err := os.ReadFile(filepath.Join(repoRoot, ".ai", "AGENTS.md"))
		if err != nil {
			t.Fatalf("file missing: %v", err)
		}
		if string(data) != "new content" {
			t.Errorf("content mismatch: %s", data)
		}

		// Verify old file was removed.
		if _, err := os.Stat(filepath.Join(repoRoot, ".ai", "old.md")); !os.IsNotExist(err) {
			t.Error("old file should have been removed")
		}
	})

	t.Run("missing base in template", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoRoot := t.TempDir()

		err := CopyAI(tmpDir, repoRoot)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "does not contain a .ai/") {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestDeployWorkflows(t *testing.T) {
	t.Run("copies workflow files", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoRoot := t.TempDir()

		// Create source workflows in template.
		srcWf := filepath.Join(tmpDir, ".ai", ".github", "workflows")
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

		srcWf := filepath.Join(tmpDir, ".ai", ".github", "workflows")
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

		srcWf := filepath.Join(tmpDir, ".ai", ".github", "workflows")
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

		srcWf := filepath.Join(tmpDir, ".ai", ".github", "workflows")
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
	// ShowDiff makes 8 git calls:
	//   1. git diff .ai/
	//   2. ls-files .ai/
	//   3. git diff .github/workflows/
	//   4. ls-files .github/workflows/
	//   5. git diff .github/actions/
	//   6. ls-files .github/actions/
	//   7. git diff .goose/recipes/
	//   8. ls-files .goose/recipes/

	t.Run("returns diff output", func(t *testing.T) {
		responses := []struct {
			output string
			err    error
		}{
			{output: "diff --git a/.ai/RULEBOOK.md", err: nil}, // git diff .ai/
			{output: "", err: nil},                              // ls-files .ai/
			{output: "", err: nil},                              // git diff .github/workflows/
			{output: "", err: nil},                              // ls-files .github/workflows/
			{output: "", err: nil},                              // git diff .github/actions/
			{output: "", err: nil},                              // ls-files .github/actions/
			{output: "", err: nil},                              // git diff .goose/recipes/
			{output: "", err: nil},                              // ls-files .goose/recipes/
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
			{output: "", err: nil},                  // git diff .ai/
			{output: ".ai/new-file.md\n", err: nil}, // ls-files .ai/
			{output: "", err: nil},                  // git diff .github/workflows/
			{output: "", err: nil},                  // ls-files .github/workflows/
			{output: "", err: nil},                  // git diff .github/actions/
			{output: "", err: nil},                  // ls-files .github/actions/
			{output: "", err: nil},                  // git diff .goose/recipes/
			{output: "", err: nil},                  // ls-files .goose/recipes/
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
			{output: "", err: nil},                                      // git diff .ai/
			{output: "", err: nil},                                      // ls-files .ai/
			{output: "diff --git a/.github/workflows/ci.yml", err: nil}, // git diff .github/workflows/
			{output: "", err: nil},                                      // ls-files .github/workflows/
			{output: "", err: nil},                                      // git diff .github/actions/
			{output: "", err: nil},                                      // ls-files .github/actions/
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
			{output: "", err: nil},                                     // git diff .ai/
			{output: "", err: nil},                                     // ls-files .ai/
			{output: "", err: nil},                                     // git diff .github/workflows/
			{output: ".github/workflows/new-pipeline.yml\n", err: nil}, // ls-files .github/workflows/
			{output: "", err: nil},                                     // git diff .github/actions/
			{output: "", err: nil},                                     // ls-files .github/actions/
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

	t.Run("includes action diffs", func(t *testing.T) {
		responses := []struct {
			output string
			err    error
		}{
			{output: "", err: nil},                                                                 // git diff .ai/
			{output: "", err: nil},                                                                 // ls-files .ai/
			{output: "", err: nil},                                                                 // git diff .github/workflows/
			{output: "", err: nil},                                                                 // ls-files .github/workflows/
			{output: "diff --git a/.github/actions/install-ai-tools/action.yml", err: nil},        // git diff .github/actions/
			{output: "", err: nil},                                                                 // ls-files .github/actions/
			{output: "", err: nil},                                                                 // git diff .goose/recipes/
			{output: "", err: nil},                                                                 // ls-files .goose/recipes/
		}
		run := fakeRunMulti(responses)

		diff, err := ShowDiff("/repo", run)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(diff, ".github/actions/install-ai-tools/action.yml") {
			t.Errorf("expected action diff, got %q", diff)
		}
	})

	t.Run("includes new action files", func(t *testing.T) {
		responses := []struct {
			output string
			err    error
		}{
			{output: "", err: nil},                                                        // git diff .ai/
			{output: "", err: nil},                                                        // ls-files .ai/
			{output: "", err: nil},                                                        // git diff .github/workflows/
			{output: "", err: nil},                                                        // ls-files .github/workflows/
			{output: "", err: nil},                                                        // git diff .github/actions/
			{output: ".github/actions/install-ai-tools/action.yml\n", err: nil},          // ls-files .github/actions/
			{output: "", err: nil},                                                        // git diff .goose/recipes/
			{output: "", err: nil},                                                        // ls-files .goose/recipes/
		}
		run := fakeRunMulti(responses)

		diff, err := ShowDiff("/repo", run)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(diff, "New action files") {
			t.Errorf("expected new action files section, got %q", diff)
		}
	})

	t.Run("includes recipe diffs", func(t *testing.T) {
		responses := []struct {
			output string
			err    error
		}{
			{output: "", err: nil},                                     // git diff .ai/
			{output: "", err: nil},                                     // ls-files .ai/
			{output: "", err: nil},                                     // git diff .github/workflows/
			{output: "", err: nil},                                     // ls-files .github/workflows/
			{output: "", err: nil},                                     // git diff .github/actions/
			{output: "", err: nil},                                     // ls-files .github/actions/
			{output: "diff --git a/.goose/recipes/dev.yaml", err: nil}, // git diff .goose/recipes/
			{output: "", err: nil},                                     // ls-files .goose/recipes/
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
			{output: "", err: nil},                                 // git diff .ai/
			{output: "", err: nil},                                 // ls-files .ai/
			{output: "", err: nil},                                 // git diff .github/workflows/
			{output: "", err: nil},                                 // ls-files .github/workflows/
			{output: "", err: nil},                                 // git diff .github/actions/
			{output: "", err: nil},                                 // ls-files .github/actions/
			{output: "", err: nil},                                 // git diff .goose/recipes/
			{output: ".goose/recipes/new-recipe.yaml\n", err: nil}, // ls-files .goose/recipes/
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

func TestDeployActions(t *testing.T) {
	t.Run("deploys action files from template", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoRoot := t.TempDir()

		// Create template action structure.
		actionSrc := filepath.Join(tmpDir, ".ai", ".github", "actions", "install-ai-tools")
		if err := os.MkdirAll(actionSrc, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(actionSrc, "action.yml"), []byte("name: Install AI Tools\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		err := DeployActions(tmpDir, repoRoot)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify action was deployed.
		data, err := os.ReadFile(filepath.Join(repoRoot, ".github", "actions", "install-ai-tools", "action.yml"))
		if err != nil {
			t.Fatalf(".github/actions/install-ai-tools/action.yml should exist: %v", err)
		}
		if string(data) != "name: Install AI Tools\n" {
			t.Errorf("content = %q, want %q", data, "name: Install AI Tools\n")
		}
	})

	t.Run("no-op when source does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoRoot := t.TempDir()

		err := DeployActions(tmpDir, repoRoot)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// .github/actions/ should not have been created.
		if _, err := os.Stat(filepath.Join(repoRoot, ".github", "actions")); err == nil {
			t.Error(".github/actions/ should not exist when template has no actions")
		}
	})

	t.Run("overwrites existing action file", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoRoot := t.TempDir()

		// Pre-existing action.
		actionDst := filepath.Join(repoRoot, ".github", "actions", "install-ai-tools")
		if err := os.MkdirAll(actionDst, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(actionDst, "action.yml"), []byte("old content\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		// Template with updated content.
		actionSrc := filepath.Join(tmpDir, ".ai", ".github", "actions", "install-ai-tools")
		if err := os.MkdirAll(actionSrc, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(actionSrc, "action.yml"), []byte("new content\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		err := DeployActions(tmpDir, repoRoot)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(actionDst, "action.yml"))
		if string(data) != "new content\n" {
			t.Errorf("content = %q, want %q", data, "new content\n")
		}
	})
}

func TestUpdateVersion(t *testing.T) {
	t.Run("writes version to config.yml", func(t *testing.T) {
		root := t.TempDir()
		// Pre-create .ai/config.yml so UpdateVersion can read the template field.
		if err := os.MkdirAll(filepath.Join(root, ".ai"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".ai", "config.yml"),
			[]byte("template: owner/template\nversion: v0.1.0\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		err := UpdateVersion(root, "v0.2.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify .ai/config.yml was updated.
		data, err := os.ReadFile(filepath.Join(root, ".ai", "config.yml"))
		if err != nil {
			t.Fatal(err)
		}
		cfg, parseErr := ReadAIConfig(root)
		if parseErr != nil {
			t.Fatalf("re-reading config.yml: %v (raw: %s)", parseErr, data)
		}
		if cfg.Version != "v0.2.0" {
			t.Errorf("version = %q, want %q", cfg.Version, "v0.2.0")
		}
		if cfg.Template != "owner/template" {
			t.Errorf("template = %q, want %q", cfg.Template, "owner/template")
		}
	})

	t.Run("falls back to TEMPLATE_SOURCE when config.yml absent", func(t *testing.T) {
		root := t.TempDir()
		// Only TEMPLATE_SOURCE exists — no .ai/config.yml yet.
		if err := os.WriteFile(filepath.Join(root, "TEMPLATE_SOURCE"),
			[]byte("owner/fallback\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		// .ai/ dir must exist for WriteFile to succeed.
		if err := os.MkdirAll(filepath.Join(root, ".ai"), 0o755); err != nil {
			t.Fatal(err)
		}

		err := UpdateVersion(root, "v0.3.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		cfg, parseErr := ReadAIConfig(root)
		if parseErr != nil {
			t.Fatalf("reading config.yml after update: %v", parseErr)
		}
		if cfg.Version != "v0.3.0" {
			t.Errorf("version = %q, want %q", cfg.Version, "v0.3.0")
		}
		if cfg.Template != "owner/fallback" {
			t.Errorf("template = %q, want %q", cfg.Template, "owner/fallback")
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

		// Expect 2 calls: git rm (legacy files) + git add.
		if len(calls) != 2 {
			t.Fatalf("expected 2 calls, got %d: %v", len(calls), calls)
		}

		// Verify git rm was called to remove legacy files.
		if !strings.Contains(calls[0], "git") || !strings.Contains(calls[0], "rm") {
			t.Errorf("expected git rm call: %s", calls[0])
		}
		if !strings.Contains(calls[0], "TEMPLATE_SOURCE") {
			t.Errorf("expected TEMPLATE_SOURCE in git rm: %s", calls[0])
		}
		if !strings.Contains(calls[0], "TEMPLATE_VERSION") {
			t.Errorf("expected TEMPLATE_VERSION in git rm: %s", calls[0])
		}

		// Verify git add was called with the right paths.
		if !strings.Contains(calls[1], "git") || !strings.Contains(calls[1], "add") {
			t.Errorf("expected git add call: %s", calls[1])
		}
		if !strings.Contains(calls[1], ".ai/") {
			t.Errorf("expected .ai/ in git add: %s", calls[1])
		}
		if !strings.Contains(calls[1], "CLAUDE.md") {
			t.Errorf("expected CLAUDE.md in git add: %s", calls[1])
		}
		if !strings.Contains(calls[1], "AGENTS.md") {
			t.Errorf("expected AGENTS.md in git add: %s", calls[1])
		}
		if !strings.Contains(calls[1], ".github/workflows/") {
			t.Errorf("expected .github/workflows/ in git add: %s", calls[1])
		}
		if !strings.Contains(calls[1], ".goose/recipes/") {
			t.Errorf("expected .goose/recipes/ in git add: %s", calls[1])
		}
		if !strings.Contains(calls[1], "POST_SYNC.md") {
			t.Errorf("expected POST_SYNC.md in git add: %s", calls[1])
		}
	})

	t.Run("git rm error", func(t *testing.T) {
		run := fakeRun("error", fmt.Errorf("exit 1"))
		err := StageSync("/repo", run)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "git rm") {
			t.Errorf("error should mention git rm: %v", err)
		}
	})

	t.Run("git add error", func(t *testing.T) {
		callCount := 0
		run := func(name string, args ...string) (string, error) {
			callCount++
			if callCount == 1 {
				return "", nil // git rm succeeds
			}
			return "error", fmt.Errorf("exit 1") // git add fails
		}
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

		if len(calls) != 4 {
			t.Fatalf("expected 4 calls, got %d: %v", len(calls), calls)
		}

		// Verify git rm was called to remove legacy files.
		if !strings.Contains(calls[0], "git") || !strings.Contains(calls[0], "rm") {
			t.Errorf("first call should be git rm: %s", calls[0])
		}

		// Verify git add was called.
		if !strings.Contains(calls[1], "git") || !strings.Contains(calls[1], "add") {
			t.Errorf("second call should be git add: %s", calls[1])
		}

		// Verify git diff --cached --quiet was called.
		if !strings.Contains(calls[2], "diff") || !strings.Contains(calls[2], "--cached") {
			t.Errorf("third call should be git diff --cached: %s", calls[2])
		}

		// Verify git commit was called with correct message.
		if !strings.Contains(calls[3], "commit") || !strings.Contains(calls[3], "sync .ai/, workflows, and recipes") {
			t.Errorf("fourth call should be git commit with updated message: %s", calls[3])
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

		if len(calls) != 3 {
			t.Fatalf("expected 3 calls (git rm + add + diff check), got %d: %v", len(calls), calls)
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

func TestRestoreAI(t *testing.T) {
	t.Run("restores from backup", func(t *testing.T) {
		root := t.TempDir()
		backupDir := t.TempDir()

		// Create backup content.
		if err := os.MkdirAll(filepath.Join(backupDir, "ai"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(backupDir, "ai", "AGENTS.md"), []byte("original"), 0o644); err != nil {
			t.Fatal(err)
		}

		// Create modified .ai/ in repo.
		if err := os.MkdirAll(filepath.Join(root, ".ai"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".ai", "AGENTS.md"), []byte("modified"), 0o644); err != nil {
			t.Fatal(err)
		}

		err := RestoreAI(root, backupDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(filepath.Join(root, ".ai", "AGENTS.md"))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "original" {
			t.Errorf("expected original content, got %q", data)
		}
	})

	t.Run("empty backup dir is no-op", func(t *testing.T) {
		err := RestoreAI("/repo", "")
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

func TestMigrateBaseToAI(t *testing.T) {
	t.Run("base only — migrates to .ai/", func(t *testing.T) {
		root := t.TempDir()
		// Create base/ directory.
		if err := os.MkdirAll(filepath.Join(root, "base", "skills"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "base", "RULEBOOK.md"), []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}

		run := fakeRun("", nil)
		migrated, err := MigrateBaseToAI(root, run)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !migrated {
			t.Error("expected migration to occur")
		}
	})

	t.Run(".ai/ exists — no migration", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".ai"), 0o755); err != nil {
			t.Fatal(err)
		}

		run := fakeRun("", nil)
		migrated, err := MigrateBaseToAI(root, run)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if migrated {
			t.Error("expected no migration when .ai/ exists")
		}
	})

	t.Run("both exist — no migration", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "base"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(root, ".ai"), 0o755); err != nil {
			t.Fatal(err)
		}

		run := fakeRun("", nil)
		migrated, err := MigrateBaseToAI(root, run)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if migrated {
			t.Error("expected no migration when both exist")
		}
	})

	t.Run("neither exists — no migration", func(t *testing.T) {
		root := t.TempDir()

		run := fakeRun("", nil)
		migrated, err := MigrateBaseToAI(root, run)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if migrated {
			t.Error("expected no migration when neither exists")
		}
	})

	t.Run("git mv failure — returns error", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "base"), 0o755); err != nil {
			t.Fatal(err)
		}

		run := fakeRun("fatal: not a git repository", fmt.Errorf("exit 128"))
		migrated, err := MigrateBaseToAI(root, run)
		if err == nil {
			t.Fatal("expected error when git mv fails")
		}
		if migrated {
			t.Error("expected no migration on failure")
		}
		if !strings.Contains(err.Error(), "migrating base/ to .ai/") {
			t.Errorf("error should mention migration: %v", err)
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
