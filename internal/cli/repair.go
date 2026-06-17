package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/auth"
	"github.com/eddiecarpenter/gh-agentic/internal/doctor"
	initpkg "github.com/eddiecarpenter/gh-agentic/internal/init"
	"github.com/eddiecarpenter/gh-agentic/internal/project"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// newRepairCmd constructs the top-level `gh agentic repair` command.
func newRepairCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repair",
		Short: "Auto-fix issues reported by 'check'",
		Long: `Run all health checks and automatically fix what can be fixed.

Topology is always deduced from the project-linked-repo graph plus ownership
(project owner vs repo owner) — no prompts, no manual override. Each repo's
repair fixes only its own state. If a federated-domain repo detects that the
control plane has missing state, repair terminates with a pointed instruction
to run 'gh agentic repair' on the control plane repo.

Auto-repairs:
  - Framework not mounted        → mounts the latest version
  - Missing project board views  → recreates views from the template
  - Topology variables           → writes AGENTIC_FRAMEWORK_VERSION on the CP
                                   when missing; clears stray values on domains
  - .agents/ missing from .gitignore → appends the entry
  - Workflow version tag drift   → rewrites @vX.Y.Z to match the mounted framework

Variables and secrets cannot be auto-repaired (they need human-supplied values).
Those failures are surfaced with the exact 'gh' command to run.`,
		Example:      `  gh agentic repair`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			deps, err := resolveProjectDeps()
			if err != nil {
				return err
			}

			w := cmd.OutOrStdout()

			// Framework-source mode (this repo IS gh-agentic): skip the
			// project-scope auto-repairs entirely. Those touch mount
			// state and `.agents/` content, which would harm the committed
			// symlink. The pipeline-scope repairs (missing variables,
			// missing secrets, runner label) still run and remain
			// useful.
			root, rerr := os.Getwd()
			if rerr != nil {
				return fmt.Errorf("resolving working directory: %w", rerr)
			}
			isFrameworkSource := project.IsFrameworkSource(root)

			// Phase 1: project-side checks and auto-repairs.
			var projectResult project.RepairResult
			if !isFrameworkSource {
				_ = ui.RunWithDynamicSpinner(w, "Running checks...", func(setLabel func(string)) error {
					projectResult = project.RepairWithProgress(deps, setLabel)
					return nil
				})
			}

			// Phase 2: pipeline-side checks and auto-repairs.
			// Skip when the framework mount is out of sync — the pipeline
			// checks run against `.agents/` and only produce noise until the
			// user runs `gh agentic repair`.
			pipelineDeps, pdepsErr := buildPipelineCheckDeps(deps)
			if pdepsErr == nil {
				pipelineDeps.FrameworkSource = isFrameworkSource
			}
			var pipelineResult doctor.RepairResult
			pipelineSkipped := projectResult.FrameworkOutOfSync
			if pdepsErr == nil && !pipelineSkipped {
				_ = ui.RunWithDynamicSpinner(w, "Running pipeline checks...", func(setLabel func(string)) error {
					pipelineResult = doctor.RepairPipeline(pipelineDeps, setLabel)
					return nil
				})
			}

			// Render combined output.
			fmt.Fprintln(w, "")
			fmt.Fprintln(w, "  "+ui.SectionHeading.Render("gh agentic — repair"))
			fmt.Fprintln(w, "")

			if isFrameworkSource {
				fmt.Fprintln(w, "  "+ui.StatusWarning.Render("⚠")+"  Framework source detected (.agents is a symlink)")
				fmt.Fprintln(w, "  "+ui.Muted.Render("   Content-layer repairs are skipped. Config-layer repairs (variables, secrets) run below."))
				fmt.Fprintln(w, "")
			} else {
				fmt.Fprintln(w, "  "+ui.SectionHeading.Render("Project"))
				fmt.Fprintln(w, "  "+ui.Divider(48))
				for _, line := range projectResult.Lines {
					fmt.Fprintln(w, line)
				}
			}

			renderRepairPipelineSection(w, pipelineSkipped, pdepsErr, pipelineResult)

			// Phase 3: prompt for missing variables/secrets and apply them.
			if !pipelineSkipped && pdepsErr == nil && len(pipelineResult.PendingPrompts) > 0 {
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

	return cmd
}

// renderRepairPipelineSection writes the Pipeline section of the repair report to w.
// It is extracted from RunE so the pipelineSkipped output path can be tested directly
// without wiring up live project and pipeline dependencies — analogous to
// renderCheckSections in check.go.
func renderRepairPipelineSection(w io.Writer, pipelineSkipped bool, pdepsErr error, pipelineResult doctor.RepairResult) {
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  "+ui.SectionHeading.Render("Pipeline"))
	fmt.Fprintln(w, "  "+ui.Divider(48))
	switch {
	case pipelineSkipped:
		fmt.Fprintf(w, "  %s  Skipped — framework is out of sync; run 'gh agentic repair' to sync\n", ui.StatusWarning.Render("⚠"))
	case pdepsErr != nil:
		fmt.Fprintf(w, "  %s  Skipped: %v\n", ui.StatusWarning.Render("⚠"), pdepsErr)
	case len(pipelineResult.Lines) == 0 && len(pipelineResult.PendingPrompts) == 0:
		fmt.Fprintf(w, "  %s  No pipeline issues found\n", ui.StatusOK.Render("✓"))
	default:
		for _, line := range pipelineResult.Lines {
			fmt.Fprintln(w, line)
		}
	}
}

