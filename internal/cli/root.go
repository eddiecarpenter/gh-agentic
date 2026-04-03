// Package cli defines the cobra command tree for gh-agentic.
// All command definitions live here; business logic is delegated to
// internal sub-packages (bootstrap, inception, sync).
package cli

import (
	"github.com/spf13/cobra"
)

// Version is the current release version of gh-agentic.
const Version = "0.1.0"

// newRootCmd constructs a fresh root cobra command. Called by Execute and in
// tests so that each invocation starts from a clean command tree.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "gh-agentic",
		Short:   "Agentic development framework — environment management for gh",
		Long:    "gh-agentic bootstraps and manages agentic development environments via the GitHub CLI.",
		Version: Version,
	}
	root.AddCommand(newBootstrapCmd())
	return root
}

// Execute builds and runs the root command. Called by main.go.
func Execute() error {
	return newRootCmd().Execute()
}
