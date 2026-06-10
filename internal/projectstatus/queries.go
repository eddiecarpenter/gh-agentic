package projectstatus

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// closesPattern matches the GitHub `Closes #N` / `Closes owner/repo#N`
// marker used to link a feature to its parent requirement. The form is
// deliberately narrow — we do not parse free-form prose.
var closesPattern = regexp.MustCompile(`(?i)\bcloses\s+(?:([A-Za-z0-9-]+/[A-Za-z0-9_.-]+))?#(\d+)\b`)

// featureBranchPattern derives the feature branch name prefix for a given
// feature issue. Feature branches follow `feature/<N>-<slug>`; we strip the
// slug so callers can match by prefix.
const featureBranchPrefix = "feature/"

// FetchRequirements returns the requirement issues on the project board,
// sorted by issue number ascending. Closed or done items are filtered out
// unless includeDone is true.
//
// Blocked-by annotation is left nil by this function; wiring the dependency
// mechanism is scoped to task #501.
func FetchRequirements(deps Deps, projectID string, includeDone bool) ([]Requirement, error) {
	issues, err := deps.FetchProjectIssues(projectID)
	if err != nil {
		return nil, fmt.Errorf("fetching project issues: %w", err)
	}

	reqs := make([]Requirement, 0)
	for _, issue := range issues {
		if issue.Type != "requirement" {
			continue
		}
		if !includeDone && (issue.State == "closed" || issue.Stage == StageDone) {
			continue
		}
		r := Requirement{
			Number:             issue.Number,
			Title:              issue.Title,
			Body:               issue.Body,
			Stage:              issue.Stage,
			CreatedAt:          issue.CreatedAt,
			LastTransitionedAt: issue.LastTransitionedAt,
			OwningRepo:         issue.OwningRepo,
		}
		populateBlocked(deps, &r.Blocked, issue.OwningRepo, issue.Number, issue.Body)
		reqs = append(reqs, r)
	}

	sort.Slice(reqs, func(i, j int) bool { return reqs[i].Number < reqs[j].Number })
	return reqs, nil
}

// FetchRequirement returns a single requirement with its linked features
// embedded. Returns ErrIssueNotFound when the issue does not appear on the
// project board, or *ErrWrongType when the referenced issue is a feature or
// task.
//
// Linked features are derived by scanning feature issues in the same project
// for a `Closes #N` marker that points at this requirement. For each match
// the function fetches branch and PR state via the injectable Deps so the
// summary carries a pre-rendered BranchOneLiner.
func FetchRequirement(deps Deps, projectID string, number int) (*Requirement, error) {
	issues, err := deps.FetchProjectIssues(projectID)
	if err != nil {
		return nil, fmt.Errorf("fetching project issues: %w", err)
	}

	found := findByNumber(issues, number)
	if found == nil {
		return nil, ErrIssueNotFound
	}
	if found.Type != "requirement" {
		return nil, &ErrWrongType{Number: number, ActualType: found.Type, WantedType: "requirement"}
	}

	req := &Requirement{
		Number:             found.Number,
		Title:              found.Title,
		Body:               found.Body,
		Stage:              found.Stage,
		CreatedAt:          found.CreatedAt,
		LastTransitionedAt: found.LastTransitionedAt,
		OwningRepo:         found.OwningRepo,
	}
	populateBlocked(deps, &req.Blocked, found.OwningRepo, found.Number, found.Body)

	// Build linked features — use native sub-issue relationship.
	// NOTE: deps.FetchSubIssues queries subIssues(first: 100); requirements with
	// more than 100 linked features will be silently truncated at that limit.
	// A paginated approach can be introduced if the scale requires it.
	linked := make([]FeatureSummary, 0)
	reqOwner, reqRepo := splitOwnerRepo(found.OwningRepo)
	if reqOwner != "" && reqRepo != "" && deps.FetchSubIssues != nil {
		subRefs, err := deps.FetchSubIssues(reqOwner, reqRepo, number)
		if err != nil {
			// Partial failure — populate the error field and continue with an
			// empty linked-features list rather than aborting the whole render.
			req.LinkedFeaturesError = fmt.Sprintf("%s: %v", found.OwningRepo, err)
		}
		for _, sub := range subRefs {
			// Cross-reference with the project board. Features that are native
			// sub-issues of the requirement but were manually removed from the
			// project board are silently skipped — without project-board context
			// (stage, owning repo) they cannot be rendered consistently.
			issue := findByNumber(issues, sub.Number)
			if issue == nil || issue.Type != "feature" {
				continue
			}
			summary := FeatureSummary{
				Number:     issue.Number,
				Title:      issue.Title,
				Stage:      issue.Stage,
				OwningRepo: issue.OwningRepo,
			}
			owner, repo := splitOwnerRepo(issue.OwningRepo)
			branchName := fmt.Sprintf("%s%d", featureBranchPrefix, issue.Number)
			if owner != "" && repo != "" && deps.FetchBranch != nil {
				if br, err := deps.FetchBranch(owner, repo, branchName); err == nil && br != nil {
					summary.BranchOneLiner = renderBranchOneLiner(br)
				}
			}
			if owner != "" && repo != "" && deps.FetchPR != nil {
				if pr, err := deps.FetchPR(owner, repo, branchName); err == nil {
					summary.PR = pr
				}
			}
			linked = append(linked, summary)
		}
	}
	sort.Slice(linked, func(i, j int) bool { return linked[i].Number < linked[j].Number })
	req.LinkedFeatures = linked

	return req, nil
}

