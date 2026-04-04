package verify

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckCLAUDEMD_Present_ReturnsPass(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("# CLAUDE.md"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := CheckCLAUDEMD(root)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckCLAUDEMD_Missing_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	result := CheckCLAUDEMD(root)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v", result.Status)
	}
}

func TestCheckAGENTSLocalMD_Present_ReturnsPass(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "AGENTS.local.md"), []byte("# local"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := CheckAGENTSLocalMD(root)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckAGENTSLocalMD_Missing_ReturnsWarning(t *testing.T) {
	root := t.TempDir()
	result := CheckAGENTSLocalMD(root)
	if result.Status != Warning {
		t.Errorf("expected Warning, got %v", result.Status)
	}
}

func TestCheckTEMPLATESOURCE_Present_ReturnsPass(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "TEMPLATE_SOURCE"), []byte("eddiecarpenter/agentic-development\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := CheckTEMPLATESOURCE(root)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckTEMPLATESOURCE_Missing_ReturnsWarning(t *testing.T) {
	root := t.TempDir()
	result := CheckTEMPLATESOURCE(root)
	if result.Status != Warning {
		t.Errorf("expected Warning, got %v", result.Status)
	}
}

func TestCheckTEMPLATEVERSION_Present_ReturnsPass(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "TEMPLATE_VERSION"), []byte("v1.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := CheckTEMPLATEVERSION(root)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckTEMPLATEVERSION_Missing_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	result := CheckTEMPLATEVERSION(root)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v", result.Status)
	}
}

func TestCheckREPOSMD_Present_ReturnsPass(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "REPOS.md"), []byte("# REPOS"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := CheckREPOSMD(root)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckREPOSMD_Missing_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	result := CheckREPOSMD(root)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v", result.Status)
	}
}

func TestCheckREADMEMD_Present_ReturnsPass(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# README"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := CheckREADMEMD(root)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckREADMEMD_Missing_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	result := CheckREADMEMD(root)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v", result.Status)
	}
}

