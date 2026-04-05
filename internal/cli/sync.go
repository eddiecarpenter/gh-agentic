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
	run          bootstrap.RunCommandFunc
	fetchRelease sync.FetchReleaseFunc
	spinner      sync.SpinnerFunc
}

// newSyncCmd constructs the `gh agentic sync` subcommand with production defaults.
func newSyncCmd() *cobra.Command {
	return newSyncCmdWithDeps(syncDeps{
		run:          bootstrap.DefaultRunCommand,
		fetchRelease: sync.DefaultFetchRelease,
		spinner:      sync.DefaultSpinner,
	})
}

// newSyncCmdWithDeps constructs the `gh agentic sync` subcommand with the
// given dependencies. This allows tests to inject fakes for run, fetchRelease,
// and spinner without making real shell or network calls.
func newSyncCmdWithDeps(deps syncDeps) *cobra.Command {
	var force bool
	var yes bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync base/ from the upstream template",
		Long: "Syncs the base/ directory from the upstream agentic-development template.\n" +
			"Reads TEMPLATE_SOURCE and TEMPLATE_VERSION to determine what to sync.\n" +
			"Shows a diff and asks for confirmation before committing.\n" +
			"Pass --force to re-sync even when already at the latest version.\n" +
			"Pass --yes to automatically confirm all prompts.",
		RunE: func(cmd *cobra.Command, args []string) error {
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
				deps.fetchRelease,
				deps.spinner,
				confirmFn,
				force,
			)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "re-sync even if already at the latest version")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "automatically confirm all prompts")
	return cmd
}

// findRepoRoot walks up from the current working directory until it finds a
// directory containing TEMPLATE_SOURCE, which indicates the agentic repo root.
// Returns an error if not run inside an agentic repo.
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "TEMPLATE_SOURCE")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding TEMPLATE_SOURCE.
			return "", fmt.Errorf("not inside an agentic repo (no TEMPLATE_SOURCE found)")
		}
		dir = parent
	}
}
