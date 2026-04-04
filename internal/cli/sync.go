package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
	"github.com/eddiecarpenter/gh-agentic/internal/sync"
)

// newSyncCmd constructs the `gh agentic sync` subcommand.
func newSyncCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync base/ from the upstream template",
		Long: "Syncs the base/ directory from the upstream agentic-development template.\n" +
			"Reads TEMPLATE_SOURCE and TEMPLATE_VERSION to determine what to sync.\n" +
			"Shows a diff and asks for confirmation before committing.\n" +
			"Pass --force to re-sync even when already at the latest version.",
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

			return sync.RunSync(
				w,
				repoRoot,
				bootstrap.DefaultRunCommand,
				sync.DefaultFetchRelease,
				sync.DefaultSpinner,
				sync.DefaultConfirm,
				force,
			)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "re-sync even if already at the latest version")
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
