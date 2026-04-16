// Package cli defines the cobra command tree for gh-agentic.
// All command definitions live here; business logic is delegated to
// internal sub-packages.
package cli

import (
	"errors"
	"os"

	"github.com/spf13/cobra"
)

// ErrSilent is returned by subcommands that have already printed a
// user-friendly error message. main.go checks for it and exits non-zero
// without printing anything further.
var ErrSilent = errors.New("silent exit")

// newRootCmd constructs a fresh root cobra command. Called by Execute and in
// tests so that each invocation starts from a clean command tree.
func newRootCmd(version, date string) *cobra.Command {
	root := &cobra.Command{
		Use:           "gh-agentic",
		Short: "Agentic software delivery — environment management for gh",
		Long: `gh-agentic bootstraps and manages agentic software delivery environments via the GitHub CLI.

Run 'gh agentic <command> --help' on any command for detailed usage and examples.

Common workflows:

  First-time setup — join or establish an agentic project:
    gh agentic project init

  Switch agentic project or framework version:
    gh agentic project switch project
    gh agentic project switch version <version>

  Sync the framework to the current version:
    gh agentic mount

  Check and repair agentic project health:
    gh agentic project check
    gh agentic project repair`,
		Version:       version,
		SilenceErrors: true,
	}

	// Disable the auto-generated completion command — tab completion is not
	// supported for gh extensions invoked via `gh agentic`, so exposing it
	// would only confuse users.
	root.CompletionOptions.DisableDefaultCmd = true

	// Allow `help` as a positional argument on any leaf command (e.g. `gh agentic init help`).
	// Cobra only generates a `help` subcommand for parent commands; this makes it
	// consistent for leaf commands too.
	root.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		for _, arg := range args {
			if arg == "help" {
				_ = cmd.Help()
				os.Exit(0)
			}
		}
	}

	root.AddCommand(newInfoCmd(version, date))
	root.AddCommand(newMountCmd())
	root.AddCommand(newAuthCmd())
	root.AddCommand(newDoctorCmd())
	root.AddCommand(newProjectCmd())

	return root
}



// Execute builds and runs the root command. Called by main.go.
func Execute(version, date string) error {
	return newRootCmd(version, date).Execute()
}
