package project

import (
	"fmt"
)

// Linked-repos fetch logic. Extracted from api.go so it stays covered by the
// project package's test suite — api.go is excluded from coverage as
// network-bound (see sonar-project.properties), and that exclusion is correct
// for client construction but not for the control flow below.
//
// Keep this logic here. Moving it back into api.go silently removes it from
// coverage again.

// graphQLDoer is the subset of *api.GraphQLClient this package needs. It exists
// so the fetch logic can be driven by a fake in tests without reaching the
// network. The signature matches go-gh's GraphQLClient.Do exactly, so the real
// client satisfies it with no adapter.
type graphQLDoer interface {
	Do(query string, variables map[string]interface{}, response interface{}) error
}

// linkedReposPageSize is the number of repositories requested per page. 100 is
// GitHub's maximum for a connection, and matches the paginated query in
// internal/projectstatus/queries_default.go.
const linkedReposPageSize = 100

// graphqlLinkedReposResponse is the response shape for the linked repos query.
type graphqlLinkedReposResponse struct {
	Node struct {
		Title        string `json:"title"`
		Repositories struct {
			Nodes []struct {
				Name          string `json:"name"`
				NameWithOwner string `json:"nameWithOwner"`
				URL           string `json:"url"`
			} `json:"nodes"`
			PageInfo struct {
				HasNextPage bool   `json:"hasNextPage"`
				EndCursor   string `json:"endCursor"`
			} `json:"pageInfo"`
		} `json:"repositories"`
	} `json:"node"`
}

// linkedReposQuery lists the repositories linked to a ProjectV2, one page at a
// time. $after is null for the first page and carries the previous page's
// endCursor thereafter.
const linkedReposQuery = `query($id: ID!, $after: String) {
	node(id: $id) {
		... on ProjectV2 {
			title
			repositories(first: 100, after: $after) {
				nodes {
					name
					nameWithOwner
					url
				}
				pageInfo {
					hasNextPage
					endCursor
				}
			}
		}
	}
}`

// fetchLinkedRepos executes the linked-repos query through doer, following the
// connection's cursor until it is exhausted, and maps the accumulated response
// onto LinkedRepo values.
//
// A project may link more repositories than a single page returns. Reading only
// the first page makes linked repos indistinguishable from unlinked ones, so
// callers must never receive a partial set: any page error aborts the whole
// fetch and returns no repos.
func fetchLinkedRepos(doer graphQLDoer, projectID string) ([]LinkedRepo, error) {
	var (
		repos []LinkedRepo
		after interface{} // nil on the first request — GraphQL `after: null`
	)

	for {
		var resp graphqlLinkedReposResponse
		vars := map[string]interface{}{"id": projectID, "after": after}
		if err := doer.Do(linkedReposQuery, vars, &resp); err != nil {
			return nil, fmt.Errorf("querying linked repos for project %s: %w", projectID, err)
		}

		connection := resp.Node.Repositories
		for _, n := range connection.Nodes {
			repos = append(repos, LinkedRepo{
				Name:          n.Name,
				NameWithOwner: n.NameWithOwner,
				URL:           n.URL,
			})
		}

		// Stop when the connection says there is no more, and also when it
		// claims another page but hands back no cursor to fetch it with —
		// without that guard a malformed response loops forever.
		if !connection.PageInfo.HasNextPage || connection.PageInfo.EndCursor == "" {
			break
		}
		after = connection.PageInfo.EndCursor
	}

	if repos == nil {
		repos = []LinkedRepo{}
	}
	return repos, nil
}
