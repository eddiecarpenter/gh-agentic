package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestBootstrapCmd_Help(t *testing.T) {
	buf := &bytes.Buffer{}
	root := newRootCmd()
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

func TestBootstrapCmd_Run_PrintsNotImplemented(t *testing.T) {
	buf := &bytes.Buffer{}
	root := newRootCmd()
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"bootstrap"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute() bootstrap returned unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "not yet implemented") {
		t.Errorf("expected stub output 'not yet implemented', got: %s", out)
	}
}
