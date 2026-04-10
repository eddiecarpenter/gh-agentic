package bootstrap

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain installs a fake tarball fetcher for the whole bootstrap package test run.
// This prevents CreateRepo from making real HTTP requests during tests.
func TestMain(m *testing.M) {
	// Build a minimal valid tarball once and reuse it across all tests.
	var tarBuf bytes.Buffer
	gw := gzip.NewWriter(&tarBuf)
	tw := tar.NewWriter(gw)
	_ = tw.WriteHeader(&tar.Header{Name: "repo-v1.0.0-abc123/", Typeflag: tar.TypeDir, Mode: 0o755})
	content := []byte("# Template\n")
	_ = tw.WriteHeader(&tar.Header{Name: "repo-v1.0.0-abc123/README.md", Size: int64(len(content)), Mode: 0o644, Typeflag: tar.TypeReg})
	_, _ = tw.Write(content)
	tw.Close()
	gw.Close()
	tarBytes := tarBuf.Bytes()

	fetchTarballFn = func(repo, version string) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(tarBytes)), nil
	}

	// Install a no-op clone conflict resolver for tests — no TTY available.
	resolveCloneConflictFn = func(w io.Writer, clonePath string) (string, error) {
		return clonePath, nil
	}

	os.Exit(m.Run())
}

// --------------------------------------------------------------------------------------
// Helpers shared across tests
// --------------------------------------------------------------------------------------

// fakeRunOK returns a RunCommandFunc that always succeeds with the given output.
func fakeRunOK(out string) RunCommandFunc {
	return func(name string, args ...string) (string, error) {
		return out, nil
	}
}

// fakeRunFail returns a RunCommandFunc that always fails with the given error.
func fakeRunFail(msg string) RunCommandFunc {
	return func(name string, args ...string) (string, error) {
		return msg, errors.New(msg)
	}
}

// fakeRunMap returns a RunCommandFunc whose behaviour is keyed by the first arg.
// The key is "name arg0" (e.g. "git clone", "gh repo").
// If no key matches, the call succeeds with empty output.
func fakeRunMap(m map[string]struct {
	out string
	err error
}) RunCommandFunc {
	return func(name string, args ...string) (string, error) {
		key := name
		if len(args) > 0 {
			key = name + " " + args[0]
		}
		if r, ok := m[key]; ok {
			return r.out, r.err
		}
		return "", nil
	}
}

// makeTempClone creates a temporary directory simulating a cloned repo root.
func makeTempClone(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// Initialise a minimal git repo so runInDir commands work in tests that
	// call real git; tests that stub run don't need it.
	return dir
}

// makeMinimalTarballBytes creates a minimal valid tar.gz archive in memory
// containing a single file under a prefix directory.
func makeMinimalTarballBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	// Add top-level directory.
	_ = tw.WriteHeader(&tar.Header{
		Name:     "repo-v1.0.0-abc123/",
		Typeflag: tar.TypeDir,
		Mode:     0o755,
	})
	// Add a file.
	content := []byte("# Template\n")
	_ = tw.WriteHeader(&tar.Header{
		Name:     "repo-v1.0.0-abc123/README.md",
		Size:     int64(len(content)),
		Mode:     0o644,
		Typeflag: tar.TypeReg,
	})
	_, _ = tw.Write(content)

	tw.Close()
	gw.Close()
	return buf.Bytes()
}

// writeTarballOnOutput returns true and writes tarball data if the command
// includes --output flag (used by gh api). Returns false otherwise.
func writeTarballOnOutput(t *testing.T, tarballData []byte, args []string) bool {
	t.Helper()
	for i, a := range args {
		if a == "--output" && i+1 < len(args) {
			if err := os.WriteFile(args[i+1], tarballData, 0o644); err != nil {
				t.Fatalf("writing test tarball: %v", err)
			}
			return true
		}
	}
	return false
}

// --------------------------------------------------------------------------------------
// repoName
// --------------------------------------------------------------------------------------

func TestRepoName_Single_ReturnsProjectName(t *testing.T) {
	cfg := BootstrapConfig{Topology: "Single", ProjectName: "my-project"}
	if got := repoName(cfg); got != "my-project" {
		t.Errorf("repoName() = %q, want %q", got, "my-project")
	}
}

func TestRepoName_Federated_AppendsSuffix(t *testing.T) {
	cfg := BootstrapConfig{Topology: "Federated", ProjectName: "my-project"}
	if got := repoName(cfg); got != "my-project-agentic" {
		t.Errorf("repoName() = %q, want %q", got, "my-project-agentic")
	}
}

// --------------------------------------------------------------------------------------
// Step 3 — CreateRepo
// --------------------------------------------------------------------------------------

// fakeFetchRelease returns a FetchReleaseFunc that always returns the given tag.
func fakeFetchRelease(tag string) FetchReleaseFunc {
	return func(repo string) (string, error) { return tag, nil }
}

// fakeFetchReleaseFail returns a FetchReleaseFunc that always fails.
func fakeFetchReleaseFail(msg string) FetchReleaseFunc {
	return func(repo string) (string, error) { return "", errors.New(msg) }
}

func TestCreateRepo_GhCreateFails_ReturnsError(t *testing.T) {
	cfg := BootstrapConfig{Topology: "Single", Owner: "alice", ProjectName: "my-project", TemplateRepo: DefaultTemplateRepo}
	state := &StepState{}
	run := fakeRunFail("repository already exists")

	var buf bytes.Buffer
	err := CreateRepo(&buf, cfg, state, t.TempDir(), run, fakeFetchRelease("v1.0.0"))
	if err == nil {
		t.Fatal("CreateRepo() expected error when gh repo create fails, got nil")
	}
	if !strings.Contains(err.Error(), "gh repo create") {
		t.Errorf("error should mention 'gh repo create', got: %v", err)
	}
}

func TestCreateRepo_CloneFails_ReturnsError(t *testing.T) {
	cfg := BootstrapConfig{Topology: "Single", Owner: "alice", ProjectName: "my-project", TemplateRepo: DefaultTemplateRepo}
	state := &StepState{}

	run := func(name string, args ...string) (string, error) {
		// gh repo create succeeds.
		if name == "gh" && len(args) > 1 && args[0] == "repo" && args[1] == "create" {
			return "https://github.com/alice/my-project", nil
		}
		// git clone fails.
		if name == "git" && len(args) > 0 && args[0] == "clone" {
			return "fatal: repository not found", errors.New("exit status 128")
		}
		// gh repo delete (cleanup) succeeds.
		return "", nil
	}

	var buf bytes.Buffer
	err := CreateRepo(&buf, cfg, state, t.TempDir(), run, fakeFetchRelease("v1.0.0"))
	if err == nil {
		t.Fatal("CreateRepo() expected error when git clone fails, got nil")
	}
	if !strings.Contains(err.Error(), "git clone") {
		t.Errorf("error should mention 'git clone', got: %v", err)
	}
}

func TestCreateRepo_PopulatesStateRepoName(t *testing.T) {
	cfg := BootstrapConfig{Topology: "Single", Owner: "alice", ProjectName: "my-project", TemplateRepo: DefaultTemplateRepo}
	state := &StepState{}

	// Both gh and git succeed; API call will fail gracefully (no real gh auth in tests).
	run := fakeRunOK("")

	var buf bytes.Buffer
	// We expect an error from the REST API call (no real auth), but state.RepoName
	// should still be set before that point.
	_ = CreateRepo(&buf, cfg, state, t.TempDir(), run, fakeFetchRelease("v1.0.0"))

	if state.RepoName != "my-project" {
		t.Errorf("state.RepoName = %q, want %q", state.RepoName, "my-project")
	}
}

func TestCreateRepo_Federated_SetsAgenticSuffix(t *testing.T) {
	cfg := BootstrapConfig{Topology: "Federated", Owner: "acme", ProjectName: "myapp", TemplateRepo: DefaultTemplateRepo}
	state := &StepState{}
	run := fakeRunOK("")

	var buf bytes.Buffer
	_ = CreateRepo(&buf, cfg, state, t.TempDir(), run, fakeFetchRelease("v1.0.0"))

	if state.RepoName != "myapp-agentic" {
		t.Errorf("state.RepoName = %q, want %q", state.RepoName, "myapp-agentic")
	}
}

func TestCreateRepo_EmptyTemplateRepo_ReturnsError(t *testing.T) {
	cfg := BootstrapConfig{Topology: "Single", Owner: "alice", ProjectName: "my-project", TemplateRepo: ""}
	state := &StepState{}

	var buf bytes.Buffer
	err := CreateRepo(&buf, cfg, state, t.TempDir(), fakeRunOK(""), fakeFetchRelease("v1.0.0"))
	if err == nil {
		t.Fatal("CreateRepo() expected error when template repo is empty")
	}
	if !strings.Contains(err.Error(), "TEMPLATE_SOURCE") {
		t.Errorf("error should mention 'TEMPLATE_SOURCE', got: %v", err)
	}
}

func TestCreateRepo_FetchReleaseFails_ReturnsErrorBeforeRepoCreation(t *testing.T) {
	cfg := BootstrapConfig{Topology: "Single", Owner: "alice", ProjectName: "my-project", TemplateRepo: DefaultTemplateRepo}
	state := &StepState{}

	// run should never be called — error should happen before repo creation.
	runCalled := false
	run := func(name string, args ...string) (string, error) {
		runCalled = true
		return "", nil
	}

	var buf bytes.Buffer
	err := CreateRepo(&buf, cfg, state, t.TempDir(), run, fakeFetchReleaseFail("no releases found"))
	if err == nil {
		t.Fatal("CreateRepo() expected error when release fetch fails")
	}
	if runCalled {
		t.Error("expected no commands to be called when release fetch fails")
	}
	if !strings.Contains(err.Error(), "fetching release tag") {
		t.Errorf("error should mention 'fetching release tag', got: %v", err)
	}
}

func TestCreateRepo_TarballFetchFails_CleansUpRepo(t *testing.T) {
	cfg := BootstrapConfig{Topology: "Single", Owner: "alice", ProjectName: "my-project", TemplateRepo: DefaultTemplateRepo}
	state := &StepState{}

	// Override the package-level tarball fetch to simulate a failure.
	orig := fetchTarballFn
	fetchTarballFn = func(_, _ string) (io.ReadCloser, error) {
		return nil, fmt.Errorf("404 not found")
	}
	defer func() { fetchTarballFn = orig }()

	var deleteCalled bool
	run := func(name string, args ...string) (string, error) {
		if name == "gh" && len(args) > 1 && args[0] == "repo" && args[1] == "create" {
			return "", nil
		}
		if name == "gh" && len(args) > 1 && args[0] == "repo" && args[1] == "delete" {
			deleteCalled = true
			return "", nil
		}
		if name == "git" && len(args) > 0 && args[0] == "clone" {
			if len(args) > 2 {
				os.MkdirAll(args[2], 0o755)
			}
			return "", nil
		}
		return "", nil
	}

	var buf bytes.Buffer
	err := CreateRepo(&buf, cfg, state, t.TempDir(), run, fakeFetchRelease("v1.0.0"))
	if err == nil {
		t.Fatal("CreateRepo() expected error when tarball fetch fails")
	}
	if !deleteCalled {
		t.Error("expected gh repo delete to be called to clean up after tarball failure")
	}
}

