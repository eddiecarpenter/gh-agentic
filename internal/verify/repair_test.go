package verify

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

	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
	"github.com/eddiecarpenter/gh-agentic/internal/tarball"
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
		return "eddiecarpenter/ai-native-delivery", nil
	}
	result := RepairTEMPLATESOURCE(root, confirmFn)
	if result.Status != Pass {
		t.Fatalf("expected Pass, got %v: %s", result.Status, result.Message)
	}

	data, err := os.ReadFile(filepath.Join(root, "TEMPLATE_SOURCE"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "eddiecarpenter/ai-native-delivery" {
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
	writeTemplateConfig(t, root, "owner/template", "v1.0.0")
	confirmFn := func(prompt string) (bool, error) {
		return true, nil
	}
	fetch := buildTestFetchFunc(t, map[string]string{
		"base/skills/session-init.md": "# Session Init",
		"base/skills/dev-session.md":  "# Dev Session",
	})

	result := RepairBaseRecipes(root, confirmFn, fetch)
	if result.Status != Pass {
		t.Errorf("expected Pass after confirmed repair, got %v: %s", result.Status, result.Message)
	}

	// Verify extracted file exists.
	data, err := os.ReadFile(filepath.Join(root, "base", "skills", "session-init.md"))
	if err != nil {
		t.Fatalf("expected extracted file: %v", err)
	}
	if string(data) != "# Session Init" {
		t.Errorf("unexpected content: %q", string(data))
	}
}

func TestRepairBaseRecipes_UserDeclines_ReturnsWarning(t *testing.T) {
	root := t.TempDir()
	writeTemplateConfig(t, root, "owner/template", "v1.0.0")
	confirmFn := func(prompt string) (bool, error) {
		return false, nil
	}

	result := RepairBaseRecipes(root, confirmFn, nil)
	if result.Status != Warning {
		t.Errorf("expected Warning when user declines, got %v: %s", result.Status, result.Message)
	}
}

func TestRepairBaseRecipes_MissingTemplateConfig_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	// No TEMPLATE_SOURCE or TEMPLATE_VERSION — should fail.
	confirmFn := func(prompt string) (bool, error) {
		return true, nil
	}

	result := RepairBaseRecipes(root, confirmFn, nil)
	if result.Status != Fail {
		t.Errorf("expected Fail when template config missing, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "TEMPLATE_SOURCE") {
		t.Errorf("message should mention TEMPLATE_SOURCE, got: %s", result.Message)
	}
}

func TestRepairGooseRecipes_ExtractsFromTarball(t *testing.T) {
	root := t.TempDir()
	writeTemplateConfig(t, root, "owner/template", "v1.0.0")

	// Build a tarball with recipe files.
	recipeFiles := make(map[string]string)
	for _, name := range expectedRecipeYAMLs {
		recipeFiles[".goose/recipes/"+name] = "name: " + name
	}
	fetch := buildTestFetchFunc(t, recipeFiles)

	result := RepairGooseRecipes(root, fetch)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}

	// Verify every recipe was extracted.
	for _, name := range expectedRecipeYAMLs {
		data, err := os.ReadFile(filepath.Join(root, ".goose", "recipes", name))
		if err != nil {
			t.Fatalf("failed to read %s: %v", name, err)
		}
		expected := "name: " + name
		if string(data) != expected {
			t.Errorf("%s: expected %q, got %q", name, expected, string(data))
		}
	}
}

func TestRepairGooseRecipes_MissingConfig_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	// No TEMPLATE_SOURCE — repair should fail with a clear message.
	result := RepairGooseRecipes(root, nil)
	if result.Status != Fail {
		t.Errorf("expected Fail when template config missing, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "TEMPLATE_SOURCE") {
		t.Error("message should mention TEMPLATE_SOURCE")
	}
}

func TestRepairGooseRecipes_TarballError_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	writeTemplateConfig(t, root, "owner/template", "v1.0.0")

	result := RepairGooseRecipes(root, failingFetchFunc("download failed"))
	if result.Status != Fail {
		t.Errorf("expected Fail on tarball error, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "tarball extraction failed") {
		t.Errorf("expected tarball error in message, got: %s", result.Message)
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

	result := RepairWorkflows(root, bootstrap.OwnerTypeOrg, fakeRun, nil)
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

func TestRepairWorkflows_Fallback_ExtractsFromTarball(t *testing.T) {
	root := t.TempDir()
	writeTemplateConfig(t, root, "owner/template", "v1.0.0")

	// No base/.github/workflows/ — fallback to tarball.
	workflowFiles := make(map[string]string)
	for _, name := range expectedWorkflowYMLs {
		workflowFiles[".github/workflows/"+name] = "on: push # " + name
	}
	fetch := buildTestFetchFunc(t, workflowFiles)

	fakeRun := func(name string, args ...string) (string, error) {
		return "", nil
	}

	result := RepairWorkflows(root, bootstrap.OwnerTypeOrg, fakeRun, fetch)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}

	// Verify workflows were extracted.
	for _, name := range expectedWorkflowYMLs {
		data, err := os.ReadFile(filepath.Join(root, ".github", "workflows", name))
		if err != nil {
			t.Errorf("expected workflow %s: %v", name, err)
			continue
		}
		expected := "on: push # " + name
		if string(data) != expected {
			t.Errorf("%s: expected %q, got %q", name, expected, string(data))
		}
	}
}

func TestRepairWorkflows_Fallback_MissingConfig_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	// No base/ and no TEMPLATE_SOURCE — should fail.
	fakeRun := func(name string, args ...string) (string, error) {
		return "", nil
	}
	result := RepairWorkflows(root, bootstrap.OwnerTypeOrg, fakeRun, nil)
	if result.Status != Fail {
		t.Errorf("expected Fail when template config missing, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "TEMPLATE_SOURCE") {
		t.Errorf("expected TEMPLATE_SOURCE in message, got: %s", result.Message)
	}
}

func TestRepairWorkflows_PersonalAccount_SkipsOrgOnlyWorkflow(t *testing.T) {
	root := t.TempDir()

	// Base has both a regular and an org-only workflow.
	baseWfDir := filepath.Join(root, "base", ".github", "workflows")
	if err := os.MkdirAll(baseWfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(baseWfDir, "agentic-pipeline.yml"), []byte("pipeline-v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(baseWfDir, "sync-status-to-label.yml"), []byte("org-only"), 0o644); err != nil {
		t.Fatal(err)
	}

	wfDir := filepath.Join(root, ".github", "workflows")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, "agentic-pipeline.yml"), []byte("pipeline-v1"), 0o644); err != nil {
		t.Fatal(err)
	}

	fakeRun := func(name string, args ...string) (string, error) {
		return "", nil
	}

	result := RepairWorkflows(root, bootstrap.OwnerTypeUser, fakeRun, nil)
	if result.Status != Pass {
		t.Errorf("expected Pass for personal account repair, got %v: %s", result.Status, result.Message)
	}

	// org-only file must NOT have been copied into .github/workflows/.
	if _, err := os.Stat(filepath.Join(wfDir, "sync-status-to-label.yml")); err == nil {
		t.Error("org-only workflow should not be copied for personal account")
	}

	// Regular file should have been updated.
	data, err := os.ReadFile(filepath.Join(wfDir, "agentic-pipeline.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "pipeline-v2" {
		t.Errorf("expected updated pipeline content, got %q", data)
	}
}

func TestRepairWorkflows_Fallback_PersonalAccount_SkipsOrgOnly(t *testing.T) {
	root := t.TempDir()
	writeTemplateConfig(t, root, "owner/template", "v1.0.0")

	// No base/.github/workflows/ — fallback to tarball.
	// Include both regular and org-only workflows in tarball.
	workflowFiles := map[string]string{
		".github/workflows/agentic-pipeline.yml":     "on: push",
		".github/workflows/sync-status-to-label.yml": "org-only",
	}
	fetch := buildTestFetchFunc(t, workflowFiles)

	fakeRun := func(name string, args ...string) (string, error) {
		return "", nil
	}

	result := RepairWorkflows(root, bootstrap.OwnerTypeUser, fakeRun, fetch)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}

	// org-only file must NOT have been written.
	wfDir := filepath.Join(root, ".github", "workflows")
	if _, err := os.Stat(filepath.Join(wfDir, "sync-status-to-label.yml")); err == nil {
		t.Error("org-only workflow should not be extracted for personal account")
	}

	// Regular file should be present.
	if _, err := os.Stat(filepath.Join(wfDir, "agentic-pipeline.yml")); err != nil {
		t.Errorf("expected regular workflow file: %v", err)
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

	result := RepairProjectStatus("owner", "my-repo", root, fakeRun)
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

	result := RepairProjectStatus("owner", "my-repo", root, fakeRun)
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

	result := RepairProjectStatus("owner", "my-repo", root, fakeRun)
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

	result := RepairProjectStatus("owner", "my-repo", root, fakeRun)
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

	result := RepairProjectCollaborator("owner", "my-repo", "goose-agent", fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
}

func TestRepairProjectCollaborator_EmptyAgentUser_ReturnsPass(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		t.Fatal("run should not be called when agent user is empty")
		return "", nil
	}

	result := RepairProjectCollaborator("owner", "my-repo", "", fakeRun)
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

	result := RepairProjectCollaborator("owner", "my-repo", "goose-agent", fakeRun)
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

	result := RepairProjectCollaborator("owner", "my-repo", "goose-agent", fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v: %s", result.Status, result.Message)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// resyncProjectItemStatuses tests
// ──────────────────────────────────────────────────────────────────────────────

// statusOptionsResponse is the mock response for fetchStatusOptionMap.
const statusOptionsResponse = "OPT_1|Backlog\nOPT_2|Scoping\nOPT_3|Scheduled\nOPT_4|In Design\nOPT_5|In Development\nOPT_6|In Review\nOPT_7|Done"

// makeItemsJSON builds a mock GraphQL response for fetchAllProjectItems.
func makeItemsJSON(items []testItem, hasNextPage bool, endCursor string) string {
	var nodes []string
	for _, item := range items {
		var labelNodes []string
		for _, l := range item.labels {
			labelNodes = append(labelNodes, fmt.Sprintf(`{"name":"%s"}`, l))
		}
		var fvNodes []string
		if item.currentStatus != "" {
			fvNodes = append(fvNodes, fmt.Sprintf(`{"field":{"id":"%s"},"name":"%s"}`, item.fieldID, item.currentStatus))
		}
		nodes = append(nodes, fmt.Sprintf(`{"id":"%s","content":{"state":"%s","labels":{"nodes":[%s]}},"fieldValues":{"nodes":[%s]}}`,
			item.id, item.state, strings.Join(labelNodes, ","), strings.Join(fvNodes, ",")))
	}
	return fmt.Sprintf(`{"data":{"node":{"items":{"pageInfo":{"hasNextPage":%t,"endCursor":"%s"},"nodes":[%s]}}}}`,
		hasNextPage, endCursor, strings.Join(nodes, ","))
}

type testItem struct {
	id            string
	state         string
	labels        []string
	currentStatus string
	fieldID       string
}

func TestResyncProjectItemStatuses_Success(t *testing.T) {
	root := t.TempDir()
	writeTestProjectTemplate(t, root)
	tmpl, _ := loadTestTemplate(t, root)

	items := []testItem{
		{id: "ITEM_1", state: "OPEN", labels: []string{"in-development"}, currentStatus: "Backlog", fieldID: "FIELD_1"},
		{id: "ITEM_2", state: "CLOSED", labels: []string{"done"}, currentStatus: "Done", fieldID: "FIELD_1"},
	}

	callCount := 0
	var mutationItemIDs []string
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		switch callCount {
		case 1:
			// fetchStatusOptionMap
			return statusOptionsResponse, nil
		case 2:
			// fetchAllProjectItems
			return makeItemsJSON(items, false, ""), nil
		default:
			// Update mutations — record which item was updated.
			for _, a := range args {
				if strings.Contains(a, "updateProjectV2ItemFieldValue") {
					for _, item := range items {
						if strings.Contains(a, item.id) {
							mutationItemIDs = append(mutationItemIDs, item.id)
						}
					}
				}
			}
			return `{"data":{}}`, nil
		}
	}

	updated, correct, err := resyncProjectItemStatuses("owner", "PVT_123", "FIELD_1", tmpl, fakeRun)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated != 1 {
		t.Errorf("expected 1 updated, got %d", updated)
	}
	if correct != 1 {
		t.Errorf("expected 1 correct, got %d", correct)
	}
	// ITEM_1 should have been updated (in-development → In Development, but current was Backlog).
	if len(mutationItemIDs) != 1 || mutationItemIDs[0] != "ITEM_1" {
		t.Errorf("expected mutation for ITEM_1, got %v", mutationItemIDs)
	}
}

func TestResyncProjectItemStatuses_Pagination(t *testing.T) {
	root := t.TempDir()
	writeTestProjectTemplate(t, root)
	tmpl, _ := loadTestTemplate(t, root)

	page1Items := []testItem{
		{id: "ITEM_1", state: "OPEN", labels: []string{"backlog"}, currentStatus: "Backlog", fieldID: "FIELD_1"},
	}
	page2Items := []testItem{
		{id: "ITEM_2", state: "OPEN", labels: []string{"scoping"}, currentStatus: "Backlog", fieldID: "FIELD_1"},
	}

	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		switch callCount {
		case 1:
			return statusOptionsResponse, nil
		case 2:
			// First page — hasNextPage=true.
			return makeItemsJSON(page1Items, true, "CURSOR_1"), nil
		case 3:
			// Second page — hasNextPage=false.
			return makeItemsJSON(page2Items, false, ""), nil
		default:
			// Update mutation for ITEM_2.
			return `{"data":{}}`, nil
		}
	}

	updated, correct, err := resyncProjectItemStatuses("owner", "PVT_123", "FIELD_1", tmpl, fakeRun)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// ITEM_1 is already correct (Backlog), ITEM_2 needs update (scoping → Scoping).
	if updated != 1 {
		t.Errorf("expected 1 updated, got %d", updated)
	}
	if correct != 1 {
		t.Errorf("expected 1 correct, got %d", correct)
	}
	// Verify second page was fetched (callCount should be at least 4: options, page1, page2, mutation).
	if callCount < 4 {
		t.Errorf("expected at least 4 calls (pagination), got %d", callCount)
	}
}

func TestResyncProjectItemStatuses_ClosedBeforeBacklog(t *testing.T) {
	root := t.TempDir()
	writeTestProjectTemplate(t, root)
	tmpl, _ := loadTestTemplate(t, root)

	// Critical edge case: item is CLOSED with backlog label but NO pipeline label.
	// Rule 7 (CLOSED → Done) must take priority over rule 8 (backlog → Backlog).
	items := []testItem{
		{id: "ITEM_1", state: "CLOSED", labels: []string{"backlog"}, currentStatus: "Backlog", fieldID: "FIELD_1"},
	}

	callCount := 0
	var updatedWithOptionID string
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		switch callCount {
		case 1:
			return statusOptionsResponse, nil
		case 2:
			return makeItemsJSON(items, false, ""), nil
		default:
			// Capture the option ID used in the mutation.
			for _, a := range args {
				if strings.Contains(a, "updateProjectV2ItemFieldValue") && strings.Contains(a, "ITEM_1") {
					// Extract option ID from the mutation.
					if strings.Contains(a, "OPT_7") {
						updatedWithOptionID = "OPT_7" // Done
					} else if strings.Contains(a, "OPT_1") {
						updatedWithOptionID = "OPT_1" // Backlog
					}
				}
			}
			return `{"data":{}}`, nil
		}
	}

	updated, _, err := resyncProjectItemStatuses("owner", "PVT_123", "FIELD_1", tmpl, fakeRun)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated != 1 {
		t.Errorf("expected 1 updated, got %d", updated)
	}
	if updatedWithOptionID != "OPT_7" {
		t.Errorf("expected Done option (OPT_7), got %s — CLOSED-before-backlog priority violated", updatedWithOptionID)
	}
}

func TestResyncProjectItemStatuses_AlreadyCorrect(t *testing.T) {
	root := t.TempDir()
	writeTestProjectTemplate(t, root)
	tmpl, _ := loadTestTemplate(t, root)

	items := []testItem{
		{id: "ITEM_1", state: "OPEN", labels: []string{"in-development"}, currentStatus: "In Development", fieldID: "FIELD_1"},
		{id: "ITEM_2", state: "OPEN", labels: []string{"backlog"}, currentStatus: "Backlog", fieldID: "FIELD_1"},
		{id: "ITEM_3", state: "CLOSED", labels: []string{"done"}, currentStatus: "Done", fieldID: "FIELD_1"},
	}

	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		switch callCount {
		case 1:
			return statusOptionsResponse, nil
		case 2:
			return makeItemsJSON(items, false, ""), nil
		default:
			t.Error("no update mutation should be called when all items are correct")
			return `{"data":{}}`, nil
		}
	}

	updated, correct, err := resyncProjectItemStatuses("owner", "PVT_123", "FIELD_1", tmpl, fakeRun)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated != 0 {
		t.Errorf("expected 0 updated, got %d", updated)
	}
	if correct != 3 {
		t.Errorf("expected 3 correct, got %d", correct)
	}
}

