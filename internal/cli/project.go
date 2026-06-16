package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/mount"
	"github.com/eddiecarpenter/gh-agentic/internal/project"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// newProjectCmd constructs the `gh agentic project` command group.
func newProjectCmd() *cobra.Command {
	b := ui.SectionHeading.Render
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage this repo's agentic project membership",
		Long: fmt.Sprintf(`Manage which control plane this repo belongs to.

An agentic project ties together a GitHub Project board, a control plane repo,
domain repos, and a shared framework version. This command group covers the
lifecycle operations — who this repo is affiliated with, and who governs it.

For first-time setup of a repo, run 'gh agentic init' instead. For health
checks and automatic repairs, run 'gh agentic check' and 'gh agentic repair'.

%s establishes this repo as the control plane of a new agentic project — it
creates the GitHub Project board and wires this repo to it.

%s brings this repo in as a domain repo under an existing control plane. The
control plane must already exist.

%s changes which control plane this repo belongs to — moves this repo from one
agentic project to another.

%s detaches this repo from its agentic project without touching the project
board or the framework mount.`,
			b("create"), b("join"), b("switch"), b("unlink")),
		// PersistentPreRunE covers the bare `project` command AND every
		// subcommand — none of them apply on the framework source, so
		// the guard belongs here rather than being duplicated in each
		// subcommand's RunE.
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			root, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("resolving working directory: %w", err)
			}
			// Build the refusal command name: "project" when invoked
			// bare, "project <sub>" when a subcommand fired it (e.g.
			// "project create"). cmd.Name() is the leaf name so we
			// only need to prepend "project" for subcommands.
			name := "project"
			if cmd.Name() != "project" {
				name = "project " + cmd.Name()
			}
			return refuseIfFrameworkSource(cmd, root, name)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newProjectCreateCmd())
	cmd.AddCommand(newProjectJoinCmd())
	cmd.AddCommand(newProjectUnlinkCmd())
	cmd.AddCommand(newProjectSwitchCmd())

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

// currentProjectID returns the current repo's agentic project ID by routing
// through project.Resolve. Used by the project sub-commands (list, join,
// switch) to mark the currently-affiliated project in selection pickers —
// replaces ad-hoc deps.GetRepoVariable(..., project.ProjectVarName) reads so
// every command consumes project state from the same canonical source.
// Returns "" if the repo is unaffiliated or the resolve fails.
func currentProjectID(deps project.Deps) string {
	ctx, err := project.Resolve(deps)
	if err != nil || ctx == nil {
		return ""
	}
	return ctx.ProjectID
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

			// Refuse on the framework source. `project create` would
			// create a new GitHub Project and link it to gh-agentic,
			// corrupting the framework's own project membership.
			root, rerr := os.Getwd()
			if rerr != nil {
				return fmt.Errorf("resolving working directory: %w", rerr)
			}
			if err := refuseIfFrameworkSource(cmd, root, "project create"); err != nil {
				return err
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

// newProjectJoinCmd constructs the `gh agentic project join` subcommand. Run on
// the control plane, it registers a domain repo (pure code — no framework mount).
func newProjectJoinCmd() *cobra.Command {
	var domain, purpose, domainPurpose string

	cmd := &cobra.Command{
		Use:   "join <owner/repo>",
		Short: "Register a domain repo with this control plane (no framework mount)",
		Long: `Register a domain repository with this federated control plane.

Run on the control plane. Adds <owner/repo> to the domain-grouped FEDERATION.md
under --domain (creating the domain if it does not exist yet), links the repo to
the GitHub Project, and sets AGENTIC_PROJECT_ID on the target repo. The domain
repo is pure code — no framework is mounted into it.`,
		Example: `  gh agentic project join acme/billing-svc --domain billing --purpose "Bill runs"`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, rerr := os.Getwd()
			if rerr != nil {
				return fmt.Errorf("resolving working directory: %w", rerr)
			}
			if err := refuseIfFrameworkSource(cmd, root, "project join"); err != nil {
				return err
			}
			if strings.TrimSpace(domain) == "" {
				return fmt.Errorf("--domain is required (the domain the repo belongs to)")
			}
			parts := strings.SplitN(args[0], "/", 2)
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				return fmt.Errorf("expected <owner/repo>, got %q", args[0])
			}
			targetOwner, targetRepo := parts[0], parts[1]

			deps, err := resolveProjectDeps()
			if err != nil {
				return err
			}
			ctx, err := project.Resolve(deps)
			if err != nil {
				return err
			}
			if ctx == nil || ctx.ProjectID == "" {
				return fmt.Errorf("this repo is not a control plane (no agentic project) — run 'gh agentic project create' first")
			}

			// A new domain needs a purpose collected before JoinDomain creates it.
			newDomain := true
			if fed, ferr := project.ReadFederation(deps.Root); ferr == nil {
				newDomain = !fed.HasDomain(domain)
			}
			if strings.TrimSpace(purpose) == "" {
				purpose, err = promptText(fmt.Sprintf("Purpose of %s within the federation", args[0]))
				if err != nil {
					return err
				}
			}
			if newDomain && strings.TrimSpace(domainPurpose) == "" {
				domainPurpose, err = promptText(fmt.Sprintf("Purpose of the new domain %q", domain))
				if err != nil {
					return err
				}
			}

			return project.JoinDomain(cmd.OutOrStdout(), deps, ctx.ProjectID, targetOwner, targetRepo, domain, domainPurpose, purpose)
		},
	}

	cmd.Flags().StringVar(&domain, "domain", "", "domain the repo belongs to (required)")
	cmd.Flags().StringVar(&purpose, "purpose", "", "the repo's purpose within its domain")
	cmd.Flags().StringVar(&domainPurpose, "domain-purpose", "", "purpose of the domain (used when creating a new domain)")

	return cmd
}

