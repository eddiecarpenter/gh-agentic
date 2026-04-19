package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// errStatusNotImplemented is returned by status sub-command stubs until the
// handler is implemented in a later task (#495 onward).
var errStatusNotImplemented = fmt.Errorf("not yet implemented")

// newStatusCmd constructs the `gh agentic status` command group.
//
// Bare invocation prints help listing sub-commands and exits 0 — there is no
// default sub-command. Help covers the four leaf sub-commands and the shared
// plumbing they rely on (federation, blocked-by, JSON schemas).
func newStatusCmd() *cobra.Command {
	b := ui.SectionHeading.Render
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show pipeline state across requirements and features",
		Long: fmt.Sprintf(`Single-pane pipeline status view for the agentic project.

Answers "where are we?" without hunting across issues, labels, branches, and PRs.
All sub-commands accept --raw for an agent-oriented payload (tab-separated
list rows or frontmatter+verbatim-body details) suitable for scripting and
downstream skills.

By default the view aggregates across every active repo in the federation
(control plane and domain repos). Use --this-repo on any sub-command to narrow
to the current repository (detected via 'git remote get-url origin').

%s lists open requirements with stage, supports --include-done to include
completed items.

%s shows detail for a single requirement — number, title, stage, dates, body,
linked features, and blocked annotation where applicable.

%s lists open features with stage, supports --include-done to include
completed items.

%s shows detail for a single feature — number, title, stage, dates, body,
parent requirement, tasks, branch state, PR state, and blocked annotation.

%s renders requirements and features together as a side-by-side pipeline
view — the first-class way to answer "where are we?" at a glance.

Run 'gh agentic status <sub-command> --help' for detailed usage.`,
			b("requirements"), b("requirement <N>"), b("features"), b("feature <N>"), b("pipeline")),
		Example: `  # List open requirements with stage
  gh agentic status requirements

  # Detail for one requirement, in agent-oriented raw form
  gh agentic status requirement 457 --raw

  # Pipeline of requirements and features together
  gh agentic status pipeline

  # Detail for one feature
  gh agentic status feature 492`,
		// Bare invocation: print help, exit 0, do not hang on stdin.
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newStatusRequirementsCmd())
	cmd.AddCommand(newStatusRequirementCmd())
	cmd.AddCommand(newStatusFeaturesCmd())
	cmd.AddCommand(newStatusFeatureCmd())
	cmd.AddCommand(newPipelineCmd())

	return cmd
}

// statusListFlags captures the shared set of flags used by list sub-commands.
// Declared once so every list command registers the same shape; downstream
// tasks wire the actual behaviour behind each flag.
type statusListFlags struct {
	raw         bool
	verbose     bool
	kanban      bool
	horizontal  bool
	vertical    bool
	thisRepo    bool
	includeDone bool
}

// registerStatusListFlags declares the shared flag set for list sub-commands.
// After feature #518 the list sub-commands no longer carry pipeline-rendering
// flags (`--kanban`, `--horizontal`, `--vertical`) — those live on the
// `gh agentic status pipeline` command (promoted to top-level by #518, moved
// back under `status` by #549, and renamed from `kanban` to `pipeline` by
// #562). registerRemovedKanbanFlag declares `--kanban` as a hidden boolean
// on the status list commands so the handler can intercept it and emit a
// guided migration error.
func registerStatusListFlags(cmd *cobra.Command, f *statusListFlags) {
	cmd.Flags().BoolVar(&f.raw, "raw", false, "emit agent-oriented raw output (tab-separated for lists, frontmatter + markdown for details) and suppress human output")
	cmd.Flags().BoolVar(&f.verbose, "verbose", false, "include timestamps in --raw output (no-op without --raw)")
	cmd.Flags().BoolVar(&f.thisRepo, "this-repo", false, "narrow the view to the current repository only")
	cmd.Flags().BoolVar(&f.includeDone, "include-done", false, "include items in the 'done' stage")
}

