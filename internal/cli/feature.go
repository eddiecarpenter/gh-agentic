package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/projectstatus"
)

// newFeatureCmd constructs the `gh agentic feature` command group.
func newFeatureCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "feature",
		Short: "Feature-level queries for the agentic pipeline",
		Long: `Feature-level helpers used by the pipeline and by humans.

Currently exposes 'target', which resolves the implementation repo a
control-plane Feature targets (its "Target repo" field), with a
single-topology fallback to the current repository.`,
		RunE: func(cmd *cobra.Command, args []string) error { return cmd.Help() },
	}
	cmd.AddCommand(newFeatureTargetCmd())
	return cmd
}

// newFeatureTargetCmd constructs `gh agentic feature target <N>` with
// production dependencies.
func newFeatureTargetCmd() *cobra.Command {
	return newFeatureTargetCmdWithDeps(defaultStatusDeps())
}

// newFeatureTargetCmdWithDeps builds the command with an explicit statusDeps
// for testing.
func newFeatureTargetCmdWithDeps(deps statusDeps) *cobra.Command {
	var raw bool
	cmd := &cobra.Command{
		Use:   "target <number>",
		Short: "Print the implementation repo a Feature targets",
		Long: `Resolve the implementation repository a Feature targets.

In a federation, a control-plane Feature records its target repo in the
"Target repo" ProjectV2 field; this command prints it as 'owner/repo' (the
owner is always the control-plane owner). When the field is unset — a
single-topology project — it resolves to the current repository.

The pipeline reads this (with --raw) to know which repo to clone into ./project.`,
		Example: `  # Human-readable
  gh agentic feature target 873

  # Scripting/CI — owner/repo only
  gh agentic feature target 873 --raw`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			n, err := parseIssueNumberArg(args[0])
			if err != nil {
				return err
			}
			if err := runFeatureTarget(cmd.OutOrStdout(), cmd.ErrOrStderr(), n, raw, deps); err != nil {
				return renderStatusError(cmd, err)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&raw, "raw", false, "print only owner/repo on stdout (for scripting/CI)")
	return cmd
}

// runFeatureTarget resolves and prints a Feature's target repo.
func runFeatureTarget(w io.Writer, stderr io.Writer, number int, raw bool, deps statusDeps) error {
	currentRepo, err := deps.currentRepo()
	if err != nil {
		return fmt.Errorf("resolving current repository: %w", err)
	}
	projectID, err := deps.resolveProjectID(currentRepo)
	if err != nil {
		return fmt.Errorf("reading AGENTIC_PROJECT_ID for %s: %w", currentRepo, err)
	}
	if projectID == "" {
		return projectstatus.ErrProjectNotConfigured
	}

	var feature *projectstatus.Feature
	err = deps.busy(stderr, fmt.Sprintf("Resolving target for feature #%d…", number), func() error {
		var fetchErr error
		feature, fetchErr = projectstatus.FetchFeature(deps.psDeps, projectID, number, currentRepo)
		return fetchErr
	})
	if err != nil {
		return annotateDetailError(err, number, currentRepo)
	}

	target := resolveFeatureTarget(feature.TargetRepo, currentRepo)
	if raw {
		fmt.Fprintln(w, target)
	} else {
		fmt.Fprintf(w, "Feature #%d targets: %s\n", number, target)
	}
	return nil
}

// resolveFeatureTarget composes the target owner/repo for a Feature. When the
// "Target repo" field is set it holds the bare repo name and the owner is the
// current (control-plane) repo's owner — the single-owner federation rule.
// When unset — single topology — the target is the current repo itself. A
// field value that already carries an owner is used verbatim (defensive).
func resolveFeatureTarget(targetRepoField, currentRepo string) string {
	field := strings.TrimSpace(targetRepoField)
	if field == "" {
		return currentRepo
	}
	if strings.Contains(field, "/") {
		return field
	}
	owner := currentRepo
	if i := strings.IndexByte(currentRepo, '/'); i >= 0 {
		owner = currentRepo[:i]
	}
	return owner + "/" + field
}
