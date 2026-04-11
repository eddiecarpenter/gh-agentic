package verify

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFailureDir creates a directory at path so that os.WriteFile on that path
// fails with "is a directory". This is the cleanest cross-platform way to trigger
// a write failure without relying on file permissions.
func writeFailureDir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("writeFailureDir: %v", err)
	}
}

// ── Write-failure error paths ─────────────────────────────────────────────────

func TestRepairCLAUDEMD_WriteFailure_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	writeFailureDir(t, filepath.Join(root, "CLAUDE.md"))

	result := RepairCLAUDEMD(root)
	if result.Status != Fail {
		t.Errorf("expected Fail when write blocked, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "repair failed") {
		t.Errorf("expected 'repair failed' in message, got: %s", result.Message)
	}
}

func TestRepairLOCALRULESMD_WriteFailure_ReturnsWarning(t *testing.T) {
	root := t.TempDir()
	writeFailureDir(t, filepath.Join(root, "LOCALRULES.md"))

	result := RepairLOCALRULESMD(root)
	if result.Status != Warning {
		t.Errorf("expected Warning when write blocked, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "repair failed") {
		t.Errorf("expected 'repair failed' in message, got: %s", result.Message)
	}
}

func TestRepairREPOSMD_WriteFailure_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	writeFailureDir(t, filepath.Join(root, "REPOS.md"))

	result := RepairREPOSMD(root)
	if result.Status != Fail {
		t.Errorf("expected Fail when write blocked, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "repair failed") {
		t.Errorf("expected 'repair failed' in message, got: %s", result.Message)
	}
}

func TestRepairREADMEMD_WriteFailure_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	writeFailureDir(t, filepath.Join(root, "README.md"))

	result := RepairREADMEMD(root)
	if result.Status != Fail {
		t.Errorf("expected Fail when write blocked, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "repair failed") {
		t.Errorf("expected 'repair failed' in message, got: %s", result.Message)
	}
}

// ── RepairSkillsDir error paths ───────────────────────────────────────────────

func TestRepairSkillsDir_MkdirFails_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	// Place a regular file at the skills/ path so MkdirAll fails.
	if err := os.WriteFile(filepath.Join(root, "skills"), []byte("blocker"), 0o644); err != nil {
		t.Fatal(err)
	}
	fakeRun := func(name string, args ...string) (string, error) { return "", nil }

	result := RepairSkillsDir(root, fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail when mkdir blocked, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "repair failed") {
		t.Errorf("expected 'repair failed' in message, got: %s", result.Message)
	}
}

func TestRepairSkillsDir_WriteFileFails_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	// Create skills/ dir, then block .gitkeep write by placing a directory there.
	if err := os.MkdirAll(filepath.Join(root, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFailureDir(t, filepath.Join(root, "skills", ".gitkeep"))
	fakeRun := func(name string, args ...string) (string, error) { return "", nil }

	result := RepairSkillsDir(root, fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail when WriteFile blocked, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "repair failed") {
		t.Errorf("expected 'repair failed' in message, got: %s", result.Message)
	}
}

func TestRepairSkillsDir_GitAddFails_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	fakeRun := func(name string, args ...string) (string, error) {
		return "", fmt.Errorf("git add: not a git repository")
	}

	result := RepairSkillsDir(root, fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail when git add fails, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "git add failed") {
		t.Errorf("expected 'git add failed' in message, got: %s", result.Message)
	}
}

// ── RepairAIConfigYML error paths ─────────────────────────────────────────────

func TestRepairAIConfigYML_RunFails_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	writeTemplateConfig(t, root, "owner/repo", "v1.0.0")
	fakeRun := func(name string, args ...string) (string, error) {
		return "", fmt.Errorf("gh: not authenticated")
	}

	result := RepairAIConfigYML(root, fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail when run fails, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "failed to fetch latest tag") {
		t.Errorf("expected 'failed to fetch latest tag' in message, got: %s", result.Message)
	}
}

func TestRepairAIConfigYML_EmptyTag_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	writeTemplateConfig(t, root, "owner/repo", "v1.0.0")
	fakeRun := func(name string, args ...string) (string, error) {
		return "   \n", nil // whitespace only — trims to ""
	}

	result := RepairAIConfigYML(root, fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail for empty tag, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "no releases found") {
		t.Errorf("expected 'no releases found' in message, got: %s", result.Message)
	}
}

