// Package cli defines the cobra command tree for gh-agentic.
// All command definitions live here; business logic is delegated to
// internal sub-packages.
package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// ErrSilent is returned by subcommands that have already printed a
// user-friendly error message. main.go checks for it and exits non-zero
// without printing anything further.
var ErrSilent = errors.New("silent exit")

// newRootCmd constructs a fresh root cobra command. Called by Execute and in
// tests so that each invocation starts from a clean command tree.
func newRootCmd(version, date string) *cobra.Command {
	b := ui.SectionHeading.Render

	root := &cobra.Command{
		Use:   "gh-agentic",
		Short: "Agentic software delivery — environment management for gh",
		Long: fmt.Sprintf(`gh-agentic manages agentic software delivery environments via the GitHub CLI.

An agentic project ties together a GitHub Project board, a control plane repo,
(optionally) domain repos, a shared framework version, and a GitHub Actions
pipeline that runs the agent.

%s walks you through first-time setup — creating or joining an agentic project,
mounting the framework, and configuring the pipeline in one flow.

%s verifies the full health of the environment: project membership, topology,
framework version sync, workflows, runtime variables and secrets, and agent
instruction files.

%s auto-fixes what can be fixed and reports the rest with manual remediation
steps.

%s installs or changes the framework version for this repo (also handles
first-time install and migration from the legacy gitignored mount).

%s manages ongoing project membership — create, join, switch, unlink.

%s shows the current state of this repo's agentic setup.

%s manages the Claude credentials used by the agent pipeline.

%s shows pipeline state across requirements and features, including the
pipeline sub-view that renders requirements and features together — the
first-class way to answer "where are we?" at a glance.

Run 'gh agentic <command> --help' on any command for detailed usage.`,
			b("init"), b("check"), b("repair"), b("upgrade"), b("project"), b("info"), b("auth"), b("status")),
		Version:       version,
		SilenceErrors: true,
	}

	// Disable the auto-generated completion command — tab completion is not
	// supported for gh extensions invoked via `gh agentic`, so exposing it
	// would only confuse users.
	root.CompletionOptions.DisableDefaultCmd = true

	// Allow `help` as a positional argument on any leaf command.
	root.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		for _, arg := range args {
			if arg == "help" {
				_ = cmd.Help()
				os.Exit(0)
			}
		}
	}

	root.AddCommand(newInitCmd())
	root.AddCommand(newCheckCmd())
	root.AddCommand(newRepairCmd())
	root.AddCommand(newUpgradeCmd(version))
	root.AddCommand(newProjectCmd())
	root.AddCommand(newInfoCmd(version, date))
	root.AddCommand(newAuthCmd())
	root.AddCommand(newStatusCmd())

	return root
}

// Execute builds and runs the root command. Called by main.go.
func Execute(version, date string) error {
	return newRootCmd(version, date).Execute()
}
