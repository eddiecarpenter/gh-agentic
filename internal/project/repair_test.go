package project

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/eddiecarpenter/gh-agentic/internal/mount"
)

func TestRepair_NoIssues(t *testing.T) {
	deps := testDeps("owner", "repo")

	var buf bytes.Buffer
	if err := Repair(&buf, deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "No agentic project issues found") {
		t.Errorf("expected 'No agentic project issues found' in output, got:\n%s", buf.String())
	}
}

func TestRepair_FrameworkNotMounted_FixesIt(t *testing.T) {
	tmp := t.TempDir()

	var cloneCalled bool
	deps := testDeps("owner", "repo")
	deps.Root = tmp
	// Framework not mounted.
	deps.ReadAIVersion = func(root string) (string, error) {
		return "", errors.New("not mounted")
	}
	deps.FetchReleases = func(repo string) ([]mount.Release, error) {
		return []mount.Release{{TagName: "v2.0.10"}}, nil
	}
	deps.Clone = func(repoURL, tag, destDir string) error {
		cloneCalled = true
		return nil
	}

	var buf bytes.Buffer
	if err := Repair(&buf, deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cloneCalled {
		t.Error("expected Clone to be called during framework repair")
	}

	out := buf.String()
	if !strings.Contains(out, "v2.0.10") {
		t.Errorf("expected version in output, got:\n%s", out)
	}
}

func TestRepair_FrameworkNotMounted_FetchReleasesError(t *testing.T) {
	tmp := t.TempDir()

	deps := testDeps("owner", "repo")
	deps.Root = tmp
	deps.ReadAIVersion = func(root string) (string, error) {
		return "", errors.New("not mounted")
	}
	deps.FetchReleases = func(repo string) ([]mount.Release, error) {
		return nil, errors.New("network error")
	}

	var buf bytes.Buffer
	// Repair should not return an error itself — it reports unrepaired items.
	if err := Repair(&buf, deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "manual attention") {
		t.Errorf("expected manual attention message in output, got:\n%s", out)
	}
}

func TestRepair_MissingViews_RecreatesThem(t *testing.T) {
	var createdViews []string

	deps := testDeps("owner", "repo")
	// One view is missing from the project.
	deps.FetchProjectViews = func(projectID string) ([]ProjectView, error) {
		return []ProjectView{{Name: "Requirements"}, {Name: "Requirements Kanban"}}, nil
	}
	deps.CreateProjectView = func(owner, ownerType string, projectNumber int, name, layout, filter string) error {
		createdViews = append(createdViews, name)
		return nil
	}

	var buf bytes.Buffer
	if err := Repair(&buf, deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(createdViews) == 0 {
		t.Error("expected at least one view to be recreated")
	}
	// "Features Kanban" is the missing one based on testDeps returning only 2 views.
	found := false
	for _, v := range createdViews {
		if v == "Features Kanban" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'Features Kanban' to be recreated, got: %v", createdViews)
	}
}

func TestRepair_NonFrameworkCheck_ReportsUnrepairable(t *testing.T) {
	deps := testDeps("owner", "repo")
	// project-id check fails → non-repairable.
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		return "", errors.New("not set")
	}

	var buf bytes.Buffer
	if err := Repair(&buf, deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "manual attention") {
		t.Errorf("expected manual attention message in output, got:\n%s", out)
	}
}
