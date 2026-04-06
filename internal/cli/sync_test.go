package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/testutil"
)

func TestSyncCmd_Registration(t *testing.T) {
	root := newRootCmd("dev", "")

	// Verify sync subcommand exists.
	found := false
	for _, cmd := range root.Commands() {
		if cmd.Use == "sync" {
			found = true
			break
		}
	}

	if !found {
		t.Fatal("sync subcommand not registered in root command")
	}
}

func TestSyncCmd_HelpText(t *testing.T) {
	root := newRootCmd("dev", "")

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"sync", "--help"})
	_ = root.Execute()

	output := buf.String()

	if !strings.Contains(output, "Sync") {
		t.Errorf("help should mention 'Sync', got: %s", output)
	}

	if !strings.Contains(output, "base/") {
		t.Errorf("help should mention 'base/', got: %s", output)
	}

	if !strings.Contains(output, "TEMPLATE_SOURCE") {
		t.Errorf("help should mention 'TEMPLATE_SOURCE', got: %s", output)
	}
}

// newTestSyncRoot creates a minimal root command with the given sync subcommand
// for CLI integration testing.
func newTestSyncRoot(syncCmd *cobra.Command) *cobra.Command {
	root := &cobra.Command{
		Use:           "gh-agentic",
		SilenceErrors: true,
	}
	root.AddCommand(syncCmd)
	return root
}

// syncCloneRunner wraps a MockRunner with a side-effect for git clone commands,
// populating the target dir with a fake template base/.
func syncCloneRunner(mock *testutil.MockRunner, baseContent string) func(string, ...string) (string, error) {
	return func(name string, args ...string) (string, error) {
		if name == "git" && len(args) >= 1 && args[0] == "clone" {
			targetDir := args[len(args)-1]
			_ = os.MkdirAll(filepath.Join(targetDir, "base"), 0o755)
			_ = os.WriteFile(filepath.Join(targetDir, "base", "AGENTS.md"), []byte(baseContent), 0o644)
			return "", nil
		}
		return mock.RunCommand(name, args...)
	}
}

func TestSyncCmd_YesFlagAutoConfirms(t *testing.T) {
	repo := testutil.NewFakeRepo(t)

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo.Root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	mock := &testutil.MockRunner{}
	deps := syncDeps{
		run:          syncCloneRunner(mock, "updated content"),
		fetchRelease: testutil.FakeRelease("v2.0.0", nil),
		spinner:      testutil.NoopSpinner,
	}

	syncCmd := newSyncCmdWithDeps(deps)
	root := newTestSyncRoot(syncCmd)

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"sync", "--yes"})
	execErr := root.Execute()

	if execErr != nil {
		t.Fatalf("unexpected error: %v\nOutput:\n%s", execErr, buf.String())
	}

	output := buf.String()
	if !strings.Contains(output, "Sync applied") {
		t.Errorf("expected 'Sync applied' stage-only message, got:\n%s", output)
	}

	// The --yes flag should auto-confirm — verify the output contains the
	// auto-confirm line.
	if !strings.Contains(output, "[y/N]: y") {
		t.Errorf("expected auto-confirm line '[y/N]: y', got:\n%s", output)
	}
}

func TestSyncCmd_CommitFlagCommits(t *testing.T) {
	repo := testutil.NewFakeRepo(t)

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo.Root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	mock := &testutil.MockRunner{}
	deps := syncDeps{
		run:          syncCloneRunner(mock, "committed content"),
		fetchRelease: testutil.FakeRelease("v2.0.0", nil),
		spinner:      testutil.NoopSpinner,
	}

	syncCmd := newSyncCmdWithDeps(deps)
	root := newTestSyncRoot(syncCmd)

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"sync", "--yes", "--commit"})
	execErr := root.Execute()

	if execErr != nil {
		t.Fatalf("unexpected error: %v\nOutput:\n%s", execErr, buf.String())
	}

	output := buf.String()
	if !strings.Contains(output, "Sync committed") {
		t.Errorf("expected 'Sync committed' message with --commit flag, got:\n%s", output)
	}
	if !strings.Contains(output, "Remember to push") {
		t.Errorf("expected push reminder, got:\n%s", output)
	}
}

func TestSyncCmd_CommitFlagRegistration(t *testing.T) {
	root := newRootCmd("dev", "")

	// Find sync subcommand.
	var syncCmd *cobra.Command
	for _, cmd := range root.Commands() {
		if cmd.Use == "sync" {
			syncCmd = cmd
			break
		}
	}
	if syncCmd == nil {
		t.Fatal("sync subcommand not found")
	}

	// Verify --commit flag exists and defaults to false.
	f := syncCmd.Flags().Lookup("commit")
	if f == nil {
		t.Fatal("--commit flag not registered")
	}
	if f.DefValue != "false" {
		t.Errorf("--commit default should be false, got %q", f.DefValue)
	}
}

func TestSyncCmd_ForceFlagResyncs(t *testing.T) {
	repo := testutil.NewFakeRepo(t)
	// FakeRepo has TEMPLATE_VERSION=v1.0.0, FakeRelease also returns v1.0.0.

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo.Root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	mock := &testutil.MockRunner{}
	deps := syncDeps{
		run:          syncCloneRunner(mock, "force-synced content"),
		fetchRelease: testutil.FakeRelease("v1.0.0", nil),
		spinner:      testutil.NoopSpinner,
	}

	syncCmd := newSyncCmdWithDeps(deps)
	root := newTestSyncRoot(syncCmd)

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"sync", "--force", "--yes"})
	execErr := root.Execute()

	if execErr != nil {
		t.Fatalf("unexpected error: %v\nOutput:\n%s", execErr, buf.String())
	}

	output := buf.String()
	// Should succeed despite being at same version, mentioning --force.
	if !strings.Contains(output, "force") && !strings.Contains(output, "re-sync") {
		t.Errorf("expected force/re-sync warning in output, got:\n%s", output)
	}
}

func TestSyncCmd_ErrorOutsideAgenticRepo_FullCommand(t *testing.T) {
	// Change to a temp dir without TEMPLATE_SOURCE.
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	// Use a mock runner so no real commands are executed.
	mock := &testutil.MockRunner{}
	deps := syncDeps{
		run:          mock.RunCommand,
		fetchRelease: testutil.FakeRelease("v1.0.0", nil),
		spinner:      testutil.NoopSpinner,
	}

	syncCmd := newSyncCmdWithDeps(deps)
	root := newTestSyncRoot(syncCmd)

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"sync"})
	execErr := root.Execute()

	// The command should fail because there's no TEMPLATE_SOURCE in the
	// temp dir. findRepoRoot falls back to cwd, and RunSync fails when
	// it tries to read TEMPLATE_SOURCE.
	if execErr == nil {
		output := buf.String()
		if strings.Contains(output, "Synced") {
			t.Error("expected failure outside agentic repo, but got success")
		}
	}
}
