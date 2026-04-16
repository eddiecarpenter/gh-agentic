package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/auth"
	"github.com/eddiecarpenter/gh-agentic/internal/doctorv2"
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

// doctorDeps holds injectable dependencies for the doctor command.
type doctorDeps struct {
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

// newDoctorCmd constructs the `gh agentic doctor` command with production deps.
func newDoctorCmd() *cobra.Command {
	return newDoctorCmdWithDeps(doctorDeps{
		run: auth.DefaultRunCommand,
		readCreds: func(run auth.RunCommandFunc) ([]byte, error) {
			return auth.ReadClaudeCredentialsDefault(run)
		},
		resolveRepo: defaultResolveRepo,
	})
}

// newDoctorCmdWithDeps constructs the v2 doctor command with injectable deps.
func newDoctorCmdWithDeps(deps doctorDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "doctor",
		Short:        "Check the health of the agentic framework environment",
		Long:         "Checks the health of this repo's agentic project membership and local framework setup.\nTopology-aware: detects Single, Federated control plane, or Federated domain repo.\n✓ pass  ⚠ warning (exit 0)  ✗ fail (exit 1) with remediation commands.",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()

			// Resolve repo root — local, no API call.
			root, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("resolving working directory: %w", err)
			}

			// Everything else (repo identity, topology, checks) runs inside the
			// spinner so the user sees feedback from the very first API call.
			var report *doctorv2.Report
			var topology string

			_ = ui.RunWithDynamicSpinner(w, "Detecting repository...", func(setLabel func(string)) error {
				// Resolve repo identity — makes an API call for owner type.
				var info repoInfo
				if deps.resolveRepo != nil {
					info, _ = deps.resolveRepo()
				}

				// Resolve topology — makes API calls for variables.
				projectID := ""
				if info.FullName != "" {
					setLabel("Detecting agentic project topology...")
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

				checkDeps := doctorv2.CheckDeps{
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

				report = doctorv2.RunAllChecksWithProgress(checkDeps, setLabel)
				return nil
			})

			doctorv2.RenderHeader(w, topology)
			for _, g := range report.Groups {
				doctorv2.RenderGroup(w, g)
			}
			doctorv2.RenderSummary(w, report.FailCount(), report.WarningCount())

			if report.HasFailures() {
				return ErrSilent
			}

			return nil
		},
	}
	return cmd
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
