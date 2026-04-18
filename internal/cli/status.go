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
All sub-commands accept --json for a stable structured payload suitable for
scripting, dashboards, or downstream skills.

By default the view aggregates across every active repo in the federation
(control plane and domain repos). Use --this-repo on any sub-command to narrow
to the current repository (detected via 'git remote get-url origin').

%s lists open requirements with stage, supports --kanban for a stage-grouped
view and --include-done to include completed items.

%s shows detail for a single requirement — number, title, stage, dates, body,
linked features, and blocked annotation where applicable.

%s lists open features with stage, supports --kanban for a stage-grouped view
and --include-done to include completed items.

%s shows detail for a single feature — number, title, stage, dates, body,
parent requirement, tasks, branch state, PR state, and blocked annotation.

Run 'gh agentic status <sub-command> --help' for detailed usage.`,
			b("requirements"), b("requirement <N>"), b("features"), b("feature <N>")),
		Example: `  # List open requirements with stage
  gh agentic status requirements

  # Detail for one requirement, as JSON
  gh agentic status requirement 457 --json

  # Kanban of open features
  gh agentic status features --kanban

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

	return cmd
}

// statusListFlags captures the shared set of flags used by list sub-commands.
// Declared once so every list command registers the same shape; downstream
// tasks wire the actual behaviour behind each flag.
type statusListFlags struct {
	json        bool
	kanban      bool
	horizontal  bool
	thisRepo    bool
	includeDone bool
}

// registerStatusListFlags declares the shared flag set for list sub-commands.
// Every downstream flag is declared here so the scaffold task exposes the full
// surface area — the implementations land in later tasks.
func registerStatusListFlags(cmd *cobra.Command, f *statusListFlags) {
	cmd.Flags().BoolVar(&f.json, "json", false, "emit a stable structured JSON payload and suppress human output")
	cmd.Flags().BoolVar(&f.kanban, "kanban", false, "render a vertical stage-grouped kanban view")
	cmd.Flags().BoolVar(&f.horizontal, "horizontal", false, "render the kanban horizontally (requires --kanban; wide terminals only)")
	cmd.Flags().BoolVar(&f.thisRepo, "this-repo", false, "narrow the view to the current repository only")
	cmd.Flags().BoolVar(&f.includeDone, "include-done", false, "include items in the 'done' stage")
}

// statusDetailFlags captures the shared flag set for detail sub-commands.
type statusDetailFlags struct {
	json bool
}

// registerStatusDetailFlags declares the shared flag set for detail sub-commands.
func registerStatusDetailFlags(cmd *cobra.Command, f *statusDetailFlags) {
	cmd.Flags().BoolVar(&f.json, "json", false, "emit a stable structured JSON payload and suppress human output")
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

Pass --kanban for a stage-grouped view; see --horizontal for wide terminals.
Pass --json to emit a stable structured payload for machine consumption.
Pass --include-done to include completed requirements; by default only open
items are listed.`,
		Example: `  # Default list view
  gh agentic status requirements

  # Vertical kanban
  gh agentic status requirements --kanban

  # JSON for scripting
  gh agentic status requirements --json

  # Include closed requirements
  gh agentic status requirements --include-done`,
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatusRequirements(cmd.OutOrStdout(), flags, deps)
		},
	}

	registerStatusListFlags(cmd, &flags)
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

Pass --json to emit a stable structured object for machine consumption.`,
		Example: `  # Default detail view
  gh agentic status requirement 466

  # JSON for scripting
  gh agentic status requirement 466 --json`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			n, err := parseIssueNumberArg(args[0])
			if err != nil {
				return err
			}
			return runStatusRequirement(cmd.OutOrStdout(), n, flags, deps)
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

Pass --kanban for a stage-grouped view; see --horizontal for wide terminals.
Pass --json to emit a stable structured payload for machine consumption.
Pass --include-done to include completed features; by default only open items
are listed.`,
		Example: `  # Default list view
  gh agentic status features

  # Vertical kanban
  gh agentic status features --kanban

  # Horizontal kanban (wide terminals)
  gh agentic status features --kanban --horizontal

  # JSON for scripting
  gh agentic status features --json

  # Narrow to the current repo
  gh agentic status features --this-repo`,
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatusFeatures(cmd.OutOrStdout(), flags, deps)
		},
	}

	registerStatusListFlags(cmd, &flags)
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

Pass --json to emit a stable structured object for machine consumption.`,
		Example: `  # Default detail view
  gh agentic status feature 492

  # JSON for scripting
  gh agentic status feature 492 --json`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			n, err := parseIssueNumberArg(args[0])
			if err != nil {
				return err
			}
			return runStatusFeature(cmd.OutOrStdout(), n, flags, deps)
		},
	}

	registerStatusDetailFlags(cmd, &flags)
	return cmd
}
