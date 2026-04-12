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

// bootstrapRequiredFlags lists flags required in non-interactive mode.
var bootstrapRequiredFlags = []string{
	"topology", "owner", "project-name", "agent-user",
	"stack", "runner", "provider", "model", "pat",
}

// newBootstrapCmd constructs the `gh agentic bootstrap` subcommand.
func newBootstrapCmd() *cobra.Command {
	var (
		agentUser      string
		agentUserScope string
		templateRepo   string
		nonInteractive bool
		topology       string
		owner          string
		repo           string
		projectName    string
		description    string
		stacks         []string
		antora         bool
		runner         string
		provider       string
		model          string
		pat            string
	)

	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Bootstrap a new agentic environment (Phase 0a)",
		Long:  "Creates and configures a new agentic development environment from scratch.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Block in v2 mode.
			if err := checkV2Guard("bootstrap", &v2FlagValue); err != nil {
				return err
			}

			w := cmd.OutOrStdout()

			// Non-interactive: validate flags and field formats before running preflight.
			// This allows tests and CI to catch missing/invalid flags without needing
			// a real gh auth session.
			if nonInteractive {
				var missing []string
				if topology == "" {
					missing = append(missing, "--topology")
				}
				if owner == "" {
					missing = append(missing, "--owner")
				}
				if projectName == "" {
					missing = append(missing, "--project-name")
				}
				if agentUser == "" {
					missing = append(missing, "--agent-user")
				}
				if len(stacks) == 0 {
					missing = append(missing, "--stack")
				}
				if runner == "" {
					missing = append(missing, "--runner")
				}
				if provider == "" {
					missing = append(missing, "--provider")
				}
				if model == "" {
					missing = append(missing, "--model")
				}
				if pat == "" {
					missing = append(missing, "--pat")
				}
				if len(missing) > 0 {
					return fmt.Errorf("--non-interactive requires %s", strings.Join(missing, ", "))
				}

				if err := bootstrap.ValidateProjectName(projectName); err != nil {
					return fmt.Errorf("invalid --project-name: %w", err)
				}
				if err := bootstrap.ValidateStackSelection(stacks); err != nil {
					return fmt.Errorf("invalid --stack: %w", err)
				}
			}

			confirm := func(prompt string) (bool, error) {
				if nonInteractive {
					return true, nil
				}
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

			var cfg bootstrap.BootstrapConfig

			if nonInteractive {
				// Detect owner type.
				ownerType, err := bootstrap.DefaultDetectOwnerType(owner)
				if err != nil {
					return fmt.Errorf("detecting owner type: %w", err)
				}

				// Validate topology/owner combination.
				if err := bootstrap.ValidateTopologyOwner(topology, ownerType); err != nil {
					return err
				}

				// Determine repo mode from --repo flag.
				existingRepo := false
				if repo != "" && repo != "new" {
					existingRepo = true
					projectName = repo
				}

				// Build config from flags.
				tmpl := templateRepo
				if tmpl == "" {
					tmpl = bootstrap.DefaultTemplateRepo
				}

				cfg = bootstrap.BootstrapConfig{
					TemplateRepo:   tmpl,
					Topology:       topology,
					Owner:          owner,
					ProjectName:    projectName,
					Description:    description,
					Stacks:         stacks,
					ExistingRepo:   existingRepo,
					Antora:         antora,
					OwnerType:      ownerType,
					AgentUser:      agentUser,
					AgentUserScope: agentUserScope,
					RunnerLabel:    runner,
					GooseProvider:  provider,
					GooseModel:     model,
					GooseAgentPAT:  pat,
				}

				// Default agent user scope if not specified.
				if cfg.AgentUserScope == "" {
					if ownerType == bootstrap.OwnerTypeOrg {
						cfg.AgentUserScope = bootstrap.AgentUserScopeOrg
					} else {
						cfg.AgentUserScope = bootstrap.AgentUserScopeRepo
					}
				}
			} else {
				// Interactive path — unchanged from before.
				var err error
				cfg, err = bootstrap.RunForm(w, bootstrap.DefaultFetchOwners, bootstrap.DefaultDetectOwnerType, bootstrap.DefaultFetchRepos, bootstrap.DefaultCheckRepoExists, templateRepo, bootstrap.DefaultFormRun)
				if errors.Is(err, bootstrap.ErrAborted) || errors.Is(err, bootstrap.ErrFederatedRequiresOrg) {
					fmt.Fprintln(w, ui.Muted.Render("Aborted."))
					return nil
				}
				if err != nil {
					return err
				}

				// Apply CLI flag overrides if provided.
				if agentUser != "" {
					cfg.AgentUser = agentUser
				}
				if agentUserScope != "" {
					cfg.AgentUserScope = agentUserScope
				}
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

	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "run without prompting — all required flags must be provided")
	cmd.Flags().StringVar(&topology, "topology", "", "project topology: Single or Federated")
	cmd.Flags().StringVar(&owner, "owner", "", "GitHub account or organisation login")
	cmd.Flags().StringVar(&repo, "repo", "", "repo name or 'new' to create a new repo")
	cmd.Flags().StringVar(&projectName, "project-name", "", "project name (lowercase with hyphens only)")
	cmd.Flags().StringVar(&description, "description", "", "short project description")
	cmd.Flags().StringArrayVar(&stacks, "stack", nil, "technology stack (repeatable, e.g. --stack Go --stack Rust)")
	cmd.Flags().StringVar(&agentUser, "agent-user", "", "agent GitHub username")
	cmd.Flags().StringVar(&agentUserScope, "agent-user-scope", "", "AGENT_USER variable scope: org or repo")
	cmd.Flags().BoolVar(&antora, "antora", false, "enable Antora documentation site")
	cmd.Flags().StringVar(&runner, "runner", "", "GitHub Actions runner label")
	cmd.Flags().StringVar(&provider, "provider", "", "LLM provider for the agent")
	cmd.Flags().StringVar(&model, "model", "", "LLM model for the agent")
	cmd.Flags().StringVar(&pat, "pat", "", "GitHub Personal Access Token for the agent user")
	cmd.Flags().StringVar(&templateRepo, "template", "", "template repo owner/name (default: "+bootstrap.DefaultTemplateRepo+")")
	return cmd
}
