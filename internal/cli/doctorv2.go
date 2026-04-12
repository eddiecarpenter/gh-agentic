package cli

import (
	"fmt"
	"os"

	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/auth"
	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
	"github.com/eddiecarpenter/gh-agentic/internal/doctorv2"
	"github.com/eddiecarpenter/gh-agentic/internal/verify"
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

// doctorV2Deps holds injectable dependencies for the v2 doctor command.
type doctorV2Deps struct {
	run         bootstrap.RunCommandFunc
	readCreds   auth.ReadCredentialsFunc
	resolveRepo resolveRepoFunc
}

// defaultResolveRepo resolves the repo from git remote config.
func defaultResolveRepo() (repoInfo, error) {
	currentRepo, err := repository.Current()
	if err != nil {
		return repoInfo{}, err
	}

	ownerType, typeErr := bootstrap.DefaultDetectOwnerType(currentRepo.Owner)
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

// newDoctorV2Cmd constructs the `gh agentic -v2 doctor` command with production deps.
func newDoctorV2Cmd() *cobra.Command {
	return newDoctorV2CmdWithDeps(doctorV2Deps{
		run: bootstrap.DefaultRunCommand,
		readCreds: func(run auth.RunCommandFunc) ([]byte, error) {
			return verify.ReadClaudeCredentialsDefault(run)
		},
		resolveRepo: defaultResolveRepo,
	})
}

// newDoctorV2CmdWithDeps constructs the v2 doctor command with injectable deps.
func newDoctorV2CmdWithDeps(deps doctorV2Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor-v2",
		Short: "Health check with grouped output (v2)",
		Long: "Checks the AI-Native Delivery Framework health with grouped output.\n" +
			"Groups: Repository, Framework, Agent files, Workflows, Variables & secrets.\n" +
			"✓ pass, ⚠ warning (exit 0), ✗ fail (exit 1) with remediation commands.",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()

			// Resolve repo root.
			root, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("resolving working directory: %w", err)
			}

			// Resolve repo identity.
			var info repoInfo
			if deps.resolveRepo != nil {
				info, _ = deps.resolveRepo()
			}

			// Run all checks.
			checkDeps := doctorv2.CheckDeps{
				Root:         root,
				RepoFullName: info.FullName,
				Owner:        info.Owner,
				RepoName:     info.RepoName,
				OwnerType:    info.OwnerType,
				Run:          deps.run,
				ReadCreds:    deps.readCreds,
			}

			report := doctorv2.RunAllChecks(checkDeps)
			report.Render(w)

			if report.HasFailures() {
				return ErrSilent
			}

			return nil
		},
	}
	return cmd
}
