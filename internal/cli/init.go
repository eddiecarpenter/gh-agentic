package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/huh"
	ghAPI "github.com/cli/go-gh/v2/pkg/api"
	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/auth"
	initpkg "github.com/eddiecarpenter/gh-agentic/internal/init"
	"github.com/eddiecarpenter/gh-agentic/internal/mount"
	"github.com/eddiecarpenter/gh-agentic/internal/project"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// defaultDetectOwnerType detects whether a GitHub owner is a user or org via the API.
func defaultDetectOwnerType(owner string) (string, error) {
	client, err := ghAPI.DefaultRESTClient()
	if err != nil {
		return "", fmt.Errorf("creating GitHub API client: %w", err)
	}

	var resp struct {
		Type string `json:"type"`
	}
	if err := client.Get(fmt.Sprintf("users/%s", owner), &resp); err != nil {
		return "", fmt.Errorf("detecting owner type for %q: %w", owner, err)
	}
	return resp.Type, nil
}

// newInitCmd constructs the top-level `gh agentic init` subcommand.
func newInitCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "First-time setup — configure project and pipeline for this repo",
		Long: `Interactive wizard for first-time setup of an agentic delivery environment.

Covers both sides of setup in one flow:
  • Project side — create or join an agentic project, set topology, mount the framework
  • Pipeline side — configure runtime variables, secrets, and wrapper workflows
    needed for the agent pipeline to execute

Single topology: creates the GitHub Project board, mounts the framework, and
establishes this repo as the control plane. Best for one-repo setups where the
control plane and the code repo are the same.

Federated topology: joins an existing federated project as a domain repo. The
control plane must already exist — run 'gh agentic project create' on that repo
first.

Blocked if this repo is already initialised — use 'gh agentic project switch' to
change membership, or --force to re-run setup from scratch.`,
		Example: `  gh agentic init
  gh agentic init --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()

			deps, err := resolveProjectDeps()
			if err != nil {
				return err
			}

			// Guard: already initialised? (bypass with --force). Route the
			// existence check through project.Resolve so "am I already
			// affiliated?" flows through the same single source of truth
			// the rest of the CLI uses.
			if !force {
				ctx, _ := project.Resolve(deps)
				if ctx != nil && ctx.ProjectID != "" {
					name := project.ProjectDisplayName(deps, ctx.ProjectID)
					fmt.Fprintf(w, "  %s  Repo is already part of agentic project %q\n", ui.StatusWarning.Render("⚠"), name)
					fmt.Fprintf(w, "       → Use 'gh agentic project switch' to change project membership\n")
					fmt.Fprintf(w, "       → Or re-run with --force to overwrite the current configuration\n\n")
					return ErrSilent
				}
			}

			root, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("resolving working directory: %w", err)
			}

			// Step 1: Ask single or federated.
			var topology string
			topoForm := huh.NewForm(huh.NewGroup(
				huh.NewSelect[string]().
					Title("Project topology").
					Description("How will this repo be used?").
					Options(
						huh.NewOption("Single  — this repo is the control plane (one repo does everything)", "single"),
						huh.NewOption("Federated  — this repo joins an existing federated project", "federated"),
					).
					Value(&topology),
			))
			if err := topoForm.Run(); err != nil {
				return fmt.Errorf("topology selection: %w", err)
			}

			if topology == "single" {
				// Single path: full wizard via the init package.
				initDeps := initpkg.Deps{
					Run:   auth.DefaultRunCommand,
					Clone: mount.DefaultClone,
					CollectConfig: func(w io.Writer, repo string) (*initpkg.InitConfig, error) {
						return initpkg.CollectConfigInteractive(w, repo, initpkg.FormDeps{
							RunForm:         initpkg.DefaultFormRun,
							RunCommand:      auth.DefaultRunCommand,
							DetectOwnerType: defaultDetectOwnerType,
							FetchReleases:   mount.DefaultFetchReleases,
						})
					},
				}
				if err := initpkg.Run(w, root, force, initDeps); err != nil {
					if errors.Is(err, initpkg.ErrAlreadyInitialised) {
						return ErrSilent
					}
					return err
				}
				// Single-topology init no longer writes AGENTIC_TOPOLOGY
				// automatically (task #585). The resolver infers topology
				// from the project-linked-repo graph; the variable remains
				// an optional explicit override only.
				return nil
			}

			// Federated path: list federated projects, user picks.
			fmt.Fprintf(w, "\n  Fetching federated projects for %s...\n\n", deps.Owner)
			projects, err := project.ListFederatedProjects(deps)
			if err != nil {
				return fmt.Errorf("listing federated projects: %w", err)
			}
			if len(projects) == 0 {
				return fmt.Errorf("no federated agentic projects found for %s — run 'gh agentic project create' on the control plane repo first", deps.Owner)
			}

			var projectID string
			var options []huh.Option[string]
			for _, p := range projects {
				options = append(options, huh.NewOption(p.Title, p.ID))
			}
			projectForm := huh.NewForm(huh.NewGroup(
				huh.NewSelect[string]().
					Title("Select federated project").
					Description("The federated project to join").
					Options(options...).
					Value(&projectID),
			))
			if err := projectForm.Run(); err != nil {
				return fmt.Errorf("project selection: %w", err)
			}

			// Collect configuration.
			initCfg, err := initpkg.CollectConfigInteractive(w, deps.RepoFullName, initpkg.FormDeps{
				RunForm:         initpkg.DefaultFormRun,
				RunCommand:      auth.DefaultRunCommand,
				DetectOwnerType: defaultDetectOwnerType,
				FetchReleases:   mount.DefaultFetchReleases,
			})
			if err != nil {
				return fmt.Errorf("configuration: %w", err)
			}

			// Wire the production Confirm for the org-visibility gate in
			// initpkg.ConfigureRepo. Tests inject their own ConfirmFunc.
			initCfg.Confirm = initpkg.HuhConfirm

			return project.InitRepo(w, deps, project.InitRepoConfig{
				Mode:      project.InitModeFederated,
				ProjectID: projectID,
				InitCfg:   initCfg,
			})
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing configuration")
	return cmd
}