// FetchFeatures returns the feature issues on the project board, sorted by
// issue number ascending. Closed or done items are filtered out unless
// includeDone is true.
//
// Each returned Feature has TasksTotal and TasksDone populated from the
// sub-issue relationship (via deps.FetchSubIssues). A feature with no
// sub-issues yields both zero. Sub-issue fetch failures for an individual
// feature are treated as zero counts rather than failing the whole list —
// the task-count signal is a rendering nice-to-have, not a hard
// requirement. The per-feature loop is acceptable because the number of
// open features is bounded in practice; a future optimisation can batch
// the sub-issue queries via GraphQL aliases.
func FetchFeatures(deps Deps, projectID string, includeDone bool) ([]Feature, error) {
	issues, err := deps.FetchProjectIssues(projectID)
	if err != nil {
		return nil, fmt.Errorf("fetching project issues: %w", err)
	}

	features := make([]Feature, 0)
	for _, issue := range issues {
		if issue.Type != "feature" {
			continue
		}
		if !includeDone && (issue.State == "closed" || issue.Stage == StageDone) {
			continue
		}
		f := Feature{
			Number:             issue.Number,
			Title:              issue.Title,
			Body:               issue.Body,
			Stage:              issue.Stage,
			CreatedAt:          issue.CreatedAt,
			LastTransitionedAt: issue.LastTransitionedAt,
			OwningRepo:         issue.OwningRepo,
		}
		populateBlocked(deps, &f.Blocked, issue.OwningRepo, issue.Number, issue.Body)
		populateTaskCounts(deps, &f)
		features = append(features, f)
	}
	sort.Slice(features, func(i, j int) bool { return features[i].Number < features[j].Number })
	return features, nil
}

// populateTaskCounts writes TasksTotal and TasksDone on f using deps.FetchSubIssues.
// When the dependency is not wired or the owning repo cannot be parsed, both
// counts remain zero — the rendering layer treats that as "no task info
// available". When the fetch fails, the error is surfaced as OwningRepoError
// on the Feature rather than silently ignored, allowing renderers to emit a
// per-repo warning rather than showing empty counts without explanation.
func populateTaskCounts(deps Deps, f *Feature) {
	if f == nil || deps.FetchSubIssues == nil {
		return
	}
	owner, repo := splitOwnerRepo(f.OwningRepo)
	if owner == "" || repo == "" {
		return
	}
	tasks, err := deps.FetchSubIssues(owner, repo, f.Number)
	if err != nil {
		f.OwningRepoError = fmt.Sprintf("%s: %v", f.OwningRepo, err)
		return
	}
	f.TasksTotal = len(tasks)
	done := 0
	for _, t := range tasks {
		if t.Closed {
			done++
		}
	}
	f.TasksDone = done
}