func TestResyncProjectItemStatuses_NoItems(t *testing.T) {
	root := t.TempDir()
	writeTestProjectTemplate(t, root)
	tmpl, _ := loadTestTemplate(t, root)

	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		switch callCount {
		case 1:
			return statusOptionsResponse, nil
		case 2:
			return makeItemsJSON(nil, false, ""), nil
		}
		return "", nil
	}

	updated, correct, err := resyncProjectItemStatuses("owner", "PVT_123", "FIELD_1", tmpl, fakeRun)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated != 0 {
		t.Errorf("expected 0 updated, got %d", updated)
	}
	if correct != 0 {
		t.Errorf("expected 0 correct, got %d", correct)
	}
}

// loadTestTemplate is a test helper that loads the project template from root.
func loadTestTemplate(t *testing.T, root string) (*bootstrap.ProjectTemplate, error) {
	t.Helper()
	return bootstrap.LoadProjectTemplate(root)
}

// ──────────────────────────────────────────────────────────────────────────────
// RepairAgentUserVar tests
// ──────────────────────────────────────────────────────────────────────────────

func TestRepairAgentUserVar_RepoScope_SetsVariable(t *testing.T) {
	var setCalled bool
	fakeRun := func(name string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "variable set AGENT_USER") && strings.Contains(joined, "--repo") {
			setCalled = true
			return "", nil
		}
		return "", nil
	}

	result := RepairAgentUserVar("alice", "my-repo", "goose-agent", "repo", fakeRun, nil)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
	if !setCalled {
		t.Error("expected gh variable set to be called with --repo")
	}
}

