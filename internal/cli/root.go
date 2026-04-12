// Package cli defines the cobra command tree for gh-agentic.
// All command definitions live here; business logic is delegated to
// internal sub-packages (bootstrap, inception, sync).
package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

// ErrSilent is returned by subcommands that have already printed a
// user-friendly error message. main.go checks for it and exits non-zero
// without printing anything further.
var ErrSilent = errors.New("silent exit")

// v2Flag is the persistent flag pointer shared between root and subcommands.
// It is set to true when the user passes -v2 on the command line.
var v2FlagValue bool

// newRootCmd constructs a fresh root cobra command. Called by Execute and in
// tests so that each invocation starts from a clean command tree.
func newRootCmd(version, date string) *cobra.Command {
	// Reset the package-level flag for each new root command (important for tests).
	v2FlagValue = false

	root := &cobra.Command{
		Use:           "gh-agentic",
		Short:         "Agentic software delivery — environment management for gh",
		Long:          "gh-agentic bootstraps and manages agentic software delivery environments via the GitHub CLI.",
		Version:       version,
		SilenceErrors: true,
	}

	// Register -v2 as a persistent flag on the root command.
	root.PersistentFlags().BoolVar(&v2FlagValue, "v2", false, "use v2 command implementations")

	// v1 commands.
	root.AddCommand(newBootstrapCmd())
	root.AddCommand(newInceptionCmd())
	root.AddCommand(newSyncCmd())
	root.AddCommand(newVersionCmd(version, date))
	doctorCmd := newDoctorCmd()
	root.AddCommand(doctorCmd)
	// "verify" is a hidden alias for backwards compatibility.
	verifyAlias := *doctorCmd
	verifyAlias.Use = "verify"
	verifyAlias.Hidden = true
	root.AddCommand(&verifyAlias)

	// v2 commands — available regardless of -v2 flag, but only useful with it.
	root.AddCommand(newMountCmd())
	root.AddCommand(newInitStubCmd())
	root.AddCommand(newAuthStubCmd())
	root.AddCommand(newDoctorV2StubCmd())

	return root
}

// newInitStubCmd creates a stub init command for v2.
func newInitStubCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialise a new agentic environment (v2)",
		Long:  "Interactive wizard to configure a new agentic environment.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !v2FlagValue {
				return fmt.Errorf("init requires the -v2 flag: gh agentic -v2 init")
			}
			fmt.Fprintln(cmd.OutOrStdout(), "init: not yet implemented")
			return nil
		},
	}
	cmd.Flags().Bool("force", false, "overwrite existing configuration")
	return cmd
}

// newAuthStubCmd creates a stub auth command for v2.
func newAuthStubCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage Claude Code credentials (v2)",
		Long:  "Login, refresh, or check Claude Code credentials.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !v2FlagValue {
				return fmt.Errorf("auth requires the -v2 flag: gh agentic -v2 auth")
			}
			fmt.Fprintln(cmd.OutOrStdout(), "auth: not yet implemented")
			return nil
		},
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "login",
		Short: "Force Claude Code login and push credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "auth login: not yet implemented")
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "refresh",
		Short: "Push current local credentials to repo secret",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "auth refresh: not yet implemented")
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "check",
		Short: "Verify credentials are present and not expired",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "auth check: not yet implemented")
			return nil
		},
	})
	return cmd
}

// newDoctorV2StubCmd creates a stub doctor-v2 command.
// Named "doctor-v2" to avoid conflict with existing doctor command.
// Will be renamed to "doctor" when v2 ships as default.
func newDoctorV2StubCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "doctor-v2",
		Short:  "Health check with grouped output (v2)",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "doctor-v2: not yet implemented")
			return nil
		},
	}
}

// Execute builds and runs the root command. Called by main.go.
func Execute(version, date string) error {
	return newRootCmd(version, date).Execute()
}
