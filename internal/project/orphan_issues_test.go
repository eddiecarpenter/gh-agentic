package project

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// TestCheckOrphanProjectIssues_NoOrphans confirms the check passes
// when no type-labelled issues are missing from the project.
func TestCheckOrphanProjectIssues_NoOrphans(t *testing.T) {
	deps := testDeps("owner", "myrepo")
	result := checkOrphanProjectIssues(deps)
	if result.Status != CheckPass {
		t.Errorf("expected CheckPass, got %v — message: %s", result.Status, result.Message)
	}
}

// TestCheckOrphanProjectIssues_FailsWhenOrphansExist confirms the
// check returns a Fail result listing every orphan issue.
func TestCheckOrphanProjectIssues_FailsWhenOrphansExist(t *testing.T) {
	deps := testDeps("owner", "myrepo")
	deps.FetchOrphanIssues = func(owner, repo, projectID string) ([]OrphanIssue, error) {
		return []OrphanIssue{
			{Number: 13, NodeID: "I_13", Title: "Whisper STT", Labels: []string{"feature"}},
			{Number: 14, NodeID: "I_14", Title: "Foo", Labels: []string{"feature"}},
			{Number: 15, NodeID: "I_15", Title: "Bar", Labels: []string{"feature"}},
		}, nil
	}
	result := checkOrphanProjectIssues(deps)
	if result.Status != CheckFail {
		t.Errorf("expected CheckFail, got %v", result.Status)
	}
	for _, want := range []string{"#13", "#14", "#15", "3 open issues"} {
		if !strings.Contains(result.Message, want) {
			t.Errorf("expected message to contain %q — got %q", want, result.Message)
		}
	}
	if result.Remediation == "" {
		t.Error("expected a remediation string")
	}
}

// TestCheckOrphanProjectIssues_SkipsWhenNoProjectID confirms the
// check returns a Warn (not Fail) when the project variable is
// unset — earlier checks already report that, no need to compound.
func TestCheckOrphanProjectIssues_SkipsWhenNoProjectID(t *testing.T) {
	deps := testDeps("owner", "myrepo")
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		return "", errors.New("not set")
	}
	result := checkOrphanProjectIssues(deps)
	if result.Status != CheckWarn {
		t.Errorf("expected CheckWarn, got %v", result.Status)
	}
}

// TestCheckOrphanProjectIssues_WarnsOnFetchError confirms the check
// degrades gracefully when the API call fails. A transient GitHub
// error should not block the whole check report.
func TestCheckOrphanProjectIssues_WarnsOnFetchError(t *testing.T) {
	deps := testDeps("owner", "myrepo")
	deps.FetchOrphanIssues = func(owner, repo, projectID string) ([]OrphanIssue, error) {
		return nil, errors.New("transient API failure")
	}
	result := checkOrphanProjectIssues(deps)
	if result.Status != CheckWarn {
		t.Errorf("expected CheckWarn on fetch error, got %v", result.Status)
	}
	if !strings.Contains(result.Message, "transient API failure") {
		t.Errorf("expected message to surface the underlying error — got %q", result.Message)
	}
}

// TestRepairOrphanIssues_AddsEachOrphan confirms the repair iterates
// the orphan list and calls AddIssueToProject for each one.
func TestRepairOrphanIssues_AddsEachOrphan(t *testing.T) {
	deps := testDeps("owner", "myrepo")
	deps.FetchOrphanIssues = func(owner, repo, projectID string) ([]OrphanIssue, error) {
		return []OrphanIssue{
			{Number: 13, NodeID: "I_13", Title: "Whisper STT"},
			{Number: 14, NodeID: "I_14", Title: "Foo"},
		}, nil
	}
	calledWith := []string{}
	deps.AddIssueToProject = func(projectID, issueNodeID string) error {
		calledWith = append(calledWith, projectID+":"+issueNodeID)
		return nil
	}
	var buf bytes.Buffer
	if err := repairOrphanIssues(&buf, deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(calledWith) != 2 {
		t.Errorf("expected 2 calls to AddIssueToProject, got %d (%v)", len(calledWith), calledWith)
	}
	for _, want := range []string{"PVT_test123:I_13", "PVT_test123:I_14"} {
		found := false
		for _, got := range calledWith {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected AddIssueToProject called with %q — got %v", want, calledWith)
		}
	}
	if !strings.Contains(buf.String(), "Added #13") || !strings.Contains(buf.String(), "Added #14") {
		t.Errorf("expected per-issue ✓ lines in output — got:\n%s", buf.String())
	}
}

// TestRepairOrphanIssues_PartialFailureReportsAggregateError
// confirms that when one issue add fails, the repair surfaces what
// succeeded vs failed rather than silently swallowing the error.
func TestRepairOrphanIssues_PartialFailureReportsAggregateError(t *testing.T) {
	deps := testDeps("owner", "myrepo")
	deps.FetchOrphanIssues = func(owner, repo, projectID string) ([]OrphanIssue, error) {
		return []OrphanIssue{
			{Number: 13, NodeID: "I_13", Title: "ok"},
			{Number: 14, NodeID: "I_14", Title: "fails"},
		}, nil
	}
	deps.AddIssueToProject = func(projectID, issueNodeID string) error {
		if issueNodeID == "I_14" {
			return errors.New("API rejected")
		}
		return nil
	}
	var buf bytes.Buffer
	err := repairOrphanIssues(&buf, deps)
	if err == nil {
		t.Fatal("expected an aggregate error reporting the failed add")
	}
	if !strings.Contains(err.Error(), "#14") || !strings.Contains(err.Error(), "API rejected") {
		t.Errorf("expected error to surface #14 and underlying cause — got %q", err)
	}
	if !strings.Contains(err.Error(), "added 1") {
		t.Errorf("expected error to report the successful adds — got %q", err)
	}
}

// TestRepairOrphanIssues_EmptyList exits cleanly and reports no work.
func TestRepairOrphanIssues_EmptyList(t *testing.T) {
	deps := testDeps("owner", "myrepo") // default fake returns nil
	var buf bytes.Buffer
	if err := repairOrphanIssues(&buf, deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "No orphan issues") {
		t.Errorf("expected 'No orphan issues' message — got:\n%s", buf.String())
	}
}
