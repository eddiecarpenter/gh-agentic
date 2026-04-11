package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionCmd_Help(t *testing.T) {
	buf := &bytes.Buffer{}
	root := newRootCmd("dev", "")
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"version", "--help"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute() version --help returned unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "version") {
		t.Errorf("expected help output to contain 'version', got: %s", out)
	}
}

func TestVersionCmd_Run_ShowsVersion(t *testing.T) {
	buf := &bytes.Buffer{}
	root := newRootCmd("1.2.3", "")
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"version"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute() version returned unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "1.2.3") {
		t.Errorf("expected version output to contain '1.2.3', got: %s", out)
	}
}

func TestVersionCmd_Run_DevBuild_NoDate(t *testing.T) {
	buf := &bytes.Buffer{}
	root := newRootCmd("dev", "")
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"version"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute() version returned unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "n/a (dev build)") {
		t.Errorf("expected 'n/a (dev build)' in output, got: %s", out)
	}
}

func TestVersionCmd_Run_WithValidRFC3339Date_ShowsFormattedDate(t *testing.T) {
	buf := &bytes.Buffer{}
	root := newRootCmd("1.5.1", "2025-04-10T12:00:00Z")
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"version"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute() version returned unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "2025-04-10") {
		t.Errorf("expected formatted date '2025-04-10' in output, got: %s", out)
	}
}

func TestVersionCmd_Run_WithInvalidDate_ShowsRawDate(t *testing.T) {
	buf := &bytes.Buffer{}
	root := newRootCmd("1.5.1", "not-a-date")
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"version"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute() version returned unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "not-a-date") {
		t.Errorf("expected raw date 'not-a-date' in output, got: %s", out)
	}
}

func TestVersionCmd_Run_ShowsVersionLabel(t *testing.T) {
	buf := &bytes.Buffer{}
	root := newRootCmd("0.0.1", "")
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"version"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() version returned unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Version:") {
		t.Errorf("expected 'Version:' label in output, got: %s", out)
	}
}

func TestVersionCmd_Run_ShowsReleasedLabel(t *testing.T) {
	buf := &bytes.Buffer{}
	root := newRootCmd("0.0.1", "2025-01-01T00:00:00Z")
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"version"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() version returned unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Released:") {
		t.Errorf("expected 'Released:' label in output, got: %s", out)
	}
}