func TestRepairAgentUserVar_InvalidScope_ReturnsFail(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return "", nil
	}

	result := RepairAgentUserVar("alice", "my-repo", "goose-agent", "invalid", fakeRun, nil)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "invalid scope") {
		t.Errorf("expected message about invalid scope, got %q", result.Message)
	}
}

func TestRepairAgentUserVar_EmptyUser_PromptsAndSets(t *testing.T) {
	var setCalled bool
	fakeRun := func(name string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "variable set AGENT_USER") {
			setCalled = true
		}
		return "", nil
	}
	fakeConfirm := func(prompt string) (string, error) {
		if strings.Contains(prompt, "username") {
			return "my-agent", nil
		}
		return "", nil
	}

	result := RepairAgentUserVar("alice", "my-repo", "", "repo", fakeRun, fakeConfirm)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
	if !setCalled {
		t.Error("expected gh variable set to be called")
	}
}

func TestRepairAgentUserVar_EmptyScope_PromptsAndSets(t *testing.T) {
	var setCalled bool
	fakeRun := func(name string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "variable set AGENT_USER") {
			setCalled = true
		}
		return "", nil
	}
	fakeConfirm := func(prompt string) (string, error) {
		if strings.Contains(prompt, "scope") {
			return "repo", nil
		}
		return "", nil
	}

	result := RepairAgentUserVar("alice", "my-repo", "goose-agent", "", fakeRun, fakeConfirm)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
	if !setCalled {
		t.Error("expected gh variable set to be called")
	}
}

