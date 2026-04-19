package project

import "testing"

// TestProjectScopeReExports_AreWiredCorrectly verifies the project package
// re-exports of the scope API delegate to internal/scope. The full routing
// matrix is exercised by internal/scope/scope_test.go; this test exists to
// ensure the project-level aliases stay in sync with the underlying
// implementation.
func TestProjectScopeReExports_AreWiredCorrectly(t *testing.T) {
	const (
		owner = "acme-org"
		repo  = "acme-org/charging-cp"
	)

	if !IsSharedName("AGENT_USER") {
		t.Error("IsSharedName(AGENT_USER) should be true")
	}
	if !IsIdentityName("AGENTIC_PROJECT_ID") {
		t.Error("IsIdentityName(AGENTIC_PROJECT_ID) should be true")
	}

	flag, target := ScopeFor("AGENT_USER", "federated", owner, repo)
	if flag != ScopeFlagOrg || target != owner {
		t.Errorf("federated shared: got (%q, %q), want (%q, %q)", flag, target, ScopeFlagOrg, owner)
	}

	flag, target = ScopeFor("AGENTIC_PROJECT_ID", "federated", owner, repo)
	if flag != ScopeFlagRepo || target != repo {
		t.Errorf("federated identity: got (%q, %q), want (%q, %q)", flag, target, ScopeFlagRepo, repo)
	}
}
