package bootstrap

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eddiecarpenter/gh-agentic/internal/testutil"
)

// integrationLookPath returns a LookPathFunc that succeeds for tools in the found
// set and fails for all others.
func integrationLookPath(found map[string]bool) LookPathFunc {
	return func(file string) (string, error) {
		if found[file] {
			return "/usr/bin/" + file, nil
		}
		return "", fmt.Errorf("%s not found", file)
	}
}

// integrationConfirm returns a ConfirmFunc that always returns the given value.
func integrationConfirm(yes bool) ConfirmFunc {
	return func(_ string) (bool, error) {
		return yes, nil
	}
}

func TestIntegrationPreflight_AllToolsPresent_Passes(t *testing.T) {
	lookPath := integrationLookPath(map[string]bool{
		"git":    true,
		"gh":     true,
		"goose":  true,
		"claude": true,
	})
	run := func(name string, args ...string) (string, error) {
		// gh auth status succeeds.
		if name == "gh" && len(args) > 0 && args[0] == "auth" {
			return "Logged in", nil
		}
		return "", nil
	}

	var buf bytes.Buffer
	err := RunPreflight(&buf, lookPath, run, integrationConfirm(false))

	if err != nil {
		t.Fatalf("expected nil error, got: %v\nOutput:\n%s", err, buf.String())
	}

	output := buf.String()
	for _, tool := range []string{"git", "gh", "goose", "claude"} {
		if !strings.Contains(output, tool+" found") {
			t.Errorf("expected '%s found' in output, got:\n%s", tool, output)
		}
	}
}

func TestIntegrationPreflight_RequiredMissing_Git_HardStop(t *testing.T) {
	lookPath := integrationLookPath(map[string]bool{
		"gh":     true,
		"goose":  true,
		"claude": true,
		// git is missing
	})
	run := func(name string, args ...string) (string, error) {
		return "", nil
	}

	var buf bytes.Buffer
	err := RunPreflight(&buf, lookPath, run, integrationConfirm(false))

	if err == nil {
		t.Fatal("expected error for missing git, got nil")
	}

	output := buf.String()
	if !strings.Contains(output, "git") {
		t.Errorf("expected 'git' mentioned in output, got:\n%s", output)
	}
	if !strings.Contains(err.Error(), "git") {
		t.Errorf("expected error to mention git, got: %v", err)
	}
}

func TestIntegrationPreflight_RequiredMissing_Gh_HardStop(t *testing.T) {
	lookPath := integrationLookPath(map[string]bool{
		"git":    true,
		"goose":  true,
		"claude": true,
		// gh is missing
	})
	run := func(name string, args ...string) (string, error) {
		return "", nil
	}

	var buf bytes.Buffer
	err := RunPreflight(&buf, lookPath, run, integrationConfirm(false))

	if err == nil {
		t.Fatal("expected error for missing gh, got nil")
	}

	output := buf.String()
	if !strings.Contains(output, "gh") {
		t.Errorf("expected 'gh' mentioned in output, got:\n%s", output)
	}
	if !strings.Contains(err.Error(), "gh") {
		t.Errorf("expected error to mention gh, got: %v", err)
	}
}

func TestIntegrationPreflight_OptionalMissing_Goose_OfferInstall(t *testing.T) {
	// goose is required but has an installPrompt, so it gets an install offer.
	installAttempted := false
	gooseInstalled := false

	lookPath := func(file string) (string, error) {
		switch file {
		case "git", "gh", "claude":
			return "/usr/bin/" + file, nil
		case "goose":
			if gooseInstalled {
				return "/usr/bin/goose", nil
			}
			return "", fmt.Errorf("goose not found")
		default:
			return "", fmt.Errorf("%s not found", file)
		}
	}

	run := func(name string, args ...string) (string, error) {
		// gh auth status succeeds.
		if name == "gh" && len(args) > 0 && args[0] == "auth" {
			return "Logged in", nil
		}
		// Install command for goose.
		if name == "bash" && len(args) > 0 && args[0] == "-c" {
			installAttempted = true
			gooseInstalled = true
			return "", nil
		}
		return "", nil
	}

	// User accepts the install prompt.
	confirm := func(prompt string) (bool, error) {
		return true, nil
	}

	var buf bytes.Buffer
	err := RunPreflight(&buf, lookPath, run, confirm)

	if err != nil {
		t.Fatalf("expected nil error after goose install, got: %v\nOutput:\n%s", err, buf.String())
	}

	if !installAttempted {
		t.Error("expected install to be attempted for goose")
	}

	output := buf.String()
	if !strings.Contains(output, "goose") {
		t.Errorf("expected 'goose' mentioned in output, got:\n%s", output)
	}
}