func TestRepairAgentUserVar_EmptyUserAfterPrompt_ReturnsFail(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return "", nil
	}
	fakeConfirm := func(prompt string) (string, error) {
		return "", nil // empty response
	}

	result := RepairAgentUserVar("alice", "my-repo", "", "repo", fakeRun, fakeConfirm)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "required") {
		t.Errorf("expected message about required, got %q", result.Message)
	}
}

func TestRepairAgentUserVar_SetFails_ReturnsFail(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "variable set") {
			return "", fmt.Errorf("permission denied")
		}
		return "", nil
	}

	result := RepairAgentUserVar("alice", "my-repo", "goose-agent", "repo", fakeRun, nil)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v: %s", result.Status, result.Message)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// RepairAgenticProjectID tests
// ──────────────────────────────────────────────────────────────────────────────

func TestRepairAgenticProjectID(t *testing.T) {
	projectListJSON := `{"projects":[{"id":"PVT_kwDOBtest","title":"repo","number":1,"url":"https://github.com/users/owner/projects/1","owner":{"login":"owner","type":"User"}}]}`

	tests := []struct {
		name       string
		fakeRun    func(name string, args ...string) (string, error)
		wantStatus CheckStatus
		wantMsg    string
	}{
		{
			name: "project found and variable set succeeds returns Pass",
			fakeRun: func(name string, args ...string) (string, error) {
				cmd := strings.Join(append([]string{name}, args...), " ")
				if strings.Contains(cmd, "project list") {
					return projectListJSON, nil
				}
				if strings.Contains(cmd, "variable set") {
					return "", nil
				}
				return "", nil
			},
			wantStatus: Pass,
		},
		{
			name: "project not found returns Fail",
			fakeRun: func(name string, args ...string) (string, error) {
				return `{"projects":[]}`, nil
			},
			wantStatus: Fail,
			wantMsg:    "cannot resolve project",
		},
		{
			name: "project found but variable set fails returns Fail",
			fakeRun: func(name string, args ...string) (string, error) {
				cmd := strings.Join(append([]string{name}, args...), " ")
				if strings.Contains(cmd, "project list") {
					return projectListJSON, nil
				}
				if strings.Contains(cmd, "variable set") {
					return "", fmt.Errorf("permission denied")
				}
				return "", nil
			},
			wantStatus: Fail,
			wantMsg:    "failed to set variable",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := RepairAgenticProjectID("owner/repo", "owner", "repo", bootstrap.OwnerTypeUser, tc.fakeRun)
			if result.Status != tc.wantStatus {
				t.Errorf("expected %v, got %v: %s", tc.wantStatus, result.Status, result.Message)
			}
			if tc.wantMsg != "" && !strings.Contains(result.Message, tc.wantMsg) {
				t.Errorf("expected message containing %q, got %q", tc.wantMsg, result.Message)
			}
			if result.Name != "AGENTIC_PROJECT_ID is configured" {
				t.Errorf("unexpected check name: %s", result.Name)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Pipeline variable repair tests
// ──────────────────────────────────────────────────────────────────────────────

func TestRepairRunnerLabelVar_Success(t *testing.T) {
	var setCalled bool
	fakeRun := func(name string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "variable set RUNNER_LABEL") && strings.Contains(joined, "ubuntu-latest") {
			setCalled = true
			return "", nil
		}
		return "", nil
	}

	result := RepairRunnerLabelVar("owner", "repo", bootstrap.OwnerTypeUser, fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
	if !setCalled {
		t.Error("expected gh variable set to be called")
	}
}

func TestRepairRunnerLabelVar_Failure(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return "", fmt.Errorf("permission denied")
	}
	result := RepairRunnerLabelVar("owner", "repo", bootstrap.OwnerTypeUser, fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v: %s", result.Status, result.Message)
	}
}

func TestRepairGooseProviderVar_Success(t *testing.T) {
	var setCalled bool
	fakeRun := func(name string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "variable set GOOSE_PROVIDER") && strings.Contains(joined, "claude-code") {
			setCalled = true
			return "", nil
		}
		return "", nil
	}

	result := RepairGooseProviderVar("owner", "repo", bootstrap.OwnerTypeUser, fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
	if !setCalled {
		t.Error("expected gh variable set to be called")
	}
}

func TestRepairGooseProviderVar_Failure(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return "", fmt.Errorf("permission denied")
	}
	result := RepairGooseProviderVar("owner", "repo", bootstrap.OwnerTypeUser, fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v: %s", result.Status, result.Message)
	}
}

func TestRepairGooseModelVar_Success(t *testing.T) {
	var setCalled bool
	fakeRun := func(name string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "variable set GOOSE_MODEL") && strings.Contains(joined, "default") {
			setCalled = true
			return "", nil
		}
		return "", nil
	}

	result := RepairGooseModelVar("owner", "repo", bootstrap.OwnerTypeUser, fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
	if !setCalled {
		t.Error("expected gh variable set to be called")
	}
}

func TestRepairGooseModelVar_Failure(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return "", fmt.Errorf("permission denied")
	}
	result := RepairGooseModelVar("owner", "repo", bootstrap.OwnerTypeUser, fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v: %s", result.Status, result.Message)
	}
}

func TestRepairAgenticProjectID_OrgScope_UsesOrgFlag(t *testing.T) {
	projectListJSON := `{"projects":[{"id":"PVT_kwDOBtest","title":"repo","number":1,"url":"https://github.com/orgs/acme-org/projects/1","owner":{"login":"acme-org","type":"Organization"}}]}`

	var capturedArgs []string
	fakeRun := func(name string, args ...string) (string, error) {
		cmd := strings.Join(append([]string{name}, args...), " ")
		if strings.Contains(cmd, "project list") {
			return projectListJSON, nil
		}
		if strings.Contains(cmd, "variable set") {
			capturedArgs = append([]string{name}, args...)
			return "", nil
		}
		return "", nil
	}

	result := RepairAgenticProjectID("acme-org/repo", "acme-org", "repo", bootstrap.OwnerTypeOrg, fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}

	joined := strings.Join(capturedArgs, " ")
	if !strings.Contains(joined, "--org acme-org") {
		t.Errorf("expected --org acme-org in args, got: %v", capturedArgs)
	}
	if strings.Contains(joined, "--repo") {
		t.Errorf("expected no --repo flag for org scope, got: %v", capturedArgs)
	}
}

func TestRepairAgenticProjectID_Org_DeletesMisplacedRepoVar(t *testing.T) {
	projectListJSON := `{"projects":[{"id":"PVT_kwDOBtest","title":"repo","number":1,"url":"https://github.com/orgs/acme-org/projects/1","owner":{"login":"acme-org","type":"Organization"}}]}`
	var deleteCalled bool
	var deleteArgs string
	fakeRun := func(name string, args ...string) (string, error) {
		cmd := strings.Join(append([]string{name}, args...), " ")
		if strings.Contains(cmd, "project list") {
			return projectListJSON, nil
		}
		if strings.Contains(cmd, "variable set") {
			return "", nil
		}
		if strings.Contains(cmd, "variable list") && strings.Contains(cmd, "--repo") {
			return `[{"name":"AGENTIC_PROJECT_ID"}]`, nil
		}
		if strings.Contains(cmd, "variable list") && strings.Contains(cmd, "--org") {
			return `[]`, nil
		}
		if strings.Contains(cmd, "variable delete") {
			deleteCalled = true
			deleteArgs = cmd
			return "", nil
		}
		return "", nil
	}

	result := RepairAgenticProjectID("acme-org/repo", "acme-org", "repo", bootstrap.OwnerTypeOrg, fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
	if !deleteCalled {
		t.Error("expected repo-level AGENTIC_PROJECT_ID to be deleted")
	}
	if !strings.Contains(deleteArgs, "--repo acme-org/repo") {
		t.Errorf("expected delete to target repo, got %q", deleteArgs)
	}
}

func TestRepairAgenticProjectID_User_NoDeleteAttempted(t *testing.T) {
	projectListJSON := `{"projects":[{"id":"PVT_kwDOBtest","title":"repo","number":1,"url":"https://github.com/users/alice/projects/1","owner":{"login":"alice","type":"User"}}]}`
	var deleteCalled bool
	fakeRun := func(name string, args ...string) (string, error) {
		cmd := strings.Join(append([]string{name}, args...), " ")
		if strings.Contains(cmd, "project list") {
			return projectListJSON, nil
		}
		if strings.Contains(cmd, "variable set") {
			return "", nil
		}
		if strings.Contains(cmd, "variable delete") {
			deleteCalled = true
			return "", nil
		}
		return "", nil
	}

	result := RepairAgenticProjectID("alice/repo", "alice", "repo", bootstrap.OwnerTypeUser, fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
	if deleteCalled {
		t.Error("expected no delete for user topology")
	}
}

func TestRepairRunnerLabelVar_OrgScope_UsesOrgFlag(t *testing.T) {
	var setArgs []string
	fakeRun := func(name string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "variable set") {
			setArgs = append([]string{name}, args...)
		}
		return `[]`, nil
	}

	result := RepairRunnerLabelVar("acme-org", "repo", bootstrap.OwnerTypeOrg, fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}

	joined := strings.Join(setArgs, " ")
	if !strings.Contains(joined, "--org acme-org") {
		t.Errorf("expected --org acme-org in set args, got: %v", setArgs)
	}
	if strings.Contains(joined, "--repo") {
		t.Errorf("expected no --repo flag in set command for org scope, got: %v", setArgs)
	}
}

func TestRepairGooseProviderVar_OrgScope_UsesOrgFlag(t *testing.T) {
	var setArgs []string
	fakeRun := func(name string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "variable set") {
			setArgs = append([]string{name}, args...)
		}
		return `[]`, nil
	}

	result := RepairGooseProviderVar("acme-org", "repo", bootstrap.OwnerTypeOrg, fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}

	joined := strings.Join(setArgs, " ")
	if !strings.Contains(joined, "--org acme-org") {
		t.Errorf("expected --org acme-org in set args, got: %v", setArgs)
	}
}

func TestRepairGooseModelVar_OrgScope_UsesOrgFlag(t *testing.T) {
	var setArgs []string
	fakeRun := func(name string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "variable set") {
			setArgs = append([]string{name}, args...)
		}
		return `[]`, nil
	}

	result := RepairGooseModelVar("acme-org", "repo", bootstrap.OwnerTypeOrg, fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}

	joined := strings.Join(setArgs, " ")
	if !strings.Contains(joined, "--org acme-org") {
		t.Errorf("expected --org acme-org in set args, got: %v", setArgs)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Pipeline secret repair tests
// ──────────────────────────────────────────────────────────────────────────────

func TestRepairGooseAgentPATSecret_ReturnsManualAction(t *testing.T) {
	result := RepairGooseAgentPATSecret("owner", "repo", bootstrap.OwnerTypeUser)
	if result.Status != ManualAction {
		t.Errorf("expected ManualAction, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "https://github.com/owner/repo/settings/secrets/actions") {
		t.Errorf("expected GitHub secrets URL in message, got %q", result.Message)
	}
	if !strings.Contains(result.Message, "repo and project scopes") {
		t.Errorf("expected scope instructions in message, got %q", result.Message)
	}
}

func TestRepairClaudeCredentialsSecret_FileExists_SetsSecret(t *testing.T) {
	var setCalled bool
	fakeRun := func(name string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "secret set CLAUDE_CREDENTIALS_JSON") {
			setCalled = true
			return "", nil
		}
		return "", nil
	}
	fakeReadFile := func(path string) ([]byte, error) {
		return []byte(`{"token":"abc123"}`), nil
	}
	fakeHomeDir := func() (string, error) {
		return "/home/testuser", nil
	}

	result := RepairClaudeCredentialsSecretWithReadFile("owner", "repo", "User", fakeRun, fakeReadFile, fakeHomeDir)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
	if !setCalled {
		t.Error("expected gh secret set to be called")
	}
}

func TestRepairClaudeCredentialsSecret_FileMissing_ReturnsManualAction(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		// Keychain lookup returns empty — triggers ManualAction.
		return "", nil
	}
	fakeReadFile := func(path string) ([]byte, error) {
		return nil, fmt.Errorf("file not found")
	}
	fakeHomeDir := func() (string, error) {
		return "/home/testuser", nil
	}

	result := RepairClaudeCredentialsSecretWithReadFile("owner", "repo", "User", fakeRun, fakeReadFile, fakeHomeDir)
	if result.Status != ManualAction {
		t.Errorf("expected ManualAction, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "gh secret set") {
		t.Errorf("expected manual instructions in message, got %q", result.Message)
	}
}

func TestRepairClaudeCredentialsSecret_FileMissing_KeychainPresent_SetsSecret(t *testing.T) {
	var setCalled bool
	fakeRun := func(name string, args ...string) (string, error) {
		// Simulate macOS Keychain returning credentials.
		if name == "security" {
			return `{"token":"keychain-value"}`, nil
		}
		if strings.Contains(strings.Join(args, " "), "secret set CLAUDE_CREDENTIALS_JSON") {
			setCalled = true
		}
		return "", nil
	}
	fakeReadFile := func(path string) ([]byte, error) {
		return nil, fmt.Errorf("file not found")
	}
	fakeHomeDir := func() (string, error) {
		return "/home/testuser", nil
	}

	result := RepairClaudeCredentialsSecretWithReadFile("owner", "repo", "User", fakeRun, fakeReadFile, fakeHomeDir)
	if result.Status != Pass {
		t.Errorf("expected Pass when keychain has credentials, got %v: %s", result.Status, result.Message)
	}
	if !setCalled {
		t.Error("expected gh secret set to be called with keychain credentials")
	}
}

func TestRepairClaudeCredentialsSecret_HomeDirError_ReturnsManualAction(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return "", nil
	}
	fakeReadFile := func(path string) ([]byte, error) {
		return []byte("data"), nil
	}
	fakeHomeDir := func() (string, error) {
		return "", fmt.Errorf("no home")
	}

	result := RepairClaudeCredentialsSecretWithReadFile("owner", "repo", "User", fakeRun, fakeReadFile, fakeHomeDir)
	if result.Status != ManualAction {
		t.Errorf("expected ManualAction, got %v: %s", result.Status, result.Message)
	}
}

func TestRepairClaudeCredentialsSecret_SetFails_ReturnsFail(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		// Auth succeeds but gh secret set fails.
		if name == "claude" {
			return "Hello!", nil
		}
		return "", fmt.Errorf("permission denied")
	}
	fakeReadFile := func(path string) ([]byte, error) {
		return []byte(`{"token":"abc123"}`), nil
	}
	fakeHomeDir := func() (string, error) {
		return "/home/testuser", nil
	}

	result := RepairClaudeCredentialsSecretWithReadFile("owner", "repo", "User", fakeRun, fakeReadFile, fakeHomeDir)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v: %s", result.Status, result.Message)
	}
}

