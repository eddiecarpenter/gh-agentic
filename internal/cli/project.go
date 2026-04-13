package cli

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/mount"
	"github.com/eddiecarpenter/gh-agentic/internal/project"
)

// newProjectCmd constructs the `gh agentic project` command group.
func newProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage project affiliation and health",
		Long:  "Create, join, check, and manage GitHub Project affiliation for this repository.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newProjectInfoCmd())
	cmd.AddCommand(newProjectCheckCmd())
	cmd.AddCommand(newProjectCreateCmd())
	cmd.AddCommand(newProjectJoinCmd())
	cmd.AddCommand(newProjectUnlinkCmd())
	cmd.AddCommand(newProjectRepairCmd())

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

// newProjectInfoCmd constructs the `gh agentic project info` subcommand.
func newProjectInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show project state and topology",
		RunE: func(cmd *cobra.Command, args []string) error {
			deps, err := resolveProjectDeps()
			if err != nil {
				return err
			}
			return project.PrintInfo(cmd.OutOrStdout(), deps)
		},
	}
}

// newProjectCheckCmd constructs the `gh agentic project check` subcommand.
func newProjectCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Verify project health",
		RunE: func(cmd *cobra.Command, args []string) error {
			deps, err := resolveProjectDeps()
			if err != nil {
				return err
			}

			report := project.RunChecks(deps)
			ok := project.PrintReport(cmd.OutOrStdout(), report)
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
	return &cobra.Command{
		Use:   "create",
		Short: "Create a new project and establish this repo as the control plane",
		RunE: func(cmd *cobra.Command, args []string) error {
			deps, err := resolveProjectDeps()
			if err != nil {
				return err
			}

			// Fetch available framework releases for the version selector.
			releases, err := deps.FetchReleases(mount.FrameworkRepo)
			if err != nil {
				return fmt.Errorf("fetching framework releases: %w", err)
			}
			if len(releases) == 0 {
				return fmt.Errorf("no framework releases available")
			}

			versionOptions := make([]huh.Option[string], 0, len(releases))
			for _, r := range releases {
				versionOptions = append(versionOptions, huh.NewOption(r.TagName, r.TagName))
			}

			var cfg project.CreateConfig
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

			return project.Create(cmd.OutOrStdout(), deps, cfg)
		},
	}
}

// newProjectJoinCmd constructs the `gh agentic project join` subcommand.
func newProjectJoinCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "join <project-id>",
		Short: "Affiliate this repo with an existing project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			deps, err := resolveProjectDeps()
			if err != nil {
				return err
			}
			return project.Join(cmd.OutOrStdout(), deps, args[0])
		},
	}
}

// newProjectUnlinkCmd constructs the `gh agentic project unlink` subcommand.
func newProjectUnlinkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unlink",
		Short: "Remove this repo's project affiliation",
		RunE: func(cmd *cobra.Command, args []string) error {
			deps, err := resolveProjectDeps()
			if err != nil {
				return err
			}
			return project.Unlink(cmd.OutOrStdout(), deps)
		},
	}
}

// newProjectRepairCmd constructs the `gh agentic project repair` subcommand.
func newProjectRepairCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "repair",
		Short: "Interactively fix project health issues",
		RunE: func(cmd *cobra.Command, args []string) error {
			deps, err := resolveProjectDeps()
			if err != nil {
				return err
			}
			return project.Repair(cmd.OutOrStdout(), deps)
		},
	}
}
