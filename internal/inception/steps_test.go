package inception

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
)

// --------------------------------------------------------------------------------------
// Test helpers
// --------------------------------------------------------------------------------------

// fakeRunOK returns a RunCommandFunc that always succeeds with the given output.
func fakeRunOK(out string) bootstrap.RunCommandFunc {
	return func(name string, args ...string) (string, error) {
		return out, nil
	}
}

// fakeRunFail returns a RunCommandFunc that always fails with the given error.
func fakeRunFail(msg string) bootstrap.RunCommandFunc {
	return func(name string, args ...string) (string, error) {
		return msg, errors.New(msg)
	}
}

// --------------------------------------------------------------------------------------
// Step 1 — CreateRepo
// --------------------------------------------------------------------------------------

func TestCreateRepo_Success_PopulatesState(t *testing.T) {
	dir := t.TempDir()
	cfg := &InceptionConfig{
		RepoType: "domain",
		RepoName: "charging",
		Owner:    "acme-org",
	}
	state := &StepState{}
	env := &EnvContext{AgenticRepoRoot: dir, Owner: "acme-org"}

	run := fakeRunOK("")

	var buf bytes.Buffer
	err := CreateRepo(&buf, cfg, state, env, run)
	if err != nil {
		t.Fatalf("CreateRepo() unexpected error: %v", err)
	}

	if state.RepoName != "charging-domain" {
		t.Errorf("state.RepoName = %q, want %q", state.RepoName, "charging-domain")
	}
	if state.RepoURL != "https://github.com/acme-org/charging-domain" {
		t.Errorf("state.RepoURL = %q, want %q", state.RepoURL, "https://github.com/acme-org/charging-domain")
	}
	expectedClone := filepath.Join(dir, "domains", "charging-domain")
	if state.ClonePath != expectedClone {
		t.Errorf("state.ClonePath = %q, want %q", state.ClonePath, expectedClone)
	}
}

func TestCreateRepo_GhCreateFails_ReturnsError(t *testing.T) {
	cfg := &InceptionConfig{RepoType: "domain", RepoName: "charging", Owner: "acme"}
	state := &StepState{}
	env := &EnvContext{AgenticRepoRoot: t.TempDir()}

	run := fakeRunFail("already exists")

	var buf bytes.Buffer
	err := CreateRepo(&buf, cfg, state, env, run)
	if err == nil {
		t.Fatal("CreateRepo() expected error when gh repo create fails, got nil")
	}
	if !strings.Contains(err.Error(), "gh repo create") {
		t.Errorf("error should mention 'gh repo create', got: %v", err)
	}
}

func TestCreateRepo_CloneFails_ReturnsError(t *testing.T) {
	cfg := &InceptionConfig{RepoType: "tool", RepoName: "bench", Owner: "acme"}
	state := &StepState{}
	env := &EnvContext{AgenticRepoRoot: t.TempDir()}

	callCount := 0
	run := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			return "https://github.com/acme/bench-tool", nil
		}
		return "fatal: repository not found", errors.New("exit status 128")
	}

	var buf bytes.Buffer
	err := CreateRepo(&buf, cfg, state, env, run)
	if err == nil {
		t.Fatal("CreateRepo() expected error when git clone fails, got nil")
	}
	if !strings.Contains(err.Error(), "git clone") {
		t.Errorf("error should mention 'git clone', got: %v", err)
	}
}

func TestCreateRepo_OtherType_UsesOthersDir(t *testing.T) {
	dir := t.TempDir()
	cfg := &InceptionConfig{RepoType: "other", RepoName: "my-service", Owner: "alice"}
	state := &StepState{}
	env := &EnvContext{AgenticRepoRoot: dir}

	var buf bytes.Buffer
	_ = CreateRepo(&buf, cfg, state, env, fakeRunOK(""))

	expectedClone := filepath.Join(dir, "others", "my-service")
	if state.ClonePath != expectedClone {
		t.Errorf("state.ClonePath = %q, want %q", state.ClonePath, expectedClone)
	}
}

// --------------------------------------------------------------------------------------
// Step 2 — ConfigureLabels
// --------------------------------------------------------------------------------------

func TestConfigureLabels_AllLabelsCreated_ReturnsNil(t *testing.T) {
	cfg := &InceptionConfig{Owner: "acme"}
	state := &StepState{RepoName: "charging-domain"}

	var created []string
	run := func(name string, args ...string) (string, error) {
		if name == "gh" && len(args) > 2 && args[0] == "label" && args[1] == "create" {
			created = append(created, args[2])
		}
		return "", nil
	}

	var buf bytes.Buffer
	if err := ConfigureLabels(&buf, cfg, state, run); err != nil {
		t.Fatalf("ConfigureLabels() unexpected error: %v", err)
	}
	if len(created) != len(standardLabels) {
		t.Errorf("expected %d labels created, got %d: %v", len(standardLabels), len(created), created)
	}
}

