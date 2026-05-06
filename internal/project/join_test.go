package project

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Join ---

func TestJoin_Clear_SetsVariable(t *testing.T) {
	tmp := t.TempDir()
	withFakeInstall(t)

	var setVar string
	deps := testDeps("owner", "repo")
	deps.Root = tmp
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		return "", errors.New("not set")
	}
	deps.SetRepoVariable = func(o, r, n, v string) error {
		if n == ProjectVarName {
			setVar = v
		}
		return nil
	}
	deps.Clone = func(repoURL, tag, destDir string) error { return nil }

	var buf bytes.Buffer
	if err := Join(&buf, deps, "PVT_target"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if setVar != "PVT_target" {
		t.Errorf("expected AGENTIC_PROJECT_ID set to PVT_target, got %q", setVar)
	}

	// .agents/ should now exist via the install stub.
	if _, err := os.Stat(filepath.Join(tmp, ".agents", "RULEBOOK.md")); err != nil {
		t.Errorf("expected .agents/RULEBOOK.md to exist after install: %v", err)
	}
}

func TestJoin_SameProject_NoOp(t *testing.T) {
	deps := testDeps("owner", "repo")
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		return "PVT_same", nil
	}

	var setCalled bool
	deps.SetRepoVariable = func(o, r, n, v string) error {
		setCalled = true
		return nil
	}

	var buf bytes.Buffer
	if err := Join(&buf, deps, "PVT_same"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if setCalled {
		t.Error("expected SetRepoVariable NOT to be called for same project")
	}
}

func TestJoin_WarnConfirm_Confirmed(t *testing.T) {
	tmp := t.TempDir()
	withFakeInstall(t)

	deps := testDeps("owner", "repo")
	deps.Root = tmp
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		return "PVT_old", nil
	}
	// Federated → WarnConfirm.
	deps.FetchLinkedRepos = func(projectID string) ([]LinkedRepo, error) {
		return []LinkedRepo{{Name: "other", NameWithOwner: "owner/other"}}, nil
	}
	deps.Confirm = func(prompt string) (bool, error) { return true, nil }

	var setVar string
	deps.SetRepoVariable = func(o, r, n, v string) error {
		setVar = v
		return nil
	}
	deps.Clone = func(repoURL, tag, destDir string) error { return nil }

	var buf bytes.Buffer
	if err := Join(&buf, deps, "PVT_new"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if setVar != "PVT_new" {
		t.Errorf("expected AGENTIC_PROJECT_ID set to PVT_new, got %q", setVar)
	}
}

func TestJoin_WarnConfirm_Denied(t *testing.T) {
	deps := testDeps("owner", "repo")
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		return "PVT_old", nil
	}
	deps.FetchLinkedRepos = func(projectID string) ([]LinkedRepo, error) {
		return []LinkedRepo{{Name: "other", NameWithOwner: "owner/other"}}, nil
	}
	deps.Confirm = func(prompt string) (bool, error) { return false, nil }

	var buf bytes.Buffer
	err := Join(&buf, deps, "PVT_new")
	if err == nil {
		t.Fatal("expected error when user denies confirmation")
	}
	if !strings.Contains(err.Error(), "aborted") {
		t.Errorf("expected 'aborted' in error, got: %v", err)
	}
}

func TestJoin_Blocked(t *testing.T) {
	tmp := t.TempDir()
	docsDir := tmp + "/docs"
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(docsDir+"/readme.md", []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	deps := testDeps("owner", "repo")
	deps.Root = tmp
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		return "PVT_old", nil
	}
	// Single topology + docs content → Blocked.
	deps.FetchLinkedRepos = func(projectID string) ([]LinkedRepo, error) {
		return []LinkedRepo{{Name: "repo", NameWithOwner: "owner/repo"}}, nil
	}

	var buf bytes.Buffer
	err := Join(&buf, deps, "PVT_new")
	if err == nil {
		t.Fatal("expected error when join is blocked")
	}
	if !strings.Contains(err.Error(), "blocked") {
		t.Errorf("expected 'blocked' in error, got: %v", err)
	}
}

