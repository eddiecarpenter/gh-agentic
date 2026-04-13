package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/sync"
	"github.com/eddiecarpenter/gh-agentic/internal/testutil"
)

// setupFakeTarball overrides the sync package's fetch function with a fake
// that returns a gzipped tar containing .ai/RULEBOOK.md with the given content.
// Returns a cleanup function that restores the original fetch function.
func setupFakeTarball(t *testing.T, baseContent string) func() {
	t.Helper()
	files := map[string]string{
		".ai/RULEBOOK.md": baseContent,
		".ai/config.yml":  "template: eddiecarpenter/ai-native-delivery\nversion: v0.0.0\n",
	}

	prev := sync.SetFetchTarballFn(func(repo, version string) (io.ReadCloser, error) {
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gw)

		prefix := "repo-" + version + "/"
		_ = tw.WriteHeader(&tar.Header{
			Name: prefix, Typeflag: tar.TypeDir, Mode: 0o755,
		})

		for path, content := range files {
			fullPath := prefix + path
			// Create parent dirs.
			dir := filepath.Dir(path)
			if dir != "." {
				_ = tw.WriteHeader(&tar.Header{
					Name: prefix + dir + "/", Typeflag: tar.TypeDir, Mode: 0o755,
				})
			}
			_ = tw.WriteHeader(&tar.Header{
				Name: fullPath, Size: int64(len(content)),
				Mode: 0o644, Typeflag: tar.TypeReg,
			})
			_, _ = tw.Write([]byte(content))
		}

		_ = tw.Close()
		_ = gw.Close()
		return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
	})

	return func() { sync.SetFetchTarballFn(prev) }
}

// fakeCLIReleases returns a FetchReleasesFunc that returns a single release
// with the given tag. Used by CLI-level tests.
func fakeCLIReleases(tag string) sync.FetchReleasesFunc {
	return func(_ string) ([]sync.Release, error) {
		return []sync.Release{
			{TagName: tag, Name: "Release " + tag, Body: "Release notes for " + tag, TarballURL: "https://api.github.com/repos/owner/repo/tarball/" + tag},
		}, nil
	}
}

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

	if !strings.Contains(output, ".ai/") {
		t.Errorf("help should mention '.ai/', got: %s", output)
	}

	if !strings.Contains(output, "config.yml") {
		t.Errorf("help should mention 'config.yml', got: %s", output)
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
// populating the target dir with a fake template .ai/.
func syncCloneRunner(mock *testutil.MockRunner, baseContent string) func(string, ...string) (string, error) {
	return func(name string, args ...string) (string, error) {
		if name == "git" && len(args) >= 1 && args[0] == "clone" {
			targetDir := args[len(args)-1]
			_ = os.MkdirAll(filepath.Join(targetDir, ".ai"), 0o755)
			_ = os.WriteFile(filepath.Join(targetDir, ".ai", "RULEBOOK.md"), []byte(baseContent), 0o644)
			return "", nil
		}
		return mock.RunCommand(name, args...)
	}
}

func TestSyncCmd_YesFlagAutoConfirms(t *testing.T) {
	repo := testutil.NewFakeRepo(t)
	defer setupFakeTarball(t, "updated content")()

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
		run:          mock.RunCommand,
		fetchReleases: fakeCLIReleases("v2.0.0"),
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
	defer setupFakeTarball(t, "committed content")()

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
		run:          mock.RunCommand,
		fetchReleases: fakeCLIReleases("v2.0.0"),
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
	defer setupFakeTarball(t, "force-synced content")()
	// FakeRepo has .ai/config.yml version=v1.0.0, FakeRelease also returns v1.0.0.

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
		run:          mock.RunCommand,
		fetchReleases: fakeCLIReleases("v1.0.0"),
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
		fetchReleases: fakeCLIReleases("v1.0.0"),
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

func TestSyncCmd_ListFlagRegistration(t *testing.T) {
	root := newRootCmd("dev", "")

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

	f := syncCmd.Flags().Lookup("list")
	if f == nil {
		t.Fatal("--list flag not registered")
	}
	if f.DefValue != "false" {
		t.Errorf("--list default should be false, got %q", f.DefValue)
	}
}

func TestSyncCmd_ListDisplaysReleases(t *testing.T) {
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
		run: mock.RunCommand,
		fetchReleases: func(_ string) ([]sync.Release, error) {
			return []sync.Release{
				{TagName: "v2.0.0", Name: "Latest", Body: "Latest notes", TarballURL: "https://api.github.com/repos/owner/repo/tarball/v2.0.0"},
				{TagName: "v1.5.0", Name: "Middle", Body: "Middle notes", TarballURL: "https://api.github.com/repos/owner/repo/tarball/v1.5.0"},
			}, nil
		},
		spinner: testutil.NoopSpinner,
	}

	syncCmd := newSyncCmdWithDeps(deps)
	root := newTestSyncRoot(syncCmd)

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"sync", "--list"})
	execErr := root.Execute()

	if execErr != nil {
		t.Fatalf("unexpected error: %v\nOutput:\n%s", execErr, buf.String())
	}

	output := buf.String()
	if !strings.Contains(output, "v2.0.0") {
		t.Errorf("expected v2.0.0 in output, got:\n%s", output)
	}
	if !strings.Contains(output, "v1.5.0") {
		t.Errorf("expected v1.5.0 in output, got:\n%s", output)
	}
}

