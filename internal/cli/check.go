package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/auth"
	"github.com/eddiecarpenter/gh-agentic/internal/doctorv2"
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
	run         auth.RunCommandFunc
	readCreds   auth.ReadCredentialsFunc
	resolveRepo resolveRepoFunc
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

// newCheckCmd constructs the top-level `gh agentic check` command.
func newCheckCmd() *cobra.Command {
	return newCheckCmdWithDeps(checkDeps{
		run: auth.DefaultRunCommand,
		readCreds: func(run auth.RunCommandFunc) ([]byte, error) {
			return auth.ReadClaudeCredentialsDefault(run)
		},
		resolveRepo: defaultResolveRepo,
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

			projectDeps, err := resolveProjectDeps()
			if err != nil {
				return err
			}

			var projectReport *project.CheckReport
			var pipelineReport *doctorv2.Report

			_ = ui.RunWithDynamicSpinner(w, "Running project checks...", func(setLabel func(string)) error {
				// Project-scope checks first — they already handle topology internally.
				projectReport = project.RunChecksWithProgress(projectDeps, setLabel)

				// Pipeline-scope checks need repo identity + resolved topology mode.
				setLabel("Detecting repository for pipeline checks...")
				var info repoInfo
				if deps.resolveRepo != nil {
					info, _ = deps.resolveRepo()
				}

				setLabel("Resolving pipeline topology...")
				projectID := ""
				topology := ""
				if info.FullName != "" {
					projectID, _ = runGetVariable(deps.run, info.FullName, "AGENTIC_PROJECT_ID")
					if projectID != "" {
						topoVal, _ := runGetVariable(deps.run, info.FullName, "AGENTIC_TOPOLOGY")
						switch topoVal {
						case "federated":
							topology = resolveTopologyMode(deps.run, info.FullName)
						case "single":
							topology = "single"
						default:
							topology = resolveTopologyMode(deps.run, info.FullName)
							if topology == "federated-domain" {
								topology = "single"
							}
						}
					}
				}

				pipelineCheckDeps := doctorv2.CheckDeps{
					Root:         root,
					RepoFullName: info.FullName,
					Owner:        info.Owner,
					RepoName:     info.RepoName,
					OwnerType:    info.OwnerType,
					Topology:     topology,
					ProjectID:    projectID,
					Run:          deps.run,
					ReadCreds:    deps.readCreds,
				}
				pipelineReport = doctorv2.RunAllChecksWithProgress(pipelineCheckDeps, setLabel)
				return nil
			})

			// Render combined output.
			fmt.Fprintln(w, "")
			fmt.Fprintln(w, "  "+ui.SectionHeading.Render("gh agentic — check"))
			fmt.Fprintln(w, "")

			// Project section.
			fmt.Fprintln(w, "  "+ui.SectionHeading.Render("Project"))
			fmt.Fprintln(w, "  "+ui.Divider(48))
			for _, r := range projectReport.Results {
				fmt.Fprintf(w, "  %s  %s\n", project.StatusIcon(r.Status), r.Message)
				if r.Remediation != "" {
					fmt.Fprintf(w, "       %s\n", ui.Muted.Render("→ "+r.Remediation))
				}
			}
			fmt.Fprintln(w, "")

			// Pipeline section.
			fmt.Fprintln(w, "  "+ui.SectionHeading.Render("Pipeline"))
			fmt.Fprintln(w, "  "+ui.Divider(48))
			for _, g := range pipelineReport.Groups {
				doctorv2.RenderGroup(w, g)
			}

			// Combined summary.
			fails := projectReport.FailCount() + pipelineReport.FailCount()
			warns := projectReport.WarnCount() + pipelineReport.WarningCount()
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

// runGetVariable reads a GitHub repo variable value using the gh CLI.
func runGetVariable(run auth.RunCommandFunc, repoFullName, name string) (string, error) {
	if run == nil {
		return "", fmt.Errorf("no run func")
	}
	out, err := run("gh", "variable", "get", name, "--repo", repoFullName)
	return strings.TrimSpace(out), err
}

// resolveTopologyMode determines whether this federated repo is the control plane
// or a domain repo. The control plane is identified by the presence of
// AGENTIC_FRAMEWORK_VERSION — only the CP sets this variable.
func resolveTopologyMode(run auth.RunCommandFunc, repoFullName string) string {
	if run == nil {
		return "federated-domain"
	}
	out, err := run("gh", "variable", "get", "AGENTIC_FRAMEWORK_VERSION", "--repo", repoFullName)
	if err == nil && strings.TrimSpace(out) != "" {
		return "federated-cp"
	}
	return "federated-domain"
}