// FetchFeature returns a single feature with parent requirement, tasks,
// branch state, and PR state embedded. Returns ErrIssueNotFound when the
// issue does not appear on the project board, or *ErrWrongType when the
// referenced issue is a requirement or task.
func FetchFeature(deps Deps, projectID string, number int) (*Feature, error) {
	issues, err := deps.FetchProjectIssues(projectID)
	if err != nil {
		return nil, fmt.Errorf("fetching project issues: %w", err)
	}

	found := findByNumber(issues, number)
	if found == nil {
		return nil, ErrIssueNotFound
	}
	if found.Type != "feature" {
		return nil, &ErrWrongType{Number: number, ActualType: found.Type, WantedType: "feature"}
	}

	owner, repo := splitOwnerRepo(found.OwningRepo)
	feature := &Feature{
		Number:             found.Number,
		Title:              found.Title,
		Body:               found.Body,
		Stage:              found.Stage,
		CreatedAt:          found.CreatedAt,
		LastTransitionedAt: found.LastTransitionedAt,
		OwningRepo:         found.OwningRepo,
	}
	populateBlocked(deps, &feature.Blocked, found.OwningRepo, found.Number, found.Body)

	// Tasks — sub-issue relationship.
	if deps.FetchSubIssues != nil && owner != "" && repo != "" {
		tasks, err := deps.FetchSubIssues(owner, repo, number)
		if err != nil {
			return nil, fmt.Errorf("fetching sub-issues for feature %d: %w", number, err)
		}
		feature.Tasks = tasks
		// Mirror the internal counts used by list-context renderers so the
		// detail path exposes the same signal even though it already has
		// the full Tasks slice available.
		feature.TasksTotal = len(tasks)
		for _, t := range tasks {
			if t.Closed {
				feature.TasksDone++
			}
		}
	}

	// Parent requirement — native first, then Closes #N fallback.
	parent, err := FetchParentRequirement(deps, owner, repo, number, found.Body)
	if err != nil {
		return nil, fmt.Errorf("resolving parent requirement for feature %d: %w", number, err)
	}
	feature.ParentRequirement = parent

	// Branch + PR state. Errors here indicate the owning repo is unreachable
	// (permissions, deleted repo, network). Rather than aborting the whole
	// render, populate OwningRepoError and continue — the feature still
	// renders with its other fields intact.
	branchName := fmt.Sprintf("%s%d", featureBranchPrefix, number)
	if deps.FetchBranch != nil && owner != "" && repo != "" {
		br, err := deps.FetchBranch(owner, repo, branchName)
		if err != nil {
			feature.OwningRepoError = fmt.Sprintf("%s: %v", found.OwningRepo, err)
		} else {
			feature.Branch = br
		}
	}
	if deps.FetchPR != nil && owner != "" && repo != "" {
		pr, err := deps.FetchPR(owner, repo, branchName)
		if err != nil {
			if feature.OwningRepoError == "" {
				feature.OwningRepoError = fmt.Sprintf("%s: %v", found.OwningRepo, err)
			}
		} else {
			feature.PR = pr
		}
	}

	return feature, nil
}