func TestSyncCmd_ListAndReleaseMutuallyExclusive(t *testing.T) {
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
		run:           mock.RunCommand,
		fetchReleases: fakeCLIReleases("v2.0.0"),
		spinner:       testutil.NoopSpinner,
	}

	syncCmd := newSyncCmdWithDeps(deps)
	root := newTestSyncRoot(syncCmd)

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"sync", "--list", "--release", "v1.0.0"})
	execErr := root.Execute()

	if execErr == nil {
		t.Fatal("expected error for mutually exclusive flags")
	}
	if !strings.Contains(execErr.Error(), "mutually exclusive") {
		t.Errorf("expected 'mutually exclusive' error, got: %v", execErr)
	}
}

func TestSyncCmd_ReleaseFlagRegistration(t *testing.T) {
	root := newRootCmd("dev", "")

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

	f := syncCmd.Flags().Lookup("release")
	if f == nil {
		t.Fatal("--release flag not registered")
	}
	if f.DefValue != "" {
		t.Errorf("--release default should be empty, got %q", f.DefValue)
	}
}

func TestSyncCmd_ReleaseSyncsToSpecificVersion(t *testing.T) {
	repo := testutil.NewFakeRepo(t)
	defer setupFakeTarball(t, "targeted content")()

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
		run: mock.RunCommand,
		fetchReleases: func(_ string) ([]sync.Release, error) {
			return []sync.Release{
				{TagName: "v2.0.0", Name: "Latest", Body: "Latest notes", TarballURL: "https://api.github.com/repos/owner/repo/tarball/v2.0.0"},
				{TagName: "v1.5.0", Name: "Middle", Body: "Middle notes", TarballURL: "https://api.github.com/repos/owner/repo/tarball/v1.5.0"},
			}, nil
		},
		spinner: testutil.NoopSpinner,
	}

	syncCmd := newSyncCmdWithDeps(deps)
	root := newTestSyncRoot(syncCmd)

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"sync", "--release", "v1.5.0", "--yes"})
	execErr := root.Execute()

	if execErr != nil {
		t.Fatalf("unexpected error: %v\nOutput:\n%s", execErr, buf.String())
	}

	output := buf.String()
	if !strings.Contains(output, "Sync applied") {
		t.Errorf("expected 'Sync applied' message, got:\n%s", output)
	}
}

func TestSyncCmd_DeprecationNotice(t *testing.T) {
	repo := testutil.NewFakeRepo(t)
	defer setupFakeTarball(t, "deprecation test")()

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
		run:           mock.RunCommand,
		fetchReleases: fakeCLIReleases("v2.0.0"),
		spinner:       testutil.NoopSpinner,
	}

	syncCmd := newSyncCmdWithDeps(deps)
	root := newTestSyncRoot(syncCmd)

	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"sync", "--yes"})
	_ = root.Execute()

	// The deprecation notice should be printed to stderr.
	errOutput := stderr.String()
	if !strings.Contains(errOutput, "Deprecated") {
		t.Errorf("expected deprecation notice in stderr, got: %q", errOutput)
	}
	if !strings.Contains(errOutput, "gh agentic -v2 mount") {
		t.Errorf("expected 'gh agentic -v2 mount' in deprecation notice, got: %q", errOutput)
	}
}

func TestSyncCmd_ReleaseNotFound(t *testing.T) {
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
		run: mock.RunCommand,
		fetchReleases: func(_ string) ([]sync.Release, error) {
			return []sync.Release{
				{TagName: "v2.0.0", Name: "Latest", Body: "Latest notes", TarballURL: "https://api.github.com/repos/owner/repo/tarball/v2.0.0"},
			}, nil
		},
		spinner: testutil.NoopSpinner,
	}

	syncCmd := newSyncCmdWithDeps(deps)
	root := newTestSyncRoot(syncCmd)

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"sync", "--release", "vX.Y.Z"})
	execErr := root.Execute()

	if execErr == nil {
		t.Fatal("expected error for non-existent release tag")
	}
	if !strings.Contains(execErr.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", execErr)
	}
}