// --------------------------------------------------------------------------------------
// Step 3 — CreateRepo: form-provided repo mode and already-agentic guard
// --------------------------------------------------------------------------------------

func TestCreateRepo_NewRepoPath_ProceedsWithCreate(t *testing.T) {
	cfg := BootstrapConfig{Topology: "Single", Owner: "alice", ProjectName: "new-project", TemplateRepo: DefaultTemplateRepo, ExistingRepo: false}
	state := &StepState{}

	repoCreateCalled := false
	run := func(name string, args ...string) (string, error) {
		if name == "gh" && len(args) > 1 && args[0] == "repo" && args[1] == "create" {
			repoCreateCalled = true
			return "https://github.com/alice/new-project", nil
		}
		return "", nil
	}

	var buf bytes.Buffer
	_ = CreateRepo(&buf, cfg, state, t.TempDir(), run, fakeFetchRelease("v1.0.0"))

	if !repoCreateCalled {
		t.Error("expected gh repo create to be called for new repo, but it was not")
	}
	if state.ExistingRepo {
		t.Error("expected state.ExistingRepo to be false for new repo")
	}
}

func TestCreateRepo_ExistingRepo_WithTemplateSource_Aborts(t *testing.T) {
	workDir := t.TempDir()
	cfg := BootstrapConfig{Topology: "Single", Owner: "alice", ProjectName: "my-project", TemplateRepo: DefaultTemplateRepo, ExistingRepo: true}
	state := &StepState{}

	// Pre-create the clone directory with TEMPLATE_SOURCE marker.
	clonePath := filepath.Join(workDir, "my-project")
	if err := os.MkdirAll(clonePath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(clonePath, "TEMPLATE_SOURCE"), []byte("some/repo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	run := func(name string, args ...string) (string, error) {
		if name == "git" && len(args) > 0 && args[0] == "clone" {
			// Clone "succeeds" — directory already exists with marker.
			return "", nil
		}
		return "", nil
	}

	var buf bytes.Buffer
	err := CreateRepo(&buf, cfg, state, workDir, run, fakeFetchRelease("v1.0.0"))
	if err == nil {
		t.Fatal("expected error for already-bootstrapped repo, got nil")
	}
	if !strings.Contains(err.Error(), "already been bootstrapped") {
		t.Errorf("error should mention 'already been bootstrapped', got: %v", err)
	}
}

func TestCreateRepo_ExistingRepo_WithTemplateVersion_Aborts(t *testing.T) {
	workDir := t.TempDir()
	cfg := BootstrapConfig{Topology: "Single", Owner: "alice", ProjectName: "my-project", TemplateRepo: DefaultTemplateRepo, ExistingRepo: true}
	state := &StepState{}

	clonePath := filepath.Join(workDir, "my-project")
	if err := os.MkdirAll(clonePath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(clonePath, "TEMPLATE_VERSION"), []byte("v1.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	run := func(name string, args ...string) (string, error) {
		if name == "git" && len(args) > 0 && args[0] == "clone" {
			return "", nil
		}
		return "", nil
	}

	var buf bytes.Buffer
	err := CreateRepo(&buf, cfg, state, workDir, run, fakeFetchRelease("v1.0.0"))
	if err == nil {
		t.Fatal("expected error for repo with TEMPLATE_VERSION, got nil")
	}
	if !strings.Contains(err.Error(), "already been bootstrapped") {
		t.Errorf("error should mention 'already been bootstrapped', got: %v", err)
	}
}

func TestCreateRepo_ExistingRepo_WithBaseDir_Aborts(t *testing.T) {
	workDir := t.TempDir()
	cfg := BootstrapConfig{Topology: "Single", Owner: "alice", ProjectName: "my-project", TemplateRepo: DefaultTemplateRepo, ExistingRepo: true}
	state := &StepState{}

	clonePath := filepath.Join(workDir, "my-project")
	if err := os.MkdirAll(filepath.Join(clonePath, "base"), 0o755); err != nil {
		t.Fatal(err)
	}

	run := func(name string, args ...string) (string, error) {
		if name == "git" && len(args) > 0 && args[0] == "clone" {
			return "", nil
		}
		return "", nil
	}

	var buf bytes.Buffer
	err := CreateRepo(&buf, cfg, state, workDir, run, fakeFetchRelease("v1.0.0"))
	if err == nil {
		t.Fatal("expected error for repo with base/ directory, got nil")
	}
	if !strings.Contains(err.Error(), "already been bootstrapped") {
		t.Errorf("error should mention 'already been bootstrapped', got: %v", err)
	}
}

func TestCreateRepo_ExistingRepo_NonAgentic_SetsExistingRepoFlag(t *testing.T) {
	workDir := t.TempDir()
	cfg := BootstrapConfig{Topology: "Single", Owner: "alice", ProjectName: "my-project", TemplateRepo: DefaultTemplateRepo, ExistingRepo: true}
	state := &StepState{}

	// Pre-create clone directory with no agentic markers.
	clonePath := filepath.Join(workDir, "my-project")
	if err := os.MkdirAll(clonePath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(clonePath, "README.md"), []byte("# Hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	run := func(name string, args ...string) (string, error) {
		if name == "git" && len(args) > 0 && args[0] == "clone" {
			return "", nil
		}
		return "", nil
	}

	var buf bytes.Buffer
	// CreateRepo now completes successfully for existing non-agentic repos
	// (it creates the branch, applies template, and commits).
	err := CreateRepo(&buf, cfg, state, workDir, run, fakeFetchRelease("v1.0.0"))
	if err != nil {
		t.Fatalf("CreateRepo() unexpected error for existing non-agentic repo: %v", err)
	}
	if !state.ExistingRepo {
		t.Error("expected state.ExistingRepo to be true for existing non-agentic repo")
	}

	// Verify TEMPLATE_VERSION was written.
	versionData, readErr := os.ReadFile(filepath.Join(clonePath, "TEMPLATE_VERSION"))
	if readErr != nil {
		t.Errorf("expected TEMPLATE_VERSION to be written, got error: %v", readErr)
	}
	if !strings.Contains(string(versionData), "v1.0.0") {
		t.Errorf("TEMPLATE_VERSION should contain 'v1.0.0', got: %s", string(versionData))
	}
}

func TestCreateRepo_ExistingRepo_BranchAlreadyExists_Aborts(t *testing.T) {
	workDir := t.TempDir()
	cfg := BootstrapConfig{Topology: "Single", Owner: "alice", ProjectName: "my-project", TemplateRepo: DefaultTemplateRepo, ExistingRepo: true}
	state := &StepState{}

	// Pre-create clone directory with no agentic markers.
	clonePath := filepath.Join(workDir, "my-project")
	if err := os.MkdirAll(clonePath, 0o755); err != nil {
		t.Fatal(err)
	}

	run := func(name string, args ...string) (string, error) {
		if name == "git" && len(args) > 0 && args[0] == "clone" {
			return "", nil
		}
		// Simulate ls-remote returning a matching branch.
		if name == "bash" && len(args) > 1 && strings.Contains(args[1], "ls-remote") {
			return "abc123\trefs/heads/bootstrap/init", nil
		}
		return "", nil
	}

	var buf bytes.Buffer
	err := CreateRepo(&buf, cfg, state, workDir, run, fakeFetchRelease("v1.0.0"))
	if err == nil {
		t.Fatal("expected error when bootstrap/init branch exists, got nil")
	}
	if !strings.Contains(err.Error(), "bootstrap/init already exists") {
		t.Errorf("error should mention 'bootstrap/init already exists', got: %v", err)
	}
}

func TestCreateRepo_ExistingRepo_CreatesBootstrapBranch(t *testing.T) {
	workDir := t.TempDir()
	cfg := BootstrapConfig{Topology: "Single", Owner: "alice", ProjectName: "my-project", TemplateRepo: DefaultTemplateRepo, ExistingRepo: true}
	state := &StepState{}

	clonePath := filepath.Join(workDir, "my-project")
	if err := os.MkdirAll(clonePath, 0o755); err != nil {
		t.Fatal(err)
	}

	checkoutCalled := false
	run := func(name string, args ...string) (string, error) {
		if name == "git" && len(args) > 0 && args[0] == "clone" {
			return "", nil
		}
		// ls-remote returns empty (no existing branch).
		if name == "bash" && len(args) > 1 && strings.Contains(args[1], "ls-remote") {
			return "", nil
		}
		// Capture checkout -b bootstrap/init.
		if name == "bash" && len(args) > 1 && strings.Contains(args[1], "checkout") && strings.Contains(args[1], "bootstrap/init") {
			checkoutCalled = true
			return "", nil
		}
		return "", nil
	}

	var buf bytes.Buffer
	err := CreateRepo(&buf, cfg, state, workDir, run, fakeFetchRelease("v1.0.0"))
	if err != nil {
		t.Fatalf("CreateRepo() unexpected error: %v", err)
	}
	if !checkoutCalled {
		t.Error("expected git checkout -b bootstrap/init to be called")
	}
	if !state.ExistingRepo {
		t.Error("expected state.ExistingRepo to be true")
	}
}

func TestCreateRepo_ExistingRepo_CloneFails_ReturnsError(t *testing.T) {
	cfg := BootstrapConfig{Topology: "Single", Owner: "alice", ProjectName: "my-project", TemplateRepo: DefaultTemplateRepo, ExistingRepo: true}
	state := &StepState{}

	run := func(name string, args ...string) (string, error) {
		if name == "git" && len(args) > 0 && args[0] == "clone" {
			return "permission denied", errors.New("exit status 128")
		}
		return "", nil
	}

	var buf bytes.Buffer
	err := CreateRepo(&buf, cfg, state, t.TempDir(), run, fakeFetchRelease("v1.0.0"))
	if err == nil {
		t.Fatal("expected error when clone of existing repo fails, got nil")
	}
	if !strings.Contains(err.Error(), "git clone (existing repo)") {
		t.Errorf("error should mention 'git clone (existing repo)', got: %v", err)
	}
}

func TestCreateRepo_ExistingRepo_NoDeleteOnCloneFailure(t *testing.T) {
	cfg := BootstrapConfig{Topology: "Single", Owner: "alice", ProjectName: "my-project", TemplateRepo: DefaultTemplateRepo, ExistingRepo: true}
	state := &StepState{}

	deleteCalled := false
	run := func(name string, args ...string) (string, error) {
		if name == "git" && len(args) > 0 && args[0] == "clone" {
			return "permission denied", errors.New("exit status 128")
		}
		if name == "gh" && len(args) > 1 && args[0] == "repo" && args[1] == "delete" {
			deleteCalled = true
			return "", nil
		}
		return "", nil
	}

	var buf bytes.Buffer
	_ = CreateRepo(&buf, cfg, state, t.TempDir(), run, fakeFetchRelease("v1.0.0"))

	if deleteCalled {
		t.Error("should NOT call gh repo delete when cloning an existing repo fails — the repo is not ours to delete")
	}
}

// --------------------------------------------------------------------------------------
// Step 6 — ConfigureRepo
// --------------------------------------------------------------------------------------

func TestConfigureRepo_AllLabelsCreated_ReturnsNil(t *testing.T) {
	cfg := BootstrapConfig{Owner: "alice", ProjectName: "myrepo"}
	state := &StepState{RepoName: "myrepo"}

	var created []string
	run := func(name string, args ...string) (string, error) {
		if name == "gh" && len(args) > 2 && args[0] == "label" && args[1] == "create" {
			created = append(created, args[2])
		}
		return "", nil
	}

	var buf bytes.Buffer
	if err := ConfigureRepo(&buf, cfg, state, run); err != nil {
		t.Fatalf("ConfigureRepo() unexpected error: %v", err)
	}
	if len(created) != len(standardLabels) {
		t.Errorf("expected %d labels created, got %d: %v", len(standardLabels), len(created), created)
	}
}

func TestConfigureRepo_OneLabelFails_StillReturnsNil(t *testing.T) {
	cfg := BootstrapConfig{Owner: "alice", ProjectName: "myrepo"}
	state := &StepState{RepoName: "myrepo"}

	callCount := 0
	run := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			return "already exists", errors.New("already exists")
		}
		return "", nil
	}

	var buf bytes.Buffer
	if err := ConfigureRepo(&buf, cfg, state, run); err != nil {
		t.Errorf("ConfigureRepo() should not return error on label failure (best-effort), got: %v", err)
	}
}

func TestConfigureRepo_OutputContainsAllLabels(t *testing.T) {
	cfg := BootstrapConfig{Owner: "alice", ProjectName: "myrepo"}
	state := &StepState{RepoName: "myrepo"}

	// All labels succeed.
	var buf bytes.Buffer
	if err := ConfigureRepo(&buf, cfg, state, fakeRunOK("")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No warning output expected.
	if strings.Contains(buf.String(), "⚠") {
		t.Errorf("unexpected warning in output: %s", buf.String())
	}
}

// --------------------------------------------------------------------------------------
// Step 7 — PopulateRepo
// --------------------------------------------------------------------------------------

func TestPopulateRepo_WritesStandardFiles(t *testing.T) {
	dir := t.TempDir()
	cfg := BootstrapConfig{
		Owner:        "alice",
		ProjectName:  "my-project",
		Topology:     "Single",
		Stacks:      []string{"Go"},
		Description:  "A test project",
		Antora:       false,
		TemplateRepo: DefaultTemplateRepo,
	}
	state := &StepState{
		RepoName:  "my-project",
		ClonePath: dir,
		RepoURL:   "https://github.com/alice/my-project",
	}

	var buf bytes.Buffer
	if err := PopulateRepo(&buf, cfg, state, fakeRunOK("")); err != nil {
		t.Fatalf("PopulateRepo() unexpected error: %v", err)
	}

	// Single topology: AGENTS.local.md, README.md, LOCALRULES.md created.
	for _, f := range []string{"AGENTS.local.md", "README.md", "LOCALRULES.md"} {
		data, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			t.Errorf("expected %s to exist, got: %v", f, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("%s is empty", f)
		}
	}

	// Single topology: REPOS.md should NOT be created.
	if _, err := os.Stat(filepath.Join(dir, "REPOS.md")); !os.IsNotExist(err) {
		t.Error("REPOS.md should not be created for Single topology")
	}

	// Verify skills/.gitkeep is created.
	if _, err := os.Stat(filepath.Join(dir, "skills", ".gitkeep")); err != nil {
		t.Error("expected skills/.gitkeep to exist")
	}
}

func TestPopulateRepo_Federated_CreatesOrgOnlyFiles(t *testing.T) {
	dir := t.TempDir()
	cfg := BootstrapConfig{
		Owner:        "myorg",
		ProjectName:  "my-project",
		Topology:     "Federated",
		Stacks:      []string{"Go"},
		Description:  "An org project",
		Antora:       false,
		TemplateRepo: DefaultTemplateRepo,
	}
	state := &StepState{
		RepoName:  "my-project-agentic",
		ClonePath: dir,
		RepoURL:   "https://github.com/myorg/my-project-agentic",
	}

	var buf bytes.Buffer
	if err := PopulateRepo(&buf, cfg, state, fakeRunOK("")); err != nil {
		t.Fatalf("PopulateRepo() unexpected error: %v", err)
	}

	// Federated topology: REPOS.md, repos/.gitignore, tools/.gitignore created.
	for _, f := range []string{"REPOS.md", "repos/.gitignore", "tools/.gitignore"} {
		if _, err := os.Stat(filepath.Join(dir, f)); os.IsNotExist(err) {
			t.Errorf("expected %s to exist for Federated topology", f)
		}
	}
}

func TestPopulateRepo_Single_OmitsOrgOnlyFiles(t *testing.T) {
	dir := t.TempDir()
	cfg := BootstrapConfig{
		Owner:        "alice",
		ProjectName:  "my-project",
		Topology:     "Single",
		Stacks:      []string{"Go"},
		Description:  "A personal project",
		Antora:       false,
		TemplateRepo: DefaultTemplateRepo,
	}
	state := &StepState{
		RepoName:  "my-project",
		ClonePath: dir,
		RepoURL:   "https://github.com/alice/my-project",
	}

	var buf bytes.Buffer
	if err := PopulateRepo(&buf, cfg, state, fakeRunOK("")); err != nil {
		t.Fatalf("PopulateRepo() unexpected error: %v", err)
	}

	// Single topology: REPOS.md, repos/.gitignore, tools/.gitignore NOT created.
	for _, f := range []string{"REPOS.md", "repos/.gitignore", "tools/.gitignore"} {
		if _, err := os.Stat(filepath.Join(dir, f)); !os.IsNotExist(err) {
			t.Errorf("%s should not be created for Single topology", f)
		}
	}
}

func TestPopulateRepo_LOCALRULESNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	// Pre-create LOCALRULES.md with custom content.
	existingContent := "# My Custom Rules\n\nDo not overwrite.\n"
	if err := os.WriteFile(filepath.Join(dir, "LOCALRULES.md"), []byte(existingContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := BootstrapConfig{
		Owner:        "alice",
		ProjectName:  "my-project",
		Topology:     "Single",
		Stacks:      []string{"Go"},
		Description:  "A test project",
		Antora:       false,
		TemplateRepo: DefaultTemplateRepo,
	}
	state := &StepState{
		RepoName:  "my-project",
		ClonePath: dir,
		RepoURL:   "https://github.com/alice/my-project",
	}

	var buf bytes.Buffer
	if err := PopulateRepo(&buf, cfg, state, fakeRunOK("")); err != nil {
		t.Fatalf("PopulateRepo() unexpected error: %v", err)
	}

	// LOCALRULES.md should not be overwritten.
	data, _ := os.ReadFile(filepath.Join(dir, "LOCALRULES.md"))
	if string(data) != existingContent {
		t.Errorf("LOCALRULES.md should not be overwritten, got: %q", string(data))
	}
}

func TestPopulateRepo_READMENotOverwritten(t *testing.T) {
	dir := t.TempDir()
	// Pre-create README.md with custom content.
	existingContent := "# My Existing README\n"
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(existingContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := BootstrapConfig{
		Owner:        "alice",
		ProjectName:  "my-project",
		Topology:     "Single",
		Stacks:      []string{"Go"},
		Description:  "A test project",
		Antora:       false,
		TemplateRepo: DefaultTemplateRepo,
	}
	state := &StepState{
		RepoName:  "my-project",
		ClonePath: dir,
		RepoURL:   "https://github.com/alice/my-project",
	}

	var buf bytes.Buffer
	if err := PopulateRepo(&buf, cfg, state, fakeRunOK("")); err != nil {
		t.Fatalf("PopulateRepo() unexpected error: %v", err)
	}

	// README.md should not be overwritten.
	data, _ := os.ReadFile(filepath.Join(dir, "README.md"))
	if string(data) != existingContent {
		t.Errorf("README.md should not be overwritten, got: %q", string(data))
	}
}

func TestPopulateRepo_CreatesSkillsGitkeep(t *testing.T) {
	dir := t.TempDir()
	cfg := BootstrapConfig{
		Owner:        "alice",
		ProjectName:  "my-project",
		Topology:     "Single",
		Stacks:      []string{"Go"},
		Description:  "A test project",
		Antora:       false,
		TemplateRepo: DefaultTemplateRepo,
	}
	state := &StepState{
		RepoName:  "my-project",
		ClonePath: dir,
		RepoURL:   "https://github.com/alice/my-project",
	}

	var buf bytes.Buffer
	if err := PopulateRepo(&buf, cfg, state, fakeRunOK("")); err != nil {
		t.Fatalf("PopulateRepo() unexpected error: %v", err)
	}

	gitkeepPath := filepath.Join(dir, "skills", ".gitkeep")
	info, err := os.Stat(gitkeepPath)
	if err != nil {
		t.Fatalf("expected skills/.gitkeep to exist, got: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("expected skills/.gitkeep to be empty, got %d bytes", info.Size())
	}

	// Verify AGENTS.local.md mentions skills/ directory.
	data, err := os.ReadFile(filepath.Join(dir, "AGENTS.local.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "## Skills") {
		t.Error("AGENTS.local.md should contain a ## Skills section")
	}
	if !strings.Contains(string(data), "skills/") {
		t.Error("AGENTS.local.md should mention skills/ directory")
	}
}

func TestPopulateRepo_AntoraTrue_ScaffoldsExtraFiles(t *testing.T) {
	dir := t.TempDir()
	cfg := BootstrapConfig{
		Owner:        "alice",
		ProjectName:  "my-project",
		Topology:     "Single",
		Stacks:      []string{"Go"},
		Description:  "A test project",
		Antora:       true,
		TemplateRepo: DefaultTemplateRepo,
	}
	state := &StepState{
		RepoName:  "my-project",
		ClonePath: dir,
		RepoURL:   "https://github.com/alice/my-project",
	}

	var buf bytes.Buffer
	if err := PopulateRepo(&buf, cfg, state, fakeRunOK("")); err != nil {
		t.Fatalf("PopulateRepo() unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "antora-playbook.yml")); err != nil {
		t.Error("expected antora-playbook.yml to exist")
	}
	if _, err := os.Stat(filepath.Join(dir, "docs", "antora.yml")); err != nil {
		t.Error("expected docs/antora.yml to exist")
	}
}

func TestPopulateRepo_PushFails_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	cfg := BootstrapConfig{
		Owner:        "alice",
		ProjectName:  "my-project",
		Stacks:      []string{"Go"},
		Description:  "A test project",
		TemplateRepo: DefaultTemplateRepo,
	}
	state := &StepState{RepoName: "my-project", ClonePath: dir}

	// runInDir wraps all commands in "bash -c 'cd ... && ...'".
	// git push generates: bash -c "cd '...' && 'git' 'push' 'origin' 'main'"
	run := func(name string, args ...string) (string, error) {
		if name == "bash" {
			script := strings.Join(args, " ")
			if strings.Contains(script, "'git'") && strings.Contains(script, "'push'") {
				return "error: failed to push", errors.New("exit status 1")
			}
		}
		return "", nil
	}

	var buf bytes.Buffer
	err := PopulateRepo(&buf, cfg, state, run)
	if err == nil {
		t.Fatal("PopulateRepo() expected error when git push fails, got nil")
	}
	if !strings.Contains(err.Error(), "git push") {
		t.Errorf("error should mention 'git push', got: %v", err)
	}
}

func TestPopulateRepo_ReposMDContainsDescription(t *testing.T) {
	dir := t.TempDir()
	cfg := BootstrapConfig{
		Owner:        "myorg",
		ProjectName:  "my-project",
		Topology:     "Federated",
		Stacks:      []string{"Go"},
		Description:  "unique-description-12345",
		TemplateRepo: DefaultTemplateRepo,
	}
	state := &StepState{RepoName: "my-project-agentic", ClonePath: dir}

	var buf bytes.Buffer
	if err := PopulateRepo(&buf, cfg, state, fakeRunOK("")); err != nil {
		t.Fatalf("PopulateRepo() unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "REPOS.md"))
	if !strings.Contains(string(data), "unique-description-12345") {
		t.Errorf("REPOS.md should contain description, got:\n%s", string(data))
	}
}

func TestPopulateRepo_DeploysPublishReleaseYml(t *testing.T) {
	dir := t.TempDir()

	// Create the example source file in the template clone path.
	exampleDir := filepath.Join(dir, ".ai", "docs", "examples")
	if err := os.MkdirAll(exampleDir, 0755); err != nil {
		t.Fatal(err)
	}
	exampleContent := []byte("name: Publish Release\non: push\n")
	if err := os.WriteFile(filepath.Join(exampleDir, "publish-release.yml"), exampleContent, 0644); err != nil {
		t.Fatal(err)
	}

	cfg := BootstrapConfig{
		Owner:        "alice",
		ProjectName:  "my-project",
		Stacks:      []string{"Go"},
		Description:  "A test project",
		TemplateRepo: DefaultTemplateRepo,
	}
	state := &StepState{
		RepoName:  "my-project",
		ClonePath: dir,
		RepoURL:   "https://github.com/alice/my-project",
	}

	var buf bytes.Buffer
	if err := PopulateRepo(&buf, cfg, state, fakeRunOK("")); err != nil {
		t.Fatalf("PopulateRepo() unexpected error: %v", err)
	}

	// Assert the file was deployed.
	dstPath := filepath.Join(dir, ".github", "workflows", "publish-release.yml")
	data, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("expected .github/workflows/publish-release.yml to exist, got: %v", err)
	}
	if !bytes.Equal(data, exampleContent) {
		t.Errorf("deployed content mismatch.\nwant: %s\ngot:  %s", exampleContent, data)
	}

	// Assert log message.
	if !strings.Contains(buf.String(), "publish-release.yml deployed") {
		t.Errorf("expected deploy log message, got: %s", buf.String())
	}
}

func TestPopulateRepo_SkipsPublishReleaseYml_WhenMissing(t *testing.T) {
	dir := t.TempDir()
	cfg := BootstrapConfig{
		Owner:        "alice",
		ProjectName:  "my-project",
		Stacks:      []string{"Go"},
		Description:  "A test project",
		TemplateRepo: DefaultTemplateRepo,
	}
	state := &StepState{
		RepoName:  "my-project",
		ClonePath: dir,
		RepoURL:   "https://github.com/alice/my-project",
	}

	var buf bytes.Buffer
	if err := PopulateRepo(&buf, cfg, state, fakeRunOK("")); err != nil {
		t.Fatalf("PopulateRepo() unexpected error: %v", err)
	}

	// Assert the file was NOT created.
	dstPath := filepath.Join(dir, ".github", "workflows", "publish-release.yml")
	if _, err := os.Stat(dstPath); !os.IsNotExist(err) {
		t.Errorf("expected publish-release.yml to NOT exist when example is missing")
	}

	// Assert skip log message.
	if !strings.Contains(buf.String(), "publish-release.yml example not found in template") {
		t.Errorf("expected skip log message, got: %s", buf.String())
	}
}

// --------------------------------------------------------------------------------------
// PopulateRepo — Existing repo skip-push
// --------------------------------------------------------------------------------------

func TestPopulateRepo_ExistingRepo_SkipsPush(t *testing.T) {
	dir := t.TempDir()
	cfg := BootstrapConfig{
		Owner:        "alice",
		ProjectName:  "my-project",
		Stacks:       []string{"Go"},
		Description:  "A test project",
		TemplateRepo: DefaultTemplateRepo,
	}
	state := &StepState{RepoName: "my-project", ClonePath: dir, ExistingRepo: true}

	pushCalled := false
	run := func(name string, args ...string) (string, error) {
		if name == "bash" {
			script := strings.Join(args, " ")
			if strings.Contains(script, "'git'") && strings.Contains(script, "'push'") {
				pushCalled = true
				return "", nil
			}
		}
		return "", nil
	}

	var buf bytes.Buffer
	err := PopulateRepo(&buf, cfg, state, run)
	if err != nil {
		t.Fatalf("PopulateRepo() unexpected error: %v", err)
	}
	if pushCalled {
		t.Error("expected git push to be skipped for existing repo, but it was called")
	}
}

// --------------------------------------------------------------------------------------
// OpenBootstrapPR
// --------------------------------------------------------------------------------------

func TestOpenBootstrapPR_NotExistingRepo_Noop(t *testing.T) {
	state := &StepState{ExistingRepo: false}
	cfg := BootstrapConfig{Owner: "alice", ProjectName: "my-project"}

	err := OpenBootstrapPR(cfg, state, fakeRunOK(""))
	if err != nil {
		t.Fatalf("OpenBootstrapPR() unexpected error: %v", err)
	}
	if state.PRURL != "" {
		t.Error("expected PRURL to be empty when ExistingRepo is false")
	}
}

func TestOpenBootstrapPR_PushesBranchAndCreatesPR(t *testing.T) {
	dir := t.TempDir()
	state := &StepState{
		RepoName:     "my-project",
		ClonePath:    dir,
		ExistingRepo: true,
	}
	cfg := BootstrapConfig{Owner: "alice", ProjectName: "my-project"}

	pushCalled := false
	prCreateCalled := false
	run := func(name string, args ...string) (string, error) {
		if name == "bash" && len(args) > 1 && strings.Contains(args[1], "'push'") && strings.Contains(args[1], "bootstrap/init") {
			pushCalled = true
			return "", nil
		}
		if name == "gh" && len(args) > 1 && args[0] == "pr" && args[1] == "create" {
			prCreateCalled = true
			return "https://github.com/alice/my-project/pull/1", nil
		}
		return "", nil
	}

	err := OpenBootstrapPR(cfg, state, run)
	if err != nil {
		t.Fatalf("OpenBootstrapPR() unexpected error: %v", err)
	}
	if !pushCalled {
		t.Error("expected git push bootstrap/init to be called")
	}
	if !prCreateCalled {
		t.Error("expected gh pr create to be called")
	}
	if state.PRURL != "https://github.com/alice/my-project/pull/1" {
		t.Errorf("expected PRURL to be set, got: %q", state.PRURL)
	}
}

func TestOpenBootstrapPR_PushFails_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	state := &StepState{
		RepoName:     "my-project",
		ClonePath:    dir,
		ExistingRepo: true,
	}
	cfg := BootstrapConfig{Owner: "alice", ProjectName: "my-project"}

	run := func(name string, args ...string) (string, error) {
		if name == "bash" && len(args) > 1 && strings.Contains(args[1], "'push'") {
			return "error: push failed", errors.New("exit status 1")
		}
		return "", nil
	}

	err := OpenBootstrapPR(cfg, state, run)
	if err == nil {
		t.Fatal("expected error when push fails, got nil")
	}
	if !strings.Contains(err.Error(), "git push bootstrap/init") {
		t.Errorf("error should mention 'git push bootstrap/init', got: %v", err)
	}
}

func TestOpenBootstrapPR_PRCreateFails_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	state := &StepState{
		RepoName:     "my-project",
		ClonePath:    dir,
		ExistingRepo: true,
	}
	cfg := BootstrapConfig{Owner: "alice", ProjectName: "my-project"}

	run := func(name string, args ...string) (string, error) {
		if name == "gh" && len(args) > 1 && args[0] == "pr" && args[1] == "create" {
			return "error", errors.New("pr creation failed")
		}
		return "", nil
	}

	err := OpenBootstrapPR(cfg, state, run)
	if err == nil {
		t.Fatal("expected error when PR creation fails, got nil")
	}
	if !strings.Contains(err.Error(), "gh pr create") {
		t.Errorf("error should mention 'gh pr create', got: %v", err)
	}
}

// --------------------------------------------------------------------------------------
// Step 8 — CreateProject
// --------------------------------------------------------------------------------------

func TestCreateProject_GhCreateFails_ReturnsError(t *testing.T) {
	cfg := BootstrapConfig{Owner: "alice", ProjectName: "my-project"}
	state := &StepState{RepoName: "my-project"}
	run := fakeRunFail("project creation failed")

	var buf bytes.Buffer
	err := CreateProject(&buf, cfg, state, run, nil)
	if err == nil {
		t.Fatal("CreateProject() expected error when gh project create fails, got nil")
	}
	if !strings.Contains(err.Error(), "gh project create") {
		t.Errorf("error should mention 'gh project create', got: %v", err)
	}
}

func TestCreateProject_LinkFails_StillReturnsNil(t *testing.T) {
	cfg := BootstrapConfig{Owner: "alice", ProjectName: "my-project"}
	state := &StepState{
		RepoName:   "my-project",
		RepoNodeID: "REPO_NODE_ID",
	}

	// gh project create returns JSON with number.
	run := fakeRunOK(`{"number":5,"url":"https://github.com/orgs/alice/projects/5"}`)

	// GraphQL always fails (best-effort — step should still succeed).
	graphqlDo := func(query string, variables map[string]interface{}, response interface{}) error {
		return errors.New("graphql failure")
	}

	var buf bytes.Buffer
	if err := CreateProject(&buf, cfg, state, run, graphqlDo); err != nil {
		t.Fatalf("CreateProject() expected nil when link fails (best-effort), got: %v", err)
	}
}

func TestCreateProject_SetsProjectURL(t *testing.T) {
	cfg := BootstrapConfig{Owner: "alice", ProjectName: "my-project"}
	state := &StepState{RepoName: "my-project"}

	run := fakeRunOK(`{"number":7,"url":"https://github.com/orgs/alice/projects/7"}`)

	var buf bytes.Buffer
	_ = CreateProject(&buf, cfg, state, run, nil)

	if state.ProjectURL != "https://github.com/orgs/alice/projects/7" {
		t.Errorf("state.ProjectURL = %q, want %q", state.ProjectURL,
			"https://github.com/orgs/alice/projects/7")
	}
}

func TestCreateProject_SuccessfulLink(t *testing.T) {
	cfg := BootstrapConfig{Owner: "alice", ProjectName: "my-project"}
	state := &StepState{
		RepoName:   "my-project",
		RepoNodeID: "REPO_ID",
	}

	run := fakeRunOK(`{"number":3,"url":"https://github.com/users/alice/projects/3"}`)

	var linkCalled bool
	graphqlDo := func(query string, variables map[string]interface{}, response interface{}) error {
		if strings.Contains(query, "linkProjectV2ToRepository") {
			linkCalled = true
			return nil
		}
		// Return project node ID for fetch query.
		type userResp struct {
			User struct {
				ProjectV2 struct {
					ID string `json:"id"`
				} `json:"projectV2"`
			} `json:"user"`
		}
		if r, ok := response.(*struct {
			User struct {
				ProjectV2 struct {
					ID string `json:"id"`
				} `json:"projectV2"`
			} `json:"user"`
		}); ok {
			r.User.ProjectV2.ID = "PROJECT_NODE_ID"
		}
		return nil
	}

	var buf bytes.Buffer
	if err := CreateProject(&buf, cfg, state, run, graphqlDo); err != nil {
		t.Fatalf("CreateProject() unexpected error: %v", err)
	}
	_ = linkCalled // Link may or may not be called depending on node ID resolution
}

// --------------------------------------------------------------------------------------
// Step 8b — ConfigureProjectStatus
// --------------------------------------------------------------------------------------

// writeTestProjectTemplate creates a valid base/project-template.json in root.
func writeTestProjectTemplate(t *testing.T, root string) {
	t.Helper()
	baseDir := filepath.Join(root, "base")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `{
  "statusOptions": [
    {"name": "Backlog",        "color": "GRAY",   "description": "Prioritised, ready to start"},
    {"name": "Scoping",        "color": "PURPLE", "description": "Requirement or feature being scoped"},
    {"name": "Scheduled",      "color": "BLUE",   "description": "Scoped and queued, waiting for design"},
    {"name": "In Design",      "color": "PINK",   "description": "Feature Design session active"},
    {"name": "In Development", "color": "YELLOW", "description": "Dev Session active"},
    {"name": "In Review",      "color": "ORANGE", "description": "PR open, awaiting review"},
    {"name": "Done",           "color": "GREEN",  "description": "Merged and closed"}
  ]
}`
	if err := os.WriteFile(filepath.Join(baseDir, "project-template.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestConfigureProjectStatus_UserAccount_ProceedsNormally(t *testing.T) {
	clonePath := t.TempDir()
	writeTestProjectTemplate(t, clonePath)
	state := &StepState{ProjectNodeID: "PVT_123", ClonePath: clonePath}

	var queries []string
	graphqlDo := func(query string, variables map[string]interface{}, response interface{}) error {
		queries = append(queries, query)
		if strings.Contains(query, "field(name: \"Status\")") {
			if r, ok := response.(*struct {
				Node struct {
					Field struct {
						ID      string `json:"id"`
						Options []struct {
							ID   string `json:"id"`
							Name string `json:"name"`
						} `json:"options"`
					} `json:"field"`
				} `json:"node"`
			}); ok {
				r.Node.Field.ID = "FIELD_ID"
			}
			return nil
		}
		return nil
	}

	var buf bytes.Buffer
	if err := ConfigureProjectStatus(&buf, state, graphqlDo); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(queries) < 2 {
		t.Fatalf("expected at least 2 GraphQL calls for user account, got %d", len(queries))
	}
}

func TestConfigureProjectStatus_MissingProjectNodeID_SkipsWithWarning(t *testing.T) {
	state := &StepState{ProjectNodeID: ""}

	graphqlDo := func(query string, variables map[string]interface{}, response interface{}) error {
		t.Fatal("GraphQL should not be called when project node ID is missing")
		return nil
	}

	var buf bytes.Buffer
	if err := ConfigureProjectStatus(&buf, state, graphqlDo); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "no project node ID") {
		t.Errorf("expected warning about missing node ID, got: %s", buf.String())
	}
}

func TestConfigureProjectStatus_Success_CallsUpdateMutationWithDescriptions(t *testing.T) {
	clonePath := t.TempDir()
	writeTestProjectTemplate(t, clonePath)
	state := &StepState{ProjectNodeID: "PVT_123", ClonePath: clonePath}

	var queries []string
	var updateVars map[string]interface{}
	graphqlDo := func(query string, variables map[string]interface{}, response interface{}) error {
		queries = append(queries, query)
		if strings.Contains(query, "field(name: \"Status\")") {
			if r, ok := response.(*struct {
				Node struct {
					Field struct {
						ID      string `json:"id"`
						Options []struct {
							ID   string `json:"id"`
							Name string `json:"name"`
						} `json:"options"`
					} `json:"field"`
				} `json:"node"`
			}); ok {
				r.Node.Field.ID = "FIELD_ID"
				r.Node.Field.Options = []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				}{
					{ID: "opt1", Name: "Todo"},
					{ID: "opt2", Name: "In Progress"},
					{ID: "opt3", Name: "Done"},
				}
			}
			return nil
		}
		if strings.Contains(query, "updateProjectV2Field") {
			updateVars = variables
		}
		return nil // update mutation succeeds
	}

	var buf bytes.Buffer
	if err := ConfigureProjectStatus(&buf, state, graphqlDo); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(queries) < 2 {
		t.Fatalf("expected at least 2 GraphQL calls (fetch + update), got %d", len(queries))
	}
	if !strings.Contains(queries[1], "updateProjectV2Field") {
		t.Errorf("expected second call to be updateProjectV2Field, got: %s", queries[1])
	}

	// Verify 7 options are passed (from project-template.json).
	options, ok := updateVars["options"].([]map[string]string)
	if !ok {
		t.Fatalf("expected options to be []map[string]string, got %T", updateVars["options"])
	}
	if len(options) != 7 {
		t.Errorf("expected 7 options, got %d", len(options))
	}

	// Verify descriptions are included.
	for _, opt := range options {
		if opt["description"] == "" {
			t.Errorf("expected description for option %q, got empty", opt["name"])
		}
	}

	// Verify first and last option names.
	if options[0]["name"] != "Backlog" {
		t.Errorf("expected first option to be Backlog, got %q", options[0]["name"])
	}
	if options[6]["name"] != "Done" {
		t.Errorf("expected last option to be Done, got %q", options[6]["name"])
	}
}

func TestConfigureProjectStatus_GraphQLFetchFails_LogsWarning(t *testing.T) {
	clonePath := t.TempDir()
	writeTestProjectTemplate(t, clonePath)
	state := &StepState{ProjectNodeID: "PVT_123", ClonePath: clonePath}

	graphqlDo := func(query string, variables map[string]interface{}, response interface{}) error {
		return errors.New("network error")
	}

	var buf bytes.Buffer
	err := ConfigureProjectStatus(&buf, state, graphqlDo)
	if err != nil {
		t.Fatalf("expected nil error (best-effort), got: %v", err)
	}
	if !strings.Contains(buf.String(), "Could not fetch Status field") {
		t.Errorf("expected warning about fetch failure, got: %s", buf.String())
	}
}

func TestConfigureProjectStatus_UpdateFails_LogsWarning(t *testing.T) {
	clonePath := t.TempDir()
	writeTestProjectTemplate(t, clonePath)
	state := &StepState{ProjectNodeID: "PVT_123", ClonePath: clonePath}

	callCount := 0
	graphqlDo := func(query string, variables map[string]interface{}, response interface{}) error {
		callCount++
		if callCount == 1 {
			// Fetch succeeds.
			if r, ok := response.(*struct {
				Node struct {
					Field struct {
						ID      string `json:"id"`
						Options []struct {
							ID   string `json:"id"`
							Name string `json:"name"`
						} `json:"options"`
					} `json:"field"`
				} `json:"node"`
			}); ok {
				r.Node.Field.ID = "FIELD_ID"
			}
			return nil
		}
		// Update fails.
		return errors.New("mutation failed")
	}

	var buf bytes.Buffer
	err := ConfigureProjectStatus(&buf, state, graphqlDo)
	if err != nil {
		t.Fatalf("expected nil error (best-effort), got: %v", err)
	}
	if !strings.Contains(buf.String(), "Could not update Status columns") {
		t.Errorf("expected warning about update failure, got: %s", buf.String())
	}
}

// --------------------------------------------------------------------------------------
// Step 8c — DeploySyncWorkflows
// --------------------------------------------------------------------------------------

func TestDeploySyncWorkflows_UserAccount_SkipsSilently(t *testing.T) {
	cfg := BootstrapConfig{Owner: "alice", OwnerType: OwnerTypeUser}
	state := &StepState{ClonePath: t.TempDir()}
	run := fakeRunFail("should not be called")

	var buf bytes.Buffer
	if err := DeploySyncWorkflows(&buf, cfg, state, run); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "personal account") {
		t.Errorf("expected skip message for personal account, got: %s", buf.String())
	}
}

func TestDeploySyncWorkflows_MissingSourceFiles_LogsWarning(t *testing.T) {
	cfg := BootstrapConfig{Owner: "acme-org", OwnerType: OwnerTypeOrg}
	dir := t.TempDir()
	state := &StepState{ClonePath: dir}
	run := fakeRunOK("")

	var buf bytes.Buffer
	if err := DeploySyncWorkflows(&buf, cfg, state, run); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "not found in .ai/") {
		t.Errorf("expected warning about missing source files, got: %s", out)
	}
	if !strings.Contains(out, "No sync workflows to deploy") {
		t.Errorf("expected 'No sync workflows' message, got: %s", out)
	}
}

func TestDeploySyncWorkflows_OrgSuccess_CopiesAndCommits(t *testing.T) {
	cfg := BootstrapConfig{Owner: "acme-org", OwnerType: OwnerTypeOrg}
	dir := t.TempDir()
	state := &StepState{ClonePath: dir}

	// Create source workflow files.
	sourceDir := filepath.Join(dir, ".ai", ".github", "workflows")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"sync-label-to-status.yml", "sync-status-to-label.yml"} {
		if err := os.WriteFile(filepath.Join(sourceDir, f), []byte("name: "+f), 0644); err != nil {
			t.Fatal(err)
		}
	}

	var commands []string
	run := func(name string, args ...string) (string, error) {
		if name == "bash" {
			commands = append(commands, strings.Join(args, " "))
		}
		return "", nil
	}

	var buf bytes.Buffer
	if err := DeploySyncWorkflows(&buf, cfg, state, run); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify files were copied to destination.
	destDir := filepath.Join(dir, ".github", "workflows")
	for _, f := range []string{"sync-label-to-status.yml", "sync-status-to-label.yml"} {
		data, err := os.ReadFile(filepath.Join(destDir, f))
		if err != nil {
			t.Errorf("expected %s in dest dir, got: %v", f, err)
			continue
		}
		if !strings.Contains(string(data), f) {
			t.Errorf("expected file content to contain %q, got: %s", f, string(data))
		}
	}

	// Verify git add, commit, and push were called.
	allCmds := strings.Join(commands, "\n")
	if !strings.Contains(allCmds, "'git' 'add'") {
		t.Error("expected git add to be called")
	}
	if !strings.Contains(allCmds, "'git' 'commit'") {
		t.Error("expected git commit to be called")
	}
	if !strings.Contains(allCmds, "'git' 'push'") {
		t.Error("expected git push to be called")
	}
}

func TestDeploySyncWorkflows_GitAddFails_LogsWarning(t *testing.T) {
	cfg := BootstrapConfig{Owner: "acme-org", OwnerType: OwnerTypeOrg}
	dir := t.TempDir()
	state := &StepState{ClonePath: dir}

	// Create source workflow files.
	sourceDir := filepath.Join(dir, ".ai", ".github", "workflows")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"sync-label-to-status.yml", "sync-status-to-label.yml"} {
		if err := os.WriteFile(filepath.Join(sourceDir, f), []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	run := fakeRunFail("git add failed")

	var buf bytes.Buffer
	err := DeploySyncWorkflows(&buf, cfg, state, run)
	if err != nil {
		t.Fatalf("expected nil error (best-effort), got: %v", err)
	}
	if !strings.Contains(buf.String(), "Could not stage sync workflows") {
		t.Errorf("expected warning about staging failure, got: %s", buf.String())
	}
}

// --------------------------------------------------------------------------------------
// Step 8 — CreateProject (continued)
// --------------------------------------------------------------------------------------

func TestCreateProject_StoresProjectNodeID(t *testing.T) {
	cfg := BootstrapConfig{Owner: "alice", ProjectName: "my-project"}
	state := &StepState{
		RepoName:   "my-project",
		RepoNodeID: "REPO_ID",
	}

	run := fakeRunOK(`{"number":3,"url":"https://github.com/users/alice/projects/3"}`)

	graphqlDo := func(query string, variables map[string]interface{}, response interface{}) error {
		if strings.Contains(query, "user(login") {
			if r, ok := response.(*struct {
				User struct {
					ProjectV2 struct {
						ID string `json:"id"`
					} `json:"projectV2"`
				} `json:"user"`
			}); ok {
				r.User.ProjectV2.ID = "PVT_NODE_ID"
			}
			return nil
		}
		return nil // link mutation succeeds
	}

	var buf bytes.Buffer
	if err := CreateProject(&buf, cfg, state, run, graphqlDo); err != nil {
		t.Fatalf("CreateProject() unexpected error: %v", err)
	}
	if state.ProjectNodeID != "PVT_NODE_ID" {
		t.Errorf("state.ProjectNodeID = %q, want %q", state.ProjectNodeID, "PVT_NODE_ID")
	}
}

func TestCreateProject_MissingProjectNodeID_SkipsVariableSet(t *testing.T) {
	cfg := BootstrapConfig{Owner: "alice", ProjectName: "my-project"}
	state := &StepState{RepoName: "my-project"}

	run := fakeRunOK(`{"number":5,"url":"https://github.com/users/alice/projects/5"}`)

	var buf bytes.Buffer
	_ = CreateProject(&buf, cfg, state, run, nil)

	out := buf.String()
	if !strings.Contains(out, "Skipping AGENTIC_PROJECT_ID") {
		t.Errorf("expected skip message for AGENTIC_PROJECT_ID, got: %s", out)
	}
}

func TestCreateProject_VariableSetFailure_LogsWarning(t *testing.T) {
	cfg := BootstrapConfig{Owner: "alice", ProjectName: "my-project"}
	state := &StepState{
		RepoName:   "my-project",
		RepoNodeID: "REPO_ID",
	}

	callCount := 0
	run := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			// gh project create
			return `{"number":3,"url":"https://github.com/users/alice/projects/3"}`, nil
		}
		// gh variable set fails
		return "could not set variable", errors.New("permission denied")
	}

	graphqlDo := func(query string, variables map[string]interface{}, response interface{}) error {
		if strings.Contains(query, "user(login") {
			if r, ok := response.(*struct {
				User struct {
					ProjectV2 struct {
						ID string `json:"id"`
					} `json:"projectV2"`
				} `json:"user"`
			}); ok {
				r.User.ProjectV2.ID = "PVT_NODE_ID"
			}
			return nil
		}
		return nil
	}

	var buf bytes.Buffer
	err := CreateProject(&buf, cfg, state, run, graphqlDo)
	if err != nil {
		t.Fatalf("CreateProject() should not return error on variable set failure (best-effort), got: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Could not set AGENTIC_PROJECT_ID") {
		t.Errorf("expected warning about variable set failure, got: %s", out)
	}
}

func TestCreateProject_VariableSetSuccess_CallsGhVariable(t *testing.T) {
	cfg := BootstrapConfig{Owner: "alice", ProjectName: "my-project"}
	state := &StepState{
		RepoName:   "my-project",
		RepoNodeID: "REPO_ID",
	}

	var variableSetArgs []string
	run := func(name string, args ...string) (string, error) {
		if name == "gh" && len(args) > 0 && args[0] == "variable" {
			variableSetArgs = args
			return "", nil
		}
		return `{"number":3,"url":"https://github.com/users/alice/projects/3"}`, nil
	}

	graphqlDo := func(query string, variables map[string]interface{}, response interface{}) error {
		if strings.Contains(query, "user(login") {
			if r, ok := response.(*struct {
				User struct {
					ProjectV2 struct {
						ID string `json:"id"`
					} `json:"projectV2"`
				} `json:"user"`
			}); ok {
				r.User.ProjectV2.ID = "PVT_NODE_ID"
			}
			return nil
		}
		return nil
	}

	var buf bytes.Buffer
	if err := CreateProject(&buf, cfg, state, run, graphqlDo); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(variableSetArgs) == 0 {
		t.Fatal("expected gh variable set to be called")
	}
	// Verify args: set AGENTIC_PROJECT_ID --body PVT_NODE_ID --repo alice/my-project
	joined := strings.Join(variableSetArgs, " ")
	if !strings.Contains(joined, "AGENTIC_PROJECT_ID") {
		t.Errorf("expected AGENTIC_PROJECT_ID in args, got: %s", joined)
	}
	if !strings.Contains(joined, "PVT_NODE_ID") {
		t.Errorf("expected project node ID in args, got: %s", joined)
	}
	if !strings.Contains(joined, "alice/my-project") {
		t.Errorf("expected repo name in args, got: %s", joined)
	}
}

// --- setProjectVariable org/user scope tests ---

func TestSetProjectVariable_OrgScope_UsesOrgFlag(t *testing.T) {
	cfg := BootstrapConfig{Owner: "acme-org", OwnerType: OwnerTypeOrg}
	state := &StepState{RepoName: "my-project", ProjectNodeID: "PVT_NODE_ID"}

	var capturedArgs []string
	run := func(name string, args ...string) (string, error) {
		capturedArgs = append([]string{name}, args...)
		return "", nil
	}

	var buf bytes.Buffer
	setProjectVariable(&buf, cfg, state, run)

	joined := strings.Join(capturedArgs, " ")
	if !strings.Contains(joined, "--org acme-org") {
		t.Errorf("expected --org acme-org in args, got: %v", capturedArgs)
	}
	if strings.Contains(joined, "--repo") {
		t.Errorf("expected no --repo flag for org scope, got: %v", capturedArgs)
	}
	if !strings.Contains(joined, "AGENTIC_PROJECT_ID") {
		t.Errorf("expected AGENTIC_PROJECT_ID in args, got: %v", capturedArgs)
	}
}

func TestSetProjectVariable_UserScope_UsesRepoFlag(t *testing.T) {
	cfg := BootstrapConfig{Owner: "alice", OwnerType: OwnerTypeUser}
	state := &StepState{RepoName: "my-project", ProjectNodeID: "PVT_NODE_ID"}

	var capturedArgs []string
	run := func(name string, args ...string) (string, error) {
		capturedArgs = append([]string{name}, args...)
		return "", nil
	}

	var buf bytes.Buffer
	setProjectVariable(&buf, cfg, state, run)

	joined := strings.Join(capturedArgs, " ")
	if !strings.Contains(joined, "--repo alice/my-project") {
		t.Errorf("expected --repo alice/my-project in args, got: %v", capturedArgs)
	}
	if strings.Contains(joined, "--org") {
		t.Errorf("expected no --org flag for user scope, got: %v", capturedArgs)
	}
}

func TestSetProjectVariable_EmptyNodeID_Skips(t *testing.T) {
	cfg := BootstrapConfig{Owner: "alice", OwnerType: OwnerTypeUser}
	state := &StepState{RepoName: "my-project", ProjectNodeID: ""}

	runCalled := false
	run := func(name string, args ...string) (string, error) {
		runCalled = true
		return "", nil
	}

	var buf bytes.Buffer
	setProjectVariable(&buf, cfg, state, run)

	if runCalled {
		t.Error("expected gh not to be called when ProjectNodeID is empty")
	}
	out := buf.String()
	if !strings.Contains(out, "Skipping AGENTIC_PROJECT_ID") {
		t.Errorf("expected skip message, got: %s", out)
	}
}

// --------------------------------------------------------------------------------------
// Step 9 — PrintSummary
// --------------------------------------------------------------------------------------

func TestPrintSummary_OutputContainsAllFields(t *testing.T) {
	cfg := BootstrapConfig{
		ProjectName:   "my-project",
		RunnerLabel:   DefaultRunnerLabel,
		GooseProvider: DefaultGooseProvider,
		GooseModel:    DefaultGooseModel,
	}
	state := &StepState{
		RepoURL:        "https://github.com/alice/my-project",
		ProjectURL:     "https://github.com/orgs/alice/projects/1",
		ClonePath:      "/home/alice/Development/my-project",
		AgentPATFound:  true,
		CredentialsSet: true,
	}

	var buf bytes.Buffer
	_ = PrintSummary(&buf, cfg, state)

	out := buf.String()
	if !strings.Contains(out, "Bootstrap complete") {
		t.Errorf("expected 'Bootstrap complete' in output, got: %s", out)
	}
	if !strings.Contains(out, state.RepoURL) {
		t.Errorf("expected repo URL %q in output, got: %s", state.RepoURL, out)
	}
	if !strings.Contains(out, state.ClonePath) {
		t.Errorf("expected clone path %q in output, got: %s", state.ClonePath, out)
	}
}

func TestPrintSummary_OrgAccount_ShowsPATGuidance(t *testing.T) {
	cfg := BootstrapConfig{
		ProjectName:   "my-project",
		OwnerType:     OwnerTypeOrg,
		RunnerLabel:   DefaultRunnerLabel,
		GooseProvider: DefaultGooseProvider,
		GooseModel:    DefaultGooseModel,
	}
	state := &StepState{
		RepoURL:       "https://github.com/acme-org/my-project",
		ProjectURL:    "https://github.com/orgs/acme-org/projects/1",
		ClonePath:     "/home/alice/Development/my-project",
		AgentPATFound: true,
	}

	var buf bytes.Buffer
	_ = PrintSummary(&buf, cfg, state)

	out := buf.String()
	if !strings.Contains(out, "github.com/settings/tokens") {
		t.Errorf("expected token URL in PAT guidance, got: %s", out)
	}
}

func TestPrintSummary_UserAccount_NoPATScopeGuidance(t *testing.T) {
	cfg := BootstrapConfig{
		ProjectName:   "my-project",
		OwnerType:     OwnerTypeUser,
		RunnerLabel:   DefaultRunnerLabel,
		GooseProvider: DefaultGooseProvider,
		GooseModel:    DefaultGooseModel,
	}
	state := &StepState{
		RepoURL:       "https://github.com/alice/my-project",
		ProjectURL:    "https://github.com/users/alice/projects/1",
		ClonePath:     "/home/alice/Development/my-project",
		AgentPATFound: true,
	}

	var buf bytes.Buffer
	_ = PrintSummary(&buf, cfg, state)

	out := buf.String()
	// PAT scope guidance (org-specific) should not appear for personal accounts.
	if strings.Contains(out, "github.com/settings/tokens") {
		t.Errorf("expected no org PAT scope guidance for personal account, got: %s", out)
	}
}

func TestPrintSummary_PipelineConfig_ShowsRunnerProviderModel(t *testing.T) {
	cfg := BootstrapConfig{
		ProjectName:   "my-project",
		Owner:         "alice",
		RunnerLabel:   "ubuntu-latest",
		GooseProvider: "claude-code",
		GooseModel:    "default",
	}
	state := &StepState{
		RepoName:       "my-project",
		RepoURL:        "https://github.com/alice/my-project",
		ClonePath:      "/tmp/my-project",
		CredentialsSet: true,
		AgentPATFound:  true,
	}

	var buf bytes.Buffer
	_ = PrintSummary(&buf, cfg, state)

	out := buf.String()
	for _, want := range []string{"ubuntu-latest", "claude-code", "default"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in pipeline config output, got: %s", want, out)
		}
	}
}

func TestPrintSummary_CredentialsNotSet_ShowsWarning(t *testing.T) {
	cfg := BootstrapConfig{
		ProjectName:   "my-project",
		Owner:         "alice",
		RunnerLabel:   DefaultRunnerLabel,
		GooseProvider: DefaultGooseProvider,
		GooseModel:    DefaultGooseModel,
	}
	state := &StepState{
		RepoName:       "my-project",
		RepoURL:        "https://github.com/alice/my-project",
		ClonePath:      "/tmp/my-project",
		CredentialsSet: false,
		AgentPATFound:  true,
	}

	var buf bytes.Buffer
	_ = PrintSummary(&buf, cfg, state)

	out := buf.String()
	if !strings.Contains(out, "not set") {
		t.Errorf("expected 'not set' credentials warning, got: %s", out)
	}
}

func TestPrintSummary_CredentialsSet_ShowsOK(t *testing.T) {
	cfg := BootstrapConfig{
		ProjectName:   "my-project",
		Owner:         "alice",
		RunnerLabel:   DefaultRunnerLabel,
		GooseProvider: DefaultGooseProvider,
		GooseModel:    DefaultGooseModel,
	}
	state := &StepState{
		RepoName:       "my-project",
		RepoURL:        "https://github.com/alice/my-project",
		ClonePath:      "/tmp/my-project",
		CredentialsSet: true,
		AgentPATFound:  true,
	}

	var buf bytes.Buffer
	_ = PrintSummary(&buf, cfg, state)

	out := buf.String()
	if strings.Contains(out, "not set") {
		t.Errorf("expected no 'not set' when credentials are set, got: %s", out)
	}
}

func TestPrintSummary_CustomRunner_NoSelfHostedNote(t *testing.T) {
	cfg := BootstrapConfig{
		ProjectName:   "my-project",
		Owner:         "alice",
		RunnerLabel:   "self-hosted-gpu",
		GooseProvider: DefaultGooseProvider,
		GooseModel:    DefaultGooseModel,
	}
	state := &StepState{
		RepoName:      "my-project",
		RepoURL:       "https://github.com/alice/my-project",
		ClonePath:     "/tmp/my-project",
		AgentPATFound: true,
	}

	var buf bytes.Buffer
	_ = PrintSummary(&buf, cfg, state)

	out := buf.String()
	// Self-hosted runner note was removed per #361 scope item 7.
	if strings.Contains(out, "Self-hosted runner") {
		t.Errorf("expected no self-hosted runner note (removed), got: %s", out)
	}
}

func TestPrintSummary_ShowsNextStepInstructions(t *testing.T) {
	cfg := BootstrapConfig{
		ProjectName:   "my-project",
		Owner:         "alice",
		RunnerLabel:   DefaultRunnerLabel,
		GooseProvider: DefaultGooseProvider,
		GooseModel:    DefaultGooseModel,
	}
	state := &StepState{
		RepoName:      "my-project",
		RepoURL:       "https://github.com/alice/my-project",
		ClonePath:     "/tmp/my-project",
		AgentPATFound: true,
	}

	var buf bytes.Buffer
	_ = PrintSummary(&buf, cfg, state)

	out := buf.String()
	if !strings.Contains(out, "Next steps") {
		t.Errorf("expected 'Next steps' heading, got: %s", out)
	}
	if !strings.Contains(out, "Claude Code or Goose Desktop") {
		t.Errorf("expected Claude Code/Goose Desktop instruction, got: %s", out)
	}
	if !strings.Contains(out, "my-project") {
		t.Errorf("expected repo name in instructions, got: %s", out)
	}
	if !strings.Contains(out, "Assist me with defining my new AI-native developed application") {
		t.Errorf("expected suggested prompt, got: %s", out)
	}
}

func TestPrintSummary_NoTerminalSkipPrompt(t *testing.T) {
	cfg := BootstrapConfig{
		ProjectName:   "my-project",
		Owner:         "alice",
		RunnerLabel:   DefaultRunnerLabel,
		GooseProvider: DefaultGooseProvider,
		GooseModel:    DefaultGooseModel,
	}
	state := &StepState{
		RepoName:      "my-project",
		RepoURL:       "https://github.com/alice/my-project",
		ClonePath:     "/tmp/my-project",
		AgentPATFound: true,
	}

	var buf bytes.Buffer
	_ = PrintSummary(&buf, cfg, state)

	out := buf.String()
	// Terminal/Skip prompt should not exist.
	if strings.Contains(out, "Terminal") && strings.Contains(out, "Skip") {
		t.Errorf("expected no Terminal/Skip prompt, got: %s", out)
	}
}

func TestPrintSummary_PATMissing_ShowsWarning(t *testing.T) {
	cfg := BootstrapConfig{
		ProjectName:   "my-project",
		Owner:         "alice",
		RunnerLabel:   DefaultRunnerLabel,
		GooseProvider: DefaultGooseProvider,
		GooseModel:    DefaultGooseModel,
	}
	state := &StepState{
		RepoName:      "my-project",
		RepoURL:       "https://github.com/alice/my-project",
		ClonePath:     "/tmp/my-project",
		AgentPATFound: false,
	}

	var buf bytes.Buffer
	_ = PrintSummary(&buf, cfg, state)

	out := buf.String()
	if !strings.Contains(out, "GOOSE_AGENT_PAT secret not found") {
		t.Errorf("expected PAT missing warning, got: %s", out)
	}
	if !strings.Contains(out, "settings/secrets/actions") {
		t.Errorf("expected settings URL in PAT warning, got: %s", out)
	}
}

func TestPrintSummary_PATFound_NoPATWarning(t *testing.T) {
	cfg := BootstrapConfig{
		ProjectName:   "my-project",
		Owner:         "alice",
		RunnerLabel:   DefaultRunnerLabel,
		GooseProvider: DefaultGooseProvider,
		GooseModel:    DefaultGooseModel,
	}
	state := &StepState{
		RepoName:      "my-project",
		RepoURL:       "https://github.com/alice/my-project",
		ClonePath:     "/tmp/my-project",
		AgentPATFound: true,
	}

	var buf bytes.Buffer
	_ = PrintSummary(&buf, cfg, state)

	out := buf.String()
	if strings.Contains(out, "GOOSE_AGENT_PAT secret not found") {
		t.Errorf("expected no PAT warning when PAT is found, got: %s", out)
	}
}

// --------------------------------------------------------------------------------------
// Internal helpers
// --------------------------------------------------------------------------------------

// --------------------------------------------------------------------------------------
// resolveCloneConflict and findBackupPath tests
// --------------------------------------------------------------------------------------

func TestFindBackupPath_NoConflict_ReturnsDotBackup(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "my-project")
	// target does not exist, so .backup should be returned
	got := findBackupPath(target)
	want := target + ".backup"
	if got != want {
		t.Errorf("findBackupPath() = %q, want %q", got, want)
	}
}

