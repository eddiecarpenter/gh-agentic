package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/auth"
	"github.com/eddiecarpenter/gh-agentic/internal/doctor"
	"github.com/eddiecarpenter/gh-agentic/internal/project"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// repoInfo holds resolved repository identity.
type repoInfo struct {
	FullName  string
	Owner     string
	RepoName  string
	OwnerType string
}

// resolveRepoFunc resolves the current repo identity.
type resolveRepoFunc func() (repoInfo, error)

// checkDeps holds injectable dependencies for the check command.
type checkDeps struct {
	run              auth.RunCommandFunc
	readCreds        auth.ReadCredentialsFunc
	resolveRepo      resolveRepoFunc
	fetchLinkedRepos project.FetchLinkedReposFunc
}

// projectFrameworkOutOfSync returns true when the project-side
// framework-version-sync check reports a Fail status. Pipeline-scope
// checks (skill frontmatter, workflow versions) operate on
// `.agents/` and produce noise against a stale mount, so both `check` and
// `repair` short-circuit on this signal.
func projectFrameworkOutOfSync(report *project.CheckReport) bool {
	if report == nil {
		return false
	}
	for _, r := range report.Results {
		if r.Name == "framework-version-sync" && r.Status == project.CheckFail {
			return true
		}
	}
	return false
}

// defaultResolveRepo resolves the repo from git remote config.
func defaultResolveRepo() (repoInfo, error) {
	currentRepo, err := repository.Current()
	if err != nil {
		return repoInfo{}, err
	}

	ownerType, typeErr := auth.DefaultDetectOwnerType(currentRepo.Owner)
	if typeErr != nil {
		ownerType = ""
	}

	return repoInfo{
		FullName:  currentRepo.Owner + "/" + currentRepo.Name,
		Owner:     currentRepo.Owner,
		RepoName:  currentRepo.Name,
		OwnerType: ownerType,
	}, nil
}

// renderCheckSections writes the Project and Pipeline sections of the check
// report to w. It is extracted from RunE so the no-empty-heading structural
// invariant can be tested directly without a full command execution.
//
// The parent "Pipeline" heading is rendered only when pipelineSkipped is true —
// in that case it anchors the single ⚠ status line and is meaningful. In the
// non-skipped path the sub-group headings emitted by doctor.RenderGroup are
// self-labelling; a bare "Pipeline" heading with no status line beneath it
// (before the first sub-group heading) caused autonomous sessions to misread it
// as a pre-existing pipeline warning (Feature #715). Audit: every producer of
// doctor.Group always appends at least one CheckResult before returning, so
// RenderGroup never emits an empty heading; the no-empty-heading invariant test
// in check_test.go pins this going forward.
func renderCheckSections(w io.Writer, isFrameworkSource bool, projectReport *project.CheckReport, pipelineReport *doctor.Report, pipelineSkipped bool) {
	if isFrameworkSource {
		fmt.Fprintln(w, "  "+ui.StatusWarning.Render("⚠")+"  Framework source detected (.agents is a symlink)")
		fmt.Fprintln(w, "  "+ui.Muted.Render("   Project-scope and content-layer checks are skipped — they do not apply"))
		fmt.Fprintln(w, "  "+ui.Muted.Render("   when this repo IS the gh-agentic framework. Config-layer checks run below."))
		fmt.Fprintln(w, "")
	}

	// Project section — omitted in framework-source mode since
	// projectReport is nil there.
	if projectReport != nil {
		fmt.Fprintln(w, "  "+ui.SectionHeading.Render("Project"))
		fmt.Fprintln(w, "  "+ui.Divider(48))
		for _, r := range projectReport.Results {
			fmt.Fprintf(w, "  %s  %s\n", project.StatusIcon(r.Status), r.Message)
			if r.Remediation != "" {
				fmt.Fprintf(w, "       %s\n", ui.Muted.Render("→ "+r.Remediation))
			}
		}
		fmt.Fprintln(w, "")
	}

	// Pipeline section. The parent heading appears only in the skipped branch
	// where it precedes the explicit ⚠ status line. In the non-skipped branch
	// the sub-group headings are already self-labelling.
	if pipelineSkipped {
		fmt.Fprintln(w, "  "+ui.SectionHeading.Render("Pipeline"))
		fmt.Fprintln(w, "  "+ui.Divider(48))
		fmt.Fprintf(w, "  %s  Skipped — framework is out of sync; run 'gh agentic repair' to sync\n", ui.StatusWarning.Render("⚠"))
	} else if pipelineReport != nil {
		for _, g := range pipelineReport.Groups {
			doctor.RenderGroup(w, g)
		}
	}
}

// newCheckCmd constructs the top-level `gh agentic check` command.
func newCheckCmd() *cobra.Command {
	return newCheckCmdWithDeps(checkDeps{
		run: auth.DefaultRunCommand,
		readCreds: func(run auth.RunCommandFunc) ([]byte, error) {
			return auth.ReadClaudeCredentialsDefault(run)
		},
		resolveRepo:      defaultResolveRepo,
		fetchLinkedRepos: project.DefaultFetchLinkedRepos,
	})
}

