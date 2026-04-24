package init

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestRun_InvokesEnsureAppInstalledBeforeConfigure verifies the new step
// runs after mount and before ConfigureRepo — the ordering the pipeline
// depends on. We detect order by asserting that EnsureAppInstalled was
// called, that the captured config matches the one the caller provided,
// and that ConfigureRepo ran afterwards (evidenced by the "saved" lines).
func TestRun_InvokesEnsureAppInstalledBeforeConfigure(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, ".git"), 0o755)
	var buf bytes.Buffer

	cfg := &InitConfig{
		Version:       "v2.0.0",
		Topology:      "Single",
		AgentUser:     "goose-agent",
		RunnerLabel:   "ubuntu-latest",
		AgentProvider: "anthropic",
		AgentModel:    "claude-sonnet-4-6",
		RepoFullName:  "owner/repo",
		Owner:         "owner",
		RepoName:      "repo",
	}

	installerCalled := false
	var order []string

	deps := Deps{
		Run: func(name string, args ...string) (string, error) {
			if len(args) > 0 && args[0] == "variable" {
				order = append(order, "configure")
			}
			return "", nil
		},
		Clone:         fakeCloneFunc(),
		CollectConfig: fakeCollectConfig(cfg),
		EnsureAppInstalled: func(w io.Writer, gotCfg *InitConfig) error {
			installerCalled = true
			order = append(order, "install")
			if gotCfg != cfg {
				t.Errorf("expected EnsureAppInstalled to receive the collected cfg")
			}
			return nil
		},
	}

	if err := Run(&buf, root, false, deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !installerCalled {
		t.Fatalf("expected EnsureAppInstalled to run")
	}
	// The installer must precede any ConfigureRepo call. At least one
	// configure step exists because AGENT_USER et al. are non-empty.
	firstInstall := -1
	firstConfigure := -1
	for i, step := range order {
		if firstInstall == -1 && step == "install" {
			firstInstall = i
		}
		if firstConfigure == -1 && step == "configure" {
			firstConfigure = i
		}
	}
	if firstInstall == -1 {
		t.Fatalf("expected at least one install step in order; got %v", order)
	}
	if firstConfigure == -1 {
		t.Fatalf("expected at least one configure step in order; got %v", order)
	}
	if firstInstall > firstConfigure {
		t.Errorf("install step must precede configure; order=%v", order)
	}
}

// TestRun_NilEnsureAppInstalled_SkipsStep verifies the hook is optional.
// When deps.EnsureAppInstalled is nil, the wizard proceeds without
// invoking it — this is the behaviour the --skip-app-install flag wires
// into CLI callers.
func TestRun_NilEnsureAppInstalled_SkipsStep(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, ".git"), 0o755)
	var buf bytes.Buffer

	cfg := &InitConfig{
		Version:      "v2.0.0",
		RepoFullName: "owner/repo",
		Owner:        "owner",
		RepoName:     "repo",
	}

	deps := Deps{
		Run:           func(string, ...string) (string, error) { return "", nil },
		Clone:         fakeCloneFunc(),
		CollectConfig: fakeCollectConfig(cfg),
		// EnsureAppInstalled intentionally nil — simulates
		// --skip-app-install.
	}

	if err := Run(&buf, root, false, deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestRun_EnsureAppInstalledError_FailsWizard verifies a detection error
// inside the hook aborts Run with the error wrapped in a clear prefix.
// Unit tests for the four-path flow itself live in the githubapp package
// — here we only assert the wizard's integration behaviour.
func TestRun_EnsureAppInstalledError_FailsWizard(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, ".git"), 0o755)
	var buf bytes.Buffer

	cfg := &InitConfig{
		Version:      "v2.0.0",
		RepoFullName: "owner/repo",
		Owner:        "owner",
		RepoName:     "repo",
	}

	sentinel := errors.New("boom: detection failed")
	deps := Deps{
		Run:                func(string, ...string) (string, error) { return "", nil },
		Clone:              fakeCloneFunc(),
		CollectConfig:      fakeCollectConfig(cfg),
		EnsureAppInstalled: func(io.Writer, *InitConfig) error { return sentinel },
	}

	err := Run(&buf, root, false, deps)
	if err == nil {
		t.Fatalf("expected error from EnsureAppInstalled to fail wizard")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected error to wrap sentinel, got %v", err)
	}
}
