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

// --- ResolveControlPlane tests ---

// fakeHasFederationFile returns a checker that reports true for the repos in
// haveFed (by "owner/repo"), records the repos queried, and optionally fails
// for failOn.
func fakeHasFederationFile(haveFed map[string]bool, failOn string) (RepoHasFederationFileFunc, *[]string) {
	queried := &[]string{}
	fn := func(owner, repo string) (bool, error) {
		key := owner + "/" + repo
		*queried = append(*queried, key)
		if failOn != "" && key == failOn {
			return false, errors.New("boom")
		}
		return haveFed[key], nil
	}
	return fn, queried
}

func TestResolveControlPlane_FindsManifestRepoNotFirst(t *testing.T) {
	linked := []LinkedRepo{
		{NameWithOwner: "org/domain-a"},
		{NameWithOwner: "org/control-plane"},
		{NameWithOwner: "org/domain-b"},
	}
	check, queried := fakeHasFederationFile(map[string]bool{"org/control-plane": true}, "")

	cp, ok, err := ResolveControlPlane(linked, check)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if cp.NameWithOwner != "org/control-plane" {
		t.Errorf("expected org/control-plane, got %s", cp.NameWithOwner)
	}
	// Should stop at the first match — domain-b is never queried.
	if got := *queried; len(got) != 2 || got[len(got)-1] != "org/control-plane" {
		t.Errorf("expected to stop at the manifest repo, queried: %v", got)
	}
}

func TestResolveControlPlane_NoManifestRepo(t *testing.T) {
	linked := []LinkedRepo{
		{NameWithOwner: "org/domain-a"},
		{NameWithOwner: "org/domain-b"},
	}
	check, _ := fakeHasFederationFile(map[string]bool{}, "")

	_, ok, err := ResolveControlPlane(linked, check)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected ok=false when no linked repo carries FEDERATION.md")
	}
}

func TestResolveControlPlane_EmptyLinked(t *testing.T) {
	check, queried := fakeHasFederationFile(map[string]bool{}, "")
	_, ok, err := ResolveControlPlane(nil, check)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected ok=false for empty linked repos")
	}
	if len(*queried) != 0 {
		t.Errorf("expected no checks for empty linked, queried: %v", *queried)
	}
}

func TestResolveControlPlane_CheckErrorPropagates(t *testing.T) {
	linked := []LinkedRepo{{NameWithOwner: "org/domain-a"}}
	check, _ := fakeHasFederationFile(map[string]bool{}, "org/domain-a")

	_, ok, err := ResolveControlPlane(linked, check)
	if err == nil {
		t.Fatal("expected error to propagate")
	}
	if ok {
		t.Error("expected ok=false on error")
	}
}

func TestResolveControlPlane_SkipsMalformedEntries(t *testing.T) {
	linked := []LinkedRepo{
		{NameWithOwner: "not-a-valid-entry"},
		{NameWithOwner: "org/control-plane"},
	}
	check, queried := fakeHasFederationFile(map[string]bool{"org/control-plane": true}, "")

	cp, ok, err := ResolveControlPlane(linked, check)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok || cp.NameWithOwner != "org/control-plane" {
		t.Errorf("expected to skip malformed entry and resolve control-plane, got ok=%v cp=%s", ok, cp.NameWithOwner)
	}
	// The malformed entry is never queried.
	for _, q := range *queried {
		if q == "not-a-valid-entry" {
			t.Errorf("malformed entry should not be queried, queried: %v", *queried)
		}
	}
}

// --- splitOwnerRepo tests ---

func TestSplitOwnerRepo(t *testing.T) {
	cases := []struct {
		in        string
		wantOwner string
		wantRepo  string
		wantOK    bool
	}{
		{"org/repo", "org", "repo", true},
		{"Org/Repo-Name", "Org", "Repo-Name", true},
		{"noslash", "", "", false},
		{"/repo", "", "", false},
		{"owner/", "", "", false},
		{"", "", "", false},
		{"a/b/c", "a", "b/c", true},
	}
	for _, c := range cases {
		owner, repo, ok := splitOwnerRepo(c.in)
		if owner != c.wantOwner || repo != c.wantRepo || ok != c.wantOK {
			t.Errorf("splitOwnerRepo(%q) = (%q,%q,%v), want (%q,%q,%v)",
				c.in, owner, repo, ok, c.wantOwner, c.wantRepo, c.wantOK)
		}
	}
}
