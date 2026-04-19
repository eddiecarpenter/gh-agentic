package project

import (
	"errors"
	"testing"
)

// resolveDeps assembles a project.Deps with just the fields Resolve reads,
// wiring the provided variable store and linked-repo fixture. Everything
// Resolve does not touch stays at the zero value.
func resolveDeps(owner, repo string, store *variableStore, linked []LinkedRepo, fetchErr error, title string, titleErr error, aiVersion string, aiErr error) Deps {
	fetchCalls := 0
	return Deps{
		Owner:        owner,
		RepoName:     repo,
		RepoFullName: owner + "/" + repo,
		Root:         "/fake/root",
		GetRepoVariable: func(o, r, name string) (string, error) {
			return store.get(o, r, name)
		},
		FetchLinkedRepos: func(projectID string) ([]LinkedRepo, error) {
			fetchCalls++
			return linked, fetchErr
		},
		FetchProjectTitle: func(projectID string) (string, error) {
			return title, titleErr
		},
		ReadAIVersion: func(root string) (string, error) {
			return aiVersion, aiErr
		},
	}
}

func TestResolve_Single_VariableOverride_ReturnsSingleStandalone(t *testing.T) {
	store := newVariableStore(map[string]string{
		ProjectVarName:  "PVT_solo",
		TopologyVarName: "single",
	})

	deps := resolveDeps("user", "solo", store, nil, nil, "Solo", nil, "v2.1.0", nil)

	ctx, err := Resolve(deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.Topology != TopologyStringSingle {
		t.Errorf("Topology: got %q, want %q", ctx.Topology, TopologyStringSingle)
	}
	if ctx.Role != RoleStandalone {
		t.Errorf("Role: got %q, want %q", ctx.Role, RoleStandalone)
	}
	if ctx.ProjectID != "PVT_solo" {
		t.Errorf("ProjectID: got %q, want %q", ctx.ProjectID, "PVT_solo")
	}
	if ctx.ProjectName != "Solo" {
		t.Errorf("ProjectName: got %q, want %q", ctx.ProjectName, "Solo")
	}
	if ctx.ProjectDeleted {
		t.Error("ProjectDeleted: got true, want false")
	}
	if ctx.LocalAIVersion != "v2.1.0" {
		t.Errorf("LocalAIVersion: got %q, want %q", ctx.LocalAIVersion, "v2.1.0")
	}
}

func TestResolve_FederatedCP_HasFrameworkVersion_ReturnsCPRole(t *testing.T) {
	store := newVariableStore(map[string]string{
		ProjectVarName:          "PVT_fed",
		TopologyVarName:         "federated",
		FrameworkVersionVarName: "v2.1.0",
	})

	deps := resolveDeps("org", "control-plane", store, nil, nil, "Federated", nil, "v2.1.0", nil)

	ctx, err := Resolve(deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.Topology != TopologyStringFederatedCP {
		t.Errorf("Topology: got %q, want %q", ctx.Topology, TopologyStringFederatedCP)
	}
	if ctx.Role != RoleCP {
		t.Errorf("Role: got %q, want %q", ctx.Role, RoleCP)
	}
	if !ctx.IsFederatedControlPlane() {
		t.Error("IsFederatedControlPlane(): got false, want true")
	}
	if ctx.FrameworkVersion != "v2.1.0" {
		t.Errorf("FrameworkVersion: got %q, want %q", ctx.FrameworkVersion, "v2.1.0")
	}
	if !ctx.VersionInSync {
		t.Error("VersionInSync: got false, want true")
	}
}

func TestResolve_FederatedDomain_ReadsCPVersion(t *testing.T) {
	// Domain-side variable store: only PROJECT_ID and TOPOLOGY set.
	// AGENTIC_FRAMEWORK_VERSION is NOT set on the domain; the resolver must
	// read it from the control plane repo instead.
	store := &variableStore{
		values: map[string]string{
			ProjectVarName:  "PVT_fed",
			TopologyVarName: "federated",
		},
		reads: map[string]int{},
	}
	// Intercept the fake: allow the CP lookup to return a version while
	// the domain lookup returns "not found".
	getVar := func(owner, repo, name string) (string, error) {
		// Lookups on the domain repo go through the store.
		if owner == "org" && repo == "domain-one" {
			return store.get(owner, repo, name)
		}
		// CP broadcasts AGENTIC_FRAMEWORK_VERSION.
		if owner == "org" && repo == "control-plane" && name == FrameworkVersionVarName {
			return "v2.1.0", nil
		}
		return "", errors.New("not found")
	}

	deps := Deps{
		Owner:           "org",
		RepoName:        "domain-one",
		RepoFullName:    "org/domain-one",
		Root:            "/fake/root",
		GetRepoVariable: getVar,
		FetchLinkedRepos: func(projectID string) ([]LinkedRepo, error) {
			return []LinkedRepo{
				{Name: "control-plane", NameWithOwner: "org/control-plane"},
				{Name: "domain-one", NameWithOwner: "org/domain-one"},
			}, nil
		},
		FetchProjectTitle: func(projectID string) (string, error) { return "Federated", nil },
		ReadAIVersion:     func(root string) (string, error) { return "v2.1.0", nil },
	}

	ctx, err := Resolve(deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.Topology != TopologyStringFederatedDomain {
		t.Errorf("Topology: got %q, want %q", ctx.Topology, TopologyStringFederatedDomain)
	}
	if ctx.Role != RoleDomain {
		t.Errorf("Role: got %q, want %q", ctx.Role, RoleDomain)
	}
	if ctx.FrameworkVersion != "v2.1.0" {
		t.Errorf("FrameworkVersion: got %q, want %q — domain must read CP broadcast", ctx.FrameworkVersion, "v2.1.0")
	}
	if ctx.ControlPlane.NameWithOwner != "org/control-plane" {
		t.Errorf("ControlPlane: got %q, want %q", ctx.ControlPlane.NameWithOwner, "org/control-plane")
	}
	if !ctx.VersionInSync {
		t.Error("VersionInSync: got false, want true — local and CP both v2.1.0")
	}
}

func TestResolve_NoProjectAffiliation_ReturnsStandaloneContext(t *testing.T) {
	store := newVariableStore(map[string]string{})

	deps := Deps{
		Owner:        "user",
		RepoName:     "stray",
		RepoFullName: "user/stray",
		Root:         "/fake/root",
		GetRepoVariable: func(o, r, name string) (string, error) {
			return store.get(o, r, name)
		},
		// FetchLinkedRepos intentionally returns an error marker so the
		// test can assert it was never invoked.
		FetchLinkedRepos: func(projectID string) ([]LinkedRepo, error) {
			t.Fatalf("FetchLinkedRepos must not be called when ProjectID is empty")
			return nil, nil
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
	if ctx.Role != RoleStandalone {
		t.Errorf("Role: got %q, want %q", ctx.Role, RoleStandalone)
	}
}

func TestResolve_DeletedProject_FlagsAsDeleted(t *testing.T) {
	store := newVariableStore(map[string]string{
		ProjectVarName:  "PVT_ghost",
		TopologyVarName: "single",
	})

	deps := Deps{
		Owner:        "org",
		RepoName:     "ex-project",
		RepoFullName: "org/ex-project",
		Root:         "/fake/root",
		GetRepoVariable: func(o, r, name string) (string, error) {
			return store.get(o, r, name)
		},
		FetchLinkedRepos: func(projectID string) ([]LinkedRepo, error) {
			return nil, nil
		},
		FetchProjectTitle: func(projectID string) (string, error) {
			// Deleted project → title is empty.
			return "", nil
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

func TestResolve_ChargingDomainScenario_NoLocalTopologyOrVersion_DetectsFederatedDomain(t *testing.T) {
	// Reproduces the exact production misdetection that motivated #569:
	// the domain repo has AGENTIC_PROJECT_ID set but AGENTIC_TOPOLOGY and
	// AGENTIC_FRAMEWORK_VERSION are absent locally, and the project has
	// multiple linked repos including the CP.
	store := newVariableStore(map[string]string{
		ProjectVarName: "PVT_charging",
	})

	fetchCalls := 0
	deps := Deps{
		Owner:        "NewOpenBSS",
		RepoName:     "charging-domain",
		RepoFullName: "NewOpenBSS/charging-domain",
		Root:         "/fake/root",
		GetRepoVariable: func(o, r, name string) (string, error) {
			return store.get(o, r, name)
		},
		FetchLinkedRepos: func(projectID string) ([]LinkedRepo, error) {
			fetchCalls++
			return []LinkedRepo{
				{Name: "charging-control-plane", NameWithOwner: "NewOpenBSS/charging-control-plane"},
				{Name: "charging-domain", NameWithOwner: "NewOpenBSS/charging-domain"},
				{Name: "billing-domain", NameWithOwner: "NewOpenBSS/billing-domain"},
			}, nil
		},
		FetchProjectTitle: func(projectID string) (string, error) { return "Charging", nil },
		ReadAIVersion:     func(root string) (string, error) { return "v2.0.5", nil },
	}

	ctx, err := Resolve(deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.Topology != TopologyStringFederatedDomain {
		t.Fatalf("regression: got %q, want %q — charging-domain must be detected as federated-domain without the AGENTIC_TOPOLOGY stopgap",
			ctx.Topology, TopologyStringFederatedDomain)
	}
	if ctx.Role != RoleDomain {
		t.Errorf("Role: got %q, want %q", ctx.Role, RoleDomain)
	}
	if ctx.ControlPlane.NameWithOwner != "NewOpenBSS/charging-control-plane" {
		t.Errorf("ControlPlane: got %q, want %q", ctx.ControlPlane.NameWithOwner, "NewOpenBSS/charging-control-plane")
	}
	if fetchCalls != 1 {
		t.Errorf("FetchLinkedRepos calls: got %d, want 1", fetchCalls)
	}
}

func TestResolve_LegacyWrappers_ReturnConsistentResults(t *testing.T) {
	// Drive both Resolve and ResolveTopology / ResolveState with the same
	// Deps and assert they agree on the topology dimension each exposes.
	store := newVariableStore(map[string]string{
		ProjectVarName:          "PVT_fed",
		TopologyVarName:         "federated",
		FrameworkVersionVarName: "v2.1.0",
	})

	deps := Deps{
		Owner:        "org",
		RepoName:     "control-plane",
		RepoFullName: "org/control-plane",
		Root:         "/fake/root",
		GetRepoVariable: func(o, r, name string) (string, error) {
			return store.get(o, r, name)
		},
		FetchLinkedRepos: func(projectID string) ([]LinkedRepo, error) {
			return []LinkedRepo{
				{Name: "control-plane", NameWithOwner: "org/control-plane"},
				{Name: "domain-one", NameWithOwner: "org/domain-one"},
			}, nil
		},
		FetchProjectTitle: func(projectID string) (string, error) { return "Federated", nil },
		ReadAIVersion:     func(root string) (string, error) { return "v2.1.0", nil },
	}

	ctx, err := Resolve(deps)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	topology, err := ResolveTopology(ResolveTopologyDeps{
		Owner:            deps.Owner,
		Repo:             deps.RepoName,
		ProjectID:        ctx.ProjectID,
		GetRepoVariable:  deps.GetRepoVariable,
		FetchLinkedRepos: deps.FetchLinkedRepos,
	})
	if err != nil {
		t.Fatalf("ResolveTopology: %v", err)
	}
	if topology != ctx.Topology {
		t.Errorf("ResolveTopology wrapper drifted from Resolve: got %q, want %q", topology, ctx.Topology)
	}

	state, err := ResolveState(deps)
	if err != nil {
		t.Fatalf("ResolveState: %v", err)
	}
	if state.ProjectID != ctx.ProjectID {
		t.Errorf("ResolveState.ProjectID: got %q, want %q", state.ProjectID, ctx.ProjectID)
	}
	if state.ProjectName != ctx.ProjectName {
		t.Errorf("ResolveState.ProjectName: got %q, want %q", state.ProjectName, ctx.ProjectName)
	}
	if state.ControlPlaneFrameworkVersion != ctx.FrameworkVersion {
		t.Errorf("ResolveState.ControlPlaneFrameworkVersion: got %q, want %q",
			state.ControlPlaneFrameworkVersion, ctx.FrameworkVersion)
	}
}

func TestContext_RoleHelpers(t *testing.T) {
	cases := []struct {
		topology  string
		wantRole  string
		isCP      bool
		isDomain  bool
		isSingle  bool
	}{
		{TopologyStringSingle, RoleStandalone, false, false, true},
		{TopologyStringFederatedCP, RoleCP, true, false, false},
		{TopologyStringFederatedDomain, RoleDomain, false, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.topology, func(t *testing.T) {
			ctx := &Context{Topology: tc.topology, Role: roleForTopology(tc.topology)}
			if ctx.Role != tc.wantRole {
				t.Errorf("Role: got %q, want %q", ctx.Role, tc.wantRole)
			}
			if ctx.IsFederatedControlPlane() != tc.isCP {
				t.Errorf("IsFederatedControlPlane: got %v, want %v", ctx.IsFederatedControlPlane(), tc.isCP)
			}
			if ctx.IsFederatedDomain() != tc.isDomain {
				t.Errorf("IsFederatedDomain: got %v, want %v", ctx.IsFederatedDomain(), tc.isDomain)
			}
			if ctx.IsSingle() != tc.isSingle {
				t.Errorf("IsSingle: got %v, want %v", ctx.IsSingle(), tc.isSingle)
			}
		})
	}
}
