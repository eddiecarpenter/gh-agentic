package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/cli/go-gh/v2/pkg/repository"
	"golang.org/x/term"

	"github.com/eddiecarpenter/gh-agentic/internal/project"
	"github.com/eddiecarpenter/gh-agentic/internal/projectstatus"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// terminalWidth returns the width in columns of the attached terminal, or
// the fallback when os.Stdout is not a terminal (redirected output, CI).
// It is a package-level var so tests can substitute a deterministic value.
var terminalWidth = func() int {
	if width, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && width > 0 {
		return width
	}
	return 80 // conservative fallback when output is piped
}

// statusDeps bundles the resources every status sub-command needs. It is
// injectable so tests can supply deterministic fakes — defaultStatusDeps()
// wires the production clients.
type statusDeps struct {
	// currentRepo returns the `owner/name` of the repository the command
	// was invoked from.
	currentRepo func() (string, error)
	// resolveProjectID returns the GitHub ProjectV2 node ID configured for
	// the given repo. An empty return with nil error signals that no
	// AGENTIC_PROJECT_ID is set; a non-nil error signals an I/O failure.
	resolveProjectID func(repoFullName string) (string, error)
	// psDeps is the data-layer Deps consumed by projectstatus queries.
	psDeps projectstatus.Deps
	// busy wraps a long-running fetch with a delayed, non-TTY-guarded
	// busy indicator rendered on the stderr writer passed at call time.
	// Production wires ui.BusyRun; tests use testutil.NoopBusy to stay
	// silent and deterministic.
	busy ui.BusyFunc
}

// defaultStatusDeps returns production dependencies wired to gh auth.
func defaultStatusDeps() statusDeps {
	return statusDeps{
		currentRepo:      defaultCurrentRepoFullName,
		resolveProjectID: defaultResolveProjectID,
		psDeps:           projectstatus.DefaultDeps(),
		busy:             ui.BusyRun,
	}
}

// defaultCurrentRepoFullName resolves the current repository via the gh
// library's repository.Current() helper (reads git remote config).
func defaultCurrentRepoFullName() (string, error) {
	r, err := repository.Current()
	if err != nil {
		return "", err
	}
	return r.Owner + "/" + r.Name, nil
}

// defaultResolveProjectID resolves the GitHub ProjectV2 node ID for the
// given repository through the canonical project.Resolve resolver. An empty
// string with nil error signals that no project affiliation is configured;
// a non-nil error means the underlying resolve failed (auth / network).
//
// This routes through project.Resolve rather than reading the AGENTIC_*
// variable directly so every status sub-command consumes project state from
// the same single source of truth as mount, check, and repair.
func defaultResolveProjectID(repoFullName string) (string, error) {
	parts := strings.SplitN(repoFullName, "/", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid repo full name %q", repoFullName)
	}

	root, _ := os.Getwd()
	deps := project.DefaultDeps(parts[0], parts[1], root)
	ctx, err := project.Resolve(deps)
	if err != nil {
		return "", err
	}
	if ctx == nil {
		return "", nil
	}
	return ctx.ProjectID, nil
}

