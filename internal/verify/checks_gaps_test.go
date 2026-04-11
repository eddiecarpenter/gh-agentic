package verify

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
)

// ── CheckAISkills — git diff fails (lines 241-244) ───────────────────────────

func TestCheckAISkills_GitDiffFails_ReturnsWarning(t *testing.T) {
	root := t.TempDir()
	// Create .ai/skills/ so the directory-not-found check passes.
	if err := os.MkdirAll(filepath.Join(root, ".ai", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	fakeRun := func(name string, args ...string) (string, error) {
		return "", fmt.Errorf("git: not a repository")
	}

	result := CheckAISkills(root, fakeRun)
	if result.Status != Warning {
		t.Errorf("expected Warning when git diff fails, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "could not check git status") {
		t.Errorf("expected 'could not check git status', got: %s", result.Message)
	}
}

// ── CheckWorkflows — .ai dir present, content comparison paths ───────────────
// Lines 312-315 (ReadDir fails), 336-337 (ai ReadFile fails → missing)

func TestCheckWorkflows_WithAIDir_ContentMatches_ReturnsPass(t *testing.T) {
	root := t.TempDir()
	content := []byte("# canonical workflow\n")
	// Set up identical files in .ai/.github/workflows/ and .github/workflows/.
	aiDir := filepath.Join(root, ".ai", ".github", "workflows")
	if err := os.MkdirAll(aiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(aiDir, "build-and-test.yml"), content, 0o644); err != nil {
		t.Fatal(err)
	}
	destDir := filepath.Join(root, ".github", "workflows")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destDir, "build-and-test.yml"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	result := CheckWorkflows(root, bootstrap.OwnerTypeUser)
	if result.Status != Pass {
		t.Errorf("expected Pass when content matches, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckWorkflows_WithAIDir_ContentDiffers_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	aiDir := filepath.Join(root, ".ai", ".github", "workflows")
	if err := os.MkdirAll(aiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(aiDir, "build-and-test.yml"), []byte("# canonical\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	destDir := filepath.Join(root, ".github", "workflows")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destDir, "build-and-test.yml"), []byte("# modified\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := CheckWorkflows(root, bootstrap.OwnerTypeUser)
	if result.Status != Fail {
		t.Errorf("expected Fail when content differs, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "content differs") {
		t.Errorf("expected 'content differs' in message, got: %s", result.Message)
	}
}

func TestCheckWorkflows_WithAIDir_DeployedFileMissing_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	// .ai has the file but .github/workflows/ does not.
	aiDir := filepath.Join(root, ".ai", ".github", "workflows")
	if err := os.MkdirAll(aiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(aiDir, "build-and-test.yml"), []byte("# ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".github", "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}

	result := CheckWorkflows(root, bootstrap.OwnerTypeUser)
	if result.Status != Fail {
		t.Errorf("expected Fail when deployed file missing, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "missing") {
		t.Errorf("expected 'missing' in message, got: %s", result.Message)
	}
}

// ── CheckProjectCollaborator — user type, no project (lines 814-817) ──────────

func TestCheckProjectCollaborator_UserType_NoProject_ReturnsFail(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return "", fmt.Errorf("project not found")
	}

	result := CheckProjectCollaborator("owner", "repo", "bot-user", bootstrap.OwnerTypeUser, fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail when no project for user type, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "no GitHub Project found") {
		t.Errorf("expected 'no GitHub Project found', got: %s", result.Message)
	}
}

// ── CheckProjectItemStatuses — error paths (lines 708-741) ───────────────────

func TestCheckProjectItemStatuses_NoProject_ReturnsFail(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return "", fmt.Errorf("no project")
	}

	result := CheckProjectItemStatuses("owner", "repo", fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail when no project, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "no GitHub Project found") {
		t.Errorf("expected 'no GitHub Project found', got: %s", result.Message)
	}
}

func TestCheckProjectItemStatuses_FieldFetchFails_ReturnsFail(t *testing.T) {
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			return projectListJSON, nil
		}
		return "", fmt.Errorf("field query failed")
	}

	result := CheckProjectItemStatuses("owner", "repo", fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail when field fetch fails, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "failed to fetch Status field ID") {
		t.Errorf("expected 'failed to fetch Status field ID', got: %s", result.Message)
	}
}

func TestCheckProjectItemStatuses_FieldNull_ReturnsFail(t *testing.T) {
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			return projectListJSON, nil
		}
		return "null\n", nil // field ID null
	}

	result := CheckProjectItemStatuses("owner", "repo", fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail for null field ID, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "Status field not found") {
		t.Errorf("expected 'Status field not found', got: %s", result.Message)
	}
}

func TestCheckProjectItemStatuses_FetchItemsFails_ReturnsFail(t *testing.T) {
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			return projectListJSON, nil
		}
		if callCount == 2 {
			return "FIELD_123\n", nil // field ID OK
		}
		return "", fmt.Errorf("items: server error")
	}

	result := CheckProjectItemStatuses("owner", "repo", fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail when items fetch fails, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "failed to fetch project items") {
		t.Errorf("expected 'failed to fetch project items', got: %s", result.Message)
	}
}

// ── CheckLabels — JSON parse fails (lines 433-436) ───────────────────────────

func TestCheckLabels_JSONParseFails_ReturnsFail(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return "not-json", nil
	}

	result := CheckLabels("owner/repo", fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail when JSON parse fails, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "failed to parse label JSON") {
		t.Errorf("expected 'failed to parse label JSON', got: %s", result.Message)
	}
}

// ── CheckProjectStatus — load template fails (lines 539-542) ─────────────────

func TestCheckProjectStatus_LoadTemplateFails_ReturnsFail(t *testing.T) {
	root := t.TempDir() // no project-template.json
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			return projectListJSON, nil // project found
		}
		return "OPT_1|Backlog\n", nil // options
	}

	result := CheckProjectStatus("owner", "repo", root, fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail when template missing, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "project template") {
		t.Errorf("expected 'project template' in message, got: %s", result.Message)
	}
}

// Ensure bytes import is referenced (used by CheckWorkflows production code).
var _ = bytes.Equal