// registerRemovedKanbanFlag declares --kanban as a hidden boolean so the
// parse layer still accepts it. The handler checks the flag and returns a
// migration error pointing at the new sub-command. This is a deliberate
// breaking change — no deprecation grace period, per §6 of the feature
// scope — but the migration error message is actionable. The flag name
// `--kanban` is preserved here because it is the legacy user-facing flag
// being intercepted; renaming it would break the intercept.
func registerRemovedKanbanFlag(cmd *cobra.Command, kanban *bool) {
	cmd.Flags().BoolVar(kanban, "kanban", false, "removed — use 'gh agentic status pipeline' instead")
	_ = cmd.Flags().MarkHidden("kanban")
}

// statusDetailFlags captures the shared flag set for detail sub-commands.
type statusDetailFlags struct {
	raw     bool
	verbose bool
}

// registerStatusDetailFlags declares the shared flag set for detail sub-commands.
func registerStatusDetailFlags(cmd *cobra.Command, f *statusDetailFlags) {
	cmd.Flags().BoolVar(&f.raw, "raw", false, "emit agent-oriented raw output (frontmatter header + '---' + verbatim markdown body) and suppress human output")
	cmd.Flags().BoolVar(&f.verbose, "verbose", false, "include timestamps in --raw output (no-op without --raw)")
}

// newStatusRequirementsCmd constructs the `gh agentic status requirements`
// command. It is a thin wrapper that resolves default dependencies and
// delegates to newStatusRequirementsCmdWithDeps — tests construct the
// command directly with injected fakes.
func newStatusRequirementsCmd() *cobra.Command {
	return newStatusRequirementsCmdWithDeps(defaultStatusDeps())
}

// newStatusRequirementsCmdWithDeps builds the command with an explicit
// statusDeps for testing.
func newStatusRequirementsCmdWithDeps(deps statusDeps) *cobra.Command {
	var flags statusListFlags
	cmd := &cobra.Command{
		Use:   "requirements",
		Short: "List open requirements with stage",
		Long: `List open requirements across the agentic project with their pipeline stage.

Default output is a compact one-line-per-item table. Stage is shown verbatim as
the GitHub label name (backlog, scoping, scheduled, done). Items that are
blocked by another issue carry an inline '[blocked by <owner>/<repo>#N]' annotation.

Pass --raw to emit an agent-oriented tab-separated payload (header row +
one row per item, sparse cells render as '-'). Pass --raw --verbose to
append created_at and last_transitioned_at columns.
Pass --include-done to include completed requirements; by default only open
items are listed.

For a side-by-side pipeline view, use 'gh agentic status pipeline --requirements'.`,
		Example: `  # Default list view
  gh agentic status requirements

  # Agent-oriented raw TSV (no header glyphs, no totals)
  gh agentic status requirements --raw

  # Include closed requirements
  gh agentic status requirements --include-done

  # Side-by-side pipeline view
  gh agentic status pipeline --requirements`,
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := runStatusRequirements(cmd.OutOrStdout(), cmd.ErrOrStderr(), flags, deps); err != nil {
				return renderStatusError(cmd, err)
			}
			return nil
		},
	}

	registerStatusListFlags(cmd, &flags)
	registerRemovedKanbanFlag(cmd, &flags.kanban)
	return cmd
}

// newStatusRequirementCmd constructs the `gh agentic status requirement <N>`
// detail command using production dependencies.
func newStatusRequirementCmd() *cobra.Command {
	return newStatusRequirementCmdWithDeps(defaultStatusDeps())
}

// newStatusRequirementCmdWithDeps builds the detail command with an explicit
// statusDeps for testing.
func newStatusRequirementCmdWithDeps(deps statusDeps) *cobra.Command {
	var flags statusDetailFlags
	cmd := &cobra.Command{
		Use:   "requirement <number>",
		Short: "Show detail for one requirement",
		Long: `Show detail for a single requirement: number, title, stage, dates, full body,
linked features (with state), and blocked annotation where applicable.

The issue number is a plain integer (e.g. 466); no '#' prefix is required. The
command errors with a clear message if the issue does not exist or if the issue
is a feature rather than a requirement.

Pass --raw to emit a frontmatter-style header (key: value lines), a literal
'---' separator, and the verbatim issue body — suitable for agent ingestion.
Pass --raw --verbose to insert created_at and last_transitioned_at header lines.`,
		Example: `  # Default detail view
  gh agentic status requirement 466

  # Agent-oriented raw frontmatter + verbatim body
  gh agentic status requirement 466 --raw`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			n, err := parseIssueNumberArg(args[0])
			if err != nil {
				return err
			}
			if err := runStatusRequirement(cmd.OutOrStdout(), cmd.ErrOrStderr(), n, flags, deps); err != nil {
				return renderStatusError(cmd, err)
			}
			return nil
		},
	}

	registerStatusDetailFlags(cmd, &flags)
	return cmd
}

