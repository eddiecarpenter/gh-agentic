package project

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eddiecarpenter/gh-agentic/internal/mount"
)

// --- SwitchProject ---

func TestSwitchProject_RefusesUninitialised(t *testing.T) {
	deps := testDeps("owner", "repo")
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		return "", errors.New("not set")
	}

	var buf bytes.Buffer
	err := SwitchProject(&buf, deps, "PVT_new")
	if err == nil {
		t.Fatal("expected error when repo is not affiliated with a project")
	}
	if !strings.Contains(err.Error(), "not affiliated") {
		t.Errorf("error should mention 'not affiliated', got: %v", err)
	}
}

// --- PreflightSwitchVersion ---

func TestPreflightSwitchVersion_RefusesUninitialised(t *testing.T) {
	deps := testDeps("owner", "repo")
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		return "", errors.New("not set")
	}

	_, err := PreflightSwitchVersion(deps, "v2.0.0")
	if err == nil {
		t.Fatal("expected error when repo is not part of an agentic project")
	}
	if !strings.Contains(err.Error(), "not part of an agentic project") {
		t.Errorf("error should mention 'not part of an agentic project', got: %v", err)
	}
}

func TestPreflightSwitchVersion_RefusesFederatedDomain(t *testing.T) {
	// On a federated domain repo, version switching is only allowed
	// at the control plane. Verify the preflight refuses with a
	// pointer to the right repo.
	deps := testDeps("domainorg", "domain-repo")
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		if n == ProjectVarName {
			return "PVT_x", nil
		}
		return "", nil
	}
	deps.FetchLinkedRepos = func(projectID string) ([]LinkedRepo, error) {
		// Linked repo is the control plane, not the current repo.
		return []LinkedRepo{{Name: "control-plane", NameWithOwner: "cporg/control-plane"}}, nil
	}

	_, err := PreflightSwitchVersion(deps, "v2.0.0")
	if err == nil {
		t.Fatal("expected error on federated domain repo")
	}
	if !strings.Contains(err.Error(), "control plane") {
		t.Errorf("error should mention 'control plane', got: %v", err)
	}
	if !strings.Contains(err.Error(), "cporg/control-plane") {
		t.Errorf("error should name the control plane repo, got: %v", err)
	}
}

func TestPreflightSwitchVersion_FetchReleasesError(t *testing.T) {
	deps := testDeps("owner", "repo")
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		if n == ProjectVarName {
			return "PVT_x", nil
		}
		return "", nil
	}
	deps.FetchLinkedRepos = func(projectID string) ([]LinkedRepo, error) {
		return []LinkedRepo{{NameWithOwner: "owner/repo"}}, nil
	}
	deps.FetchReleases = func(repo string) ([]mount.Release, error) {
		return nil, errors.New("network down")
	}

	_, err := PreflightSwitchVersion(deps, "v2.0.0")
	if err == nil {
		t.Fatal("expected error on FetchReleases failure")
	}
	if !strings.Contains(err.Error(), "fetching releases") {
		t.Errorf("error should mention 'fetching releases', got: %v", err)
	}
}

func TestPreflightSwitchVersion_InvalidTag(t *testing.T) {
	deps := testDeps("owner", "repo")
	deps.RepoFullName = "owner/repo"
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		if n == ProjectVarName {
			return "PVT_x", nil
		}
		return "", nil
	}
	deps.FetchLinkedRepos = func(projectID string) ([]LinkedRepo, error) {
		return []LinkedRepo{{NameWithOwner: "owner/repo"}}, nil
	}
	deps.FetchReleases = func(repo string) ([]mount.Release, error) {
		return []mount.Release{{TagName: "v2.0.0"}}, nil
	}

	_, err := PreflightSwitchVersion(deps, "v9.9.9")
	if err == nil {
		t.Fatal("expected error for unknown tag")
	}
	if !strings.Contains(err.Error(), "v9.9.9") {
		t.Errorf("error should name the requested tag, got: %v", err)
	}
}

