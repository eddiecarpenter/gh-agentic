package project

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestRunChecks_HealthyRepo(t *testing.T) {
	deps := testDeps("owner", "myrepo")
	report := RunChecks(deps)

	if report.FailCount() != 0 {
		t.Errorf("expected 0 failures, got %d", report.FailCount())
	}
}

func TestRunChecks_NoProjectID(t *testing.T) {
	deps := testDeps("owner", "myrepo")
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		return "", errors.New("not found")
	}

	report := RunChecks(deps)
	if report.FailCount() == 0 {
		t.Error("expected at least one failure when project ID not set")
	}

	// Find the project-id check
	found := false
	for _, r := range report.Results {
		if r.Name == "project-id" && r.Status == CheckFail {
			found = true
		}
	}
	if !found {
		t.Error("expected project-id check to fail")
	}
}

func TestRunChecks_ProjectNotAccessible(t *testing.T) {
	deps := testDeps("owner", "myrepo")
	deps.FetchLinkedRepos = func(projectID string) ([]LinkedRepo, error) {
		return nil, errors.New("not found")
	}

	report := RunChecks(deps)
	found := false
	for _, r := range report.Results {
		if r.Name == "project-accessible" && r.Status == CheckFail {
			found = true
		}
	}
	if !found {
		t.Error("expected project-accessible check to fail")
	}
}

func TestRunChecks_TopologyUnknown(t *testing.T) {
	deps := testDeps("owner", "myrepo")
	deps.FetchLinkedRepos = func(projectID string) ([]LinkedRepo, error) {
		return []LinkedRepo{}, nil // no linked repos
	}

	report := RunChecks(deps)
	found := false
	for _, r := range report.Results {
		if r.Name == "topology" && r.Status == CheckFail {
			found = true
		}
	}
	if !found {
		t.Error("expected topology check to fail when no linked repos")
	}
}

func TestRunChecks_FrameworkNotMounted(t *testing.T) {
	deps := testDeps("owner", "myrepo")
	deps.ReadAIVersion = func(root string) (string, error) {
		return "", errors.New("not mounted")
	}

	report := RunChecks(deps)
	found := false
	for _, r := range report.Results {
		if r.Name == "framework" && r.Status == CheckFail {
			found = true
			// AC-8 regression guard: remediation must not reference the legacy mount command.
			if strings.Contains(r.Remediation, "gh agentic mount") {
				t.Errorf("framework-not-mounted remediation contains stale 'gh agentic mount': %q", r.Remediation)
			}
		}
	}
	if !found {
		t.Error("expected framework check to fail when not mounted")
	}
}

// TestCheckFrameworkVersionSync_FederatedDomainOutOfSync_NoStaleMount is the
// AC-8 regression guard for the federated-domain version-sync path in
// checkFrameworkVersionSync. It exercises the branch that previously emitted
// "run 'gh agentic mount' to sync to the control plane version".
func TestCheckFrameworkVersionSync_FederatedDomainOutOfSync_NoStaleMount(t *testing.T) {
	// Set up a federated-domain topology: the linked repo is the control plane
	// (different from the current repo), and the control plane reports a
	// different version than the local .ai-version.
	deps := testDeps("domain-owner", "domain-repo")
	deps.FetchLinkedRepos = func(projectID string) ([]LinkedRepo, error) {
		return []LinkedRepo{{Name: "cp-repo", NameWithOwner: "cp-owner/cp-repo"}}, nil
	}
	deps.ReadAIVersion = func(root string) (string, error) { return "v1.0.0", nil }
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		switch n {
		case ProjectVarName:
			return "PVT_test", nil
		case FrameworkVersionVarName:
			// Control plane reports a newer version.
			return "v2.0.0", nil
		}
		return "", errors.New("not found")
	}

	result := checkFrameworkVersionSync(deps)

	if result.Status != CheckFail {
		t.Fatalf("expected CheckFail for out-of-sync federated domain, got %v (%s)", result.Status, result.Message)
	}
	if strings.Contains(result.Remediation, "gh agentic mount") {
		t.Errorf("framework-version-sync remediation contains stale 'gh agentic mount': %q", result.Remediation)
	}
	if !strings.Contains(result.Remediation, "gh agentic repair") {
		t.Errorf("framework-version-sync remediation should reference 'gh agentic repair', got: %q", result.Remediation)
	}
}

func TestPrintReport_AllPass(t *testing.T) {
	deps := testDeps("owner", "myrepo")
	report := RunChecks(deps)

	var buf bytes.Buffer
	ok := PrintReport(&buf, report)
	if !ok {
		t.Error("expected PrintReport to return true for all-pass")
	}
	if !strings.Contains(buf.String(), "All checks passed") {
		t.Error("expected 'All checks passed' in output")
	}
}

func TestPrintReport_WithFailure_ReturnsFalse(t *testing.T) {
	deps := testDeps("owner", "myrepo")
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		return "", errors.New("not found")
	}
	report := RunChecks(deps)

	var buf bytes.Buffer
	ok := PrintReport(&buf, report)
	if ok {
		t.Error("expected PrintReport to return false when checks fail")
	}
}
