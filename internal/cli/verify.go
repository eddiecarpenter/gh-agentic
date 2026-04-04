package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/ui"
	"github.com/eddiecarpenter/gh-agentic/internal/verify"
)

// newVerifyCmd constructs the `gh agentic verify` subcommand.
func newVerifyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "verify",
		Short: "Verify an agentic environment for correctness",
		Long: "Checks an existing agentic environment for correctness and repairs\n" +
			"what it can automatically. Each check shows ✔ pass, ⚠ warning, or ✖ fail.",
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()

			fmt.Fprintln(w, ui.SectionHeading.Render("  Verify — check agentic environment"))
			fmt.Fprintln(w)

			// Collect all checks — will be populated by subsequent tasks.
			checks := []verify.CheckFunc{}

			// Repair function — will be populated by subsequent tasks.
			var repairFn verify.RepairFunc

			return verify.RunVerify(w, checks, repairFn)
		},
	}
}
