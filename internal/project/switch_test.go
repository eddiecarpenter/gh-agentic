package project

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/eddiecarpenter/gh-agentic/internal/mount"
)

// TestSwitchVersion_SetsFrameworkVersionVariable_SingleTopology covers the
// v2.3.0 regression where `gh agentic upgrade` on a single-topology repo
// updated the .ai/ mount and the workflow uses: refs but left
// AGENTIC_FRAMEWORK_VERSION at the old value. Drift between the variable
// and the mounted tree is immediately visible to `check` and to the
// pipeline's own resolve-version step, which reads the variable. The
// fix makes variable-writing unconditional — `create` and `repair`
// already did this; `SwitchVersion` is now aligned.
//
// This test freezes the expected behaviour so the regression cannot
// silently return.
func TestSwitchVersion_SetsFrameworkVersionVariable_SingleTopology(t *testing.T) {
	root := t.TempDir()

	// Seed a fake mounted .ai/ at v2.2.6 as a tracked submodule, so
	// RunSwitch's DownloadFramework dispatch sees MountStateSubmodule
	// and routes to the swap path.
	aiDir := filepath.Join(root, ".ai")
	if err := os.MkdirAll(aiDir, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitmodules"),
		[]byte(`[submodule ".ai"]`+"\n\turl = "+mount.FrameworkRepoURL+"\n"), 0o644); err != nil {
		t.Fatalf("seed .gitmodules: %v", err)
	}
	withFakeSwap(t)

	// Capture the variables the code writes.
	writes := make(map[string]string)
	reads := map[string]string{
		TopologyVarName: "", // single topology — variable unset
	}

	deps := Deps{
		Root:         root,
		Owner:        "eddiecarpenter",
		RepoName:     "ocs-testbench",
		RepoFullName: "eddiecarpenter/ocs-testbench",
		GetRepoVariable: func(owner, repo, name string) (string, error) {
			return reads[name], nil
		},
		SetRepoVariable: func(owner, repo, name, value string) error {
			writes[name] = value
			return nil
		},
		// No-op clone — the test doesn't exercise mount.RunSwitch's
		// clone logic, only the post-clone variable write. The stub
		// lets RunSwitch believe the clone succeeded.
		Clone: func(repoURL, tag, destDir string) error {
			return os.MkdirAll(destDir, 0o755)
		},
	}

	// CurrentVersion != version and IsFederatedCP: false — the exact
	// case that was broken in v2.3.0.
	pre := SwitchVersionPreflight{
		CurrentVersion: "v2.2.6",
		IsFederatedCP:  false,
	}

	var buf bytes.Buffer
	if err := SwitchVersion(&buf, deps, "v2.3.0", pre, nil); err != nil {
		// RunSwitch may fail if the stub clone does not produce the
		// expected workflow file structure; tolerate that as long as
		// the variable write happened (the bug was the write being
		// skipped, not the clone).
		t.Logf("SwitchVersion returned error (possibly from mount layer): %v", err)
	}

	if got := writes[FrameworkVersionVarName]; got != "v2.3.0" {
		t.Errorf("%s variable must be updated on single-topology upgrade, got %q (want v2.3.0)",
			FrameworkVersionVarName, got)
	}
}

// TestSwitchVersion_SetsFrameworkVersionVariable_AlreadyAtTarget covers
// the "already at version" path — variable must still be written if it
// disagrees with the mounted version. This is the recovery case for a
// repo whose .ai/ was mounted at the right version but whose variable
// was never set (e.g. manual mount).
func TestSwitchVersion_SetsFrameworkVersionVariable_AlreadyAtTarget(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".ai"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	writes := make(map[string]string)
	// Variable is absent — the bug case: user's repo has `.ai/` at
	// v2.3.0 but AGENTIC_FRAMEWORK_VERSION has never been set.
	reads := map[string]string{}

	deps := Deps{
		Root:         root,
		Owner:        "eddiecarpenter",
		RepoName:     "ocs-testbench",
		RepoFullName: "eddiecarpenter/ocs-testbench",
		GetRepoVariable: func(owner, repo, name string) (string, error) {
			return reads[name], nil
		},
		SetRepoVariable: func(owner, repo, name, value string) error {
			writes[name] = value
			return nil
		},
	}

	pre := SwitchVersionPreflight{
		CurrentVersion: "v2.3.0",
		IsFederatedCP:  false,
	}

	var buf bytes.Buffer
	if err := SwitchVersion(&buf, deps, "v2.3.0", pre, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := writes[FrameworkVersionVarName]; got != "v2.3.0" {
		t.Errorf("%s must be backfilled when mount already matches target, got %q",
			FrameworkVersionVarName, got)
	}
}

// TestSwitchVersion_SkipsVariableWrite_WhenAlreadyCorrect confirms the
// guard against needless writes: if the variable is already at the
// target value, no SetRepoVariable call is made. This is purely a
// spam-reduction check so the output doesn't claim to have changed
// something it hasn't.
func TestSwitchVersion_SkipsVariableWrite_WhenAlreadyCorrect(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".ai"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	writes := make(map[string]string)
	reads := map[string]string{
		FrameworkVersionVarName: "v2.3.0",
	}

	deps := Deps{
		Root:         root,
		Owner:        "eddiecarpenter",
		RepoName:     "ocs-testbench",
		RepoFullName: "eddiecarpenter/ocs-testbench",
		GetRepoVariable: func(owner, repo, name string) (string, error) {
			return reads[name], nil
		},
		SetRepoVariable: func(owner, repo, name, value string) error {
			writes[name] = value
			return nil
		},
	}

	pre := SwitchVersionPreflight{CurrentVersion: "v2.3.0", IsFederatedCP: false}

	var buf bytes.Buffer
	if err := SwitchVersion(&buf, deps, "v2.3.0", pre, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, wrote := writes[FrameworkVersionVarName]; wrote {
		t.Errorf("variable must not be rewritten when already at target; got write of %q",
			writes[FrameworkVersionVarName])
	}
}

// Compile-time assertion that the mount package is reachable from the
// test binary — if this breaks, the import above may have been optimised
// out by a future refactor and the test would silently stop covering
// the SwitchVersion → RunSwitch path.
var _ = mount.FrameworkRepo
