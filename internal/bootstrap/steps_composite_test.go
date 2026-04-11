package bootstrap

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helpers for DeployCompositeActions tests

func makeState(clonePath string) *StepState {
	return &StepState{ClonePath: clonePath}
}

func makeStateExisting(clonePath string) *StepState {
	return &StepState{ClonePath: clonePath, ExistingRepo: true}
}

// createActionsSource populates clonePath/.ai/.github/actions/<subdir>/<file>
// with the given content and returns the actions source directory path.
func createActionsSource(t *testing.T, clonePath, subdir, filename, content string) {
	t.Helper()
	dir := filepath.Join(clonePath, ".ai", ".github", "actions", subdir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ── source dir absent → log and return nil ───────────────────────────────────

func TestDeployCompositeActions_NoSourceDir_ReturnsNilAndLogs(t *testing.T) {
	clonePath := t.TempDir()
	state := makeState(clonePath)
	var buf bytes.Buffer

	err := DeployCompositeActions(&buf, state, fakeRunOK(""))
	if err != nil {
		t.Fatalf("expected nil when source absent, got: %v", err)
	}
	if !strings.Contains(buf.String(), "No composite actions to deploy") {
		t.Errorf("expected 'No composite actions to deploy' in output, got: %s", buf.String())
	}
}

// ── source exists, all git commands succeed (new repo → push) ────────────────

func TestDeployCompositeActions_Success_NewRepo_PushesMain(t *testing.T) {
	clonePath := t.TempDir()
	createActionsSource(t, clonePath, "install-ai-tools", "action.yml", "name: install")
	state := makeState(clonePath) // ExistingRepo=false → push runs

	callLog := []string{}
	fakeRun := func(name string, args ...string) (string, error) {
		callLog = append(callLog, strings.Join(append([]string{name}, args...), " "))
		return "", nil
	}

	var buf bytes.Buffer
	err := DeployCompositeActions(&buf, state, fakeRun)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify deployed file exists.
	dest := filepath.Join(clonePath, ".github", "actions", "install-ai-tools", "action.yml")
	if _, statErr := os.Stat(dest); statErr != nil {
		t.Errorf("expected deployed file at %s: %v", dest, statErr)
	}

	// Verify git add, commit, and push were called.
	// runInDir wraps commands as: bash -c cd '<path>' && 'git' 'add' '<arg>'
	allCalls := strings.Join(callLog, "\n")
	if !strings.Contains(allCalls, "'git' 'add'") {
		t.Errorf("expected git add call, got:\n%s", allCalls)
	}
	if !strings.Contains(allCalls, "'git' 'commit'") {
		t.Errorf("expected git commit call, got:\n%s", allCalls)
	}
	if !strings.Contains(allCalls, "'git' 'push'") {
		t.Errorf("expected git push call for new repo, got:\n%s", allCalls)
	}
}

// ── source exists, ExistingRepo=true → no push ───────────────────────────────

func TestDeployCompositeActions_ExistingRepo_SkipsPush(t *testing.T) {
	clonePath := t.TempDir()
	createActionsSource(t, clonePath, "install-ai-tools", "action.yml", "name: install")
	state := makeStateExisting(clonePath) // ExistingRepo=true → no push

	pushCalled := false
	fakeRun := func(name string, args ...string) (string, error) {
		cmdLine := strings.Join(append([]string{name}, args...), " ")
		if strings.Contains(cmdLine, "'git' 'push'") {
			pushCalled = true
		}
		return "", nil
	}

	var buf bytes.Buffer
	if err := DeployCompositeActions(&buf, state, fakeRun); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pushCalled {
		t.Error("expected no push for ExistingRepo=true")
	}
}

// ── dest dir creation fails ───────────────────────────────────────────────────

func TestDeployCompositeActions_MkdirAllFails_ReturnsError(t *testing.T) {
	clonePath := t.TempDir()
	createActionsSource(t, clonePath, "install-ai-tools", "action.yml", "name: install")
	state := makeState(clonePath)

	// Block .github/ with a regular file so MkdirAll(.github/actions/) fails.
	if err := os.WriteFile(filepath.Join(clonePath, ".github"), []byte("blocker"), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err := DeployCompositeActions(&buf, state, fakeRunOK(""))
	if err == nil {
		t.Fatal("expected error when MkdirAll fails, got nil")
	}
	if !strings.Contains(err.Error(), "creating .github/actions/") {
		t.Errorf("expected 'creating .github/actions/' in error, got: %v", err)
	}
}

// ── WalkDir write fails ───────────────────────────────────────────────────────

func TestDeployCompositeActions_WriteFileFails_ReturnsError(t *testing.T) {
	clonePath := t.TempDir()
	createActionsSource(t, clonePath, "install-ai-tools", "action.yml", "name: install")
	state := makeState(clonePath)

	// Create destination dirs first, then block the target file with a directory.
	destDir := filepath.Join(clonePath, ".github", "actions", "install-ai-tools")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(destDir, "action.yml"), 0o755); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err := DeployCompositeActions(&buf, state, fakeRunOK(""))
	if err == nil {
		t.Fatal("expected error when WriteFile blocked, got nil")
	}
	if !strings.Contains(err.Error(), "deploying composite actions") {
		t.Errorf("expected 'deploying composite actions' in error, got: %v", err)
	}
}

// ── git add fails → logs warning, returns nil ────────────────────────────────

func TestDeployCompositeActions_GitAddFails_LogsWarning(t *testing.T) {
	clonePath := t.TempDir()
	createActionsSource(t, clonePath, "install-ai-tools", "action.yml", "name: install")
	state := makeState(clonePath)

	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			// First run call is git add — fail it.
			return "not a git repo", fmt.Errorf("git add failed")
		}
		return "", nil
	}

	var buf bytes.Buffer
	err := DeployCompositeActions(&buf, state, fakeRun)
	if err != nil {
		t.Fatalf("expected nil when git add fails (just warning), got: %v", err)
	}
	if !strings.Contains(buf.String(), "Could not stage composite actions") {
		t.Errorf("expected warning in output, got: %s", buf.String())
	}
}

