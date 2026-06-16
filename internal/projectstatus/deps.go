package projectstatus

import (
	"time"

	"github.com/eddiecarpenter/gh-agentic/internal/project"
)

// ProjectIssue is the raw form of a single issue inside a ProjectV2, with
// enough information for both list and detail composers. It is the single
// shared shape returned by FetchProjectIssuesFunc so that higher-level
// composers do not need extra round trips for basic fields.
//
// Stage is already parsed (ParseStage applied to the project's Status option
// name); Type is either "requirement", "feature", "task", or "" if the issue
// carries no type label.
type ProjectIssue struct {
	Number             int
	Title              string
	Body               string
	Stage              Stage
	Type               string
	State              string // "open" | "closed"
	Labels             []string
	CreatedAt          time.Time
	LastTransitionedAt time.Time
	OwningRepo         string // "owner/name"
	TargetRepo         string // "Target repo" ProjectV2 field value; "" when unset (#872)
}

// FetchProjectIssuesFunc returns all issues that appear on the ProjectV2
// board identified by projectID. The implementation is expected to pull the
// ProjectV2 `items` list, resolve the Status single-select option per item,
// and normalise it via ParseStage.
type FetchProjectIssuesFunc func(projectID string) ([]ProjectIssue, error)

// FetchIssueFunc returns a single issue from a specific repo by number. It
// returns ErrIssueNotFound when the issue does not exist. Used by detail
// composers and the parent-requirement resolver.
type FetchIssueFunc func(owner, repo string, number int) (*ProjectIssue, error)

// FetchSubIssuesFunc returns the sub-issues of the given parent issue (via
// the GitHub `subIssues` GraphQL relationship). Used for `FetchTasksForFeature`.
type FetchSubIssuesFunc func(owner, repo string, number int) ([]TaskRef, error)

// FetchParentIssueFunc returns the parent issue of the given issue via the
// native `trackedInIssues` relationship. Returns nil (not an error) when the
// issue has no tracked-in parent.
type FetchParentIssueFunc func(owner, repo string, number int) (*RequirementSummary, error)

// FetchBranchFunc returns the state of a feature branch in a repo. Exists=false
// means no ref matches the expected name; the other fields are zero-valued.
type FetchBranchFunc func(owner, repo, branchName string) (*BranchState, error)

// FetchPRFunc returns the PR associated with head branch in a repo, or nil if
// there is no PR for that ref.
type FetchPRFunc func(owner, repo, branchName string) (*PRState, error)

// Deps holds the injectable dependencies consumed by the projectstatus
// query layer. Production wiring lives in DefaultDeps; tests build a Deps
// manually with fakes.
type Deps struct {
	FetchLinkedRepos   project.FetchLinkedReposFunc
	FetchProjectIssues FetchProjectIssuesFunc
	FetchIssue         FetchIssueFunc
	FetchSubIssues     FetchSubIssuesFunc
	FetchParentIssue   FetchParentIssueFunc
	FetchBranch        FetchBranchFunc
	FetchPR            FetchPRFunc
	FetchBlocker       FetchBlockerFunc
}

// DefaultDeps returns production dependencies wired to the GitHub GraphQL and
// REST clients. Each Default* implementation lives in queries_default.go.
func DefaultDeps() Deps {
	return Deps{
		FetchLinkedRepos:   project.DefaultFetchLinkedRepos,
		FetchProjectIssues: DefaultFetchProjectIssues,
		FetchIssue:         DefaultFetchIssue,
		FetchSubIssues:     DefaultFetchSubIssues,
		FetchParentIssue:   DefaultFetchParentIssue,
		FetchBranch:        DefaultFetchBranch,
		FetchPR:            DefaultFetchPR,
		FetchBlocker:       DefaultFetchBlocker,
	}
}
