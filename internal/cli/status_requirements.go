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
}

// defaultStatusDeps returns production dependencies wired to gh auth.
func defaultStatusDeps() statusDeps {
	return statusDeps{
		currentRepo:      defaultCurrentRepoFullName,
		resolveProjectID: defaultResolveProjectID,
		psDeps:           projectstatus.DefaultDeps(),
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

// defaultResolveProjectID reads the AGENTIC_PROJECT_ID repository variable.
// An empty string with nil error means the variable is not set; a non-nil
// error means the lookup itself failed (auth / network / unexpected API
// response).
func defaultResolveProjectID(repoFullName string) (string, error) {
	parts := strings.SplitN(repoFullName, "/", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid repo full name %q", repoFullName)
	}
	value, err := project.DefaultGetRepoVariable(parts[0], parts[1], project.ProjectVarName)
	if err != nil {
		// go-gh returns a 404 HTTPError when the variable is absent. Rather
		// than introspect the error here we surface an empty string — the
		// caller treats empty as "not configured" and renders the friendly
		// message. Task #503 will tighten this into a classified error.
		if strings.Contains(strings.ToLower(err.Error()), "not found") ||
			strings.Contains(strings.ToLower(err.Error()), "variable_not_found") {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(value), nil
}

// runStatusRequirements is the handler for `gh agentic status requirements`.
// It resolves the project ID, fetches the list via projectstatus, optionally
// narrows to the current repo, then renders one of three forms:
//
//  1. --json — envelope {items, totals} regardless of other layout flags.
//     --json silently wins over --kanban (documented in help).
//  2. --kanban — stage-grouped view (vertical by default; --horizontal
//     adds side-by-side rendering when the terminal is wide enough).
//  3. Default — compact tabular list.
func runStatusRequirements(w io.Writer, flags statusListFlags, deps statusDeps) error {
	if flags.horizontal && !flags.kanban {
		return fmt.Errorf("--horizontal requires --kanban")
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

	reqs, err := projectstatus.FetchRequirements(deps.psDeps, projectID, flags.includeDone)
	if err != nil {
		return fmt.Errorf("fetching requirements: %w", err)
	}

	if flags.thisRepo {
		reqs = filterRequirementsToRepo(reqs, currentRepo)
	}

	// --json is the highest-priority format — it wins silently over --kanban.
	// The JSON schema is identical regardless of --kanban; consumers group
	// by stage themselves if they need a kanban-shaped view.
	if flags.json {
		return writeRequirementsJSON(w, reqs)
	}

	if flags.kanban {
		columns := columnsForRequirements(flags.includeDone)
		cards := requirementCards(reqs, columns)
		if flags.horizontal {
			return writeHorizontalKanban(w, columns, cards, terminalWidth(), kanbanMinHorizontalWidthRequirements, ui.TerminalSupportsUTF8())
		}
		return writeVerticalKanban(w, "Requirements — Kanban", columns, cards)
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

