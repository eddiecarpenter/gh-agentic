package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
	"github.com/eddiecarpenter/gh-agentic/internal/sync"
)

// syncDeps holds injectable dependencies for the sync command. Tests can
// supply fakes; the production path uses newSyncCmd which fills in real defaults.
type syncDeps struct {
	run             bootstrap.RunCommandFunc
	fetchReleases   sync.FetchReleasesFunc
	spinner         sync.SpinnerFunc
	selectVersion   sync.SelectFunc
	detectOwnerType bootstrap.DetectOwnerTypeFunc
}

// newSyncCmd constructs the `gh agentic sync` subcommand with production defaults.
func newSyncCmd() *cobra.Command {
	return newSyncCmdWithDeps(syncDeps{
		run:             bootstrap.DefaultRunCommand,
		fetchReleases:   sync.DefaultFetchReleases,
		spinner:         sync.DefaultSpinner,
		selectVersion:   sync.DefaultSelect,
		detectOwnerType: bootstrap.DefaultDetectOwnerType,
	})
}

// newSyncCmdWithDeps constructs the `gh agentic sync` subcommand with the
// given dependencies. This allows tests to inject fakes for run, fetchRelease,
// and spinner without making real shell or network calls.
func newSyncCmdWithDeps(deps syncDeps) *cobra.Command {
	var force bool
	var yes bool
	var commit bool
	var list bool
	var releaseTag string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync .ai/ from the upstream template",
		Long: "Syncs the .ai/ directory from the upstream agentic-development template.\n" +
			"Reads .ai/config.yml to determine the template source and current version.\n" +
			"Shows release notes and asks for confirmation before staging.\n" +
			"By default, changes are staged but not committed.\n" +
			"Pass --commit to automatically commit after staging.\n" +
			"Pass --force to re-sync even when already at the latest version.\n" +
			"Pass --yes to automatically confirm all prompts.\n" +
			"Pass --list to display available releases without syncing.\n" +
			"Pass --release <tag> to sync to a specific release version.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Block in v2 mode.
			if err := checkV2Guard("sync", &v2FlagValue); err != nil {
				return err
			}

			// Print deprecation notice to stderr.
			printDeprecationNotice(cmd.ErrOrStderr(), "sync")

			// Validate mutually exclusive flags.
			if list && releaseTag != "" {
				return fmt.Errorf("--list and --release are mutually exclusive")
			}

			w := cmd.OutOrStdout()

			// Detect repo root by walking up from cwd.
			repoRoot, err := findRepoRoot()
			if err != nil {
				repoRoot, err = os.Getwd()
				if err != nil {
					return fmt.Errorf("resolving working directory: %w", err)
				}
			}

			confirmFn := sync.DefaultConfirm
			if yes {
				confirmFn = func(prompt string) (bool, error) {
					fmt.Fprintf(w, "  %s [y/N]: y\n", prompt)
					return true, nil
				}
			}

			return sync.RunSync(
				w,
				repoRoot,
				deps.run,
				deps.fetchReleases,
				deps.spinner,
				confirmFn,
				deps.selectVersion,
				sync.DefaultClear,
				force,
				commit,
				list,
				releaseTag,
				deps.detectOwnerType,
			)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "re-sync even if already at the latest version")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "automatically confirm all prompts")
	cmd.Flags().BoolVar(&commit, "commit", false, "commit changes after staging (default is stage-only)")
	cmd.Flags().BoolVar(&list, "list", false, "display available releases without syncing")
	cmd.Flags().StringVar(&releaseTag, "release", "", "sync to a specific release version")
	return cmd
}

// findRepoRoot walks up from the current working directory until it finds a
// directory containing .ai/config.yml, which indicates the agentic repo root.
// Falls back to TEMPLATE_SOURCE for repos not yet migrated to v1.5.0+.
//
// TODO(deprecated): remove TEMPLATE_SOURCE fallback in next major version.
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}

	for {
		// Primary: .ai/config.yml (v1.5.0+).
		if _, err := os.Stat(filepath.Join(dir, ".ai", "config.yml")); err == nil {
			return dir, nil
		}

		// TODO(deprecated): remove in next major version — TEMPLATE_SOURCE fallback for pre-v1.5.0 repos.
		if _, err := os.Stat(filepath.Join(dir, "TEMPLATE_SOURCE")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not inside an agentic repo (no .ai/config.yml found)")
		}
		dir = parent
	}
}