func TestPreflightSwitchVersion_HappyPathSingleTopology(t *testing.T) {
	deps := testDeps("owner", "repo")
	deps.RepoFullName = "owner/repo"
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		switch n {
		case ProjectVarName:
			return "PVT_x", nil
		case TopologyVarName:
			return "single", nil
		}
		return "", nil
	}
	deps.FetchLinkedRepos = func(projectID string) ([]LinkedRepo, error) {
		return []LinkedRepo{{NameWithOwner: "owner/repo"}}, nil
	}
	deps.FetchReleases = func(repo string) ([]mount.Release, error) {
		return []mount.Release{{TagName: "v2.5.6"}}, nil
	}
	deps.ReadAIVersion = func(root string) (string, error) {
		return "v2.5.5", nil
	}

	pre, err := PreflightSwitchVersion(deps, "v2.5.6")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pre.CurrentVersion != "v2.5.5" {
		t.Errorf("CurrentVersion = %q, want v2.5.5", pre.CurrentVersion)
	}
	if pre.IsFederatedCP {
		t.Error("IsFederatedCP should be false for single topology")
	}
}

// --- switchProjectListFederated ---

func TestSwitchProjectListFederated_FiltersByTopology(t *testing.T) {
	// Three projects: one is federated (CP repo has AGENTIC_TOPOLOGY=federated),
	// one is single, one has no linked repos. Only the federated one
	// should come back.
	deps := testDeps("owner", "repo")
	deps.DetectOwnerType = func(owner string) (string, error) { return "Organization", nil }
	deps.FetchProjectsForOwner = func(o, t string) ([]ProjectInfo, error) {
		return []ProjectInfo{
			{ID: "PVT_fed", Title: "Federated"},
			{ID: "PVT_single", Title: "Single"},
			{ID: "PVT_empty", Title: "No links"},
		}, nil
	}
	deps.FetchLinkedRepos = func(projectID string) ([]LinkedRepo, error) {
		switch projectID {
		case "PVT_fed":
			return []LinkedRepo{{NameWithOwner: "fedorg/cp"}}, nil
		case "PVT_single":
			return []LinkedRepo{{NameWithOwner: "owner/repo"}}, nil
		case "PVT_empty":
			return []LinkedRepo{}, nil
		}
		return nil, errors.New("unknown project")
	}
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		// Only fedorg/cp has AGENTIC_TOPOLOGY=federated.
		if o == "fedorg" && r == "cp" && n == TopologyVarName {
			return "federated", nil
		}
		return "", nil
	}

	got, err := switchProjectListFederated(deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 federated project, got %d: %+v", len(got), got)
	}
	if got[0].ID != "PVT_fed" {
		t.Errorf("expected PVT_fed, got %q", got[0].ID)
	}
}

func TestSwitchProjectListFederated_FetchProjectsError(t *testing.T) {
	deps := testDeps("owner", "repo")
	deps.DetectOwnerType = func(owner string) (string, error) { return "User", nil }
	deps.FetchProjectsForOwner = func(o, t string) ([]ProjectInfo, error) {
		return nil, errors.New("network error")
	}

	_, err := switchProjectListFederated(deps)
	if err == nil {
		t.Fatal("expected error when FetchProjectsForOwner fails")
	}
	if !strings.Contains(err.Error(), "fetching projects") {
		t.Errorf("error should mention 'fetching projects', got: %v", err)
	}
}

// TestSwitchVersion_SetsFrameworkVersionVariable_SingleTopology covers the
// v2.3.0 regression where `gh agentic upgrade` on a single-topology repo
// updated the .agents/ mount and the workflow uses: refs but left
// AGENTIC_FRAMEWORK_VERSION at the old value. Drift between the variable
// and the mounted tree is immediately visible to `check` and to the
// pipeline's own resolve-version step, which reads the variable. The
// fix makes variable-writing unconditional — `create` and `repair`
// already did this; `SwitchVersion` is now aligned.
//
// This test freezes the expected behaviour so the regression cannot
// silently return.
func TestSwitchVersion_SetsFrameworkVersionVariable_SingleTopology(t *testing.T) {
	root := t.TempDir()

	// Seed a fake mounted .agents/ at v2.2.6 as a tracked submodule, so
	// RunSwitch's DownloadFramework dispatch sees MountStateSubmodule
	// and routes to the swap path.
	aiDir := filepath.Join(root, ".agents")
	if err := os.MkdirAll(aiDir, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitmodules"),
		[]byte(`[submodule ".agents"]`+"\n\turl = "+mount.FrameworkRepoURL+"\n"), 0o644); err != nil {
		t.Fatalf("seed .gitmodules: %v", err)
	}
	withFakeSwap(t)

	// Capture the variables the code writes.
	writes := make(map[string]string)
	reads := map[string]string{
		TopologyVarName: "", // single topology — variable unset
	}

	deps := Deps{
		Root:         root,
		Owner:        "eddiecarpenter",
		RepoName:     "ocs-testbench",
		RepoFullName: "eddiecarpenter/ocs-testbench",
		GetRepoVariable: func(owner, repo, name string) (string, error) {
			return reads[name], nil
		},
		SetRepoVariable: func(owner, repo, name, value string) error {
			writes[name] = value
			return nil
		},
		// No-op clone — the test doesn't exercise mount.RunSwitch's
		// clone logic, only the post-clone variable write. The stub
		// lets RunSwitch believe the clone succeeded.
		Clone: func(repoURL, tag, destDir string) error {
			return os.MkdirAll(destDir, 0o755)
		},
	}

	// CurrentVersion != version and IsFederatedCP: false — the exact
	// case that was broken in v2.3.0.
	pre := SwitchVersionPreflight{
		CurrentVersion: "v2.2.6",
		IsFederatedCP:  false,
	}

	var buf bytes.Buffer
	if err := SwitchVersion(&buf, deps, "v2.3.0", pre, nil); err != nil {
		// RunSwitch may fail if the stub clone does not produce the
		// expected workflow file structure; tolerate that as long as
		// the variable write happened (the bug was the write being
		// skipped, not the clone).
		t.Logf("SwitchVersion returned error (possibly from mount layer): %v", err)
	}

	if got := writes[FrameworkVersionVarName]; got != "v2.3.0" {
		t.Errorf("%s variable must be updated on single-topology upgrade, got %q (want v2.3.0)",
			FrameworkVersionVarName, got)
	}
}

