package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/auth"
	"github.com/eddiecarpenter/gh-agentic/internal/initv2"
	"github.com/eddiecarpenter/gh-agentic/internal/mount"
	"github.com/eddiecarpenter/gh-agentic/internal/project"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// newProjectCmd constructs the `gh agentic project` command group.
func newProjectCmd() *cobra.Command {
	b := ui.SectionHeading.Render
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage this repo's agentic project membership and health",
		Long: fmt.Sprintf(`Set up, inspect, and maintain this repo's role in an agentic project.

An agentic project ties together a GitHub Project board, a control plane repo,
domain repos, and a shared framework version.

%s is the starting point — it walks you through creating or joining a project
and leaves the repo fully configured in one step.

%s validates the full health of this repo's project membership, topology,
framework mount, and version sync — giving you a clear pass/warn/fail picture.

%s reads the check results and automatically fixes what it can, prompting
only when it needs input it can't derive on its own.

%s is for scripted or advanced setups where you need to create the GitHub
Project board and establish the control plane without the interactive wizard.

%s lets you move this repo to a different project or change the framework
version after the repo is already initialised.

%s removes this repo from its project without touching the board or the
framework — useful when migrating to a new control plane.`,
			b("init"), b("check"), b("repair"), b("create"), b("switch"), b("unlink")),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newProjectCheckCmd())
	cmd.AddCommand(newProjectCreateCmd())
	cmd.AddCommand(newProjectJoinCmd())
	cmd.AddCommand(newProjectUnlinkCmd())
	cmd.AddCommand(newProjectRepairCmd())
	cmd.AddCommand(newProjectSwitchCmd())
	cmd.AddCommand(newProjectInitCmd())

	return cmd
}

// resolveProjectDeps resolves the current repo context and builds project.Deps.
func resolveProjectDeps() (project.Deps, error) {
	currentRepo, err := repository.Current()
	if err != nil {
		return project.Deps{}, fmt.Errorf("resolving repo context: %w", err)
	}

	root, err := os.Getwd()
	if err != nil {
		return project.Deps{}, fmt.Errorf("resolving working directory: %w", err)
	}

	return project.DefaultDeps(currentRepo.Owner, currentRepo.Name, root), nil
}

// newProjectCheckCmd constructs the `gh agentic project check` subcommand.
func newProjectCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Verify this repo's project membership and framework health",
		Long: `Run all health checks and report results with remediation hints.

Verifies: project membership, accessibility, topology, framework mount (.ai/),
framework version sync, and required project views.

Run 'gh agentic project repair' to automatically fix any reported issues.`,
		Example:      `  gh agentic project check`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			deps, err := resolveProjectDeps()
			if err != nil {
				return err
			}

			w := cmd.OutOrStdout()
			var report *project.CheckReport

			_ = ui.RunWithDynamicSpinner(w, "Checking agentic project ID...", func(setLabel func(string)) error {
				report = project.RunChecksWithProgress(deps, setLabel)
				return nil
			})

			ok := project.PrintReport(w, report)
			if !ok {
				return ErrSilent
			}
			return nil
		},
	}
}

