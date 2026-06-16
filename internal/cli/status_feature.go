package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/eddiecarpenter/gh-agentic/internal/projectstatus"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// runStatusFeature is the handler for `gh agentic status feature <N>`. It
// resolves the project ID, fetches the feature via projectstatus, and
// renders either the human detail view or the agent-oriented --raw form.
// The --json flag was removed by feature #589.
//
// stderr receives the busy-indicator rendered by deps.busy while the fetch
// is in flight; stdout (w) receives the final output. Non-TTY writers
// suppress the indicator — see ui.BusyRun.
func runStatusFeature(w io.Writer, stderr io.Writer, number int, flags statusDetailFlags, deps statusDeps) error {
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
	err = deps.busy(stderr, fmt.Sprintf("Fetching feature #%d…", number), func() error {
		var fetchErr error
		feature, fetchErr = projectstatus.FetchFeature(deps.psDeps, projectID, number)
		return fetchErr
	})
	if err != nil {
		return annotateDetailError(err, number, currentRepo)
	}

	if flags.raw {
		return writeFeatureRaw(w, feature, flags.verbose)
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
	// Emit a warning when the owning repo was unreachable (branch, PR, or
	// sub-issue fetch failed). The feature itself still renders; only the
	// branch/PR/task sections may be incomplete.
	if f.OwningRepoError != "" {
		fmt.Fprintf(w, "⚠ %s\n", f.OwningRepoError)
	}
	return nil
}

// writeFeatureRaw emits the agent-oriented frontmatter form of a feature
// detail per the `--raw` contract:
//
//	number: 492
//	stage: in-development
//	title: ...
//	owning_repo: ...
//	blocked_by:
//	parent_requirement: 457
//	branch: feature/492 (merged)
//	pr: 777 (open)
//	tasks_done_total: 3/6
//	---
//	<body verbatim>
//
// Empty values render as an empty string after the colon (no `-`). The
// `---` separator is always present, even when the body is empty.
//
// When verbose is true, two header lines (`created_at`,
// `last_transitioned_at`, ISO date) are inserted after `owning_repo` —
// matching the position used by the requirement detail raw renderer.
func writeFeatureRaw(w io.Writer, f *projectstatus.Feature, verbose bool) error {
	header := []struct {
		key   string
		value string
	}{
		{"number", fmt.Sprintf("%d", f.Number)},
		{"stage", string(f.Stage)},
		{"title", f.Title},
		{"owning_repo", f.OwningRepo},
		{"target_repo", rawTargetRepoValue(f.TargetRepo)},
	}
	if verbose {
		header = append(header,
			struct {
				key   string
				value string
			}{"created_at", rawDetailTimestamp(f.CreatedAt)},
			struct {
				key   string
				value string
			}{"last_transitioned_at", rawDetailTimestamp(f.LastTransitionedAt)},
		)
	}
	header = append(header,
		struct {
			key   string
			value string
		}{"blocked_by", rawDetailBlockedValue(f.Blocked)},
		struct {
			key   string
			value string
		}{"parent_requirement", rawParentRequirementValue(f.ParentRequirement)},
		struct {
			key   string
			value string
		}{"branch", rawBranchValue(f.Branch)},
		struct {
			key   string
			value string
		}{"pr", rawPRValue(f.PR)},
		struct {
			key   string
			value string
		}{"tasks_done_total", rawTasksDoneTotalValue(f.Tasks)},
	)
	for _, kv := range header {
		if err := writeRawHeaderLine(w, kv.key, kv.value); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w, rawDetailSeparator); err != nil {
		return fmt.Errorf("writing raw separator: %w", err)
	}
	if f.Body != "" {
		if _, err := fmt.Fprint(w, f.Body); err != nil {
			return fmt.Errorf("writing raw body: %w", err)
		}
		if !strings.HasSuffix(f.Body, "\n") {
			if _, err := fmt.Fprintln(w); err != nil {
				return fmt.Errorf("terminating raw body: %w", err)
			}
		}
	}
	return nil
}

// rawParentRequirementValue renders the parent-requirement number for the
// `parent_requirement` header field, or an empty string when the feature
// has no parent requirement on the project board.
// rawTargetRepoValue renders the Target repo field value; an unset target is
// reported as "(unset)" so consumers never assume a repo (#872, AC-2).
func rawTargetRepoValue(target string) string {
	if strings.TrimSpace(target) == "" {
		return "(unset)"
	}
	return target
}

func rawParentRequirementValue(p *projectstatus.RequirementSummary) string {
	if p == nil {
		return ""
	}
	return fmt.Sprintf("%d", p.Number)
}

// rawBranchValue renders the `branch` header field. When the branch exists
// the value is `<name>` or `<name> (merged)`; when no branch exists the
// value is empty so agents can detect "no branch yet".
func rawBranchValue(b *projectstatus.BranchState) string {
	if b == nil || !b.Exists {
		return ""
	}
	if b.Merged {
		return fmt.Sprintf("%s (merged)", b.Name)
	}
	return b.Name
}

// rawPRValue renders the `pr` header field as `<number> (<state>)`. Empty
// when no PR is associated with the feature.
func rawPRValue(pr *projectstatus.PRState) string {
	if pr == nil {
		return ""
	}
	return fmt.Sprintf("%d (%s)", pr.Number, pr.State)
}

// rawTasksDoneTotalValue counts closed tasks and renders the `done/total`
// fraction documented in the feature detail header. Renders `0/0` when the
// feature has no tasks — agents distinguish "no tasks" from "all done"
// by inspecting the denominator.
func rawTasksDoneTotalValue(tasks []projectstatus.TaskRef) string {
	done := 0
	for _, t := range tasks {
		if t.Closed {
			done++
		}
	}
	return fmt.Sprintf("%d/%d", done, len(tasks))
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
