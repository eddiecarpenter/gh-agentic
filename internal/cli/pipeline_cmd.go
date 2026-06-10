package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/projectstatus"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// pipelineFlags captures the flag surface of the `gh agentic status pipeline`
// command. Selector flags (--requirements, --features) are mutually
// exclusive; the remaining fields mirror the inherited pipeline layout
// flags so users can invoke the command with the same semantics they
// already know from the earlier list-flag form.
type pipelineFlags struct {
	requirements bool
	features     bool
	horizontal   bool
	vertical     bool
	includeDone  bool
	thisRepo     bool
	raw          bool
	verbose      bool
}

// registerPipelineFlags declares every flag the pipeline command accepts.
// Declared once so the scaffold and the real handler share the same
// surface area, and so tests can look up any flag by name without
// depending on internal wiring detail.
func registerPipelineFlags(cmd *cobra.Command, f *pipelineFlags) {
	cmd.Flags().BoolVar(&f.requirements, "requirements", false, "render only the requirements pipeline (mutually exclusive with --features)")
	cmd.Flags().BoolVar(&f.features, "features", false, "render only the features pipeline (mutually exclusive with --requirements)")
	cmd.Flags().BoolVar(&f.horizontal, "horizontal", false, "force horizontal pipeline regardless of terminal width")
	cmd.Flags().BoolVar(&f.vertical, "vertical", false, "force vertical pipeline regardless of terminal width")
	cmd.Flags().BoolVar(&f.includeDone, "include-done", false, "include items in the 'done' stage")
	cmd.Flags().BoolVar(&f.thisRepo, "this-repo", false, "narrow the view to the current repository only")
	cmd.Flags().BoolVar(&f.raw, "raw", false, "emit agent-oriented raw output (tab-separated sections per selector); --horizontal/--vertical are no-ops under --raw")
	cmd.Flags().BoolVar(&f.verbose, "verbose", false, "include timestamps in --raw output (no-op without --raw)")
}

// newPipelineCmd constructs the `gh agentic status pipeline` command with
// production dependencies. Tests use newPipelineCmdWithDeps to inject fakes.
func newPipelineCmd() *cobra.Command {
	return newPipelineCmdWithDeps(defaultStatusDeps())
}

// newPipelineCmdWithDeps builds the pipeline command with an explicit
// statusDeps — the same dependency bundle used by the status sub-commands,
// because the command reads the same project board and reuses the same
// renderer helpers.
func newPipelineCmdWithDeps(deps statusDeps) *cobra.Command {
	var flags pipelineFlags
	cmd := &cobra.Command{
		Use:   "pipeline",
		Short: "Show the agentic pipeline — requirements and features together",
		Long: `Render the agentic project's pipeline view.

Bare invocation renders the requirements pipeline first, a blank-line
separator, then the features pipeline, then a combined totals line. Use
--requirements or --features (mutually exclusive) to render only one of
the two pipelines.

--horizontal forces horizontal layout, --vertical forces vertical, and
omitting both auto-picks based on terminal width. --include-done appends
the 'done' column. --this-repo narrows the federated view to the current
repository.

Pass --raw for an agent-oriented payload — two TSV sections (one per
selector, '# requirements' and '# features' markers) with the same row
shape as the list commands. Layout flags are no-ops under --raw.`,
		Example: `  # Both pipelines stacked
  gh agentic status pipeline

  # Requirements only
  gh agentic status pipeline --requirements

  # Features only, horizontal layout, include closed features
  gh agentic status pipeline --features --horizontal --include-done

  # Agent-oriented raw TSV (combined sections)
  gh agentic status pipeline --raw`,
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.requirements && flags.features {
				return fmt.Errorf("--requirements and --features are mutually exclusive")
			}
			if err := runPipeline(cmd.OutOrStdout(), cmd.ErrOrStderr(), flags, deps); err != nil {
				return renderStatusError(cmd, err)
			}
			return nil
		},
	}

	registerPipelineFlags(cmd, &flags)
	return cmd
}

