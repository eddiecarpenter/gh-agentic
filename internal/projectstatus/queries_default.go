package projectstatus

import (
	"fmt"
	"strings"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
)

// classifyIssueType inspects the label set of an issue and returns the
// canonical type string — "requirement", "feature", "task", or "" when the
// issue is not pipeline-typed.
func classifyIssueType(labels []string) string {
	for _, l := range labels {
		switch strings.ToLower(l) {
		case "requirement":
			return "requirement"
		case "feature":
			return "feature"
		case "task":
			return "task"
		}
	}
	return ""
}

// graphqlProjectItemsResponse mirrors the shape of the ProjectV2 items query
// used by DefaultFetchProjectIssues. Only the fields we read are declared.
type graphqlProjectItemsResponse struct {
	Node struct {
		Items struct {
			PageInfo struct {
				HasNextPage bool   `json:"hasNextPage"`
				EndCursor   string `json:"endCursor"`
			} `json:"pageInfo"`
			Nodes []struct {
				FieldValueByName struct {
					Name string `json:"name"`
				} `json:"fieldValueByName"`
				Content struct {
					TypeName      string    `json:"__typename"`
					Number        int       `json:"number"`
					Title         string    `json:"title"`
					Body          string    `json:"body"`
					State         string    `json:"state"`
					CreatedAt     time.Time `json:"createdAt"`
					UpdatedAt     time.Time `json:"updatedAt"`
					Repository    struct {
						NameWithOwner string `json:"nameWithOwner"`
					} `json:"repository"`
					Labels struct {
						Nodes []struct {
							Name string `json:"name"`
						} `json:"nodes"`
					} `json:"labels"`
				} `json:"content"`
			} `json:"nodes"`
		} `json:"items"`
	} `json:"node"`
}

// DefaultFetchProjectIssues queries the ProjectV2 `items` field and maps each
// item into a ProjectIssue with its Status option resolved to Stage.
//
// The query is paginated; this implementation follows the `endCursor` until
// every page has been read.
func DefaultFetchProjectIssues(projectID string) ([]ProjectIssue, error) {
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return nil, fmt.Errorf("creating GraphQL client: %w", err)
	}

	query := `query($id: ID!, $after: String) {
		node(id: $id) {
			... on ProjectV2 {
				items(first: 100, after: $after) {
					pageInfo { hasNextPage endCursor }
					nodes {
						fieldValueByName(name: "Status") {
							... on ProjectV2ItemFieldSingleSelectValue { name }
						}
						content {
							__typename
							... on Issue {
								number title body state createdAt updatedAt
								repository { nameWithOwner }
								labels(first: 20) { nodes { name } }
							}
						}
					}
				}
			}
		}
	}`

	out := make([]ProjectIssue, 0)
	var cursor *string
	for {
		vars := map[string]interface{}{"id": projectID}
		if cursor != nil {
			vars["after"] = *cursor
		} else {
			vars["after"] = nil
		}

		var resp graphqlProjectItemsResponse
		if err := client.Do(query, vars, &resp); err != nil {
			return nil, fmt.Errorf("querying project items for %s: %w", projectID, err)
		}

		for _, n := range resp.Node.Items.Nodes {
			if n.Content.TypeName != "Issue" || n.Content.Number == 0 {
				continue
			}
			labelNames := make([]string, 0, len(n.Content.Labels.Nodes))
			for _, l := range n.Content.Labels.Nodes {
				labelNames = append(labelNames, l.Name)
			}
			out = append(out, ProjectIssue{
				Number:             n.Content.Number,
				Title:              n.Content.Title,
				Body:               n.Content.Body,
				Stage:              ParseStage(n.FieldValueByName.Name),
				Type:               classifyIssueType(labelNames),
				State:              strings.ToLower(n.Content.State),
				Labels:             labelNames,
				CreatedAt:          n.Content.CreatedAt,
				LastTransitionedAt: n.Content.UpdatedAt,
				OwningRepo:         n.Content.Repository.NameWithOwner,
			})
		}

		if !resp.Node.Items.PageInfo.HasNextPage {
			break
		}
		next := resp.Node.Items.PageInfo.EndCursor
		cursor = &next
	}

	return out, nil
}