// TestSwitchVersion_SetsFrameworkVersionVariable_AlreadyAtTarget covers
// the "already at version" path — variable must still be written if it
// disagrees with the mounted version. This is the recovery case for a
// repo whose .agents/ was mounted at the right version but whose variable
// was never set (e.g. manual mount).
func TestSwitchVersion_SetsFrameworkVersionVariable_AlreadyAtTarget(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".agents"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	writes := make(map[string]string)
	// Variable is absent — the bug case: user's repo has `.agents/` at
	// v2.3.0 but AGENTIC_FRAMEWORK_VERSION has never been set.
	reads := map[string]string{}

	deps := Deps{
		Root:         root,
		Owner:        "eddiecarpenter",
		RepoName:     "ocs-testbench",
		RepoFullName: "eddiecarpenter/ocs-testbench",
		GetRepoVariable: func(owner, repo, name string) (string, error) {
			return reads[name], nil
		},
		SetRepoVariable: func(owner, repo, name, value string) error {
			writes[name] = value
			return nil
		},
	}

	pre := SwitchVersionPreflight{
		CurrentVersion: "v2.3.0",
		IsFederatedCP:  false,
	}

	var buf bytes.Buffer
	if err := SwitchVersion(&buf, deps, "v2.3.0", pre, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := writes[FrameworkVersionVarName]; got != "v2.3.0" {
		t.Errorf("%s must be backfilled when mount already matches target, got %q",
			FrameworkVersionVarName, got)
	}
}

// TestSwitchVersion_SkipsVariableWrite_WhenAlreadyCorrect confirms the
// guard against needless writes: if the variable is already at the
// target value, no SetRepoVariable call is made. This is purely a
// spam-reduction check so the output doesn't claim to have changed
// something it hasn't.
func TestSwitchVersion_SkipsVariableWrite_WhenAlreadyCorrect(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".agents"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	writes := make(map[string]string)
	reads := map[string]string{
		FrameworkVersionVarName: "v2.3.0",
	}

	deps := Deps{
		Root:         root,
		Owner:        "eddiecarpenter",
		RepoName:     "ocs-testbench",
		RepoFullName: "eddiecarpenter/ocs-testbench",
		GetRepoVariable: func(owner, repo, name string) (string, error) {
			return reads[name], nil
		},
		SetRepoVariable: func(owner, repo, name, value string) error {
			writes[name] = value
			return nil
		},
	}

	pre := SwitchVersionPreflight{CurrentVersion: "v2.3.0", IsFederatedCP: false}

	var buf bytes.Buffer
	if err := SwitchVersion(&buf, deps, "v2.3.0", pre, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, wrote := writes[FrameworkVersionVarName]; wrote {
		t.Errorf("variable must not be rewritten when already at target; got write of %q",
			writes[FrameworkVersionVarName])
	}
}

// Compile-time assertion that the mount package is reachable from the
// test binary — if this breaks, the import above may have been optimised
// out by a future refactor and the test would silently stop covering
// the SwitchVersion → RunSwitch path.
var _ = mount.FrameworkRepo
