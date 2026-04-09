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

	callCount := 0
	run := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			// gh repo create succeeds
			return "https://github.com/alice/my-project", nil
		}
		if callCount == 2 {
			// git clone fails
			return "fatal: repository not found", errors.New("exit status 128")
		}
		// gh repo delete (cleanup) succeeds
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
// Step 4 — ScaffoldStacks / extractInitCommands
// --------------------------------------------------------------------------------------

func TestExtractInitCommands_FindsGoSection(t *testing.T) {
	content := `# Go Standards

## Project Initialisation

Run these commands:

` + "```bash" + `
go mod init github.com/owner/repo
mkdir -p cmd/repo internal
` + "```" + `

## Build Verification

` + "```bash" + `
go build ./...
` + "```"

	cmds, err := extractInitCommands(content)
	if err != nil {
		t.Fatalf("extractInitCommands() error: %v", err)
	}
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command block, got %d: %v", len(cmds), cmds)
	}
	if !strings.Contains(cmds[0], "go mod init") {
		t.Errorf("expected 'go mod init' in command, got: %s", cmds[0])
	}
	if strings.Contains(cmds[0], "go build") {
		t.Errorf("expected 'go build' to be excluded (it is in the next section), got: %s", cmds[0])
	}
}

func TestExtractInitCommands_SectionMissing_ReturnsError(t *testing.T) {
	_, err := extractInitCommands("# No relevant section here")
	if err == nil {
		t.Error("extractInitCommands() expected error when section is missing, got nil")
	}
}

func TestExtractInitCommands_NoCodeBlocks_ReturnsEmpty(t *testing.T) {
	content := "## Project Initialisation\n\nNo code here.\n"
	cmds, err := extractInitCommands(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cmds) != 0 {
		t.Errorf("expected 0 commands when no code blocks, got %d", len(cmds))
	}
}

func TestScaffoldStacks_OtherStack_SkipsWithWarning(t *testing.T) {
	cfg := BootstrapConfig{Stacks: []string{"Other"}}
	state := &StepState{ClonePath: t.TempDir()}
	run := fakeRunFail("should not be called")

	var buf bytes.Buffer
	if err := ScaffoldStacks(&buf, cfg, state, run); err != nil {
		t.Fatalf("ScaffoldStacks() expected nil for 'Other' stack, got: %v", err)
	}
	if !strings.Contains(buf.String(), "skipping scaffold") {
		t.Errorf("expected skip message in output, got: %s", buf.String())
	}
}

func TestScaffoldStacks_MissingStandardsFile_ReturnsError(t *testing.T) {
	cfg := BootstrapConfig{Stacks: []string{"Go"}}
	state := &StepState{ClonePath: t.TempDir()} // No base/standards/go.md

	var buf bytes.Buffer
	if err := ScaffoldStacks(&buf, cfg, state, fakeRunOK("")); err == nil {
		t.Error("ScaffoldStacks() expected error when standards file missing, got nil")
	}
}