// Table-driven test covering all file checks.
func TestFileChecks_TableDriven(t *testing.T) {
	tests := []struct {
		name         string
		filename     string
		checkFn      func(string) CheckResult
		missingState CheckStatus
	}{
		{"CLAUDE.md", "CLAUDE.md", CheckCLAUDEMD, Fail},
		{"AGENTS.local.md", "AGENTS.local.md", CheckAGENTSLocalMD, Warning},
		{"TEMPLATE_SOURCE", "TEMPLATE_SOURCE", CheckTEMPLATESOURCE, Warning},
		{"TEMPLATE_VERSION", "TEMPLATE_VERSION", CheckTEMPLATEVERSION, Fail},
		{"REPOS.md", "REPOS.md", CheckREPOSMD, Fail},
		{"README.md", "README.md", CheckREADMEMD, Fail},
	}

	for _, tc := range tests {
		t.Run(tc.name+"_present", func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, tc.filename), []byte("content"), 0o644); err != nil {
				t.Fatal(err)
			}
			result := tc.checkFn(root)
			if result.Status != Pass {
				t.Errorf("expected Pass when file present, got %v: %s", result.Status, result.Message)
			}
		})

		t.Run(tc.name+"_missing", func(t *testing.T) {
			root := t.TempDir()
			result := tc.checkFn(root)
			if result.Status != tc.missingState {
				t.Errorf("expected %v when file missing, got %v: %s", tc.missingState, result.Status, result.Message)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Directory integrity check tests
// ──────────────────────────────────────────────────────────────────────────────

func TestCheckBaseDir_Present_NoModifications_ReturnsPass(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "base"), 0o755); err != nil {
		t.Fatal(err)
	}

	fakeRun := func(name string, args ...string) (string, error) {
		return "", nil // No diff output means no modifications.
	}

	result := CheckBaseDir(root, fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckBaseDir_Missing_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	fakeRun := func(name string, args ...string) (string, error) {
		return "", nil
	}

	result := CheckBaseDir(root, fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckBaseDir_Modified_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "base"), 0o755); err != nil {
		t.Fatal(err)
	}

	fakeRun := func(name string, args ...string) (string, error) {
		return "diff --git a/base/file.md b/base/file.md\n+modified", nil
	}

	result := CheckBaseDir(root, fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail for modified base/, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckBaseRecipes_Present_NoModifications_ReturnsPass(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "base", "recipes"), 0o755); err != nil {
		t.Fatal(err)
	}

	fakeRun := func(name string, args ...string) (string, error) {
		return "", nil
	}

	result := CheckBaseRecipes(root, fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckBaseRecipes_Missing_ReturnsWarning(t *testing.T) {
	root := t.TempDir()
	fakeRun := func(name string, args ...string) (string, error) {
		return "", nil
	}

	result := CheckBaseRecipes(root, fakeRun)
	if result.Status != Warning {
		t.Errorf("expected Warning, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckBaseRecipes_Modified_ReturnsWarning(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "base", "recipes"), 0o755); err != nil {
		t.Fatal(err)
	}

	fakeRun := func(name string, args ...string) (string, error) {
		return "diff output", nil
	}

	result := CheckBaseRecipes(root, fakeRun)
	if result.Status != Warning {
		t.Errorf("expected Warning for modified recipes, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckGooseRecipes_AllPresent_ReturnsPass(t *testing.T) {
	root := t.TempDir()
	recipesDir := filepath.Join(root, ".goose", "recipes")
	if err := os.MkdirAll(recipesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	for _, name := range expectedRecipeYAMLs {
		if err := os.WriteFile(filepath.Join(recipesDir, name), []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	result := CheckGooseRecipes(root)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckGooseRecipes_DirMissing_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	result := CheckGooseRecipes(root)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckGooseRecipes_SomeMissing_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	recipesDir := filepath.Join(root, ".goose", "recipes")
	if err := os.MkdirAll(recipesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Only create first 3 files.
	for _, name := range expectedRecipeYAMLs[:3] {
		if err := os.WriteFile(filepath.Join(recipesDir, name), []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	result := CheckGooseRecipes(root)
	if result.Status != Fail {
		t.Errorf("expected Fail for missing files, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckWorkflows_AllPresent_ReturnsPass(t *testing.T) {
	root := t.TempDir()
	workflowsDir := filepath.Join(root, ".github", "workflows")
	if err := os.MkdirAll(workflowsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	for _, name := range expectedWorkflowYMLs {
		if err := os.WriteFile(filepath.Join(workflowsDir, name), []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	result := CheckWorkflows(root)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckWorkflows_DirMissing_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	result := CheckWorkflows(root)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckWorkflows_SomeMissing_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	workflowsDir := filepath.Join(root, ".github", "workflows")
	if err := os.MkdirAll(workflowsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create no files — directory exists but all expected files are missing.

	result := CheckWorkflows(root)
	if result.Status != Fail {
		t.Errorf("expected Fail for missing workflows, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckBaseDir_GitFails_ReturnsWarning(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "base"), 0o755); err != nil {
		t.Fatal(err)
	}

	fakeRun := func(name string, args ...string) (string, error) {
		return "", fmt.Errorf("not a git repo")
	}

	result := CheckBaseDir(root, fakeRun)
	if result.Status != Warning {
		t.Errorf("expected Warning when git fails, got %v: %s", result.Status, result.Message)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// GitHub remote check tests
// ──────────────────────────────────────────────────────────────────────────────

func TestCheckLabels_AllPresent_ReturnsPass(t *testing.T) {
	labelsJSON := `[{"name":"requirement"},{"name":"feature"},{"name":"task"},{"name":"backlog"},{"name":"draft"},{"name":"in-design"},{"name":"in-development"},{"name":"in-review"},{"name":"done"}]`
	fakeRun := func(name string, args ...string) (string, error) {
		return labelsJSON, nil
	}

	result := CheckLabels("owner/repo", fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckLabels_SomeMissing_ReturnsFail(t *testing.T) {
	labelsJSON := `[{"name":"requirement"},{"name":"feature"},{"name":"task"}]`
	fakeRun := func(name string, args ...string) (string, error) {
		return labelsJSON, nil
	}

	result := CheckLabels("owner/repo", fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v: %s", result.Status, result.Message)
	}
	if result.Message == "" {
		t.Error("expected message listing missing labels")
	}
}

func TestCheckLabels_CommandFails_ReturnsFail(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return "", fmt.Errorf("network error")
	}

	result := CheckLabels("owner/repo", fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail on command error, got %v", result.Status)
	}
}

func TestCheckLabels_WithExtraLabels_ReturnsPass(t *testing.T) {
	labelsJSON := `[{"name":"requirement"},{"name":"feature"},{"name":"task"},{"name":"backlog"},{"name":"draft"},{"name":"in-design"},{"name":"in-development"},{"name":"in-review"},{"name":"done"},{"name":"bug"},{"name":"enhancement"}]`
	fakeRun := func(name string, args ...string) (string, error) {
		return labelsJSON, nil
	}

	result := CheckLabels("owner/repo", fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass with extra labels, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckProject_Exists_ReturnsPass(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return `{"projects":[{"title":"my-project"}]}`, nil
	}

	result := CheckProject("owner", fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckProject_None_ReturnsFail(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return `{"projects":[]}`, nil
	}

	result := CheckProject("owner", fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckProject_CommandFails_ReturnsFail(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return "", fmt.Errorf("auth error")
	}

	result := CheckProject("owner", fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail on command error, got %v", result.Status)
	}
}

func TestMissingLabels_AllPresent_ReturnsEmpty(t *testing.T) {
	labelsJSON := `[{"name":"requirement"},{"name":"feature"},{"name":"task"},{"name":"backlog"},{"name":"draft"},{"name":"in-design"},{"name":"in-development"},{"name":"in-review"},{"name":"done"}]`
	fakeRun := func(name string, args ...string) (string, error) {
		return labelsJSON, nil
	}

	missing := MissingLabels("owner/repo", fakeRun)
	if len(missing) != 0 {
		t.Errorf("expected no missing labels, got %v", missing)
	}
}

func TestMissingLabels_SomeMissing_ReturnsOnlyMissing(t *testing.T) {
	labelsJSON := `[{"name":"requirement"},{"name":"feature"}]`
	fakeRun := func(name string, args ...string) (string, error) {
		return labelsJSON, nil
	}

	missing := MissingLabels("owner/repo", fakeRun)
	if len(missing) != 7 {
		t.Errorf("expected 7 missing labels, got %d: %v", len(missing), missing)
	}
}