// newProjectCreateCmd constructs the `gh agentic project create` subcommand.
// It presents an interactive form to collect the project title and framework version,
// then delegates to project.Create.
func newProjectCreateCmd() *cobra.Command {
	var version string
	var interactive bool

	cmd := &cobra.Command{
		Use:   "create [title]",
		Short: "Create a new project and establish this repo as the control plane",
		Long: `Create a GitHub Project board and establish this repo as the federated control plane.

Sets up the board, scaffolds views, mounts the framework, and configures this
repo so that domain repos can discover and sync to the correct framework version.

Use 'project init' for an interactive first-time wizard that covers both single
and federated topologies. Use this command when scripting or when the control
plane repo already exists and just needs the project created.

The framework version defaults to the latest release; use --version to pin one.`,
		Example: `  # Create using the latest framework version
  gh agentic project create "My Agentic Project"

  # Create with a specific framework version
  gh agentic project create "My Agentic Project" --version v2.1.0

  # Interactive — select title and version via form
  gh agentic project create --interactive`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// No args and no flags — show help.
			if !interactive && len(args) == 0 {
				return cmd.Help()
			}

			deps, err := resolveProjectDeps()
			if err != nil {
				return err
			}

			// Fetch available releases (needed for both paths).
			releases, err := deps.FetchReleases(mount.FrameworkRepo)
			if err != nil {
				return fmt.Errorf("fetching framework releases: %w", err)
			}
			if len(releases) == 0 {
				return fmt.Errorf("no framework releases available")
			}

			var cfg project.CreateConfig

			if interactive {
				// Interactive form — title input + version selector.
				versionOptions := make([]huh.Option[string], 0, len(releases))
				for _, r := range releases {
					versionOptions = append(versionOptions, huh.NewOption(r.TagName, r.TagName))
				}

				form := huh.NewForm(
					huh.NewGroup(
						huh.NewInput().
							Title("Project title").
							Description("Name for the new GitHub ProjectV2").
							Value(&cfg.Title),
						huh.NewSelect[string]().
							Title("Framework version").
							Description("Version of the AI-Native Delivery Framework to mount").
							Options(versionOptions...).
							Value(&cfg.Version),
					),
				)
				if err := form.Run(); err != nil {
					return fmt.Errorf("form error: %w", err)
				}
				if cfg.Title == "" {
					return fmt.Errorf("project title is required")
				}
			} else {
				// Non-interactive — title from arg, version from flag or latest.
				cfg.Title = args[0]
				if version != "" {
					cfg.Version = version
				} else {
					cfg.Version = releases[0].TagName
				}
			}

			return project.Create(cmd.OutOrStdout(), deps, cfg)
		},
	}

	cmd.Flags().StringVar(&version, "version", "", "framework version to mount (default: latest)")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "select title and version via interactive form")

	return cmd
}

// resolveProjectID resolves a project name or raw node ID to a node ID.
// If arg starts with "PVT_" it is used directly; otherwise the owner's projects
// are fetched and matched by name (case-insensitive).
func resolveProjectID(deps project.Deps, arg string) (string, error) {
	if strings.HasPrefix(arg, "PVT_") {
		return arg, nil
	}

	ownerType, err := deps.DetectOwnerType(deps.Owner)
	if err != nil {
		ownerType = "User"
	}

	projects, err := deps.FetchProjectsForOwner(deps.Owner, ownerType)
	if err != nil {
		return "", fmt.Errorf("fetching projects: %w", err)
	}

	argLower := strings.ToLower(arg)
	for _, p := range projects {
		if strings.ToLower(p.Title) == argLower {
			return p.ID, nil
		}
	}

	return "", fmt.Errorf("no project named %q found for %s", arg, deps.Owner)
}

