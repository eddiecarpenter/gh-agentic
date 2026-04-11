package verify

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
)

// ── CheckAIConfigYML — read error (not-found vs other error) ──────────────────

func TestCheckAIConfigYML_ReadError_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	// Create .ai/ then place a directory at config.yml so ReadFile fails with
	// "is a directory" — a different error from os.ErrNotExist.
	if err := os.MkdirAll(filepath.Join(root, ".ai"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".ai", "config.yml"), 0o755); err != nil {
		t.Fatal(err)
	}

	result := CheckAIConfigYML(root)
	if result.Status != Fail {
		t.Errorf("expected Fail for read error, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "could not read file") {
		t.Errorf("expected 'could not read file' in message, got: %s", result.Message)
	}
}

// ── CheckProjectViews ─────────────────────────────────────────────────────────

func TestCheckProjectViews_NoProject_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	writeTestProjectTemplate(t, root)
	fakeRun := func(name string, args ...string) (string, error) {
		return "", fmt.Errorf("no project")
	}

	result := CheckProjectViews("owner", "my-repo", root, fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "no GitHub Project found") {
		t.Errorf("expected 'no GitHub Project found', got: %s", result.Message)
	}
}

func TestCheckProjectViews_FetchViewsFails_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	writeTestProjectTemplate(t, root)
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			return projectListJSON, nil // resolve project node ID
		}
		return "", fmt.Errorf("graphql error") // fetch views query fails
	}

	result := CheckProjectViews("owner", "my-repo", root, fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "failed to fetch views") {
		t.Errorf("expected 'failed to fetch views', got: %s", result.Message)
	}
}

func TestCheckProjectViews_LoadTemplateFails_ReturnsFail(t *testing.T) {
	root := t.TempDir() // no project-template.json
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			return projectListJSON, nil // project resolved
		}
		return "SomeView\n", nil // views fetched OK
	}

	result := CheckProjectViews("owner", "my-repo", root, fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "project template") {
		t.Errorf("expected 'project template' in message, got: %s", result.Message)
	}
}

// writeProjectTemplateRequiringBoard writes a minimal project-template.json
// that requires a single "Board" view.
func writeProjectTemplateRequiringBoard(t *testing.T, root string) {
	t.Helper()
	baseDir := filepath.Join(root, "base")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `{"statusOptions":[],"requiredViews":[{"name":"Board","layout":"BOARD_LAYOUT","filter":""}]}`
	if err := os.WriteFile(filepath.Join(baseDir, "project-template.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCheckProjectViews_MissingViews_ReturnsWarning(t *testing.T) {
	root := t.TempDir()
	writeProjectTemplateRequiringBoard(t, root)
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			return projectListJSON, nil
		}
		return "", nil // no views present
	}

	result := CheckProjectViews("owner", "my-repo", root, fakeRun)
	if result.Status != Warning {
		t.Errorf("expected Warning for missing views, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "missing") {
		t.Errorf("expected 'missing' in message, got: %s", result.Message)
	}
}

func TestCheckProjectViews_AllPresent_ReturnsPass(t *testing.T) {
	root := t.TempDir()
	writeProjectTemplateRequiringBoard(t, root)
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			return projectListJSON, nil
		}
		return "Board\n", nil // required view is present
	}

	result := CheckProjectViews("owner", "my-repo", root, fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass when all views present, got %v: %s", result.Status, result.Message)
	}
}

// ── CheckStaleOpenRequirements ────────────────────────────────────────────────

