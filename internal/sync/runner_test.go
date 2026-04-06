package sync

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
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

// cloneRunnerWithWorkflows is like cloneRunner but also creates workflow files
// in the cloned template's base/.github/workflows/.
func cloneRunnerWithWorkflows(mock *testutil.MockRunner, baseContent string, workflows []string) func(string, ...string) (string, error) {
	return func(name string, args ...string) (string, error) {
		if name == "git" && len(args) >= 1 && args[0] == "clone" {
			targetDir := args[len(args)-1]
			_ = os.MkdirAll(filepath.Join(targetDir, "base"), 0o755)
			_ = os.WriteFile(filepath.Join(targetDir, "base", "AGENTS.md"), []byte(baseContent), 0o644)
			wfDir := filepath.Join(targetDir, "base", ".github", "workflows")
			_ = os.MkdirAll(wfDir, 0o755)
			for _, wf := range workflows {
				_ = os.WriteFile(filepath.Join(wfDir, wf), []byte("workflow: "+wf), 0o644)
			}
			return "", nil
		}
		return mock.RunCommand(name, args...)
	}
}

// fakeDetectOwnerType returns a DetectOwnerTypeFunc that always returns the given type.
func fakeDetectOwnerType(ownerType string) bootstrap.DetectOwnerTypeFunc {
	return func(owner string) (string, error) {
		return ownerType, nil
	}
}

// fakeDetectOwnerTypeError returns a DetectOwnerTypeFunc that always returns an error.
func fakeDetectOwnerTypeError() bootstrap.DetectOwnerTypeFunc {
	return func(owner string) (string, error) {
		return "", fmt.Errorf("API error")
	}
}

