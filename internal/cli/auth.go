package cli

import (
	"fmt"
	"os/exec"

	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/auth"
	"github.com/eddiecarpenter/gh-agentic/internal/project"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// authDeps holds injectable dependencies for the auth command.
type authDeps struct {
	run             auth.RunCommandFunc
	readCredentials auth.ReadCredentialsFunc
	claudeRefresh   auth.ClaudeRefreshFunc
}

// newAuthCmd constructs the `gh agentic --v2 auth` command with production deps.
func newAuthCmd() *cobra.Command {
	return newAuthCmdWithDeps(authDeps{
		run: auth.DefaultRunCommand,
		readCredentials: func(run auth.RunCommandFunc) ([]byte, error) {
			return auth.ReadClaudeCredentialsDefault(run)
		},
		claudeRefresh: func() error {
			cmd := exec.Command("claude", "-p", "Say Hi")
			return cmd.Run()
		},
	})
}

// controlPlaneAuthError prints the federated-CP block message and returns ErrSilent.
func controlPlaneAuthError(w interface{ Write([]byte) (int, error) }) error {
	fmt.Fprintf(w, "  %s  This repo is the federated control plane — auth commands are for repos that run Claude agents.\n", ui.StatusWarning.Render("⚠"))
	fmt.Fprintf(w, "       A federated control plane does not run Claude agents and does not need credentials.\n")
	return ErrSilent
}

// isFederatedControlPlane returns true only if this repo is the control plane
// of a federated setup — the one case where auth commands should be blocked
// because the repo does not itself run Claude agents.
//
// Single-topology repos are also "control planes" structurally, but they do run
// agents (the repo is CP + code combined), so they should not be blocked.
func isFederatedControlPlane() bool {
	deps, err := resolveProjectDeps()
	if err != nil {
		return false
	}
	projectID, err := deps.GetRepoVariable(deps.Owner, deps.RepoName, project.ProjectVarName)
	if err != nil || projectID == "" {
		return false
	}
	topoVal, _ := deps.GetRepoVariable(deps.Owner, deps.RepoName, project.TopologyVarName)
	if topoVal != "federated" {
		return false
	}
	linked, err := deps.FetchLinkedRepos(projectID)
	if err != nil {
		return false
	}
	// In a federated setup, the control plane is the repo that appears in the
	// project's linked repos (DetectTopology returns TopologySingle for it).
	return project.DetectTopology(deps.RepoFullName, linked) == project.TopologySingle
}

// newAuthCmdWithDeps constructs the auth command with injectable dependencies.
func newAuthCmdWithDeps(deps authDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage Claude Code credentials",
		Long: `Manage Claude Code credentials for domain repos that run Claude agents.

These commands are for domain repos only — the control plane does not run
Claude agents and does not need credentials.

  gh agentic auth login    — force a new Claude Code login and upload credentials
  gh agentic auth refresh  — upload current local credentials without re-logging in
  gh agentic auth check    — verify local credentials and the repo secret are in sync`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newAuthLoginCmd(deps))
	cmd.AddCommand(newAuthRefreshCmd(deps))
	cmd.AddCommand(newAuthCheckCmd(deps))

	return cmd
}

// resolveAuthDeps resolves the repo context and builds an auth.Deps.
func resolveAuthDeps(deps authDeps) (auth.Deps, error) {
	currentRepo, err := repository.Current()
	if err != nil {
		return auth.Deps{}, fmt.Errorf("resolving repo name: %w", err)
	}

	ownerType, err := auth.DefaultDetectOwnerType(currentRepo.Owner)
	if err != nil {
		ownerType = auth.OwnerTypeUser
	}

	return auth.Deps{
		Run:             deps.run,
		ReadCredentials: deps.readCredentials,
		ClaudeRefresh:   deps.claudeRefresh,
		CheckRepoSecret: auth.DefaultCheckRepoSecret,
		RepoFullName:    currentRepo.Owner + "/" + currentRepo.Name,
		Owner:           currentRepo.Owner,
		RepoName:        currentRepo.Name,
		OwnerType:       ownerType,
	}, nil
}

// newAuthLoginCmd creates the `auth login` subcommand.
func newAuthLoginCmd(deps authDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Force Claude Code login and upload credentials to repo secret",
		Long: `Force a new Claude Code login, then upload the refreshed credentials to the
CLAUDE_CREDENTIALS_JSON repo secret so that agents can authenticate.

Use this when credentials are missing or expired. For a credential refresh
without re-logging in, use 'auth refresh' instead.`,
		Example: `  gh agentic auth login`,
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()
			if isFederatedControlPlane() {
				return controlPlaneAuthError(w)
			}
			authDeps, err := resolveAuthDeps(deps)
			if err != nil {
				return err
			}
			return auth.Login(w, authDeps)
		},
	}
}

// newAuthRefreshCmd creates the `auth refresh` subcommand.
func newAuthRefreshCmd(deps authDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "refresh",
		Short: "Upload current local credentials to repo secret",
		Long: `Read the current local Claude Code credentials and upload them to the
CLAUDE_CREDENTIALS_JSON repo secret without triggering a new login.

Use this after a successful local Claude login to sync credentials to the repo,
or to push refreshed credentials when they have changed locally. Use 'auth login'
if your local credentials are missing or expired.`,
		Example: `  gh agentic auth refresh`,
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()
			if isFederatedControlPlane() {
				return controlPlaneAuthError(w)
			}
			authDeps, err := resolveAuthDeps(deps)
			if err != nil {
				return err
			}
			return auth.Refresh(w, authDeps)
		},
	}
}

// newAuthCheckCmd creates the `auth check` subcommand.
func newAuthCheckCmd(deps authDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Verify local credentials and repo secret are in sync",
		Long: `Check that local Claude Code credentials are valid and that the
CLAUDE_CREDENTIALS_JSON repo secret is set, then report whether they are in sync.

  Local valid + secret set     → in sync, no action needed
  Local valid + secret missing → run 'auth refresh' to upload
  Local missing/expired        → run 'auth login' to refresh`,
		Example: `  gh agentic auth check`,
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()

			if isFederatedControlPlane() {
				return controlPlaneAuthError(w)
			}

			authDeps, err := resolveAuthDeps(deps)
			if err != nil {
				return err
			}
			result, err := auth.Check(w, authDeps)
			if err != nil {
				return err
			}
			if !result.InSync {
				return ErrSilent
			}
			return nil
		},
	}
}
