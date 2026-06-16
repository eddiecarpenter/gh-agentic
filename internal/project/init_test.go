package project

import (
	"bytes"
	"errors"
	"testing"

	initpkg "github.com/eddiecarpenter/gh-agentic/internal/init"
)

// TestInitRepo_FederatedUserOwner_RefusesWithVerbatimError verifies the
// federated-owner guard fires at the project-level entry point for init —
// before any side effect (project mount, variable writes, ConfigureRepo).
func TestInitRepo_FederatedUserOwner_RefusesWithVerbatimError(t *testing.T) {
	var projectIDWritten bool

	deps := testDeps("eddie", "repo")
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		// Repo is not yet affiliated so the "already affiliated" guard
		// does not short-circuit ahead of the federated guard.
		return "", errors.New("not set")
	}
	deps.SetRepoVariable = func(o, r, n, v string) error {
		if n == ProjectVarName {
			projectIDWritten = true
		}
		return nil
	}
	deps.DetectOwnerType = func(owner string) (string, error) {
		return "User", nil
	}

	var buf bytes.Buffer
	err := InitRepo(&buf, deps, InitRepoConfig{
		Mode: InitModeFederated,
	})
	if err == nil {
		t.Fatal("expected error for federated init on user-owned repo")
	}
	want := "Federated topology requires a GitHub Organization. The owner 'eddie' is a user account, which cannot host org-scoped variables and secrets. Either move this repo under an organisation, or use `--topology single`."
	if err.Error() != want {
		t.Fatalf("error mismatch:\ngot:  %q\nwant: %q", err.Error(), want)
	}
	if projectIDWritten {
		t.Error("SetRepoVariable(AGENTIC_PROJECT_ID) must not be called when guard refuses")
	}
}

// TestInitRepo_SingleUserOwner_DoesNotRefuse verifies the guard is
// federated-only and does not accidentally block single-topology init on a
// user-owned repo.
func TestInitRepo_SingleUserOwner_DoesNotRefuse(t *testing.T) {
	withFakeInstall(t)
	deps := testDeps("eddie", "repo")
	deps.Root = t.TempDir()
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		return "", errors.New("not set")
	}
	deps.FetchProjectsForRepo = func(o, r string) ([]ProjectInfo, error) {
		return nil, nil
	}
	deps.DetectOwnerType = func(owner string) (string, error) {
		return "User", nil
	}
	// Allow the rest of the Single path to succeed.
	deps.FetchOwnerAndRepoIDs = func(owner, repo string) (string, string, error) {
		return "O_e", "R_r", nil
	}
	deps.CreateProject = func(ownerID, title string) (string, error) {
		return "PVT_single", nil
	}
	deps.LinkRepoToProject = func(projectID, repoID string) error { return nil }
	deps.SetRepoVariable = func(o, r, n, v string) error { return nil }
	deps.Clone = func(repoURL, tag, destDir string) error { return nil }

	var buf bytes.Buffer
	// Supply a minimal InitCfg so initSingle can dereference it; leave
	// RepoFullName empty so ConfigureRepo is a no-op (no Run shelling out).
	err := InitRepo(&buf, deps, InitRepoConfig{
		Mode:    InitModeSingle,
		InitCfg: &initpkg.InitConfig{Version: "v2.0.10"},
	})
	if err != nil {
		t.Fatalf("unexpected error for single init on user-owned repo: %v", err)
	}
}

// TestInitRepo_FederatedOrgOwner_CreatesControlPlane verifies that federated
// init on an org-owned repo creates a federated control plane (#875): the org
// guard does not fire, and an empty FEDERATION.md is scaffolded.
func TestInitRepo_FederatedOrgOwner_CreatesControlPlane(t *testing.T) {
	withFakeInstall(t)
	deps := testDeps("acme", "repo")
	deps.Root = t.TempDir()
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		return "", errors.New("not set") // not yet affiliated
	}
	deps.FetchProjectsForRepo = func(o, r string) ([]ProjectInfo, error) { return nil, nil }
	deps.DetectOwnerType = func(owner string) (string, error) { return "Organization", nil }
	deps.FetchOwnerAndRepoIDs = func(owner, repo string) (string, string, error) { return "O_acme", "R_repo", nil }
	deps.CreateProject = func(ownerID, title string) (string, error) { return "PVT_cp", nil }
	deps.LinkRepoToProject = func(projectID, repoID string) error { return nil }
	deps.SetRepoVariable = func(o, r, n, v string) error { return nil }
	deps.Clone = func(repoURL, tag, destDir string) error { return nil }

	var buf bytes.Buffer
	// No RepoFullName → ConfigureRepo is a no-op; Version drives the create.
	err := InitRepo(&buf, deps, InitRepoConfig{
		Mode:    InitModeFederated,
		InitCfg: &initpkg.InitConfig{Version: "v2.0.10"},
	})
	if err != nil {
		t.Fatalf("federated control-plane init should succeed for an org owner, got: %v", err)
	}
	// A federated control plane scaffolds an empty-but-valid FEDERATION.md.
	if !IsFederationRepo(deps.Root) {
		t.Error("expected FEDERATION.md to be scaffolded for a federated control plane")
	}
}
