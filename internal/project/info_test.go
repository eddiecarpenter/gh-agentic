package project

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/eddiecarpenter/gh-agentic/internal/mount"
)

func testDeps(owner, repo string) Deps {
	return Deps{
		RepoFullName: owner + "/" + repo,
		Owner:        owner,
		RepoName:     repo,
		Root:         "/fake/root",
		FetchLinkedRepos: func(projectID string) ([]LinkedRepo, error) {
			return []LinkedRepo{{Name: repo, NameWithOwner: owner + "/" + repo}}, nil
		},
		FetchProjectsForRepo: func(o, r string) ([]ProjectInfo, error) {
			return nil, nil
		},
		GetRepoVariable: func(o, r, name string) (string, error) {
			switch name {
			case ProjectVarName:
				return "PVT_test123", nil
			case TopologyVarName:
				return "single", nil
				// AGENTIC_FRAMEWORK_VERSION intentionally not set — single topology
				// repos don't broadcast a version; local .ai-version is authoritative.
			}
			return "", errors.New("not found")
		},
		SetRepoVariable:    func(o, r, n, v string) error { return nil },
		DeleteRepoVariable: func(o, r, n string) error { return nil },
		ReadAIVersion:      func(root string) (string, error) { return "v2.0.10", nil },
		FetchOwnerAndRepoIDs: func(owner, repo string) (string, string, error) {
			return "owner-node-id", "repo-node-id", nil
		},
		CreateProject: func(ownerID, title string) (string, error) {
			return "PVT_created123", nil
		},
		LinkRepoToProject: func(projectID, repoID string) error { return nil },
		Confirm:           func(prompt string) (bool, error) { return true, nil },
		DetectOwnerType:   func(owner string) (string, error) { return "Organization", nil },
		Clone:             func(repoURL, tag, destDir string) error { return nil },
		FetchReleases: func(repo string) ([]mount.Release, error) {
			return []mount.Release{{TagName: "v2.0.10"}}, nil
		},
		UpdateProject: func(projectID, shortDescription, readme string) error { return nil },
		FetchProjectFields: func(projectID string) ([]ProjectField, error) {
			return []ProjectField{{ID: "field-id", Name: "Status", DataType: "SINGLE_SELECT"}}, nil
		},
		UpdateStatusFieldOptions: func(fieldID string, options []StatusOption) error { return nil },
		FetchProjectNumber:       func(projectID string) (int, error) { return 1, nil },
		CreateProjectView: func(owner, ownerType string, projectNumber int, name, layout, filter string) error {
			return nil
		},
		FetchProjectViews: func(projectID string) ([]ProjectView, error) {
			return []ProjectView{{Name: "Requirements"}, {Name: "Requirements Kanban"}, {Name: "Features Kanban"}}, nil
		},
		FetchProjectsForOwner: func(owner, ownerType string) ([]ProjectInfo, error) {
			return []ProjectInfo{{ID: "PVT_test123", Title: "Test Project"}}, nil
		},
		FetchProjectTitle: func(projectID string) (string, error) {
			if projectID == "PVT_test123" {
				return "Test Project", nil
			}
			return "", nil
		},
	}
}

func TestPrintInfo_Single(t *testing.T) {
	deps := testDeps("owner", "myrepo")

	var buf bytes.Buffer
	if err := PrintInfo(&buf, deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Test Project") {
		t.Error("expected project name in output")
	}
	if !strings.Contains(out, "PVT_test123") {
		t.Error("expected project ID in output")
	}
	if !strings.Contains(out, "Single") {
		t.Error("expected topology Single in output")
	}
	if !strings.Contains(out, "v2.0.10") {
		t.Error("expected framework version in output")
	}
}

func TestPrintInfo_NoProjectID(t *testing.T) {
	deps := testDeps("owner", "myrepo")
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		return "", errors.New("not found")
	}

	var buf bytes.Buffer
	err := PrintInfo(&buf, deps)
	if err == nil {
		t.Fatal("expected error when AGENTIC_PROJECT_ID not set")
	}
}

func TestPrintInfo_FrameworkNotMounted(t *testing.T) {
	deps := testDeps("owner", "myrepo")
	deps.ReadAIVersion = func(root string) (string, error) {
		return "", errors.New("not mounted")
	}

	var buf bytes.Buffer
	if err := PrintInfo(&buf, deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "not mounted") {
		t.Error("expected 'not mounted' in output")
	}
}

func TestPrintInfo_Federated(t *testing.T) {
	deps := testDeps("org", "domain-repo")
	deps.FetchLinkedRepos = func(projectID string) ([]LinkedRepo, error) {
		return []LinkedRepo{{Name: "control-plane", NameWithOwner: "org/control-plane"}}, nil
	}

	var buf bytes.Buffer
	if err := PrintInfo(&buf, deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Federated") {
		t.Error("expected topology Federated in output")
	}
	if !strings.Contains(out, "org/control-plane") {
		t.Error("expected control plane in output")
	}
}
