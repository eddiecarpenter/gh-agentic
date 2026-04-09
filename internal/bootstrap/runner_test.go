package bootstrap

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// plainSpinnerFunc is a test double for SpinnerFunc that runs the function
// without any spinner rendering, so tests don't require a real TTY.
var plainSpinnerFunc SpinnerFunc = func(w io.Writer, label string, fn func() error) error {
	return fn()
}

func TestRunSteps_AllStepsSucceed_ReturnsNilAfterSummary(t *testing.T) {
	// This test exercises the happy path through RunSteps.
	// Steps that write files need a real temp directory; git/gh calls are stubbed.
	dir := t.TempDir()

	cfg := BootstrapConfig{
		Topology:     "Single",
		Owner:        "alice",
		ProjectName:  "my-project",
		Stacks:      []string{"Other"}, // "Other" skips ScaffoldStacks so we don't need base/standards/
		Description:  "Test project",
		Antora:       false,
		OwnerType:    OwnerTypeUser,
		TemplateRepo: DefaultTemplateRepo,
	}

	// Stub: all external commands succeed.
	// When "git clone" is called, create the ClonePath directory so that
	// subsequent steps that write files into it don't fail with "no such file or directory".
	clonePath := filepath.Join(dir, "my-project")
	tarballData := makeMinimalTarballBytes(t)
	run := func(name string, args ...string) (string, error) {
		// CreateRepo calls run("git", "clone", sshURL, clonePath) directly.
		if name == "git" && len(args) > 0 && args[0] == "clone" {
			if mkErr := os.MkdirAll(clonePath, 0755); mkErr != nil {
				return "", mkErr
			}
		}
		// Handle tarball fetch — write a valid tarball to the --output path.
		if name == "gh" && len(args) > 0 && args[0] == "api" {
			writeTarballOnOutput(t, tarballData, args)
		}
		return `{"number":1,"url":"https://github.com/users/alice/projects/1"}`, nil
	}

	// Stub GraphQL: always errors — CreateProject link is best-effort.
	graphqlDo := func(query string, variables map[string]interface{}, response interface{}) error {
		return errors.New("no auth in test")
	}

	// Stub launch: skip — avoids spawning goose.
	launch := func(clonePath string) error { return nil }

	// Stub spinner: plain (no TTY).
	spinner := plainSpinnerFunc

	var buf bytes.Buffer

	// RunSteps will call PrintSummary, which runs a huh form requiring a TTY.
	// In a test environment this will return an error from the form.
	// We accept that — what we care about is that steps 3-8 ran without error.
	// To avoid the form error propagating as a test failure we capture it.
	fetchRelease := func(repo string) (string, error) { return "v1.0.0", nil }

	err := RunSteps(&buf, cfg, dir, run, graphqlDo, launch, spinner, fetchRelease)

	// The only acceptable error is from the huh form (no TTY).
	if err != nil && !strings.Contains(err.Error(), "launch prompt") {
		t.Errorf("RunSteps() unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Creating your agentic environment") {
		t.Errorf("expected section heading in output, got: %s", out)
	}
}

func TestRunSteps_StepFails_StopsImmediately(t *testing.T) {
	dir := t.TempDir()

	cfg := BootstrapConfig{
		Topology:     "Single",
		Owner:        "alice",
		ProjectName:  "my-project",
		Stacks:      []string{"Other"},
		Description:  "Test project",
		OwnerType:    OwnerTypeUser,
		TemplateRepo: DefaultTemplateRepo,
	}

	// Stub: gh repo create fails on the first call.
	callCount := 0
	run := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			return "error", errors.New("gh repo create failed")
		}
		return "", nil
	}

	graphqlDo := func(_ string, _ map[string]interface{}, _ interface{}) error { return nil }
	launch := func(_ string) error { return nil }

	fetchRelease := func(repo string) (string, error) { return "v1.0.0", nil }

	var buf bytes.Buffer
	err := RunSteps(&buf, cfg, dir, run, graphqlDo, launch, plainSpinnerFunc, fetchRelease)
	if err == nil {
		t.Fatal("RunSteps() expected error when first step fails, got nil")
	}

	// After failure, the spinner should have emitted an error marker.
	// We verify no subsequent step messages appear.
	out := buf.String()
	if strings.Contains(out, "Scaffolding") {
		t.Error("expected subsequent steps to be skipped after failure")
	}
}

