package cli

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestPromptGhNotify_NonDarwin_SkipsSilently(t *testing.T) {
	var buf bytes.Buffer
	confirmCalled := false
	confirm := func(prompt string) (bool, error) {
		confirmCalled = true
		return false, nil
	}
	run := func(name string, args ...string) (string, error) {
		t.Error("run should not be called on non-darwin")
		return "", nil
	}

	err := PromptGhNotify(&buf, "linux", "/tmp/clone", run, confirm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if confirmCalled {
		t.Error("confirm should not be called on non-darwin")
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output, got: %s", buf.String())
	}
}

func TestPromptGhNotify_Darwin_Declined_SkipsSilently(t *testing.T) {
	var buf bytes.Buffer
	runCalled := false
	confirm := func(prompt string) (bool, error) {
		return false, nil
	}
	run := func(name string, args ...string) (string, error) {
		runCalled = true
		return "", nil
	}

	err := PromptGhNotify(&buf, "darwin", "/tmp/clone", run, confirm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runCalled {
		t.Error("run should not be called when user declines")
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output, got: %s", buf.String())
	}
}

func TestPromptGhNotify_Darwin_Confirmed_Success(t *testing.T) {
	var buf bytes.Buffer
	var calledArgs []string
	clonePath := "/tmp/test-clone"

	confirm := func(prompt string) (bool, error) {
		return true, nil
	}
	run := func(name string, args ...string) (string, error) {
		calledArgs = append([]string{name}, args...)
		return "", nil
	}

	err := PromptGhNotify(&buf, "darwin", clonePath, run, confirm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedScript := filepath.Join(clonePath, "base", "scripts", "install-gh-notify.sh")
	if len(calledArgs) != 2 || calledArgs[0] != "bash" || calledArgs[1] != expectedScript {
		t.Errorf("expected run(bash, %s), got %v", expectedScript, calledArgs)
	}

	output := buf.String()
	if !strings.Contains(output, "gh-notify installed and running") {
		t.Errorf("expected success message, got: %s", output)
	}
}

func TestPromptGhNotify_Darwin_Confirmed_ScriptFails(t *testing.T) {
	var buf bytes.Buffer
	confirm := func(prompt string) (bool, error) {
		return true, nil
	}
	run := func(name string, args ...string) (string, error) {
		return "", fmt.Errorf("script exited 1")
	}

	err := PromptGhNotify(&buf, "darwin", "/tmp/clone", run, confirm)
	if err != nil {
		t.Fatalf("expected nil error (warning only), got: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "gh-notify install failed") {
		t.Errorf("expected warning message, got: %s", output)
	}
}

func TestPromptGhNotify_Darwin_ConfirmError_ReturnsError(t *testing.T) {
	var buf bytes.Buffer
	confirm := func(prompt string) (bool, error) {
		return false, fmt.Errorf("input error")
	}
	run := func(name string, args ...string) (string, error) {
		t.Error("run should not be called when confirm errors")
		return "", nil
	}

	err := PromptGhNotify(&buf, "darwin", "/tmp/clone", run, confirm)
	if err == nil {
		t.Fatal("expected error from confirm failure")
	}
	if !strings.Contains(err.Error(), "input error") {
		t.Errorf("expected confirm error, got: %v", err)
	}
}
