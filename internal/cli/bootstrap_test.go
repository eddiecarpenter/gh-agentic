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
	// Verify that the bootstrap subcommand is registered and reachable via --help.
	// The full RunE path (preflight → form → execution) is covered by
	// internal/bootstrap package tests using injected dependencies.
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
