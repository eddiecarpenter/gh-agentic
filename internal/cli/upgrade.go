package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/mount"
	"github.com/eddiecarpenter/gh-agentic/internal/project"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// newUpgradeCmd constructs the top-level `gh agentic upgrade` command.
func newUpgradeCmd() *cobra.Command {
	var yes, list bool

	cmd := &cobra.Command{
		Use:   "upgrade [version]",
		Short: "Change the framework version for the whole agentic project",
		Long: `Upgrade or downgrade the AI-Native Delivery Framework version.

Only valid on the control plane repo. On a federated setup, changing the version
here broadcasts it to domain repos via AGENTIC_FRAMEWORK_VERSION — they pick it
up on the next 'gh agentic mount' or 'gh agentic check'.

Blocked on federated domain repos — version governance flows through the
control plane only. "Upgrade" is used generically here: specifying an older
version downgrades the federation.

Use --list to browse available versions before choosing one.`,
		Example: `  # List available versions
  gh agentic upgrade --list

  # Upgrade to a specific version
  gh agentic upgrade v2.2.0

  # Skip confirmation (for scripts)
  gh agentic upgrade v2.2.0 --yes`,
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
			if err := refuseIfFrameworkSource(root, "upgrade"); err != nil {
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
					fmt.Fprintf(w, "  %s\n", r.TagName)
				}
				fmt.Fprintln(w, "")
				return nil
			}

			if len(args) == 0 {
				return cmd.Help()
			}

			deps, err := resolveProjectDeps()
			if err != nil {
				return err
			}

			var confirm func(string) (bool, error)
			if !yes {
				confirm = deps.Confirm
			}

			// Preflight: resolve topology, validate version, read current state —
			// all behind a spinner so the user sees progress for every API call.
			var pre project.SwitchVersionPreflight
			var preflightErr error
			_ = ui.RunWithDynamicSpinner(w, "Detecting topology...", func(setLabel func(string)) error {
				setLabel("Fetching framework releases...")
				pre, preflightErr = project.PreflightSwitchVersion(deps, args[0])
				return nil
			})
			if preflightErr != nil {
				return preflightErr
			}
			return project.SwitchVersion(w, deps, args[0], pre, confirm)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation prompt")
	cmd.Flags().BoolVarP(&list, "list", "l", false, "list available framework versions")
	return cmd
}
