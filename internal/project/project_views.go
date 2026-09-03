package project

import (
	"fmt"
)

// Project-views fetch logic. Kept out of api.go so it stays covered — see the
// note in paginate.go.

// graphqlProjectViewsResponse is the response shape for the project views query.
type graphqlProjectViewsResponse struct {
	Node struct {
		Views struct {
			Nodes []struct {
				Name string `json:"name"`
			} `json:"nodes"`
			PageInfo pageInfo `json:"pageInfo"`
		} `json:"views"`
	} `json:"node"`
}

const projectViewsQuery = `query($id: ID!, $after: String) {
	node(id: $id) {
		... on ProjectV2 {
			views(first: 100, after: $after) {
				nodes { name }
				pageInfo { hasNextPage endCursor }
			}
		}
	}
}`

// fetchProjectViews lists a ProjectV2's views, following the cursor to the end.
func fetchProjectViews(doer graphQLDoer, projectID string) ([]ProjectView, error) {
	var views []ProjectView

	err := paginate(func(after interface{}) (pageInfo, error) {
		var resp graphqlProjectViewsResponse
		vars := map[string]interface{}{"id": projectID, "after": after}
		if err := doer.Do(projectViewsQuery, vars, &resp); err != nil {
			return pageInfo{}, fmt.Errorf("fetching views for project %s: %w", projectID, err)
		}
		for _, n := range resp.Node.Views.Nodes {
			views = append(views, ProjectView{Name: n.Name})
		}
		return resp.Node.Views.PageInfo, nil
	})
	if err != nil {
		return nil, err
	}
	if views == nil {
		views = []ProjectView{}
	}
	return views, nil
}