func TestRepairAIConfigYML_WriteFailure_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	// Provide the template repo via TEMPLATE_SOURCE so the function can read it
	// without needing .ai/config.yml to be readable.
	if err := os.WriteFile(filepath.Join(root, "TEMPLATE_SOURCE"), []byte("owner/repo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create .ai/ then block the config.yml write with a directory at that path.
	if err := os.MkdirAll(filepath.Join(root, ".ai"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFailureDir(t, filepath.Join(root, ".ai", "config.yml"))

	fakeRun := func(name string, args ...string) (string, error) {
		return "v2.0.0\n", nil
	}

	result := RepairAIConfigYML(root, fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail when write blocked, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "repair failed") {
		t.Errorf("expected 'repair failed' in message, got: %s", result.Message)
	}
}

// ── RepairProjectStatus additional error paths ────────────────────────────────

func TestRepairProjectStatus_FieldIDFetchFails_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	writeTestProjectTemplate(t, root)
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			return projectListJSON, nil // resolve project node ID
		}
		return "", fmt.Errorf("graphql: server error") // field ID fetch fails
	}

	result := RepairProjectStatus("owner", "my-repo", root, fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "failed to fetch Status field ID") {
		t.Errorf("expected field ID error in message, got: %s", result.Message)
	}
}

func TestRepairProjectStatus_FieldIDEmpty_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	writeTestProjectTemplate(t, root)
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			return projectListJSON, nil
		}
		return "null\n", nil // empty / null field ID
	}

	result := RepairProjectStatus("owner", "my-repo", root, fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail for null field ID, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "Status field not found") {
		t.Errorf("expected 'Status field not found' in message, got: %s", result.Message)
	}
}

func TestRepairProjectStatus_ResyncFails_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	writeTestProjectTemplate(t, root)
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		switch callCount {
		case 1:
			return projectListJSON, nil // resolve project
		case 2:
			return "FIELD_456\n", nil // field ID
		case 3:
			return `{"data":{}}`, nil // mutation OK
		default:
			return "", fmt.Errorf("resync: option fetch failed") // fetchStatusOptionMap fails
		}
	}

	result := RepairProjectStatus("owner", "my-repo", root, fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail when resync fails, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "item resync failed") {
		t.Errorf("expected 'item resync failed' in message, got: %s", result.Message)
	}
}

// ── ResyncProjectItemStatuses / RepairProjectItemStatuses ─────────────────────

func TestResyncProjectItemStatuses_NoProject_ReturnsError(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return "", fmt.Errorf("no project found")
	}

	_, _, err := ResyncProjectItemStatuses("owner", "my-repo", fakeRun)
	if err == nil {
		t.Fatal("expected error when project not found, got nil")
	}
	if !strings.Contains(err.Error(), "no GitHub Project found") {
		t.Errorf("expected 'no GitHub Project found' in error, got: %v", err)
	}
}

func TestResyncProjectItemStatuses_FieldFetchFails_ReturnsError(t *testing.T) {
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			return projectListJSON, nil // project list
		}
		return "", fmt.Errorf("field query: server error")
	}

	_, _, err := ResyncProjectItemStatuses("owner", "my-repo", fakeRun)
	if err == nil {
		t.Fatal("expected error when field fetch fails, got nil")
	}
	if !strings.Contains(err.Error(), "fetching Status field ID") {
		t.Errorf("expected field ID error, got: %v", err)
	}
}

func TestResyncProjectItemStatuses_FieldEmpty_ReturnsError(t *testing.T) {
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			return projectListJSON, nil
		}
		return "null\n", nil // field ID is null/empty
	}

	_, _, err := ResyncProjectItemStatuses("owner", "my-repo", fakeRun)
	if err == nil {
		t.Fatal("expected error for null field ID, got nil")
	}
	if !strings.Contains(err.Error(), "status field not found") {
		t.Errorf("expected 'status field not found', got: %v", err)
	}
}

func TestRepairProjectItemStatuses_Fail_ReturnsFail(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return "", fmt.Errorf("project lookup failed")
	}

	result := RepairProjectItemStatuses("owner", "my-repo", fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "resync failed") {
		t.Errorf("expected 'resync failed' in message, got: %s", result.Message)
	}
}

func TestRepairProjectItemStatuses_Success_ReturnsPass(t *testing.T) {
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		switch callCount {
		case 1:
			return projectListJSON, nil // project list
		case 2:
			return "FIELD_456\n", nil // Status field ID
		case 3:
			return "OPT_1|Backlog\nOPT_2|Done", nil // fetchStatusOptionMap
		case 4:
			// fetchAllProjectItems — empty page, no items to resync.
			return `{"data":{"node":{"items":{"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[]}}}}`, nil
		}
		return "", nil
	}

	result := RepairProjectItemStatuses("owner", "my-repo", fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
}

