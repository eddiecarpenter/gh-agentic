package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestBootstrapCmd_Help(t *testing.T) {
	buf := &bytes.Buffer{}
	root := newRootCmd("dev", "")
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"bootstrap", "--help"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute() bootstrap --help returned unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "bootstrap") {
		t.Errorf("expected help output to contain 'bootstrap', got: %s", out)
	}
}

func TestBootstrapCmd_Run_SubcommandRegistered(t *testing.T) {
	buf := &bytes.Buffer{}
	root := newRootCmd("dev", "")
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"bootstrap", "--help"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute() bootstrap --help returned unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "agentic development environment") {
		t.Errorf("expected bootstrap help to mention 'agentic development environment', got: %s", out)
	}
}

func TestBootstrapCmd_NonInteractiveFlag_Registered(t *testing.T) {
	buf := &bytes.Buffer{}
	root := newRootCmd("dev", "")
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"bootstrap", "--help"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "--non-interactive") {
		t.Errorf("expected help to contain '--non-interactive', got: %s", out)
	}
}

func TestBootstrapCmd_AllFieldFlags_Registered(t *testing.T) {
	buf := &bytes.Buffer{}
	root := newRootCmd("dev", "")
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"bootstrap", "--help"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	expectedFlags := []string{
		"--topology",
		"--owner",
		"--repo",
		"--project-name",
		"--description",
		"--stack",
		"--agent-user",
		"--antora",
		"--runner",
		"--provider",
		"--model",
		"--pat",
	}
	for _, flag := range expectedFlags {
		if !strings.Contains(out, flag) {
			t.Errorf("expected help to contain %q, got: %s", flag, out)
		}
	}
}

func TestBootstrapCmd_NonInteractive_MissingAllRequired_ReturnsError(t *testing.T) {
	buf := &bytes.Buffer{}
	root := newRootCmd("dev", "")
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"bootstrap", "--non-interactive"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing required flags, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "--non-interactive requires") {
		t.Errorf("expected '--non-interactive requires' in error, got: %s", errMsg)
	}

	// Verify all missing flags are listed.
	for _, flag := range []string{"--topology", "--owner", "--project-name", "--agent-user", "--stack", "--runner", "--provider", "--model", "--pat"} {
		if !strings.Contains(errMsg, flag) {
			t.Errorf("expected missing flag %q in error message, got: %s", flag, errMsg)
		}
	}
}

func TestBootstrapCmd_NonInteractive_MissingSomeRequired_ReturnsError(t *testing.T) {
	buf := &bytes.Buffer{}
	root := newRootCmd("dev", "")
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{
		"bootstrap", "--non-interactive",
		"--topology", "Single",
		"--owner", "alice",
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing required flags, got nil")
	}

	errMsg := err.Error()
	// Should list the missing flags, not the provided ones.
	if strings.Contains(errMsg, "--topology") {
		t.Errorf("should not list --topology as missing (it was provided), got: %s", errMsg)
	}
	if strings.Contains(errMsg, "--owner") {
		t.Errorf("should not list --owner as missing (it was provided), got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "--project-name") {
		t.Errorf("expected --project-name in missing flags, got: %s", errMsg)
	}
}

func TestBootstrapCmd_NonInteractive_InvalidProjectName_ReturnsError(t *testing.T) {
	buf := &bytes.Buffer{}
	root := newRootCmd("dev", "")
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{
		"bootstrap", "--non-interactive",
		"--topology", "Single",
		"--owner", "alice",
		"--project-name", "InvalidName",
		"--agent-user", "bot",
		"--stack", "Go",
		"--runner", "ubuntu-latest",
		"--provider", "anthropic",
		"--model", "default",
		"--pat", "ghp_xxx",
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid project name, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "invalid --project-name") {
		t.Errorf("expected 'invalid --project-name' in error, got: %s", errMsg)
	}
}

func TestBootstrapCmd_NonInteractive_StackRepeatable(t *testing.T) {
	buf := &bytes.Buffer{}
	root := newRootCmd("dev", "")
	root.SetOut(buf)
	root.SetErr(buf)
	// Use missing --owner to trigger the missing-flags error early,
	// but verify --stack accepts multiple values.
	root.SetArgs([]string{
		"bootstrap", "--non-interactive",
		"--topology", "Single",
		"--project-name", "test",
		"--agent-user", "bot",
		"--stack", "Go",
		"--stack", "Rust",
		"--runner", "ubuntu-latest",
		"--provider", "anthropic",
		"--model", "default",
		"--pat", "ghp_xxx",
	})

	err := root.Execute()
	// This will fail because --owner is missing, but --stack should not cause an error.
	if err == nil {
		t.Fatal("expected error for missing --owner, got nil")
	}
	errMsg := err.Error()
	if strings.Contains(errMsg, "--stack") {
		t.Errorf("--stack was provided (twice) but listed as missing: %s", errMsg)
	}
	if !strings.Contains(errMsg, "--owner") {
		t.Errorf("expected --owner in missing flags, got: %s", errMsg)
	}
}

func TestBootstrapCmd_NonInteractive_EmptyStack_ReturnsError(t *testing.T) {
	buf := &bytes.Buffer{}
	root := newRootCmd("dev", "")
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{
		"bootstrap", "--non-interactive",
		"--topology", "Single",
		"--owner", "alice",
		"--project-name", "test",
		"--agent-user", "bot",
		"--runner", "ubuntu-latest",
		"--provider", "anthropic",
		"--model", "default",
		"--pat", "ghp_xxx",
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing --stack, got nil")
	}
	if !strings.Contains(err.Error(), "--stack") {
		t.Errorf("expected --stack in error, got: %s", err.Error())
	}
}

func TestBootstrapCmd_DeprecationNotice(t *testing.T) {
	// Bootstrap requires non-interactive flags to get past preflight.
	// The deprecation notice should appear on stderr regardless of whether
	// the command succeeds or fails. We use --non-interactive with missing
	// flags to trigger early failure, but the notice prints before validation.
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root := newRootCmd("dev", "")
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs([]string{"bootstrap", "--non-interactive"})

	_ = root.Execute()

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "Deprecated") {
		t.Errorf("expected deprecation notice in stderr, got: %q", errOutput)
	}
	if !strings.Contains(errOutput, "gh agentic --v2 init") {
		t.Errorf("expected 'gh agentic --v2 init' in deprecation notice, got: %q", errOutput)
	}
}
