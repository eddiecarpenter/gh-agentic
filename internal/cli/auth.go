package cli

import (
	"fmt"
	"os/exec"

	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/auth"
	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
	"github.com/eddiecarpenter/gh-agentic/internal/verify"
)

// authDeps holds injectable dependencies for the auth command.
type authDeps struct {
	run             auth.RunCommandFunc
	readCredentials auth.ReadCredentialsFunc
	claudeRefresh   auth.ClaudeRefreshFunc
}

// newAuthCmd constructs the `gh agentic -v2 auth` command with production deps.
func newAuthCmd() *cobra.Command {
	return newAuthCmdWithDeps(authDeps{
		run: bootstrap.DefaultRunCommand,
		readCredentials: func(run auth.RunCommandFunc) ([]byte, error) {
			return verify.ReadClaudeCredentialsDefault(run)
		},
		claudeRefresh: func() error {
			cmd := exec.Command("claude", "-p", "Say Hi")
			return cmd.Run()
		},
	})
}

// newAuthCmdWithDeps constructs the auth command with injectable dependencies.
func newAuthCmdWithDeps(deps authDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage Claude Code credentials (v2)",
		Long:  "Login, refresh, or check Claude Code credentials.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !v2FlagValue {
				return fmt.Errorf("auth requires the -v2 flag: gh agentic -v2 auth <subcommand>")
			}
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

	ownerType, err := bootstrap.DefaultDetectOwnerType(currentRepo.Owner)
	if err != nil {
		ownerType = bootstrap.OwnerTypeUser
	}

	return auth.Deps{
		Run:             deps.run,
		ReadCredentials: deps.readCredentials,
		ClaudeRefresh:   deps.claudeRefresh,
		RepoFullName:    currentRepo.Owner + "/" + currentRepo.Name,
		Owner:           currentRepo.Owner,
		OwnerType:       ownerType,
	}, nil
}

// newAuthLoginCmd creates the `auth login` subcommand.
func newAuthLoginCmd(deps authDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Force Claude Code login and push credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()
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
		Short: "Push current local credentials to repo secret",
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()
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
		Short: "Verify credentials are present and not expired",
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()
			authDeps, err := resolveAuthDeps(deps)
			if err != nil {
				return err
			}
			result, err := auth.Check(w, authDeps)
			if err != nil {
				return err
			}
			if !result.Valid {
				return ErrSilent
			}
			return nil
		},
	}
}