// newProjectJoinCmd constructs the `gh agentic project join` subcommand.
func newProjectJoinCmd() *cobra.Command {
	var list, interactive bool

	cmd := &cobra.Command{
		Use:   "join [project-name]",
		Short: "Join an existing project as a domain repo",
		Long: `Bring this repo into an existing project as a domain repo.

For interactive first-time setup, prefer 'project init' — it guides you through
topology selection and runs this step automatically.

Use this command directly when scripting or re-joining after an unlink. The
project name is matched case-insensitively; quote names that contain spaces.`,
		Example: `  # List available projects
  gh agentic project join --list

  # Interactive — select from the list
  gh agentic project join --interactive

  # Direct — provide the project name (quote names with spaces)
  gh agentic project join "My Agentic Project"`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// No flags and no args — show help.
			if !list && !interactive && len(args) == 0 {
				return cmd.Help()
			}

			deps, err := resolveProjectDeps()
			if err != nil {
				return err
			}

			// --list: print available projects and exit.
			if list {
				return printAvailableProjects(cmd, deps)
			}

			projectID := ""
			if len(args) == 1 {
				projectID, err = resolveProjectID(deps, args[0])
				if err != nil {
					return err
				}
			} else {
				// --interactive: show pre-guard warning then project picker.
				preGuard, err := project.EvalPreJoinWarning(deps)
				if err != nil {
					return fmt.Errorf("pre-join check: %w", err)
				}
				switch preGuard.Guard {
				case project.JoinGuardBlocked:
					return fmt.Errorf("%s", preGuard.Message)
				case project.JoinGuardWarnConfirm:
					fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s\n\n", ui.StatusWarning.Render("⚠"), preGuard.Message)
					ok, err := deps.Confirm("Proceed?")
					if err != nil {
						return fmt.Errorf("confirmation failed: %w", err)
					}
					if !ok {
						return fmt.Errorf("aborted by user")
					}
				}

				ownerType, err := deps.DetectOwnerType(deps.Owner)
				if err != nil {
					ownerType = "User"
				}

				var projects []project.ProjectInfo
				var fetchErr error
				_ = ui.RunWithSpinner(cmd.OutOrStdout(), "Fetching agentic projects...", func() error {
					projects, fetchErr = deps.FetchProjectsForOwner(deps.Owner, ownerType)
					return fetchErr
				})
				if fetchErr != nil {
					return fmt.Errorf("fetching projects: %w", fetchErr)
				}
				if len(projects) == 0 {
					return fmt.Errorf("no projects found for %s", deps.Owner)
				}

				currentID, _ := deps.GetRepoVariable(deps.Owner, deps.RepoName, project.ProjectVarName)

				var options []huh.Option[string]
				for _, p := range projects {
					label := p.Title
					if p.ID == currentID {
						label += " (current)"
					}
					options = append(options, huh.NewOption(label, p.ID))
				}

				form := huh.NewForm(
					huh.NewGroup(
						huh.NewSelect[string]().
							Title("Select project").
							Description("GitHub ProjectV2 to affiliate this repository with").
							Options(options...).
							Value(&projectID),
					),
				)
				if err := form.Run(); err != nil {
					return fmt.Errorf("form error: %w", err)
				}
			}

			if projectID == "" {
				return fmt.Errorf("project name is required")
			}

			if len(args) == 1 {
				return project.Join(cmd.OutOrStdout(), deps, projectID)
			}
			return project.JoinConfirmed(cmd.OutOrStdout(), deps, projectID)
		},
	}

	cmd.Flags().BoolVarP(&list, "list", "l", false, "list available projects and exit")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "select project interactively")

	return cmd
}

// printAvailableProjects fetches and prints the projects available to the owner.
func printAvailableProjects(cmd *cobra.Command, deps project.Deps) error {
	w := cmd.OutOrStdout()
	ownerType, err := deps.DetectOwnerType(deps.Owner)
	if err != nil {
		ownerType = "User"
	}

	var projects []project.ProjectInfo
	var fetchErr error
	_ = ui.RunWithSpinner(w, "Fetching agentic projects...", func() error {
		projects, fetchErr = deps.FetchProjectsForOwner(deps.Owner, ownerType)
		return fetchErr
	})
	if fetchErr != nil {
		return fmt.Errorf("fetching projects: %w", fetchErr)
	}
	if len(projects) == 0 {
		fmt.Fprintf(w, "No projects found for %s\n", deps.Owner)
		return nil
	}

	currentID, _ := deps.GetRepoVariable(deps.Owner, deps.RepoName, project.ProjectVarName)

	fmt.Fprintf(w, "Available projects for %s:\n\n", deps.Owner)
	for _, p := range projects {
		marker := "  "
		if p.ID == currentID {
			marker = "* "
		}
		fmt.Fprintf(w, "%s%s\n", marker, p.Title)
	}
	if currentID != "" {
		fmt.Fprintln(w, "\n* current agentic project membership")
	}
	return nil
}

