package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── BackupAI — .github/actions/ present ──────────────────────────────────────

func TestBackupAI_WithActions(t *testing.T) {
	root := t.TempDir()

	actionsDir := filepath.Join(root, ".github", "actions", "install-ai-tools")
	if err := os.MkdirAll(actionsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(actionsDir, "action.yml"), []byte("name: install"), 0o644); err != nil {
		t.Fatal(err)
	}

	backupDir, err := BackupAI(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if backupDir == "" {
		t.Fatal("expected non-empty backupDir")
	}
	t.Cleanup(func() { _ = os.RemoveAll(backupDir) })

	backed := filepath.Join(backupDir, "actions", "install-ai-tools", "action.yml")
	if _, err := os.Stat(backed); err != nil {
		t.Errorf("expected actions backup at %s: %v", backed, err)
	}
}

// ── RestoreAI — actions backup present ───────────────────────────────────────

func TestRestoreAI_WithActions(t *testing.T) {
	root := t.TempDir()
	backupDir := t.TempDir()

	// Populate backup with actions content.
	actionsBackup := filepath.Join(backupDir, "actions", "install-ai-tools")
	if err := os.MkdirAll(actionsBackup, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(actionsBackup, "action.yml"), []byte("name: install"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a modified version in the repo.
	repoDst := filepath.Join(root, ".github", "actions", "install-ai-tools")
	if err := os.MkdirAll(repoDst, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDst, "action.yml"), []byte("name: modified"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := RestoreAI(root, backupDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(repoDst, "action.yml"))
	if err != nil {
		t.Fatalf("expected restored file: %v", err)
	}
	if string(data) != "name: install" {
		t.Errorf("expected restored content, got %q", data)
	}
}

// ── BackupAI — all four directories present ───────────────────────────────────

func TestBackupAI_AllFourDirs(t *testing.T) {
	root := t.TempDir()

	// Create .ai/.
	if err := os.MkdirAll(filepath.Join(root, ".ai"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".ai", "RULEBOOK.md"), []byte("rules"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create .github/workflows/.
	if err := os.MkdirAll(filepath.Join(root, ".github", "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".github", "workflows", "ci.yml"), []byte("on: push"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create .github/actions/.
	if err := os.MkdirAll(filepath.Join(root, ".github", "actions"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".github", "actions", "action.yml"), []byte("name: act"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create .goose/recipes/.
	if err := os.MkdirAll(filepath.Join(root, ".goose", "recipes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".goose", "recipes", "dev.yaml"), []byte("recipe"), 0o644); err != nil {
		t.Fatal(err)
	}

	backupDir, err := BackupAI(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if backupDir == "" {
		t.Fatal("expected non-empty backupDir")
	}
	t.Cleanup(func() { _ = os.RemoveAll(backupDir) })

	for _, check := range []string{
		filepath.Join("ai", "RULEBOOK.md"),
		filepath.Join("workflows", "ci.yml"),
		filepath.Join("actions", "action.yml"),
		filepath.Join("recipes", "dev.yaml"),
	} {
		if _, err := os.Stat(filepath.Join(backupDir, check)); err != nil {
			t.Errorf("expected backup file %s: %v", check, err)
		}
	}
}

// ── RestoreAI — all four directories present ─────────────────────────────────

func TestRestoreAI_AllFourDirs(t *testing.T) {
	root := t.TempDir()
	backupDir := t.TempDir()

	// Populate backup.
	for _, entry := range []struct{ dir, file, content string }{
		{"ai", "RULEBOOK.md", "original-rules"},
		{"workflows", "ci.yml", "original-ci"},
		{"actions", "action.yml", "original-action"},
		{"recipes", "dev.yaml", "original-recipe"},
	} {
		if err := os.MkdirAll(filepath.Join(backupDir, entry.dir), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(backupDir, entry.dir, entry.file), []byte(entry.content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := RestoreAI(root, backupDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all restored.
	for _, check := range []struct {
		path    string
		content string
	}{
		{filepath.Join(".ai", "RULEBOOK.md"), "original-rules"},
		{filepath.Join(".github", "workflows", "ci.yml"), "original-ci"},
		{filepath.Join(".github", "actions", "action.yml"), "original-action"},
		{filepath.Join(".goose", "recipes", "dev.yaml"), "original-recipe"},
	} {
		data, err := os.ReadFile(filepath.Join(root, check.path))
		if err != nil {
			t.Errorf("expected restored file %s: %v", check.path, err)
			continue
		}
		if string(data) != check.content {
			t.Errorf("%s = %q, want %q", check.path, data, check.content)
		}
	}
}

// ── FetchAndExtractTemplate — CopyAI fails (tarball has no .ai/) ─────────────

func TestFetchAndExtractTemplate_CopyAIFails_ReturnsError(t *testing.T) {
	orig := fetchTarballFn
	t.Cleanup(func() { fetchTarballFn = orig })

	// Tarball has only .goose/ content — no .ai/ → CopyAI will fail.
	fetchTarballFn = createFakeTarballFetch(t, map[string]string{
		".goose/recipes/dev.yaml": "recipe",
	})

	root := t.TempDir()
	err := FetchAndExtractTemplate(
		"https://api.github.com/repos/owner/repo/tarball/v1.0.0",
		"owner/repo", "v1.0.0", root, nil)
	if err == nil {
		t.Fatal("expected error when .ai/ missing from tarball, got nil")
	}
	if !strings.Contains(err.Error(), "template does not contain a .ai/ directory") {
		t.Errorf("expected '.ai/ directory' error, got: %v", err)
	}
}

// ── FetchAndExtractTemplate — DeployWorkflows fails ──────────────────────────

func TestFetchAndExtractTemplate_DeployWorkflowsFails_ReturnsError(t *testing.T) {
	orig := fetchTarballFn
	t.Cleanup(func() { fetchTarballFn = orig })

	// Tarball has .ai/ and a workflow inside .ai/.github/workflows/.
	fetchTarballFn = createFakeTarballFetch(t, map[string]string{
		".ai/RULEBOOK.md":                      "rules",
		".ai/.github/workflows/ci.yml":         "on: push",
	})

	root := t.TempDir()
	// Block .github/ from being created by placing a file there.
	if err := os.WriteFile(filepath.Join(root, ".github"), []byte("blocker"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := FetchAndExtractTemplate(
		"https://api.github.com/repos/owner/repo/tarball/v1.0.0",
		"owner/repo", "v1.0.0", root, nil)
	if err == nil {
		t.Fatal("expected error when DeployWorkflows fails, got nil")
	}
	if !strings.Contains(err.Error(), "creating .github/workflows/") {
		t.Errorf("expected 'creating .github/workflows/' error, got: %v", err)
	}
}

// ── FetchAndExtractTemplate — DeployRecipes fails ────────────────────────────

func TestFetchAndExtractTemplate_DeployRecipesFails_ReturnsError(t *testing.T) {
	orig := fetchTarballFn
	t.Cleanup(func() { fetchTarballFn = orig })

	// Tarball has .ai/ (no workflows) and .goose/recipes/.
	fetchTarballFn = createFakeTarballFetch(t, map[string]string{
		".ai/RULEBOOK.md":          "rules",
		".goose/recipes/dev.yaml":  "recipe",
	})

	root := t.TempDir()
	// Block .goose/ from being created.
	if err := os.WriteFile(filepath.Join(root, ".goose"), []byte("blocker"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := FetchAndExtractTemplate(
		"https://api.github.com/repos/owner/repo/tarball/v1.0.0",
		"owner/repo", "v1.0.0", root, nil)
	if err == nil {
		t.Fatal("expected error when DeployRecipes fails, got nil")
	}
	if !strings.Contains(err.Error(), ".goose/recipes/") {
		t.Errorf("expected '.goose/recipes/' in error, got: %v", err)
	}
}

// ── FetchAndExtractTemplate — DeployActions fails ────────────────────────────

func TestFetchAndExtractTemplate_DeployActionsFails_ReturnsError(t *testing.T) {
	orig := fetchTarballFn
	t.Cleanup(func() { fetchTarballFn = orig })

	// Tarball has .ai/ and .ai/.github/actions/ (no workflows or recipes).
	fetchTarballFn = createFakeTarballFetch(t, map[string]string{
		".ai/RULEBOOK.md":                              "rules",
		".ai/.github/actions/install-ai-tools/action.yml": "name: install",
	})

	root := t.TempDir()
	// Block .github/ entirely (affects both workflows and actions).
	if err := os.WriteFile(filepath.Join(root, ".github"), []byte("blocker"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := FetchAndExtractTemplate(
		"https://api.github.com/repos/owner/repo/tarball/v1.0.0",
		"owner/repo", "v1.0.0", root, nil)
	if err == nil {
		t.Fatal("expected error when .github/ blocked, got nil")
	}
	// Should fail at DeployWorkflows or DeployActions — both blocked by same file.
	if !strings.Contains(err.Error(), ".github/") {
		t.Errorf("expected '.github/' in error, got: %v", err)
	}
}

// ── WritePostSyncMD — WriteFile error ────────────────────────────────────────

func TestWritePostSyncMD_WriteFileFails_ReturnsError(t *testing.T) {
	root := t.TempDir()
	// Place a directory at POST_SYNC.md so os.WriteFile fails.
	if err := os.Mkdir(filepath.Join(root, "POST_SYNC.md"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := WritePostSyncMD(root, "# release notes")
	if err == nil {
		t.Fatal("expected error when WriteFile blocked, got nil")
	}
	if !strings.Contains(err.Error(), "writing POST_SYNC.md") {
		t.Errorf("expected 'writing POST_SYNC.md' in error, got: %v", err)
	}
}

// ── DeleteLegacyVersionFiles — Remove fails for non-empty dir ────────────────

func TestDeleteLegacyVersionFiles_RemoveFails_ReturnsError(t *testing.T) {
	root := t.TempDir()
	// Create a non-empty directory named TEMPLATE_SOURCE — os.Remove on a
	// non-empty directory returns ENOTEMPTY, not os.ErrNotExist.
	tplSrcDir := filepath.Join(root, "TEMPLATE_SOURCE")
	if err := os.Mkdir(tplSrcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tplSrcDir, "content.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := DeleteLegacyVersionFiles(root)
	if err == nil {
		t.Fatal("expected error when TEMPLATE_SOURCE is a non-empty directory, got nil")
	}
	if !strings.Contains(err.Error(), "removing TEMPLATE_SOURCE") {
		t.Errorf("expected 'removing TEMPLATE_SOURCE' in error, got: %v", err)
	}
}

// ── DeployRecipes — WriteFile error path ─────────────────────────────────────

func TestDeployRecipes_WriteFileFails_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	repoRoot := t.TempDir()

	// Create a source recipe.
	srcRecipes := filepath.Join(tmpDir, ".goose", "recipes")
	if err := os.MkdirAll(srcRecipes, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcRecipes, "dev.yaml"), []byte("recipe"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Block the destination file by placing a directory there.
	dstRecipes := filepath.Join(repoRoot, ".goose", "recipes")
	if err := os.MkdirAll(dstRecipes, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dstRecipes, "dev.yaml"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := DeployRecipes(tmpDir, repoRoot)
	if err == nil {
		t.Fatal("expected error when WriteFile blocked, got nil")
	}
	if !strings.Contains(err.Error(), "writing recipe") {
		t.Errorf("expected 'writing recipe' in error, got: %v", err)
	}
}

// ── DeployRootFiles — WriteFile error ────────────────────────────────────────

func TestDeployRootFiles_WriteFileFails_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	repoRoot := t.TempDir()

	// Create CLAUDE.md in the extracted template.
	if err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte("# Claude"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Block CLAUDE.md in repoRoot with a directory.
	if err := os.Mkdir(filepath.Join(repoRoot, "CLAUDE.md"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := DeployRootFiles(tmpDir, repoRoot)
	if err == nil {
		t.Fatal("expected error when WriteFile blocked, got nil")
	}
	if !strings.Contains(err.Error(), "writing CLAUDE.md") {
		t.Errorf("expected 'writing CLAUDE.md' in error, got: %v", err)
	}
}

// ── UpdateVersion — WriteFile error ──────────────────────────────────────────

func TestUpdateVersion_WriteFileFails_ReturnsError(t *testing.T) {
	root := t.TempDir()
	// Create .ai/ directory but place config.yml as a DIRECTORY so WriteFile fails.
	// ReadAIConfig will fail (can't read a directory), which falls back to
	// ReadTemplateSource → we provide TEMPLATE_SOURCE so that succeeds.
	// Then yaml.Marshal succeeds, but WriteFile on the directory path fails.
	aiDir := filepath.Join(root, ".ai")
	if err := os.MkdirAll(aiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(aiDir, "config.yml"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Provide TEMPLATE_SOURCE so ReadTemplateSource succeeds.
	if err := os.WriteFile(filepath.Join(root, "TEMPLATE_SOURCE"), []byte("owner/repo"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := UpdateVersion(root, "v2.0.0")
	if err == nil {
		t.Fatal("expected error when WriteFile blocked, got nil")
	}
	if !strings.Contains(err.Error(), "writing .ai/config.yml") {
		t.Errorf("expected 'writing .ai/config.yml' in error, got: %v", err)
	}
}

// ── UpdateVersion — ReadTemplateSource fallback error ────────────────────────

func TestUpdateVersion_ReadAIConfigFails_ReadTemplateSourceFails_ReturnsError(t *testing.T) {
	root := t.TempDir()
	// No .ai/config.yml → ReadAIConfig fails. No TEMPLATE_SOURCE → ReadTemplateSource fails.

	err := UpdateVersion(root, "v2.0.0")
	if err == nil {
		t.Fatal("expected error when no config and no TEMPLATE_SOURCE, got nil")
	}
	if !strings.Contains(err.Error(), "reading template source for config.yml") {
		t.Errorf("expected 'reading template source for config.yml' in error, got: %v", err)
	}
}

// ── CopyAI — CopyDir fails when src is unreadable ────────────────────────────

func TestCopyAI_CopyDirFails_ReturnsError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root can read all files — chmod test not meaningful")
	}

	tmpDir := t.TempDir()
	repoRoot := t.TempDir()

	// Create .ai/ in tmpDir with content.
	aiSrc := filepath.Join(tmpDir, ".ai")
	if err := os.Mkdir(aiSrc, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(aiSrc, "RULEBOOK.md"), []byte("rules"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Make .ai/ unreadable so CopyDir fails.
	if err := os.Chmod(aiSrc, 0o000); err != nil { // NOSONAR: intentionally setting restrictive permissions to simulate error path in tests
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(aiSrc, 0o755) })

	err := CopyAI(tmpDir, repoRoot)
	if err == nil {
		t.Fatal("expected error when .ai/ is unreadable, got nil")
	}
	_ = fmt.Sprintf("%v", err) // ensure error string is used
}

// ── DeployWorkflows — WriteFile error path ───────────────────────────────────

func TestDeployWorkflows_WriteFileFails_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	repoRoot := t.TempDir()

	// Create source workflow in .ai/.github/workflows/.
	srcWF := filepath.Join(tmpDir, ".ai", ".github", "workflows")
	if err := os.MkdirAll(srcWF, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcWF, "ci.yml"), []byte("on: push"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create destination directory but block the workflow file with a directory.
	dstWF := filepath.Join(repoRoot, ".github", "workflows")
	if err := os.MkdirAll(dstWF, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dstWF, "ci.yml"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := DeployWorkflows(tmpDir, repoRoot, nil)
	if err == nil {
		t.Fatal("expected error when WriteFile blocked, got nil")
	}
	if !strings.Contains(err.Error(), "writing workflow ci.yml") {
		t.Errorf("expected 'writing workflow ci.yml' in error, got: %v", err)
	}
}