// promptText runs a single huh text input and returns the trimmed, non-empty value.
func promptText(title string) (string, error) {
	var v string
	if err := huh.NewForm(huh.NewGroup(huh.NewInput().Title(title).Value(&v))).Run(); err != nil {
		return "", fmt.Errorf("input form: %w", err)
	}
	if strings.TrimSpace(v) == "" {
		return "", fmt.Errorf("%q is required", title)
	}
	return v, nil
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

	currentID := currentProjectID(deps)

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
and the framework mount at .agents/ is left in place.

Blocked if this repo is the control plane and docs/ has content — migrate to
a new control plane first.

Use --yes to skip the confirmation prompt in scripts.`,
		Example: `  # Interactive confirmation
  gh agentic project unlink

  # Skip confirmation (for scripts)
  gh agentic project unlink --yes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Refuse on the framework source. Unlinking would delete
			// AGENTIC_PROJECT_ID, decoupling the framework from its own
			// canonical project.
			root, rerr := os.Getwd()
			if rerr != nil {
				return fmt.Errorf("resolving working directory: %w", rerr)
			}
			if err := refuseIfFrameworkSource(cmd, root, "project unlink"); err != nil {
				return err
			}

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

// newProjectSwitchCmd constructs the `gh agentic project switch` subcommand.
// This is a flat command (no sub-subcommand): it moves this repo between
// agentic projects. To change framework version, use `gh agentic upgrade`.
func newProjectSwitchCmd() *cobra.Command {
	var list, interactive bool

	cmd := &cobra.Command{
		Use:   "switch [project-name]",
		Short: "Move this repo to a different agentic project",
		Long: `Change which control plane this repo belongs to.

The repo must already be initialised. Project names are matched
case-insensitively; quote names that contain spaces.

To change the framework version for the whole federation, use
'gh agentic upgrade' on the control plane repo.`,
		Example: `  # List available projects
  gh agentic project switch --list

  # Interactive picker
  gh agentic project switch --interactive

  # Direct by name
  gh agentic project switch "My Other Project"`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !list && !interactive && len(args) == 0 {
				return cmd.Help()
			}

			// Refuse on the framework source.
			root, rerr := os.Getwd()
			if rerr != nil {
				return fmt.Errorf("resolving working directory: %w", rerr)
			}
			if err := refuseIfFrameworkSource(cmd, root, "project switch"); err != nil {
				return err
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

			currentID := currentProjectID(deps)
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
					Description("Agentic project to move this repository to").
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
