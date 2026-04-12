// Package cli defines the cobra command tree for gh-agentic.
// All command definitions live here; business logic is delegated to
// internal sub-packages (bootstrap, inception, sync).
package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

// ErrSilent is returned by subcommands that have already printed a
// user-friendly error message. main.go checks for it and exits non-zero
// without printing anything further.
var ErrSilent = errors.New("silent exit")

// v2Flag is the persistent flag pointer shared between root and subcommands.
// It is set to true when the user passes -v2 on the command line.
var v2FlagValue bool

// newRootCmd constructs a fresh root cobra command. Called by Execute and in
// tests so that each invocation starts from a clean command tree.
func newRootCmd(version, date string) *cobra.Command {
	// Reset the package-level flag for each new root command (important for tests).
	v2FlagValue = false

	root := &cobra.Command{
		Use:           "gh-agentic",
		Short:         "Agentic software delivery — environment management for gh",
		Long:          "gh-agentic bootstraps and manages agentic software delivery environments via the GitHub CLI.",
		Version:       version,
		SilenceErrors: true,
	}

	// Register -v2 as a persistent flag on the root command.
	root.PersistentFlags().BoolVar(&v2FlagValue, "v2", false, "use v2 command implementations")

	// v1 commands.
	root.AddCommand(newBootstrapCmd())
	root.AddCommand(newInceptionCmd())
	root.AddCommand(newSyncCmd())
	root.AddCommand(newVersionCmd(version, date))
	doctorCmd := newDoctorCmd()
	root.AddCommand(doctorCmd)
	// "verify" is a hidden alias for backwards compatibility.
	verifyAlias := *doctorCmd
	verifyAlias.Use = "verify"
	verifyAlias.Hidden = true
	root.AddCommand(&verifyAlias)

	// v2 commands — available regardless of -v2 flag, but only useful with it.
	root.AddCommand(newMountCmd())
	root.AddCommand(newInitCmd())
	root.AddCommand(newAuthCmd())
	root.AddCommand(newDoctorV2Cmd())

	return root
}



// Execute builds and runs the root command. Called by main.go.
func Execute(version, date string) error {
	return newRootCmd(version, date).Execute()
}
