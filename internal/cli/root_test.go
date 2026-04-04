package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestExecute_Help(t *testing.T) {
	buf := &bytes.Buffer{}
	cmd := newRootCmd("dev", "")
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
	cmd := newRootCmd("v0.1.0-test", "")
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--version"})

	// Cobra --version exits with nil and writes to stdout.
	_ = cmd.Execute()

	out := buf.String()
	if !strings.Contains(out, "v0.1.0-test") {
		t.Errorf("expected --version output to contain %q, got: %s", "v0.1.0-test", out)
	}
}
