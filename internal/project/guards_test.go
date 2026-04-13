package project

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// --- HasDocsContent ---

func TestHasDocsContent_EmptyDir(t *testing.T) {
	tmp := t.TempDir()
	docsDir := filepath.Join(tmp, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if HasDocsContent(tmp) {
		t.Error("expected false for empty docs/")
	}
}

func TestHasDocsContent_WithFile(t *testing.T) {
	tmp := t.TempDir()
	docsDir := filepath.Join(tmp, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, "readme.md"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !HasDocsContent(tmp) {
		t.Error("expected true when docs/ has a file")
	}
}

func TestHasDocsContent_NoDocsDir(t *testing.T) {
	tmp := t.TempDir()
	if HasDocsContent(tmp) {
		t.Error("expected false when docs/ does not exist")
	}
}

func TestHasDocsContent_WithSubdirAndFile(t *testing.T) {
	tmp := t.TempDir()
	subDir := filepath.Join(tmp, "docs", "sub")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "file.md"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !HasDocsContent(tmp) {
		t.Error("expected true when docs/sub/ has a file")
	}
}

// --- EvalJoinGuard ---

func TestEvalJoinGuard_Clear_NoCurrentID(t *testing.T) {
	deps := testDeps("owner", "repo")
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		return "", errors.New("not set")
	}

	result, err := EvalJoinGuard(deps, "PVT_new")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Guard != JoinGuardClear {
		t.Errorf("expected JoinGuardClear, got %v", result.Guard)
	}
}

func TestEvalJoinGuard_SameProject(t *testing.T) {
	deps := testDeps("owner", "repo")
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		return "PVT_existing", nil
	}

	result, err := EvalJoinGuard(deps, "PVT_existing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Guard != JoinGuardSameProject {
		t.Errorf("expected JoinGuardSameProject, got %v", result.Guard)
	}
}

func TestEvalJoinGuard_WarnConfirm_Federated(t *testing.T) {
	deps := testDeps("owner", "repo")
	// Current affiliation is a different project, topology is Federated.
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		return "PVT_old", nil
	}
	deps.FetchLinkedRepos = func(projectID string) ([]LinkedRepo, error) {
		// owner/repo is NOT among linked repos → Federated.
		return []LinkedRepo{{Name: "control-plane", NameWithOwner: "owner/control-plane"}}, nil
	}

	result, err := EvalJoinGuard(deps, "PVT_new")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Guard != JoinGuardWarnConfirm {
		t.Errorf("expected JoinGuardWarnConfirm, got %v", result.Guard)
	}
}

func TestEvalJoinGuard_Blocked_SingleWithDocsContent(t *testing.T) {
	tmp := t.TempDir()
	docsDir := filepath.Join(tmp, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, "readme.md"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	deps := testDeps("owner", "repo")
	deps.Root = tmp
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		return "PVT_old", nil
	}
	// Topology is Single: owner/repo IS among linked repos.
	deps.FetchLinkedRepos = func(projectID string) ([]LinkedRepo, error) {
		return []LinkedRepo{{Name: "repo", NameWithOwner: "owner/repo"}}, nil
	}

	result, err := EvalJoinGuard(deps, "PVT_new")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Guard != JoinGuardBlocked {
		t.Errorf("expected JoinGuardBlocked, got %v", result.Guard)
	}
}

func TestEvalJoinGuard_WarnConfirm_SingleNoDocsContent(t *testing.T) {
	tmp := t.TempDir()
	// No docs/ content.

	deps := testDeps("owner", "repo")
	deps.Root = tmp
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		return "PVT_old", nil
	}
	// Topology is Single.
	deps.FetchLinkedRepos = func(projectID string) ([]LinkedRepo, error) {
		return []LinkedRepo{{Name: "repo", NameWithOwner: "owner/repo"}}, nil
	}

	result, err := EvalJoinGuard(deps, "PVT_new")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Guard != JoinGuardWarnConfirm {
		t.Errorf("expected JoinGuardWarnConfirm, got %v", result.Guard)
	}
}

// --- EvalUnlinkGuard ---

func TestEvalUnlinkGuard_Clear_NoCurrentID(t *testing.T) {
	deps := testDeps("owner", "repo")
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		return "", errors.New("not set")
	}

	result, err := EvalUnlinkGuard(deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Guard != JoinGuardClear {
		t.Errorf("expected JoinGuardClear, got %v", result.Guard)
	}
}

func TestEvalUnlinkGuard_WarnConfirm_Federated(t *testing.T) {
	deps := testDeps("owner", "repo")
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		return "PVT_existing", nil
	}
	deps.FetchLinkedRepos = func(projectID string) ([]LinkedRepo, error) {
		return []LinkedRepo{{Name: "control-plane", NameWithOwner: "owner/control-plane"}}, nil
	}

	result, err := EvalUnlinkGuard(deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Guard != JoinGuardWarnConfirm {
		t.Errorf("expected JoinGuardWarnConfirm, got %v", result.Guard)
	}
}

func TestEvalUnlinkGuard_Blocked_SingleWithDocsContent(t *testing.T) {
	tmp := t.TempDir()
	docsDir := filepath.Join(tmp, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, "content.md"), []byte("docs"), 0o644); err != nil {
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

	result, err := EvalUnlinkGuard(deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Guard != JoinGuardBlocked {
		t.Errorf("expected JoinGuardBlocked, got %v", result.Guard)
	}
}

func TestEvalUnlinkGuard_WarnConfirm_SingleNoDocsContent(t *testing.T) {
	tmp := t.TempDir()

	deps := testDeps("owner", "repo")
	deps.Root = tmp
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		return "PVT_existing", nil
	}
	deps.FetchLinkedRepos = func(projectID string) ([]LinkedRepo, error) {
		return []LinkedRepo{{Name: "repo", NameWithOwner: "owner/repo"}}, nil
	}

	result, err := EvalUnlinkGuard(deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Guard != JoinGuardWarnConfirm {
		t.Errorf("expected JoinGuardWarnConfirm, got %v", result.Guard)
	}
}
