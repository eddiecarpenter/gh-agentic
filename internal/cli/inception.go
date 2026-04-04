package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
	"github.com/eddiecarpenter/gh-agentic/internal/inception"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// newInceptionCmd constructs the `gh agentic inception` subcommand.
func newInceptionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "inception",
		Short: "Register a new repo in an existing agentic environment (Phase 0b)",
		Long:  "Creates and configures a new domain, tool, or other repo and registers it in REPOS.md.",
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()

			// Step 1: Validate environment.
			fmt.Fprintln(w, ui.SectionHeading.Render("  Inception — register a new repo"))
			fmt.Fprintln(w)

			envCtx, err := inception.ValidateEnvironment(bootstrap.DefaultRunCommand)
			if err != nil {
				fmt.Fprintln(w, "  "+ui.RenderError(err.Error()))
				return err
			}
			fmt.Fprintln(w, "  "+ui.RenderOK("Agentic environment detected (owner: "+envCtx.Owner+")"))
			fmt.Fprintln(w)

			// Steps 2-5 will be wired in Task 4 (runner orchestration).
			_ = envCtx
			return nil
		},
	}
}
