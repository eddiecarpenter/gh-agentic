package verify

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func TestCheckSkillsDir_Present_ReturnsPass(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	result := CheckSkillsDir(root)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckSkillsDir_Absent_ReturnsWarning(t *testing.T) {
	root := t.TempDir()
	result := CheckSkillsDir(root)
	if result.Status != Warning {
		t.Errorf("expected Warning, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckTEMPLATESOURCE_Present_ReturnsPass(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "TEMPLATE_SOURCE"), []byte("eddiecarpenter/ai-native-delivery\n"), 0o644); err != nil {
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
	if err := os.MkdirAll(filepath.Join(root, "base", "skills"), 0o755); err != nil {
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
	if err := os.MkdirAll(filepath.Join(root, "base", "skills"), 0o755); err != nil {
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

func TestCheckWorkflows_WithBase_ContentMatches_ReturnsPass(t *testing.T) {
	root := t.TempDir()
	baseWfDir := filepath.Join(root, "base", ".github", "workflows")
	wfDir := filepath.Join(root, ".github", "workflows")
	if err := os.MkdirAll(baseWfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(baseWfDir, "pipeline.yml"), []byte("same"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, "pipeline.yml"), []byte("same"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := CheckWorkflows(root)
	if result.Status != Pass {
		t.Errorf("expected Pass when content matches, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckWorkflows_WithBase_ContentDiffers_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	baseWfDir := filepath.Join(root, "base", ".github", "workflows")
	wfDir := filepath.Join(root, ".github", "workflows")
	if err := os.MkdirAll(baseWfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(baseWfDir, "pipeline.yml"), []byte("new-content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, "pipeline.yml"), []byte("old-content"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := CheckWorkflows(root)
	if result.Status != Fail {
		t.Errorf("expected Fail when content differs, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "content differs") {
		t.Errorf("message should mention content differs: %s", result.Message)
	}
}

func TestCheckWorkflows_WithBase_FileMissing_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	baseWfDir := filepath.Join(root, "base", ".github", "workflows")
	if err := os.MkdirAll(baseWfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(baseWfDir, "pipeline.yml"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	// No .github/workflows/ directory at all.

	result := CheckWorkflows(root)
	if result.Status != Fail {
		t.Errorf("expected Fail when file missing, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "missing") {
		t.Errorf("message should mention missing: %s", result.Message)
	}
}

func TestCheckWorkflows_NoBase_FallsBackToExistence(t *testing.T) {
	root := t.TempDir()
	wfDir := filepath.Join(root, ".github", "workflows")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range expectedWorkflowYMLs {
		if err := os.WriteFile(filepath.Join(wfDir, name), []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// No base/.github/workflows/ — should fall back to existence check.
	result := CheckWorkflows(root)
	if result.Status != Pass {
		t.Errorf("expected Pass in fallback mode, got %v: %s", result.Status, result.Message)
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
	labelsJSON := `[{"name":"requirement"},{"name":"feature"},{"name":"task"},{"name":"backlog"},{"name":"draft"},{"name":"scoping"},{"name":"scheduled"},{"name":"in-design"},{"name":"in-development"},{"name":"in-review"},{"name":"done"}]`
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
	labelsJSON := `[{"name":"requirement"},{"name":"feature"},{"name":"task"},{"name":"backlog"},{"name":"draft"},{"name":"scoping"},{"name":"scheduled"},{"name":"in-design"},{"name":"in-development"},{"name":"in-review"},{"name":"done"},{"name":"bug"},{"name":"enhancement"}]`
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
	labelsJSON := `[{"name":"requirement"},{"name":"feature"},{"name":"task"},{"name":"backlog"},{"name":"draft"},{"name":"scoping"},{"name":"scheduled"},{"name":"in-design"},{"name":"in-development"},{"name":"in-review"},{"name":"done"}]`
	fakeRun := func(name string, args ...string) (string, error) {
		return labelsJSON, nil
	}

	missing := MissingLabels("owner/repo", fakeRun)
	if len(missing) != 0 {
		t.Errorf("expected no missing labels, got %v", missing)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// gh-notify LaunchAgent check tests
// ──────────────────────────────────────────────────────────────────────────────

func TestCheckGhNotify_NonDarwin_ReturnsPass(t *testing.T) {
	orig := ghNotifyGOOS
	defer func() { ghNotifyGOOS = orig }()
	ghNotifyGOOS = "linux"

	root := t.TempDir()
	fakeRun := func(name string, args ...string) (string, error) {
		t.Error("run should not be called on non-darwin")
		return "", nil
	}

	result := CheckGhNotify(root, fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass on non-darwin, got %v: %s", result.Status, result.Message)
	}
	if result.Message != "not applicable on this OS" {
		t.Errorf("unexpected message: %s", result.Message)
	}
}

func TestCheckGhNotify_PlistMissing_ReturnsFail(t *testing.T) {
	orig := ghNotifyGOOS
	defer func() { ghNotifyGOOS = orig }()
	ghNotifyGOOS = "darwin"

	// Skip if the plist already exists on this machine — the test requires it to be absent.
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}
	plistPath := filepath.Join(homeDir, "Library", "LaunchAgents", "com.user.gh-notify.plist")
	if _, statErr := os.Stat(plistPath); statErr == nil {
		t.Skip("plist already exists on this machine — skipping PlistMissing test")
	}

	root := t.TempDir()
	fakeRun := func(name string, args ...string) (string, error) {
		return "", nil
	}

	result := CheckGhNotify(root, fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail when plist missing, got %v: %s", result.Status, result.Message)
	}
	if result.Message != "plist not found at ~/Library/LaunchAgents/com.user.gh-notify.plist" {
		t.Errorf("unexpected message: %s", result.Message)
	}
}

func TestCheckGhNotify_LaunchctlFails_ReturnsFail(t *testing.T) {
	orig := ghNotifyGOOS
	defer func() { ghNotifyGOOS = orig }()
	ghNotifyGOOS = "darwin"

	// Create the plist file in the user's expected location.
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}
	plistDir := filepath.Join(homeDir, "Library", "LaunchAgents")
	plistPath := filepath.Join(plistDir, "com.user.gh-notify.plist")

	// Only run this test if the plist file exists (or we can create a temp one).
	// Since we can't reliably create in ~/Library, use a different approach:
	// check if the plist already exists.
	if _, statErr := os.Stat(plistPath); os.IsNotExist(statErr) {
		// Create a temporary plist to test launchctl failure path.
		if mkErr := os.MkdirAll(plistDir, 0o755); mkErr != nil {
			t.Skipf("cannot create plist dir: %v", mkErr)
		}
		if wErr := os.WriteFile(plistPath, []byte("<plist/>"), 0o644); wErr != nil {
			t.Skipf("cannot create plist file: %v", wErr)
		}
		defer os.Remove(plistPath)
	}

	root := t.TempDir()
	fakeRun := func(name string, args ...string) (string, error) {
		return "", fmt.Errorf("not loaded")
	}

	result := CheckGhNotify(root, fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail when launchctl fails, got %v: %s", result.Status, result.Message)
	}
	if result.Message != "LaunchAgent not loaded" {
		t.Errorf("unexpected message: %s", result.Message)
	}
}

func TestCheckGhNotify_AllGood_ReturnsPass(t *testing.T) {
	orig := ghNotifyGOOS
	defer func() { ghNotifyGOOS = orig }()
	ghNotifyGOOS = "darwin"

	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}
	plistDir := filepath.Join(homeDir, "Library", "LaunchAgents")
	plistPath := filepath.Join(plistDir, "com.user.gh-notify.plist")

	if _, statErr := os.Stat(plistPath); os.IsNotExist(statErr) {
		if mkErr := os.MkdirAll(plistDir, 0o755); mkErr != nil {
			t.Skipf("cannot create plist dir: %v", mkErr)
		}
		if wErr := os.WriteFile(plistPath, []byte("<plist/>"), 0o644); wErr != nil {
			t.Skipf("cannot create plist file: %v", wErr)
		}
		defer os.Remove(plistPath)
	}

	root := t.TempDir()
	fakeRun := func(name string, args ...string) (string, error) {
		return "com.user.gh-notify", nil // Success
	}

	result := CheckGhNotify(root, fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
	if result.Message != "gh-notify LaunchAgent installed and running" {
		t.Errorf("unexpected message: %s", result.Message)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// resolveProjectNodeIDViaRun tests
// ──────────────────────────────────────────────────────────────────────────────

func TestResolveProjectNodeIDViaRun_TitleMatch_ReturnsCorrectID(t *testing.T) {
	tests := []struct {
		name       string
		repoName   string
		jsonResp   string
		runErr     error
		expectedID string
	}{
		{
			name:       "exact title match among multiple projects",
			repoName:   "my-domain",
			jsonResp:   `{"projects":[{"id":"PVT_AAA","title":"other-project","number":1},{"id":"PVT_BBB","title":"my-domain","number":2},{"id":"PVT_CCC","title":"yet-another","number":3}]}`,
			expectedID: "PVT_BBB",
		},
		{
			name:       "no title match falls back to first project",
			repoName:   "nonexistent-repo",
			jsonResp:   `{"projects":[{"id":"PVT_AAA","title":"alpha","number":1},{"id":"PVT_BBB","title":"beta","number":2}]}`,
			expectedID: "PVT_AAA",
		},
		{
			name:       "empty project list returns empty string",
			repoName:   "my-repo",
			jsonResp:   `{"projects":[]}`,
			expectedID: "",
		},
		{
			name:       "single project matching returns its ID",
			repoName:   "solo-project",
			jsonResp:   `{"projects":[{"id":"PVT_SOLO","title":"solo-project","number":1}]}`,
			expectedID: "PVT_SOLO",
		},
		{
			name:       "single project not matching still returns its ID as fallback",
			repoName:   "different-name",
			jsonResp:   `{"projects":[{"id":"PVT_SOLO","title":"solo-project","number":1}]}`,
			expectedID: "PVT_SOLO",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fakeRun := func(name string, args ...string) (string, error) {
				if tc.runErr != nil {
					return "", tc.runErr
				}
				return tc.jsonResp, nil
			}

			got := resolveProjectNodeIDViaRun("owner", tc.repoName, fakeRun)
			if got != tc.expectedID {
				t.Errorf("expected %q, got %q", tc.expectedID, got)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// CheckProjectStatus tests
// ──────────────────────────────────────────────────────────────────────────────

// projectListJSON is the JSON returned by `gh project list --format json`.
const projectListJSON = `{"projects":[{"id":"PVT_123","title":"my-repo","number":1}]}`

// writeTestProjectTemplate creates a valid base/project-template.json fixture.
func writeTestProjectTemplate(t *testing.T, root string) {
	t.Helper()
	baseDir := filepath.Join(root, "base")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `{
  "statusOptions": [
    {"name": "Backlog",        "color": "GRAY",   "description": "Prioritised, ready to start"},
    {"name": "Scoping",        "color": "PURPLE", "description": "Requirement or feature being scoped"},
    {"name": "Scheduled",      "color": "BLUE",   "description": "Scoped and queued, waiting for design"},
    {"name": "In Design",      "color": "PINK",   "description": "Feature Design session active"},
    {"name": "In Development", "color": "YELLOW", "description": "Dev Session active"},
    {"name": "In Review",      "color": "ORANGE", "description": "PR open, awaiting review"},
    {"name": "Done",           "color": "GREEN",  "description": "Merged and closed"}
  ]
}`
	if err := os.WriteFile(filepath.Join(baseDir, "project-template.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCheckProjectStatus_AllMatch_ReturnsPass(t *testing.T) {
	root := t.TempDir()
	writeTestProjectTemplate(t, root)
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		// First call: gh project list (resolve project node ID).
		if callCount == 1 {
			return projectListJSON, nil
		}
		// Second call: fetch status options (canonical 7-option order).
		return "Backlog|GRAY\nScoping|PURPLE\nScheduled|BLUE\nIn Design|PINK\nIn Development|YELLOW\nIn Review|ORANGE\nDone|GREEN", nil
	}

	result := CheckProjectStatus("owner", "my-repo", root, fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckProjectStatus_WrongOrder_ReturnsWarning(t *testing.T) {
	root := t.TempDir()
	writeTestProjectTemplate(t, root)
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			return projectListJSON, nil
		}
		// Wrong order: Done first.
		return "Done|GREEN\nBacklog|GRAY\nScoping|PURPLE\nScheduled|BLUE\nIn Design|PINK\nIn Development|YELLOW\nIn Review|ORANGE", nil
	}

	result := CheckProjectStatus("owner", "my-repo", root, fakeRun)
	if result.Status != Warning {
		t.Errorf("expected Warning for wrong order, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckProjectStatus_MissingOption_ReturnsWarning(t *testing.T) {
	root := t.TempDir()
	writeTestProjectTemplate(t, root)
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			return projectListJSON, nil
		}
		// Only 5 options.
		return "Backlog|GRAY\nScoping|PURPLE\nScheduled|BLUE\nIn Design|PINK\nDone|GREEN", nil
	}

	result := CheckProjectStatus("owner", "my-repo", root, fakeRun)
	if result.Status != Warning {
		t.Errorf("expected Warning for missing option, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckProjectStatus_ExtraOption_ReturnsWarning(t *testing.T) {
	root := t.TempDir()
	writeTestProjectTemplate(t, root)
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			return projectListJSON, nil
		}
		// 8 options (one extra).
		return "Backlog|GRAY\nScoping|PURPLE\nScheduled|BLUE\nIn Design|PINK\nIn Development|YELLOW\nIn Review|ORANGE\nDone|GREEN\nArchived|RED", nil
	}

	result := CheckProjectStatus("owner", "my-repo", root, fakeRun)
	if result.Status != Warning {
		t.Errorf("expected Warning for extra option, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckProjectStatus_NoProject_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	writeTestProjectTemplate(t, root)
	fakeRun := func(name string, args ...string) (string, error) {
		return "", fmt.Errorf("no project found")
	}

	result := CheckProjectStatus("owner", "my-repo", root, fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail for no project, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckProjectStatus_GraphQLError_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	writeTestProjectTemplate(t, root)
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			return projectListJSON, nil
		}
		return "", fmt.Errorf("GraphQL error")
	}

	result := CheckProjectStatus("owner", "my-repo", root, fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail for GraphQL error, got %v: %s", result.Status, result.Message)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// CheckProjectCollaborator tests
// ──────────────────────────────────────────────────────────────────────────────

func TestCheckProjectCollaborator_EmptyAgentUser_ReturnsPass(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		t.Fatal("run should not be called when agent user is empty")
		return "", nil
	}

	result := CheckProjectCollaborator("owner", "my-repo", "", fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckProjectCollaborator_UserPresent_ReturnsPass(t *testing.T) {
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			return projectListJSON, nil // resolve project node ID via gh project list
		}
		return "alice\ngoose-agent\nbob", nil // collaborators
	}

	result := CheckProjectCollaborator("owner", "my-repo", "goose-agent", fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckProjectCollaborator_UserAbsent_ReturnsFail(t *testing.T) {
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			return projectListJSON, nil
		}
		return "alice\nbob", nil // goose-agent not present
	}

	result := CheckProjectCollaborator("owner", "my-repo", "goose-agent", fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckProjectCollaborator_APIFailure_ReturnsFail(t *testing.T) {
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			return projectListJSON, nil
		}
		return "", fmt.Errorf("API error")
	}

	result := CheckProjectCollaborator("owner", "my-repo", "goose-agent", fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v: %s", result.Status, result.Message)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// CheckProjectItemStatuses tests
// ──────────────────────────────────────────────────────────────────────────────

func TestCheckProjectItemStatuses_AllHaveStatus_ReturnsPass(t *testing.T) {
	root := t.TempDir()
	writeTestProjectTemplate(t, root)
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		switch callCount {
		case 1:
			// resolveProjectNodeIDViaRun
			return projectListJSON, nil
		case 2:
			// Fetch Status field ID.
			return "FIELD_1", nil
		case 3:
			// fetchAllProjectItems — all items have a status.
			return `{"data":{"node":{"items":{"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[{"id":"I1","content":{"state":"OPEN","labels":{"nodes":[]}},"fieldValues":{"nodes":[{"field":{"id":"FIELD_1"},"name":"Backlog"}]}},{"id":"I2","content":{"state":"CLOSED","labels":{"nodes":[]}},"fieldValues":{"nodes":[{"field":{"id":"FIELD_1"},"name":"Done"}]}}]}}}}`, nil
		}
		return "", nil
	}

	result := CheckProjectItemStatuses("owner", "my-repo", root, fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckProjectItemStatuses_SomeMissing_ReturnsWarning(t *testing.T) {
	root := t.TempDir()
	writeTestProjectTemplate(t, root)
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		switch callCount {
		case 1:
			return projectListJSON, nil
		case 2:
			return "FIELD_1", nil
		case 3:
			// Two items: one with status, one without.
			return `{"data":{"node":{"items":{"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[{"id":"I1","content":{"state":"OPEN","labels":{"nodes":[]}},"fieldValues":{"nodes":[{"field":{"id":"FIELD_1"},"name":"Backlog"}]}},{"id":"I2","content":{"state":"OPEN","labels":{"nodes":[]}},"fieldValues":{"nodes":[]}}]}}}}`, nil
		}
		return "", nil
	}

	result := CheckProjectItemStatuses("owner", "my-repo", root, fakeRun)
	if result.Status != Warning {
		t.Errorf("expected Warning, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "1 project items have no status") {
		t.Errorf("expected count in message, got: %s", result.Message)
	}
}

func TestCheckProjectItemStatuses_NoItems_ReturnsPass(t *testing.T) {
	root := t.TempDir()
	writeTestProjectTemplate(t, root)
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		switch callCount {
		case 1:
			return projectListJSON, nil
		case 2:
			return "FIELD_1", nil
		case 3:
			return `{"data":{"node":{"items":{"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[]}}}}`, nil
		}
		return "", nil
	}

	result := CheckProjectItemStatuses("owner", "my-repo", root, fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass for empty project, got %v: %s", result.Status, result.Message)
	}
}

func TestMissingLabels_SomeMissing_ReturnsOnlyMissing(t *testing.T) {
	labelsJSON := `[{"name":"requirement"},{"name":"feature"}]`
	fakeRun := func(name string, args ...string) (string, error) {
		return labelsJSON, nil
	}

	missing := MissingLabels("owner/repo", fakeRun)
	if len(missing) != 9 {
		t.Errorf("expected 9 missing labels, got %d: %v", len(missing), missing)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// CheckAgentUserVar tests
// ──────────────────────────────────────────────────────────────────────────────

func TestCheckAgentUserVar_PresentAtOrgLevel_ReturnsPass(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "--org") {
			return `[{"name":"AGENT_USER"},{"name":"OTHER_VAR"}]`, nil
		}
		return `[]`, nil
	}

	result := CheckAgentUserVar("acme-org", "my-repo", fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "org level") {
		t.Errorf("expected message about org level, got %q", result.Message)
	}
}

func TestCheckAgentUserVar_PresentAtRepoLevel_ReturnsPass(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "--org") {
			return "", fmt.Errorf("not an org")
		}
		if strings.Contains(joined, "--repo") {
			return `[{"name":"AGENT_USER"}]`, nil
		}
		return `[]`, nil
	}

	result := CheckAgentUserVar("alice", "my-repo", fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "repo level") {
		t.Errorf("expected message about repo level, got %q", result.Message)
	}
}

func TestCheckAgentUserVar_MissingAtBothLevels_ReturnsFail(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "--org") {
			return "", fmt.Errorf("not an org")
		}
		if strings.Contains(joined, "--repo") {
			return `[{"name":"OTHER_VAR"}]`, nil
		}
		return `[]`, nil
	}

	result := CheckAgentUserVar("alice", "my-repo", fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "not set") {
		t.Errorf("expected message about not set, got %q", result.Message)
	}
}

func TestCheckAgentUserVar_OrgSucceedsButNoAgentUser_FallsToRepo(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "--org") {
			return `[{"name":"OTHER_VAR"}]`, nil
		}
		if strings.Contains(joined, "--repo") {
			return `[{"name":"AGENT_USER"}]`, nil
		}
		return `[]`, nil
	}

	result := CheckAgentUserVar("acme-org", "my-repo", fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "repo level") {
		t.Errorf("expected message about repo level, got %q", result.Message)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// ReadAgentUserVar tests
// ──────────────────────────────────────────────────────────────────────────────

func TestReadAgentUserVar_FoundAtOrgLevel_ReturnsValue(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "--org") {
			return `[{"name":"AGENT_USER","value":"goose-agent"}]`, nil
		}
		return `[]`, nil
	}

	val := ReadAgentUserVar("acme-org", "my-repo", fakeRun)
	if val != "goose-agent" {
		t.Errorf("expected %q, got %q", "goose-agent", val)
	}
}

func TestReadAgentUserVar_FoundAtRepoLevel_ReturnsValue(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "--org") {
			return "", fmt.Errorf("not an org")
		}
		if strings.Contains(joined, "--repo") {
			return `[{"name":"AGENT_USER","value":"repo-agent"}]`, nil
		}
		return `[]`, nil
	}

	val := ReadAgentUserVar("alice", "my-repo", fakeRun)
	if val != "repo-agent" {
		t.Errorf("expected %q, got %q", "repo-agent", val)
	}
}

func TestReadAgentUserVar_NotFound_ReturnsEmpty(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "--org") {
			return "", fmt.Errorf("not an org")
		}
		return `[]`, nil
	}

	val := ReadAgentUserVar("alice", "my-repo", fakeRun)
	if val != "" {
		t.Errorf("expected empty, got %q", val)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// CheckAgenticProjectID tests
// ──────────────────────────────────────────────────────────────────────────────

func TestCheckAgenticProjectID(t *testing.T) {
	tests := []struct {
		name       string
		runOut     string
		runErr     error
		wantStatus CheckStatus
	}{
		{
			name:       "variable is set returns Pass",
			runOut:     "PVT_kwDOBtest",
			runErr:     nil,
			wantStatus: Pass,
		},
		{
			name:       "variable command fails returns Fail",
			runOut:     "",
			runErr:     fmt.Errorf("exit status 1"),
			wantStatus: Fail,
		},
		{
			name:       "variable returns empty returns Fail",
			runOut:     "",
			runErr:     nil,
			wantStatus: Fail,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fakeRun := func(name string, args ...string) (string, error) {
				return tc.runOut, tc.runErr
			}
			result := CheckAgenticProjectID("owner/repo", "owner", "repo", fakeRun)
			if result.Status != tc.wantStatus {
				t.Errorf("expected %v, got %v: %s", tc.wantStatus, result.Status, result.Message)
			}
			if result.Name != "AGENTIC_PROJECT_ID is configured" {
				t.Errorf("unexpected check name: %s", result.Name)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Pipeline variable check tests
// ──────────────────────────────────────────────────────────────────────────────

func TestCheckRunnerLabelVar_Present_ReturnsPass(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return `[{"name":"RUNNER_LABEL"},{"name":"OTHER"}]`, nil
	}
	result := CheckRunnerLabelVar("owner", "repo", fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckRunnerLabelVar_Missing_ReturnsWarning(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return `[{"name":"OTHER"}]`, nil
	}
	result := CheckRunnerLabelVar("owner", "repo", fakeRun)
	if result.Status != Warning {
		t.Errorf("expected Warning, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "ubuntu-latest") {
		t.Errorf("expected message to mention default value, got %q", result.Message)
	}
}

func TestCheckRunnerLabelVar_ParseError_ReturnsWarning(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return `not json`, nil
	}
	result := CheckRunnerLabelVar("owner", "repo", fakeRun)
	if result.Status != Warning {
		t.Errorf("expected Warning, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "parse") {
		t.Errorf("expected parse error message, got %q", result.Message)
	}
}

func TestCheckGooseProviderVar_Present_ReturnsPass(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return `[{"name":"GOOSE_PROVIDER"}]`, nil
	}
	result := CheckGooseProviderVar("owner", "repo", fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckGooseProviderVar_Missing_ReturnsWarning(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return `[]`, nil
	}
	result := CheckGooseProviderVar("owner", "repo", fakeRun)
	if result.Status != Warning {
		t.Errorf("expected Warning, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "claude-code") {
		t.Errorf("expected message to mention default value, got %q", result.Message)
	}
}

func TestCheckGooseProviderVar_ParseError_ReturnsWarning(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return `{bad}`, nil
	}
	result := CheckGooseProviderVar("owner", "repo", fakeRun)
	if result.Status != Warning {
		t.Errorf("expected Warning, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckGooseModelVar_Present_ReturnsPass(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return `[{"name":"GOOSE_MODEL"}]`, nil
	}
	result := CheckGooseModelVar("owner", "repo", fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckGooseModelVar_Missing_ReturnsWarning(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return `[]`, nil
	}
	result := CheckGooseModelVar("owner", "repo", fakeRun)
	if result.Status != Warning {
		t.Errorf("expected Warning, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "default") {
		t.Errorf("expected message to mention default value, got %q", result.Message)
	}
}

func TestCheckGooseModelVar_ParseError_ReturnsWarning(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return `not valid`, nil
	}
	result := CheckGooseModelVar("owner", "repo", fakeRun)
	if result.Status != Warning {
		t.Errorf("expected Warning, got %v: %s", result.Status, result.Message)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Pipeline secret check tests
// ──────────────────────────────────────────────────────────────────────────────

func TestCheckGooseAgentPATSecret_Present_ReturnsPass(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return `[{"name":"GOOSE_AGENT_PAT"}]`, nil
	}
	result := CheckGooseAgentPATSecret("owner", "repo", fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckGooseAgentPATSecret_Missing_ReturnsWarning(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return `[{"name":"OTHER_SECRET"}]`, nil
	}
	result := CheckGooseAgentPATSecret("owner", "repo", fakeRun)
	if result.Status != Warning {
		t.Errorf("expected Warning, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckGooseAgentPATSecret_ParseError_ReturnsWarning(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return `bad json`, nil
	}
	result := CheckGooseAgentPATSecret("owner", "repo", fakeRun)
	if result.Status != Warning {
		t.Errorf("expected Warning, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckClaudeCredentialsSecret_Present_ReturnsPass(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return `[{"name":"CLAUDE_CREDENTIALS_JSON"}]`, nil
	}
	result := CheckClaudeCredentialsSecret("owner", "repo", fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckClaudeCredentialsSecret_Missing_ReturnsWarning(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return `[]`, nil
	}
	result := CheckClaudeCredentialsSecret("owner", "repo", fakeRun)
	if result.Status != Warning {
		t.Errorf("expected Warning, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckClaudeCredentialsSecret_ParseError_ReturnsWarning(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return `invalid`, nil
	}
	result := CheckClaudeCredentialsSecret("owner", "repo", fakeRun)
	if result.Status != Warning {
		t.Errorf("expected Warning, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckRunnerLabelVar_CommandError_ReturnsWarning(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return "", fmt.Errorf("network error")
	}
	result := CheckRunnerLabelVar("owner", "repo", fakeRun)
	if result.Status != Warning {
		t.Errorf("expected Warning, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckGooseAgentPATSecret_CommandError_ReturnsWarning(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return "", fmt.Errorf("network error")
	}
	result := CheckGooseAgentPATSecret("owner", "repo", fakeRun)
	if result.Status != Warning {
		t.Errorf("expected Warning, got %v: %s", result.Status, result.Message)
	}
}