// ── RepairAIDir — git checkout failure path ───────────────────────────────────

func TestRepairAIDir_GitCheckoutFails_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	// Create .ai/ so the repair takes the git-checkout path (not the sync path).
	if err := os.MkdirAll(filepath.Join(root, ".ai"), 0o755); err != nil {
		t.Fatal(err)
	}
	fakeRun := func(name string, args ...string) (string, error) {
		return "", fmt.Errorf("git checkout: permission denied")
	}
	confirmFn := func(prompt string) (bool, error) { return true, nil }

	result := RepairAIDir(root, fakeRun, confirmFn)
	if result.Status != Fail {
		t.Errorf("expected Fail when git checkout fails, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "git checkout failed") {
		t.Errorf("expected 'git checkout failed' in message, got: %s", result.Message)
	}
}

// ── RepairAISkills — tarball failure path ─────────────────────────────────────

func TestRepairAISkills_TarballFails_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	writeTemplateConfig(t, root, "owner/template", "v1.0.0")
	confirmFn := func(prompt string) (bool, error) { return true, nil }

	result := RepairAISkills(root, confirmFn, failingFetchFunc("tarball unavailable"))
	if result.Status != Fail {
		t.Errorf("expected Fail when tarball fails, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "tarball extraction failed") {
		t.Errorf("expected 'tarball extraction failed' in message, got: %s", result.Message)
	}
}

// ── closeStaleIssues / RepairStaleOpen* ──────────────────────────────────────

const (
	staleIssueListJSON    = `[{"number":42,"title":"Stale Requirement"}]`
	allClosedSubIssuesJSON = `{"subIssues":[{"state":"CLOSED"},{"state":"CLOSED"}]}`
)

func TestRepairStaleOpenRequirements_FetchFails_ReturnsFail(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return "", fmt.Errorf("gh: not authenticated")
	}

	result := RepairStaleOpenRequirements("owner/repo", fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail when fetch fails, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "failed to fetch stale issues") {
		t.Errorf("expected 'failed to fetch stale issues' in message, got: %s", result.Message)
	}
}

func TestRepairStaleOpenRequirements_NoStale_ReturnsPass(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return "[]", nil // empty issue list
	}

	result := RepairStaleOpenRequirements("owner/repo", fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass when no stale issues, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "nothing to repair") {
		t.Errorf("expected 'nothing to repair' in message, got: %s", result.Message)
	}
}

func TestRepairStaleOpenRequirements_LabelFails_ReturnsFail(t *testing.T) {
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		switch callCount {
		case 1:
			return staleIssueListJSON, nil // issue list
		case 2:
			return allClosedSubIssuesJSON, nil // sub-issues (all closed → stale)
		case 3:
			return "", fmt.Errorf("label edit failed") // issue edit --add-label done
		}
		return "", nil
	}

	result := RepairStaleOpenRequirements("owner/repo", fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail when label edit fails, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "fixing labels") {
		t.Errorf("expected 'fixing labels' in message, got: %s", result.Message)
	}
}

func TestRepairStaleOpenRequirements_CloseFails_ReturnsFail(t *testing.T) {
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		switch callCount {
		case 1:
			return staleIssueListJSON, nil
		case 2:
			return allClosedSubIssuesJSON, nil
		case 3:
			return "", nil // label edit OK
		case 4:
			return "", fmt.Errorf("issue close: server error")
		}
		return "", nil
	}

	result := RepairStaleOpenRequirements("owner/repo", fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail when close fails, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "closing") {
		t.Errorf("expected 'closing' in message, got: %s", result.Message)
	}
}

func TestRepairStaleOpenRequirements_Success_ReturnsPass(t *testing.T) {
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		switch callCount {
		case 1:
			return staleIssueListJSON, nil
		case 2:
			return allClosedSubIssuesJSON, nil
		case 3:
			return "", nil // label edit
		case 4:
			return "", nil // issue close
		}
		return "", nil
	}

	result := RepairStaleOpenRequirements("owner/repo", fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "#42") {
		t.Errorf("expected closed issue #42 in message, got: %s", result.Message)
	}
}