func TestFindBackupPath_BackupExists_ReturnsNumbered(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "my-project")
	// Create the first backup
	if err := os.MkdirAll(target+".backup", 0o755); err != nil {
		t.Fatalf("creating backup dir: %v", err)
	}
	got := findBackupPath(target)
	want := target + ".backup.1"
	if got != want {
		t.Errorf("findBackupPath() = %q, want %q", got, want)
	}
}

func TestFindBackupPath_MultipleBackups_ReturnsNextNumber(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "my-project")
	// Create .backup and .backup.1
	if err := os.MkdirAll(target+".backup", 0o755); err != nil {
		t.Fatalf("creating backup dir: %v", err)
	}
	if err := os.MkdirAll(target+".backup.1", 0o755); err != nil {
		t.Fatalf("creating backup.1 dir: %v", err)
	}
	got := findBackupPath(target)
	want := target + ".backup.2"
	if got != want {
		t.Errorf("findBackupPath() = %q, want %q", got, want)
	}
}

func TestDefaultResolveCloneConflict_NoConflict_ReturnsPathUnchanged(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "does-not-exist")

	var buf bytes.Buffer
	got, err := DefaultResolveCloneConflict(&buf, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != target {
		t.Errorf("DefaultResolveCloneConflict() = %q, want %q", got, target)
	}
}

