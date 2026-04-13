package project

import (
	"bytes"
	"errors"
	"strings"
	"testing"
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
			if name == ProjectVarName {
				return "PVT_test123", nil
			}
			return "", errors.New("not found")
		},
		SetRepoVariable:    func(o, r, n, v string) error { return nil },
		DeleteRepoVariable: func(o, r, n string) error { return nil },
		ReadAIVersion:      func(root string) (string, error) { return "v2.0.10", nil },
	}
}

func TestPrintInfo_Single(t *testing.T) {
	deps := testDeps("owner", "myrepo")

	var buf bytes.Buffer
	if err := PrintInfo(&buf, deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
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