func TestRunSync_UpToDate(t *testing.T) {
	repo := testutil.NewFakeRepo(t)

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
		false,
		nil,
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

func TestRunSync_ConfirmAndStageOnly(t *testing.T) {
	repo := testutil.NewFakeRepo(t)

	mock := &testutil.MockRunner{}

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		cloneRunner(mock, "updated content"),
		testutil.FakeRelease("v2.0.0", nil),
		testutil.NoopSpinner,
		func(_ string) (bool, error) { return true, nil },
		false,
		false,
		nil,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(repo.Root, "TEMPLATE_VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "v2.0.0" {
		t.Errorf("TEMPLATE_VERSION = %q, want v2.0.0", string(data))
	}

	data, err = os.ReadFile(filepath.Join(repo.Root, "base", "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "updated content" {
		t.Errorf("base/AGENTS.md = %q, want 'updated content'", data)
	}

	output := buf.String()
	if !strings.Contains(output, "Sync applied") {
		t.Errorf("expected 'Sync applied' message, got: %s", output)
	}
	if !strings.Contains(output, "git diff --cached") {
		t.Errorf("expected review instructions, got: %s", output)
	}

	mock.AssertExpectations(t)
}

func TestRunSync_ConfirmAndCommit(t *testing.T) {
	repo := testutil.NewFakeRepo(t)

	mock := &testutil.MockRunner{}

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		cloneRunner(mock, "updated content"),
		testutil.FakeRelease("v2.0.0", nil),
		testutil.NoopSpinner,
		func(_ string) (bool, error) { return true, nil },
		false,
		true,
		nil,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(repo.Root, "TEMPLATE_VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "v2.0.0" {
		t.Errorf("TEMPLATE_VERSION = %q, want v2.0.0", string(data))
	}

	data, err = os.ReadFile(filepath.Join(repo.Root, "base", "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "updated content" {
		t.Errorf("base/AGENTS.md = %q, want 'updated content'", data)
	}

	output := buf.String()
	if !strings.Contains(output, "Sync committed") {
		t.Errorf("expected 'Sync committed' message, got: %s", output)
	}
	if !strings.Contains(output, "Remember to push") {
		t.Errorf("expected push reminder, got: %s", output)
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
		func(_ string) (bool, error) { return false, nil },
		false,
		false,
		nil,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(repo.Root, "base", "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "# AGENTS.md\n" {
		t.Errorf("base/AGENTS.md = %q, want '# AGENTS.md\\n'", data)
	}

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
		false,
		nil,
	)

	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "API error") {
		t.Errorf("unexpected error: %v", err)
	}

	mock.AssertExpectations(t)
}

func TestRunSync_ForceResyncsWhenUpToDate(t *testing.T) {
	repo := testutil.NewFakeRepo(t)

	mock := &testutil.MockRunner{}

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		cloneRunner(mock, "re-synced content"),
		testutil.FakeRelease("v1.0.0", nil),
		testutil.NoopSpinner,
		func(_ string) (bool, error) { return true, nil },
		true,
		false,
		nil,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "force") && !strings.Contains(output, "re-sync") {
		t.Errorf("expected force/re-sync warning in output, got: %s", output)
	}

	data, err := os.ReadFile(filepath.Join(repo.Root, "base", "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "re-synced content" {
		t.Errorf("base/AGENTS.md = %q, want 're-synced content'", data)
	}

	data, err = os.ReadFile(filepath.Join(repo.Root, "TEMPLATE_VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "v1.0.0" {
		t.Errorf("TEMPLATE_VERSION = %q, want v1.0.0", string(data))
	}

	mock.AssertExpectations(t)
}

func TestRunSync_BaseMissing_RestoresOnConfirm(t *testing.T) {
	repo := testutil.NewFakeRepo(t)

	if err := os.RemoveAll(filepath.Join(repo.Root, "base")); err != nil {
		t.Fatal(err)
	}

	mock := &testutil.MockRunner{}

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		cloneRunner(mock, "restored content"),
		testutil.FakeRelease("v1.0.0", nil),
		testutil.NoopSpinner,
		func(_ string) (bool, error) { return true, nil },
		false,
		false,
		nil,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "missing") {
		t.Errorf("expected 'missing' warning in output, got: %s", output)
	}

	data, err := os.ReadFile(filepath.Join(repo.Root, "base", "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "restored content" {
		t.Errorf("base/AGENTS.md = %q, want 'restored content'", data)
	}

	mock.AssertExpectations(t)
}

func TestRunSync_YesAutoConfirms(t *testing.T) {
	repo := testutil.NewFakeRepo(t)

	mock := &testutil.MockRunner{}

	confirmCalled := 0
	confirmFn := func(_ string) (bool, error) {
		confirmCalled++
		return true, nil
	}

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		cloneRunner(mock, "auto-confirmed content"),
		testutil.FakeRelease("v2.0.0", nil),
		testutil.NoopSpinner,
		confirmFn,
		false,
		false,
		nil,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if confirmCalled != 1 {
		t.Errorf("expected confirm called 1 time, got %d", confirmCalled)
	}

	data, err := os.ReadFile(filepath.Join(repo.Root, "TEMPLATE_VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "v2.0.0" {
		t.Errorf("TEMPLATE_VERSION = %q, want v2.0.0", string(data))
	}

	output := buf.String()
	if !strings.Contains(output, "Sync applied") {
		t.Errorf("expected 'Sync applied' message, got: %s", output)
	}

	mock.AssertExpectations(t)
}

func TestRunSync_UserOwner_SkipsSyncStatusToLabel(t *testing.T) {
	repo := testutil.NewFakeRepo(t)

	// Write AGENTS.local.md with owner info.
	agentsLocal := "## Repo\n\n- **Owner:** alice\n"
	if err := os.WriteFile(filepath.Join(repo.Root, "AGENTS.local.md"), []byte(agentsLocal), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a pre-existing sync-status-to-label.yml to test retroactive removal.
	wfDir := filepath.Join(repo.Root, ".github", "workflows")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, "sync-status-to-label.yml"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	mock := &testutil.MockRunner{}
	workflows := []string{"dev-session.yml", "sync-status-to-label.yml"}

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		cloneRunnerWithWorkflows(mock, "updated", workflows),
		testutil.FakeRelease("v2.0.0", nil),
		testutil.NoopSpinner,
		func(_ string) (bool, error) { return true, nil },
		false,
		false,
		fakeDetectOwnerType(bootstrap.OwnerTypeUser),
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify sync-status-to-label.yml was NOT deployed and pre-existing one was removed.
	syncPath := filepath.Join(repo.Root, ".github", "workflows", "sync-status-to-label.yml")
	if _, err := os.Stat(syncPath); err == nil {
		t.Error("sync-status-to-label.yml should NOT exist for personal repos")
	}

	// Verify dev-session.yml WAS deployed.
	devPath := filepath.Join(repo.Root, ".github", "workflows", "dev-session.yml")
	if _, err := os.Stat(devPath); os.IsNotExist(err) {
		t.Error("dev-session.yml should be deployed")
	}

	mock.AssertExpectations(t)
}

func TestRunSync_OrgOwner_IncludesSyncStatusToLabel(t *testing.T) {
	repo := testutil.NewFakeRepo(t)

	// Write AGENTS.local.md with org owner info.
	agentsLocal := "## Repo\n\n- **Owner:** acme-org\n"
	if err := os.WriteFile(filepath.Join(repo.Root, "AGENTS.local.md"), []byte(agentsLocal), 0o644); err != nil {
		t.Fatal(err)
	}

	mock := &testutil.MockRunner{}
	workflows := []string{"dev-session.yml", "sync-status-to-label.yml"}

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		cloneRunnerWithWorkflows(mock, "updated", workflows),
		testutil.FakeRelease("v2.0.0", nil),
		testutil.NoopSpinner,
		func(_ string) (bool, error) { return true, nil },
		false,
		false,
		fakeDetectOwnerType(bootstrap.OwnerTypeOrg),
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify sync-status-to-label.yml WAS deployed for org repos.
	syncPath := filepath.Join(repo.Root, ".github", "workflows", "sync-status-to-label.yml")
	if _, err := os.Stat(syncPath); os.IsNotExist(err) {
		t.Error("sync-status-to-label.yml should be deployed for org repos")
	}

	mock.AssertExpectations(t)
}

func TestRunSync_DetectOwnerTypeError_FallbackDeploysAll(t *testing.T) {
	repo := testutil.NewFakeRepo(t)

	// Write AGENTS.local.md with owner info.
	agentsLocal := "## Repo\n\n- **Owner:** alice\n"
	if err := os.WriteFile(filepath.Join(repo.Root, "AGENTS.local.md"), []byte(agentsLocal), 0o644); err != nil {
		t.Fatal(err)
	}

	mock := &testutil.MockRunner{}
	workflows := []string{"dev-session.yml", "sync-status-to-label.yml"}

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		cloneRunnerWithWorkflows(mock, "updated", workflows),
		testutil.FakeRelease("v2.0.0", nil),
		testutil.NoopSpinner,
		func(_ string) (bool, error) { return true, nil },
		false,
		false,
		fakeDetectOwnerTypeError(), // detection fails
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify sync-status-to-label.yml WAS deployed (safe fallback).
	syncPath := filepath.Join(repo.Root, ".github", "workflows", "sync-status-to-label.yml")
	if _, err := os.Stat(syncPath); os.IsNotExist(err) {
		t.Error("sync-status-to-label.yml should be deployed when detection fails (safe fallback)")
	}

	mock.AssertExpectations(t)
}

func TestRunSync_NilDetectOwnerType_DeploysAll(t *testing.T) {
	repo := testutil.NewFakeRepo(t)

	// Write AGENTS.local.md with owner info.
	agentsLocal := "## Repo\n\n- **Owner:** alice\n"
	if err := os.WriteFile(filepath.Join(repo.Root, "AGENTS.local.md"), []byte(agentsLocal), 0o644); err != nil {
		t.Fatal(err)
	}

	mock := &testutil.MockRunner{}
	workflows := []string{"dev-session.yml", "sync-status-to-label.yml"}

	var buf bytes.Buffer
	err := RunSync(
		&buf,
		repo.Root,
		cloneRunnerWithWorkflows(mock, "updated", workflows),
		testutil.FakeRelease("v2.0.0", nil),
		testutil.NoopSpinner,
		func(_ string) (bool, error) { return true, nil },
		false,
		false,
		nil, // no detect function
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify sync-status-to-label.yml WAS deployed when detectOwnerType is nil.
	syncPath := filepath.Join(repo.Root, ".github", "workflows", "sync-status-to-label.yml")
	if _, err := os.Stat(syncPath); os.IsNotExist(err) {
		t.Error("sync-status-to-label.yml should be deployed when detectOwnerType is nil")
	}

	mock.AssertExpectations(t)
}