// newProjectUnlinkCmd constructs the `gh agentic project unlink` subcommand.
func newProjectUnlinkCmd() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "unlink",
		Short: "Remove this repo from its project",
		Long: `Remove this repo from the project. The GitHub Project board is not deleted
and the framework mount at .ai/ is left in place.

Blocked if this repo is the control plane and docs/ has content — migrate to
a new control plane first.

Use --yes to skip the confirmation prompt in scripts.`,
		Example: `  # Interactive confirmation
  gh agentic project unlink

  # Skip confirmation (for scripts)
  gh agentic project unlink --yes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			deps, err := resolveProjectDeps()
			if err != nil {
				return err
			}
			if yes {
				deps.Confirm = func(prompt string) (bool, error) { return true, nil }
			}
			return project.Unlink(cmd.OutOrStdout(), deps)
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation prompt")

	return cmd
}

// newProjectRepairCmd constructs the `gh agentic project repair` subcommand.
func newProjectRepairCmd() *cobra.Command {
	var topologyFlag string

	cmd := &cobra.Command{
		Use:   "repair",
		Short: "Auto-repair agentic project configuration issues found by check",
		Long: `Run all health checks and automatically fix any issues found.

Auto-repairs:
  - Framework not mounted      → mounts the latest version
  - Missing project views      → recreates views from the template
  - Topology misconfigured     → sets or corrects topology and framework version

When topology cannot be determined automatically you will be prompted, or use
--topology to skip the prompt:

  --topology single     standalone repo (control plane and code in one)
  --topology federated  this repo is the control plane for domain repos

Issues that cannot be auto-repaired are listed with manual remediation steps.`,
		Example: `  gh agentic project repair
  gh agentic project repair --topology federated`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			deps, err := resolveProjectDeps()
			if err != nil {
				return err
			}

			w := cmd.OutOrStdout()

			// Phase 1: run checks and attempt all auto-repairs.
			var result project.RepairResult
			_ = ui.RunWithDynamicSpinner(w, "Running agentic project checks...", func(setLabel func(string)) error {
				result = project.RepairWithProgress(deps, setLabel)
				return nil
			})

			// Resolve topology: flag > prompt > skip.
			needsTopology := result.NeedsTopologyPrompt
			chosenTopology := topologyFlag

			if needsTopology && chosenTopology == "" {
				fmt.Fprintln(w, "")
				fmt.Fprintf(w, "  %s  %s cannot be determined automatically.\n",
					ui.StatusWarning.Render("⚠"), "AGENTIC_TOPOLOGY")
				fmt.Fprintln(w, "")

				form := huh.NewForm(huh.NewGroup(
					huh.NewSelect[string]().
						Title("What is the role of this repository?").
						Description("Choose how this repo fits into the agentic project topology.").
						Options(
							huh.NewOption("Federated — this is the control plane for other (domain) repos", "federated"),
							huh.NewOption("Single    — this repo is the only repo (control plane + code together)", "single"),
						).
						Value(&chosenTopology),
				))
				if err := form.Run(); err != nil {
					return fmt.Errorf("topology selection: %w", err)
				}
			}

			// If --topology was passed directly, force a topology repair even when
			// the check didn't flag it (covers the "wrong value, undetectable" case).
			if chosenTopology != "" {
				var result2 project.RepairResult
				_ = ui.RunWithDynamicSpinner(w, "Repairing topology variables...", func(setLabel func(string)) error {
					result2 = project.RepairTopologyWithChoice(deps, chosenTopology)
					return nil
				})
				result.Lines = append(result.Lines, result2.Lines...)
				result.Repaired += result2.Repaired
				result.Unrepaired += result2.Unrepaired
			}

			fmt.Fprintln(w, "")
			fmt.Fprintln(w, "  "+ui.SectionHeading.Render("gh agentic — project repair"))
			fmt.Fprintln(w, "")
			for _, line := range result.Lines {
				fmt.Fprintln(w, line)
			}

			if needsTopology || chosenTopology != "" {
				fmt.Fprintln(w, "")
				if result.Unrepaired > 0 {
					fmt.Fprintf(w, "  %s\n\n", ui.StatusWarning.Render(fmt.Sprintf("%d issue(s) repaired, %d require manual attention", result.Repaired, result.Unrepaired)))
				} else {
					fmt.Fprintf(w, "  %s\n\n", ui.StatusOK.Render(fmt.Sprintf("%d issue(s) repaired", result.Repaired)))
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&topologyFlag, "topology", "", "override topology: 'single' or 'federated'")
	return cmd
}

// newProjectSwitchCmd constructs the `gh agentic project switch` command group.
func newProjectSwitchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "switch",
		Short: "Change project membership or framework version",
		Long: `Change this repo's project or upgrade/downgrade the framework version.

