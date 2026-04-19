package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

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
// narrows to the current repo, then renders either the --json envelope or
// the compact tabular list.
//
// The legacy --kanban flag was removed by feature #518. If the caller
// passes --kanban (hidden on this command for interception), the handler
// returns errPipelineCommandRenamed pointing at `gh agentic status
// pipeline --requirements` — feature #549 moved the sub-command under
// `status` and feature #562 renamed it from `kanban` to `pipeline`.
//
// stderr receives the busy-indicator rendered by deps.busy while the
// fetch is in flight; stdout (w) receives the final human or JSON output.
// Non-TTY writers suppress the indicator — see ui.BusyRun.
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
	// indicator writes to stderr so --json consumers piping stdout to jq
	// get clean output; non-TTY writers suppress the glyphs entirely.
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

	if flags.json {
		return writeRequirementsJSON(w, reqs)
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

// writeRequirementsJSON marshals the envelope {items, totals} as pretty
// JSON. Items is a concrete slice rather than interface{} so the marshalled
// shape includes every Requirement field — callers relying on schema
// stability get a deterministic payload.
func writeRequirementsJSON(w io.Writer, reqs []projectstatus.Requirement) error {
	// Ensure a non-nil slice so the JSON always contains "items": [] rather
	// than "items": null for the empty case.
	if reqs == nil {
		reqs = []projectstatus.Requirement{}
	}
	// Ensure LinkedFeatures is a non-nil slice per Requirement so the JSON
	// emits [] rather than null — consumers parse items uniformly.
	for i := range reqs {
		if reqs[i].LinkedFeatures == nil {
			reqs[i].LinkedFeatures = []projectstatus.FeatureSummary{}
		}
	}
	envelope := projectstatus.ListEnvelope{
		Items:  reqs,
		Totals: countRequirementsTotals(reqs),
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(envelope); err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}
	return nil
}

// countRequirementsTotals computes the Open / Blocked counts rendered into
// the list envelope totals.
func countRequirementsTotals(reqs []projectstatus.Requirement) projectstatus.ListTotals {
	blocked := 0
	for _, r := range reqs {
		if r.Blocked != nil {
			blocked++
		}
	}
	return projectstatus.ListTotals{Open: len(reqs), Blocked: blocked}
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

