package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/eddiecarpenter/gh-agentic/internal/projectstatus"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// runStatusFeature is the handler for `gh agentic status feature <N>`. It
// resolves the project ID, fetches the feature via projectstatus, and
// renders either the human detail view or the self-contained JSON object.
func runStatusFeature(w io.Writer, number int, flags statusDetailFlags, deps statusDeps) error {
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

	feature, err := projectstatus.FetchFeature(deps.psDeps, projectID, number)
	if err != nil {
		return annotateDetailError(err, number, currentRepo)
	}

	if flags.json {
		return writeFeatureJSON(w, feature)
	}
	return writeFeatureDetail(w, feature, ui.TerminalSupportsUTF8())
}

// writeFeatureDetail renders the UX-3 feature detail view: title line,
// stage/dates summary, optional Blocked line, body, separator, then the
// structured fields (parent requirement, branch, PR, tasks).
func writeFeatureDetail(w io.Writer, f *projectstatus.Feature, utf8 bool) error {
	fmt.Fprintln(w, f.Title)
	fmt.Fprintf(w, "Stage: %s    Created: %s    Last transition: %s\n",
		stageDisplay(f.Stage),
		formatISODate(f.CreatedAt),
		formatISODate(f.LastTransitionedAt),
	)
	if f.Blocked != nil {
		fmt.Fprintln(w, blockedDetailLine(f.Blocked))
	}
	fmt.Fprintln(w, "")
	if strings.TrimSpace(f.Body) != "" {
		fmt.Fprintln(w, f.Body)
	}
	fmt.Fprintln(w, "---")
	fmt.Fprintln(w, "")

	// Parent requirement.
	if f.ParentRequirement != nil {
		fmt.Fprintf(w, "Parent requirement:  %s\n", parentRequirementOneLiner(f.ParentRequirement))
	} else {
		fmt.Fprintln(w, "Parent requirement:  (none)")
	}

	// Branch.
	if f.Branch != nil && f.Branch.Exists {
		fmt.Fprintf(w, "Branch:              %s\n", renderFeatureBranchOneLiner(f.Branch))
	} else {
		fmt.Fprintln(w, "Branch:              (no branch yet)")
	}

	// PR.
	if f.PR != nil {
		fmt.Fprintf(w, "PR:                  %s\n", formatPROneLiner(f.PR))
	} else {
		fmt.Fprintln(w, "PR:                  (no PR opened)")
	}

	// Tasks — include a done-count summary and a checklist.
	done := 0
	for _, t := range f.Tasks {
		if t.Closed {
			done++
		}
	}
	fmt.Fprintf(w, "Tasks:               %d / %d done\n", done, len(f.Tasks))
	glyphDone := "✓"
	glyphOpen := "☐"
	if !utf8 {
		glyphDone = "[x]"
		glyphOpen = "[ ]"
	}
	for _, t := range f.Tasks {
		glyph := glyphOpen
		if t.Closed {
			glyph = glyphDone
		}
		fmt.Fprintf(w, "  %s  #%d  %s\n", glyph, t.Number, t.Title)
	}
	return nil
}

// writeFeatureJSON emits the single-object payload with nullable collections
// normalised so consumers see [] instead of null.
func writeFeatureJSON(w io.Writer, f *projectstatus.Feature) error {
	payload := *f
	if payload.Tasks == nil {
		payload.Tasks = []projectstatus.TaskRef{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}
	return nil
}

// parentRequirementOneLiner renders the `#466 [done]  feat: ...` form used
// next to the "Parent requirement:" label.
func parentRequirementOneLiner(p *projectstatus.RequirementSummary) string {
	if p == nil {
		return ""
	}
	stage := stageDisplay(p.Stage)
	return fmt.Sprintf("#%d [%s]  %s", p.Number, stage, p.Title)
}

// renderFeatureBranchOneLiner renders the `feature/<N>-<slug> (merged)`
// form used next to the "Branch:" label in feature detail.
func renderFeatureBranchOneLiner(b *projectstatus.BranchState) string {
	if b == nil || !b.Exists {
		return ""
	}
	if b.Merged {
		return fmt.Sprintf("%s (merged)", b.Name)
	}
	return b.Name
}

