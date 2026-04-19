package project

import "testing"

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
