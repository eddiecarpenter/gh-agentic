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

// linkedReposQuery is the GraphQL document used to list the repositories linked
// to a ProjectV2.
const linkedReposQuery = `query($id: ID!) {
	node(id: $id) {
		... on ProjectV2 {
			title
			repositories(first: 20) {
				nodes {
					name
					nameWithOwner
					url
				}
			}
		}
	}
}`

// fetchLinkedRepos executes the linked-repos query through doer and maps the
// response onto LinkedRepo values.
func fetchLinkedRepos(doer graphQLDoer, projectID string) ([]LinkedRepo, error) {
	var resp graphqlLinkedReposResponse
	if err := doer.Do(linkedReposQuery, map[string]interface{}{"id": projectID}, &resp); err != nil {
		return nil, fmt.Errorf("querying linked repos for project %s: %w", projectID, err)
	}

	nodes := resp.Node.Repositories.Nodes
	repos := make([]LinkedRepo, 0, len(nodes))
	for _, n := range nodes {
		repos = append(repos, LinkedRepo{
			Name:          n.Name,
			NameWithOwner: n.NameWithOwner,
			URL:           n.URL,
		})
	}
	return repos, nil
}
