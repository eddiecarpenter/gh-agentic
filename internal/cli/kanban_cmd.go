package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// errKanbanNotImplemented is the stub sentinel returned by the scaffold
// handler in task #522. Task #523 replaces the stub with the real
// implementation.
var errKanbanNotImplemented = fmt.Errorf("not yet implemented")

// kanbanFlags captures the flag surface of the new `gh agentic kanban`
// command. Selector flags (--requirements, --features) are mutually
// exclusive; the remaining fields mirror the inherited kanban layout
// flags so users can invoke the new command with the same semantics they
// already know from the old `status … --kanban` form.
type kanbanFlags struct {
	requirements bool
	features     bool
	horizontal   bool
	vertical     bool
	includeDone  bool
	thisRepo     bool
	json         bool
}

// registerKanbanFlags declares every flag the kanban command accepts.
// Declared once so the scaffold and the real handler share the same
// surface area, and so tests can look up any flag by name without
// depending on internal wiring detail.
func registerKanbanFlags(cmd *cobra.Command, f *kanbanFlags) {
	cmd.Flags().BoolVar(&f.requirements, "requirements", false, "render only the requirements kanban (mutually exclusive with --features)")
	cmd.Flags().BoolVar(&f.features, "features", false, "render only the features kanban (mutually exclusive with --requirements)")
	cmd.Flags().BoolVar(&f.horizontal, "horizontal", false, "force horizontal kanban regardless of terminal width")
	cmd.Flags().BoolVar(&f.vertical, "vertical", false, "force vertical kanban regardless of terminal width")
	cmd.Flags().BoolVar(&f.includeDone, "include-done", false, "include items in the 'done' stage")
	cmd.Flags().BoolVar(&f.thisRepo, "this-repo", false, "narrow the view to the current repository only")
	cmd.Flags().BoolVar(&f.json, "json", false, "emit a stable structured JSON payload and suppress human output")
}

// newKanbanCmd constructs the `gh agentic kanban` command with production
// dependencies. Tests use newKanbanCmdWithDeps to inject fakes.
func newKanbanCmd() *cobra.Command {
	return newKanbanCmdWithDeps(defaultStatusDeps())
}

// newKanbanCmdWithDeps builds the kanban command with an explicit
// statusDeps — the same dependency bundle used by the status sub-commands,
// because the new command reads the same project board and reuses the
// same renderer helpers.
func newKanbanCmdWithDeps(deps statusDeps) *cobra.Command {
	var flags kanbanFlags
	cmd := &cobra.Command{
		Use:   "kanban",
		Short: "Show the pipeline as a kanban — requirements and features together",
		Long: `Render the agentic project's pipeline as a kanban view.

Bare invocation renders the requirements kanban first, a blank-line
separator, then the features kanban, then a combined totals line. Use
--requirements or --features (mutually exclusive) to render only one of
the two kanbans.

The layout flags mirror the semantics of the legacy 'status … --kanban'
form: --horizontal forces horizontal layout, --vertical forces vertical,
and omitting both auto-picks based on terminal width. --include-done
appends the 'done' column. --this-repo narrows the federated view to
the current repository. --json emits a stable structured payload
suitable for jq, dashboards, and scripting.`,
		Example: `  # Both kanbans stacked
  gh agentic kanban

  # Requirements only
  gh agentic kanban --requirements

  # Features only, horizontal layout, include closed features
  gh agentic kanban --features --horizontal --include-done

  # JSON for scripting
  gh agentic kanban --json`,
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.requirements && flags.features {
				return fmt.Errorf("--requirements and --features are mutually exclusive")
			}
			return runKanban(cmd.OutOrStdout(), cmd.ErrOrStderr(), flags, deps)
		},
	}

	registerKanbanFlags(cmd, &flags)
	return cmd
}

// runKanban is the command handler. The scaffold implementation returns
// a sentinel error; task #523 replaces it with the real rendering path.
//
// stdout (w) receives the final human or JSON output. stderr receives
// the busy indicator rendered by deps.busy while the fetch is in flight.
func runKanban(w io.Writer, stderr io.Writer, flags kanbanFlags, deps statusDeps) error {
	_ = w
	_ = stderr
	_ = flags
	_ = deps
	return errKanbanNotImplemented
}
