package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/eddiecarpenter/gh-agentic/internal/project"
	"github.com/eddiecarpenter/gh-agentic/internal/projectstatus"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// runStatusFeatures is the handler for `gh agentic status features`. It
// resolves the project ID, fetches the list via projectstatus.FetchFeatures
// (which aggregates across every repo linked to the project), optionally
// narrows to the current repo, then renders:
//
//  1. --json — envelope {items, totals}; silently takes precedence over --kanban.
//  2. --kanban — stage-grouped view, vertical by default / side-by-side
//     when --horizontal is also set and the terminal is wide enough.
//  3. Default — compact tabular list.
func runStatusFeatures(w io.Writer, flags statusListFlags, deps statusDeps) error {
	if (flags.horizontal || flags.vertical) && !flags.kanban {
		return fmt.Errorf("--horizontal and --vertical require --kanban")
	}
	// Resolve layout early so mutually-exclusive flags fail fast before any
	// network call.
	var layout kanbanLayout
	if flags.kanban {
		var err error
		layout, err = resolveKanbanLayout(flags, terminalWidth(), featureKanbanMinWidth)
		if err != nil {
			return err
		}
	}

	currentRepo, err := deps.currentRepo()
	if err != nil {
		return fmt.Errorf("resolving current repository: %w", err)
	}

	projectID, err := deps.resolveProjectID(currentRepo)
	if err != nil {
		return fmt.Errorf("reading %s for %s: %w", project.ProjectVarName, currentRepo, err)
	}
	if projectID == "" {
		return projectstatus.ErrProjectNotConfigured
	}

	features, err := projectstatus.FetchFeatures(deps.psDeps, projectID, flags.includeDone)
	if err != nil {
		return fmt.Errorf("fetching features: %w", err)
	}

	if flags.thisRepo {
		features = filterFeaturesToRepo(features, currentRepo)
	}

	if flags.json {
		return writeFeaturesJSON(w, features)
	}

	if flags.kanban {
		unicode := ui.TerminalSupportsUTF8()
		columns := columnsForFeatures(flags.includeDone)
		cards := featureCards(features, columns, unicode)
		if layout.horizontal {
			return writeHorizontalKanban(w, columns, cards, terminalWidth(), featureKanbanMinWidth, unicode)
		}
		return writeVerticalKanban(w, "Features — Kanban", columns, cards, layout.notice)
	}

	return writeFeaturesTable(w, features, currentRepo)
}

// filterFeaturesToRepo drops features whose OwningRepo is not the current
// repo. Items with no owning-repo metadata are retained as "local" — the
// same defensive default used for requirements.
func filterFeaturesToRepo(features []projectstatus.Feature, currentRepo string) []projectstatus.Feature {
	out := make([]projectstatus.Feature, 0, len(features))
	for _, f := range features {
		if f.OwningRepo == "" || strings.EqualFold(f.OwningRepo, currentRepo) {
			out = append(out, f)
		}
	}
	return out
}

// writeFeaturesTable renders the UX-1 features table. The REPO column is
// added when at least one row is cross-repo; otherwise the table is three
// columns wide to match the single-repo case.
func writeFeaturesTable(w io.Writer, features []projectstatus.Feature, currentRepo string) error {
	showRepoCol := anyFeatureCrossRepo(features, currentRepo)

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if showRepoCol {
		fmt.Fprintln(tw, "FEATURE\tSTAGE\tTITLE\tREPO")
	} else {
		fmt.Fprintln(tw, "FEATURE\tSTAGE\tTITLE")
	}

	blocked := 0
	for _, f := range features {
		if f.Blocked != nil {
			blocked++
		}
		numberCol := fmt.Sprintf("#%d", f.Number)
		title := f.Title
		if f.Blocked != nil && f.Blocked.BlockingRef != "" {
			title = fmt.Sprintf("%s [blocked by %s]", f.Title, f.Blocked.BlockingRef)
		}
		stageCol := stageDisplay(f.Stage)
		if showRepoCol {
			repoCol := repoDisplay(f.OwningRepo, currentRepo)
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", numberCol, stageCol, title, repoCol)
		} else {
			fmt.Fprintf(tw, "%s\t%s\t%s\n", numberCol, stageCol, title)
		}
	}
	if err := tw.Flush(); err != nil {
		return fmt.Errorf("flushing table: %w", err)
	}

	fmt.Fprintln(w, "")
	fmt.Fprintln(w, featuresTotalsLine(len(features), blocked))
	return nil
}

// writeFeaturesJSON emits {items, totals} matching the documented schema.
func writeFeaturesJSON(w io.Writer, features []projectstatus.Feature) error {
	if features == nil {
		features = []projectstatus.Feature{}
	}
	for i := range features {
		if features[i].Tasks == nil {
			features[i].Tasks = []projectstatus.TaskRef{}
		}
	}
	envelope := projectstatus.ListEnvelope{
		Items:  features,
		Totals: countFeaturesTotals(features),
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(envelope); err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}
	return nil
}

// countFeaturesTotals computes the Open / Blocked counts for the envelope.
func countFeaturesTotals(features []projectstatus.Feature) projectstatus.ListTotals {
	blocked := 0
	for _, f := range features {
		if f.Blocked != nil {
			blocked++
		}
	}
	return projectstatus.ListTotals{Open: len(features), Blocked: blocked}
}

// anyFeatureCrossRepo reports whether any feature has an owning repo that
// differs from currentRepo — drives REPO column visibility.
func anyFeatureCrossRepo(features []projectstatus.Feature, currentRepo string) bool {
	for _, f := range features {
		if f.OwningRepo != "" && !strings.EqualFold(f.OwningRepo, currentRepo) {
			return true
		}
	}
	return false
}

// featuresTotalsLine is the summary rendered below the table.
func featuresTotalsLine(n, blocked int) string {
	noun := "features"
	if n == 1 {
		noun = "feature"
	}
	if blocked > 0 {
		return fmt.Sprintf("%d open %s (%d blocked)", n, noun, blocked)
	}
	return fmt.Sprintf("%d open %s", n, noun)
}
