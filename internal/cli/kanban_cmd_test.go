package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// TestKanbanCmd_RegisteredOnRoot verifies the new `gh agentic kanban`
// command appears as a direct child of the root command.
func TestKanbanCmd_RegisteredOnRoot(t *testing.T) {
	root := newRootCmd("test", "test")
	child := findChild(root, "kanban")
	if child == nil {
		t.Fatalf("kanban command not registered on root")
	}
	if child.Use != "kanban" {
		t.Errorf("Use = %q, want %q", child.Use, "kanban")
	}
}

// TestKanbanCmd_HelpListsFlags verifies --help text mentions every
// documented flag so users can discover the command's full surface.
func TestKanbanCmd_HelpListsFlags(t *testing.T) {
	cmd := newKanbanCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("kanban --help: %v", err)
	}
	out := buf.String()
	for _, tok := range []string{
		"--requirements",
		"--features",
		"--horizontal",
		"--vertical",
		"--include-done",
		"--this-repo",
		"--json",
	} {
		if !strings.Contains(out, tok) {
			t.Errorf("help missing flag %q; got:\n%s", tok, out)
		}
	}
}

// TestKanbanCmd_AllFlagsRegistered verifies every flag the scope
// documents is declared on the Cobra command — belt-and-braces over the
// help-output check in case the long-description wording drifts.
func TestKanbanCmd_AllFlagsRegistered(t *testing.T) {
	cmd := newKanbanCmd()
	for _, name := range []string{"requirements", "features", "horizontal", "vertical", "include-done", "this-repo", "json"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag --%s not registered", name)
		}
	}
}

// TestKanbanCmd_MutuallyExclusiveSelectorError verifies that passing both
// --requirements and --features produces the documented error and does
// not invoke the (stubbed) handler.
func TestKanbanCmd_MutuallyExclusiveSelectorError(t *testing.T) {
	cmd := newKanbanCmd()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"--requirements", "--features"})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected mutually-exclusive error; got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "--requirements") || !strings.Contains(msg, "--features") {
		t.Errorf("error should name both selector flags; got %q", msg)
	}
	if !strings.Contains(msg, "mutually exclusive") {
		t.Errorf("error should mention 'mutually exclusive'; got %q", msg)
	}
}

// TestKanbanCmd_PositionalArgsRejected verifies cobra.NoArgs is honoured
// so a stray positional fails fast without invoking the handler.
func TestKanbanCmd_PositionalArgsRejected(t *testing.T) {
	cmd := newKanbanCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"oops"})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error for positional argument; got nil")
	}
}

// TestKanbanCmd_BareInvocationReturnsStub verifies the scaffold handler
// is reachable and returns the not-implemented sentinel; task #523
// replaces it with real behaviour and this test is updated then.
func TestKanbanCmd_BareInvocationReturnsStub(t *testing.T) {
	cmd := newKanbanCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if !errors.Is(err, errKanbanNotImplemented) {
		t.Errorf("expected errKanbanNotImplemented; got %v", err)
	}
}

// TestKanbanCmd_RootLongDescriptionMentionsKanban verifies the top-level
// help advertises the new command so users discover it.
func TestKanbanCmd_RootLongDescriptionMentionsKanban(t *testing.T) {
	root := newRootCmd("test", "test")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("root --help: %v", err)
	}
	if !strings.Contains(buf.String(), "kanban") {
		t.Errorf("root help should mention 'kanban'; got:\n%s", buf.String())
	}
}