// buildPipelineCheckDeps constructs the doctor.CheckDeps used for the
// pipeline-side health checks. It routes every AGENTIC_* read through the
// single canonical resolver (project.Resolve), so both the topology
// decision and the ProjectID come from the same code path.
func buildPipelineCheckDeps(pdeps project.Deps) (doctor.CheckDeps, error) {
	root, err := os.Getwd()
	if err != nil {
		return doctor.CheckDeps{}, fmt.Errorf("resolving working directory: %w", err)
	}

	ownerType, otErr := auth.DefaultDetectOwnerType(pdeps.Owner)
	if otErr != nil {
		ownerType = ""
	}

	run := pdeps.Run
	if run == nil {
		run = auth.DefaultRunCommand
	}

	// Single canonical read — project.Resolve handles variable precedence,
	// linked-repo inspection, and the federated-domain fallback.
	ctx, _ := project.Resolve(pdeps)
	projectID := ""
	topology := ""
	frameworkVersion := ""
	projectIDReadFailed := false
	if ctx != nil {
		projectID = ctx.ProjectID
		topology = ctx.Topology
		frameworkVersion = ctx.FrameworkVersion
		projectIDReadFailed = ctx.ProjectIDReadFailed
	}

	return doctor.CheckDeps{
		Root:                root,
		RepoFullName:        pdeps.RepoFullName,
		Owner:               pdeps.Owner,
		RepoName:            pdeps.RepoName,
		OwnerType:           ownerType,
		Topology:            topology,
		ProjectID:           projectID,
		FrameworkVersion:    frameworkVersion,
		ProjectIDReadFailed: projectIDReadFailed,
		Run:                 run,
		ReadCreds: func(r auth.RunCommandFunc) ([]byte, error) {
			return auth.ReadClaudeCredentialsDefault(r)
		},
		FetchProjectTitle:        project.DefaultFetchProjectTitle,
		FetchProjectFields:       project.DefaultFetchProjectFields,
		UpdateStatusFieldOptions: project.DefaultUpdateStatusFieldOptions,
		CreateProjectField:       project.DefaultCreateProjectField,
		FetchLinkedRepos:         project.DefaultFetchLinkedRepos,
		FetchOwnerAndRepoIDs:     project.DefaultFetchOwnerAndRepoIDs,
		LinkRepoToProject:        project.DefaultLinkRepoToProject,
	}, nil
}

// promptAndApplyPending presents one huh form per pending variable/secret,
// then applies non-empty answers via gh. Each prompt is its own form so the
// user can Esc out of one without losing the rest. Returns a partial result
// (Lines + counts) for the caller to merge.
func promptAndApplyPending(w io.Writer, deps doctor.CheckDeps, prompts []doctor.PendingPrompt) (doctor.RepairResult, error) {
	res := doctor.RepairResult{}

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

		applyErr := doctor.ApplyPendingPrompt(deps.Run, deps.RepoFullName, p, value)
		res.Lines = append(res.Lines, doctor.FormatPromptApplied(p, applyErr))
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
func promptValue(p doctor.PendingPrompt, deps doctor.CheckDeps) (string, error) {
	if p.Name == "RUNNER_LABEL" {
		return promptRunnerLabel(deps)
	}

	title := p.Name
	if p.Description != "" {
		title = p.Name + " — " + p.Description
	}

	// When the variable has a sensible default, ask whether to accept it or
	// supply a custom value, rather than presenting an open-ended input.
	if p.Default != "" && p.Kind != "secret" {
		const (
			useDefault = "default"
			useCustom  = "custom"
		)
		choice := useDefault
		selectForm := huh.NewForm(huh.NewGroup(
			huh.NewSelect[string]().
				Title(title).
				Options(
					huh.NewOption(fmt.Sprintf("Use default (%q)", p.Default), useDefault),
					huh.NewOption("Set a custom value", useCustom),
				).
				Value(&choice),
		))
		if err := selectForm.Run(); err != nil {
			return "", err
		}
		if choice == useDefault {
			return p.Default, nil
		}
	}

	var value string
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

// promptRunnerLabel mirrors initpkg.collectPipelineConfig's runner picker:
// a select with sensible candidates, falling through to a custom-label input
// when "other" is chosen.
func promptRunnerLabel(deps doctor.CheckDeps) (string, error) {
	value := initpkg.DefaultRunnerLabel
	options := initpkg.BuildRunnerOptions(deps.RepoName, deps.Owner)

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

	if value != initpkg.RunnerOther {
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