// runPipeline is the command handler for `gh agentic status pipeline`. It
// resolves the project ID, fetches requirements and/or features (as selected
// by the selector flags) through deps.busy so long-running federated queries
// show a delayed indicator on stderr, then renders either the agent-oriented
// `--raw` form or the stacked / selector human pipeline view. The `--json`
// flag was removed by feature #589.
//
// stdout (w) receives the final output. stderr receives the busy
// indicator rendered by deps.busy while the fetch is in flight.
func runPipeline(w io.Writer, stderr io.Writer, flags pipelineFlags, deps statusDeps) error {
	// Resolve layout early for each pipeline so mutually-exclusive flags
	// fail fast before any network call. The widths differ by entity
	// (requirements fit in 100 columns; features need 120) so each
	// pipeline has its own resolution.
	//
	// `--raw` skips layout resolution because its renderer is layout-free;
	// `--horizontal` and `--vertical` are documented as no-ops under
	// `--raw`.
	var reqLayout, feaLayout pipelineLayout
	if !flags.raw {
		if flags.requirements || !flags.features {
			var err error
			reqLayout, err = resolvePipelineLayout(pipelineToStatusListFlags(flags), terminalWidth(), requirementPipelineMinWidth)
			if err != nil {
				return err
			}
		}
		if flags.features || !flags.requirements {
			var err error
			feaLayout, err = resolvePipelineLayout(pipelineToStatusListFlags(flags), terminalWidth(), featurePipelineMinWidth)
			if err != nil {
				return err
			}
		}
	}

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

	// Decide which pipelines to fetch. With no selector, fetch both.
	fetchReqs := flags.requirements || !flags.features
	fetchFeats := flags.features || !flags.requirements

	var reqs []projectstatus.Requirement
	var features []projectstatus.Feature
	label := pipelineBusyLabel(fetchReqs, fetchFeats)
	err = deps.busy(stderr, label, func() error {
		if fetchReqs {
			r, e := projectstatus.FetchRequirements(deps.psDeps, projectID, flags.includeDone)
			if e != nil {
				return fmt.Errorf("fetching requirements: %w", e)
			}
			reqs = r
		}
		if fetchFeats {
			f, e := projectstatus.FetchFeatures(deps.psDeps, projectID, flags.includeDone)
			if e != nil {
				return fmt.Errorf("fetching features: %w", e)
			}
			features = f
		}
		return nil
	})
	if err != nil {
		return err
	}

	if flags.thisRepo {
		if fetchReqs {
			reqs = filterRequirementsToRepo(reqs, currentRepo)
		}
		if fetchFeats {
			features = filterFeaturesToRepo(features, currentRepo)
		}
	}

	if flags.raw {
		return writePipelineRaw(w, reqs, features, fetchReqs, fetchFeats, flags.verbose)
	}

	return writePipelineHuman(w, reqs, features, fetchReqs, fetchFeats, flags, reqLayout, feaLayout)
}

// pipelineToStatusListFlags bridges the pipeline-command flag struct to the
// statusListFlags shape expected by the existing resolvePipelineLayout
// helper. Only the layout-relevant fields are copied across; the legacy
// `kanban` field is set true to satisfy the resolver's precondition check
// (which is a no-op on the pipeline command since every invocation renders
// a pipeline).
func pipelineToStatusListFlags(f pipelineFlags) statusListFlags {
	return statusListFlags{
		kanban:      true,
		horizontal:  f.horizontal,
		vertical:    f.vertical,
		thisRepo:    f.thisRepo,
		includeDone: f.includeDone,
	}
}

// pipelineBusyLabel returns the message displayed by the busy indicator
// while the pipeline fetch is in flight. The label is specialised per
// selector so the user sees exactly which data is being loaded — "both
// pipelines" gets the generic phrase documented in UX-4 of the scope.
func pipelineBusyLabel(fetchReqs, fetchFeats bool) string {
	switch {
	case fetchReqs && fetchFeats:
		return "Fetching pipeline state…"
	case fetchReqs:
		return "Fetching requirements…"
	case fetchFeats:
		return "Fetching features…"
	default:
		// Unreachable — RunE rejects the both-selectors case and at
		// least one is always true in the bare / single-selector paths.
		return "Fetching…"
	}
}

// writePipelineHuman renders the text pipeline view(s) to w per the UX-1 /
// UX-2 specification. When both pipelines are selected the requirements
// pipeline is rendered first, followed by a blank-line separator, then
// the features pipeline, then a combined totals line. Selector-scoped
// invocations render only the relevant pipeline with its own totals line.
func writePipelineHuman(w io.Writer, reqs []projectstatus.Requirement, features []projectstatus.Feature, includeReqs, includeFeats bool, flags pipelineFlags, reqLayout, feaLayout pipelineLayout) error {
	unicode := ui.TerminalSupportsUTF8()

	if includeReqs {
		cols := columnsForRequirements(flags.includeDone)
		cards := requirementCards(reqs, cols)
		if reqLayout.horizontal {
			if err := writeHorizontalPipelineWithHeading(w, "Requirements — Pipeline", cols, cards, terminalWidth(), requirementPipelineMinWidth, unicode); err != nil {
				return err
			}
		} else {
			if err := writeVerticalPipeline(w, "Requirements — Pipeline", cols, cards, reqLayout.notice); err != nil {
				return err
			}
		}
	}

	if includeReqs && includeFeats {
		// Blank-line separator between the two pipelines. The === heading
		// on the features pipeline is sufficient visual separation so we
		// emit a single empty line rather than a decorative rule.
		fmt.Fprintln(w, "")
	}

	if includeFeats {
		cols := columnsForFeatures(flags.includeDone)
		cards := featureCards(features, cols, unicode)
		if feaLayout.horizontal {
			if err := writeHorizontalPipelineWithHeading(w, "Features — Pipeline", cols, cards, terminalWidth(), featurePipelineMinWidth, unicode); err != nil {
				return err
			}
		} else {
			if err := writeVerticalPipeline(w, "Features — Pipeline", cols, cards, feaLayout.notice); err != nil {
				return err
			}
		}
	}

	// Totals footer. Each branch emits the appropriate wording; the
	// combined line sums blocked counts across both lists.
	fmt.Fprintln(w, "")
	switch {
	case includeReqs && includeFeats:
		fmt.Fprintln(w, combinedTotalsLine(reqs, features))
	case includeReqs:
		fmt.Fprintln(w, requirementsTotalsLine(len(reqs), blockedCountRequirements(reqs)))
	case includeFeats:
		fmt.Fprintln(w, featuresTotalsLine(len(features), blockedCountFeatures(features)))
	}
	// Emit repo-error warnings collected across both lists so the user can see
	// which repos were unreachable without inspecting individual items.
	for _, r := range reqs {
		if r.LinkedFeaturesError != "" {
			fmt.Fprintf(w, "⚠ %s\n", r.LinkedFeaturesError)
		}
	}
	for _, f := range features {
		if f.OwningRepoError != "" {
			fmt.Fprintf(w, "⚠ %s\n", f.OwningRepoError)
		}
	}
	return nil
}

