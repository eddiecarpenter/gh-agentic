package project

import (
	"fmt"
)

// Project-lookup fetch logic: the ProjectV2 connections hanging off a repository,
// an organization, and the authenticated viewer. Kept out of api.go so it stays
// covered — see the note in paginate.go.

// projectNodes is the shared shape of a projectsV2 connection: id/title nodes
// plus the cursor. All three lookups below select exactly this.
type projectNodes struct {
	Nodes []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	} `json:"nodes"`
	PageInfo pageInfo `json:"pageInfo"`
}

// graphqlProjectsForRepoResponse is the response shape for the repo projects query.
type graphqlProjectsForRepoResponse struct {
	Repository struct {
		ProjectsV2 projectNodes `json:"projectsV2"`
	} `json:"repository"`
}

// graphqlOrgProjectsResponse is the response shape for the org projects query.
type graphqlOrgProjectsResponse struct {
	Organization struct {
		ProjectsV2 projectNodes `json:"projectsV2"`
	} `json:"organization"`
}

// graphqlUserProjectsResponse is the response shape for the viewer projects query.
type graphqlUserProjectsResponse struct {
	Viewer struct {
		ProjectsV2 projectNodes `json:"projectsV2"`
	} `json:"viewer"`
}

const projectsForRepoQuery = `query($owner: String!, $repo: String!, $after: String) {
	repository(owner: $owner, name: $repo) {
		projectsV2(first: 100, after: $after) {
			nodes { id title }
			pageInfo { hasNextPage endCursor }
		}
	}
}`

const orgProjectsQuery = `query($login: String!, $after: String) {
	organization(login: $login) {
		projectsV2(first: 100, after: $after) {
			nodes { id title }
			pageInfo { hasNextPage endCursor }
		}
	}
}`

const viewerProjectsQuery = `query($after: String) {
	viewer {
		projectsV2(first: 100, after: $after) {
			nodes { id title }
			pageInfo { hasNextPage endCursor }
		}
	}
}`

// collectProjects appends a page's nodes to projects and returns its pageInfo.
func collectProjects(projects *[]ProjectInfo, page projectNodes) pageInfo {
	for _, n := range page.Nodes {
		*projects = append(*projects, ProjectInfo{ID: n.ID, Title: n.Title})
	}
	return page.PageInfo
}

// fetchProjectsForRepo lists the ProjectV2s linked to a repository.
func fetchProjectsForRepo(doer graphQLDoer, owner, repo string) ([]ProjectInfo, error) {
	var projects []ProjectInfo

	err := paginate(func(after interface{}) (pageInfo, error) {
		var resp graphqlProjectsForRepoResponse
		vars := map[string]interface{}{"owner": owner, "repo": repo, "after": after}
		if err := doer.Do(projectsForRepoQuery, vars, &resp); err != nil {
			return pageInfo{}, fmt.Errorf("querying projects for %s/%s: %w", owner, repo, err)
		}
		return collectProjects(&projects, resp.Repository.ProjectsV2), nil
	})
	if err != nil {
		return nil, err
	}
	if projects == nil {
		projects = []ProjectInfo{}
	}
	return projects, nil
}

// fetchProjectsForOwner lists the ProjectV2s owned by an organization, or by the
// authenticated user when ownerType is not "Organization".
func fetchProjectsForOwner(doer graphQLDoer, owner, ownerType string) ([]ProjectInfo, error) {
	var projects []ProjectInfo

	err := paginate(func(after interface{}) (pageInfo, error) {
		if ownerType == "Organization" {
			var resp graphqlOrgProjectsResponse
			vars := map[string]interface{}{"login": owner, "after": after}
			if err := doer.Do(orgProjectsQuery, vars, &resp); err != nil {
				return pageInfo{}, fmt.Errorf("fetching projects for org %s: %w", owner, err)
			}
			return collectProjects(&projects, resp.Organization.ProjectsV2), nil
		}

		// Viewer query — no owner parameter; it resolves the authenticated user.
		// It still needs a variables map now that it threads $after.
		var resp graphqlUserProjectsResponse
		vars := map[string]interface{}{"after": after}
		if err := doer.Do(viewerProjectsQuery, vars, &resp); err != nil {
			return pageInfo{}, fmt.Errorf("fetching projects for user %s: %w", owner, err)
		}
		return collectProjects(&projects, resp.Viewer.ProjectsV2), nil
	})
	if err != nil {
		return nil, err
	}
	if projects == nil {
		projects = []ProjectInfo{}
	}
	return projects, nil
}