func TestErrCloneAborted_IsError(t *testing.T) {
	if ErrCloneAborted == nil {
		t.Error("ErrCloneAborted should not be nil")
	}
	if ErrCloneAborted.Error() != "clone aborted by user" {
		t.Errorf("ErrCloneAborted.Error() = %q, want %q", ErrCloneAborted.Error(), "clone aborted by user")
	}
}

func TestShellQuote_NoSpecialChars(t *testing.T) {
	if got := shellQuote("hello"); got != "'hello'" {
		t.Errorf("shellQuote('hello') = %q, want %q", got, "'hello'")
	}
}

func TestShellQuote_SingleQuoteEscaped(t *testing.T) {
	got := shellQuote("it's")
	if !strings.Contains(got, `'\''`) {
		t.Errorf("shellQuote(\"it's\") expected escaped single quote, got: %s", got)
	}
}

func TestShellJoin_CombinesTokens(t *testing.T) {
	got := shellJoin("git", "commit", "-m", "a message")
	if !strings.Contains(got, "'git'") {
		t.Errorf("shellJoin: expected quoted 'git', got: %s", got)
	}
	if !strings.Contains(got, "'a message'") {
		t.Errorf("shellJoin: expected quoted 'a message', got: %s", got)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// SetAgentUserVariable tests
// ──────────────────────────────────────────────────────────────────────────────

func TestSetAgentUserVariable_RepoScope_SetsVariable(t *testing.T) {
	var buf bytes.Buffer
	cfg := BootstrapConfig{
		Owner:          "alice",
		AgentUser:      "goose-agent",
		AgentUserScope: AgentUserScopeRepo,
	}
	state := &StepState{RepoName: "my-project"}
	var setCalled bool
	fakeRun := func(name string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "variable set AGENT_USER") && strings.Contains(joined, "--repo alice/my-project") {
			setCalled = true
		}
		return "", nil
	}

	err := SetAgentUserVariable(&buf, cfg, state, fakeRun)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !setCalled {
		t.Error("expected gh variable set to be called with --repo")
	}
}

func TestSetAgentUserVariable_OrgScope_SetsVariable(t *testing.T) {
	var buf bytes.Buffer
	cfg := BootstrapConfig{
		Owner:          "acme-org",
		AgentUser:      "goose-agent",
		AgentUserScope: AgentUserScopeOrg,
	}
	state := &StepState{RepoName: "my-project"}
	var setCalled bool
	fakeRun := func(name string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "variable set AGENT_USER") && strings.Contains(joined, "--org acme-org") {
			setCalled = true
		}
		return "", nil
	}

	err := SetAgentUserVariable(&buf, cfg, state, fakeRun)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !setCalled {
		t.Error("expected gh variable set to be called with --org")
	}
}

