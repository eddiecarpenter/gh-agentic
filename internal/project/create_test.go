package project

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestCreate_BlockedWhenAlreadyAffiliated(t *testing.T) {
	deps := testDeps("owner", "repo")
	// AGENTIC_PROJECT_ID is already set.
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		return "PVT_existing", nil
	}

	var buf bytes.Buffer
	err := Create(&buf, deps, CreateConfig{Title: "New Project", Version: "v2.0.10"})
	if err == nil {
		t.Fatal("expected error when already affiliated")
	}
	if !strings.Contains(err.Error(), "PVT_existing") {
		t.Errorf("expected error to mention existing project ID, got: %v", err)
	}
}

func TestCreate_BlockedWhenAlreadyLinkedToProject(t *testing.T) {
	deps := testDeps("owner", "repo")
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		return "", errors.New("not set")
	}
	deps.FetchProjectsForRepo = func(o, r string) ([]ProjectInfo, error) {
		return []ProjectInfo{{ID: "PVT_linked", Title: "Existing Project"}}, nil
	}

	var buf bytes.Buffer
	err := Create(&buf, deps, CreateConfig{Title: "New Project", Version: "v2.0.10"})
	if err == nil {
		t.Fatal("expected error when repo is already linked to a project")
	}
	if !strings.Contains(err.Error(), "Existing Project") {
		t.Errorf("expected error to mention existing project title, got: %v", err)
	}
}

func TestCreate_SuccessfulFlow(t *testing.T) {
	tmp := t.TempDir()

	var setVarCalled bool
	var createdProjectID string
	var linkedProjectID string
	var cloneCalled bool

	deps := testDeps("owner", "repo")
	deps.Root = tmp
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		return "", errors.New("not set")
	}
	deps.FetchProjectsForRepo = func(o, r string) ([]ProjectInfo, error) {
		return nil, nil
	}
	deps.DetectOwnerType = func(owner string) (string, error) {
		return "Organization", nil
	}
	deps.FetchOwnerAndRepoIDs = func(owner, repo string) (string, string, error) {
		return "O_owner", "R_repo", nil
	}
	deps.CreateProject = func(ownerID, title string) (string, error) {
		createdProjectID = "PVT_new"
		return createdProjectID, nil
	}
	deps.LinkRepoToProject = func(projectID, repoID string) error {
		linkedProjectID = projectID
		return nil
	}
	deps.SetRepoVariable = func(o, r, n, v string) error {
		if n == ProjectVarName {
			setVarCalled = true
		}
		return nil
	}
	deps.Clone = func(repoURL, tag, destDir string) error {
		cloneCalled = true
		return nil
	}

	var buf bytes.Buffer
	err := Create(&buf, deps, CreateConfig{Title: "My Project", Version: "v2.0.10"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !setVarCalled {
		t.Error("expected SetRepoVariable to be called")
	}
	if createdProjectID != "PVT_new" {
		t.Errorf("expected project ID PVT_new, got %s", createdProjectID)
	}
	if linkedProjectID != "PVT_new" {
		t.Errorf("expected linked project ID PVT_new, got %s", linkedProjectID)
	}
	if !cloneCalled {
		t.Error("expected Clone to be called")
	}

	out := buf.String()
	if !strings.Contains(out, "PVT_new") {
		t.Errorf("expected project ID in output, got:\n%s", out)
	}
}

func TestCreate_WarnsForPersonalAccount(t *testing.T) {
	tmp := t.TempDir()

	deps := testDeps("owner", "repo")
	deps.Root = tmp
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		return "", errors.New("not set")
	}
	deps.FetchProjectsForRepo = func(o, r string) ([]ProjectInfo, error) {
		return nil, nil
	}
	deps.DetectOwnerType = func(owner string) (string, error) {
		return "User", nil
	}

	var buf bytes.Buffer
	_ = Create(&buf, deps, CreateConfig{Title: "My Project", Version: "v2.0.10"})

	out := buf.String()
	if !strings.Contains(out, "personal account") {
		t.Errorf("expected personal account warning in output, got:\n%s", out)
	}
}

func TestCreate_PropagatesCreateProjectError(t *testing.T) {
	tmp := t.TempDir()

	deps := testDeps("owner", "repo")
	deps.Root = tmp
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		return "", errors.New("not set")
	}
	deps.FetchProjectsForRepo = func(o, r string) ([]ProjectInfo, error) {
		return nil, nil
	}
	deps.CreateProject = func(ownerID, title string) (string, error) {
		return "", errors.New("API error")
	}

	var buf bytes.Buffer
	err := Create(&buf, deps, CreateConfig{Title: "My Project", Version: "v2.0.10"})
	if err == nil {
		t.Fatal("expected error from CreateProject to propagate")
	}
}
