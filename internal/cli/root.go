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

// newRootCmd constructs a fresh root cobra command. Called by Execute and in
// tests so that each invocation starts from a clean command tree.
func newRootCmd(version, date string) *cobra.Command {
	root := &cobra.Command{
		Use:           "gh-agentic",
		Short:         "Agentic software delivery — environment management for gh",
		Long:          "gh-agentic bootstraps and manages agentic software delivery environments via the GitHub CLI.",
		Version:       version,
		SilenceErrors: true,
	}
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
	return root
}

// Execute builds and runs the root command. Called by main.go.
func Execute(version, date string) error {
	return newRootCmd(version, date).Execute()
}
