package sync

import (
	"fmt"
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

func TestCloneTemplate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tmpDir := t.TempDir()
		run := fakeRun("Cloning...", nil)
		err := CloneTemplate("owner/repo", tmpDir, run)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("git clone error", func(t *testing.T) {
		tmpDir := t.TempDir()
		run := fakeRun("fatal: not found", fmt.Errorf("exit 128"))
		err := CloneTemplate("owner/repo", tmpDir, run)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "git clone template") {
			t.Errorf("error should mention git clone: %v", err)
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

		err := DeployWorkflows(tmpDir, repoRoot)
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

		err := DeployWorkflows(tmpDir, repoRoot)
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

		err := DeployWorkflows(tmpDir, repoRoot)
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
			{output: "", err: nil},                    // git diff base/
			{output: "base/new-file.md\n", err: nil},  // ls-files base/
			{output: "", err: nil},                    // git diff .github/workflows/
			{output: "", err: nil},                    // ls-files .github/workflows/
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
			{output: "", err: nil},                                     // git diff base/
			{output: "", err: nil},                                     // ls-files base/
			{output: "diff --git a/.github/workflows/ci.yml", err: nil}, // git diff .github/workflows/
			{output: "", err: nil},                                     // ls-files .github/workflows/
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
			{output: "", err: nil},                                                    // git diff base/
			{output: "", err: nil},                                                    // ls-files base/
			{output: "", err: nil},                                                    // git diff .github/workflows/
			{output: ".github/workflows/new-pipeline.yml\n", err: nil},                // ls-files .github/workflows/
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
		if !strings.Contains(calls[2], "commit") || !strings.Contains(calls[2], "sync base/ and workflows") {
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
