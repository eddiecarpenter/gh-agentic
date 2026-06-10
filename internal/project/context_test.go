package project

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// resolveDeps assembles a project.Deps with just the fields Resolve reads,
// wiring the provided variable store and project-title fixture. Everything
// Resolve does not touch stays at the zero value.
func resolveDeps(owner, repo, root string, store *variableStore, title string, titleErr error, aiVersion string, aiErr error) Deps {
	return Deps{
		Owner:        owner,
		RepoName:     repo,
		RepoFullName: owner + "/" + repo,
		Root:         root,
		GetRepoVariable: func(o, r, name string) (string, error) {
			return store.get(o, r, name)
		},
		FetchProjectTitle: func(projectID string) (string, error) {
			return title, titleErr
		},
		ReadAIVersion: func(root string) (string, error) {
			return aiVersion, aiErr
		},
	}
}

// TestResolve_NoFederationMD_ReturnsSingleTopology verifies that the absence
// of FEDERATION.md produces topology="single".
func TestResolve_NoFederationMD_ReturnsSingleTopology(t *testing.T) {
	dir := t.TempDir()
	store := newVariableStore(map[string]string{
		ProjectVarName: "PVT_solo",
	})

	deps := resolveDeps("user", "solo", dir, store, "Solo", nil, "v2.1.0", nil)

	ctx, err := Resolve(deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.Topology != TopologyStringSingle {
		t.Errorf("Topology: got %q, want %q", ctx.Topology, TopologyStringSingle)
	}
	if ctx.ProjectID != "PVT_solo" {
		t.Errorf("ProjectID: got %q, want %q", ctx.ProjectID, "PVT_solo")
	}
	if ctx.ProjectName != "Solo" {
		t.Errorf("ProjectName: got %q, want %q", ctx.ProjectName, "Solo")
	}
}

// TestResolve_FederationMDPresent_ReturnsFederationTopology verifies that
// FEDERATION.md presence at deps.Root produces topology="federation".
func TestResolve_FederationMDPresent_ReturnsFederationTopology(t *testing.T) {
	dir := t.TempDir()
	content := `repos:
  - name: org/domain-one
    purpose: "First domain"
`
	if err := os.WriteFile(filepath.Join(dir, federationFileName), []byte(content), 0644); err != nil {
		t.Fatalf("writing FEDERATION.md: %v", err)
	}

	store := newVariableStore(map[string]string{
		ProjectVarName: "PVT_fed",
	})

	deps := resolveDeps("org", "control-plane", dir, store, "Federation", nil, "v2.1.0", nil)

	ctx, err := Resolve(deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.Topology != TopologyStringFederation {
		t.Errorf("Topology: got %q, want %q", ctx.Topology, TopologyStringFederation)
	}
	if ctx.ProjectID != "PVT_fed" {
		t.Errorf("ProjectID: got %q, want %q", ctx.ProjectID, "PVT_fed")
	}
}

// TestResolve_NoProjectAffiliation_ReturnsSingleContext verifies that an
// unaffiliated repo (no AGENTIC_PROJECT_ID) resolves without error and
// FetchProjectTitle is never called.
func TestResolve_NoProjectAffiliation_ReturnsSingleContext(t *testing.T) {
	dir := t.TempDir()
	store := newVariableStore(map[string]string{})

	deps := Deps{
		Owner:        "user",
		RepoName:     "stray",
		RepoFullName: "user/stray",
		Root:         dir,
		GetRepoVariable: func(o, r, name string) (string, error) {
			return store.get(o, r, name)
		},
		FetchProjectTitle: func(projectID string) (string, error) {
			t.Fatalf("FetchProjectTitle must not be called when ProjectID is empty")
			return "", nil
		},
		ReadAIVersion: func(root string) (string, error) { return "", nil },
	}

	ctx, err := Resolve(deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.ProjectID != "" {
		t.Errorf("ProjectID: got %q, want empty", ctx.ProjectID)
	}
	if ctx.Topology != TopologyStringSingle {
		t.Errorf("Topology: got %q, want %q", ctx.Topology, TopologyStringSingle)
	}
}

// TestResolve_DeletedProject_FlagsAsDeleted verifies that when the project
// title cannot be fetched, ProjectDeleted is true and ProjectName falls back
// to the ID.
func TestResolve_DeletedProject_FlagsAsDeleted(t *testing.T) {
	dir := t.TempDir()
	store := newVariableStore(map[string]string{
		ProjectVarName: "PVT_ghost",
	})

	deps := Deps{
		Owner:        "org",
		RepoName:     "ex-project",
		RepoFullName: "org/ex-project",
		Root:         dir,
		GetRepoVariable: func(o, r, name string) (string, error) {
			return store.get(o, r, name)
		},
		FetchProjectTitle: func(projectID string) (string, error) {
			return "", nil // deleted — title is empty
		},
		ReadAIVersion: func(root string) (string, error) { return "v2.0.0", nil },
	}

	ctx, err := Resolve(deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ctx.ProjectDeleted {
		t.Error("ProjectDeleted: got false, want true")
	}
	if ctx.ProjectName != "PVT_ghost" {
		t.Errorf("ProjectName: got %q, want fallback to ID %q", ctx.ProjectName, "PVT_ghost")
	}
}

// TestResolve_VersionInSync_WhenLocalMatchesRemote verifies that VersionInSync
// is true when LocalAIVersion equals FrameworkVersion.
func TestResolve_VersionInSync_WhenLocalMatchesRemote(t *testing.T) {
	dir := t.TempDir()
	store := newVariableStore(map[string]string{
		ProjectVarName:          "PVT_v",
		FrameworkVersionVarName: "v2.1.0",
	})

	deps := resolveDeps("org", "repo", dir, store, "MyProject", nil, "v2.1.0", nil)

	ctx, err := Resolve(deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.FrameworkVersion != "v2.1.0" {
		t.Errorf("FrameworkVersion: got %q, want v2.1.0", ctx.FrameworkVersion)
	}
	if ctx.LocalAIVersion != "v2.1.0" {
		t.Errorf("LocalAIVersion: got %q, want v2.1.0", ctx.LocalAIVersion)
	}
	if !ctx.VersionInSync {
		t.Error("VersionInSync: got false, want true — versions match")
	}
}

// TestResolve_VersionOutOfSync_WhenVersionsDiffer verifies that VersionInSync
// is false when LocalAIVersion differs from FrameworkVersion.
func TestResolve_VersionOutOfSync_WhenVersionsDiffer(t *testing.T) {
	dir := t.TempDir()
	store := newVariableStore(map[string]string{
		ProjectVarName:          "PVT_v",
		FrameworkVersionVarName: "v2.1.0",
	})

	deps := resolveDeps("org", "repo", dir, store, "MyProject", nil, "v2.0.5", nil)

	ctx, err := Resolve(deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.VersionInSync {
		t.Error("VersionInSync: got true, want false — local v2.0.5 ≠ remote v2.1.0")
	}
}

// TestResolve_NoFrameworkVersion_VersionInSyncTrue verifies that when no
// AGENTIC_FRAMEWORK_VERSION is published, VersionInSync is always true
// (nothing to sync to).
func TestResolve_NoFrameworkVersion_VersionInSyncTrue(t *testing.T) {
	dir := t.TempDir()
	store := newVariableStore(map[string]string{
		ProjectVarName: "PVT_v",
	})

	deps := resolveDeps("org", "repo", dir, store, "MyProject", nil, "v2.0.5", nil)

	ctx, err := Resolve(deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ctx.VersionInSync {
		t.Error("VersionInSync: got false, want true — no remote version to compare against")
	}
}

// TestResolve_PermissionError_SetsProjectIDReadFailed verifies that a 403
// error from GetRepoVariable sets ProjectIDReadFailed without returning an
// error from Resolve.
func TestResolve_PermissionError_SetsProjectIDReadFailed(t *testing.T) {
	dir := t.TempDir()

	deps := Deps{
		Owner:        "org",
		RepoName:     "repo",
		RepoFullName: "org/repo",
		Root:         dir,
		GetRepoVariable: func(o, r, name string) (string, error) {
			return "", errors.New("HTTP 403: Resource not accessible by integration")
		},
		ReadAIVersion: func(root string) (string, error) { return "", nil },
	}

	ctx, err := Resolve(deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ctx.ProjectIDReadFailed {
		t.Error("ProjectIDReadFailed: got false, want true for HTTP 403 error")
	}
	if ctx.ProjectID != "" {
		t.Errorf("ProjectID: got %q, want empty — permission error means ID is unknown", ctx.ProjectID)
	}
}

// TestResolve_FederationMDPresent_AndNoProjectID verifies that FEDERATION.md
// presence is detected even when the repo has no AGENTIC_PROJECT_ID.
func TestResolve_FederationMDPresent_AndNoProjectID(t *testing.T) {
	dir := t.TempDir()
	content := `repos:
  - name: org/domain-one
    purpose: "First domain"
`
	if err := os.WriteFile(filepath.Join(dir, federationFileName), []byte(content), 0644); err != nil {
		t.Fatalf("writing FEDERATION.md: %v", err)
	}

	deps := Deps{
		Owner:        "org",
		RepoName:     "control-plane",
		RepoFullName: "org/control-plane",
		Root:         dir,
		GetRepoVariable: func(o, r, name string) (string, error) {
			return "", errors.New("not found")
		},
		ReadAIVersion: func(root string) (string, error) { return "", nil },
	}

	ctx, err := Resolve(deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.Topology != TopologyStringFederation {
		t.Errorf("Topology: got %q, want %q — FEDERATION.md present", ctx.Topology, TopologyStringFederation)
	}
	if ctx.ProjectID != "" {
		t.Errorf("ProjectID: got %q, want empty", ctx.ProjectID)
	}
}
