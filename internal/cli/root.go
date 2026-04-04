// Package cli defines the cobra command tree for gh-agentic.
// All command definitions live here; business logic is delegated to
// internal sub-packages (bootstrap, inception, sync).
package cli

import (
	"github.com/spf13/cobra"
)

// newRootCmd constructs a fresh root cobra command. Called by Execute and in
// tests so that each invocation starts from a clean command tree.
func newRootCmd(version string) *cobra.Command {
	root := &cobra.Command{
		Use:          "gh-agentic",
		Short:        "Agentic software delivery — environment management for gh",
		Long:         "gh-agentic bootstraps and manages agentic software delivery environments via the GitHub CLI.",
		Version:      version,
		SilenceErrors: true,
	}
	root.AddCommand(newBootstrapCmd())
	root.AddCommand(newInceptionCmd())
	root.AddCommand(newSyncCmd())
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
func Execute(version string) error {
	return newRootCmd(version).Execute()
}
