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

// TestStatusCmd_RegistersExpectedSubCommands verifies the command tree
// wiring: requirements, requirement, features, feature, and pipeline are
// all registered as direct children of 'status'. Feature #549 moved the
// previously top-level kanban command under `status`; feature #562
// renamed it from `kanban` to `pipeline`. The expected list reflects
// the post-#562 surface.
func TestStatusCmd_RegistersExpectedSubCommands(t *testing.T) {
	cmd := newStatusCmd()

	wanted := []string{"requirements", "requirement", "features", "feature", "pipeline"}
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
	// Help output should mention each of the five sub-commands so the human
	// knows what they can run. The pipeline sub-command was added under
	// `status` by feature #549 and renamed from `kanban` to `pipeline` by
	// feature #562.
	for _, token := range []string{"requirements", "requirement", "features", "feature", "pipeline"} {
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
	// Only the still-stubbed sub-commands are listed here. As each sub-command
	// is implemented in a later task, its entry is removed from this table.
	// All four sub-commands are now wired (tasks #494–#499). This table
	// remains in the suite as a guard — if a future refactor re-introduces a
	// stub, add it here so the regression is caught.
	cases := []struct {
		name string
		args []string
	}{}

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

// TestStatusCmd_ListFlagsRegistered verifies every stable flag the list
// sub-commands expose is declared on both. After feature #518 the pipeline
// layout flags (--kanban, --horizontal, --vertical) no longer live on
// these commands — they have moved to `gh agentic status pipeline` (moved
// back under `status` by feature #549 and renamed from `kanban` to
// `pipeline` by feature #562). The `--json` flag was removed by feature
// #589 in favour of `--raw` / `--raw --verbose`.
func TestStatusCmd_ListFlagsRegistered(t *testing.T) {
	expected := []string{"raw", "verbose", "this-repo", "include-done"}

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
			// Removed layout / JSON flags must not appear in help output.
			for _, removed := range []string{"horizontal", "vertical", "json"} {
				if child.Flags().Lookup(removed) != nil {
					t.Errorf("status %s: flag --%s should have been removed but is still declared", parent, removed)
				}
			}
		})
	}
}

// TestStatusCmd_DetailFlagsRegistered verifies the detail sub-commands declare
// the agent-oriented flags (--raw, --verbose). The `--json` flag was removed
// by feature #589.
func TestStatusCmd_DetailFlagsRegistered(t *testing.T) {
	for _, parent := range []string{"requirement", "feature"} {
		t.Run(parent, func(t *testing.T) {
			cmd := newStatusCmd()
			child := findChild(cmd, parent)
			if child == nil {
				t.Fatalf("status: sub-command %q not found", parent)
			}
			for _, name := range []string{"raw", "verbose"} {
				if child.Flags().Lookup(name) == nil {
					t.Errorf("status %s: expected flag --%s to be declared, but it was not", parent, name)
				}
			}
			if child.Flags().Lookup("json") != nil {
				t.Errorf("status %s: --json should have been removed but is still declared", parent)
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

// TestStatusCmd_KanbanFlagHiddenOnList verifies the --kanban flag is still
// parsed on the list commands (to intercept it) but hidden from help so
// users don't see it as a supported option.
func TestStatusCmd_KanbanFlagHiddenOnList(t *testing.T) {
	for _, parent := range []string{"requirements", "features"} {
		t.Run(parent, func(t *testing.T) {
			cmd := newStatusCmd()
			child := findChild(cmd, parent)
			if child == nil {
				t.Fatalf("status: sub-command %q not found", parent)
			}
			f := child.Flags().Lookup("kanban")
			if f == nil {
				t.Fatalf("status %s: --kanban must remain declared so the migration error fires", parent)
			}
			if !f.Hidden {
				t.Errorf("status %s: --kanban must be hidden from help", parent)
			}
		})
	}
}

// TestStatusCmd_KanbanFlagProducesMigrationError verifies that passing the
// legacy --kanban flag to 'status requirements' / 'status features' fails
// with the documented two-line migration message and exits non-zero. The
// suggested command points at the `pipeline` sub-command (the command
// was renamed from `kanban` to `pipeline` by feature #562).
func TestStatusCmd_KanbanFlagProducesMigrationError(t *testing.T) {
	cases := []struct {
		name           string
		args           []string
		expectContains string
	}{
		{"requirements", []string{"requirements", "--kanban"}, "gh agentic status pipeline --requirements"},
		{"features", []string{"features", "--kanban"}, "gh agentic status pipeline --features"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newStatusCmdWithDeps(stubStatusDepsForMigration())
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			cmd.SetOut(stdout)
			cmd.SetErr(stderr)
			cmd.SetArgs(tc.args)
			err := cmd.Execute()
			if err == nil {
				t.Fatalf("%s: expected error; got nil", tc.name)
			}
			// Handler returns ErrSilent after renderStatusError writes
			// the formatted message on stderr — so the execution error
			// is ErrSilent and the visible text lives on stderr.
			if !errors.Is(err, ErrSilent) {
				t.Errorf("%s: expected ErrSilent, got %v", tc.name, err)
			}
			out := stderr.String()
			if !strings.Contains(out, "--kanban has been removed from this command") {
				t.Errorf("%s: expected migration heading in stderr; got:\n%s", tc.name, out)
			}
			if !strings.Contains(out, tc.expectContains) {
				t.Errorf("%s: expected stderr to name %q; got:\n%s", tc.name, tc.expectContains, out)
			}
		})
	}
}

// stubStatusDepsForMigration returns deps that never reach the network —
// the migration error fires before the fetch runs.
func stubStatusDepsForMigration() statusDeps {
	sd := defaultStatusDeps()
	// Short-circuit currentRepo and project lookup so the test doesn't
	// touch gh auth — the handler returns the migration error before
	// either runs, but defensive inputs keep the test hermetic.
	sd.currentRepo = func() (string, error) { return "eddiecarpenter/gh-agentic", nil }
	sd.resolveProjectID = func(string) (string, error) { return "PROJ_ID", nil }
	return sd
}

// newStatusCmdWithDeps mirrors newStatusCmd but injects an explicit deps
// bundle into every list sub-command so tests can stub the environment.
func newStatusCmdWithDeps(deps statusDeps) *cobra.Command {
	cmd := newStatusCmd()
	// Rebuild the list sub-commands with injected deps while leaving
	// the detail commands alone (they aren't exercised by these tests).
	for i, c := range cmd.Commands() {
		_ = i
		switch strings.Split(c.Use, " ")[0] {
		case "requirements":
			cmd.RemoveCommand(c)
			cmd.AddCommand(newStatusRequirementsCmdWithDeps(deps))
		case "features":
			cmd.RemoveCommand(c)
			cmd.AddCommand(newStatusFeaturesCmdWithDeps(deps))
		}
	}
	return cmd
}