// graphqlIssueResponse mirrors a single-issue query.
type graphqlIssueResponse struct {
	Repository struct {
		Issue *struct {
			Number     int       `json:"number"`
			Title      string    `json:"title"`
			Body       string    `json:"body"`
			State      string    `json:"state"`
			CreatedAt  time.Time `json:"createdAt"`
			UpdatedAt  time.Time `json:"updatedAt"`
			Repository struct {
				NameWithOwner string `json:"nameWithOwner"`
			} `json:"repository"`
			Labels struct {
				Nodes []struct {
					Name string `json:"name"`
				} `json:"nodes"`
			} `json:"labels"`
		} `json:"issue"`
	} `json:"repository"`
}

// DefaultFetchIssue queries GitHub for a single issue by number. Returns
// ErrIssueNotFound when the issue does not exist.
func DefaultFetchIssue(owner, repo string, number int) (*ProjectIssue, error) {
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return nil, fmt.Errorf("creating GraphQL client: %w", err)
	}

	query := `query($owner: String!, $repo: String!, $number: Int!) {
		repository(owner: $owner, name: $repo) {
			issue(number: $number) {
				number title body state createdAt updatedAt
				repository { nameWithOwner }
				labels(first: 20) { nodes { name } }
			}
		}
	}`

	var resp graphqlIssueResponse
	if err := client.Do(query, map[string]interface{}{"owner": owner, "repo": repo, "number": number}, &resp); err != nil {
		return nil, fmt.Errorf("fetching issue %s/%s#%d: %w", owner, repo, number, err)
	}

	iss := resp.Repository.Issue
	if iss == nil {
		return nil, ErrIssueNotFound
	}
	labelNames := make([]string, 0, len(iss.Labels.Nodes))
	for _, l := range iss.Labels.Nodes {
		labelNames = append(labelNames, l.Name)
	}
	return &ProjectIssue{
		Number:             iss.Number,
		Title:              iss.Title,
		Body:               iss.Body,
		State:              strings.ToLower(iss.State),
		Type:               classifyIssueType(labelNames),
		Labels:             labelNames,
		CreatedAt:          iss.CreatedAt,
		LastTransitionedAt: iss.UpdatedAt,
		OwningRepo:         iss.Repository.NameWithOwner,
	}, nil
}

// graphqlSubIssuesResponse mirrors the sub-issues query. subIssues is a first-class
// relationship on Issue; we read number, title, and state.
type graphqlSubIssuesResponse struct {
	Repository struct {
		Issue *struct {
			SubIssues struct {
				Nodes []struct {
					Number int    `json:"number"`
					Title  string `json:"title"`
					State  string `json:"state"`
				} `json:"nodes"`
			} `json:"subIssues"`
		} `json:"issue"`
	} `json:"repository"`
}

// DefaultFetchSubIssues reads the native sub-issue relationship of an issue
// and returns TaskRefs. Closed sub-issues have Closed=true so the renderer
// can show ✓ vs ☐.
func DefaultFetchSubIssues(owner, repo string, number int) ([]TaskRef, error) {
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return nil, fmt.Errorf("creating GraphQL client: %w", err)
	}

	query := `query($owner: String!, $repo: String!, $number: Int!) {
		repository(owner: $owner, name: $repo) {
			issue(number: $number) {
				subIssues(first: 100) {
					nodes { number title state }
				}
			}
		}
	}`

	var resp graphqlSubIssuesResponse
	if err := client.Do(query, map[string]interface{}{"owner": owner, "repo": repo, "number": number}, &resp); err != nil {
		return nil, fmt.Errorf("fetching sub-issues for %s/%s#%d: %w", owner, repo, number, err)
	}

	iss := resp.Repository.Issue
	if iss == nil {
		return nil, ErrIssueNotFound
	}
	out := make([]TaskRef, 0, len(iss.SubIssues.Nodes))
	for _, n := range iss.SubIssues.Nodes {
		out = append(out, TaskRef{
			Number: n.Number,
			Title:  n.Title,
			Closed: strings.EqualFold(n.State, "closed"),
		})
	}
	return out, nil
}

// graphqlParentIssueResponse mirrors the tracked-in-issues query used to
// discover the parent of a feature.
type graphqlParentIssueResponse struct {
	Repository struct {
		Issue *struct {
			TrackedInIssues struct {
				Nodes []struct {
					Number     int    `json:"number"`
					Title      string `json:"title"`
					Repository struct {
						NameWithOwner string `json:"nameWithOwner"`
					} `json:"repository"`
					Labels struct {
						Nodes []struct {
							Name string `json:"name"`
						} `json:"nodes"`
					} `json:"labels"`
				} `json:"nodes"`
			} `json:"trackedInIssues"`
		} `json:"issue"`
	} `json:"repository"`
}