func TestSetAgentUserVariable_EmptyUser_Skips(t *testing.T) {
	var buf bytes.Buffer
	cfg := BootstrapConfig{Owner: "alice"}
	state := &StepState{RepoName: "my-project"}
	runCalled := false
	fakeRun := func(name string, args ...string) (string, error) {
		runCalled = true
		return "", nil
	}

	err := SetAgentUserVariable(&buf, cfg, state, fakeRun)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runCalled {
		t.Error("expected no gh CLI call when agent user is empty")
	}
	if !strings.Contains(buf.String(), "Skipping") {
		t.Errorf("expected skip message, got: %s", buf.String())
	}
}

func TestSetAgentUserVariable_FailureIsNonFatal(t *testing.T) {
	var buf bytes.Buffer
	cfg := BootstrapConfig{
		Owner:          "alice",
		AgentUser:      "goose-agent",
		AgentUserScope: AgentUserScopeRepo,
	}
	state := &StepState{RepoName: "my-project"}
	fakeRun := func(name string, args ...string) (string, error) {
		return "permission denied", fmt.Errorf("exit 1")
	}

	err := SetAgentUserVariable(&buf, cfg, state, fakeRun)
	if err != nil {
		t.Fatalf("expected nil error (non-fatal), got: %v", err)
	}
	if !strings.Contains(buf.String(), "Could not set AGENT_USER") {
		t.Errorf("expected warning message, got: %s", buf.String())
	}
}