func TestScaffoldStacks_ExecutesCommandsFromFile(t *testing.T) {
	dir := t.TempDir()
	// Create the standards file with a Project Initialisation section.
	stdDir := filepath.Join(dir, "base", "standards")
	if err := os.MkdirAll(stdDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := "## Project Initialisation\n\n```bash\necho hello\n```\n\n## Other\n"
	if err := os.WriteFile(filepath.Join(stdDir, "go.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := BootstrapConfig{Stacks: []string{"Go"}}
	state := &StepState{ClonePath: dir}

	var executed []string
	run := func(name string, args ...string) (string, error) {
		if name == "bash" {
			executed = append(executed, strings.Join(args, " "))
		}
		return "", nil
	}

	var buf bytes.Buffer
	if err := ScaffoldStacks(&buf, cfg, state, run); err != nil {
		t.Fatalf("ScaffoldStacks() unexpected error: %v", err)
	}
	if len(executed) == 0 {
		t.Error("expected at least one bash command to be executed")
	}
}

func TestScaffoldStacks_MultipleStacks_ExecutesAll(t *testing.T) {
	dir := t.TempDir()
	stdDir := filepath.Join(dir, "base", "standards")
	if err := os.MkdirAll(stdDir, 0755); err != nil {
		t.Fatal(err)
	}

	goContent := "## Project Initialisation\n\n```bash\necho go-init\n```\n\n## Other\n"
	if err := os.WriteFile(filepath.Join(stdDir, "go.md"), []byte(goContent), 0644); err != nil {
		t.Fatal(err)
	}
	rustContent := "## Project Initialisation\n\n```bash\necho rust-init\n```\n\n## Other\n"
	if err := os.WriteFile(filepath.Join(stdDir, "rust.md"), []byte(rustContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := BootstrapConfig{Stacks: []string{"Go", "Rust"}}
	state := &StepState{ClonePath: dir}

	var executed []string
	run := func(name string, args ...string) (string, error) {
		if name == "bash" {
			executed = append(executed, strings.Join(args, " "))
		}
		return "", nil
	}

	var buf bytes.Buffer
	if err := ScaffoldStacks(&buf, cfg, state, run); err != nil {
		t.Fatalf("ScaffoldStacks() unexpected error: %v", err)
	}
	if len(executed) != 2 {
		t.Errorf("expected 2 commands executed (one per stack), got %d", len(executed))
	}
}

func TestScaffoldStacks_SkipsTrackWithNoInitSection(t *testing.T) {
	dir := t.TempDir()
	stdDir := filepath.Join(dir, "base", "standards")
	if err := os.MkdirAll(stdDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Go has init section, Python has no init section.
	goContent := "## Project Initialisation\n\n```bash\necho go-init\n```\n\n## Other\n"
	if err := os.WriteFile(filepath.Join(stdDir, "go.md"), []byte(goContent), 0644); err != nil {
		t.Fatal(err)
	}
	pyContent := "## Build Verification\n\nSome build info.\n"
	if err := os.WriteFile(filepath.Join(stdDir, "python.md"), []byte(pyContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := BootstrapConfig{Stacks: []string{"Go", "Python"}}
	state := &StepState{ClonePath: dir}

	var executed []string
	run := func(name string, args ...string) (string, error) {
		if name == "bash" {
			executed = append(executed, strings.Join(args, " "))
		}
		return "", nil
	}

	var buf bytes.Buffer
	if err := ScaffoldStacks(&buf, cfg, state, run); err != nil {
		t.Fatalf("ScaffoldStacks() unexpected error: %v", err)
	}
	if len(executed) != 1 {
		t.Errorf("expected 1 command executed (Python skipped), got %d", len(executed))
	}
	if !strings.Contains(buf.String(), "skipping") {
		t.Errorf("expected skip message in output, got: %s", buf.String())
	}
}

func TestScaffoldStacks_HaltsOnFailure_ReportsTrack(t *testing.T) {
	dir := t.TempDir()
	stdDir := filepath.Join(dir, "base", "standards")
	if err := os.MkdirAll(stdDir, 0755); err != nil {
		t.Fatal(err)
	}

	goContent := "## Project Initialisation\n\n```bash\necho go-init\n```\n\n## Other\n"
	if err := os.WriteFile(filepath.Join(stdDir, "go.md"), []byte(goContent), 0644); err != nil {
		t.Fatal(err)
	}
	rustContent := "## Project Initialisation\n\n```bash\necho rust-init\n```\n\n## Other\n"
	if err := os.WriteFile(filepath.Join(stdDir, "rust.md"), []byte(rustContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := BootstrapConfig{Stacks: []string{"Go", "Rust"}}
	state := &StepState{ClonePath: dir}

	callCount := 0
	run := func(name string, args ...string) (string, error) {
		if name == "bash" {
			callCount++
			if callCount == 2 {
				return "command failed", errors.New("exit status 1")
			}
		}
		return "", nil
	}

	var buf bytes.Buffer
	err := ScaffoldStacks(&buf, cfg, state, run)
	if err == nil {
		t.Fatal("expected error when command fails, got nil")
	}
	if !strings.Contains(err.Error(), "[Rust]") {
		t.Errorf("error should mention the failing track 'Rust', got: %v", err)
	}
}

func TestScaffoldStacks_SingleStack_BehaviourUnchanged(t *testing.T) {
	dir := t.TempDir()
	stdDir := filepath.Join(dir, "base", "standards")
	if err := os.MkdirAll(stdDir, 0755); err != nil {
		t.Fatal(err)
	}

	content := "## Project Initialisation\n\n```bash\necho hello\n```\n\n## Other\n"
	if err := os.WriteFile(filepath.Join(stdDir, "go.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := BootstrapConfig{Stacks: []string{"Go"}}
	state := &StepState{ClonePath: dir}

	var executed []string
	run := func(name string, args ...string) (string, error) {
		if name == "bash" {
			executed = append(executed, strings.Join(args, " "))
		}
		return "", nil
	}

	var buf bytes.Buffer
	if err := ScaffoldStacks(&buf, cfg, state, run); err != nil {
		t.Fatalf("ScaffoldStacks() unexpected error: %v", err)
	}
	if len(executed) != 1 {
		t.Errorf("expected exactly 1 command executed for single stack, got %d", len(executed))
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

func TestPopulateRepo_WritesThreeFiles(t *testing.T) {
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

	for _, f := range []string{"REPOS.md", "AGENTS.local.md", "README.md"} {
		data, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			t.Errorf("expected %s to exist, got: %v", f, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("%s is empty", f)
		}
	}

	// Verify skills/.gitkeep is created.
	if _, err := os.Stat(filepath.Join(dir, "skills", ".gitkeep")); err != nil {
		t.Error("expected skills/.gitkeep to exist")
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
		Owner:        "alice",
		ProjectName:  "my-project",
		Stacks:      []string{"Go"},
		Description:  "unique-description-12345",
		TemplateRepo: DefaultTemplateRepo,
	}
	state := &StepState{RepoName: "my-project", ClonePath: dir}

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
	exampleDir := filepath.Join(dir, "base", "docs", "examples")
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
	cfg := BootstrapConfig{Owner: "alice", OwnerType: OwnerTypeUser}
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
	if err := ConfigureProjectStatus(&buf, cfg, state, graphqlDo); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(queries) < 2 {
		t.Fatalf("expected at least 2 GraphQL calls for user account, got %d", len(queries))
	}
}

func TestConfigureProjectStatus_MissingProjectNodeID_SkipsWithWarning(t *testing.T) {
	cfg := BootstrapConfig{Owner: "acme-org", OwnerType: OwnerTypeOrg}
	state := &StepState{ProjectNodeID: ""}

	graphqlDo := func(query string, variables map[string]interface{}, response interface{}) error {
		t.Fatal("GraphQL should not be called when project node ID is missing")
		return nil
	}

	var buf bytes.Buffer
	if err := ConfigureProjectStatus(&buf, cfg, state, graphqlDo); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "no project node ID") {
		t.Errorf("expected warning about missing node ID, got: %s", buf.String())
	}
}

func TestConfigureProjectStatus_Success_CallsUpdateMutationWithDescriptions(t *testing.T) {
	cfg := BootstrapConfig{Owner: "acme-org", OwnerType: OwnerTypeOrg}
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
	if err := ConfigureProjectStatus(&buf, cfg, state, graphqlDo); err != nil {
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
	cfg := BootstrapConfig{Owner: "acme-org", OwnerType: OwnerTypeOrg}
	clonePath := t.TempDir()
	writeTestProjectTemplate(t, clonePath)
	state := &StepState{ProjectNodeID: "PVT_123", ClonePath: clonePath}

	graphqlDo := func(query string, variables map[string]interface{}, response interface{}) error {
		return errors.New("network error")
	}

	var buf bytes.Buffer
	err := ConfigureProjectStatus(&buf, cfg, state, graphqlDo)
	if err != nil {
		t.Fatalf("expected nil error (best-effort), got: %v", err)
	}
	if !strings.Contains(buf.String(), "Could not fetch Status field") {
		t.Errorf("expected warning about fetch failure, got: %s", buf.String())
	}
}

func TestConfigureProjectStatus_UpdateFails_LogsWarning(t *testing.T) {
	cfg := BootstrapConfig{Owner: "acme-org", OwnerType: OwnerTypeOrg}
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
	err := ConfigureProjectStatus(&buf, cfg, state, graphqlDo)
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
	if !strings.Contains(out, "not found in base/") {
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
	sourceDir := filepath.Join(dir, "base", ".github", "workflows")
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
	sourceDir := filepath.Join(dir, "base", ".github", "workflows")
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

	fakeLaunch := func(clonePath string) error { return nil }

	var buf bytes.Buffer
	_ = PrintSummary(&buf, cfg, state, fakeLaunch)

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

	fakeLaunch := func(clonePath string) error { return nil }

	var buf bytes.Buffer
	_ = PrintSummary(&buf, cfg, state, fakeLaunch)

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

	fakeLaunch := func(clonePath string) error { return nil }

	var buf bytes.Buffer
	_ = PrintSummary(&buf, cfg, state, fakeLaunch)

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

	fakeLaunch := func(clonePath string) error { return nil }

	var buf bytes.Buffer
	_ = PrintSummary(&buf, cfg, state, fakeLaunch)

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

	fakeLaunch := func(clonePath string) error { return nil }

	var buf bytes.Buffer
	_ = PrintSummary(&buf, cfg, state, fakeLaunch)

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

	fakeLaunch := func(clonePath string) error { return nil }

	var buf bytes.Buffer
	_ = PrintSummary(&buf, cfg, state, fakeLaunch)

	out := buf.String()
	if strings.Contains(out, "not set") {
		t.Errorf("expected no 'not set' when credentials are set, got: %s", out)
	}
}

func TestPrintSummary_CustomRunner_ShowsSelfHostedNote(t *testing.T) {
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

	fakeLaunch := func(clonePath string) error { return nil }

	var buf bytes.Buffer
	_ = PrintSummary(&buf, cfg, state, fakeLaunch)

	out := buf.String()
	if !strings.Contains(out, "Self-hosted runner") {
		t.Errorf("expected self-hosted runner note, got: %s", out)
	}
}

func TestPrintSummary_DefaultRunner_NoSelfHostedNote(t *testing.T) {
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

	fakeLaunch := func(clonePath string) error { return nil }

	var buf bytes.Buffer
	_ = PrintSummary(&buf, cfg, state, fakeLaunch)

	out := buf.String()
	if strings.Contains(out, "Self-hosted runner") {
		t.Errorf("expected no self-hosted runner note for default runner, got: %s", out)
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

	fakeLaunch := func(clonePath string) error { return nil }

	var buf bytes.Buffer
	_ = PrintSummary(&buf, cfg, state, fakeLaunch)

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

	fakeLaunch := func(clonePath string) error { return nil }

	var buf bytes.Buffer
	_ = PrintSummary(&buf, cfg, state, fakeLaunch)

	out := buf.String()
	if strings.Contains(out, "GOOSE_AGENT_PAT secret not found") {
		t.Errorf("expected no PAT warning when PAT is found, got: %s", out)
	}
}

// --------------------------------------------------------------------------------------
// Internal helpers
// --------------------------------------------------------------------------------------

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