// DefaultFetchParentIssue returns the first tracked-in issue labelled
// `requirement`, or nil when none is present.
func DefaultFetchParentIssue(owner, repo string, number int) (*RequirementSummary, error) {
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return nil, fmt.Errorf("creating GraphQL client: %w", err)
	}

	query := `query($owner: String!, $repo: String!, $number: Int!) {
		repository(owner: $owner, name: $repo) {
			issue(number: $number) {
				trackedInIssues(first: 10) {
					nodes {
						number title
						repository { nameWithOwner }
						labels(first: 20) { nodes { name } }
					}
				}
			}
		}
	}`

	var resp graphqlParentIssueResponse
	if err := client.Do(query, map[string]interface{}{"owner": owner, "repo": repo, "number": number}, &resp); err != nil {
		return nil, fmt.Errorf("fetching parent issues for %s/%s#%d: %w", owner, repo, number, err)
	}

	iss := resp.Repository.Issue
	if iss == nil {
		return nil, nil
	}
	for _, n := range iss.TrackedInIssues.Nodes {
		labelNames := make([]string, 0, len(n.Labels.Nodes))
		for _, l := range n.Labels.Nodes {
			labelNames = append(labelNames, l.Name)
		}
		if classifyIssueType(labelNames) != "requirement" {
			continue
		}
		return &RequirementSummary{
			Number:     n.Number,
			Title:      n.Title,
			OwningRepo: n.Repository.NameWithOwner,
			// Stage is left unknown — the requirement appears on the project
			// board and callers typically resolve Stage from there. Leaving
			// it as StageUnknown keeps the caller in charge of canonicalisation.
		}, nil
	}
	return nil, nil
}

// graphqlBranchResponse mirrors the branch-ref query.
type graphqlBranchResponse struct {
	Repository struct {
		Ref *struct {
			Name                 string `json:"name"`
			AssociatedPullRequests struct {
				Nodes []struct {
					Merged bool   `json:"merged"`
					State  string `json:"state"`
				} `json:"nodes"`
			} `json:"associatedPullRequests"`
		} `json:"ref"`
	} `json:"repository"`
}

// DefaultFetchBranch queries a branch ref and reports existence + merged
// state. Merged is derived from the most-recent associated PR.
func DefaultFetchBranch(owner, repo, branchName string) (*BranchState, error) {
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return nil, fmt.Errorf("creating GraphQL client: %w", err)
	}

	query := `query($owner: String!, $repo: String!, $name: String!) {
		repository(owner: $owner, name: $repo) {
			ref(qualifiedName: $name) {
				name
				associatedPullRequests(first: 1, states: [OPEN, MERGED, CLOSED]) {
					nodes { merged state }
				}
			}
		}
	}`

	qualified := "refs/heads/" + branchName
	var resp graphqlBranchResponse
	if err := client.Do(query, map[string]interface{}{"owner": owner, "repo": repo, "name": qualified}, &resp); err != nil {
		return nil, fmt.Errorf("fetching branch %s/%s/%s: %w", owner, repo, branchName, err)
	}

	if resp.Repository.Ref == nil {
		return &BranchState{Name: branchName, Exists: false}, nil
	}

	merged := false
	for _, pr := range resp.Repository.Ref.AssociatedPullRequests.Nodes {
		if pr.Merged {
			merged = true
			break
		}
	}
	return &BranchState{Name: branchName, Exists: true, Merged: merged}, nil
}

// graphqlPRResponse mirrors the head-branch PR lookup.
type graphqlPRResponse struct {
	Repository struct {
		PullRequests struct {
			Nodes []struct {
				Number         int    `json:"number"`
				State          string `json:"state"`
				Merged         bool   `json:"merged"`
				ReviewRequests struct {
					Nodes []struct {
						RequestedReviewer struct {
							TypeName string `json:"__typename"`
							Login    string `json:"login"`
						} `json:"requestedReviewer"`
					} `json:"nodes"`
				} `json:"reviewRequests"`
				Reviews struct {
					Nodes []struct {
						Author struct {
							Login string `json:"login"`
						} `json:"author"`
					} `json:"nodes"`
				} `json:"reviews"`
			} `json:"nodes"`
		} `json:"pullRequests"`
	} `json:"repository"`
}