Requires the repo to already be initialised. Use 'switch project' to move
between projects, or 'switch version' to change the framework version.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newProjectSwitchProjectCmd())
	cmd.AddCommand(newProjectSwitchVersionCmd())
	return cmd
}

// newProjectSwitchProjectCmd constructs `project switch project`.
func newProjectSwitchProjectCmd() *cobra.Command {
	var list, interactive bool

	cmd := &cobra.Command{
		Use:   "project [project-name]",
		Short: "Move this repo to a different project",
		Long: `Move this repo to a different project.

The repo must already be initialised. Project names are matched
case-insensitively; quote names that contain spaces.`,
		Example: `  # List available projects
  gh agentic project switch project --list

  # Interactive picker
  gh agentic project switch project --interactive

  # Direct by name
  gh agentic project switch project "My Other Project"`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !list && !interactive && len(args) == 0 {
				return cmd.Help()
			}

			deps, err := resolveProjectDeps()
			if err != nil {
				return err
			}

			if list {
				return printAvailableProjects(cmd, deps)
			}

			var projectID string
			if len(args) == 1 {
				projectID, err = resolveProjectID(deps, args[0])
				if err != nil {
					return err
				}
				return project.SwitchProject(cmd.OutOrStdout(), deps, projectID)
			}

			// Interactive picker.
			ownerType, err := deps.DetectOwnerType(deps.Owner)
			if err != nil {
				ownerType = "User"
			}
			var projects []project.ProjectInfo
			var fetchErr error
			_ = ui.RunWithSpinner(cmd.OutOrStdout(), "Fetching agentic projects...", func() error {
				projects, fetchErr = deps.FetchProjectsForOwner(deps.Owner, ownerType)
				return fetchErr
			})
			if fetchErr != nil {
				return fmt.Errorf("fetching projects: %w", fetchErr)
			}
			if len(projects) == 0 {
				return fmt.Errorf("no projects found for %s", deps.Owner)
			}

			currentID, _ := deps.GetRepoVariable(deps.Owner, deps.RepoName, project.ProjectVarName)
			var options []huh.Option[string]
			for _, p := range projects {
				label := p.Title
				if p.ID == currentID {
					label += " (current)"
				}
				options = append(options, huh.NewOption(label, p.ID))
			}

			form := huh.NewForm(huh.NewGroup(
				huh.NewSelect[string]().
					Title("Select project").
					Description("GitHub ProjectV2 to move this repository to").
					Options(options...).
					Value(&projectID),
			))
			if err := form.Run(); err != nil {
				return fmt.Errorf("form error: %w", err)
			}
			if projectID == "" {
				return fmt.Errorf("project name is required")
			}
			return project.SwitchProject(cmd.OutOrStdout(), deps, projectID)
		},
	}
	cmd.Flags().BoolVarP(&list, "list", "l", false, "list available projects and exit")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "select project interactively")
	return cmd
}

