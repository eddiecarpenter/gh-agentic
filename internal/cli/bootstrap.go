package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newBootstrapCmd constructs the `gh agentic bootstrap` subcommand.
// Full implementation is delivered by feature #7; this is a stub only.
func newBootstrapCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "bootstrap",
		Short: "Bootstrap a new agentic environment (Phase 0a)",
		Long:  "Creates and configures a new agentic development environment from scratch.",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "bootstrap: not yet implemented")
			return nil
		},
	}
}