// ResolveFederatedRepos returns the `owner/name` list of every repository
// linked to the given project. In a single-topology project the slice
// contains exactly one entry; in a federated topology it contains the
// control plane plus every active domain repo.
func ResolveFederatedRepos(deps Deps, projectID string) ([]string, error) {
	if deps.FetchLinkedRepos == nil {
		return nil, fmt.Errorf("FetchLinkedRepos dependency not wired")
	}
	linked, err := deps.FetchLinkedRepos(projectID)
	if err != nil {
		return nil, fmt.Errorf("fetching linked repos for project %s: %w", projectID, err)
	}
	repos := make([]string, 0, len(linked))
	for _, r := range linked {
		if r.NameWithOwner != "" {
			repos = append(repos, r.NameWithOwner)
		}
	}
	sort.Strings(repos)
	return repos, nil
}

// FetchBranchState delegates to Deps.FetchBranch and wraps the error with
// context so the CLI renderer can surface a meaningful message.
func FetchBranchState(deps Deps, owner, repo, branchName string) (*BranchState, error) {
	if deps.FetchBranch == nil {
		return nil, fmt.Errorf("FetchBranch dependency not wired")
	}
	br, err := deps.FetchBranch(owner, repo, branchName)
	if err != nil {
		return nil, fmt.Errorf("fetching branch %s in %s/%s: %w", branchName, owner, repo, err)
	}
	return br, nil
}

// FetchPRState delegates to Deps.FetchPR.
func FetchPRState(deps Deps, owner, repo, branchName string) (*PRState, error) {
	if deps.FetchPR == nil {
		return nil, fmt.Errorf("FetchPR dependency not wired")
	}
	pr, err := deps.FetchPR(owner, repo, branchName)
	if err != nil {
		return nil, fmt.Errorf("fetching PR for %s in %s/%s: %w", branchName, owner, repo, err)
	}
	return pr, nil
}

// FetchTasksForFeature delegates to Deps.FetchSubIssues, returning the
// feature's sub-issues as typed TaskRefs. This is the single path used for
// rendering the ✓/☐ task list in feature detail views — prose parsing of
// issue bodies is not used.
func FetchTasksForFeature(deps Deps, owner, repo string, featureNumber int) ([]TaskRef, error) {
	if deps.FetchSubIssues == nil {
		return nil, fmt.Errorf("FetchSubIssues dependency not wired")
	}
	tasks, err := deps.FetchSubIssues(owner, repo, featureNumber)
	if err != nil {
		return nil, fmt.Errorf("fetching sub-issues for feature %d: %w", featureNumber, err)
	}
	return tasks, nil
}

// FetchParentRequirement resolves the parent requirement of a feature. The
// resolution order is:
//
//  1. Native GitHub tracked-in (`trackedInIssues`) — preferred. If the
//     feature has a tracked-in parent and that parent is labelled
//     `requirement`, use it.
//  2. `Closes <owner/repo>#N` / `Closes #N` marker in the feature body —
//     fallback when native tracking is not wired.
//  3. Neither present — return (nil, nil). Absent parent is not an error.
//
// featureBody is passed in so callers that already have the body (e.g.
// FetchFeature) do not need a second fetch; if empty, the function falls back
// to Deps.FetchIssue.
func FetchParentRequirement(deps Deps, owner, repo string, featureNumber int, featureBody string) (*RequirementSummary, error) {
	// Step 1 — native tracked-in.
	if deps.FetchParentIssue != nil {
		parent, err := deps.FetchParentIssue(owner, repo, featureNumber)
		if err != nil {
			return nil, fmt.Errorf("querying parent for feature %d: %w", featureNumber, err)
		}
		if parent != nil {
			return parent, nil
		}
	}

	// Step 2 — Closes marker.
	body := featureBody
	if body == "" && deps.FetchIssue != nil {
		issue, err := deps.FetchIssue(owner, repo, featureNumber)
		if err != nil {
			// Absent feature is not a parent-resolution error; let the caller
			// see the underlying fetch failure.
			return nil, fmt.Errorf("loading feature %d body: %w", featureNumber, err)
		}
		if issue != nil {
			body = issue.Body
		}
	}

	refOwner, refRepo, refNumber, ok := parseClosesReference(body, owner, repo)
	if !ok {
		return nil, nil
	}

	// Resolve the referenced issue so we can populate the stage and title.
	if deps.FetchIssue == nil {
		return nil, nil
	}
	issue, err := deps.FetchIssue(refOwner, refRepo, refNumber)
	if err != nil {
		// If the referenced issue cannot be fetched, treat as absent rather
		// than propagating an error — a missing parent should degrade to
		// "no parent annotation" in the UI.
		return nil, nil
	}
	if issue == nil || issue.Type != "requirement" {
		return nil, nil
	}
	return &RequirementSummary{
		Number:     issue.Number,
		Title:      issue.Title,
		Stage:      issue.Stage,
		OwningRepo: issue.OwningRepo,
	}, nil
}

