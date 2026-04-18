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
		reqs = append(reqs, Requirement{
			Number:             issue.Number,
			Title:              issue.Title,
			Body:               issue.Body,
			Stage:              issue.Stage,
			CreatedAt:          issue.CreatedAt,
			LastTransitionedAt: issue.LastTransitionedAt,
			OwningRepo:         issue.OwningRepo,
		})
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

	// Build linked features — scan feature issues for a `Closes #<number>` marker.
	linked := make([]FeatureSummary, 0)
	for _, issue := range issues {
		if issue.Type != "feature" {
			continue
		}
		if !bodyReferencesRequirement(issue.Body, number, issue.OwningRepo, found.OwningRepo) {
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
	sort.Slice(linked, func(i, j int) bool { return linked[i].Number < linked[j].Number })
	req.LinkedFeatures = linked

	return req, nil
}

// FetchFeatures returns the feature issues on the project board, sorted by
// issue number ascending. Closed or done items are filtered out unless
// includeDone is true.
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
		features = append(features, Feature{
			Number:             issue.Number,
			Title:              issue.Title,
			Body:               issue.Body,
			Stage:              issue.Stage,
			CreatedAt:          issue.CreatedAt,
			LastTransitionedAt: issue.LastTransitionedAt,
			OwningRepo:         issue.OwningRepo,
		})
	}
	sort.Slice(features, func(i, j int) bool { return features[i].Number < features[j].Number })
	return features, nil
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

	// Tasks — sub-issue relationship.
	if deps.FetchSubIssues != nil && owner != "" && repo != "" {
		tasks, err := deps.FetchSubIssues(owner, repo, number)
		if err != nil {
			return nil, fmt.Errorf("fetching sub-issues for feature %d: %w", number, err)
		}
		feature.Tasks = tasks
	}

	// Parent requirement — native first, then Closes #N fallback.
	parent, err := FetchParentRequirement(deps, owner, repo, number, found.Body)
	if err != nil {
		return nil, fmt.Errorf("resolving parent requirement for feature %d: %w", number, err)
	}
	feature.ParentRequirement = parent

	// Branch + PR state.
	branchName := fmt.Sprintf("%s%d", featureBranchPrefix, number)
	if deps.FetchBranch != nil && owner != "" && repo != "" {
		br, err := deps.FetchBranch(owner, repo, branchName)
		if err != nil {
			return nil, fmt.Errorf("fetching branch state for feature %d: %w", number, err)
		}
		feature.Branch = br
	}
	if deps.FetchPR != nil && owner != "" && repo != "" {
		pr, err := deps.FetchPR(owner, repo, branchName)
		if err != nil {
			return nil, fmt.Errorf("fetching PR state for feature %d: %w", number, err)
		}
		feature.PR = pr
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
