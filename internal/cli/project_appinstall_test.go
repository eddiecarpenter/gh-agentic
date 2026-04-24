package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/eddiecarpenter/gh-agentic/internal/githubapp"
	"github.com/eddiecarpenter/gh-agentic/internal/project"
)

// withProjectFlow installs a fake Flow into the package-level
// projectAppInstallFlow var for the duration of the test. Restoring the
// production flow on cleanup keeps other tests deterministic.
func withProjectFlow(t *testing.T, fake *githubapp.Flow) {
	t.Helper()
	orig := projectAppInstallFlow
	projectAppInstallFlow = fake
	t.Cleanup(func() { projectAppInstallFlow = orig })
}

// fakeChecker is a minimal AppInstallChecker used by the CLI-level
// integration tests. It mirrors the shape of the one in the githubapp
// tests but is package-local to avoid exporting test helpers.
type fakeChecker struct {
	repoInstalled bool
	repoErr       error
	orgInstalled  bool
	orgErr        error
	repoCalls     int
	orgCalls      int
}

func (f *fakeChecker) CheckRepoInstallation(ctx context.Context, owner, repo string) (bool, int64, error) {
	f.repoCalls++
	if f.repoErr != nil {
		return false, 0, f.repoErr
	}
	if f.repoInstalled {
		return true, 123, nil
	}
	return false, 0, nil
}

func (f *fakeChecker) CheckOrgInstallation(ctx context.Context, org string) (bool, int64, error) {
	f.orgCalls++
	if f.orgErr != nil {
		return false, 0, f.orgErr
	}
	if f.orgInstalled {
		return true, 456, nil
	}
	return false, 0, nil
}

