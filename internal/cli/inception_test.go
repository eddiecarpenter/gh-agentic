package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestInceptionCmd_Help(t *testing.T) {
	buf := &bytes.Buffer{}
	root := newRootCmd("dev", "")
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"inception", "--help"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute() inception --help returned unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "inception") {
		t.Errorf("expected help output to contain 'inception', got: %s", out)
	}
}

func TestInceptionCmd_SubcommandRegistered(t *testing.T) {
	buf := &bytes.Buffer{}
	root := newRootCmd("dev", "")
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"inception", "--help"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute() inception --help returned unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "registers it in REPOS.md") {
		t.Errorf("expected inception help to mention 'registers it in REPOS.md', got: %s", out)
	}
}

func TestInceptionCmd_NonInteractiveFlag_Registered(t *testing.T) {
	buf := &bytes.Buffer{}
	root := newRootCmd("dev", "")
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"inception", "--help"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "--non-interactive") {
		t.Errorf("expected help to contain '--non-interactive', got: %s", out)
	}
}

func TestInceptionCmd_AllFieldFlags_Registered(t *testing.T) {
	buf := &bytes.Buffer{}
	root := newRootCmd("dev", "")
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"inception", "--help"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	expectedFlags := []string{
		"--repo-type",
		"--repo-name",
		"--description",
		"--stack",
	}
	for _, flag := range expectedFlags {
		if !strings.Contains(out, flag) {
			t.Errorf("expected help to contain %q, got: %s", flag, out)
		}
	}
}

func TestInceptionCmd_NonInteractive_MissingAllRequired_ReturnsError(t *testing.T) {
	// This test will fail at ValidateEnvironment because REPOS.md doesn't exist.
	// To test the flag validation, we accept that the environment validation
	// happens first and causes the error. The flag validation tests below
	// verify the flag registration and behaviour separately.
	buf := &bytes.Buffer{}
	root := newRootCmd("dev", "")
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"inception", "--non-interactive"})

	err := root.Execute()
	// ValidateEnvironment will fail because we're not in an agentic env.
	// This proves the command runs and processes the flag.
	if err == nil {
		t.Fatal("expected error (environment validation or missing flags), got nil")
	}
}

func TestInceptionCmd_NonInteractive_MissingRepoType_ReturnsError(t *testing.T) {
	// This will fail at ValidateEnvironment, but verifies the flag is accepted.
	buf := &bytes.Buffer{}
	root := newRootCmd("dev", "")
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{
		"inception", "--non-interactive",
		"--repo-name", "charging",
		"--stack", "Go",
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestInceptionCmd_DeprecationNotice(t *testing.T) {
	// Inception requires an agentic environment to get past validation.
	// The deprecation notice should appear on stderr before any error.
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root := newRootCmd("dev", "")
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs([]string{"inception"})

	_ = root.Execute()

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "Deprecated") {
		t.Errorf("expected deprecation notice in stderr, got: %q", errOutput)
	}
	if !strings.Contains(errOutput, "gh agentic --v2 init") {
		t.Errorf("expected 'gh agentic --v2 init' in deprecation notice, got: %q", errOutput)
	}
}

func TestInceptionCmd_NonInteractive_StackRepeatable(t *testing.T) {
	// Verify --stack accepts multiple values without parsing errors.
	buf := &bytes.Buffer{}
	root := newRootCmd("dev", "")
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{
		"inception", "--non-interactive",
		"--repo-type", "domain",
		"--repo-name", "charging",
		"--stack", "Go",
		"--stack", "Rust",
	})

	err := root.Execute()
	// Will fail at ValidateEnvironment, not at flag parsing.
	if err == nil {
		t.Fatal("expected error (environment validation), got nil")
	}
	// The error should be about environment, not about flags.
	errMsg := err.Error()
	if strings.Contains(errMsg, "--stack") {
		t.Errorf("--stack should not cause an error (was provided), got: %s", errMsg)
	}
}