func TestConfigureLabels_OneLabelFails_StillReturnsNil(t *testing.T) {
	cfg := &InceptionConfig{Owner: "acme"}
	state := &StepState{RepoName: "charging-domain"}

	callCount := 0
	run := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			return "already exists", errors.New("already exists")
		}
		return "", nil
	}

	var buf bytes.Buffer
	if err := ConfigureLabels(&buf, cfg, state, run); err != nil {
		t.Errorf("ConfigureLabels() should not return error on label failure (best-effort), got: %v", err)
	}
}

// --------------------------------------------------------------------------------------
// Step 3 — ScaffoldStack
// --------------------------------------------------------------------------------------

func TestScaffoldStack_OtherStack_SkipsWithWarning(t *testing.T) {
	cfg := &InceptionConfig{Stack: "Other"}
	state := &StepState{ClonePath: t.TempDir()}
	env := &EnvContext{AgenticRepoRoot: t.TempDir()}

	var buf bytes.Buffer
	if err := ScaffoldStack(&buf, cfg, state, env, fakeRunFail("should not be called")); err != nil {
		t.Fatalf("ScaffoldStack() expected nil for 'Other' stack, got: %v", err)
	}
	if !strings.Contains(buf.String(), "skipping scaffold") {
		t.Errorf("expected skip message in output, got: %s", buf.String())
	}
}

func TestScaffoldStack_MissingStandardsFile_ReturnsError(t *testing.T) {
	cfg := &InceptionConfig{Stack: "Go"}
	state := &StepState{ClonePath: t.TempDir()}
	env := &EnvContext{AgenticRepoRoot: t.TempDir()} // No base/standards/go.md

	var buf bytes.Buffer
	if err := ScaffoldStack(&buf, cfg, state, env, fakeRunOK("")); err == nil {
		t.Error("ScaffoldStack() expected error when standards file missing, got nil")
	}
}

