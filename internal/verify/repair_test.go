package verify

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepairSkillsDir_CreatesDirectoryAndFile(t *testing.T) {
	root := t.TempDir()
	fakeRun := func(name string, args ...string) (string, error) {
		return "", nil
	}

	result := RepairSkillsDir(root, fakeRun)
	if result.Status != Pass {
		t.Fatalf("expected Pass, got %v: %s", result.Status, result.Message)
	}

	// Verify skills/.gitkeep exists.
	info, err := os.Stat(filepath.Join(root, "skills", ".gitkeep"))
	if err != nil {
		t.Fatalf("expected skills/.gitkeep to exist: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("expected .gitkeep to be empty, got %d bytes", info.Size())
	}
}

func TestRepairSkillsDir_StagesViaGitAdd(t *testing.T) {
	root := t.TempDir()
	var gitAddCalled bool
	fakeRun := func(name string, args ...string) (string, error) {
		if name == "bash" && len(args) > 0 {
			script := strings.Join(args, " ")
			if strings.Contains(script, "git add skills/.gitkeep") {
				gitAddCalled = true
			}
		}
		return "", nil
	}

	result := RepairSkillsDir(root, fakeRun)
	if result.Status != Pass {
		t.Fatalf("expected Pass, got %v: %s", result.Status, result.Message)
	}
	if !gitAddCalled {
		t.Error("expected git add skills/.gitkeep to be called")
	}
}

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
// gh-notify LaunchAgent repair tests
// ──────────────────────────────────────────────────────────────────────────────

func TestRepairGhNotify_Success_ReturnsPass(t *testing.T) {
	root := t.TempDir()
	var calledWith []string
	fakeRun := func(name string, args ...string) (string, error) {
		calledWith = append(calledWith, name)
		calledWith = append(calledWith, args...)
		return "", nil
	}

	result := RepairGhNotify(root, fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
	if result.Message != "gh-notify installed and running" {
		t.Errorf("unexpected message: %s", result.Message)
	}
	expectedScript := filepath.Join(root, "base", "scripts", "install-gh-notify.sh")
	if len(calledWith) < 2 || calledWith[0] != "bash" || calledWith[1] != expectedScript {
		t.Errorf("expected run(bash, %s), got %v", expectedScript, calledWith)
	}
}

func TestRepairGhNotify_ScriptFails_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	fakeRun := func(name string, args ...string) (string, error) {
		return "", fmt.Errorf("script exited with code 1")
	}

	result := RepairGhNotify(root, fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "install script failed") {
		t.Errorf("expected error detail in message, got: %s", result.Message)
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

func TestRepairGooseRecipes_AlwaysOverwrites(t *testing.T) {
	root := t.TempDir()
	// Write TEMPLATE_SOURCE so the repair doesn't fail early.
	if err := os.WriteFile(filepath.Join(root, "TEMPLATE_SOURCE"), []byte("owner/template"), 0o644); err != nil {
		t.Fatal(err)
	}
	recipesDir := filepath.Join(root, ".goose", "recipes")

	// Create all expected files with old content.
	if err := os.MkdirAll(recipesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range expectedRecipeYAMLs {
		if err := os.WriteFile(filepath.Join(recipesDir, name), []byte("old-content"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Stub fetchFileFn to return updated content.
	origFetch := fetchFileFn
	defer func() { fetchFileFn = origFetch }()
	fetchFileFn = func(repo, path string) ([]byte, error) {
		return []byte("updated-content"), nil
	}

	result := RepairGooseRecipes(root)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}

	// Verify every recipe was overwritten with the new content.
	for _, name := range expectedRecipeYAMLs {
		data, err := os.ReadFile(filepath.Join(recipesDir, name))
		if err != nil {
			t.Fatalf("failed to read %s: %v", name, err)
		}
		if string(data) != "updated-content" {
			t.Errorf("%s: expected 'updated-content', got %q", name, string(data))
		}
	}
}

func TestRepairGooseRecipes_MissingSource_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	// No TEMPLATE_SOURCE — repair should fail with a clear message.
	result := RepairGooseRecipes(root)
	if result.Status != Fail {
		t.Errorf("expected Fail when TEMPLATE_SOURCE missing, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "TEMPLATE_SOURCE") {
		t.Error("message should mention TEMPLATE_SOURCE")
	}
}

func TestRepairWorkflows_FromBase_CopiesAndStages(t *testing.T) {
	root := t.TempDir()

	// Create base/.github/workflows/ with content.
	baseWfDir := filepath.Join(root, "base", ".github", "workflows")
	if err := os.MkdirAll(baseWfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(baseWfDir, "pipeline.yml"), []byte("pipeline-v2"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create .github/workflows/ with outdated content.
	wfDir := filepath.Join(root, ".github", "workflows")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, "pipeline.yml"), []byte("pipeline-v1"), 0o644); err != nil {
		t.Fatal(err)
	}

	var gitAddCalled bool
	fakeRun := func(name string, args ...string) (string, error) {
		if name == "bash" && len(args) > 0 && strings.Contains(args[len(args)-1], "git add .github/workflows/") {
			gitAddCalled = true
		}
		return "", nil
	}

	result := RepairWorkflows(root, fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}

	// Verify content was overwritten.
	data, err := os.ReadFile(filepath.Join(wfDir, "pipeline.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "pipeline-v2" {
		t.Errorf("expected updated content, got %q", data)
	}

	// Verify git add was called.
	if !gitAddCalled {
		t.Error("expected git add .github/workflows/ to be called")
	}
}

func TestRepairWorkflows_Fallback_AllPresent(t *testing.T) {
	root := t.TempDir()
	workflowsDir := filepath.Join(root, ".github", "workflows")

	// No base/.github/workflows/ — fallback mode.
	// Create all expected files.
	if err := os.MkdirAll(workflowsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range expectedWorkflowYMLs {
		if err := os.WriteFile(filepath.Join(workflowsDir, name), []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	fakeRun := func(name string, args ...string) (string, error) {
		return "", nil
	}

	result := RepairWorkflows(root, fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass when all files present, got %v: %s", result.Status, result.Message)
	}
}

func TestRepairWorkflows_MissingFiles_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	fakeRun := func(name string, args ...string) (string, error) {
		return "", nil
	}
	result := RepairWorkflows(root, fakeRun)
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

	// Should have created 9 labels (11 standard - 2 existing).
	if len(createdLabels) != 9 {
		t.Errorf("expected 9 labels created, got %d: %v", len(createdLabels), createdLabels)
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

// ──────────────────────────────────────────────────────────────────────────────
// RepairProjectStatus tests
// ──────────────────────────────────────────────────────────────────────────────

func TestRepairProjectStatus_Success_ReturnsPass(t *testing.T) {
	root := t.TempDir()
	writeTestProjectTemplate(t, root)
	callCount := 0
	var mutationCalled bool
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		switch callCount {
		case 1:
			// Resolve project node ID via gh project list.
			return projectListJSON, nil
		case 2:
			// Fetch Status field ID.
			return "FIELD_456", nil
		case 3:
			// Update mutation.
			mutationCalled = true
			return `{"data":{}}`, nil
		case 4:
			// fetchStatusOptionMap: fetch status options with id|name.
			return "OPT_1|Backlog\nOPT_2|Scoping\nOPT_3|Scheduled\nOPT_4|In Design\nOPT_5|In Development\nOPT_6|In Review\nOPT_7|Done", nil
		case 5:
			// fetchAllProjectItems: return empty items list (no items to resync).
			return `{"data":{"node":{"items":{"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[]}}}}`, nil
		}
		return "", nil
	}

	result := RepairProjectStatus("owner", root, fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
	if !mutationCalled {
		t.Error("expected update mutation to be called")
	}
}

func TestRepairProjectStatus_NoProject_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	writeTestProjectTemplate(t, root)
	fakeRun := func(name string, args ...string) (string, error) {
		return "", fmt.Errorf("no project found")
	}

	result := RepairProjectStatus("owner", root, fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v: %s", result.Status, result.Message)
	}
}

func TestRepairProjectStatus_MutationFails_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	writeTestProjectTemplate(t, root)
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		switch callCount {
		case 1:
			return projectListJSON, nil
		case 2:
			return "FIELD_456", nil
		case 3:
			return "mutation error", fmt.Errorf("mutation failed")
		}
		return "", nil
	}

	result := RepairProjectStatus("owner", root, fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v: %s", result.Status, result.Message)
	}
}

func TestRepairProjectStatus_MissingTemplate_ReturnsFail(t *testing.T) {
	root := t.TempDir() // No base/project-template.json.
	fakeRun := func(name string, args ...string) (string, error) {
		t.Fatal("run should not be called when template is missing")
		return "", nil
	}

	result := RepairProjectStatus("owner", root, fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "project template") {
		t.Errorf("expected message about project template, got: %s", result.Message)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// RepairProjectCollaborator tests
// ──────────────────────────────────────────────────────────────────────────────

func TestRepairProjectCollaborator_Success_ReturnsPass(t *testing.T) {
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		switch callCount {
		case 1:
			return projectListJSON, nil // resolve project node ID via gh project list
		case 2:
			return "USER_NODE_456", nil // resolve user node ID
		case 3:
			return `{"data":{}}`, nil // mutation success
		}
		return "", nil
	}

	result := RepairProjectCollaborator("owner", "goose-agent", fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
}

func TestRepairProjectCollaborator_EmptyAgentUser_ReturnsPass(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		t.Fatal("run should not be called when agent user is empty")
		return "", nil
	}

	result := RepairProjectCollaborator("owner", "", fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
}

func TestRepairProjectCollaborator_UserResolutionFails_ReturnsFail(t *testing.T) {
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			return projectListJSON, nil
		}
		return "", fmt.Errorf("user not found")
	}

	result := RepairProjectCollaborator("owner", "goose-agent", fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v: %s", result.Status, result.Message)
	}
}

func TestRepairProjectCollaborator_MutationFails_ReturnsFail(t *testing.T) {
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		switch callCount {
		case 1:
			return projectListJSON, nil
		case 2:
			return "USER_NODE_456", nil
		case 3:
			return "error", fmt.Errorf("mutation failed")
		}
		return "", nil
	}

	result := RepairProjectCollaborator("owner", "goose-agent", fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v: %s", result.Status, result.Message)
	}
}
