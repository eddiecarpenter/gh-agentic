package project

import (
	"errors"
	"testing"
)

// --- DetectTopology tests ---

func TestDetectTopology_Single(t *testing.T) {
	linked := []LinkedRepo{
		{Name: "myrepo", NameWithOwner: "owner/myrepo", URL: "https://github.com/owner/myrepo"},
	}
	got := DetectTopology("owner/myrepo", linked)
	if got != TopologySingle {
		t.Errorf("expected Single, got %s", got)
	}
}

func TestDetectTopology_SingleCaseInsensitive(t *testing.T) {
	linked := []LinkedRepo{
		{Name: "MyRepo", NameWithOwner: "Owner/MyRepo"},
	}
	got := DetectTopology("owner/myrepo", linked)
	if got != TopologySingle {
		t.Errorf("expected Single (case-insensitive), got %s", got)
	}
}

func TestDetectTopology_Federated(t *testing.T) {
	linked := []LinkedRepo{
		{Name: "control-plane", NameWithOwner: "org/control-plane"},
	}
	got := DetectTopology("org/domain-repo", linked)
	if got != TopologyFederated {
		t.Errorf("expected Federated, got %s", got)
	}
}

func TestDetectTopology_Unknown_NoLinkedRepos(t *testing.T) {
	got := DetectTopology("owner/myrepo", nil)
	if got != TopologyUnknown {
		t.Errorf("expected Unknown for empty linked repos, got %s", got)
	}
}

// --- ControlPlaneRepo tests ---

func TestControlPlaneRepo_ReturnsFirst(t *testing.T) {
	linked := []LinkedRepo{
		{Name: "control-plane", NameWithOwner: "org/control-plane"},
	}
	repo, ok := ControlPlaneRepo(linked)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if repo.NameWithOwner != "org/control-plane" {
		t.Errorf("expected org/control-plane, got %s", repo.NameWithOwner)
	}
}

func TestControlPlaneRepo_EmptyLinked(t *testing.T) {
	_, ok := ControlPlaneRepo(nil)
	if ok {
		t.Error("expected ok=false for empty linked repos")
	}
}

// --- Injectable fake helpers ---

func fakeFetchLinkedRepos(repos []LinkedRepo, err error) FetchLinkedReposFunc {
	return func(projectID string) ([]LinkedRepo, error) {
		return repos, err
	}
}

func fakeFetchProjectsForRepo(projects []ProjectInfo, err error) FetchProjectsForRepoFunc {
	return func(owner, repo string) ([]ProjectInfo, error) {
		return projects, err
	}
}

func fakeGetRepoVariable(value string, err error) GetRepoVariableFunc {
	return func(owner, repo, name string) (string, error) {
		return value, err
	}
}

func fakeSetRepoVariable(err error) SetRepoVariableFunc {
	return func(owner, repo, name, value string) error {
		return err
	}
}

func fakeDeleteRepoVariable(err error) DeleteRepoVariableFunc {
	return func(owner, repo, name string) error {
		return err
	}
}

// --- Fake-driven topology resolution tests ---

func TestDetectTopology_ViaFakes_Single(t *testing.T) {
	fetch := fakeFetchLinkedRepos([]LinkedRepo{
		{NameWithOwner: "owner/myrepo"},
	}, nil)

	linked, err := fetch("PVT_test")
	if err != nil {
		t.Fatal(err)
	}
	topo := DetectTopology("owner/myrepo", linked)
	if topo != TopologySingle {
		t.Errorf("expected Single, got %s", topo)
	}
}

func TestDetectTopology_ViaFakes_Federated(t *testing.T) {
	fetch := fakeFetchLinkedRepos([]LinkedRepo{
		{NameWithOwner: "org/control-plane"},
	}, nil)

	linked, err := fetch("PVT_test")
	if err != nil {
		t.Fatal(err)
	}
	topo := DetectTopology("org/domain-repo", linked)
	if topo != TopologyFederated {
		t.Errorf("expected Federated, got %s", topo)
	}
}

func TestFetchLinkedRepos_ErrorPropagated(t *testing.T) {
	fetch := fakeFetchLinkedRepos(nil, errors.New("network error"))
	_, err := fetch("PVT_test")
	if err == nil {
		t.Error("expected error to propagate")
	}
}

func TestGetRepoVariable_ValueReturned(t *testing.T) {
	get := fakeGetRepoVariable("PVT_abc123", nil)
	val, err := get("owner", "repo", ProjectVarName)
	if err != nil {
		t.Fatal(err)
	}
	if val != "PVT_abc123" {
		t.Errorf("expected PVT_abc123, got %s", val)
	}
}

func TestSetRepoVariable_Success(t *testing.T) {
	set := fakeSetRepoVariable(nil)
	if err := set("owner", "repo", ProjectVarName, "PVT_abc123"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSetRepoVariable_ErrorPropagated(t *testing.T) {
	set := fakeSetRepoVariable(errors.New("api error"))
	err := set("owner", "repo", ProjectVarName, "PVT_abc123")
	if err == nil {
		t.Error("expected error to propagate")
	}
}

func TestDeleteRepoVariable_Success(t *testing.T) {
	del := fakeDeleteRepoVariable(nil)
	if err := del("owner", "repo", ProjectVarName); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