func TestScaffoldStack_ExecutesCommandsFromFile(t *testing.T) {
	agenticDir := t.TempDir()
	stdDir := filepath.Join(agenticDir, "base", "standards")
	if err := os.MkdirAll(stdDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := "## Project Initialisation\n\n```bash\necho hello\n```\n\n## Other\n"
	if err := os.WriteFile(filepath.Join(stdDir, "go.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &InceptionConfig{Stack: "Go"}
	state := &StepState{ClonePath: t.TempDir()}
	env := &EnvContext{AgenticRepoRoot: agenticDir}

	var executed []string
	run := func(name string, args ...string) (string, error) {
		if name == "bash" {
			executed = append(executed, strings.Join(args, " "))
		}
		return "", nil
	}

	var buf bytes.Buffer
	if err := ScaffoldStack(&buf, cfg, state, env, run); err != nil {
		t.Fatalf("ScaffoldStack() unexpected error: %v", err)
	}
	if len(executed) == 0 {
		t.Error("expected at least one bash command to be executed")
	}
}

// --------------------------------------------------------------------------------------
// Step 4 — PopulateRepo
// --------------------------------------------------------------------------------------

func TestPopulateRepo_WritesThreeFiles(t *testing.T) {
	dir := t.TempDir()
	cfg := &InceptionConfig{
		RepoType:    "domain",
		RepoName:    "charging",
		Description: "OCS charging engine",
		Stack:       "Go",
		Owner:       "acme-org",
	}
	state := &StepState{
		RepoName:  "charging-domain",
		ClonePath: dir,
		RepoURL:   "https://github.com/acme-org/charging-domain",
	}
	env := &EnvContext{AgenticRepoRoot: t.TempDir(), Owner: "acme-org"}

	var buf bytes.Buffer
	if err := PopulateRepo(&buf, cfg, state, env, fakeRunOK("")); err != nil {
		t.Fatalf("PopulateRepo() unexpected error: %v", err)
	}

	for _, f := range []string{"CLAUDE.md", "AGENTS.local.md", "README.md"} {
		data, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			t.Errorf("expected %s to exist, got: %v", f, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("%s is empty", f)
		}
	}
}

func TestPopulateRepo_CLAUDEMDReferencesAGENTSMD(t *testing.T) {
	dir := t.TempDir()
	cfg := &InceptionConfig{Owner: "acme-org", RepoType: "domain", RepoName: "charging"}
	state := &StepState{RepoName: "charging-domain", ClonePath: dir}
	env := &EnvContext{AgenticRepoRoot: t.TempDir()}

	var buf bytes.Buffer
	_ = PopulateRepo(&buf, cfg, state, env, fakeRunOK(""))

	data, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if !strings.Contains(string(data), "AGENTS.md") {
		t.Errorf("CLAUDE.md should reference AGENTS.md, got:\n%s", string(data))
	}
}

func TestPopulateRepo_PushFails_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	cfg := &InceptionConfig{Owner: "acme-org", RepoType: "domain", RepoName: "charging"}
	state := &StepState{RepoName: "charging-domain", ClonePath: dir}
	env := &EnvContext{AgenticRepoRoot: t.TempDir()}

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
	err := PopulateRepo(&buf, cfg, state, env, run)
	if err == nil {
		t.Fatal("PopulateRepo() expected error when git push fails, got nil")
	}
	if !strings.Contains(err.Error(), "git push") {
		t.Errorf("error should mention 'git push', got: %v", err)
	}
}

// --------------------------------------------------------------------------------------
// Step 5 — RegisterInREPOS
// --------------------------------------------------------------------------------------

func TestRegisterInREPOS_AppendsEntry(t *testing.T) {
	dir := t.TempDir()
	reposPath := filepath.Join(dir, "REPOS.md")
	if err := os.WriteFile(reposPath, []byte("# REPOS.md\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &InceptionConfig{
		RepoType:    "domain",
		RepoName:    "charging",
		Description: "OCS charging engine",
		Stack:       "Go",
		Owner:       "acme-org",
	}
	state := &StepState{RepoName: "charging-domain"}
	env := &EnvContext{AgenticRepoRoot: dir}

	var buf bytes.Buffer
	if err := RegisterInREPOS(&buf, cfg, state, env, fakeRunOK("")); err != nil {
		t.Fatalf("RegisterInREPOS() unexpected error: %v", err)
	}

	data, err := os.ReadFile(reposPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	checks := []string{
		"charging-domain",
		"acme-org",
		"Go",
		"domain",
		"active",
		"OCS charging engine",
	}
	for _, want := range checks {
		if !strings.Contains(content, want) {
			t.Errorf("REPOS.md should contain %q, got:\n%s", want, content)
		}
	}
}

func TestRegisterInREPOS_ReadFails_ReturnsError(t *testing.T) {
	cfg := &InceptionConfig{Owner: "acme"}
	state := &StepState{RepoName: "charging-domain"}
	env := &EnvContext{AgenticRepoRoot: "/nonexistent"}

	var buf bytes.Buffer
	err := RegisterInREPOS(&buf, cfg, state, env, fakeRunOK(""))
	if err == nil {
		t.Fatal("RegisterInREPOS() expected error when REPOS.md missing, got nil")
	}
}

// --------------------------------------------------------------------------------------
// Step 6 — PrintSummary
// --------------------------------------------------------------------------------------

func TestPrintSummary_OutputContainsAllFields(t *testing.T) {
	cfg := &InceptionConfig{RepoType: "domain"}
	state := &StepState{
		RepoURL:   "https://github.com/acme-org/charging-domain",
		ClonePath: "/home/user/domains/charging-domain",
	}

	var buf bytes.Buffer
	PrintSummary(&buf, cfg, state)

	out := buf.String()
	if !strings.Contains(out, "Inception complete") {
		t.Errorf("expected 'Inception complete' in output, got: %s", out)
	}
	if !strings.Contains(out, state.RepoURL) {
		t.Errorf("expected repo URL in output, got: %s", out)
	}
	if !strings.Contains(out, state.ClonePath) {
		t.Errorf("expected clone path in output, got: %s", out)
	}
	if !strings.Contains(out, "domain") {
		t.Errorf("expected repo type in output, got: %s", out)
	}
}

// --------------------------------------------------------------------------------------
// Internal helpers
// --------------------------------------------------------------------------------------

func TestExtractInitCommands_FindsSection(t *testing.T) {
	content := "## Project Initialisation\n\n```bash\necho hello\n```\n\n## Other\n"
	cmds, err := extractInitCommands(content)
	if err != nil {
		t.Fatalf("extractInitCommands() error: %v", err)
	}
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command block, got %d", len(cmds))
	}
	if !strings.Contains(cmds[0], "echo hello") {
		t.Errorf("expected 'echo hello' in command, got: %s", cmds[0])
	}
}

func TestExtractInitCommands_SectionMissing_ReturnsError(t *testing.T) {
	_, err := extractInitCommands("# No relevant section")
	if err == nil {
		t.Error("expected error when section is missing, got nil")
	}
}

func TestShellQuote_EscapesSingleQuotes(t *testing.T) {
	got := shellQuote("it's")
	if !strings.Contains(got, `'\''`) {
		t.Errorf("shellQuote should escape single quotes, got: %s", got)
	}
}