// runStatusRequirements is the handler for `gh agentic status requirements`.
// It resolves the project ID, fetches the list via projectstatus, optionally
// narrows to the current repo, then renders either the --raw TSV form or
// the compact tabular human list. The --json flag was removed by feature
// #589 in favour of the agent-oriented --raw shape.
//
// The legacy --kanban flag was removed by feature #518. If the caller
// passes --kanban (hidden on this command for interception), the handler
// returns errPipelineCommandRenamed pointing at `gh agentic status
// pipeline --requirements` — feature #549 moved the sub-command under
// `status` and feature #562 renamed it from `kanban` to `pipeline`.
//
// stderr receives the busy-indicator rendered by deps.busy while the
// fetch is in flight; stdout (w) receives the final output. Non-TTY
// writers suppress the indicator — see ui.BusyRun.
func runStatusRequirements(w io.Writer, stderr io.Writer, flags statusListFlags, deps statusDeps) error {
	if flags.kanban {
		return &errPipelineCommandRenamed{suggestedCommand: "gh agentic status pipeline --requirements"}
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

	// Wrap the network-bound fetch in the shared busy indicator. The
	// indicator writes to stderr so --raw consumers piping stdout get
	// clean output; non-TTY writers suppress the glyphs entirely.
	var reqs []projectstatus.Requirement
	err = deps.busy(stderr, "Fetching requirements…", func() error {
		var fetchErr error
		reqs, fetchErr = projectstatus.FetchRequirements(deps.psDeps, projectID, flags.includeDone)
		return fetchErr
	})
	if err != nil {
		return fmt.Errorf("fetching requirements: %w", err)
	}

	if flags.thisRepo {
		reqs = filterRequirementsToRepo(reqs, currentRepo)
	}

	if flags.raw {
		return writeRequirementsRaw(w, reqs, flags.verbose)
	}

	return writeRequirementsTable(w, reqs, currentRepo)
}

// filterRequirementsToRepo returns only the requirements owned by currentRepo.
// Items with an empty OwningRepo are assumed local (defensive default when
// the project board omits repository metadata).
func filterRequirementsToRepo(reqs []projectstatus.Requirement, currentRepo string) []projectstatus.Requirement {
	out := make([]projectstatus.Requirement, 0, len(reqs))
	for _, r := range reqs {
		if r.OwningRepo == "" || strings.EqualFold(r.OwningRepo, currentRepo) {
			out = append(out, r)
		}
	}
	return out
}

// writeRequirementsTable renders the compact human table defined in UX-1 of
// feature #492: fixed column order (REQUIREMENT, STAGE, TITLE), optional REPO
// column when any row's owning repo differs from the current repo, totals
// line at the bottom.
func writeRequirementsTable(w io.Writer, reqs []projectstatus.Requirement, currentRepo string) error {
	showRepoCol := anyRequirementCrossRepo(reqs, currentRepo)

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if showRepoCol {
		fmt.Fprintln(tw, "REQUIREMENT\tSTAGE\tTITLE\tREPO")
	} else {
		fmt.Fprintln(tw, "REQUIREMENT\tSTAGE\tTITLE")
	}

	blocked := 0
	for _, r := range reqs {
		if r.Blocked != nil {
			blocked++
		}
		title := r.Title
		if r.Blocked != nil && r.Blocked.BlockingRef != "" {
			title = fmt.Sprintf("%s [blocked by %s]", r.Title, r.Blocked.BlockingRef)
		}
		numberCol := fmt.Sprintf("#%d", r.Number)
		stageCol := stageDisplay(r.Stage)
		if showRepoCol {
			repoCol := repoDisplay(r.OwningRepo, currentRepo)
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", numberCol, stageCol, title, repoCol)
		} else {
			fmt.Fprintf(tw, "%s\t%s\t%s\n", numberCol, stageCol, title)
		}
	}
	if err := tw.Flush(); err != nil {
		return fmt.Errorf("flushing table: %w", err)
	}
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, requirementsTotalsLine(len(reqs), blocked))
	return nil
}

// rawListSeparator is the column separator used by every list-form `--raw`
// renderer. Tab is the contract — agents split on it; never change.
const rawListSeparator = "\t"

// rawAbsentValue is the sentinel rendered into a list-form `--raw` cell when
// the value is empty / unknown. Agents test for `-` rather than empty strings
// because empty strings collide with the field separator on parse.
const rawAbsentValue = "-"

// rawField normalises a list-form cell value: empty strings render as the
// sentinel `-`, whitespace is preserved verbatim. The TSV contract forbids
// tabs and newlines inside a cell; both are stripped to spaces so the
// columns remain stable for `awk -F\\t` or equivalent.
func rawField(v string) string {
	if v == "" {
		return rawAbsentValue
	}
	v = strings.ReplaceAll(v, "\t", " ")
	v = strings.ReplaceAll(v, "\n", " ")
	return v
}

