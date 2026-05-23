package project

import (
	"fmt"
	"sort"

	"github.com/cli/go-gh/v2/pkg/api"
)

// OrphanIssue describes an issue carrying a type label
// (feature / requirement / task) but missing from the agentic
// ProjectV2 board. The pipeline cannot resolve these issues via
// `gh agentic status …`, so the agent halts during design at
// "issue not found in project" — the symptom that surfaced this
// gap on yapper.
type OrphanIssue struct {
	Number int
	NodeID string
	Title  string
	Labels []string // the type label(s) the issue carries
}

// FetchOrphanIssuesFunc returns the list of open issues carrying a
// type label that are NOT members of the named project.
type FetchOrphanIssuesFunc func(owner, repo, projectID string) ([]OrphanIssue, error)

// AddIssueToProjectFunc adds a single issue to a ProjectV2 board via
// the `addProjectV2ItemById` GraphQL mutation. Idempotent — adding
// an issue that is already a member returns the existing item ID.
type AddIssueToProjectFunc func(projectID, issueNodeID string) error

// graphqlOrphanIssuesResponse is the response shape of the
// triple-aliased query that fetches all type-labelled open issues.
type graphqlOrphanIssuesResponse struct {
	Repository struct {
		Features     issueWithProjectsConnection `json:"features"`
		Requirements issueWithProjectsConnection `json:"requirements"`
		Tasks        issueWithProjectsConnection `json:"tasks"`
	} `json:"repository"`
}

type issueWithProjectsConnection struct {
	Nodes []issueWithProjectsNode `json:"nodes"`
}

type issueWithProjectsNode struct {
	Number       int            `json:"number"`
	ID           string         `json:"id"`
	Title        string         `json:"title"`
	Labels       labelsNode     `json:"labels"`
	ProjectItems projectsResult `json:"projectItems"`
}

type labelsNode struct {
	Nodes []struct {
		Name string `json:"name"`
	} `json:"nodes"`
}

type projectsResult struct {
	Nodes []struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	} `json:"nodes"`
}

// DefaultFetchOrphanIssues runs one aliased GraphQL query that
// fetches every open issue carrying a `feature`, `requirement`, or
// `task` label, with each issue's `projectItems` so we can detect
// project-membership locally without N+1 round trips.
//
// Issues that ARE members of `projectID` are filtered out; the
// returned slice contains only orphans, sorted by issue number.
func DefaultFetchOrphanIssues(owner, repo, projectID string) ([]OrphanIssue, error) {
	if projectID == "" {
		return nil, fmt.Errorf("project ID is empty")
	}
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return nil, fmt.Errorf("creating GraphQL client: %w", err)
	}

	// Triple-aliased query: GitHub's `labels:` arg on issues is an
	// AND filter, so we cannot ask for "any of [feature, requirement,
	// task]" in one call. Three aliased connections in one query is
	// still one HTTP round trip.
	const query = `
query($owner: String!, $repo: String!) {
  repository(owner: $owner, name: $repo) {
    features: issues(states: OPEN, labels: ["feature"], first: 100) {
      nodes { ...IssueWithProjects }
    }
    requirements: issues(states: OPEN, labels: ["requirement"], first: 100) {
      nodes { ...IssueWithProjects }
    }
    tasks: issues(states: OPEN, labels: ["task"], first: 100) {
      nodes { ...IssueWithProjects }
    }
  }
}
fragment IssueWithProjects on Issue {
  number
  id
  title
  labels(first: 20) { nodes { name } }
  projectItems(first: 10) { nodes { project { id } } }
}`

	var resp graphqlOrphanIssuesResponse
	if err := client.Do(query, map[string]interface{}{
		"owner": owner,
		"repo":  repo,
	}, &resp); err != nil {
		return nil, fmt.Errorf("querying orphan issues for %s/%s: %w", owner, repo, err)
	}

	// Merge the three connections by node ID — an issue can appear
	// in two (e.g. a `task` that's also somehow labelled `feature`,
	// rare but possible). De-dupe to avoid double-counting orphans.
	merged := map[string]issueWithProjectsNode{}
	for _, n := range resp.Repository.Features.Nodes {
		merged[n.ID] = n
	}
	for _, n := range resp.Repository.Requirements.Nodes {
		merged[n.ID] = n
	}
	for _, n := range resp.Repository.Tasks.Nodes {
		merged[n.ID] = n
	}

	var orphans []OrphanIssue
	for _, n := range merged {
		if isMemberOfProject(n.ProjectItems, projectID) {
			continue
		}
		labels := make([]string, 0, len(n.Labels.Nodes))
		for _, lbl := range n.Labels.Nodes {
			labels = append(labels, lbl.Name)
		}
		orphans = append(orphans, OrphanIssue{
			Number: n.Number,
			NodeID: n.ID,
			Title:  n.Title,
			Labels: labels,
		})
	}
	sort.Slice(orphans, func(i, j int) bool {
		return orphans[i].Number < orphans[j].Number
	})
	return orphans, nil
}

// isMemberOfProject returns true if any of the issue's projectItems
// belongs to the named project.
func isMemberOfProject(items projectsResult, projectID string) bool {
	for _, item := range items.Nodes {
		if item.Project.ID == projectID {
			return true
		}
	}
	return false
}

// DefaultAddIssueToProject adds an issue to a ProjectV2 via the
// `addProjectV2ItemById` mutation. The mutation is idempotent at the
// GraphQL layer — adding an already-present issue returns its
// existing item ID without error.
func DefaultAddIssueToProject(projectID, issueNodeID string) error {
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return fmt.Errorf("creating GraphQL client: %w", err)
	}
	const mutation = `
mutation($projectId: ID!, $contentId: ID!) {
  addProjectV2ItemById(input: { projectId: $projectId, contentId: $contentId }) {
    item { id }
  }
}`
	var resp struct {
		AddProjectV2ItemById struct {
			Item struct {
				ID string `json:"id"`
			} `json:"item"`
		} `json:"addProjectV2ItemById"`
	}
	if err := client.Do(mutation, map[string]interface{}{
		"projectId": projectID,
		"contentId": issueNodeID,
	}, &resp); err != nil {
		return fmt.Errorf("adding issue %s to project %s: %w", issueNodeID, projectID, err)
	}
	return nil
}
