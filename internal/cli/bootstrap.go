package cli

import (
	"bufio"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
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

			cfg, err := bootstrap.RunForm(w, bootstrap.DefaultFetchOwners, bootstrap.DefaultDetectOwnerType, templateRepo)
			if errors.Is(err, bootstrap.ErrAborted) || errors.Is(err, bootstrap.ErrFederatedRequiresOrg) {
				fmt.Fprintln(w, ui.Muted.Render("Aborted."))
				return nil
			}
			if err != nil {
				return err
			}

			// Detect owner type and capture it in the config for step functions.
			ownerType, detectErr := bootstrap.DefaultDetectOwnerType(cfg.Owner)
			if detectErr != nil {
				fmt.Fprintln(w, "  "+ui.RenderWarning("Could not detect owner type: "+detectErr.Error()))
				fmt.Fprintln(w, "  "+ui.Muted.Render("Defaulting to personal account — org-only features will be skipped."))
				cfg.OwnerType = bootstrap.OwnerTypeUser
			} else {
				cfg.OwnerType = ownerType
			}

			// Populate agent user fields from CLI flags.
			cfg.AgentUser = agentUser
			cfg.AgentUserScope = agentUserScope

			// Resolve agent user interactively if flags not fully provided.
			textPrompt := func(prompt string) (string, error) {
				fmt.Fprintf(w, "  %s: ", prompt)
				scanner := bufio.NewScanner(cmd.InOrStdin())
				if scanner.Scan() {
					return strings.TrimSpace(scanner.Text()), nil
				}
				return "", scanner.Err()
			}
			if err := bootstrap.ResolveAgentUser(w, &cfg, bootstrap.DefaultRunCommand, textPrompt); err != nil {
				return fmt.Errorf("resolving agent user: %w", err)
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
				bootstrap.DefaultLaunchGoose,
				bootstrap.DefaultSpinner,
				sync.DefaultFetchRelease,
			); err != nil {
				return err
			}

			clonePath := filepath.Join(workDir, cfg.ProjectName)
			return PromptGhNotify(w, runtime.GOOS, clonePath, bootstrap.DefaultRunCommand, confirm)
		},
	}

	cmd.Flags().StringVar(&agentUser, "agent-user", "", "agent GitHub username (optional — prompted if not provided)")
	cmd.Flags().StringVar(&agentUserScope, "agent-user-scope", "", "AGENT_USER variable scope: org or repo (optional — prompted if not provided)")
	cmd.Flags().StringVar(&templateRepo, "template", "", "template repo owner/name (default: "+bootstrap.DefaultTemplateRepo+")")
	return cmd
}