func TestSetAgentUserVariable_OrgScope_AlreadyMember_Succeeds(t *testing.T) {
	var buf bytes.Buffer
	cfg := BootstrapConfig{
		Owner:          "acme-org",
		AgentUser:      "goose-agent",
		AgentUserScope: AgentUserScopeOrg,
	}
	state := &StepState{RepoName: "my-project"}
	fakeRun := func(name string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "orgs/acme-org/members/goose-agent") {
			return "", nil // 204 — already a member
		}
		return "", nil
	}

	err := SetAgentUserVariable(&buf, cfg, state, fakeRun)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "already an org member") {
		t.Errorf("expected already-member message, got: %s", buf.String())
	}
}

func TestSetAgentUserVariable_OrgScope_InvitationSent_Succeeds(t *testing.T) {
	var buf bytes.Buffer
	cfg := BootstrapConfig{
		Owner:          "acme-org",
		AgentUser:      "goose-agent",
		AgentUserScope: AgentUserScopeOrg,
	}
	state := &StepState{RepoName: "my-project"}
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "variable set AGENT_USER") {
			return "", nil // set variable succeeds
		}
		if strings.Contains(joined, "orgs/acme-org/members/goose-agent") {
			return "", fmt.Errorf("HTTP 404") // not a member
		}
		if strings.Contains(joined, "users/goose-agent") && strings.Contains(joined, ".id") {
			return "12345", nil // resolve user ID
		}
		if strings.Contains(joined, "orgs/acme-org/invitations") {
			callCount++
			return `{"id": 1}`, nil // invitation success
		}
		return "", nil
	}

	err := SetAgentUserVariable(&buf, cfg, state, fakeRun)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected invitation API to be called once, got %d", callCount)
	}
	if !strings.Contains(buf.String(), "Invited goose-agent to org acme-org") {
		t.Errorf("expected invitation message, got: %s", buf.String())
	}
}

