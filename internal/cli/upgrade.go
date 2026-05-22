package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/mount"
	"github.com/eddiecarpenter/gh-agentic/internal/project"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// newUpgradeCmd constructs the top-level `gh agentic upgrade` command.
// cliVersion is the version of this binary, used as the default target when
// no explicit version argument is supplied.
func newUpgradeCmd(cliVersion string) *cobra.Command {
	var yes, list bool

	cmd := &cobra.Command{
		Use:   "upgrade [version]",
		Short: "Change the framework version for the whole agentic project",
		Long: `Upgrade or downgrade the AI-Native Delivery Framework version.

When no version is given, defaults to the version of this CLI binary — the
CLI and framework are released together and are designed to run at the same
version. Pass an explicit version to pin to a specific release (e.g. to
downgrade or test a release candidate).

Only valid on the control plane repo. On a federated setup, changing the version
here broadcasts it to domain repos via AGENTIC_FRAMEWORK_VERSION — they pick it
up via 'gh agentic check'.

Blocked on federated domain repos — version governance flows through the
control plane only.

Use --list to browse available versions before choosing one.`,
		Example: `  # Upgrade to the version matching this CLI (most common)
  gh agentic upgrade

  # Upgrade to a specific version
  gh agentic upgrade v2.2.0

  # List available versions
  gh agentic upgrade --list

  # Skip confirmation (for scripts)
  gh agentic upgrade --yes`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()

			// Refuse on the framework source. --list is informational but
			// the whole "change the framework version" concept does not
			// apply when this repo IS the framework — consistency trumps
			// the informational sub-mode.
			root, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("resolving working directory: %w", err)
			}
			if err := refuseIfFrameworkSource(cmd, root, "upgrade"); err != nil {
				return err
			}

			if list {
				var releases []mount.Release
				_ = ui.RunWithSpinner(w, "Fetching available framework versions...", func() error {
					var err error
					releases, err = mount.DefaultFetchReleases(mount.FrameworkRepo)
					return err
				})
				if len(releases) == 0 {
					fmt.Fprintf(w, "  No releases found.\n")
					return nil
				}
				fmt.Fprintln(w, "")
				fmt.Fprintln(w, "  "+ui.SectionHeading.Render("Available framework versions"))
				fmt.Fprintln(w, "  "+ui.Divider(48))
				for _, r := range releases {
					fmt.Fprintf(w, "  %s\n", mount.TrimVPrefix(r.TagName))
				}
				fmt.Fprintln(w, "")
				return nil
			}

			// Resolve target version: explicit arg wins; fall back to CLI version.
			// Tags always carry a "v" prefix; GoReleaser strips it from the
			// binary version string, so we normalise before the tag lookup.
			target := ensureVPrefix(cliVersion)
			explicitVersion := len(args) > 0
			if explicitVersion {
				target = ensureVPrefix(args[0])
			} else {
				fmt.Fprintf(w, "  No version specified — using CLI version %s.\n", mount.TrimVPrefix(target))
			}

			deps, err := resolveProjectDeps()
			if err != nil {
				return err
			}

			// Confirm only when an explicit version was supplied — the user is
			// making a deliberate pin choice (upgrade or downgrade) and should
			// acknowledge the switch. When defaulting to the CLI version the
			// intent is unambiguous, so skip the prompt unless --yes was given
			// (which would be a no-op here anyway).
			var confirm func(string) (bool, error)
			if !yes && explicitVersion {
				confirm = deps.Confirm
			}

			// Preflight: resolve topology, validate version, read current state —
			// all behind a spinner so the user sees progress for every API call.
			var pre project.SwitchVersionPreflight
			var preflightErr error
			_ = ui.RunWithDynamicSpinner(w, "Detecting topology...", func(setLabel func(string)) error {
				setLabel("Fetching framework releases...")
				pre, preflightErr = project.PreflightSwitchVersion(deps, target)
				return nil
			})
			if preflightErr != nil {
				return preflightErr
			}
			return project.SwitchVersion(w, deps, target, pre, confirm)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation prompt")
	cmd.Flags().BoolVarP(&list, "list", "l", false, "list available framework versions")
	return cmd
}

// ensureVPrefix normalises a version string to have a "v" prefix.
// GoReleaser injects {{.Version}} (e.g. "2.6.2") into the binary, but git
// tags and GitHub releases use the full tag name (e.g. "v2.6.2").
func ensureVPrefix(v string) string {
	if strings.HasPrefix(v, "v") {
		return v
	}
	return "v" + v
}