// writeHorizontalPipelineWithHeading prints the `=== <heading> ===`
// banner then delegates to the existing writeHorizontalPipeline renderer.
// writeVerticalPipeline already emits its own heading; this wrapper brings
// the horizontal path to the same visual contract used by the stacked
// pipeline output.
func writeHorizontalPipelineWithHeading(w io.Writer, heading string, columns []projectstatus.Stage, cards map[projectstatus.Stage][]pipelineCard, actualWidth, minWidth int, unicode bool) error {
	fmt.Fprintln(w, "=== "+heading+" ===")
	fmt.Fprintln(w, "")
	return writeHorizontalPipeline(w, columns, cards, actualWidth, minWidth, unicode)
}

// writePipelineRaw emits the agent-oriented combined / selector form of the
// pipeline per the `--raw` contract:
//
//	# requirements
//	<list TSV header>
//	<requirement rows>
//	<blank line>
//	# features
//	<list TSV header>
//	<feature rows>
//
// The selector form (--requirements or --features) drops the irrelevant
// section entirely — no marker, no trailing blank line. The per-section
// row shape is delegated to writeRequirementsRaw / writeFeaturesRaw so the
// pipeline `--raw` row layout cannot drift from the list-command goldens.
func writePipelineRaw(w io.Writer, reqs []projectstatus.Requirement, features []projectstatus.Feature, includeReqs, includeFeats, verbose bool) error {
	if includeReqs {
		if _, err := fmt.Fprintln(w, "# requirements"); err != nil {
			return fmt.Errorf("writing raw requirements marker: %w", err)
		}
		if err := writeRequirementsRaw(w, reqs, verbose); err != nil {
			return err
		}
	}
	if includeReqs && includeFeats {
		// Single blank line between the two sections — the `# features`
		// marker that follows is sufficient visual separation; never use
		// glyphs / borders under --raw.
		if _, err := fmt.Fprintln(w); err != nil {
			return fmt.Errorf("writing raw section separator: %w", err)
		}
	}
	if includeFeats {
		if _, err := fmt.Fprintln(w, "# features"); err != nil {
			return fmt.Errorf("writing raw features marker: %w", err)
		}
		if err := writeFeaturesRaw(w, features, verbose); err != nil {
			return err
		}
	}
	return nil
}

// blockedCountRequirements counts requirements with a non-nil Blocked
// annotation. Shared between the human and JSON renderers so the totals
// footer and the envelope Totals.Blocked field agree.
func blockedCountRequirements(reqs []projectstatus.Requirement) int {
	n := 0
	for _, r := range reqs {
		if r.Blocked != nil {
			n++
		}
	}
	return n
}

// blockedCountFeatures is the feature-side counterpart to
// blockedCountRequirements.
func blockedCountFeatures(features []projectstatus.Feature) int {
	n := 0
	for _, f := range features {
		if f.Blocked != nil {
			n++
		}
	}
	return n
}

// combinedTotalsLine renders the bottom line of the stacked default
// view: `N open requirement(s) · M open feature(s)` with an optional
// `(K blocked)` suffix when either list contains a blocked item. The
// blocked count is the sum across both lists, matching §2 of the
// feature scope.
func combinedTotalsLine(reqs []projectstatus.Requirement, features []projectstatus.Feature) string {
	rNoun := "requirements"
	if len(reqs) == 1 {
		rNoun = "requirement"
	}
	fNoun := "features"
	if len(features) == 1 {
		fNoun = "feature"
	}
	blocked := blockedCountRequirements(reqs) + blockedCountFeatures(features)
	base := fmt.Sprintf("%d open %s · %d open %s", len(reqs), rNoun, len(features), fNoun)
	if blocked > 0 {
		return fmt.Sprintf("%s (%d blocked)", base, blocked)
	}
	return base
}