// graphqlTrackedIssuesResponse mirrors the native issue-dependency lookup.
// `trackedIssues` returns the set of issues the target issue depends on —
// i.e. the blockers. We read the first non-closed blocker and surface its
// title as the "reason".
type graphqlTrackedIssuesResponse struct {
	Repository struct {
		Issue *struct {
			TrackedIssues struct {
				Nodes []struct {
					Number     int    `json:"number"`
					Title      string `json:"title"`
					State      string `json:"state"`
					Repository struct {
						NameWithOwner string `json:"nameWithOwner"`
					} `json:"repository"`
				} `json:"nodes"`
			} `json:"trackedIssues"`
		} `json:"issue"`
	} `json:"repository"`
}

// DefaultFetchBlocker queries the native GitHub issue-dependency relationship
// (`trackedIssues` on the target issue) and returns the first open blocker.
// A nil result with a nil error means the issue has no native blocker; the
// caller then falls back to the structured convention in the body.
func DefaultFetchBlocker(owner, repo string, number int) (*BlockedInfo, error) {
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return nil, fmt.Errorf("creating GraphQL client: %w", err)
	}

	query := `query($owner: String!, $repo: String!, $number: Int!) {
		repository(owner: $owner, name: $repo) {
			issue(number: $number) {
				trackedIssues(first: 10) {
					nodes {
						number title state
						repository { nameWithOwner }
					}
				}
			}
		}
	}`

	var resp graphqlTrackedIssuesResponse
	if err := client.Do(query, map[string]interface{}{"owner": owner, "repo": repo, "number": number}, &resp); err != nil {
		// If the field is not supported on the target repo, the GraphQL
		// error propagates to the caller — FetchBlocker wraps it; the CLI
		// layer classifies into network / auth / server.
		return nil, fmt.Errorf("querying trackedIssues for %s/%s#%d: %w", owner, repo, number, err)
	}

	iss := resp.Repository.Issue
	if iss == nil {
		return nil, nil
	}
	for _, n := range iss.TrackedIssues.Nodes {
		if strings.EqualFold(n.State, "closed") {
			continue
		}
		ref := fmt.Sprintf("%s#%d", n.Repository.NameWithOwner, n.Number)
		if n.Repository.NameWithOwner == "" {
			ref = fmt.Sprintf("#%d", n.Number)
		}
		return &BlockedInfo{BlockingRef: ref, Reason: n.Title}, nil
	}
	return nil, nil
}

// DefaultFetchPR returns the PR associated with the given head branch, or
// nil when no PR exists for that ref. State is normalised to "open" |
// "merged" | "closed".
func DefaultFetchPR(owner, repo, branchName string) (*PRState, error) {
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return nil, fmt.Errorf("creating GraphQL client: %w", err)
	}

	query := `query($owner: String!, $repo: String!, $head: String!) {
		repository(owner: $owner, name: $repo) {
			pullRequests(first: 5, headRefName: $head, orderBy: {field: CREATED_AT, direction: DESC}) {
				nodes {
					number state merged
					reviewRequests(first: 20) {
						nodes { requestedReviewer { __typename ... on User { login } } }
					}
					reviews(first: 50) {
						nodes { author { login } }
					}
				}
			}
		}
	}`

	var resp graphqlPRResponse
	if err := client.Do(query, map[string]interface{}{"owner": owner, "repo": repo, "head": branchName}, &resp); err != nil {
		return nil, fmt.Errorf("fetching PR for %s/%s %s: %w", owner, repo, branchName, err)
	}

	if len(resp.Repository.PullRequests.Nodes) == 0 {
		return nil, nil
	}

	pr := resp.Repository.PullRequests.Nodes[0]
	state := strings.ToLower(pr.State)
	if pr.Merged {
		state = "merged"
	}

	// Collect reviewers — deduped, preserving first-seen order.
	seen := map[string]bool{}
	reviewers := make([]string, 0)
	for _, r := range pr.ReviewRequests.Nodes {
		login := r.RequestedReviewer.Login
		if login != "" && !seen[login] {
			reviewers = append(reviewers, login)
			seen[login] = true
		}
	}
	for _, r := range pr.Reviews.Nodes {
		login := r.Author.Login
		if login != "" && !seen[login] {
			reviewers = append(reviewers, login)
			seen[login] = true
		}
	}

	return &PRState{
		Number:    pr.Number,
		State:     state,
		Reviewers: reviewers,
	}, nil
}
