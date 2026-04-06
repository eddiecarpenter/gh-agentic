package inception

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
)

// plainSpinnerFunc is a test double for SpinnerFunc that runs the function
// without any spinner rendering, so tests don't require a real TTY.
var plainSpinnerFunc SpinnerFunc = func(w io.Writer, label string, fn func() error) error {
	return fn()
}

func TestRunSteps_AllStepsSucceed_PrintsSummary(t *testing.T) {
	agenticDir := t.TempDir()

	// Create REPOS.md for RegisterInREPOS step.
	if err := os.WriteFile(filepath.Join(agenticDir, "REPOS.md"), []byte("# REPOS.md\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &InceptionConfig{
		RepoType:    "domain",
		RepoName:    "charging",
		Description: "OCS charging engine",
		Stack:       "Other", // Skip scaffold
		Owner:       "acme-org",
	}
	env := &EnvContext{
		AgenticRepoRoot: agenticDir,
		Owner:           "acme-org",
		TemplateRepo:    bootstrap.DefaultTemplateRepo,
	}

	// When git clone is called, create the clone directory so PopulateRepo can write files.
	run := func(name string, args ...string) (string, error) {
		if name == "git" && len(args) > 0 && args[0] == "clone" {
			clonePath := args[len(args)-1]
			if mkErr := os.MkdirAll(clonePath, 0755); mkErr != nil {
				return "", mkErr
			}
		}
		return "", nil
	}

	var buf bytes.Buffer
	err := RunSteps(&buf, cfg, env, run, plainSpinnerFunc)
	if err != nil {
		t.Fatalf("RunSteps() unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Creating new repo") {
		t.Errorf("expected section heading in output, got: %s", out)
	}
	if !strings.Contains(out, "Inception complete") {
		t.Errorf("expected 'Inception complete' in output, got: %s", out)
	}
}

func TestRunSteps_StepFails_StopsImmediately(t *testing.T) {
	agenticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(agenticDir, "REPOS.md"), []byte("# REPOS.md\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &InceptionConfig{
		RepoType: "domain",
		RepoName: "charging",
		Stack:    "Other",
		Owner:    "acme-org",
	}
	env := &EnvContext{AgenticRepoRoot: agenticDir, Owner: "acme-org", TemplateRepo: bootstrap.DefaultTemplateRepo}

	// First call (gh repo create) fails.
	callCount := 0
	run := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			return "error", errors.New("gh repo create failed")
		}
		return "", nil
	}

	var buf bytes.Buffer
	err := RunSteps(&buf, cfg, env, run, plainSpinnerFunc)
	if err == nil {
		t.Fatal("RunSteps() expected error when first step fails, got nil")
	}
}

func TestRunSteps_StepSequence_CorrectOrder(t *testing.T) {
	agenticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(agenticDir, "REPOS.md"), []byte("# REPOS.md\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &InceptionConfig{
		RepoType: "domain",
		RepoName: "charging",
		Stack:    "Other",
		Owner:    "acme-org",
	}
	env := &EnvContext{AgenticRepoRoot: agenticDir, Owner: "acme-org", TemplateRepo: bootstrap.DefaultTemplateRepo}

	// Track the order of spinner labels.
	var labels []string
	trackingSpinner := func(w io.Writer, label string, fn func() error) error {
		labels = append(labels, label)
		// Create clone dir on first step (CreateRepo).
		if strings.Contains(label, "Creating repository") {
			clonePath := filepath.Join(agenticDir, "domains", "charging-domain")
			if mkErr := os.MkdirAll(clonePath, 0755); mkErr != nil {
				return mkErr
			}
		}
		return fn()
	}

	var buf bytes.Buffer
	err := RunSteps(&buf, cfg, env, fakeRunOK(""), trackingSpinner)
	if err != nil {
		t.Fatalf("RunSteps() unexpected error: %v", err)
	}

	expectedOrder := []string{
		"Creating repository",
		"Configuring labels",
		"Scaffolding",
		"Populating repository",
		"Registering in REPOS.md",
	}
	if len(labels) != len(expectedOrder) {
		t.Fatalf("expected %d steps, got %d: %v", len(expectedOrder), len(labels), labels)
	}
	for i, want := range expectedOrder {
		if !strings.Contains(labels[i], want) {
			t.Errorf("step %d: expected label containing %q, got %q", i, want, labels[i])
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
}
