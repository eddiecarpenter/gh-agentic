package verify

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── repairOrgMembership — null/empty user ID path (lines 716-719) ─────────────

func TestRepairProjectCollaborator_Org_NullUserID_ReturnsFail(t *testing.T) {
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			return "", fmt.Errorf("HTTP 404") // not a member
		}
		return "null\n", nil // user ID is null → user not found
	}

	result := RepairProjectCollaborator("myorg", "repo", "bot-user", "Organization", fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail for null user ID, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "not found on GitHub") {
		t.Errorf("expected 'not found on GitHub', got: %s", result.Message)
	}
}

// ── repairProjectCollaboratorUser — no project (lines 757-760) ────────────────

func TestRepairProjectCollaborator_User_NoProject_ReturnsFail(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return "", fmt.Errorf("no project found")
	}

	result := RepairProjectCollaborator("owner", "repo", "bot-user", "User", fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail when no project, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "no GitHub Project found") {
		t.Errorf("expected 'no GitHub Project found', got: %s", result.Message)
	}
}

// ── repairProjectCollaboratorUser — null user ID (lines 777-780) ──────────────

func TestRepairProjectCollaborator_User_NullUserID_ReturnsFail(t *testing.T) {
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			return projectListJSON, nil // project found
		}
		return "null\n", nil // user node ID is null
	}

	result := RepairProjectCollaborator("owner", "repo", "bot-user", "User", fakeRun)
	if result.Status != Fail {
		t.Errorf("expected Fail for null user ID, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "not found on GitHub") {
		t.Errorf("expected 'not found on GitHub', got: %s", result.Message)
	}
}

// ── RepairAIDirWithWriter — confirm declined / error (lines 820-825) ──────────

func TestRepairAIDir_ConfirmDeclined_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	fakeRun := func(name string, args ...string) (string, error) { return "", nil }

	result := RepairAIDir(root, fakeRun, func(string) (bool, error) { return false, nil })
	if result.Status != Fail {
		t.Errorf("expected Fail when user declines, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "user declined re-sync") {
		t.Errorf("expected 'user declined re-sync', got: %s", result.Message)
	}
}

func TestRepairAIDir_ConfirmError_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	fakeRun := func(name string, args ...string) (string, error) { return "", nil }

	result := RepairAIDir(root, fakeRun, func(string) (bool, error) {
		return false, fmt.Errorf("TTY error")
	})
	if result.Status != Fail {
		t.Errorf("expected Fail when confirm errors, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "user declined re-sync") {
		t.Errorf("expected 'user declined re-sync', got: %s", result.Message)
	}
}

// ── RepairWorkflows — MkdirAll fails (lines 937-940) ─────────────────────────

func TestRepairWorkflows_MkdirAllFails_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	// Block .github/ creation by placing a regular file at that path.
	if err := os.WriteFile(filepath.Join(root, ".github"), []byte("blocker"), 0o644); err != nil {
		t.Fatal(err)
	}
	fakeRun := func(name string, args ...string) (string, error) { return "", nil }

	result := RepairWorkflows(root, "User", fakeRun, failingFetchFunc("should not reach"))
	if result.Status != Fail {
		t.Errorf("expected Fail when MkdirAll blocked, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "could not create directory") {
		t.Errorf("expected 'could not create directory', got: %s", result.Message)
	}
}

// ── RepairWorkflows — .ai dir present, WriteFile fails (lines 974-977) ────────

