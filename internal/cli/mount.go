package cli

import (
	"fmt"
	"os"

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
	resolveCP      func(root string) string
	syncCP         mount.CPSyncFunc
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
		resolveCP: resolveFederatedCP,
		syncCP:    mount.DefaultCPSync,
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
Federated domain repos additionally get a read-only .cp/ mount — a sparse
checkout of the control plane repo's docs/ tracking main — providing
system-level knowledge.

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

			// Detect first-time vs remount vs switch by inspecting the
			// current .ai/ mount. ReadAIVersionFromGit returns an error
			// when the directory or its git metadata is absent — that
			// is the first-time signal now that the flat .ai-version
			// file is gone (#585).
			currentVersion, aiVersionErr := mount.ReadAIVersionFromGit(root)

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
				if err := mount.RunFirstTime(w, root, version, deps.clone); err != nil {
					return err
				}
			} else if currentVersion == version {
				if err := mount.RunRemount(w, root, version, deps.clone); err != nil {
					return err
				}
			} else {
				confirm := deps.confirm
				if yes {
					confirm = nil
				}
				if err := mount.RunSwitch(w, root, currentVersion, version, deps.clone, confirm); err != nil {
					return err
				}
			}

			// Federated domain repos also need the control plane knowledge mount.
			if deps.resolveCP != nil {
				if cpNameWithOwner := deps.resolveCP(root); cpNameWithOwner != "" {
					if err := mount.MountControlPlane(w, root, cpNameWithOwner, deps.syncCP); err != nil {
						return err
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation prompt on version switch")

	return cmd
}

// resolveMountVersion determines the correct framework version to mount
// through the canonical project.Resolve resolver. The resolver's
// FrameworkVersion field already encodes the per-topology precedence:
//   - single / federated-cp → AGENTIC_FRAMEWORK_VERSION on this repo
//   - federated-domain      → AGENTIC_FRAMEWORK_VERSION on the CP repo
//
// When the authoritative version is unset (e.g. first-time single mount
// before any version has been pinned) we fall back to the local .ai-version
// file — that fallback disappears in task #585 once the file is removed.
func resolveMountVersion(root string) (string, error) {
	ctx, err := resolveContextForMount(root)
	if err != nil {
		return localVersionFallback(root)
	}
	if ctx.FrameworkVersion != "" {
		return ctx.FrameworkVersion, nil
	}
	return localVersionFallback(root)
}

// resolveFederatedCP returns the control plane repo (owner/name) when the
// current directory is a federated domain repo. Returns "" for any other
// topology or any lookup failure — callers treat empty as "no .cp/ mount
// needed".
//
// Delegates to project.Resolve so topology detection stays on the single
// canonical code path.
func resolveFederatedCP(root string) string {
	ctx, err := resolveContextForMount(root)
	if err != nil {
		return ""
	}
	if !ctx.IsFederatedDomain() {
		return ""
	}
	return ctx.ControlPlane.NameWithOwner
}

// resolveContextForMount builds the project.Deps for the current repo and
// delegates to project.Resolve. Kept as a package-level helper so both the
// version resolver and the CP resolver share the same single read path.
func resolveContextForMount(root string) (*project.Context, error) {
	deps, err := resolveProjectDeps()
	if err != nil {
		return nil, err
	}
	// resolveProjectDeps hardcodes os.Getwd() as Root; mount is called from a
	// possibly-different root (for tests), so we override explicitly.
	deps.Root = root
	return project.Resolve(deps)
}

// localVersionFallback returns the version the current .ai/ mount is
// pinned to, derived from the clone's .git metadata. With the flat
// .ai-version file removed (#585), this is the only local fallback when
// the resolver produces no authoritative AGENTIC_FRAMEWORK_VERSION.
func localVersionFallback(root string) (string, error) {
	v, err := mount.ReadAIVersionFromGit(root)
	if err != nil {
		return "", fmt.Errorf("no version found — run 'gh agentic project init' to set up this repo")
	}
	return v, nil
}