func TestJoin_FrameworkAlreadyMounted(t *testing.T) {
	tmp := t.TempDir()
	// Create .agents/ to simulate already mounted.
	aiDir := tmp + "/.ai"
	if err := os.MkdirAll(aiDir, 0o755); err != nil {
		t.Fatal(err)
	}

	deps := testDeps("owner", "repo")
	deps.Root = tmp
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		return "", errors.New("not set")
	}
	deps.SetRepoVariable = func(o, r, n, v string) error { return nil }
	deps.ReadAIVersion = func(root string) (string, error) { return "v2.0.10", nil }

	var cloneCalled bool
	deps.Clone = func(repoURL, tag, destDir string) error {
		cloneCalled = true
		return nil
	}

	var buf bytes.Buffer
	if err := Join(&buf, deps, "PVT_target"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cloneCalled {
		t.Error("expected Clone NOT to be called when .agents/ already exists")
	}
}

// --- Unlink ---

func TestUnlink_Clear_NothingToUnlink(t *testing.T) {
	deps := testDeps("owner", "repo")
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		return "", errors.New("not set")
	}

	var deleteCalled bool
	deps.DeleteRepoVariable = func(o, r, n string) error {
		deleteCalled = true
		return nil
	}

	var buf bytes.Buffer
	if err := Unlink(&buf, deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleteCalled {
		t.Error("expected DeleteRepoVariable NOT to be called when nothing to unlink")
	}
	if !strings.Contains(buf.String(), "nothing to unlink") {
		t.Errorf("expected 'nothing to unlink' in output, got:\n%s", buf.String())
	}
}

func TestUnlink_WarnConfirm_Confirmed(t *testing.T) {
	deps := testDeps("owner", "repo")
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		return "PVT_existing", nil
	}
	deps.FetchLinkedRepos = func(projectID string) ([]LinkedRepo, error) {
		return []LinkedRepo{{Name: "other", NameWithOwner: "owner/other"}}, nil
	}
	deps.Confirm = func(prompt string) (bool, error) { return true, nil }

	var deleteCalled bool
	deps.DeleteRepoVariable = func(o, r, n string) error {
		deleteCalled = true
		return nil
	}

	var buf bytes.Buffer
	if err := Unlink(&buf, deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deleteCalled {
		t.Error("expected DeleteRepoVariable to be called")
	}
}

func TestUnlink_WarnConfirm_Denied(t *testing.T) {
	deps := testDeps("owner", "repo")
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		return "PVT_existing", nil
	}
	deps.FetchLinkedRepos = func(projectID string) ([]LinkedRepo, error) {
		return []LinkedRepo{{Name: "other", NameWithOwner: "owner/other"}}, nil
	}
	deps.Confirm = func(prompt string) (bool, error) { return false, nil }

	var buf bytes.Buffer
	err := Unlink(&buf, deps)
	if err == nil {
		t.Fatal("expected error when user denies")
	}
	if !strings.Contains(err.Error(), "aborted") {
		t.Errorf("expected 'aborted' in error, got: %v", err)
	}
}

func TestUnlink_Blocked_SingleWithDocsContent(t *testing.T) {
	tmp := t.TempDir()
	docsDir := tmp + "/docs"
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(docsDir+"/readme.md", []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	deps := testDeps("owner", "repo")
	deps.Root = tmp
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		return "PVT_existing", nil
	}
	deps.FetchLinkedRepos = func(projectID string) ([]LinkedRepo, error) {
		return []LinkedRepo{{Name: "repo", NameWithOwner: "owner/repo"}}, nil
	}

	var buf bytes.Buffer
	err := Unlink(&buf, deps)
	if err == nil {
		t.Fatal("expected error when unlink is blocked")
	}
	if !strings.Contains(err.Error(), "blocked") {
		t.Errorf("expected 'blocked' in error, got: %v", err)
	}
}
