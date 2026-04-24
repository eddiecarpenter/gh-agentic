package cli

import (
	"strings"
	"testing"

	"github.com/eddiecarpenter/gh-agentic/internal/githubapp"
	initpkg "github.com/eddiecarpenter/gh-agentic/internal/init"
)

func TestBuildAppInstaller_SkipReturnsNilHook(t *testing.T) {
	// When --skip-app-install is true the installer must be nil so
	// initpkg.Run's nil-hook short-circuit does the right thing.
	got := buildAppInstaller(newInitCmd(), true)
	if got != nil {
		t.Fatalf("expected nil installer when skip=true; got %T", got)
	}
}

func TestTargetFromConfig_SingleTopologyProducesRepoTarget(t *testing.T) {
	cfg := &initpkg.InitConfig{
		Topology: "Single",
		Owner:    "eddie",
		RepoName: "tools",
	}
	got := targetFromConfig(cfg)
	if got.Type != githubapp.TargetRepo {
		t.Errorf("expected TargetRepo for Single topology, got %v", got.Type)
	}
	if got.Owner != "eddie" || got.Repo != "tools" {
		t.Errorf("unexpected target fields %+v", got)
	}
}

func TestTargetFromConfig_FederatedProducesOrgTarget(t *testing.T) {
	cfg := &initpkg.InitConfig{
		Topology: "Federated",
		Owner:    "acme",
		RepoName: "domain-repo",
	}
	got := targetFromConfig(cfg)
	if got.Type != githubapp.TargetOrg {
		t.Errorf("expected TargetOrg for Federated topology, got %v", got.Type)
	}
	if got.Owner != "acme" {
		t.Errorf("expected owner=acme, got %q", got.Owner)
	}
}

func TestTargetFromConfig_NilCfg_ReturnsEmpty(t *testing.T) {
	// A defensive path — the installer should bail on empty-owner
	// without calling into githubapp, and targetFromConfig supplies the
	// zero value that triggers that bail.
	got := targetFromConfig(nil)
	if got.Owner != "" {
		t.Errorf("expected zero-value target for nil cfg, got %+v", got)
	}
}

func TestTargetFromConfig_TopologyCaseInsensitive(t *testing.T) {
	// InitConfig.Topology carries the wizard's capitalised value. The
	// federated check must be case-insensitive so internal callers that
	// pass lowercase also route to the org target.
	cases := []string{"federated", "FEDERATED", "Federated"}
	for _, topo := range cases {
		cfg := &initpkg.InitConfig{Topology: topo, Owner: "acme"}
		if got := targetFromConfig(cfg); got.Type != githubapp.TargetOrg {
			t.Errorf("topology %q: expected TargetOrg, got %v", topo, got.Type)
		}
	}
}

func TestInitCmd_HasSkipAppInstallFlag(t *testing.T) {
	cmd := newInitCmd()
	f := cmd.Flags().Lookup("skip-app-install")
	if f == nil {
		t.Fatalf("expected --skip-app-install flag to be registered on init")
	}
	if f.Value.Type() != "bool" {
		t.Errorf("expected --skip-app-install to be bool, got %s", f.Value.Type())
	}
	if !strings.Contains(f.Usage, "App") {
		t.Errorf("expected flag usage to mention App; got %q", f.Usage)
	}
}