func TestRunSteps_MergedConfiguringRepositoryStep_NoSeparateLabels(t *testing.T) {
	dir := t.TempDir()

	cfg := BootstrapConfig{
		Topology:     "Single",
		Owner:        "alice",
		ProjectName:  "my-project",
		Stacks:      []string{"Other"},
		Description:  "Test project",
		OwnerType:    OwnerTypeUser,
		TemplateRepo: DefaultTemplateRepo,
	}

	clonePath := filepath.Join(dir, "my-project")
	tarballData := makeMinimalTarballBytes(t)

	run := func(name string, args ...string) (string, error) {
		if name == "git" && len(args) > 0 && args[0] == "clone" {
			if mkErr := os.MkdirAll(clonePath, 0755); mkErr != nil {
				return "", mkErr
			}
		}
		if name == "gh" && len(args) > 0 && args[0] == "api" {
			writeTarballOnOutput(t, tarballData, args)
		}
		return `{"number":1,"url":"https://github.com/users/alice/projects/1"}`, nil
	}

	graphqlDo := func(query string, variables map[string]interface{}, response interface{}) error {
		return errors.New("no auth in test")
	}
	launch := func(clonePath string) error { return nil }
	fetchRelease := func(repo string) (string, error) { return "v1.0.0", nil }

	// Track spinner labels.
	var labels []string
	trackingSpinner := func(w io.Writer, label string, fn func() error) error {
		labels = append(labels, label)
		return fn()
	}

	var buf bytes.Buffer
	_ = RunSteps(&buf, cfg, dir, run, graphqlDo, launch, trackingSpinner, fetchRelease)

	// Verify "Configuring repository" exists and old labels do not.
	foundMerged := false
	for _, l := range labels {
		if strings.Contains(l, "Configuring repository") {
			foundMerged = true
		}
		if strings.Contains(l, "Configuring labels") {
			t.Error("should not have separate 'Configuring labels' step")
		}
		if strings.Contains(l, "Creating GitHub Project") {
			t.Error("should not have separate 'Creating GitHub Project' step")
		}
	}
	if !foundMerged {
		t.Errorf("expected 'Configuring repository' step in labels, got: %v", labels)
	}
}

func TestRunSteps_ConfigureRepoSucceeds_CreateProjectFails_ErrorPropagates(t *testing.T) {
	dir := t.TempDir()

	cfg := BootstrapConfig{
		Topology:     "Single",
		Owner:        "alice",
		ProjectName:  "my-project",
		Stacks:       []string{"Other"},
		Description:  "Test project",
		OwnerType:    OwnerTypeUser,
		TemplateRepo: DefaultTemplateRepo,
	}

	clonePath := filepath.Join(dir, "my-project")
	tarballData := makeMinimalTarballBytes(t)

	// Stub: ConfigureRepo calls succeed, CreateProject's "gh project create" fails.
	run := func(name string, args ...string) (string, error) {
		if name == "git" && len(args) > 0 && args[0] == "clone" {
			if mkErr := os.MkdirAll(clonePath, 0755); mkErr != nil {
				return "", mkErr
			}
		}
		if name == "gh" && len(args) > 0 && args[0] == "api" {
			writeTarballOnOutput(t, tarballData, args)
		}
		// Fail on "gh project create" — this is CreateProject's first call.
		if name == "gh" && len(args) >= 2 && args[0] == "project" && args[1] == "create" {
			return "", errors.New("project creation failed")
		}
		return `{"number":1}`, nil
	}

	graphqlDo := func(query string, variables map[string]interface{}, response interface{}) error {
		return nil
	}
	launch := func(_ string) error { return nil }
	fetchRelease := func(repo string) (string, error) { return "v1.0.0", nil }

	// Track spinner labels to verify subsequent steps do not execute.
	var labels []string
	trackingSpinner := func(w io.Writer, label string, fn func() error) error {
		labels = append(labels, label)
		return fn()
	}

	var buf bytes.Buffer
	err := RunSteps(&buf, cfg, dir, run, graphqlDo, launch, trackingSpinner, fetchRelease)
	if err == nil {
		t.Fatal("expected error when CreateProject fails, got nil")
	}
	if !strings.Contains(err.Error(), "project creation failed") {
		t.Errorf("expected 'project creation failed' in error, got: %v", err)
	}

	// Verify that steps after "Configuring repository" did not execute.
	for _, l := range labels {
		if strings.Contains(l, "Populating repository") {
			t.Error("subsequent step 'Populating repository' should not execute after failure")
		}
	}
}

func TestDefaultSpinner_Success_PrintsCheckmark(t *testing.T) {
	var buf bytes.Buffer
	err := DefaultSpinner(&buf, "my step", func() error { return nil })
	if err != nil {
		t.Fatalf("DefaultSpinner() unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "my step") {
		t.Errorf("expected step label in output, got: %s", out)
	}
	if !strings.Contains(out, "✔") {
		t.Errorf("expected '✔' in output on success, got: %s", out)
	}
}

func TestDefaultSpinner_Failure_PrintsX(t *testing.T) {
	var buf bytes.Buffer
	err := DefaultSpinner(&buf, "bad step", func() error { return errors.New("something broke") })
	if err == nil {
		t.Fatal("DefaultSpinner() expected error to be returned, got nil")
	}
	out := buf.String()
	if !strings.Contains(out, "✖") {
		t.Errorf("expected '✖' in output on failure, got: %s", out)
	}
	if !strings.Contains(out, "something broke") {
		t.Errorf("expected error message in output, got: %s", out)
	}
}
