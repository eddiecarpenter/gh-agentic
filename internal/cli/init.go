package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	ghAPI "github.com/cli/go-gh/v2/pkg/api"
	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/auth"
	"github.com/eddiecarpenter/gh-agentic/internal/githubapp"
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
	var skipAppInstall bool

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

			// Refuse on the framework source. `init` scaffolds CLAUDE.md,
			// AGENTS.md, and the wrapper workflows — running on gh-agentic
			// would overwrite committed source files.
			initRoot, ierr := os.Getwd()
			if ierr != nil {
				return fmt.Errorf("resolving working directory: %w", ierr)
			}
			if err := refuseIfFrameworkSource(cmd, initRoot, "init"); err != nil {
				return err
			}

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
				// Single path: collect config (topology already known, project
				// created automatically by project.InitRepo — no project ID
				// prompt), then delegate to project.InitRepo(InitModeSingle)
				// which calls project.Create to build the board and mount the
				// framework.
				initCfg, err := initpkg.CollectConfigInteractive(w, deps.RepoFullName, initpkg.FormDeps{
					RunForm:         initpkg.DefaultFormRun,
					RunCommand:      auth.DefaultRunCommand,
					DetectOwnerType: defaultDetectOwnerType,
					FetchReleases:   mount.DefaultFetchReleases,
					Topology:        "single",
				})
				if err != nil {
					return fmt.Errorf("configuration: %w", err)
				}

				// Run the App-install guidance step before identity writes.
				if installer := buildAppInstaller(cmd, skipAppInstall); installer != nil {
					if err := installer(w, initCfg); err != nil {
						return fmt.Errorf("App install check: %w", err)
					}
				}

				if err := project.InitRepo(w, deps, project.InitRepoConfig{
					Mode:    project.InitModeSingle,
					InitCfg: initCfg,
				}); err != nil {
					return err
				}
				tryUploadClaudeCredentials(w, deps, initCfg.OwnerType)
				return nil
			}

			// Federated path: list federated projects, user picks one.
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

			// Collect configuration — topology is federated (already captured),
			// project ID comes from the list picker above (not a form field).
			initCfg, err := initpkg.CollectConfigInteractive(w, deps.RepoFullName, initpkg.FormDeps{
				RunForm:         initpkg.DefaultFormRun,
				RunCommand:      auth.DefaultRunCommand,
				DetectOwnerType: defaultDetectOwnerType,
				FetchReleases:   mount.DefaultFetchReleases,
				Topology:        "federated",
			})
			if err != nil {
				return fmt.Errorf("configuration: %w", err)
			}

			// Wire the production Confirm for the org-visibility gate in
			// initpkg.ConfigureRepo. Tests inject their own ConfirmFunc.
			initCfg.Confirm = initpkg.HuhConfirm

			// Run the App-install guidance step before identity writes.
			if installer := buildAppInstaller(cmd, skipAppInstall); installer != nil {
				if err := installer(w, initCfg); err != nil {
					return fmt.Errorf("App install check: %w", err)
				}
			}

			if err := project.InitRepo(w, deps, project.InitRepoConfig{
				Mode:      project.InitModeFederated,
				ProjectID: projectID,
				InitCfg:   initCfg,
			}); err != nil {
				return err
			}
			tryUploadClaudeCredentials(w, deps, initCfg.OwnerType)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing configuration")
	cmd.Flags().BoolVar(&skipAppInstall, "skip-app-install", false, "skip the agentic GitHub App install-state check and install guidance")
	return cmd
}

// tryUploadClaudeCredentials reads Claude credentials from the local machine
// and uploads them as the CLAUDE_CREDENTIALS_JSON secret, using the same
// mechanism as 'gh agentic auth refresh'. If credentials are not found locally
// (e.g. the user has never logged in to Claude Code on this machine), a guidance
// message is printed and init continues — the user can upload credentials later
// with 'gh agentic auth refresh'.
func tryUploadClaudeCredentials(w io.Writer, deps project.Deps, ownerType string) {
	fmt.Fprintf(w, "\n")
	authDeps := auth.Deps{
		Run:             auth.DefaultRunCommand,
		ReadCredentials: auth.ReadClaudeCredentialsDefault,
		RepoFullName:    deps.RepoFullName,
		Owner:           deps.Owner,
		RepoName:        deps.RepoName,
		OwnerType:       ownerType,
	}
	if err := auth.Refresh(w, authDeps); err != nil {
		fmt.Fprintf(w, "  %s  Claude credentials not found locally — run 'gh agentic auth refresh' to upload them\n", ui.StatusWarning.Render("⚠"))
	}
}

// buildAppInstaller returns the EnsureAppInstalled hook wired to the
// production githubapp.EnsureInstalled flow, or nil when the caller asks
// for the step to be skipped. A nil return matches the existing no-op
// path inside initpkg.Run so the two callers agree on semantics.
//
// The hook derives the target from the InitConfig's topology — a Single
// topology install runs against the repo-level endpoint; a Federated
// topology install runs against the org-level endpoint because every
// domain repo under the same org benefits from a single install.
func buildAppInstaller(cmd *cobra.Command, skip bool) initpkg.EnsureAppInstalledFunc {
	if skip {
		return nil
	}
	checker, err := githubapp.NewChecker(githubapp.DefaultAppSlug)
	if err != nil {
		// Surface a warning but do not break init — the App flow is a
		// convenience, not a hard requirement.
		fmt.Fprintf(cmd.OutOrStdout(), "  %s  Could not initialise App install checker: %v — skipping App install guidance\n", ui.StatusWarning.Render("⚠"), err)
		return nil
	}
	flow := &githubapp.Flow{
		Checker:       checker,
		Slug:          githubapp.DefaultAppSlug,
		OpenURL:       ui.OpenURL,
		Confirm:       huhConfirmWithTitle,
		IsCI:          ui.IsCI,
		IsInteractive: ui.IsInteractive,
		WaitEnter:     githubapp.WaitEnterFromReader,
	}
	return func(w io.Writer, cfg *initpkg.InitConfig) error {
		target := targetFromConfig(cfg)
		if target.Owner == "" {
			return nil
		}
		_, err := githubapp.EnsureInstalled(context.Background(), w, os.Stdin, flow, target)
		return err
	}
}

// targetFromConfig chooses the install-flow target based on the collected
// wizard configuration. Federated topology prefers the org-level endpoint
// so the App covers every domain repo under the same org; single topology
// uses the repo-level endpoint (the only repo that matters is the current
// one).
func targetFromConfig(cfg *initpkg.InitConfig) githubapp.Target {
	if cfg == nil {
		return githubapp.Target{}
	}
	if strings.EqualFold(cfg.Topology, "federated") {
		return githubapp.Target{Type: githubapp.TargetOrg, Owner: cfg.Owner}
	}
	return githubapp.Target{Type: githubapp.TargetRepo, Owner: cfg.Owner, Repo: cfg.RepoName}
}

// huhConfirmWithTitle is the production Confirm for the App install
// prompt. It delegates to huh's confirm form — tests inject their own
// Confirm via the Flow struct and never hit this.
func huhConfirmWithTitle(title, description string) (bool, error) {
	var confirmed bool
	form := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title(title).
			Description(description).
			Value(&confirmed),
	))
	if err := form.Run(); err != nil {
		return false, err
	}
	return confirmed, nil
}
