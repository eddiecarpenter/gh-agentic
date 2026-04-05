package sync

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eddiecarpenter/gh-agentic/internal/testutil"
)

// cloneRunner wraps a MockRunner with a side-effect for git clone commands.
// When a git clone call is intercepted, it populates the target directory with
// a fake template base/ containing baseContent and then delegates to the mock.
// All other commands are passed through to the mock directly.
func cloneRunner(mock *testutil.MockRunner, baseContent string) func(string, ...string) (string, error) {
	return func(name string, args ...string) (string, error) {
		if name == "git" && len(args) >= 1 && args[0] == "clone" {
			// The last arg is the target directory.
			targetDir := args[len(args)-1]
			_ = os.MkdirAll(filepath.Join(targetDir, "base"), 0o755)
			_ = os.WriteFile(filepath.Join(targetDir, "base", "AGENTS.md"), []byte(baseContent), 0o644)
			return "", nil
		}
		return mock.RunCommand(name, args...)
	}
}

func TestRunSync_UpToDate(t *testing.T) {
	repo := testutil.NewFakeRepo(t)
	// FakeRepo already has TEMPLATE_SOURCE and TEMPLATE_VERSION=v1.0.0.

	mock := &testutil.MockRunner{}

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		mock.RunCommand,
		testutil.FakeRelease("v1.0.0", nil),
		testutil.NoopSpinner,
		func(_ string) (bool, error) { return false, nil },
		false,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "up to date") {
		t.Errorf("expected 'up to date' message, got: %s", output)
	}

	mock.AssertExpectations(t)
}

func TestRunSync_ConfirmAndCommit(t *testing.T) {
	repo := testutil.NewFakeRepo(t)

	mock := &testutil.MockRunner{}
	// git diff, git add, and git commit are called but not specifically matched —
	// MockRunner returns ("", nil) for unmatched calls.

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		cloneRunner(mock, "updated content"),
		testutil.FakeRelease("v2.0.0", nil),
		testutil.NoopSpinner,
		func(_ string) (bool, error) { return true, nil }, // confirm yes
		false,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify TEMPLATE_VERSION was updated.
	data, err := os.ReadFile(filepath.Join(repo.Root, "TEMPLATE_VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "v2.0.0" {
		t.Errorf("TEMPLATE_VERSION = %q, want v2.0.0", string(data))
	}

	// Verify base/ was updated.
	data, err = os.ReadFile(filepath.Join(repo.Root, "base", "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "updated content" {
		t.Errorf("base/AGENTS.md = %q, want 'updated content'", data)
	}

	output := buf.String()
	if !strings.Contains(output, "Synced") {
		t.Errorf("expected success message, got: %s", output)
	}

	mock.AssertExpectations(t)
}

func TestRunSync_DeclineAndRestore(t *testing.T) {
	repo := testutil.NewFakeRepo(t)

	mock := &testutil.MockRunner{}

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		cloneRunner(mock, "new content"),
		testutil.FakeRelease("v2.0.0", nil),
		testutil.NoopSpinner,
		func(_ string) (bool, error) { return false, nil }, // decline
		false,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify base/ was restored to original.
	data, err := os.ReadFile(filepath.Join(repo.Root, "base", "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "# AGENTS.md\n" {
		t.Errorf("base/AGENTS.md = %q, want '# AGENTS.md\\n'", data)
	}

	// Verify TEMPLATE_VERSION was NOT updated.
	data, err = os.ReadFile(filepath.Join(repo.Root, "TEMPLATE_VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "v1.0.0" {
		t.Errorf("TEMPLATE_VERSION = %q, want v1.0.0", string(data))
	}

	output := buf.String()
	if !strings.Contains(output, "cancelled") {
		t.Errorf("expected 'cancelled' message, got: %s", output)
	}

	mock.AssertExpectations(t)
}

func TestRunSync_FetchError(t *testing.T) {
	repo := testutil.NewFakeRepo(t)

	mock := &testutil.MockRunner{}

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		mock.RunCommand,
		testutil.FakeRelease("", fmt.Errorf("API error")),
		testutil.NoopSpinner,
		func(_ string) (bool, error) { return false, nil },
		false,
	)

	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "API error") {
		t.Errorf("unexpected error: %v", err)
	}

	mock.AssertExpectations(t)
}
