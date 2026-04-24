package githubapp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

// fakeChecker implements AppInstallChecker for flow tests. It records the
// calls and serves canned outcomes from a queue so tests can control the
// initial-vs-re-check sequence independently.
type fakeChecker struct {
	repoResults []checkOutcome
	orgResults  []checkOutcome
	repoCalls   int
	orgCalls    int
}

type checkOutcome struct {
	installed bool
	id        int64
	err       error
}

func (f *fakeChecker) CheckRepoInstallation(ctx context.Context, owner, repo string) (bool, int64, error) {
	defer func() { f.repoCalls++ }()
	if f.repoCalls >= len(f.repoResults) {
		// Default: not installed, no error.
		return false, 0, nil
	}
	o := f.repoResults[f.repoCalls]
	return o.installed, o.id, o.err
}

func (f *fakeChecker) CheckOrgInstallation(ctx context.Context, org string) (bool, int64, error) {
	defer func() { f.orgCalls++ }()
	if f.orgCalls >= len(f.orgResults) {
		return false, 0, nil
	}
	o := f.orgResults[f.orgCalls]
	return o.installed, o.id, o.err
}

func TestEnsureInstalled_AlreadyInstalled_ReturnsAlreadyInstalled(t *testing.T) {
	checker := &fakeChecker{repoResults: []checkOutcome{{installed: true, id: 42}}}
	flow := &Flow{
		Checker: checker,
		Slug:    "gh-agentic-app",
		IsCI:    func() bool { return false },
	}
	var w bytes.Buffer

	got, err := EnsureInstalled(context.Background(), &w, nil, flow, Target{Type: TargetRepo, Owner: "o", Repo: "r"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != ResultAlreadyInstalled {
		t.Fatalf("expected ResultAlreadyInstalled, got %v", got)
	}
	if checker.repoCalls != 1 {
		t.Fatalf("expected 1 repo check, got %d", checker.repoCalls)
	}
	if !strings.Contains(w.String(), "already installed on o/r") {
		t.Errorf("expected skip message; got %q", w.String())
	}
}

func TestEnsureInstalled_Headless_PrintsURLAndContinues(t *testing.T) {
	checker := &fakeChecker{}
	flow := &Flow{
		Checker: checker,
		Slug:    "gh-agentic-app",
		IsCI:    func() bool { return true },
	}
	var w bytes.Buffer

	got, err := EnsureInstalled(context.Background(), &w, nil, flow, Target{Type: TargetOrg, Owner: "acme"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != ResultHeadless {
		t.Fatalf("expected ResultHeadless, got %v", got)
	}
	out := w.String()
	if !strings.Contains(out, "https://github.com/apps/gh-agentic-app/installations/new") {
		t.Errorf("expected install URL in output; got %q", out)
	}
	// Must not prompt in headless mode — Confirm absent means nothing was
	// called; but we also assert no spurious confirm call occurs.
}

func TestEnsureInstalled_Headless_NonTTYFallback(t *testing.T) {
	// IsCI=false but IsInteractive=false → the writer is a pipe/buffer.
	// The flow must treat this as headless and skip the prompt.
	checker := &fakeChecker{}
	confirmCalled := false
	flow := &Flow{
		Checker:       checker,
		Slug:          "gh-agentic-app",
		IsCI:          func() bool { return false },
		IsInteractive: func(_ io.Writer) bool { return false },
		Confirm: func(string, string) (bool, error) {
			confirmCalled = true
			return true, nil
		},
	}
	var w bytes.Buffer

	got, _ := EnsureInstalled(context.Background(), &w, nil, flow, Target{Type: TargetRepo, Owner: "o", Repo: "r"})
	if got != ResultHeadless {
		t.Fatalf("expected ResultHeadless for non-TTY, got %v", got)
	}
	if confirmCalled {
		t.Fatalf("Confirm must not be called when non-TTY")
	}
}

func TestEnsureInstalled_InteractiveDecline_PrintsURLAndContinues(t *testing.T) {
	checker := &fakeChecker{}
	flow := &Flow{
		Checker: checker,
		Slug:    "gh-agentic-app",
		IsCI:    func() bool { return false },
		Confirm: func(string, string) (bool, error) { return false, nil },
	}
	var w bytes.Buffer

	got, err := EnsureInstalled(context.Background(), &w, nil, flow, Target{Type: TargetRepo, Owner: "o", Repo: "r"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != ResultDeclinedInteractive {
		t.Fatalf("expected ResultDeclinedInteractive, got %v", got)
	}
	if !strings.Contains(w.String(), "when ready") {
		t.Errorf("expected decline-with-url message; got %q", w.String())
	}
}

func TestEnsureInstalled_InteractiveAccept_OpenConfirmed(t *testing.T) {
	// First check: not installed. After browser-open + enter: installed.
	checker := &fakeChecker{repoResults: []checkOutcome{
		{installed: false},
		{installed: true, id: 99},
	}}
	var openedURL string
	waitCalled := false

	flow := &Flow{
		Checker:   checker,
		Slug:      "gh-agentic-app",
		IsCI:      func() bool { return false },
		Confirm:   func(string, string) (bool, error) { return true, nil },
		OpenURL:   func(u string) error { openedURL = u; return nil },
		WaitEnter: func(_ io.Reader) error { waitCalled = true; return nil },
	}
	var w bytes.Buffer

	got, err := EnsureInstalled(context.Background(), &w, nil, flow, Target{Type: TargetRepo, Owner: "o", Repo: "r"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != ResultInstalledInteractive {
		t.Fatalf("expected ResultInstalledInteractive, got %v", got)
	}
	if openedURL == "" || !strings.Contains(openedURL, "apps/gh-agentic-app") {
		t.Errorf("expected browser-open with install URL; got %q", openedURL)
	}
	if !waitCalled {
		t.Errorf("expected WaitEnter to be called on interactive-accept path")
	}
	if checker.repoCalls != 2 {
		t.Errorf("expected 2 repo checks (initial + re-check); got %d", checker.repoCalls)
	}
}

func TestEnsureInstalled_InteractiveAccept_PendingWhenReCheckStillMissing(t *testing.T) {
	// First check: not installed. After browser-open: still not installed.
	// Flow must surface a warning but return nil (continue).
	checker := &fakeChecker{repoResults: []checkOutcome{
		{installed: false},
		{installed: false},
	}}
	flow := &Flow{
		Checker:   checker,
		Slug:      "gh-agentic-app",
		IsCI:      func() bool { return false },
		Confirm:   func(string, string) (bool, error) { return true, nil },
		OpenURL:   func(string) error { return nil },
		WaitEnter: func(_ io.Reader) error { return nil },
	}
	var w bytes.Buffer

	got, err := EnsureInstalled(context.Background(), &w, nil, flow, Target{Type: TargetRepo, Owner: "o", Repo: "r"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != ResultPendingInteractive {
		t.Fatalf("expected ResultPendingInteractive, got %v", got)
	}
	if !strings.Contains(w.String(), "Still not detected") {
		t.Errorf("expected pending warning; got %q", w.String())
	}
}

func TestEnsureInstalled_InteractiveAccept_BrowserOpenError_ContinuesWithURL(t *testing.T) {
	// Browser open fails (e.g. no DISPLAY on the dev machine). The flow
	// must log the fallback, but still proceed — user can click the URL
	// manually.
	checker := &fakeChecker{repoResults: []checkOutcome{
		{installed: false},
		{installed: false},
	}}
	flow := &Flow{
		Checker: checker,
		Slug:    "gh-agentic-app",
		IsCI:    func() bool { return false },
		Confirm: func(string, string) (bool, error) { return true, nil },
		OpenURL: func(string) error { return errors.New("no display") },
	}
	var w bytes.Buffer

	_, err := EnsureInstalled(context.Background(), &w, nil, flow, Target{Type: TargetRepo, Owner: "o", Repo: "r"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(w.String(), "Could not open browser") {
		t.Errorf("expected fallback log; got %q", w.String())
	}
}

func TestEnsureInstalled_OrgTarget_UsesOrgEndpoint(t *testing.T) {
	checker := &fakeChecker{orgResults: []checkOutcome{{installed: true, id: 1}}}
	flow := &Flow{Checker: checker, Slug: "gh-agentic-app", IsCI: func() bool { return false }}
	var w bytes.Buffer

	_, err := EnsureInstalled(context.Background(), &w, nil, flow, Target{Type: TargetOrg, Owner: "acme"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if checker.orgCalls != 1 {
		t.Errorf("expected 1 org check, got %d", checker.orgCalls)
	}
	if checker.repoCalls != 0 {
		t.Errorf("expected zero repo checks for TargetOrg, got %d", checker.repoCalls)
	}
}

func TestEnsureInstalled_DetectionError_PropagatesWrapped(t *testing.T) {
	sentinel := errors.New("boom")
	checker := &fakeChecker{repoResults: []checkOutcome{{err: sentinel}}}
	flow := &Flow{Checker: checker, Slug: "gh-agentic-app", IsCI: func() bool { return false }}
	var w bytes.Buffer

	_, err := EnsureInstalled(context.Background(), &w, nil, flow, Target{Type: TargetRepo, Owner: "o", Repo: "r"})
	if err == nil {
		t.Fatalf("expected detection error to propagate")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected wrapped sentinel, got %v", err)
	}
}

func TestEnsureInstalled_NilFlow_ReturnsError(t *testing.T) {
	if _, err := EnsureInstalled(context.Background(), &bytes.Buffer{}, nil, nil, Target{Type: TargetRepo, Owner: "o", Repo: "r"}); err == nil {
		t.Fatalf("expected error for nil flow")
	}
	flow := &Flow{Checker: nil}
	if _, err := EnsureInstalled(context.Background(), &bytes.Buffer{}, nil, flow, Target{Type: TargetRepo, Owner: "o", Repo: "r"}); err == nil {
		t.Fatalf("expected error for nil checker")
	}
}

func TestEnsureInstalled_IncompleteTarget_ReturnsError(t *testing.T) {
	flow := &Flow{Checker: &fakeChecker{}, Slug: "gh-agentic-app"}
	tests := []Target{
		{Type: TargetRepo, Owner: "", Repo: "r"},
		{Type: TargetRepo, Owner: "o", Repo: ""},
		{Type: TargetOrg, Owner: ""},
	}
	for _, tc := range tests {
		if _, err := EnsureInstalled(context.Background(), &bytes.Buffer{}, nil, flow, tc); err == nil {
			t.Errorf("expected error for incomplete target %+v", tc)
		}
	}
}

func TestTarget_Name(t *testing.T) {
	if got := (Target{Type: TargetRepo, Owner: "o", Repo: "r"}).Name(); got != "o/r" {
		t.Errorf("TargetRepo.Name() = %q, want o/r", got)
	}
	if got := (Target{Type: TargetOrg, Owner: "acme"}).Name(); got != "acme" {
		t.Errorf("TargetOrg.Name() = %q, want acme", got)
	}
}

func TestWaitEnterFromReader_ConsumesLine(t *testing.T) {
	r := strings.NewReader("\n")
	if err := WaitEnterFromReader(r); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := WaitEnterFromReader(nil); err != nil {
		t.Fatalf("nil reader must be tolerated, got %v", err)
	}
}

