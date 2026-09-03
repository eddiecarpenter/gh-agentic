package project

import (
	"fmt"
)

// Project-fields fetch logic. Kept out of api.go so it stays covered — see the
// note in paginate.go.
//
// This connection matters more than most. EnsureTargetRepoField (fields.go)
// creates the "Target repo" field when it does not find one, so a truncated
// field list does not merely misreport — it causes gh agentic repair to create a
// duplicate field on the user's project. Returning a partial list from here is
// therefore a write hazard, not a display bug.

// graphqlProjectFieldsResponse is the response shape for the project fields query.
type graphqlProjectFieldsResponse struct {
	Node struct {
		Fields struct {
			Nodes []struct {
				ID       string `json:"id"`
				Name     string `json:"name"`
				DataType string `json:"dataType"`
				Options  []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"options"`
			} `json:"nodes"`
			PageInfo pageInfo `json:"pageInfo"`
		} `json:"fields"`
	} `json:"node"`
}

// projectFieldsQuery lists a ProjectV2's fields, one page at a time.
const projectFieldsQuery = `query($id: ID!, $after: String) {
	node(id: $id) {
		... on ProjectV2 {
			fields(first: 100, after: $after) {
				nodes {
					... on ProjectV2Field { id name dataType }
					... on ProjectV2SingleSelectField {
						id name dataType
						options { id name }
					}
					... on ProjectV2IterationField { id name dataType }
				}
				pageInfo {
					hasNextPage
					endCursor
				}
			}
		}
	}
}`

// fetchProjectFields executes the project-fields query through doer, following
// the connection's cursor until it is exhausted.
//
// Any page error aborts the whole fetch and returns no fields. Callers use the
// result to decide whether a field exists, and a caller that concludes "absent"
// from a truncated list will create a duplicate.
func fetchProjectFields(doer graphQLDoer, projectID string) ([]ProjectField, error) {
	var fields []ProjectField

	err := paginate(func(after interface{}) (pageInfo, error) {
		var resp graphqlProjectFieldsResponse
		vars := map[string]interface{}{"id": projectID, "after": after}
		if err := doer.Do(projectFieldsQuery, vars, &resp); err != nil {
			return pageInfo{}, fmt.Errorf("fetching fields for project %s: %w", projectID, err)
		}

		for _, n := range resp.Node.Fields.Nodes {
			f := ProjectField{ID: n.ID, Name: n.Name, DataType: n.DataType}
			for _, o := range n.Options {
				f.Options = append(f.Options, FieldOption{ID: o.ID, Name: o.Name})
			}
			fields = append(fields, f)
		}
		return resp.Node.Fields.PageInfo, nil
	})
	if err != nil {
		return nil, err
	}

	if fields == nil {
		fields = []ProjectField{}
	}
	return fields, nil
}