// newCheckCmdWithDeps builds the check command with injectable dependencies for testing.
func newCheckCmdWithDeps(deps checkDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Verify project membership and pipeline readiness",
		Long: `Run a full health check covering both project membership and pipeline readiness.

The Project section verifies this repo's agentic project membership: that the
project is set and accessible, topology is correct, the framework is mounted
and in sync with the control plane, and the project board has the expected views.

The Pipeline section verifies the infrastructure the agent needs to run:
wrapper workflows pinned to the correct version, runtime variables and secrets,
and the agent instruction files (CLAUDE.md, AGENTS.md, LOCALRULES.md).

Run 'gh agentic repair' to auto-fix any issues that can be fixed automatically.`,
		Example:      `  gh agentic check`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()

			root, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("resolving working directory: %w", err)
			}

			// Framework-source mode (this repo IS gh-agentic): skip the
			// project-scope checks entirely — those inspect mount state
			// and project membership in ways that don't apply when the
			// framework IS the source. The pipeline-scope doctor checks
			// still run, configured with FrameworkSource: true so they
			// substitute a synthetic "skipped — framework source" group
			// for the content-layer checks and run the config-layer
			// (variables, secrets, workflows, reachability) as normal.
			isFrameworkSource := project.IsFrameworkSource(root)

			projectDeps, err := resolveProjectDeps()
			if err != nil {
				return err
			}

			var projectReport *project.CheckReport
			var pipelineReport *doctor.Report
			var pipelineSkipped bool

			_ = ui.RunWithDynamicSpinner(w, "Running project checks...", func(setLabel func(string)) error {
				if isFrameworkSource {
					setLabel("Framework source detected — skipping project-scope checks...")
					// Leave projectReport nil; the renderer treats nil as
					// "no project-scope output" and moves straight to the
					// pipeline-scope section.
				} else {
					// Project-scope checks first — they already handle topology internally.
					projectReport = project.RunChecksWithProgress(projectDeps, setLabel)

					// Short-circuit: if the mounted framework is out of sync
					// with the authoritative version, pipeline-side checks
					// (skill frontmatter, workflow versions) will generate
					// noise against a stale `.agents/`. Stop here and direct
					// the user to `gh agentic repair`.
					if projectFrameworkOutOfSync(projectReport) {
						pipelineSkipped = true
						return nil
					}
				}

				// Pipeline-scope checks need repo identity + resolved topology mode.
				setLabel("Detecting repository for pipeline checks...")
				var info repoInfo
				if deps.resolveRepo != nil {
					info, _ = deps.resolveRepo()
				}

				setLabel("Resolving pipeline topology...")
				// Single canonical read through project.Resolve — every
				// AGENTIC_* read in the check command now flows through
				// the resolver so the pipeline-side CheckDeps stay in
				// sync with project-side decisions.
				ctx, _ := project.Resolve(projectDeps)
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

				pipelineCheckDeps := doctor.CheckDeps{
					Root:                     root,
					RepoFullName:             info.FullName,
					Owner:                    info.Owner,
					RepoName:                 info.RepoName,
					OwnerType:                info.OwnerType,
					Topology:                 topology,
					ProjectID:                projectID,
					FrameworkVersion:         frameworkVersion,
					ProjectIDReadFailed:      projectIDReadFailed,
					Run:                      deps.run,
					ReadCreds:                deps.readCreds,
					FetchProjectTitle:        project.DefaultFetchProjectTitle,
					FetchProjectFields:       project.DefaultFetchProjectFields,
					UpdateStatusFieldOptions: project.DefaultUpdateStatusFieldOptions,
					FetchLinkedRepos:         project.DefaultFetchLinkedRepos,
					FetchOwnerAndRepoIDs:     project.DefaultFetchOwnerAndRepoIDs,
					// LinkRepoToProject intentionally omitted — check reads only;
					// repair wires it via buildPipelineCheckDeps.
					FrameworkSource: isFrameworkSource,
				}
				pipelineReport = doctor.RunAllChecksWithProgress(pipelineCheckDeps, setLabel)
				return nil
			})

			// Render combined output.
			fmt.Fprintln(w, "")
			fmt.Fprintln(w, "  "+ui.SectionHeading.Render("gh agentic — check"))
			fmt.Fprintln(w, "")

			renderCheckSections(w, isFrameworkSource, projectReport, pipelineReport, pipelineSkipped)

			// Combined summary.
			pipelineFails := 0
			pipelineWarns := 0
			if pipelineReport != nil {
				pipelineFails = pipelineReport.FailCount()
				pipelineWarns = pipelineReport.WarningCount()
			}
			projFails, projWarns := 0, 0
			if projectReport != nil {
				projFails = projectReport.FailCount()
				projWarns = projectReport.WarnCount()
			}
			fails := projFails + pipelineFails
			warns := projWarns + pipelineWarns
			fmt.Fprintln(w, "")
			switch {
			case fails > 0:
				fmt.Fprintf(w, "  %s\n\n", ui.StatusDanger.Render(fmt.Sprintf("%d check(s) failed, %d warning(s)", fails, warns)))
				return ErrSilent
			case warns > 0:
				fmt.Fprintf(w, "  %s\n\n", ui.StatusWarning.Render(fmt.Sprintf("%d warning(s)", warns)))
			default:
				fmt.Fprintf(w, "  %s\n\n", ui.StatusOK.Render("All checks passed"))
			}
			return nil
		},
	}
}