func TestRepairStaleOpenFeatures_FetchFails_ReturnsFail(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return "", fmt.Errorf("api error")
	}

	result := RepairStaleOpenFeatures("owner/repo", fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v: %s", result.Status, result.Message)
	}
}

// ── layoutToREST ──────────────────────────────────────────────────────────────

func TestLayoutToREST_KnownLayouts(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"BOARD_LAYOUT", "board"},
		{"TABLE_LAYOUT", "table"},
		{"ROADMAP_LAYOUT", "roadmap"},
	}
	for _, tc := range tests {
		got := layoutToREST(tc.input)
		if got != tc.expected {
			t.Errorf("layoutToREST(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestLayoutToREST_DefaultFallback(t *testing.T) {
	got := layoutToREST("CUSTOM_LAYOUT")
	if got != "custom" {
		t.Errorf("layoutToREST(\"CUSTOM_LAYOUT\") = %q, want \"custom\"", got)
	}
}

// ── RepairProjectViews ────────────────────────────────────────────────────────

// projectListJSONWithOwner includes owner.type, required by resolveProjectEntry.
const projectListJSONWithOwner = `{"projects":[{"id":"PVT_123","title":"my-repo","number":1,"owner":{"login":"owner","type":"User"}}]}`

// writeProjectTemplateWithViews writes base/project-template.json including requiredViews.
func writeProjectTemplateWithViews(t *testing.T, root string) {
	t.Helper()
	baseDir := filepath.Join(root, "base")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `{
  "statusOptions": [{"name":"Backlog","color":"GRAY","description":""}],
  "requiredViews": [
    {"name":"Board","layout":"BOARD_LAYOUT","filter":""},
    {"name":"Table","layout":"TABLE_LAYOUT","filter":"is:open"}
  ]
}`
	if err := os.WriteFile(filepath.Join(baseDir, "project-template.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRepairProjectViews_NoProject_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	writeProjectTemplateWithViews(t, root)
	fakeRun := func(name string, args ...string) (string, error) {
		return "", fmt.Errorf("no project")
	}

	result := RepairProjectViews("owner", "my-repo", root, fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "no GitHub Project found") {
		t.Errorf("expected 'no GitHub Project found', got: %s", result.Message)
	}
}

func TestRepairProjectViews_LoadTemplateFails_ReturnsFail(t *testing.T) {
	root := t.TempDir() // no project-template.json
	fakeRun := func(name string, args ...string) (string, error) {
		return projectListJSONWithOwner, nil
	}

	result := RepairProjectViews("owner", "my-repo", root, fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "project template") {
		t.Errorf("expected 'project template' in message, got: %s", result.Message)
	}
}

func TestRepairProjectViews_FetchViewsFails_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	writeProjectTemplateWithViews(t, root)
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			return projectListJSONWithOwner, nil // resolve project entry
		}
		return "", fmt.Errorf("graphql error") // fetch existing views
	}

	result := RepairProjectViews("owner", "my-repo", root, fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "failed to fetch views") {
		t.Errorf("expected 'failed to fetch views', got: %s", result.Message)
	}
}

func TestRepairProjectViews_AllPresent_ReturnsPass(t *testing.T) {
	root := t.TempDir()
	writeProjectTemplateWithViews(t, root)
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			return projectListJSONWithOwner, nil
		}
		return "Board\nTable\n", nil // both required views present
	}

	result := RepairProjectViews("owner", "my-repo", root, fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass when all views present, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "all required views present") {
		t.Errorf("expected 'all required views present', got: %s", result.Message)
	}
}

func TestRepairProjectViews_CreateFails_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	writeProjectTemplateWithViews(t, root)
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			return projectListJSONWithOwner, nil
		}
		if callCount == 2 {
			return "", nil // fetch views — none present
		}
		return "", fmt.Errorf("create view: insufficient permissions")
	}

	result := RepairProjectViews("owner", "my-repo", root, fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail when create fails, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "failed to create view") {
		t.Errorf("expected 'failed to create view', got: %s", result.Message)
	}
}

func TestRepairProjectViews_CreatesNewViews_ReturnsPass(t *testing.T) {
	root := t.TempDir()
	writeProjectTemplateWithViews(t, root)
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			return projectListJSONWithOwner, nil
		}
		if callCount == 2 {
			return "", nil // fetch views — none present
		}
		return `{"id":"view_1"}`, nil // create Board, then Table — both succeed
	}

	result := RepairProjectViews("owner", "my-repo", root, fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass when views created, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "created views") {
		t.Errorf("expected 'created views' in message, got: %s", result.Message)
	}
}