// findByNumber returns a pointer to the first issue in issues with the given
// number, or nil when no match is found. Returning a pointer avoids an extra
// struct copy in the caller.
func findByNumber(issues []ProjectIssue, number int) *ProjectIssue {
	for i := range issues {
		if issues[i].Number == number {
			return &issues[i]
		}
	}
	return nil
}

// splitOwnerRepo splits an "owner/name" string. Empty or malformed input
// returns two empty strings — callers guard on that.
func splitOwnerRepo(nameWithOwner string) (string, string) {
	parts := strings.SplitN(nameWithOwner, "/", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

// bodyReferencesRequirement returns true when the feature body contains a
// `Closes <owner/repo>#N` or `Closes #N` marker pointing at the requirement.
// A bare `Closes #N` only matches when the feature and requirement share an
// owning repo — this protects against cross-repo number collisions.
func bodyReferencesRequirement(body string, reqNumber int, featureRepo, requirementRepo string) bool {
	matches := closesPattern.FindAllStringSubmatch(body, -1)
	for _, m := range matches {
		n, err := strconv.Atoi(m[2])
		if err != nil {
			continue
		}
		if n != reqNumber {
			continue
		}
		if m[1] != "" {
			// Fully qualified — `Closes owner/repo#N`.
			return strings.EqualFold(m[1], requirementRepo)
		}
		// Bare form — only match within the same repo.
		return strings.EqualFold(featureRepo, requirementRepo)
	}
	return false
}

// parseClosesReference extracts the first `Closes #N` / `Closes owner/repo#N`
// marker from a feature body. When the marker omits the owner/repo prefix,
// defaultOwner/defaultRepo are used.
func parseClosesReference(body, defaultOwner, defaultRepo string) (owner, repo string, number int, ok bool) {
	m := closesPattern.FindStringSubmatch(body)
	if len(m) == 0 {
		return "", "", 0, false
	}
	n, err := strconv.Atoi(m[2])
	if err != nil {
		return "", "", 0, false
	}
	if m[1] != "" {
		parts := strings.SplitN(m[1], "/", 2)
		if len(parts) != 2 {
			return "", "", 0, false
		}
		return parts[0], parts[1], n, true
	}
	return defaultOwner, defaultRepo, n, true
}

// populateBlocked resolves the Blocked annotation for an issue and writes
// the result into target. Errors from the blocker lookup are swallowed —
// the UX treats a blocker-lookup failure as "no annotation" rather than
// failing the whole list view. The CLI layer still surfaces the underlying
// GraphQL error separately via ClassifyAPIError when it matters.
func populateBlocked(deps Deps, target **BlockedInfo, owningRepo string, number int, body string) {
	if target == nil {
		return
	}
	owner, repo := splitOwnerRepo(owningRepo)
	info, err := FetchBlocker(deps, owner, repo, number, body)
	if err != nil || info == nil {
		return
	}
	*target = info
}

// renderBranchOneLiner produces the compact string shown in a requirement's
// linked-features list, e.g. "feature/483-skill (merged)".
func renderBranchOneLiner(br *BranchState) string {
	if br == nil || !br.Exists {
		return ""
	}
	if br.Merged {
		return fmt.Sprintf("%s (merged)", br.Name)
	}
	return br.Name
}
