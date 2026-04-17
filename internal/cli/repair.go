package cli

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/project"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// newRepairCmd constructs the top-level `gh agentic repair` command.
func newRepairCmd() *cobra.Command {
	var topologyFlag string

	cmd := &cobra.Command{
		Use:   "repair",
		Short: "Auto-fix issues reported by 'check'",
		Long: `Run all health checks and automatically fix what can be fixed.

Auto-repairs:
  - Framework not mounted      → mounts the latest version
  - Missing project board views → recreates views from the template
  - Topology misconfigured      → sets or corrects topology and framework version

When topology cannot be determined automatically you will be prompted, or use
--topology to skip the prompt:

  --topology single     standalone repo (control plane and code in one)
  --topology federated  this repo is the control plane for domain repos

Pipeline-side issues (missing secrets, runtime variables, workflow drift) are
reported with manual remediation steps — they are not auto-repairable.`,
		Example: `  gh agentic repair
  gh agentic repair --topology federated`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			deps, err := resolveProjectDeps()
			if err != nil {
				return err
			}

			w := cmd.OutOrStdout()

			// Phase 1: run checks and attempt all auto-repairs.
			var result project.RepairResult
			_ = ui.RunWithDynamicSpinner(w, "Running checks...", func(setLabel func(string)) error {
				result = project.RepairWithProgress(deps, setLabel)
				return nil
			})

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
			fmt.Fprintln(w, "  "+ui.SectionHeading.Render("gh agentic — repair"))
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
