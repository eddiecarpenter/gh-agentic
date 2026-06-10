package project

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/eddiecarpenter/gh-agentic/internal/mount"
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
	var installCalled bool

	originalInstall := mount.InstallSubmodule
	mount.InstallSubmodule = func(root, tag string) error {
		installCalled = true
		return nil
	}
	t.Cleanup(func() { mount.InstallSubmodule = originalInstall })

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
	// deps.Clone is no longer consulted by production — kept for source
	// compatibility with the Deps struct; install side-effects are
	// stubbed via mount.InstallSubmodule above.
	deps.Clone = func(repoURL, tag, destDir string) error { return nil }

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
	if !installCalled {
		t.Error("expected mount.InstallSubmodule to be called")
	}

	out := buf.String()
	if !strings.Contains(out, "PVT_new") {
		t.Errorf("expected project ID in output, got:\n%s", out)
	}
}

func TestCreate_ScaffoldsProjectFromEmbeddedTemplate(t *testing.T) {
	tmp := t.TempDir()
	withFakeInstall(t)

	var updateProjectCalled bool
	var updateFieldCalled bool
	var fetchFieldsCalled bool
	var fetchNumberCalled bool
	var createdViews []string

	deps := testDeps("owner", "repo")
	deps.Root = tmp
	deps.GetRepoVariable = func(o, r, n string) (string, error) { return "", errors.New("not set") }
	deps.FetchProjectsForRepo = func(o, r string) ([]ProjectInfo, error) { return nil, nil }
	deps.DetectOwnerType = func(owner string) (string, error) { return "Organization", nil }
	deps.FetchOwnerAndRepoIDs = func(owner, repo string) (string, string, error) {
		return "O_owner", "R_repo", nil
	}
	deps.CreateProject = func(ownerID, title string) (string, error) { return "PVT_scaffold", nil }
	deps.LinkRepoToProject = func(projectID, repoID string) error { return nil }
	deps.SetRepoVariable = func(o, r, n, v string) error { return nil }
	deps.Clone = func(repoURL, tag, destDir string) error { return nil }
	deps.UpdateProject = func(projectID, shortDescription, readme string) error {
		updateProjectCalled = true
		return nil
	}
	deps.FetchProjectFields = func(projectID string) ([]ProjectField, error) {
		fetchFieldsCalled = true
		return []ProjectField{{ID: "PVTSSF_status", Name: "Status", DataType: "SINGLE_SELECT"}}, nil
	}
	deps.UpdateStatusFieldOptions = func(fieldID string, options []StatusOption) error {
		updateFieldCalled = true
		if len(options) == 0 {
			t.Error("expected status options, got none")
		}
		return nil
	}
	deps.FetchProjectNumber = func(projectID string) (int, error) {
		fetchNumberCalled = true
		return 42, nil
	}
	deps.CreateProjectView = func(owner, ownerType string, projectNumber int, name, layout, filter string) error {
		createdViews = append(createdViews, name)
		return nil
	}

	var buf bytes.Buffer
	if err := Create(&buf, deps, CreateConfig{Title: "Scaffold Test", Version: "v2.0.10"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !updateProjectCalled {
		t.Error("expected UpdateProject to be called")
	}
	if !fetchFieldsCalled {
		t.Error("expected FetchProjectFields to be called")
	}
	if !updateFieldCalled {
		t.Error("expected UpdateStatusFieldOptions to be called")
	}
	if !fetchNumberCalled {
		t.Error("expected FetchProjectNumber to be called")
	}
	if len(createdViews) == 0 {
		t.Errorf("expected at least one view to be created, got: %v", createdViews)
	}

	out := buf.String()
	if !strings.Contains(out, "View") {
		t.Errorf("expected view creation output, got:\n%s", out)
	}
}

func TestCreate_WarnsForPersonalAccount_SingleTopology(t *testing.T) {
	// Under single topology a personal account is merely sub-optimal — the
	// warning is preserved for discoverability. Federated + user is refused
	// (see TestCreate_FederatedUserOwner_RefusesWithVerbatimError).
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
	deps.FetchOwnerAndRepoIDs = func(owner, repo string) (string, string, error) {
		return "O_owner", "R_repo", nil
	}
	deps.CreateProject = func(ownerID, title string) (string, error) { return "PVT_x", nil }
	deps.LinkRepoToProject = func(projectID, repoID string) error { return nil }
	deps.SetRepoVariable = func(o, r, n, v string) error { return nil }
	deps.Clone = func(repoURL, tag, destDir string) error { return nil }

	var buf bytes.Buffer
	_ = Create(&buf, deps, CreateConfig{Title: "My Project", Version: "v2.0.10", Topology: "single"})

	out := buf.String()
	if !strings.Contains(out, "personal account") {
		t.Errorf("expected personal account warning in output, got:\n%s", out)
	}
}

func TestCreate_FederatedUserOwner_RefusesWithVerbatimError(t *testing.T) {
	tmp := t.TempDir()

	var createProjectCalled bool

	deps := testDeps("eddie", "repo")
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
	deps.CreateProject = func(ownerID, title string) (string, error) {
		createProjectCalled = true
		return "PVT_shouldnt", nil
	}

	var buf bytes.Buffer
	// Default topology is "federation" (no Topology field — Feature #824).
	err := Create(&buf, deps, CreateConfig{Title: "x", Version: "v2.0.10"})
	if err == nil {
		t.Fatal("expected error for federated+user owner")
	}
	want := "Federated topology requires a GitHub Organization. The owner 'eddie' is a user account, which cannot host org-scoped variables and secrets. Either move this repo under an organisation, or use `--topology single`."
	if err.Error() != want {
		t.Fatalf("error mismatch:\ngot:  %q\nwant: %q", err.Error(), want)
	}
	if createProjectCalled {
		t.Error("CreateProject must not be called when guard refuses")
	}
}

func TestCreate_SingleTopology_UserOwner_Proceeds(t *testing.T) {
	// Single topology is still allowed on user accounts — only federated
	// is hard-blocked. Verify the guard does not fire and the project is
	// created as before.
	tmp := t.TempDir()
	withFakeInstall(t)

	var createProjectCalled bool
	var topologyWritten string

	deps := testDeps("eddie", "repo")
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
	deps.FetchOwnerAndRepoIDs = func(owner, repo string) (string, string, error) {
		return "O_e", "R_r", nil
	}
	deps.CreateProject = func(ownerID, title string) (string, error) {
		createProjectCalled = true
		return "PVT_single", nil
	}
	deps.LinkRepoToProject = func(projectID, repoID string) error { return nil }
	deps.SetRepoVariable = func(o, r, n, v string) error {
		if n == TopologyVarName {
			topologyWritten = v
		}
		return nil
	}
	deps.Clone = func(repoURL, tag, destDir string) error { return nil }

	var buf bytes.Buffer
	err := Create(&buf, deps, CreateConfig{Title: "x", Version: "v2.0.10", Topology: "single"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !createProjectCalled {
		t.Error("expected CreateProject to be called for single-topology user owner")
	}
	if topologyWritten != "single" {
		t.Errorf("expected AGENTIC_TOPOLOGY=single, got %q", topologyWritten)
	}
}

func TestCreate_FederatedOrgOwner_Proceeds(t *testing.T) {
	// Baseline: federated + org owner must still work.
	tmp := t.TempDir()
	withFakeInstall(t)

	var topologyWritten string

	deps := testDeps("acme", "repo")
	deps.Root = tmp
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		return "", errors.New("not set")
	}
	deps.FetchProjectsForRepo = func(o, r string) ([]ProjectInfo, error) { return nil, nil }
	deps.DetectOwnerType = func(owner string) (string, error) { return "Organization", nil }
	deps.FetchOwnerAndRepoIDs = func(owner, repo string) (string, string, error) {
		return "O_a", "R_r", nil
	}
	deps.CreateProject = func(ownerID, title string) (string, error) { return "PVT_fed", nil }
	deps.LinkRepoToProject = func(projectID, repoID string) error { return nil }
	deps.SetRepoVariable = func(o, r, n, v string) error {
		if n == TopologyVarName {
			topologyWritten = v
		}
		return nil
	}
	deps.Clone = func(repoURL, tag, destDir string) error { return nil }

	var buf bytes.Buffer
	if err := Create(&buf, deps, CreateConfig{Title: "x", Version: "v2.0.10"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if topologyWritten != "federation" {
		t.Errorf("expected AGENTIC_TOPOLOGY=federation, got %q", topologyWritten)
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
