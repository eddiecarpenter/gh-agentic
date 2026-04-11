package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
	"github.com/eddiecarpenter/gh-agentic/internal/inception"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// newInceptionCmd constructs the `gh agentic inception` subcommand.
func newInceptionCmd() *cobra.Command {
	var (
		nonInteractive bool
		repoType       string
		repoName       string
		description    string
		stacks         []string
	)

	cmd := &cobra.Command{
		Use:   "inception",
		Short: "Register a new repo in an existing agentic environment (Phase 0b)",
		Long:  "Creates and configures a new domain, tool, or other repo and registers it in REPOS.md.",
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()

			// Step 1: Validate environment.
			fmt.Fprintln(w, ui.SectionHeading.Render("  Inception — register a new repo"))
			fmt.Fprintln(w)

			envCtx, err := inception.ValidateEnvironment(bootstrap.DefaultRunCommand, bootstrap.DefaultDetectOwnerType)
			if err != nil {
				fmt.Fprintln(w, "  "+ui.RenderError(err.Error()))
				return err
			}
			fmt.Fprintln(w, "  "+ui.RenderOK("Agentic environment detected (owner: "+envCtx.Owner+")"))
			fmt.Fprintln(w)

			var cfg *inception.InceptionConfig

			if nonInteractive {
				// Validate required flags.
				var missing []string
				if repoType == "" {
					missing = append(missing, "--repo-type")
				}
				if repoName == "" {
					missing = append(missing, "--repo-name")
				}
				if len(stacks) == 0 {
					missing = append(missing, "--stack")
				}

				if len(missing) > 0 {
					return fmt.Errorf("--non-interactive requires %s", strings.Join(missing, ", "))
				}

				// Validate field values.
				if err := inception.ValidateRepoName(repoName); err != nil {
					return fmt.Errorf("invalid --repo-name: %w", err)
				}
				if err := inception.ValidateStackSelection(stacks); err != nil {
					return fmt.Errorf("invalid --stack: %w", err)
				}

				// Build config from flags plus env context.
				cfg = &inception.InceptionConfig{
					RepoType:    repoType,
					RepoName:    repoName,
					Description: description,
					Stacks:      stacks,
					Owner:       envCtx.Owner,
				}
			} else {
				// Interactive path — unchanged.
				cfg, err = inception.RunForm(w, *envCtx, inception.DefaultFormRun)
				if errors.Is(err, inception.ErrAborted) {
					fmt.Fprintln(w, ui.Muted.Render("Aborted."))
					return nil
				}
				if err != nil {
					return err
				}
			}

			// Execute inception steps with spinner.
			return inception.RunSteps(
				w,
				cfg,
				envCtx,
				bootstrap.DefaultRunCommand,
				inception.DefaultSpinner,
			)
		},
	}

	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "run without prompting — all required flags must be provided")
	cmd.Flags().StringVar(&repoType, "repo-type", "", "repo type: domain, tool, or other")
	cmd.Flags().StringVar(&repoName, "repo-name", "", "repo name (without suffix, e.g. 'charging')")
	cmd.Flags().StringVar(&description, "description", "", "short repo description")
	cmd.Flags().StringArrayVar(&stacks, "stack", nil, "technology stack (repeatable, e.g. --stack Go --stack Rust)")
	return cmd
}
