package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/mount"
)

// mountDeps holds injectable dependencies for the mount command.
type mountDeps struct {
	fetchReleases mount.FetchReleasesFunc
	clone         mount.CloneFunc
	confirm       mount.ConfirmFunc
}

// newMountCmd constructs the `gh agentic --v2 mount` subcommand with production deps.
func newMountCmd() *cobra.Command {
	return newMountCmdWithDeps(mountDeps{
		fetchReleases: mount.DefaultFetchReleases,
		clone:         mount.DefaultClone,
		confirm:       nil, // Set when needed for version switch.
	})
}

// newMountCmdWithDeps constructs the mount command with injectable dependencies.
func newMountCmdWithDeps(deps mountDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mount [version]",
		Short: "Mount the AI-Native Delivery Framework at .ai/",
		Long: "Clones the AI-Native Delivery Framework at the specified version tag into .ai/.\n" +
			"First time: generates CLAUDE.md, AGENTS.md, wrapper workflows.\n" +
			"Version switch: prompts for confirmation, updates references.\n" +
			"No args: remounts at current .ai-version.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()

			root, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("resolving working directory: %w", err)
			}

			var version string
			if len(args) > 0 {
				version = args[0]
			}

			currentVersion, aiVersionErr := mount.ReadAIVersion(root)

			if version == "" && aiVersionErr != nil {
				return fmt.Errorf("no version specified and no .ai-version found — usage: gh agentic --v2 mount <version>")
			}

			if version == "" {
				version = currentVersion
			}

			releases, err := deps.fetchReleases(mount.FrameworkRepo)
			if err != nil {
				return fmt.Errorf("fetching releases: %w", err)
			}

			if err := mount.ValidateTag(version, releases); err != nil {
				return err
			}

			if aiVersionErr != nil {
				return mount.RunFirstTime(w, root, version, deps.clone)
			}

			if currentVersion == version {
				return mount.RunRemount(w, root, version, deps.clone)
			}

			return mount.RunSwitch(w, root, currentVersion, version, deps.clone, deps.confirm)
		},
	}
	return cmd
}