func TestRepairClaudeCredentialsSecret_AuthFails_ReturnsManualAction(t *testing.T) {
	var secretSetCalled bool
	fakeRun := func(name string, args ...string) (string, error) {
		if name == "claude" {
			return "", fmt.Errorf("auth required")
		}
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "secret set CLAUDE_CREDENTIALS_JSON") {
			secretSetCalled = true
		}
		return "", nil
	}
	fakeReadFile := func(path string) ([]byte, error) {
		return []byte(`{"token":"abc123"}`), nil
	}
	fakeHomeDir := func() (string, error) {
		return "/home/testuser", nil
	}

	result := RepairClaudeCredentialsSecretWithReadFile("owner", "repo", "User", fakeRun, fakeReadFile, fakeHomeDir)
	if result.Status != ManualAction {
		t.Errorf("expected ManualAction, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "claude auth login") {
		t.Errorf("expected 'claude auth login' in message, got %q", result.Message)
	}
	if secretSetCalled {
		t.Error("expected gh secret set NOT to be called when auth fails")
	}
}

func TestRepairClaudeCredentialsSecret_AuthSucceeds_SetsSecret(t *testing.T) {
	var setCalled bool
	fakeRun := func(name string, args ...string) (string, error) {
		if name == "claude" {
			return "Hello!", nil
		}
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "secret set CLAUDE_CREDENTIALS_JSON") {
			setCalled = true
		}
		return "", nil
	}
	fakeReadFile := func(path string) ([]byte, error) {
		return []byte(`{"token":"abc123"}`), nil
	}
	fakeHomeDir := func() (string, error) {
		return "/home/testuser", nil
	}

	result := RepairClaudeCredentialsSecretWithReadFile("owner", "repo", "User", fakeRun, fakeReadFile, fakeHomeDir)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
	if !setCalled {
		t.Error("expected gh secret set to be called when auth succeeds")
	}
}

