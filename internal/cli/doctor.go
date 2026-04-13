package cli

import (
	"fmt"
	"os"

	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/auth"
	"github.com/eddiecarpenter/gh-agentic/internal/doctorv2"
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
		Use:   "doctor",
		Short: "Check the health of the agentic framework environment",
		Long: "Checks the AI-Native Delivery Framework health with grouped output.\n" +
			"Groups: Repository, Framework, Agent files, Workflows, Variables & secrets.\n" +
			"✓ pass, ⚠ warning (exit 0), ✗ fail (exit 1) with remediation commands.",
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

			checkDeps := doctorv2.CheckDeps{
				Root:         root,
				RepoFullName: info.FullName,
				Owner:        info.Owner,
				RepoName:     info.RepoName,
				OwnerType:    info.OwnerType,
				Run:          deps.run,
				ReadCreds:    deps.readCreds,
			}

			// Stream results — print each group as its checks complete.
			doctorv2.RenderHeader(w)
			report := doctorv2.StreamAllChecks(w, checkDeps)

			doctorv2.RenderSummary(w, report.FailCount(), report.WarningCount())

			if report.HasFailures() {
				return ErrSilent
			}

			return nil
		},
	}
	return cmd
}