// newStatusFeaturesCmd constructs the `gh agentic status features` command.
func newStatusFeaturesCmd() *cobra.Command {
	return newStatusFeaturesCmdWithDeps(defaultStatusDeps())
}

// newStatusFeaturesCmdWithDeps builds the command with an explicit statusDeps
// for testing.
func newStatusFeaturesCmdWithDeps(deps statusDeps) *cobra.Command {
	var flags statusListFlags
	cmd := &cobra.Command{
		Use:   "features",
		Short: "List open features with stage",
		Long: `List open features across the agentic project with their pipeline stage.

Default output is a compact one-line-per-item table. Stage is shown verbatim as
the GitHub label name (backlog, in-design, in-development, in-review, done).
The owning repo is shown when it differs from the current repo. Features that
are blocked by another issue carry an inline '[blocked by <owner>/<repo>#N]'
annotation.

Pass --raw to emit an agent-oriented tab-separated payload (header row +
one row per item, sparse cells render as '-'). Pass --raw --verbose to
append created_at and last_transitioned_at columns.
Pass --include-done to include completed features; by default only open items
are listed.

For a side-by-side pipeline view, use 'gh agentic status pipeline --features'.`,
		Example: `  # Default list view
  gh agentic status features

  # Agent-oriented raw TSV (no header glyphs, no totals)
  gh agentic status features --raw

  # Narrow to the current repo
  gh agentic status features --this-repo

  # Side-by-side pipeline view
  gh agentic status pipeline --features`,
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := runStatusFeatures(cmd.OutOrStdout(), cmd.ErrOrStderr(), flags, deps); err != nil {
				return renderStatusError(cmd, err)
			}
			return nil
		},
	}

	registerStatusListFlags(cmd, &flags)
	registerRemovedKanbanFlag(cmd, &flags.kanban)
	return cmd
}

// newStatusFeatureCmd constructs the `gh agentic status feature <N>` detail
// command using production dependencies.
func newStatusFeatureCmd() *cobra.Command {
	return newStatusFeatureCmdWithDeps(defaultStatusDeps())
}

// newStatusFeatureCmdWithDeps builds the command with an explicit statusDeps
// for testing.
func newStatusFeatureCmdWithDeps(deps statusDeps) *cobra.Command {
	var flags statusDetailFlags
	cmd := &cobra.Command{
		Use:   "feature <number>",
		Short: "Show detail for one feature",
		Long: `Show detail for a single feature: number, title, stage, dates, full body,
parent requirement, tasks (with open/done glyphs), branch state, PR state, and
blocked annotation where applicable.

The issue number is a plain integer (e.g. 492); no '#' prefix is required. The
command errors with a clear message if the issue does not exist or if the issue
is a requirement rather than a feature.

Pass --raw to emit a frontmatter-style header (key: value lines), a literal
'---' separator, and the verbatim issue body — suitable for agent ingestion.
Pass --raw --verbose to insert created_at and last_transitioned_at header lines.`,
		Example: `  # Default detail view
  gh agentic status feature 492

  # Agent-oriented raw frontmatter + verbatim body
  gh agentic status feature 492 --raw`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			n, err := parseIssueNumberArg(args[0])
			if err != nil {
				return err
			}
			if err := runStatusFeature(cmd.OutOrStdout(), cmd.ErrOrStderr(), n, flags, deps); err != nil {
				return renderStatusError(cmd, err)
			}
			return nil
		},
	}

	registerStatusDetailFlags(cmd, &flags)
	return cmd
}
