package cli

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

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

	if flags.raw {
		return writeRequirementRaw(w, req, flags.verbose)
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

// rawDetailSeparator is the literal line emitted between the frontmatter
// header and the verbatim body for every detail-form `--raw` renderer. The
// separator is always present even when the body is empty, so agents can
// rely on the header / body split being unconditional.
const rawDetailSeparator = "---"

// writeRequirementRaw emits the agent-oriented frontmatter form of a
// requirement detail per the `--raw` contract:
//
//	number: 569
//	stage: ready-to-implement
//	title: ...
//	owning_repo: ...
//	blocked_by:
//	linked_features: 571 572
//	---
//	<body verbatim>
//
// Empty values render as an empty string after the colon (no `-`). The
// `---` separator is always present, even for an empty body, so agents
// can split on it without checking for body presence.
//
// When verbose is true, two additional header lines are inserted after
// `owning_repo` — `created_at` and `last_transitioned_at`, both ISO date.
func writeRequirementRaw(w io.Writer, r *projectstatus.Requirement, verbose bool) error {
	header := []struct {
		key   string
		value string
	}{
		{"number", fmt.Sprintf("%d", r.Number)},
		{"stage", string(r.Stage)},
		{"title", r.Title},
		{"owning_repo", r.OwningRepo},
	}
	if verbose {
		header = append(header,
			struct {
				key   string
				value string
			}{"created_at", rawDetailTimestamp(r.CreatedAt)},
			struct {
				key   string
				value string
			}{"last_transitioned_at", rawDetailTimestamp(r.LastTransitionedAt)},
		)
	}
	header = append(header,
		struct {
			key   string
			value string
		}{"blocked_by", rawDetailBlockedValue(r.Blocked)},
		struct {
			key   string
			value string
		}{"linked_features", rawLinkedFeaturesValue(r.LinkedFeatures)},
	)
	for _, kv := range header {
		if err := writeRawHeaderLine(w, kv.key, kv.value); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w, rawDetailSeparator); err != nil {
		return fmt.Errorf("writing raw separator: %w", err)
	}
	if r.Body != "" {
		if _, err := fmt.Fprint(w, r.Body); err != nil {
			return fmt.Errorf("writing raw body: %w", err)
		}
		// Ensure the body terminates with a newline so the final agent line
		// is well-formed; do not double up if the body already ends with \n.
		if !strings.HasSuffix(r.Body, "\n") {
			if _, err := fmt.Fprintln(w); err != nil {
				return fmt.Errorf("terminating raw body: %w", err)
			}
		}
	}
	return nil
}

// writeRawHeaderLine writes a single `key: value` header line. Empty
// values omit the trailing space — the rendered line is `key:` with no
// space — so agents can splitN(":", 2) without normalising the suffix.
func writeRawHeaderLine(w io.Writer, key, value string) error {
	var line string
	if value == "" {
		line = key + ":"
	} else {
		line = key + ": " + value
	}
	if _, err := fmt.Fprintln(w, line); err != nil {
		return fmt.Errorf("writing raw header: %w", err)
	}
	return nil
}

// rawDetailBlockedValue returns the value rendered for the `blocked_by`
// header field on detail-form raw output. Mirrors the BlockingRef when
// blocked, empty string otherwise — agents test for empty to detect
// "not blocked".
func rawDetailBlockedValue(b *projectstatus.BlockedInfo) string {
	if b == nil {
		return ""
	}
	return b.BlockingRef
}

// rawDetailTimestamp renders a time.Time as an ISO-8601 date for use in
// detail-form `--raw --verbose` headers. Mirrors formatISODate from the
// human detail view: zero times render as the empty string so the line
// reads `created_at:` (no trailing space) — agents test for empty to
// detect missing timestamps.
func rawDetailTimestamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

// rawLinkedFeaturesValue renders the `linked_features` header field as a
// space-separated list of feature numbers (no leading `#`). Empty when the
// requirement has no linked features.
func rawLinkedFeaturesValue(features []projectstatus.FeatureSummary) string {
	if len(features) == 0 {
		return ""
	}
	parts := make([]string, 0, len(features))
	for _, f := range features {
		parts = append(parts, fmt.Sprintf("%d", f.Number))
	}
	return strings.Join(parts, " ")
}

// formatISODate returns the ISO-8601 date portion of t. Zero times render as
// an empty string — UX prefers missing over "0001-01-01".
func formatISODate(t interface {
	IsZero() bool
	Format(string) string
}) string {
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