func TestRunAppInstallStep_Skipped_NoCalls(t *testing.T) {
	checker := &fakeChecker{}
	withProjectFlow(t, &githubapp.Flow{
		Checker: checker,
		Slug:    "gh-agentic-app",
		IsCI:    func() bool { return true },
	})

	deps := fakeProjectDeps("owner", "repo")
	err := runAppInstallStep(context.Background(), io.Discard, deps, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if checker.orgCalls+checker.repoCalls != 0 {
		t.Errorf("expected zero checks when skip=true; got org=%d repo=%d", checker.orgCalls, checker.repoCalls)
	}
}

func TestRunAppInstallStep_NilFlow_NoOp(t *testing.T) {
	withProjectFlow(t, nil)
	deps := fakeProjectDeps("owner", "repo")
	if err := runAppInstallStep(context.Background(), io.Discard, deps, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunAppInstallStep_OrgOwner_UsesOrgEndpoint(t *testing.T) {
	// Headless environment so the flow does not block. fakeProjectDeps
	// uses DetectOwnerType = Organization.
	checker := &fakeChecker{orgInstalled: true}
	withProjectFlow(t, &githubapp.Flow{
		Checker: checker,
		Slug:    "gh-agentic-app",
		IsCI:    func() bool { return true },
	})

	deps := fakeProjectDeps("acme", "repo")
	var buf bytes.Buffer
	if err := runAppInstallStep(context.Background(), &buf, deps, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if checker.orgCalls == 0 {
		t.Errorf("expected org endpoint to be used for Organization owner; org=%d repo=%d", checker.orgCalls, checker.repoCalls)
	}
	if !strings.Contains(buf.String(), "already installed on acme") {
		t.Errorf("expected skip message for installed org; got %q", buf.String())
	}
}

func TestRunAppInstallStep_UserOwner_UsesRepoEndpoint(t *testing.T) {
	checker := &fakeChecker{repoInstalled: true}
	withProjectFlow(t, &githubapp.Flow{
		Checker: checker,
		Slug:    "gh-agentic-app",
		IsCI:    func() bool { return true },
	})

	deps := fakeProjectDeps("eddie", "tools")
	deps.DetectOwnerType = func(string) (string, error) { return "User", nil }

	var buf bytes.Buffer
	if err := runAppInstallStep(context.Background(), &buf, deps, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if checker.repoCalls == 0 {
		t.Errorf("expected repo endpoint to be used for User owner; org=%d repo=%d", checker.orgCalls, checker.repoCalls)
	}
	if !strings.Contains(buf.String(), "already installed on eddie/tools") {
		t.Errorf("expected skip message for installed repo; got %q", buf.String())
	}
}

func TestRunAppInstallStep_DetectOwnerTypeError_FallsBackToRepo(t *testing.T) {
	// The conservative fallback: when owner-type detection fails we
	// pick the repo-level endpoint so we don't inadvertently ask the
	// user to install org-wide.
	checker := &fakeChecker{repoInstalled: true}
	withProjectFlow(t, &githubapp.Flow{
		Checker: checker,
		Slug:    "gh-agentic-app",
		IsCI:    func() bool { return true },
	})

	deps := fakeProjectDeps("mystery", "repo")
	deps.DetectOwnerType = func(string) (string, error) { return "", errors.New("lookup failed") }

	if err := runAppInstallStep(context.Background(), io.Discard, deps, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if checker.orgCalls != 0 {
		t.Errorf("expected zero org calls when detect fails; got %d", checker.orgCalls)
	}
	if checker.repoCalls == 0 {
		t.Errorf("expected repo call as fallback")
	}
}

func TestRunAppInstallStep_HeadlessMissingApp_PrintsURLAndContinues(t *testing.T) {
	checker := &fakeChecker{orgInstalled: false}
	withProjectFlow(t, &githubapp.Flow{
		Checker: checker,
		Slug:    "gh-agentic-app",
		IsCI:    func() bool { return true },
	})

	deps := fakeProjectDeps("acme", "repo")
	var buf bytes.Buffer
	if err := runAppInstallStep(context.Background(), &buf, deps, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "https://github.com/apps/gh-agentic-app/installations/new") {
		t.Errorf("expected install URL to be printed in headless mode; got %q", buf.String())
	}
}

func TestRunAppInstallStep_InteractiveDecline_PrintsURLAndContinues(t *testing.T) {
	checker := &fakeChecker{}
	withProjectFlow(t, &githubapp.Flow{
		Checker: checker,
		Slug:    "gh-agentic-app",
		IsCI:    func() bool { return false },
		Confirm: func(string, string) (bool, error) { return false, nil },
	})

	deps := fakeProjectDeps("acme", "repo")
	var buf bytes.Buffer
	if err := runAppInstallStep(context.Background(), &buf, deps, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "when ready") {
		t.Errorf("expected decline-with-url message; got %q", buf.String())
	}
}

func TestRunAppInstallStep_InteractiveAccept_InvokesBrowser(t *testing.T) {
	// Arrange for the second check to report installed so we don't
	// stall on the pending branch.
	checker := &checkerWithSequence{results: []bool{false, true}}
	var openedURL string
	withProjectFlow(t, &githubapp.Flow{
		Checker: checker,
		Slug:    "gh-agentic-app",
		IsCI:    func() bool { return false },
		Confirm: func(string, string) (bool, error) { return true, nil },
		OpenURL: func(u string) error { openedURL = u; return nil },
	})

	deps := fakeProjectDeps("acme", "repo")
	var buf bytes.Buffer
	if err := runAppInstallStep(context.Background(), &buf, deps, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if openedURL == "" {
		t.Fatalf("expected OpenURL to be called in interactive-accept path")
	}
	if !strings.Contains(openedURL, "apps/gh-agentic-app") {
		t.Errorf("expected install URL in %q", openedURL)
	}
}

// checkerWithSequence serves an ordered sequence of installed outcomes
// for repo / org checks. Used to simulate the interactive re-check
// transition from not-installed to installed.
type checkerWithSequence struct {
	results   []bool
	callIndex int
}

func (c *checkerWithSequence) next() bool {
	if c.callIndex >= len(c.results) {
		return false
	}
	v := c.results[c.callIndex]
	c.callIndex++
	return v
}

func (c *checkerWithSequence) CheckRepoInstallation(ctx context.Context, owner, repo string) (bool, int64, error) {
	return c.next(), 0, nil
}
func (c *checkerWithSequence) CheckOrgInstallation(ctx context.Context, org string) (bool, int64, error) {
	return c.next(), 0, nil
}

func TestResolveJoinTarget_NoDetectFn_FallsBackToRepo(t *testing.T) {
	deps := project.Deps{Owner: "eddie", RepoName: "tools"}
	got := resolveJoinTarget(deps)
	if got.Type != githubapp.TargetRepo || got.Owner != "eddie" || got.Repo != "tools" {
		t.Errorf("expected TargetRepo fallback, got %+v", got)
	}
}

func TestProjectJoinCmd_HasSkipAppInstallFlag(t *testing.T) {
	cmd := newProjectJoinCmd()
	f := cmd.Flags().Lookup("skip-app-install")
	if f == nil {
		t.Fatalf("expected --skip-app-install flag on project join")
	}
	if f.Value.Type() != "bool" {
		t.Errorf("expected bool flag, got %s", f.Value.Type())
	}
}
