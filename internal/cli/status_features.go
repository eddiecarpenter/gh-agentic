package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/eddiecarpenter/gh-agentic/internal/projectstatus"
)

// runStatusFeatures is the handler for `gh agentic status features`. It
// resolves the project ID, fetches the list via projectstatus.FetchFeatures
// (which aggregates across every repo linked to the project), optionally
// narrows to the current repo, then renders either the --json envelope or
// the compact tabular list.
//
// The legacy --kanban flag was removed by feature #518. If the caller
// passes --kanban (hidden on this command for interception), the handler
// returns errPipelineCommandRenamed pointing at `gh agentic status
// pipeline --features` — feature #549 moved the sub-command under
// `status` and feature #562 renamed it from `kanban` to `pipeline`.
//
// stderr receives the busy-indicator rendered by deps.busy while the
// federated fetch is in flight; stdout (w) receives the final output.
// Non-TTY writers suppress the indicator — see ui.BusyRun.
func runStatusFeatures(w io.Writer, stderr io.Writer, flags statusListFlags, deps statusDeps) error {
	if flags.kanban {
		return &errPipelineCommandRenamed{suggestedCommand: "gh agentic status pipeline --features"}
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

	// Wrap the federated feature fetch in the shared busy indicator.
	// The indicator writes to stderr so stdout stays clean for --json
	// consumers; non-TTY writers suppress the glyphs entirely.
	var features []projectstatus.Feature
	err = deps.busy(stderr, "Fetching features…", func() error {
		var fetchErr error
		features, fetchErr = projectstatus.FetchFeatures(deps.psDeps, projectID, flags.includeDone)
		return fetchErr
	})
	if err != nil {
		return fmt.Errorf("fetching features: %w", err)
	}

	if flags.thisRepo {
		features = filterFeaturesToRepo(features, currentRepo)
	}

	if flags.json {
		return writeFeaturesJSON(w, features)
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

// writeFeaturesTable renders the UX-1 features table. Every row carries a
// compact TASKS column (values like `3/6`) so the list and the pipeline
// views surface equivalent progress information — the pipeline shows a
// block-bar glyph, the list shows just the numeric per AC-8. The REPO
// column is added when at least one row is cross-repo; otherwise the
// table is four columns wide (FEATURE / STAGE / TASKS / TITLE).
func writeFeaturesTable(w io.Writer, features []projectstatus.Feature, currentRepo string) error {
	showRepoCol := anyFeatureCrossRepo(features, currentRepo)

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if showRepoCol {
		fmt.Fprintln(tw, "FEATURE\tSTAGE\tTASKS\tTITLE\tREPO")
	} else {
		fmt.Fprintln(tw, "FEATURE\tSTAGE\tTASKS\tTITLE")
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
		tasksCol := fmt.Sprintf("%d/%d", f.TasksDone, f.TasksTotal)
		if showRepoCol {
			repoCol := repoDisplay(f.OwningRepo, currentRepo)
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", numberCol, stageCol, tasksCol, title, repoCol)
		} else {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", numberCol, stageCol, tasksCol, title)
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