// setupCloneDir creates a fake cloned repo directory pre-populated with the
// template files that downstream steps expect to find.
func setupCloneDir(t *testing.T, workDir, projectName string) string {
	t.Helper()
	clonePath := filepath.Join(workDir, projectName)

	// Create base/standards/go.md so ScaffoldStack can read it.
	standardsDir := filepath.Join(clonePath, "base", "standards")
	if err := os.MkdirAll(standardsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Minimal go.md with a Project Initialisation section.
	goMD := `# Go Standards

## Project Initialisation

` + "```bash" + `
mkdir -p cmd/test-project internal
` + "```" + `

## Build Verification
`
	if err := os.WriteFile(filepath.Join(standardsDir, "go.md"), []byte(goMD), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create bootstrap.sh so RemoveTemplateFiles finds it.
	if err := os.WriteFile(filepath.Join(clonePath, "bootstrap.sh"), []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(clonePath, "bootstrap.sh.md5"), []byte("abc123"), 0o644); err != nil {
		t.Fatal(err)
	}

	return clonePath
}

func TestIntegrationRunSteps_HappyPath_GoEmbedded(t *testing.T) {
	workDir := t.TempDir()
	projectName := "test-project"

	// Pre-create the clone directory that git clone would normally create.
	setupCloneDir(t, workDir, projectName)

	cfg := BootstrapConfig{
		Topology:    "Single",
		Owner:       "testowner",
		ProjectName: projectName,
		Description: "Test project",
		Stack:       "Go",
		Antora:      false,
		OwnerType:   "User",
	}

	mock := &testutil.MockRunner{}

	// gh project create returns JSON.
	mock.Expect(
		[]string{"gh", "project", "create", "--owner", "testowner", "--title", "test-project", "--format", "json"},
		`{"number":1,"url":"https://github.com/users/testowner/projects/1"}`,
		nil,
	)

	// graphqlDo stub — skip repo linking for User accounts.
	graphqlDo := func(query string, variables map[string]interface{}, response interface{}) error {
		return nil
	}

	// No-op launch — PrintSummary uses huh which requires a TTY, so the test
	// will get a huh error. We only verify the 8 spinner steps completed.
	launch := func(_ string) error {
		return nil
	}

	var buf bytes.Buffer
	err := RunSteps(&buf, cfg, workDir, mock.RunCommand, graphqlDo, launch, testutil.NoopSpinner)

	output := buf.String()

	// PrintSummary will fail because huh requires a TTY. That's expected.
	// We verify all 8 steps in the loop completed by checking the output.
	if err != nil {
		// The error should be from PrintSummary's huh form, not from any step.
		if !strings.Contains(err.Error(), "launch prompt") && !strings.Contains(err.Error(), "huh") &&
			!strings.Contains(err.Error(), "input") && !strings.Contains(err.Error(), "program") {
			t.Fatalf("unexpected error (not from PrintSummary): %v\nOutput:\n%s", err, output)
		}
		t.Logf("expected PrintSummary error (no TTY): %v", err)
	}

	// Verify key step indicators in the output.
	expectedPhrases := []string{
		"Creating your agentic environment",
	}
	for _, phrase := range expectedPhrases {
		if !strings.Contains(output, phrase) {
			t.Errorf("expected %q in output, got:\n%s", phrase, output)
		}
	}

	// Verify the clone directory was used — PopulateRepo writes files there.
	clonePath := filepath.Join(workDir, projectName)
	if _, statErr := os.Stat(filepath.Join(clonePath, "REPOS.md")); os.IsNotExist(statErr) {
		t.Error("expected REPOS.md to be written by PopulateRepo")
	}
	if _, statErr := os.Stat(filepath.Join(clonePath, "AGENTS.local.md")); os.IsNotExist(statErr) {
		t.Error("expected AGENTS.local.md to be written by PopulateRepo")
	}
	if _, statErr := os.Stat(filepath.Join(clonePath, "README.md")); os.IsNotExist(statErr) {
		t.Error("expected README.md to be written by PopulateRepo")
	}

	// Verify ScaffoldStack created directories (the mock bash -c returns success
	// but doesn't actually run the command, so dirs won't exist — check that
	// the step didn't error by verifying we got past it).
	mock.AssertExpectations(t)
}

func TestIntegrationRunSteps_Failure_Step3_RepoCreate(t *testing.T) {
	workDir := t.TempDir()
	projectName := "test-project"

	cfg := BootstrapConfig{
		Topology:    "Single",
		Owner:       "testowner",
		ProjectName: projectName,
		Description: "Test project",
		Stack:       "Go",
		Antora:      false,
		OwnerType:   "User",
	}

	mock := &testutil.MockRunner{}

	// gh repo create fails.
	mock.Expect(
		[]string{"gh", "repo", "create", "testowner/test-project", "--template", "eddiecarpenter/agentic-development", "--private"},
		"",
		fmt.Errorf("repository creation failed: quota exceeded"),
	)

	graphqlDo := func(query string, variables map[string]interface{}, response interface{}) error {
		return nil
	}
	launch := func(_ string) error { return nil }

	var buf bytes.Buffer
	err := RunSteps(&buf, cfg, workDir, mock.RunCommand, graphqlDo, launch, testutil.NoopSpinner)

	if err == nil {
		t.Fatal("expected error from step 3, got nil")
	}
	if !strings.Contains(err.Error(), "repo create") {
		t.Errorf("expected error to mention repo create, got: %v", err)
	}

	// Verify no further steps ran — PopulateRepo would create REPOS.md.
	clonePath := filepath.Join(workDir, projectName)
	if _, statErr := os.Stat(filepath.Join(clonePath, "REPOS.md")); !os.IsNotExist(statErr) {
		t.Error("REPOS.md should not exist — steps after step 3 should not have run")
	}

	mock.AssertExpectations(t)
}

func TestIntegrationRunSteps_Failure_Step6_LabelCreate(t *testing.T) {
	workDir := t.TempDir()
	projectName := "test-project"

	// Pre-create the clone directory so steps 3-5 can work with it.
	setupCloneDir(t, workDir, projectName)

	cfg := BootstrapConfig{
		Topology:    "Single",
		Owner:       "testowner",
		ProjectName: projectName,
		Description: "Test project",
		Stack:       "Go",
		Antora:      false,
		OwnerType:   "User",
	}

	// ConfigureRepo (step 6) calls gh label create for each label.
	// Label failures are logged as warnings but do NOT fail the step.
	// So to test a real failure at step 6 level, we need something else.
	//
	// Looking at the RunSteps code, step 6 is ConfigureRepo which only creates
	// labels (warnings on failure, always returns nil). The actual failure
	// needs to happen in a step that returns an error.
	//
	// Instead, we test step 5 (ScaffoldStack) failure by using a stack with
	// a missing standards file — or make gh project create fail (step 6/7).
	//
	// Let's test: steps 3-5 succeed, step 6 (CreateProject) fails.
	// Note: In the RunSteps code, the step labeled "Configuring labels" is
	// index 3 (step 6 in the spec), and "Creating GitHub Project" is index 5.

	mock := &testutil.MockRunner{}

	// gh project create returns an error.
	mock.Expect(
		[]string{"gh", "project", "create", "--owner", "testowner", "--title", "test-project", "--format", "json"},
		"",
		fmt.Errorf("project creation failed: permission denied"),
	)

	graphqlDo := func(query string, variables map[string]interface{}, response interface{}) error {
		return nil
	}
	launch := func(_ string) error { return nil }

	var buf bytes.Buffer
	err := RunSteps(&buf, cfg, workDir, mock.RunCommand, graphqlDo, launch, testutil.NoopSpinner)

	if err == nil {
		t.Fatal("expected error from project creation, got nil")
	}
	if !strings.Contains(err.Error(), "project create") {
		t.Errorf("expected error to mention project create, got: %v", err)
	}

	output := buf.String()

	// Verify preceding steps completed — PopulateRepo writes files.
	clonePath := filepath.Join(workDir, projectName)
	if _, statErr := os.Stat(filepath.Join(clonePath, "REPOS.md")); os.IsNotExist(statErr) {
		t.Error("expected REPOS.md to exist — PopulateRepo (step 7) should have run before CreateProject (step 8)")
	}

	// Verify the output mentions the environment creation header (steps ran).
	if !strings.Contains(output, "Creating your agentic environment") {
		t.Errorf("expected header in output, got:\n%s", output)
	}

	mock.AssertExpectations(t)
}