func TestRepairClaudeCredentialsSecret_DelegatesToWithReadFile(t *testing.T) {
	// RepairClaudeCredentialsSecret delegates to RepairClaudeCredentialsSecretWithReadFile.
	// Since both os.ReadFile and os.UserHomeDir are injected, calling the wrapper
	// with a non-existent home/credentials path should produce the same ManualAction
	// result as calling WithReadFile directly with failing injections.
	// We can't inject fakes into the wrapper, but we can verify the function
	// returns a valid CheckResult (not a panic or compile error).
	result := RepairClaudeCredentialsSecret("owner", "repo", "User", func(name string, args ...string) (string, error) {
		// Keychain lookup fails — triggers ManualAction.
		return "", fmt.Errorf("not available")
	})
	// The wrapper calls os.UserHomeDir and os.ReadFile which may or may not succeed
	// in CI, but it should never panic and should return a valid result.
	if result.Name != checkClaudeCredentialsSecretName {
		t.Errorf("expected check name %q, got %q", checkClaudeCredentialsSecretName, result.Name)
	}
}

func TestRepairClaudeCredentialsSecret_OrgScope_UsesOrgFlag(t *testing.T) {
	var setArgs []string
	fakeRun := func(name string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if name == "gh" && strings.Contains(joined, "secret set") {
			setArgs = args
		}
		return `[]`, nil
	}
	fakeReadFile := func(path string) ([]byte, error) {
		return []byte(`{"token":"abc123"}`), nil
	}
	fakeHomeDir := func() (string, error) {
		return "/home/testuser", nil
	}

	result := RepairClaudeCredentialsSecretWithReadFile("acme-org", "repo", "Organization", fakeRun, fakeReadFile, fakeHomeDir)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}

	// Verify --org flag is used in the set command.
	joined := strings.Join(setArgs, " ")
	if !strings.Contains(joined, "--org acme-org") {
		t.Errorf("expected --org acme-org in set args, got: %v", setArgs)
	}
	if strings.Contains(joined, "--repo") {
		t.Errorf("expected no --repo flag in set command for org scope, got: %v", setArgs)
	}
}

