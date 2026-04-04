package verify

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepairCLAUDEMD_CreatesFile(t *testing.T) {
	root := t.TempDir()
	result := RepairCLAUDEMD(root)
	if result.Status != Pass {
		t.Fatalf("expected Pass, got %v: %s", result.Status, result.Message)
	}

	data, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "@base/AGENTS.md") {
		t.Error("CLAUDE.md should reference @base/AGENTS.md")
	}
	if !strings.Contains(content, "@AGENTS.local.md") {
		t.Error("CLAUDE.md should reference @AGENTS.local.md")
	}
}

func TestRepairAGENTSLocalMD_CreatesFile(t *testing.T) {
	root := t.TempDir()
	result := RepairAGENTSLocalMD(root)
	if result.Status != Pass {
		t.Fatalf("expected Pass, got %v: %s", result.Status, result.Message)
	}

	data, err := os.ReadFile(filepath.Join(root, "AGENTS.local.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "Local Overrides") {
		t.Error("AGENTS.local.md should contain 'Local Overrides'")
	}
	if !strings.Contains(content, "TODO") {
		t.Error("AGENTS.local.md should contain TODO markers")
	}
}

func TestRepairTEMPLATESOURCE_WithConfirm_CreatesFile(t *testing.T) {
	root := t.TempDir()
	confirmFn := func(prompt string) (string, error) {
		return "eddiecarpenter/agentic-development", nil
	}
	result := RepairTEMPLATESOURCE(root, confirmFn)
	if result.Status != Pass {
		t.Fatalf("expected Pass, got %v: %s", result.Status, result.Message)
	}

	data, err := os.ReadFile(filepath.Join(root, "TEMPLATE_SOURCE"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "eddiecarpenter/agentic-development" {
		t.Errorf("unexpected content: %s", string(data))
	}
}

func TestRepairTEMPLATESOURCE_NilConfirm_ReturnsWarning(t *testing.T) {
	root := t.TempDir()
	result := RepairTEMPLATESOURCE(root, nil)
	if result.Status != Warning {
		t.Errorf("expected Warning with nil confirm, got %v", result.Status)
	}
}

func TestRepairTEMPLATESOURCE_EmptyInput_ReturnsWarning(t *testing.T) {
	root := t.TempDir()
	confirmFn := func(prompt string) (string, error) {
		return "", nil
	}
	result := RepairTEMPLATESOURCE(root, confirmFn)
	if result.Status != Warning {
		t.Errorf("expected Warning for empty input, got %v", result.Status)
	}
}

func TestRepairTEMPLATEVERSION_Success(t *testing.T) {
	root := t.TempDir()
	// Write TEMPLATE_SOURCE first — required by the repair.
	if err := os.WriteFile(filepath.Join(root, "TEMPLATE_SOURCE"), []byte("owner/repo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	fakeRun := func(name string, args ...string) (string, error) {
		return "v1.2.3\n", nil
	}

	result := RepairTEMPLATEVERSION(root, fakeRun)
	if result.Status != Pass {
		t.Fatalf("expected Pass, got %v: %s", result.Status, result.Message)
	}

	data, err := os.ReadFile(filepath.Join(root, "TEMPLATE_VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "v1.2.3" {
		t.Errorf("unexpected version: %s", string(data))
	}
}

func TestRepairTEMPLATEVERSION_MissingSource_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	fakeRun := func(name string, args ...string) (string, error) {
		return "", nil
	}
	result := RepairTEMPLATEVERSION(root, fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail without TEMPLATE_SOURCE, got %v", result.Status)
	}
}

func TestRepairREPOSMD_CreatesFile(t *testing.T) {
	root := t.TempDir()
	result := RepairREPOSMD(root)
	if result.Status != Pass {
		t.Fatalf("expected Pass, got %v: %s", result.Status, result.Message)
	}

	data, err := os.ReadFile(filepath.Join(root, "REPOS.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "Repository Registry") {
		t.Error("REPOS.md should contain 'Repository Registry'")
	}
	if !strings.Contains(content, "embedded topology") {
		t.Error("REPOS.md should contain 'embedded topology'")
	}
}

func TestRepairREADMEMD_CreatesFile(t *testing.T) {
	root := t.TempDir()
	result := RepairREADMEMD(root)
	if result.Status != Pass {
		t.Fatalf("expected Pass, got %v: %s", result.Status, result.Message)
	}

	data, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "# Project") {
		t.Error("README.md should contain project heading")
	}
	if !strings.Contains(content, "agentic development framework") {
		t.Error("README.md should reference agentic development framework")
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Directory integrity repair tests
// ──────────────────────────────────────────────────────────────────────────────

func TestRepairBaseDir_UserConfirms_ReturnsPass(t *testing.T) {
	root := t.TempDir()
	// Create base/ so the repair takes the git-checkout path (not sync).
	if err := os.MkdirAll(filepath.Join(root, "base"), 0o755); err != nil {
		t.Fatal(err)
	}
	fakeRun := func(name string, args ...string) (string, error) {
		return "", nil
	}
	confirmFn := func(prompt string) (bool, error) {
		return true, nil
	}

	result := RepairBaseDir(root, fakeRun, confirmFn)
	if result.Status != Pass {
		t.Errorf("expected Pass after confirmed repair, got %v: %s", result.Status, result.Message)
	}
}

func TestRepairBaseDir_UserDeclines_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	fakeRun := func(name string, args ...string) (string, error) {
		return "", nil
	}
	confirmFn := func(prompt string) (bool, error) {
		return false, nil
	}

	result := RepairBaseDir(root, fakeRun, confirmFn)
	if result.Status != Fail {
		t.Errorf("expected Fail when user declines, got %v: %s", result.Status, result.Message)
	}
}

func TestRepairBaseRecipes_UserConfirms_ReturnsPass(t *testing.T) {
	root := t.TempDir()
	fakeRun := func(name string, args ...string) (string, error) {
		return "", nil
	}
	confirmFn := func(prompt string) (bool, error) {
		return true, nil
	}

	result := RepairBaseRecipes(root, fakeRun, confirmFn)
	if result.Status != Pass {
		t.Errorf("expected Pass after confirmed repair, got %v: %s", result.Status, result.Message)
	}
}

func TestRepairBaseRecipes_UserDeclines_ReturnsWarning(t *testing.T) {
	root := t.TempDir()
	fakeRun := func(name string, args ...string) (string, error) {
		return "", nil
	}
	confirmFn := func(prompt string) (bool, error) {
		return false, nil
	}

	result := RepairBaseRecipes(root, fakeRun, confirmFn)
	if result.Status != Warning {
		t.Errorf("expected Warning when user declines, got %v: %s", result.Status, result.Message)
	}
}

func TestRepairGooseRecipes_CreatesDir(t *testing.T) {
	root := t.TempDir()
	recipesDir := filepath.Join(root, ".goose", "recipes")

	// Create all expected files.
	if err := os.MkdirAll(recipesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range expectedRecipeYAMLs {
		if err := os.WriteFile(filepath.Join(recipesDir, name), []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	result := RepairGooseRecipes(root)
	if result.Status != Pass {
		t.Errorf("expected Pass when all files present, got %v: %s", result.Status, result.Message)
	}
}

func TestRepairGooseRecipes_MissingFiles_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	// Don't create any recipe files.
	result := RepairGooseRecipes(root)
	if result.Status != Fail {
		t.Errorf("expected Fail for missing recipe files, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "gh agentic sync") {
		t.Error("message should suggest running 'gh agentic sync'")
	}
}

func TestRepairWorkflows_CreatesDir(t *testing.T) {
	root := t.TempDir()
	workflowsDir := filepath.Join(root, ".github", "workflows")

	// Create all expected files.
	if err := os.MkdirAll(workflowsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range expectedWorkflowYMLs {
		if err := os.WriteFile(filepath.Join(workflowsDir, name), []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	result := RepairWorkflows(root)
	if result.Status != Pass {
		t.Errorf("expected Pass when all files present, got %v: %s", result.Status, result.Message)
	}
}

func TestRepairWorkflows_MissingFiles_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	result := RepairWorkflows(root)
	if result.Status != Fail {
		t.Errorf("expected Fail for missing workflow files, got %v: %s", result.Status, result.Message)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// GitHub remote repair tests
// ──────────────────────────────────────────────────────────────────────────────

func TestRepairLabels_CreatesMissingOnly(t *testing.T) {
	// Existing labels: only requirement and feature.
	labelsJSON := `[{"name":"requirement"},{"name":"feature"}]`
	var createdLabels []string

	fakeRun := func(name string, args ...string) (string, error) {
		// First call is gh label list, subsequent calls are gh label create.
		if len(args) > 0 && args[0] == "label" && args[1] == "list" {
			return labelsJSON, nil
		}
		if len(args) > 0 && args[0] == "label" && args[1] == "create" {
			createdLabels = append(createdLabels, args[2])
			return "", nil
		}
		return "", nil
	}

	result := RepairLabels("owner/repo", fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}

	// Should have created 7 labels (9 standard - 2 existing).
	if len(createdLabels) != 7 {
		t.Errorf("expected 7 labels created, got %d: %v", len(createdLabels), createdLabels)
	}

	// Verify requirement and feature were NOT in the created list.
	for _, l := range createdLabels {
		if l == "requirement" || l == "feature" {
			t.Errorf("should not have created existing label %q", l)
		}
	}
}

func TestRepairLabels_AllPresent_ReturnsPass(t *testing.T) {
	labelsJSON := `[{"name":"requirement"},{"name":"feature"},{"name":"task"},{"name":"backlog"},{"name":"draft"},{"name":"in-design"},{"name":"in-development"},{"name":"in-review"},{"name":"done"}]`
	fakeRun := func(name string, args ...string) (string, error) {
		return labelsJSON, nil
	}

	result := RepairLabels("owner/repo", fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass when all labels present, got %v: %s", result.Status, result.Message)
	}
}

func TestRepairLabels_CreateFails_ReturnsFail(t *testing.T) {
	labelsJSON := `[]`
	fakeRun := func(name string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "label" && args[1] == "list" {
			return labelsJSON, nil
		}
		return "", fmt.Errorf("create failed")
	}

	result := RepairLabels("owner/repo", fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail when creates fail, got %v: %s", result.Status, result.Message)
	}
}

func TestRepairProject_Success(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return "", nil
	}

	result := RepairProject("owner", "my-project", fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
}

func TestRepairProject_Fails_ReturnsFail(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return "", fmt.Errorf("create failed")
	}

	result := RepairProject("owner", "my-project", fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v: %s", result.Status, result.Message)
	}
}