// newProjectSwitchVersionCmd constructs `project switch version`.
func newProjectSwitchVersionCmd() *cobra.Command {
	var yes, list bool

	cmd := &cobra.Command{
		Use:   "version [version]",
		Short: "Upgrade or downgrade the framework version (control plane only)",
		Long: `Change the framework version for the whole project.

Only valid on the control plane repo. On a federated setup, domain repos are
notified via the version sync check and can run 'gh agentic mount' to update.

Blocked on domain repos — version governance flows through the control plane.

Use --list to browse available versions before switching.`,
		Example: `  # List available versions
  gh agentic project switch version --list

  # Switch to a specific version
  gh agentic project switch version v2.2.0

  # Switch without confirmation prompt (for scripts)
  gh agentic project switch version v2.2.0 --yes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()

			if list {
				var releases []mount.Release
				_ = ui.RunWithSpinner(w, "Fetching available framework versions...", func() error {
					var err error
					releases, err = mount.DefaultFetchReleases(mount.FrameworkRepo)
					return err
				})
				if len(releases) == 0 {
					fmt.Fprintf(w, "  No releases found.\n")
					return nil
				}
				fmt.Fprintln(w, "")
				fmt.Fprintln(w, "  "+ui.SectionHeading.Render("Available framework versions"))
				fmt.Fprintln(w, "  "+ui.Divider(48))
				for _, r := range releases {
					fmt.Fprintf(w, "  %s\n", r.TagName)
				}
				fmt.Fprintln(w, "")
				return nil
			}

			if len(args) == 0 {
				return cmd.Help()
			}

			deps, err := resolveProjectDeps()
			if err != nil {
				return err
			}

			var confirm func(string) (bool, error)
			if !yes {
				confirm = deps.Confirm
			}

			// Preflight: resolve topology, validate version, read current state —
			// all behind a spinner. SwitchVersion then uses the pre-resolved data
			// directly with no duplicate API calls.
			var pre project.SwitchVersionPreflight
			var preflightErr error
			_ = ui.RunWithDynamicSpinner(w, "Detecting topology...", func(setLabel func(string)) error {
				setLabel("Fetching framework releases...")
				pre, preflightErr = project.PreflightSwitchVersion(deps, args[0])
				return nil
			})
			if preflightErr != nil {
				return preflightErr
			}
			return project.SwitchVersion(w, deps, args[0], pre, confirm)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation prompt")
	cmd.Flags().BoolVarP(&list, "list", "l", false, "list available framework versions")
	return cmd
}

// newProjectInitCmd constructs the `gh agentic project init` subcommand.
func newProjectInitCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "First-time setup — join or create a project for this repo",
		Long: `Interactive wizard for first-time project setup.

Single repo: creates the GitHub Project board, mounts the framework, and
establishes this repo as the control plane — everything in one step.

Joining a federated project: the control plane must already exist (run
'project create' on the control plane repo first). Lists available projects,
lets you select one, and brings this repo in as a domain repo.

Blocked if this repo is already initialised — use 'project switch project' to
change membership, or --force to re-run setup.`,
		Example: `  gh agentic project init
  gh agentic project init --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()

			deps, err := resolveProjectDeps()
			if err != nil {
				return err
			}

			// Guard: already initialised? (bypass with --force)
			if !force {
				existing, _ := deps.GetRepoVariable(deps.Owner, deps.RepoName, project.ProjectVarName)
				if existing != "" {
					name := project.ProjectDisplayName(deps, existing)
					fmt.Fprintf(w, "  %s  Repo is already part of agentic project %q\n", ui.StatusWarning.Render("⚠"), name)
					fmt.Fprintf(w, "       → Use 'gh agentic project switch project' to change agentic project membership\n")
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
				// Single path: full wizard via initv2.
				initDeps := initv2.Deps{
					Run:   auth.DefaultRunCommand,
					Clone: mount.DefaultClone,
					CollectConfig: func(w io.Writer, repo string) (*initv2.InitConfig, error) {
						return initv2.CollectConfigInteractive(w, repo, initv2.FormDeps{
							RunForm:         initv2.DefaultFormRun,
							RunCommand:      auth.DefaultRunCommand,
							DetectOwnerType: defaultDetectOwnerType,
							FetchReleases:   mount.DefaultFetchReleases,
						})
					},
				}
				if err := initv2.Run(w, root, force, initDeps); err != nil {
					if errors.Is(err, initv2.ErrAlreadyInitialised) {
						return ErrSilent
					}
					return err
				}
				// Set topology variable.
				_ = deps.SetRepoVariable(deps.Owner, deps.RepoName, project.TopologyVarName, "single")
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
			initCfg, err := initv2.CollectConfigInteractive(w, deps.RepoFullName, initv2.FormDeps{
				RunForm:         initv2.DefaultFormRun,
				RunCommand:      auth.DefaultRunCommand,
				DetectOwnerType: defaultDetectOwnerType,
				FetchReleases:   mount.DefaultFetchReleases,
			})
			if err != nil {
				return fmt.Errorf("configuration: %w", err)
			}

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
