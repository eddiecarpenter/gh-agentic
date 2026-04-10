package cli

import (
	"bufio"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
	"github.com/eddiecarpenter/gh-agentic/internal/sync"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// newBootstrapCmd constructs the `gh agentic bootstrap` subcommand.
func newBootstrapCmd() *cobra.Command {
	var agentUser string
	var agentUserScope string
	var templateRepo string

	cmd := &cobra.Command{
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

			cfg, err := bootstrap.RunForm(w, bootstrap.DefaultFetchOwners, bootstrap.DefaultDetectOwnerType, bootstrap.DefaultFetchRepos, bootstrap.DefaultCheckRepoExists, templateRepo)
			if errors.Is(err, bootstrap.ErrAborted) || errors.Is(err, bootstrap.ErrFederatedRequiresOrg) {
				fmt.Fprintln(w, ui.Muted.Render("Aborted."))
				return nil
			}
			if err != nil {
				return err
			}

			// Apply CLI flag overrides if provided — they take precedence over form values.
			if agentUser != "" {
				cfg.AgentUser = agentUser
			}
			if agentUserScope != "" {
				cfg.AgentUserScope = agentUserScope
			}

			workDir := bootstrap.DefaultWorkDirOrHome()

			graphqlDo, err := bootstrap.DefaultGraphQLDo()
			if err != nil {
				return fmt.Errorf("initialising GitHub GraphQL client: %w", err)
			}

			if err := bootstrap.RunSteps(
				w,
				cfg,
				workDir,
				bootstrap.DefaultRunCommand,
				graphqlDo,
				bootstrap.DefaultSpinner,
				sync.DefaultFetchRelease,
			); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&agentUser, "agent-user", "", "agent GitHub username (optional — prompted if not provided)")
	cmd.Flags().StringVar(&agentUserScope, "agent-user-scope", "", "AGENT_USER variable scope: org or repo (optional — prompted if not provided)")
	cmd.Flags().StringVar(&templateRepo, "template", "", "template repo owner/name (default: "+bootstrap.DefaultTemplateRepo+")")
	return cmd
}
