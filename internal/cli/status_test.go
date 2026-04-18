package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// findChild returns the direct child of parent with the given Use prefix,
// or nil if no such child is registered. Cobra stores the first token of
// Use as the command's Name — so matching on strings.Split(Use, " ")[0] is
// equivalent to matching on cobra.Command.Name().
func findChild(parent *cobra.Command, use string) *cobra.Command {
	for _, c := range parent.Commands() {
		if strings.Split(c.Use, " ")[0] == use {
			return c
		}
	}
	return nil
}

// TestStatusCmd_RegistersFourSubCommands verifies the command tree wiring:
// requirements, requirement, features, feature are all registered as direct
// children of 'status'.
func TestStatusCmd_RegistersFourSubCommands(t *testing.T) {
	cmd := newStatusCmd()

	wanted := []string{"requirements", "requirement", "features", "feature"}
	for _, name := range wanted {
		if findChild(cmd, name) == nil {
			t.Errorf("status: expected sub-command %q to be registered, but it was not", name)
		}
	}

	// Sanity: nothing else was registered.
	got := make([]string, 0, len(cmd.Commands()))
	for _, c := range cmd.Commands() {
		got = append(got, strings.Split(c.Use, " ")[0])
	}
	if len(got) != len(wanted) {
		t.Errorf("status: expected exactly %d sub-commands %v, got %v", len(wanted), wanted, got)
	}
}

// TestStatusCmd_BareInvocationShowsHelp verifies the bare 'status' invocation
// prints help, exits 0, does not route to a default sub-command, and does not
// hang waiting for input.
//
// Cobra's help writes to stdout via the command's OutOrStdout stream.
func TestStatusCmd_BareInvocationShowsHelp(t *testing.T) {
	cmd := newStatusCmd()

	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("status bare invocation returned unexpected error: %v", err)
	}

	out := buf.String()
	// Help output should mention each of the four sub-commands so the human
	// knows what they can run.
	for _, token := range []string{"requirements", "requirement", "features", "feature"} {
		if !strings.Contains(out, token) {
			t.Errorf("status bare help missing sub-command %q in output:\n%s", token, out)
		}
	}
}

// TestStatusCmd_SubCommandsReturnNotImplemented verifies the stubs return a
// clean "not yet implemented" error rather than silently succeeding or
// panicking.
//
// Exact error shape is not important — later tasks replace these handlers —
// but they must return some error so the scaffold is clearly a stub.
func TestStatusCmd_SubCommandsReturnNotImplemented(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"requirements", []string{"requirements"}},
		{"requirement", []string{"requirement", "42"}},
		{"features", []string{"features"}},
		{"feature", []string{"feature", "42"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newStatusCmd()
			buf := &bytes.Buffer{}
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs(tc.args)

			err := cmd.Execute()
			if err == nil {
				t.Fatalf("%s: expected error from stub, got nil", tc.name)
			}
			if !errors.Is(err, errStatusNotImplemented) {
				t.Errorf("%s: expected errStatusNotImplemented, got %v", tc.name, err)
			}
		})
	}
}

// TestStatusCmd_ListFlagsRegistered verifies every flag that downstream tasks
// will consume is declared on the list sub-commands and parses without error.
func TestStatusCmd_ListFlagsRegistered(t *testing.T) {
	expected := []string{"json", "kanban", "horizontal", "this-repo", "include-done"}

	for _, parent := range []string{"requirements", "features"} {
		t.Run(parent, func(t *testing.T) {
			cmd := newStatusCmd()
			child := findChild(cmd, parent)
			if child == nil {
				t.Fatalf("status: sub-command %q not found", parent)
			}
			for _, name := range expected {
				if child.Flags().Lookup(name) == nil {
					t.Errorf("status %s: expected flag --%s to be declared, but it was not", parent, name)
				}
			}
		})
	}
}

// TestStatusCmd_DetailFlagsRegistered verifies the detail sub-commands declare
// at least --json. They do not use --kanban etc. — detail is always either
// rendered or JSON.
func TestStatusCmd_DetailFlagsRegistered(t *testing.T) {
	for _, parent := range []string{"requirement", "features"} {
		_ = parent
	}
	for _, parent := range []string{"requirement", "feature"} {
		t.Run(parent, func(t *testing.T) {
			cmd := newStatusCmd()
			child := findChild(cmd, parent)
			if child == nil {
				t.Fatalf("status: sub-command %q not found", parent)
			}
			if child.Flags().Lookup("json") == nil {
				t.Errorf("status %s: expected flag --json to be declared, but it was not", parent)
			}
		})
	}
}

// TestStatusCmd_DetailCommandsRequireOneArg verifies the detail commands
// enforce exactly one positional argument (the issue number). Zero args fails,
// two args fails, one arg parses.
func TestStatusCmd_DetailCommandsRequireOneArg(t *testing.T) {
	for _, parent := range []string{"requirement", "feature"} {
		t.Run(parent+"/no-arg", func(t *testing.T) {
			cmd := newStatusCmd()
			buf := &bytes.Buffer{}
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs([]string{parent})
			if err := cmd.Execute(); err == nil {
				t.Errorf("status %s: expected error with zero args, got nil", parent)
			}
		})
		t.Run(parent+"/too-many", func(t *testing.T) {
			cmd := newStatusCmd()
			buf := &bytes.Buffer{}
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs([]string{parent, "1", "2"})
			if err := cmd.Execute(); err == nil {
				t.Errorf("status %s: expected error with too many args, got nil", parent)
			}
		})
	}
}

// TestStatusCmd_ListCommandsRejectPositionalArgs verifies list sub-commands do
// not accept positional arguments — they work on the whole project.
func TestStatusCmd_ListCommandsRejectPositionalArgs(t *testing.T) {
	for _, parent := range []string{"requirements", "features"} {
		t.Run(parent, func(t *testing.T) {
			cmd := newStatusCmd()
			buf := &bytes.Buffer{}
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs([]string{parent, "42"})
			if err := cmd.Execute(); err == nil {
				t.Errorf("status %s: expected error when passed positional arg, got nil", parent)
			}
		})
	}
}

// TestStatusCmd_RegisteredOnRoot verifies the status command group is wired
// into the root command tree.
func TestStatusCmd_RegisteredOnRoot(t *testing.T) {
	root := newRootCmd("dev", "")
	if findChild(root, "status") == nil {
		t.Fatalf("root: expected 'status' sub-command to be registered")
	}
}