// ── git commit fails → logs warning, returns nil ─────────────────────────────

func TestDeployCompositeActions_GitCommitFails_LogsWarning(t *testing.T) {
	clonePath := t.TempDir()
	createActionsSource(t, clonePath, "install-ai-tools", "action.yml", "name: install")
	state := makeState(clonePath)

	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			return "", nil // git add succeeds
		}
		return "nothing to commit", fmt.Errorf("git commit: exit 1")
	}

	var buf bytes.Buffer
	err := DeployCompositeActions(&buf, state, fakeRun)
	if err != nil {
		t.Fatalf("expected nil when git commit fails (just warning), got: %v", err)
	}
	if !strings.Contains(buf.String(), "Could not commit composite actions") {
		t.Errorf("expected commit warning in output, got: %s", buf.String())
	}
}

// ── git push fails → logs warning, returns nil ───────────────────────────────

func TestDeployCompositeActions_GitPushFails_LogsWarning(t *testing.T) {
	clonePath := t.TempDir()
	createActionsSource(t, clonePath, "install-ai-tools", "action.yml", "name: install")
	state := makeState(clonePath) // ExistingRepo=false → push runs

	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		if callCount <= 2 {
			return "", nil // git add and commit succeed
		}
		return "push failed", fmt.Errorf("git push: remote error")
	}

	var buf bytes.Buffer
	err := DeployCompositeActions(&buf, state, fakeRun)
	if err != nil {
		t.Fatalf("expected nil when git push fails (just warning), got: %v", err)
	}
	if !strings.Contains(buf.String(), "Could not push composite actions") {
		t.Errorf("expected push warning in output, got: %s", buf.String())
	}
}

// ── multiple action directories deployed correctly ───────────────────────────

func TestDeployCompositeActions_MultipleActions_AllDeployed(t *testing.T) {
	clonePath := t.TempDir()
	createActionsSource(t, clonePath, "install-ai-tools", "action.yml", "name: install")
	createActionsSource(t, clonePath, "setup-goose", "action.yml", "name: goose")
	state := makeState(clonePath)

	var buf bytes.Buffer
	if err := DeployCompositeActions(&buf, state, fakeRunOK("")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, check := range []string{
		filepath.Join(".github", "actions", "install-ai-tools", "action.yml"),
		filepath.Join(".github", "actions", "setup-goose", "action.yml"),
	} {
		if _, err := os.Stat(filepath.Join(clonePath, check)); err != nil {
			t.Errorf("expected file %s: %v", check, err)
		}
	}
}
