package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/mount"
	"github.com/eddiecarpenter/gh-agentic/internal/sync"
	"github.com/eddiecarpenter/gh-agentic/internal/tarball"
)

// mountDeps holds injectable dependencies for the mount command.
type mountDeps struct {
	fetchReleases mount.FetchReleasesFunc
	fetchTarball  mount.FetchTarballFunc
	confirm       mount.ConfirmFunc
}

// newMountCmd constructs the `gh agentic --v2 mount` subcommand with production deps.
func newMountCmd() *cobra.Command {
	return newMountCmdWithDeps(mountDeps{
		fetchReleases: sync.DefaultFetchReleases,
		fetchTarball:  tarball.DefaultFetch,
		confirm:       nil, // Set when needed for version switch.
	})
}

// newMountCmdWithDeps constructs the mount command with injectable dependencies.
func newMountCmdWithDeps(deps mountDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mount [version]",
		Short: "Mount the AI-Native Delivery Framework at .ai/ (v2)",
		Long: "Downloads and mounts the AI-Native Delivery Framework at the specified version.\n" +
			"First time: generates CLAUDE.md, AGENTS.md, wrapper workflows.\n" +
			"Version switch: prompts for confirmation, updates references.\n" +
			"No args: remounts at current .ai-version.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !v2FlagValue {
				return fmt.Errorf("mount requires the --v2 flag: gh agentic --v2 mount [version]")
			}

			w := cmd.OutOrStdout()

			// Determine repo root.
			root, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("resolving working directory: %w", err)
			}

			// Determine requested version.
			var version string
			if len(args) > 0 {
				version = args[0]
			}

			// Check for existing .ai-version.
			currentVersion, aiVersionErr := mount.ReadAIVersion(root)

			// If no version specified and no .ai-version, error.
			if version == "" && aiVersionErr != nil {
				return fmt.Errorf("no version specified and no .ai-version found — usage: gh agentic --v2 mount <version>")
			}

			// If no version specified, use current .ai-version (remount).
			if version == "" {
				version = currentVersion
			}

			// Fetch available releases and validate the tag.
			releases, err := deps.fetchReleases(mount.FrameworkRepo)
			if err != nil {
				return fmt.Errorf("fetching releases: %w", err)
			}

			if err := mount.ValidateTag(version, releases); err != nil {
				return err
			}

			// Route to appropriate flow.
			if aiVersionErr != nil {
				// No .ai-version: first-time flow.
				return mount.RunFirstTime(w, root, version, deps.fetchTarball)
			}

			if currentVersion == version {
				// Same version: remount flow.
				return mount.RunRemount(w, root, version, deps.fetchTarball)
			}

			// Different version: switch flow.
			return mount.RunSwitch(w, root, currentVersion, version, deps.fetchTarball, deps.confirm)
		},
	}
	return cmd
}