func TestRepairWorkflows_AIDir_WriteFileFails_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	// Create .ai/.github/workflows/ with a source file.
	aiWorkflowsDir := filepath.Join(root, ".ai", ".github", "workflows")
	if err := os.MkdirAll(aiWorkflowsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(aiWorkflowsDir, "build-and-test.yml"), []byte("# test"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create destination dir, then block the destination file write.
	destDir := filepath.Join(root, ".github", "workflows")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFailureDir(t, filepath.Join(destDir, "build-and-test.yml"))

	fakeRun := func(name string, args ...string) (string, error) { return "", nil }

	result := RepairWorkflows(root, "User", fakeRun, failingFetchFunc("should not reach"))
	if result.Status != Fail {
		t.Errorf("expected Fail when WriteFile blocked, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "writing") {
		t.Errorf("expected 'writing' in message, got: %s", result.Message)
	}
}

// ── RepairWorkflows — .ai dir present, git add fails (lines 986-989) ─────────

func TestRepairWorkflows_AIDir_GitAddFails_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	aiWorkflowsDir := filepath.Join(root, ".ai", ".github", "workflows")
	if err := os.MkdirAll(aiWorkflowsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(aiWorkflowsDir, "build-and-test.yml"), []byte("# test"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".github", "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}

	fakeRun := func(name string, args ...string) (string, error) {
		return "", fmt.Errorf("git add: not a git repository")
	}

	result := RepairWorkflows(root, "User", fakeRun, failingFetchFunc("should not reach"))
	if result.Status != Fail {
		t.Errorf("expected Fail when git add fails, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "git add failed") {
		t.Errorf("expected 'git add failed', got: %s", result.Message)
	}
}

// ── RepairWorkflows fallback — tarball fails (lines 1022-1025) ────────────────

func TestRepairWorkflows_Fallback_TarballExtractFails_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	writeTemplateConfig(t, root, "owner/template", "v1.0.0")
	fakeRun := func(name string, args ...string) (string, error) { return "", nil }

	result := RepairWorkflows(root, "User", fakeRun, failingFetchFunc("tarball unavailable"))
	if result.Status != Fail {
		t.Errorf("expected Fail when tarball fails, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "tarball extraction failed") {
		t.Errorf("expected 'tarball extraction failed', got: %s", result.Message)
	}
}

// ── RepairAgentUserVar — prompt and org-scope error paths ────────────────────
// (lines 1293, 1305, 1320-1321, 1327-1328, 1331-1332)

func TestRepairAgentUserVar_PromptUserFails_ReturnsFail(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) { return "", nil }

	result := RepairAgentUserVar("owner", "repo", "", "repo", fakeRun,
		func(string) (string, error) { return "", fmt.Errorf("TTY unavailable") })
	if result.Status != Fail {
		t.Errorf("expected Fail when user prompt errors, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "prompt failed") {
		t.Errorf("expected 'prompt failed', got: %s", result.Message)
	}
}

func TestRepairAgentUserVar_PromptScopeFails_ReturnsFail(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) { return "", nil }

	result := RepairAgentUserVar("owner", "repo", "bot-user", "", fakeRun,
		func(string) (string, error) { return "", fmt.Errorf("TTY unavailable") })
	if result.Status != Fail {
		t.Errorf("expected Fail when scope prompt errors, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "prompt failed") {
		t.Errorf("expected 'prompt failed', got: %s", result.Message)
	}
}

func TestRepairAgentUserVar_OrgScope_OwnerAPIFails_ReturnsFail(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return "", fmt.Errorf("api: unauthorized")
	}

	result := RepairAgentUserVar("owner", "repo", "bot-user", "org", fakeRun, nil)
	if result.Status != Fail {
		t.Errorf("expected Fail when owner API fails, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "failed to detect owner type") {
		t.Errorf("expected 'failed to detect owner type', got: %s", result.Message)
	}
}

func TestRepairAgentUserVar_OrgScope_JSONParseFails_ReturnsFail(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return "not-valid-json", nil
	}

	result := RepairAgentUserVar("owner", "repo", "bot-user", "org", fakeRun, nil)
	if result.Status != Fail {
		t.Errorf("expected Fail when JSON parse fails, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "failed to parse owner type") {
		t.Errorf("expected 'failed to parse owner type', got: %s", result.Message)
	}
}

func TestRepairAgentUserVar_OrgScope_OwnerIsUser_ReturnsFail(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return `{"type":"User"}`, nil
	}

	result := RepairAgentUserVar("owner", "repo", "bot-user", "org", fakeRun, nil)
	if result.Status != Fail {
		t.Errorf("expected Fail when owner is not an org, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "personal account") {
		t.Errorf("expected 'personal account', got: %s", result.Message)
	}
}

// ── resyncProjectItemStatuses — fetchAllProjectItems fails (line 449) ─────────

func TestResyncProjectItemStatuses_FetchAllItemsFails_ReturnsError(t *testing.T) {
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			return projectListJSON, nil // project list for ResyncProjectItemStatuses
		}
		if callCount == 2 {
			return "FIELD_456\n", nil // Status field ID
		}
		if callCount == 3 {
			return "OPT_1|Backlog\n", nil // fetchStatusOptionMap succeeds
		}
		return "", fmt.Errorf("items: server error") // fetchAllProjectItems fails
	}

	_, _, err := ResyncProjectItemStatuses("owner", "repo", fakeRun)
	if err == nil {
		t.Fatal("expected error when fetchAllProjectItems fails, got nil")
	}
	if !strings.Contains(err.Error(), "fetching project items") {
		t.Errorf("expected 'fetching project items' in error, got: %v", err)
	}
}
