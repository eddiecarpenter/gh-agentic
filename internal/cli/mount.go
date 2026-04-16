package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/mount"
	"github.com/eddiecarpenter/gh-agentic/internal/project"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// mountDeps holds injectable dependencies for the mount command.
type mountDeps struct {
	fetchReleases  mount.FetchReleasesFunc
	clone          mount.CloneFunc
	confirm        mount.ConfirmFunc
	resolveVersion func(root string) (string, error)
}

// newMountCmd constructs the `gh agentic mount` subcommand with production deps.
func newMountCmd() *cobra.Command {
	return newMountCmdWithDeps(mountDeps{
		fetchReleases: mount.DefaultFetchReleases,
		clone:         mount.DefaultClone,
		confirm:       nil,
		resolveVersion: func(root string) (string, error) {
			return resolveMountVersion(root)
		},
	})
}

// newMountCmdWithDeps constructs the mount command with injectable dependencies.
func newMountCmdWithDeps(deps mountDeps) *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "mount",
		Short: "Mount the AI-Native Delivery Framework at .ai/",
		Long: `Syncs the AI-Native Delivery Framework at .ai/ to the correct version.

For single topology repos, mounts at the version recorded in .ai-version.
For federated domain repos, reads AGENTIC_FRAMEWORK_VERSION from the control
plane and mounts that version — keeping all domain repos in sync automatically.

Use 'project switch version <version>' on the control plane repo to upgrade
the framework version for the whole federation.

Use --yes to skip the confirmation prompt when switching versions (for scripts).`,
		Example: `  # Sync to the correct version (restore, remount, or update)
  gh agentic mount

  # Skip confirmation when version switches (for scripts)
  gh agentic mount --yes`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()

			root, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("resolving working directory: %w", err)
			}

			// Resolve the target version — topology-aware.
			var version string
			if vErr := ui.RunWithSpinner(w, "Resolving framework version...", func() error {
				var resolveErr error
				version, resolveErr = deps.resolveVersion(root)
				return resolveErr
			}); vErr != nil {
				return vErr
			}

			currentVersion, aiVersionErr := mount.ReadAIVersion(root)

			var releases []mount.Release
			if rErr := ui.RunWithSpinner(w, "Fetching available releases...", func() error {
				var fetchErr error
				releases, fetchErr = deps.fetchReleases(mount.FrameworkRepo)
				return fetchErr
			}); rErr != nil {
				return fmt.Errorf("fetching releases: %w", rErr)
			}

			if err := mount.ValidateTag(version, releases); err != nil {
				return err
			}

			if aiVersionErr != nil {
				// No .ai-version — first-time mount.
				return mount.RunFirstTime(w, root, version, deps.clone)
			}

			if currentVersion == version {
				return mount.RunRemount(w, root, version, deps.clone)
			}

			confirm := deps.confirm
			if yes {
				confirm = nil
			}
			return mount.RunSwitch(w, root, currentVersion, version, deps.clone, confirm)
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation prompt on version switch")

	return cmd
}

// resolveMountVersion determines the correct framework version to mount.
// For federated domain repos: reads AGENTIC_FRAMEWORK_VERSION from the control plane.
// For all other cases: falls back to the local .ai-version.
func resolveMountVersion(root string) (string, error) {
	// Try to detect topology from repo context.
	currentRepo, err := repository.Current()
	if err != nil {
		// Not in a GitHub repo — fall back to local .ai-version.
		return localVersionFallback(root)
	}

	// Check if this repo is a federated domain repo.
	topology, _ := project.DefaultGetRepoVariable(currentRepo.Owner, currentRepo.Name, project.TopologyVarName)
	if topology != "federated" {
		// Single topology or control plane — use local .ai-version.
		return localVersionFallback(root)
	}

	// Federated domain repo — find the control plane and read its version.
	projectID, err := project.DefaultGetRepoVariable(currentRepo.Owner, currentRepo.Name, project.ProjectVarName)
	if err != nil || projectID == "" {
		return localVersionFallback(root)
	}

	linked, err := project.DefaultFetchLinkedRepos(projectID)
	if err != nil {
		return localVersionFallback(root)
	}

	cp, ok := project.ControlPlaneRepo(linked)
	if !ok {
		return localVersionFallback(root)
	}

	// Parse "owner/repo" from NameWithOwner.
	parts := strings.SplitN(cp.NameWithOwner, "/", 2)
	if len(parts) != 2 {
		return localVersionFallback(root)
	}

	cpVersion, err := project.DefaultGetRepoVariable(parts[0], parts[1], project.FrameworkVersionVarName)
	if err != nil || cpVersion == "" {
		return localVersionFallback(root)
	}

	return cpVersion, nil
}

func localVersionFallback(root string) (string, error) {
	v, err := mount.ReadAIVersion(root)
	if err != nil {
		return "", fmt.Errorf("no version found — run 'gh agentic project init' to set up this repo")
	}
	return v, nil
}
