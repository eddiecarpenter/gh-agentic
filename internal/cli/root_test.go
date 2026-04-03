package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersion_Constant(t *testing.T) {
	if Version == "" {
		t.Error("Version must not be empty")
	}
}

func TestExecute_Help(t *testing.T) {
	buf := &bytes.Buffer{}
	cmd := newRootCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("Execute() --help returned unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "gh-agentic") {
		t.Errorf("expected --help output to contain 'gh-agentic', got: %s", out)
	}
}

func TestExecute_Version(t *testing.T) {
	buf := &bytes.Buffer{}
	cmd := newRootCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--version"})

	// Cobra --version exits with nil and writes to stdout.
	_ = cmd.Execute()

	out := buf.String()
	if !strings.Contains(out, Version) {
		t.Errorf("expected --version output to contain %q, got: %s", Version, out)
	}
}
