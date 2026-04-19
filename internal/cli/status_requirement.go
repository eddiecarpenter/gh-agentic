package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/eddiecarpenter/gh-agentic/internal/projectstatus"
)

// parseIssueNumberArg converts the positional argument of a detail command
// into an integer, tolerating a leading `#` and surrounding whitespace.
func parseIssueNumberArg(raw string) (int, error) {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "#")
	n, err := strconv.Atoi(trimmed)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid issue number %q", raw)
	}
	return n, nil
}

// runStatusRequirement is the handler for `gh agentic status requirement <N>`.
// It resolves the project ID, fetches the requirement via projectstatus, and
// renders either the human detail view or a single-object JSON payload.
//
// stderr receives the busy-indicator rendered by deps.busy while the fetch
// is in flight; stdout (w) receives the final output. Non-TTY writers
// suppress the indicator — see ui.BusyRun.
func runStatusRequirement(w io.Writer, stderr io.Writer, number int, flags statusDetailFlags, deps statusDeps) error {
	currentRepo, err := deps.currentRepo()
	if err != nil {
		return fmt.Errorf("resolving current repository: %w", err)
	}

	projectID, err := deps.resolveProjectID(currentRepo)
	if err != nil {
		return fmt.Errorf("reading %s for %s: %w", "AGENTIC_PROJECT_ID", currentRepo, err)
	}
	if projectID == "" {
		return projectstatus.ErrProjectNotConfigured
	}

	var req *projectstatus.Requirement
	err = deps.busy(stderr, fmt.Sprintf("Fetching requirement #%d…", number), func() error {
		var fetchErr error
		req, fetchErr = projectstatus.FetchRequirement(deps.psDeps, projectID, number)
		return fetchErr
	})
	if err != nil {
		// Surface ErrIssueNotFound with the repo context so the renderer can
		// print the concrete "not found in <repo>" message. Likewise for
		// *ErrWrongType.
		return annotateDetailError(err, number, currentRepo)
	}

	if flags.json {
		return writeRequirementJSON(w, req)
	}
	return writeRequirementDetail(w, req)
}

// annotateDetailError wraps data-layer errors with enough context for the
// renderer (or later task #503) to produce a concrete user-facing message.
// ErrIssueNotFound gains the current repo; *ErrWrongType passes through
// (already carries Number / ActualType / WantedType).
func annotateDetailError(err error, number int, currentRepo string) error {
	if errors.Is(err, projectstatus.ErrIssueNotFound) {
		return fmt.Errorf("issue #%d not found in %s: %w", number, currentRepo, err)
	}
	return err
}

// writeRequirementDetail renders UX-3: title line, stage/dates summary,
// optional blocked line, body, separator, linked features block.
func writeRequirementDetail(w io.Writer, r *projectstatus.Requirement) error {
	fmt.Fprintln(w, r.Title)
	fmt.Fprintf(w, "Stage: %s    Created: %s    Last transition: %s\n",
		stageDisplay(r.Stage),
		formatISODate(r.CreatedAt),
		formatISODate(r.LastTransitionedAt),
	)
	if r.Blocked != nil {
		fmt.Fprintln(w, blockedDetailLine(r.Blocked))
	}
	fmt.Fprintln(w, "")
	if strings.TrimSpace(r.Body) != "" {
		fmt.Fprintln(w, r.Body)
	}
	fmt.Fprintln(w, "---")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Linked features:")
	if len(r.LinkedFeatures) == 0 {
		fmt.Fprintln(w, "  (none)")
		return nil
	}
	for _, f := range r.LinkedFeatures {
		fmt.Fprintf(w, "  #%-4d %-14s %s\n", f.Number, stageDisplay(f.Stage), f.Title)
		if f.BranchOneLiner != "" {
			fmt.Fprintf(w, "        branch: %s\n", f.BranchOneLiner)
		}
		if f.PR != nil {
			fmt.Fprintln(w, "        pr: "+formatPROneLiner(f.PR))
		}
	}
	return nil
}

// writeRequirementJSON emits the single-object payload with indentation for
// readability and Go's default time.Time RFC3339 serialisation.
func writeRequirementJSON(w io.Writer, r *projectstatus.Requirement) error {
	// Normalise nullable collections so consumers see [] instead of null for
	// LinkedFeatures — matches the list envelope convention.
	payload := *r
	if payload.LinkedFeatures == nil {
		payload.LinkedFeatures = []projectstatus.FeatureSummary{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}
	return nil
}

// formatISODate returns the ISO-8601 date portion of t. Zero times render as
// an empty string — UX prefers missing over "0001-01-01".
func formatISODate(t interface{ IsZero() bool; Format(string) string }) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

// blockedDetailLine renders the one-line blocked annotation used in detail
// views. When a reason is present the line reads "Blocked: <reason>"; when
// only a blocking ref is available the ref is shown verbatim.
func blockedDetailLine(b *projectstatus.BlockedInfo) string {
	if b == nil {
		return ""
	}
	switch {
	case b.Reason != "" && b.BlockingRef != "":
		return fmt.Sprintf("Blocked: %s (%s)", b.Reason, b.BlockingRef)
	case b.Reason != "":
		return "Blocked: " + b.Reason
	case b.BlockingRef != "":
		return "Blocked: " + b.BlockingRef
	default:
		return "Blocked"
	}
}

// formatPROneLiner produces the compact `#N (state) — reviewers: a, b`
// string used in linked-feature blocks. Reviewers are listed only when
// non-empty.
func formatPROneLiner(pr *projectstatus.PRState) string {
	if pr == nil {
		return ""
	}
	base := fmt.Sprintf("#%d (%s)", pr.Number, pr.State)
	if len(pr.Reviewers) > 0 {
		return base + " — reviewers: " + strings.Join(pr.Reviewers, ", ")
	}
	return base
}
