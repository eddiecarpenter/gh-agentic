package project

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// TestResolveTopology_ChargingDomainRegression guards against the exact
// misdetection observed in production against NewOpenBSS/charging-domain
// (see Feature #555 in eddiecarpenter/gh-agentic).
//
// Production scenario the resolver must handle correctly:
//
//   - The domain repo has no AGENTIC_TOPOLOGY variable set locally
//     (only the control plane broadcasts shared values).
//   - The domain repo has no AGENTIC_FRAMEWORK_VERSION variable set
//     locally (again, only the CP sets it on itself).
//   - AGENTIC_PROJECT_ID is known, and the linked project contains more
//     than one repo — specifically, it includes the control plane plus
//     one or more domain repos.
//
// Before the fix, `gh agentic check` downgraded this case to "single"
// via the default branch of the topology switch, and then reported
// shared org-level variables as missing because the caller treated the
// repo as standalone. After the fix, ResolveTopology must return
// "federated-domain" so downstream pipeline checks correctly expect
// shared values at the org level.
func TestResolveTopology_ChargingDomainRegression(t *testing.T) {
	// Charging-domain-like inputs: NewOpenBSS owns the project; the
	// project has a control plane plus multiple domain repos.
	linkedRepos := []LinkedRepo{
		{Name: "charging-control-plane", NameWithOwner: "NewOpenBSS/charging-control-plane"},
		{Name: "charging-domain", NameWithOwner: "NewOpenBSS/charging-domain"},
		{Name: "billing-domain", NameWithOwner: "NewOpenBSS/billing-domain"},
	}

	fetchCalls := 0
	fetch := func(projectID string) ([]LinkedRepo, error) {
		fetchCalls++
		if projectID != "PVT_charging" {
			t.Errorf("FetchLinkedRepos: got projectID %q, want %q", projectID, "PVT_charging")
		}
		return linkedRepos, nil
	}

	// Both AGENTIC_TOPOLOGY and AGENTIC_FRAMEWORK_VERSION are absent on
	// the domain repo — simulating reality.
	store := newVariableStore(map[string]string{})

	got, err := ResolveTopology(ResolveTopologyDeps{
		Owner:            "NewOpenBSS",
		Repo:             "charging-domain",
		ProjectID:        "PVT_charging",
		GetRepoVariable:  store.get,
		FetchLinkedRepos: fetch,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != TopologyStringFederatedDomain {
		t.Fatalf("regression: got %q, want %q — charging-domain must be detected as federated-domain when AGENTIC_TOPOLOGY is unset and the project has multiple linked repos",
			got, TopologyStringFederatedDomain)
	}

	// Topology decisions must not require more than one GraphQL round
	// trip — check and repair are already chatty.
	if fetchCalls != 1 {
		t.Errorf("FetchLinkedRepos invocations: got %d, want 1 (must be cached within a single resolve)", fetchCalls)
	}
}

// TestResolveTopology_CrossOwnerProject_DetectsFederatedDomain covers
// Feature #602: when the project is owned by a different login than the
// current repo, the resolver must classify the repo as federated-domain
// regardless of the linked-graph shape (even when the graph is sparse or
// the current repo is not linked).
func TestResolveTopology_CrossOwnerProject_DetectsFederatedDomain(t *testing.T) {
	store := newVariableStore(map[string]string{})

	got, err := ResolveTopology(ResolveTopologyDeps{
		Owner:            "acme-domain-owner",
		Repo:             "widget-service",
		ProjectID:        "PVT_cp",
		GetRepoVariable:  store.get,
		FetchLinkedRepos: func(string) ([]LinkedRepo, error) { return nil, nil },
		FetchProjectOwner: func(string) (string, error) {
			return "acme-platform", nil
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != TopologyStringFederatedDomain {
		t.Errorf("cross-owner: got %q, want %q", got, TopologyStringFederatedDomain)
	}
}

// TestRepairTopologyVars_FederatedDomain_MissingCPVersion_HardStops covers
// Feature #602: when a federated-domain repo discovers the CP has no
// AGENTIC_FRAMEWORK_VERSION, repair must terminate with a pointed message
// naming the CP repo — never cross-write.
func TestRepairTopologyVars_FederatedDomain_MissingCPVersion_HardStops(t *testing.T) {
	// Domain repo with no local vars beyond PROJECT_ID; CP has no fwVer either.
	deps := testDeps("NewOpenBSS", "charging-domain")
	deps.GetRepoVariable = func(owner, repo, name string) (string, error) {
		if owner == "NewOpenBSS" && repo == "charging-domain" && name == ProjectVarName {
			return "PVT_charging", nil
		}
		return "", errors.New("not found")
	}
	deps.FetchLinkedRepos = func(string) ([]LinkedRepo, error) {
		return []LinkedRepo{
			{Name: "charging-control-plane", NameWithOwner: "NewOpenBSS/charging-control-plane"},
			{Name: "charging-domain", NameWithOwner: "NewOpenBSS/charging-domain"},
		}, nil
	}
	deps.FetchProjectOwner = func(string) (string, error) { return "NewOpenBSS", nil }

	var buf bytes.Buffer
	err := repairTopologyVars(&buf, deps)
	if err == nil {
		t.Fatal("expected a hard-stop error; got nil")
	}
	if !strings.Contains(err.Error(), "charging-control-plane") {
		t.Errorf("error must name the CP repo; got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "gh agentic repair") {
		t.Errorf("error must instruct to run repair on the CP; got %q", err.Error())
	}
}

// TestResolve_ChargingDomainEndToEnd locks in Feature #571 AC3: the bug
// that produced 16 spurious errors on 2026-04-19 (requirement #569) cannot
// recur. The test feeds project.Resolve the exact variable configuration
// that tripped charging-domain and asserts the entire Context shape that
// downstream commands consume — not just the topology string.
//
// Fails if a future change reinstates an ad-hoc AGENTIC_* read in mount /
// check / repair (the boundary test at internal/project/boundary_test.go
// covers that syntactically; this test covers the semantic outcome end-to-
// end).
func TestResolve_ChargingDomainEndToEnd(t *testing.T) {
	// Variable store that mirrors the charging-domain repo on the day of
	// the incident: AGENTIC_PROJECT_ID is set, AGENTIC_TOPOLOGY is NOT set
	// (the stopgap), and AGENTIC_FRAMEWORK_VERSION is NOT set locally
	// (it lives on the CP). Any lookup for an unset variable errors, as
	// the go-gh 404 wrapper does.
	store := newVariableStore(map[string]string{
		ProjectVarName: "PVT_charging",
	})

	// The CP broadcasts its framework version via the same variable name
	// on a different repo. The resolver must reach across repo owners to
	// pick this up.
	cpVersion := "v2.1.0"
	linked := []LinkedRepo{
		{Name: "charging-control-plane", NameWithOwner: "NewOpenBSS/charging-control-plane"},
		{Name: "charging-domain", NameWithOwner: "NewOpenBSS/charging-domain"},
		{Name: "billing-domain", NameWithOwner: "NewOpenBSS/billing-domain"},
	}

	fetchCalls := 0

	deps := Deps{
		Owner:        "NewOpenBSS",
		RepoName:     "charging-domain",
		RepoFullName: "NewOpenBSS/charging-domain",
		Root:         "/fake/root",
		GetRepoVariable: func(owner, repo, name string) (string, error) {
			// Domain-side lookups go through the empty-store fake.
			if owner == "NewOpenBSS" && repo == "charging-domain" {
				return store.get(owner, repo, name)
			}
			// CP broadcasts AGENTIC_FRAMEWORK_VERSION on itself. Only
			// this read is expected cross-repo.
			if owner == "NewOpenBSS" && repo == "charging-control-plane" && name == FrameworkVersionVarName {
				return cpVersion, nil
			}
			return "", errors.New("not found")
		},
		FetchLinkedRepos: func(projectID string) ([]LinkedRepo, error) {
			fetchCalls++
			if projectID != "PVT_charging" {
				t.Errorf("FetchLinkedRepos: projectID=%q, want %q", projectID, "PVT_charging")
			}
			return linked, nil
		},
		FetchProjectTitle: func(projectID string) (string, error) {
			return "Charging Project", nil
		},
		ReadAIVersion: func(root string) (string, error) {
			// Domain repo has the clone at v2.0.5 — different from CP
			// to prove VersionInSync is computed off FrameworkVersion,
			// not LocalAIVersion.
			return "v2.0.5", nil
		},
	}

	ctx, err := Resolve(deps)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	// Full Context-shape assertion — AC3 "the resolver-derived Context is
	// the exact shape check / mount / repair expect".
	if ctx.Topology != TopologyStringFederatedDomain {
		t.Errorf("Topology: got %q, want %q — the #569 regression misdetected this as single",
			ctx.Topology, TopologyStringFederatedDomain)
	}
	if ctx.Role != RoleDomain {
		t.Errorf("Role: got %q, want %q", ctx.Role, RoleDomain)
	}
	if ctx.ProjectID != "PVT_charging" {
		t.Errorf("ProjectID: got %q, want %q", ctx.ProjectID, "PVT_charging")
	}
	if ctx.ProjectName != "Charging Project" {
		t.Errorf("ProjectName: got %q, want %q", ctx.ProjectName, "Charging Project")
	}
	if ctx.ProjectDeleted {
		t.Error("ProjectDeleted: got true, want false")
	}

	// The framework version must be sourced from the CP's broadcast, not
	// from the domain repo itself (which never sets it).
	if ctx.FrameworkVersion != cpVersion {
		t.Errorf("FrameworkVersion: got %q, want %q — resolver must read AGENTIC_FRAMEWORK_VERSION from the CP for a federated-domain repo",
			ctx.FrameworkVersion, cpVersion)
	}

	// ControlPlane must be the CP LinkedRepo, NOT the current repo.
	if ctx.ControlPlane.NameWithOwner != "NewOpenBSS/charging-control-plane" {
		t.Errorf("ControlPlane: got %q, want %q",
			ctx.ControlPlane.NameWithOwner, "NewOpenBSS/charging-control-plane")
	}

	// LinkedRepos must contain all three federation members so pipeline
	// commands can iterate if needed.
	if len(ctx.LinkedRepos) != 3 {
		t.Errorf("LinkedRepos length: got %d, want 3", len(ctx.LinkedRepos))
	}

	// Helper methods consumed by the CLI — these are the "is this the
	// federated CP?" / "am I a domain?" checks that mount, auth, and
	// init rely on. The bug-day misdetection made all three wrong.
	if !ctx.IsFederatedDomain() {
		t.Error("IsFederatedDomain(): got false, want true")
	}
	if ctx.IsFederatedControlPlane() {
		t.Error("IsFederatedControlPlane(): got true, want false")
	}
	if ctx.IsSingle() {
		t.Error("IsSingle(): got true, want false")
	}

	// LocalAIVersion reflects the disk state independently of the
	// authoritative FrameworkVersion. VersionInSync is computed off
	// FrameworkVersion — not equal here, so VersionInSync must be false.
	if ctx.LocalAIVersion != "v2.0.5" {
		t.Errorf("LocalAIVersion: got %q, want %q", ctx.LocalAIVersion, "v2.0.5")
	}
	if ctx.VersionInSync {
		t.Error("VersionInSync: got true, want false — local v2.0.5 differs from CP v2.1.0, this is the 'domain out of sync' signal mount uses")
	}

	// Performance guard: the full resolve must not explode into N
	// GraphQL calls. The topology + linked-repo inspection share the
	// same fetch (one call).
	if fetchCalls != 1 {
		t.Errorf("FetchLinkedRepos: got %d calls, want 1", fetchCalls)
	}
}
