package cli

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/auth"
	"github.com/eddiecarpenter/gh-agentic/internal/doctorv2"
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
  - Framework not mounted        → mounts the latest version
  - Missing project board views  → recreates views from the template
  - Topology misconfigured       → sets or corrects topology and framework version
  - .ai/ missing from .gitignore → appends the entry
  - Workflow version tag drift   → rewrites @vX.Y.Z to match the mounted framework

When topology cannot be determined automatically you will be prompted, or use
--topology to skip the prompt:

  --topology single     standalone repo (control plane and code in one)
  --topology federated  this repo is the control plane for domain repos

Variables and secrets cannot be auto-repaired (they need human-supplied values).
Those failures are surfaced with the exact 'gh' command to run.`,
		Example: `  gh agentic repair
  gh agentic repair --topology federated`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			deps, err := resolveProjectDeps()
			if err != nil {
				return err
			}

			w := cmd.OutOrStdout()

			// Phase 1: project-side checks and auto-repairs.
			var projectResult project.RepairResult
			_ = ui.RunWithDynamicSpinner(w, "Running checks...", func(setLabel func(string)) error {
				projectResult = project.RepairWithProgress(deps, setLabel)
				return nil
			})

			needsTopology := projectResult.NeedsTopologyPrompt
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
				var topoResult project.RepairResult
				_ = ui.RunWithDynamicSpinner(w, "Repairing topology variables...", func(setLabel func(string)) error {
					topoResult = project.RepairTopologyWithChoice(deps, chosenTopology)
					return nil
				})
				projectResult.Lines = append(projectResult.Lines, topoResult.Lines...)
				projectResult.Repaired += topoResult.Repaired
				projectResult.Unrepaired += topoResult.Unrepaired
			}

			// Phase 2: pipeline-side checks and auto-repairs.
			pipelineDeps, pdepsErr := buildPipelineCheckDeps(deps)
			var pipelineResult doctorv2.RepairResult
			if pdepsErr == nil {
				_ = ui.RunWithDynamicSpinner(w, "Running pipeline checks...", func(setLabel func(string)) error {
					pipelineResult = doctorv2.RepairPipeline(pipelineDeps, setLabel)
					return nil
				})
			}

			// Render combined output.
			fmt.Fprintln(w, "")
			fmt.Fprintln(w, "  "+ui.SectionHeading.Render("gh agentic — repair"))
			fmt.Fprintln(w, "")

			fmt.Fprintln(w, "  "+ui.SectionHeading.Render("Project"))
			fmt.Fprintln(w, "  "+ui.Divider(48))
			for _, line := range projectResult.Lines {
				fmt.Fprintln(w, line)
			}

			fmt.Fprintln(w, "")
			fmt.Fprintln(w, "  "+ui.SectionHeading.Render("Pipeline"))
			fmt.Fprintln(w, "  "+ui.Divider(48))
			if pdepsErr != nil {
				fmt.Fprintf(w, "  %s  Skipped: %v\n", ui.StatusWarning.Render("⚠"), pdepsErr)
			} else if len(pipelineResult.Lines) == 0 {
				fmt.Fprintf(w, "  %s  No pipeline issues found\n", ui.StatusOK.Render("✓"))
			} else {
				for _, line := range pipelineResult.Lines {
					fmt.Fprintln(w, line)
				}
			}

			totalRepaired := projectResult.Repaired + pipelineResult.Repaired
			totalUnrepaired := projectResult.Unrepaired + pipelineResult.Unrepaired

			fmt.Fprintln(w, "")
			switch {
			case totalRepaired == 0 && totalUnrepaired == 0:
				fmt.Fprintf(w, "  %s\n\n", ui.StatusOK.Render("Nothing to repair"))
			case totalUnrepaired > 0:
				fmt.Fprintf(w, "  %s\n\n",
					ui.StatusWarning.Render(fmt.Sprintf("%d issue(s) repaired, %d require manual attention",
						totalRepaired, totalUnrepaired)))
			default:
				fmt.Fprintf(w, "  %s\n\n",
					ui.StatusOK.Render(fmt.Sprintf("%d issue(s) repaired", totalRepaired)))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&topologyFlag, "topology", "", "override topology: 'single' or 'federated'")
	return cmd
}

// buildPipelineCheckDeps constructs the doctorv2.CheckDeps used for the
// pipeline-side health checks, mirroring how `gh agentic check` resolves them.
func buildPipelineCheckDeps(pdeps project.Deps) (doctorv2.CheckDeps, error) {
	root, err := os.Getwd()
	if err != nil {
		return doctorv2.CheckDeps{}, fmt.Errorf("resolving working directory: %w", err)
	}

	repoFullName := pdeps.RepoFullName
	owner := pdeps.Owner
	repoName := pdeps.RepoName

	ownerType, otErr := auth.DefaultDetectOwnerType(owner)
	if otErr != nil {
		ownerType = ""
	}

	run := pdeps.Run
	if run == nil {
		run = auth.DefaultRunCommand
	}

	projectID, _ := runGetVariable(run, repoFullName, "AGENTIC_PROJECT_ID")
	topology := ""
	if projectID != "" {
		topoVal, _ := runGetVariable(run, repoFullName, "AGENTIC_TOPOLOGY")
		switch topoVal {
		case "federated":
			topology = resolveTopologyMode(run, repoFullName)
		case "single":
			topology = "single"
		default:
			topology = resolveTopologyMode(run, repoFullName)
			if topology == "federated-domain" {
				topology = "single"
			}
		}
	}

	return doctorv2.CheckDeps{
		Root:         root,
		RepoFullName: repoFullName,
		Owner:        owner,
		RepoName:     repoName,
		OwnerType:    ownerType,
		Topology:     topology,
		ProjectID:    projectID,
		Run:          run,
		ReadCreds: func(r auth.RunCommandFunc) ([]byte, error) {
			return auth.ReadClaudeCredentialsDefault(r)
		},
	}, nil
}
