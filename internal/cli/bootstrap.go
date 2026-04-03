package cli

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
)

// newBootstrapCmd constructs the `gh agentic bootstrap` subcommand.
func newBootstrapCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "bootstrap",
		Short: "Bootstrap a new agentic environment (Phase 0a)",
		Long:  "Creates and configures a new agentic development environment from scratch.",
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()

			confirm := func(prompt string) (bool, error) {
				fmt.Fprintf(w, "  %s [Yes/No]: ", prompt)
				scanner := bufio.NewScanner(cmd.InOrStdin())
				if scanner.Scan() {
					answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
					return answer == "yes" || answer == "y", nil
				}
				return false, scanner.Err()
			}

			if err := bootstrap.RunPreflight(w, bootstrap.DefaultLookPath, bootstrap.DefaultRunCommand, confirm); err != nil {
				return err
			}

			fmt.Fprintln(w, "bootstrap: preflight passed — form not yet implemented")
			return nil
		},
	}
}
