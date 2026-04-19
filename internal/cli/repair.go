package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/auth"
	"github.com/eddiecarpenter/gh-agentic/internal/doctorv2"
	"github.com/eddiecarpenter/gh-agentic/internal/initv2"
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
				// Refuse federated topology against a user-owned repo before
				// any side effect runs. DetectOwnerType is best-effort — if
				// it fails, the guard inside repairTopologyVars still enforces
				// the rule when the detected type is available there.
				if ownerType, otErr := deps.DetectOwnerType(deps.Owner); otErr == nil {
					if guardErr := project.EnsureFederatedOwnerIsOrg(chosenTopology, deps.Owner, ownerType); guardErr != nil {
						return guardErr
					}
				}

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
			} else if len(pipelineResult.Lines) == 0 && len(pipelineResult.PendingPrompts) == 0 {
				fmt.Fprintf(w, "  %s  No pipeline issues found\n", ui.StatusOK.Render("✓"))
			} else {
				for _, line := range pipelineResult.Lines {
					fmt.Fprintln(w, line)
				}
			}

			// Phase 3: prompt for missing variables/secrets and apply them.
			if pdepsErr == nil && len(pipelineResult.PendingPrompts) > 0 {
				applied, err := promptAndApplyPending(w, pipelineDeps, pipelineResult.PendingPrompts)
				if err != nil {
					fmt.Fprintf(w, "\n  %s  Prompt cancelled: %v\n", ui.StatusWarning.Render("⚠"), err)
					pipelineResult.Unrepaired += len(pipelineResult.PendingPrompts)
				} else {
					for _, line := range applied.Lines {
						fmt.Fprintln(w, line)
					}
					pipelineResult.Repaired += applied.Repaired
					pipelineResult.Unrepaired += applied.Unrepaired
				}
			}

			// Phase 4: shadow-vars batch confirm + delete. The prompt must
			// live outside the spinner phase so huh.NewConfirm can own the
			// terminal. One confirmation covers the whole batch.
			if pdepsErr == nil && pipelineResult.ShadowBatch != nil {
				shadowRes := doctorv2.RepairShadowValues(
					pipelineResult.ShadowBatch.Items,
					pipelineDeps.Run,
					huhConfirm,
				)
				for _, line := range shadowRes.Lines {
					fmt.Fprintln(w, line)
				}
				pipelineResult.Repaired += shadowRes.Repaired
				pipelineResult.Unrepaired += shadowRes.Unrepaired
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

// promptAndApplyPending presents one huh form per pending variable/secret,
// then applies non-empty answers via gh. Each prompt is its own form so the
// user can Esc out of one without losing the rest. Returns a partial result
// (Lines + counts) for the caller to merge.
func promptAndApplyPending(w io.Writer, deps doctorv2.CheckDeps, prompts []doctorv2.PendingPrompt) (doctorv2.RepairResult, error) {
	res := doctorv2.RepairResult{}

	fmt.Fprintln(w, "")
	fmt.Fprintf(w, "  %s  %d value(s) need to be set. Leave blank to skip any.\n",
		ui.Muted.Render("→"), len(prompts))
	fmt.Fprintln(w, "")

	for _, p := range prompts {
		value, err := promptValue(p, deps)
		if err != nil {
			// User cancelled the form (Ctrl+C / Esc). Bail out — the caller
			// counts remaining prompts as Unrepaired.
			return res, err
		}

		// Empty answer + a non-empty default → use the default.
		if strings.TrimSpace(value) == "" && p.Default != "" {
			value = p.Default
		}

		applyErr := doctorv2.ApplyPendingPrompt(deps.Run, deps.RepoFullName, p, value)
		res.Lines = append(res.Lines, doctorv2.FormatPromptApplied(p, applyErr))
		if applyErr != nil {
			res.Unrepaired++
		} else {
			res.Repaired++
		}
	}

	return res, nil
}

// promptValue runs the appropriate huh form for a single pending prompt and
// returns the user-supplied value. RUNNER_LABEL gets the same select-then-
// custom flow used by `gh agentic init`; everything else uses a single text
// input (with password masking for secrets).
func promptValue(p doctorv2.PendingPrompt, deps doctorv2.CheckDeps) (string, error) {
	if p.Name == "RUNNER_LABEL" {
		return promptRunnerLabel(deps)
	}

	var value string
	title := p.Name
	if p.Description != "" {
		title = p.Name + " — " + p.Description
	}
	input := huh.NewInput().Title(title).Value(&value)
	if p.Default != "" {
		input = input.Placeholder(p.Default)
	}
	if p.Kind == "secret" {
		input = input.EchoMode(huh.EchoModePassword)
	}
	if err := huh.NewForm(huh.NewGroup(input)).Run(); err != nil {
		return "", err
	}
	return value, nil
}

// huhConfirm is the production ConfirmFunc used by shadow-vars repair.
// The description shows the bulleted list of names under the title so the
// human sees exactly what will be removed before answering.
func huhConfirm(title, description string) (bool, error) {
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

// promptRunnerLabel mirrors initv2.collectPipelineConfig's runner picker:
// a select with sensible candidates, falling through to a custom-label input
// when "other" is chosen.
func promptRunnerLabel(deps doctorv2.CheckDeps) (string, error) {
	value := initv2.DefaultRunnerLabel
	options := initv2.BuildRunnerOptions(deps.RepoName, deps.Owner)

	selectForm := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("RUNNER_LABEL").
			Description("The GitHub Actions runner label for the agentic pipeline").
			Options(options...).
			Value(&value),
	))
	if err := selectForm.Run(); err != nil {
		return "", err
	}

	if value != initv2.RunnerOther {
		return value, nil
	}

	value = ""
	customForm := huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title("Custom runner label").
			Description("Enter your custom GitHub Actions runner label").
			Value(&value),
	))
	if err := customForm.Run(); err != nil {
		return "", err
	}
	return value, nil
}