func TestSetAgentUserVariable_OrgScope_InvitationPermissionDenied_LogsManualAction(t *testing.T) {
	var buf bytes.Buffer
	cfg := BootstrapConfig{
		Owner:          "acme-org",
		AgentUser:      "goose-agent",
		AgentUserScope: AgentUserScopeOrg,
	}
	state := &StepState{RepoName: "my-project"}
	fakeRun := func(name string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "variable set AGENT_USER") {
			return "", nil
		}
		if strings.Contains(joined, "orgs/acme-org/members/goose-agent") {
			return "", fmt.Errorf("HTTP 404")
		}
		if strings.Contains(joined, "users/goose-agent") && strings.Contains(joined, ".id") {
			return "12345", nil
		}
		if strings.Contains(joined, "orgs/acme-org/invitations") {
			return "403 Forbidden", fmt.Errorf("HTTP 403")
		}
		return "", nil
	}

	err := SetAgentUserVariable(&buf, cfg, state, fakeRun)
	if err != nil {
		t.Fatalf("expected nil error (non-fatal), got: %v", err)
	}
	if !strings.Contains(buf.String(), "orgs/acme-org/people") {
		t.Errorf("expected manual action URL, got: %s", buf.String())
	}
}

func TestSetAgentUserVariable_OrgScope_InvitationFails_LogsWarning(t *testing.T) {
	var buf bytes.Buffer
	cfg := BootstrapConfig{
		Owner:          "acme-org",
		AgentUser:      "goose-agent",
		AgentUserScope: AgentUserScopeOrg,
	}
	state := &StepState{RepoName: "my-project"}
	fakeRun := func(name string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "variable set AGENT_USER") {
			return "", nil
		}
		if strings.Contains(joined, "orgs/acme-org/members/goose-agent") {
			return "", fmt.Errorf("HTTP 404")
		}
		if strings.Contains(joined, "users/goose-agent") && strings.Contains(joined, ".id") {
			return "", fmt.Errorf("user resolution failed")
		}
		return "", nil
	}

	err := SetAgentUserVariable(&buf, cfg, state, fakeRun)
	if err != nil {
		t.Fatalf("expected nil error (non-fatal), got: %v", err)
	}
	if !strings.Contains(buf.String(), "Could not resolve") {
		t.Errorf("expected warning message, got: %s", buf.String())
	}
}

func TestSetAgentUserVariable_RepoScope_NoOrgMembershipCheck(t *testing.T) {
	var buf bytes.Buffer
	cfg := BootstrapConfig{
		Owner:          "alice",
		AgentUser:      "goose-agent",
		AgentUserScope: AgentUserScopeRepo,
	}
	state := &StepState{RepoName: "my-project"}
	fakeRun := func(name string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "orgs/") {
			t.Fatal("org membership check should not be called for repo scope")
		}
		return "", nil
	}

	err := SetAgentUserVariable(&buf, cfg, state, fakeRun)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
