package cli

import (
	"fmt"
	"os"

	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/spf13/cobra"

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
