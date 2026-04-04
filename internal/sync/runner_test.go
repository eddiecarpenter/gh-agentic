package sync

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testSpinner is a SpinnerFunc that just runs the function without visual output.
func testSpinner(_ io.Writer, _ string, fn func() error) error {
	return fn()
}

// setupTestRepo creates a temporary repo root with TEMPLATE_SOURCE, TEMPLATE_VERSION,
// and a base/ directory with a sample file.
func setupTestRepo(t *testing.T, source, version string) string {
	t.Helper()
	root := t.TempDir()

	if err := os.WriteFile(filepath.Join(root, "TEMPLATE_SOURCE"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "TEMPLATE_VERSION"), []byte(version), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "base", "standards"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "base", "AGENTS.md"), []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}

	return root
}

// cloneInterceptor returns a RunCommandFunc that intercepts git clone calls and
// populates the target directory with a fake template base/ containing the given content.
func cloneInterceptor(baseContent string) func(string, ...string) (string, error) {
	return func(name string, args ...string) (string, error) {
		// Direct git clone call: run("git", "clone", "--depth", "1", url, targetDir)
		if name == "git" && len(args) >= 1 && args[0] == "clone" {
			// The last arg is the target directory.
			targetDir := args[len(args)-1]
			_ = os.MkdirAll(filepath.Join(targetDir, "base"), 0o755)
			_ = os.WriteFile(filepath.Join(targetDir, "base", "AGENTS.md"), []byte(baseContent), 0o644)
			return "", nil
		}

		// All other commands (git diff, git add, git commit, etc.) — just succeed.
		return "", nil
	}
}

func TestRunSync_UpToDate(t *testing.T) {
	root := setupTestRepo(t, "owner/template", "v1.0.0")

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		root,
		fakeRun("", nil),
		func(_ string) (string, error) { return "v1.0.0", nil },
		testSpinner,
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
}

func TestRunSync_ConfirmAndCommit(t *testing.T) {
	root := setupTestRepo(t, "owner/template", "v1.0.0")

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		root,
		cloneInterceptor("updated content"),
		func(_ string) (string, error) { return "v2.0.0", nil },
		testSpinner,
		func(_ string) (bool, error) { return true, nil }, // confirm yes
		false,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify TEMPLATE_VERSION was updated.
	data, err := os.ReadFile(filepath.Join(root, "TEMPLATE_VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "v2.0.0" {
		t.Errorf("TEMPLATE_VERSION = %q, want v2.0.0", string(data))
	}

	// Verify base/ was updated.
	data, err = os.ReadFile(filepath.Join(root, "base", "AGENTS.md"))
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
}

func TestRunSync_DeclineAndRestore(t *testing.T) {
	root := setupTestRepo(t, "owner/template", "v1.0.0")

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		root,
		cloneInterceptor("new content"),
		func(_ string) (string, error) { return "v2.0.0", nil },
		testSpinner,
		func(_ string) (bool, error) { return false, nil }, // decline
		false,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify base/ was restored to original.
	data, err := os.ReadFile(filepath.Join(root, "base", "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "original" {
		t.Errorf("base/AGENTS.md = %q, want 'original'", data)
	}

	// Verify TEMPLATE_VERSION was NOT updated.
	data, err = os.ReadFile(filepath.Join(root, "TEMPLATE_VERSION"))
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
}

func TestRunSync_FetchError(t *testing.T) {
	root := setupTestRepo(t, "owner/template", "v1.0.0")

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		root,
		fakeRun("", nil),
		func(_ string) (string, error) { return "", fmt.Errorf("API error") },
		testSpinner,
		func(_ string) (bool, error) { return false, nil },
		false,
	)

	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "API error") {
		t.Errorf("unexpected error: %v", err)
	}
}