func TestCheckStaleOpenRequirements_FetchFails_ReturnsFail(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return "", fmt.Errorf("api error")
	}

	result := CheckStaleOpenRequirements("owner/repo", fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckStaleOpenRequirements_NoStale_ReturnsPass(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return "[]", nil
	}

	result := CheckStaleOpenRequirements("owner/repo", fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass for no stale issues, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckStaleOpenRequirements_HasStale_ReturnsWarning(t *testing.T) {
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		switch callCount {
		case 1:
			return `[{"number":5,"title":"Stale Req"}]`, nil
		case 2:
			return `{"subIssues":[{"state":"CLOSED"}]}`, nil
		}
		return "[]", nil
	}

	result := CheckStaleOpenRequirements("owner/repo", fakeRun)
	if result.Status != Warning {
		t.Errorf("expected Warning for stale requirement, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "#5") {
		t.Errorf("expected issue number in message, got: %s", result.Message)
	}
}

// ── CheckStaleOpenFeatures ────────────────────────────────────────────────────

func TestCheckStaleOpenFeatures_FetchFails_ReturnsFail(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return "", fmt.Errorf("api error")
	}

	result := CheckStaleOpenFeatures("owner/repo", fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckStaleOpenFeatures_NoStale_ReturnsPass(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return "[]", nil
	}

	result := CheckStaleOpenFeatures("owner/repo", fakeRun)
	if result.Status != Pass {
		t.Errorf("expected Pass for no stale features, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckStaleOpenFeatures_HasStale_ReturnsWarning(t *testing.T) {
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		switch callCount {
		case 1:
			return `[{"number":10,"title":"Stale Feature"}]`, nil
		case 2:
			return `{"subIssues":[{"state":"CLOSED"},{"state":"CLOSED"}]}`, nil
		}
		return "[]", nil
	}

	result := CheckStaleOpenFeatures("owner/repo", fakeRun)
	if result.Status != Warning {
		t.Errorf("expected Warning for stale feature, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "#10") {
		t.Errorf("expected issue number in message, got: %s", result.Message)
	}
}

// ── checkRepoVariable error paths ─────────────────────────────────────────────

func TestCheckRepoVariable_Org_ListFails_ReturnsWarning(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return "", fmt.Errorf("org variable list failed")
	}

	result := checkRepoVariable("owner", "my-repo", "RUNNER_LABEL", "runner label", "ubuntu-latest", bootstrap.OwnerTypeOrg, fakeRun)
	if result.Status != Warning {
		t.Errorf("expected Warning when org list fails, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "could not list org variables") {
		t.Errorf("expected 'could not list org variables', got: %s", result.Message)
	}
}

func TestCheckRepoVariable_Org_RepoLevelCheckFails_ReturnsWarning(t *testing.T) {
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			return "[]", nil // org level: variable not found
		}
		return "", fmt.Errorf("repo variable list failed") // repo level check fails
	}

	result := checkRepoVariable("owner", "my-repo", "RUNNER_LABEL", "runner label", "ubuntu-latest", bootstrap.OwnerTypeOrg, fakeRun)
	if result.Status != Warning {
		t.Errorf("expected Warning when repo level check fails, got %v: %s", result.Status, result.Message)
	}
}

// ── checkRepoSecret error paths ───────────────────────────────────────────────

func TestCheckRepoSecret_Org_ListFails_ReturnsWarning(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return "", fmt.Errorf("org secret list failed")
	}

	result := checkRepoSecret("owner", "my-repo", "GOOSE_AGENT_PAT", "pat check", bootstrap.OwnerTypeOrg, fakeRun)
	if result.Status != Warning {
		t.Errorf("expected Warning when org secret list fails, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "could not list org secrets") {
		t.Errorf("expected 'could not list org secrets', got: %s", result.Message)
	}
}

func TestCheckRepoSecret_Org_RepoLevelCheckFails_ReturnsWarning(t *testing.T) {
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			return "[]", nil // org level: secret not found
		}
		return "", fmt.Errorf("repo secret list failed") // repo level check fails
	}

	result := checkRepoSecret("owner", "my-repo", "GOOSE_AGENT_PAT", "pat check", bootstrap.OwnerTypeOrg, fakeRun)
	if result.Status != Warning {
		t.Errorf("expected Warning when repo level check fails, got %v: %s", result.Status, result.Message)
	}
}
