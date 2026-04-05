package cli

import (
	"bufio"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
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

			cfg, err := bootstrap.RunForm(w, bootstrap.DefaultFetchOwners, bootstrap.DefaultDetectOwnerType)
			if errors.Is(err, bootstrap.ErrAborted) || errors.Is(err, bootstrap.ErrFederatedRequiresOrg) {
				fmt.Fprintln(w, ui.Muted.Render("Aborted."))
				return nil
			}
			if err != nil {
				return err
			}

			workDir := bootstrap.DefaultWorkDirOrHome()

			graphqlDo, err := bootstrap.DefaultGraphQLDo()
			if err != nil {
				return fmt.Errorf("initialising GitHub GraphQL client: %w", err)
			}

			return bootstrap.RunSteps(
				w,
				cfg,
				workDir,
				bootstrap.DefaultRunCommand,
				graphqlDo,
				bootstrap.DefaultLaunchGoose,
				bootstrap.DefaultSpinner,
			)
		},
	}
}
