package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestInceptionCmd_Help(t *testing.T) {
	buf := &bytes.Buffer{}
	root := newRootCmd("dev")
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
	root := newRootCmd("dev")
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
