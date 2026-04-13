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
		}
	}
	if !found {
		t.Error("expected framework check to fail when not mounted")
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