func TestRepairClaudeCredentialsSecret_UserScope_UsesRepoFlag(t *testing.T) {
	var ghArgs []string
	fakeRun := func(name string, args ...string) (string, error) {
		if name == "gh" {
			ghArgs = args
		}
		return "", nil
	}
	fakeReadFile := func(path string) ([]byte, error) {
		return []byte(`{"token":"abc123"}`), nil
	}
	fakeHomeDir := func() (string, error) {
		return "/home/testuser", nil
	}

	result := RepairClaudeCredentialsSecretWithReadFile("alice", "repo", "User", fakeRun, fakeReadFile, fakeHomeDir)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}

	// Verify --repo flag is used with owner/repo.
	joined := strings.Join(ghArgs, " ")
	if !strings.Contains(joined, "--repo alice/repo") {
		t.Errorf("expected --repo alice/repo in gh args, got: %v", ghArgs)
	}
	if strings.Contains(joined, "--org") {
		t.Errorf("expected no --org flag for user scope, got: %v", ghArgs)
	}
}

func TestRepairClaudeCredentialsManualAction_OrgScope_ShowsOrgInstructions(t *testing.T) {
	result := RepairClaudeCredentialsSecretWithReadFile("acme-org", "repo", "Organization",
		func(name string, args ...string) (string, error) { return "", nil },
		func(path string) ([]byte, error) { return nil, fmt.Errorf("not found") },
		func() (string, error) { return "/home/testuser", nil },
	)
	if result.Status != ManualAction {
		t.Errorf("expected ManualAction, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "--org acme-org") {
		t.Errorf("expected --org acme-org in manual instructions, got %q", result.Message)
	}
	if strings.Contains(result.Message, "--repo") {
		t.Errorf("expected no --repo in manual instructions for org, got %q", result.Message)
	}
}

func TestRepairClaudeCredentialsManualAction_UserScope_ShowsRepoInstructions(t *testing.T) {
	result := RepairClaudeCredentialsSecretWithReadFile("alice", "repo", "User",
		func(name string, args ...string) (string, error) { return "", nil },
		func(path string) ([]byte, error) { return nil, fmt.Errorf("not found") },
		func() (string, error) { return "/home/testuser", nil },
	)
	if result.Status != ManualAction {
		t.Errorf("expected ManualAction, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "--repo alice/repo") {
		t.Errorf("expected --repo alice/repo in manual instructions, got %q", result.Message)
	}
	if strings.Contains(result.Message, "--org") {
		t.Errorf("expected no --org in manual instructions for user, got %q", result.Message)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test helpers for tarball-based repairs
// ──────────────────────────────────────────────────────────────────────────────

// writeTemplateConfig writes TEMPLATE_SOURCE and TEMPLATE_VERSION files.
func writeTemplateConfig(t *testing.T, root, source, version string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "TEMPLATE_SOURCE"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "TEMPLATE_VERSION"), []byte(version), 0o644); err != nil {
		t.Fatal(err)
	}
}

// buildTestTarGz creates a gzipped tarball with the given files.
// Keys are relative paths (without top-level prefix); values are content.
// A "repo-v1.0.0/" prefix is added automatically.
func buildTestTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	prefix := "repo-v1.0.0/"

	for name, content := range files {
		fullPath := prefix + name
		// Create parent directories.
		parts := strings.Split(fullPath, "/")
		for i := 1; i < len(parts); i++ {
			dirPath := strings.Join(parts[:i], "/") + "/"
			_ = tw.WriteHeader(&tar.Header{
				Name:     dirPath,
				Typeflag: tar.TypeDir,
				Mode:     0o755,
			})
		}
		if err := tw.WriteHeader(&tar.Header{
			Name:     fullPath,
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Size:     int64(len(content)),
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// buildTestFetchFunc returns a tarball.FetchFunc that serves a tarball
// containing the given files.
func buildTestFetchFunc(t *testing.T, files map[string]string) tarball.FetchFunc {
	t.Helper()
	data := buildTestTarGz(t, files)
	return func(repo, version string) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(data)), nil
	}
}

// failingFetchFunc returns a tarball.FetchFunc that always errors.
func failingFetchFunc(msg string) tarball.FetchFunc {
	return func(repo, version string) (io.ReadCloser, error) {
		return nil, fmt.Errorf("%s", msg)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Topology-aware repair tests — variable move capability
// ──────────────────────────────────────────────────────────────────────────────

func TestRepairRepoVariable_Org_DeletesMisplacedRepoVar(t *testing.T) {
	var deleteCalled bool
	var deleteArgs string
	fakeRun := func(name string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "variable set") {
			return "", nil
		}
		if strings.Contains(joined, "variable list") && strings.Contains(joined, "--repo") {
			return `[{"name":"RUNNER_LABEL"}]`, nil
		}
		if strings.Contains(joined, "variable list") && strings.Contains(joined, "--org") {
			return `[]`, nil
		}
		if strings.Contains(joined, "variable delete") {
			deleteCalled = true
			deleteArgs = joined
			return "", nil
		}
		return `[]`, nil
	}

	result := RepairRunnerLabelVar("acme-org", "my-repo", bootstrap.OwnerTypeOrg, fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
	if !deleteCalled {
		t.Error("expected repo-level variable to be deleted")
	}
	if !strings.Contains(deleteArgs, "--repo acme-org/my-repo") {
		t.Errorf("expected delete to target repo, got %q", deleteArgs)
	}
}

func TestRepairRepoVariable_Org_NoDeleteWhenNoRepoVar(t *testing.T) {
	var deleteCalled bool
	fakeRun := func(name string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "variable set") {
			return "", nil
		}
		if strings.Contains(joined, "variable list") {
			return `[]`, nil
		}
		if strings.Contains(joined, "variable delete") {
			deleteCalled = true
			return "", nil
		}
		return `[]`, nil
	}

	result := RepairRunnerLabelVar("acme-org", "my-repo", bootstrap.OwnerTypeOrg, fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
	if deleteCalled {
		t.Error("expected no delete when variable not at repo level")
	}
}

func TestRepairRepoVariable_User_NoDeleteAttempted(t *testing.T) {
	var deleteCalled bool
	fakeRun := func(name string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "variable set") {
			return "", nil
		}
		if strings.Contains(joined, "variable delete") {
			deleteCalled = true
			return "", nil
		}
		return `[]`, nil
	}

	result := RepairRunnerLabelVar("alice", "my-repo", bootstrap.OwnerTypeUser, fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
	if deleteCalled {
		t.Error("expected no delete for user topology")
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Topology-aware repair tests — GooseAgentPAT org/user messages
// ──────────────────────────────────────────────────────────────────────────────

func TestRepairGooseAgentPATSecret_Org_ReferencesOrgScope(t *testing.T) {
	result := RepairGooseAgentPATSecret("acme-org", "my-repo", bootstrap.OwnerTypeOrg)
	if result.Status != ManualAction {
		t.Errorf("expected ManualAction, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "org level") {
		t.Errorf("expected org-level reference in message, got %q", result.Message)
	}
	if !strings.Contains(result.Message, "https://github.com/organizations/acme-org/settings/secrets/actions") {
		t.Errorf("expected org secrets URL, got %q", result.Message)
	}
}

func TestRepairGooseAgentPATSecret_User_ReferencesRepoScope(t *testing.T) {
	result := RepairGooseAgentPATSecret("alice", "my-repo", bootstrap.OwnerTypeUser)
	if result.Status != ManualAction {
		t.Errorf("expected ManualAction, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "https://github.com/alice/my-repo/settings/secrets/actions") {
		t.Errorf("expected repo secrets URL, got %q", result.Message)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Topology-aware repair tests — ClaudeCredentials move capability
// ──────────────────────────────────────────────────────────────────────────────

func TestRepairClaudeCredentialsSecret_Org_DeletesMisplacedRepoSecret(t *testing.T) {
	var deleteCalled bool
	var deleteArgs string
	fakeRun := func(name string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "secret set") {
			return "", nil
		}
		if strings.Contains(joined, "secret list") && strings.Contains(joined, "--repo") {
			return `[{"name":"CLAUDE_CREDENTIALS_JSON"}]`, nil
		}
		if strings.Contains(joined, "secret list") && strings.Contains(joined, "--org") {
			return `[]`, nil
		}
		if strings.Contains(joined, "secret delete") {
			deleteCalled = true
			deleteArgs = joined
			return "", nil
		}
		return "", nil
	}
	fakeReadFile := func(path string) ([]byte, error) {
		return []byte(`{"token":"abc123"}`), nil
	}
	fakeHomeDir := func() (string, error) {
		return "/home/testuser", nil
	}

	result := RepairClaudeCredentialsSecretWithReadFile("acme-org", "my-repo", bootstrap.OwnerTypeOrg, fakeRun, fakeReadFile, fakeHomeDir)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
	if !deleteCalled {
		t.Error("expected repo-level secret to be deleted")
	}
	if !strings.Contains(deleteArgs, "--repo acme-org/my-repo") {
		t.Errorf("expected delete to target repo, got %q", deleteArgs)
	}
}

func TestRepairClaudeCredentialsSecret_User_NoDeleteAttempted(t *testing.T) {
	var deleteCalled bool
	fakeRun := func(name string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "secret set") {
			return "", nil
		}
		if strings.Contains(joined, "secret delete") {
			deleteCalled = true
			return "", nil
		}
		return "", nil
	}
	fakeReadFile := func(path string) ([]byte, error) {
		return []byte(`{"token":"abc123"}`), nil
	}
	fakeHomeDir := func() (string, error) {
		return "/home/testuser", nil
	}

	result := RepairClaudeCredentialsSecretWithReadFile("alice", "my-repo", bootstrap.OwnerTypeUser, fakeRun, fakeReadFile, fakeHomeDir)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
	if deleteCalled {
		t.Error("expected no delete for user topology")
	}
}