// rawBlockedField returns the blocked-by ref string for the list-form raw
// renderers. Mirrors the existing `Blocked.BlockingRef` value when present;
// emits `-` otherwise so every cell has a value.
func rawBlockedField(b *projectstatus.BlockedInfo) string {
	if b == nil || b.BlockingRef == "" {
		return rawAbsentValue
	}
	return b.BlockingRef
}

// writeRequirementsRaw emits the agent-oriented TSV form of the requirements
// list per the `--raw` contract:
//
//	number<TAB>stage<TAB>title<TAB>blocked_by<TAB>owning_repo
//
// Header row first, one row per item, no presentation glyphs / colours /
// borders, and no trailing totals line. Sparse cells render as `-`. The
// column order is frozen — any reshuffle would break every agent
// consumer that splits on `\t`.
//
// When verbose is true, the header gains `created_at` and
// `last_transitioned_at` columns at the end (in that order, ISO date) so
// agents that need timing pay only for the bytes they ask for.
func writeRequirementsRaw(w io.Writer, reqs []projectstatus.Requirement, verbose bool) error {
	cols := []string{"number", "stage", "title", "blocked_by", "owning_repo"}
	if verbose {
		cols = append(cols, "created_at", "last_transitioned_at")
	}
	if _, err := fmt.Fprintln(w, strings.Join(cols, rawListSeparator)); err != nil {
		return fmt.Errorf("writing raw header: %w", err)
	}
	for _, r := range reqs {
		row := []string{
			fmt.Sprintf("%d", r.Number),
			rawField(string(r.Stage)),
			rawField(r.Title),
			rawBlockedField(r.Blocked),
			rawField(r.OwningRepo),
		}
		if verbose {
			row = append(row, rawTimestampField(r.CreatedAt), rawTimestampField(r.LastTransitionedAt))
		}
		if _, err := fmt.Fprintln(w, strings.Join(row, rawListSeparator)); err != nil {
			return fmt.Errorf("writing raw row: %w", err)
		}
	}
	return nil
}

// rawTimestampField renders a time.Time as an ISO-8601 date (YYYY-MM-DD)
// for inclusion in `--raw --verbose` output. Zero times render as the
// absent sentinel `-` so the column count stays stable across rows.
func rawTimestampField(t time.Time) string {
	if t.IsZero() {
		return rawAbsentValue
	}
	return t.Format("2006-01-02")
}

// anyRequirementCrossRepo returns true when any row has an owning repo that
// differs from currentRepo — drives whether the REPO column is rendered.
func anyRequirementCrossRepo(reqs []projectstatus.Requirement, currentRepo string) bool {
	for _, r := range reqs {
		if r.OwningRepo != "" && !strings.EqualFold(r.OwningRepo, currentRepo) {
			return true
		}
	}
	return false
}

// repoDisplay renders the REPO cell — "(this repo)" for the current repo,
// "owner/name" otherwise, or "-" when the owning repo is unknown.
func repoDisplay(owningRepo, currentRepo string) string {
	if owningRepo == "" {
		return "-"
	}
	if strings.EqualFold(owningRepo, currentRepo) {
		return "(this repo)"
	}
	return owningRepo
}

// stageDisplay renders the STAGE cell — the canonical stage verbatim, or "-"
// when the stage is unknown.
func stageDisplay(s projectstatus.Stage) string {
	if s == projectstatus.StageUnknown {
		return "-"
	}
	return string(s)
}

// requirementsTotalsLine builds the summary line shown at the bottom of the
// table view. The subject is singular when n==1.
func requirementsTotalsLine(n, blocked int) string {
	noun := "requirements"
	if n == 1 {
		noun = "requirement"
	}
	if blocked > 0 {
		return fmt.Sprintf("%d open %s (%d blocked)", n, noun, blocked)
	}
	return fmt.Sprintf("%d open %s", n, noun)
}

