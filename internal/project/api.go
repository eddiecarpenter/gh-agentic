// Package project implements GitHub ProjectV2 API helpers for the
// gh agentic project command group.
package project

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
)

// Topology describes whether a repo is an embedded control plane (Single)
// or a domain repo in a federated setup (Federated).
type Topology string

const (
	// TopologySingle means this repo IS the control plane (linked repo == current repo).
	TopologySingle Topology = "Single"
	// TopologyFederated means this repo is a domain repo; the control plane is elsewhere.
	TopologyFederated Topology = "Federated"
	// TopologyUnknown means topology could not be determined (no project affiliation).
	TopologyUnknown Topology = "Unknown"
)

// ProjectVarName is the repo-level variable that stores the GitHub ProjectV2 node ID.
const ProjectVarName = "AGENTIC_PROJECT_ID"

const TopologyVarName = "AGENTIC_TOPOLOGY"
const FrameworkVersionVarName = "AGENTIC_FRAMEWORK_VERSION"

// LinkedRepo represents a repository linked to a GitHub ProjectV2.
type LinkedRepo struct {
	Name          string
	NameWithOwner string
	URL           string
}

// ProjectInfo represents a GitHub ProjectV2.
type ProjectInfo struct {
	ID    string
	Title string
}

// --- Function types (injectable for tests) ---

// FetchLinkedReposFunc fetches repositories linked to a ProjectV2 by its node ID.
type FetchLinkedReposFunc func(projectID string) ([]LinkedRepo, error)

// FetchProjectsForRepoFunc fetches projects linked to a repository.
type FetchProjectsForRepoFunc func(owner, repo string) ([]ProjectInfo, error)

// GetRepoVariableFunc reads a repo-level Actions variable value.
type GetRepoVariableFunc func(owner, repo, name string) (string, error)

// SetRepoVariableFunc creates or updates a repo-level Actions variable.
type SetRepoVariableFunc func(owner, repo, name, value string) error

// DeleteRepoVariableFunc deletes a repo-level Actions variable.
type DeleteRepoVariableFunc func(owner, repo, name string) error

// FetchOwnerAndRepoIDsFunc fetches the GraphQL node IDs for a GitHub owner and repository.
type FetchOwnerAndRepoIDsFunc func(owner, repo string) (ownerID, repoID string, err error)

// CreateProjectFunc creates a new GitHub ProjectV2 and returns its node ID.
type CreateProjectFunc func(ownerID, title string) (projectID string, err error)

// LinkRepoToProjectFunc links a repository to a GitHub ProjectV2.
type LinkRepoToProjectFunc func(projectID, repoID string) error

// ConfirmFunc prompts the user for a yes/no confirmation. Returns true if confirmed.
type ConfirmFunc func(prompt string) (bool, error)

// UpdateProjectFunc updates a project's short description and readme.
type UpdateProjectFunc func(projectID, shortDescription, readme string) error

// FetchProjectNumberFunc fetches the numeric project number from a ProjectV2 node ID.
type FetchProjectNumberFunc func(projectID string) (int, error)

// CreateProjectViewFunc creates a view in a GitHub ProjectV2 via the REST API.
// owner is the GitHub login; ownerType is "Organization" or "User";
// projectNumber is the numeric project number; layout is "table", "board", or "roadmap".
type CreateProjectViewFunc func(owner, ownerType string, projectNumber int, name, layout, filter string) error

// ProjectField represents a field on a GitHub ProjectV2.
type ProjectField struct {
	ID       string
	Name     string
	DataType string
}

// FetchProjectFieldsFunc fetches the fields defined on a ProjectV2.
type FetchProjectFieldsFunc func(projectID string) ([]ProjectField, error)

// UpdateStatusFieldOptionsFunc replaces the options on a single-select field.
type UpdateStatusFieldOptionsFunc func(fieldID string, options []StatusOption) error

// --- Production implementations ---
//
// Pure topology-detection helpers (DetectTopology, ControlPlaneRepo)
// were extracted to topology_detect.go so they remain coverable while
// this file is excluded from coverage as network-bound.

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
		} `json:"repositories"`
	} `json:"node"`
}

// DefaultFetchLinkedRepos queries the GitHub GraphQL API for repos linked to a ProjectV2.
func DefaultFetchLinkedRepos(projectID string) ([]LinkedRepo, error) {
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return nil, fmt.Errorf("creating GraphQL client: %w", err)
	}

	query := `query($id: ID!) {
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

	var resp graphqlLinkedReposResponse
	if err := client.Do(query, map[string]interface{}{"id": projectID}, &resp); err != nil {
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

// graphqlProjectsForRepoResponse is the response shape for the repo projects query.
type graphqlProjectsForRepoResponse struct {
	Repository struct {
		ProjectsV2 struct {
			Nodes []struct {
				ID    string `json:"id"`
				Title string `json:"title"`
			} `json:"nodes"`
		} `json:"projectsV2"`
	} `json:"repository"`
}

// DefaultFetchProjectsForRepo queries the GitHub GraphQL API for projects linked to a repo.
func DefaultFetchProjectsForRepo(owner, repo string) ([]ProjectInfo, error) {
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return nil, fmt.Errorf("creating GraphQL client: %w", err)
	}

	query := `query($owner: String!, $repo: String!) {
		repository(owner: $owner, name: $repo) {
			projectsV2(first: 10) {
				nodes {
					id
					title
				}
			}
		}
	}`

	var resp graphqlProjectsForRepoResponse
	if err := client.Do(query, map[string]interface{}{"owner": owner, "repo": repo}, &resp); err != nil {
		return nil, fmt.Errorf("querying projects for %s/%s: %w", owner, repo, err)
	}

	nodes := resp.Repository.ProjectsV2.Nodes
	projects := make([]ProjectInfo, 0, len(nodes))
	for _, n := range nodes {
		projects = append(projects, ProjectInfo{ID: n.ID, Title: n.Title})
	}
	return projects, nil
}

// repoVariableResponse is the REST response shape for a single variable.
type repoVariableResponse struct {
	Value string `json:"value"`
}

// DefaultGetRepoVariable reads a repo-level Actions variable value via the REST API.
func DefaultGetRepoVariable(owner, repo, name string) (string, error) {
	client, err := api.DefaultRESTClient()
	if err != nil {
		return "", fmt.Errorf("creating REST client: %w", err)
	}

	var resp repoVariableResponse
	endpoint := fmt.Sprintf("repos/%s/%s/actions/variables/%s", owner, repo, name)
	if err := client.Get(endpoint, &resp); err != nil {
		return "", fmt.Errorf("getting variable %s: %w", name, err)
	}
	return resp.Value, nil
}

// repoVariableRequest is the REST request body for creating/updating a variable.
type repoVariableRequest struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// DefaultSetRepoVariable creates or updates a repo-level Actions variable.
// It always targets the repo level — never org level.
func DefaultSetRepoVariable(owner, repo, name, value string) error {
	client, err := api.DefaultRESTClient()
	if err != nil {
		return fmt.Errorf("creating REST client: %w", err)
	}

	body := repoVariableRequest{Name: name, Value: value}
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshalling variable request: %w", err)
	}

	patchEndpoint := fmt.Sprintf("repos/%s/%s/actions/variables/%s", owner, repo, name)

	// Try PATCH (update) first; fall back to POST (create) if not found.
	if err := client.Patch(patchEndpoint, bytes.NewReader(data), nil); err != nil {
		data2, _ := json.Marshal(body)
		createEndpoint := fmt.Sprintf("repos/%s/%s/actions/variables", owner, repo)
		if err2 := client.Post(createEndpoint, bytes.NewReader(data2), nil); err2 != nil {
			return fmt.Errorf("setting variable %s: %w", name, err2)
		}
	}
	return nil
}

// DefaultDeleteRepoVariable deletes a repo-level Actions variable.
func DefaultDeleteRepoVariable(owner, repo, name string) error {
	client, err := api.DefaultRESTClient()
	if err != nil {
		return fmt.Errorf("creating REST client: %w", err)
	}

	endpoint := fmt.Sprintf("repos/%s/%s/actions/variables/%s", owner, repo, name)
	if err := client.Delete(endpoint, nil); err != nil {
		return fmt.Errorf("deleting variable %s: %w", name, err)
	}
	return nil
}

// graphqlOwnerAndRepoIDsResponse is the response shape for the owner+repo ID query.
type graphqlOwnerAndRepoIDsResponse struct {
	RepositoryOwner struct {
		ID string `json:"id"`
	} `json:"repositoryOwner"`
	Repository struct {
		ID string `json:"id"`
	} `json:"repository"`
}

// DefaultFetchOwnerAndRepoIDs fetches the GraphQL node IDs for the given owner and repo.
func DefaultFetchOwnerAndRepoIDs(owner, repo string) (ownerID, repoID string, err error) {
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return "", "", fmt.Errorf("creating GraphQL client: %w", err)
	}

	query := `query($owner: String!, $name: String!) {
		repositoryOwner(login: $owner) { id }
		repository(owner: $owner, name: $name) { id }
	}`

	var resp graphqlOwnerAndRepoIDsResponse
	if err := client.Do(query, map[string]interface{}{"owner": owner, "name": repo}, &resp); err != nil {
		return "", "", fmt.Errorf("fetching owner/repo IDs for %s/%s: %w", owner, repo, err)
	}
	return resp.RepositoryOwner.ID, resp.Repository.ID, nil
}

// graphqlCreateProjectResponse is the response shape for the createProjectV2 mutation.
type graphqlCreateProjectResponse struct {
	CreateProjectV2 struct {
		ProjectV2 struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"projectV2"`
	} `json:"createProjectV2"`
}

// DefaultCreateProject creates a new GitHub ProjectV2 under the given owner node ID.
func DefaultCreateProject(ownerID, title string) (projectID string, err error) {
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return "", fmt.Errorf("creating GraphQL client: %w", err)
	}

	mutation := `mutation($ownerId: ID!, $title: String!) {
		createProjectV2(input: {ownerId: $ownerId, title: $title}) {
			projectV2 { id title }
		}
	}`

	var resp graphqlCreateProjectResponse
	if err := client.Do(mutation, map[string]interface{}{"ownerId": ownerID, "title": title}, &resp); err != nil {
		return "", fmt.Errorf("creating project %q: %w", title, err)
	}
	return resp.CreateProjectV2.ProjectV2.ID, nil
}

// graphqlLinkRepoResponse is the response shape for the linkProjectV2ToRepository mutation.
type graphqlLinkRepoResponse struct {
	LinkProjectV2ToRepository struct {
		Repository struct {
			ID string `json:"id"`
		} `json:"repository"`
	} `json:"linkProjectV2ToRepository"`
}

// DefaultLinkRepoToProject links the given repository to a GitHub ProjectV2.
func DefaultLinkRepoToProject(projectID, repoID string) error {
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return fmt.Errorf("creating GraphQL client: %w", err)
	}

	mutation := `mutation($projectId: ID!, $repositoryId: ID!) {
		linkProjectV2ToRepository(input: {projectId: $projectId, repositoryId: $repositoryId}) {
			repository { id }
		}
	}`

	var resp graphqlLinkRepoResponse
	if err := client.Do(mutation, map[string]interface{}{"projectId": projectID, "repositoryId": repoID}, &resp); err != nil {
		return fmt.Errorf("linking repo to project: %w", err)
	}
	return nil
}

// graphqlUpdateProjectResponse is the response shape for the updateProjectV2 mutation.
type graphqlUpdateProjectResponse struct {
	UpdateProjectV2 struct {
		ProjectV2 struct {
			ID string `json:"id"`
		} `json:"projectV2"`
	} `json:"updateProjectV2"`
}

// DefaultUpdateProject updates the short description and readme of a GitHub ProjectV2.
func DefaultUpdateProject(projectID, shortDescription, readme string) error {
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return fmt.Errorf("creating GraphQL client: %w", err)
	}

	mutation := `mutation($projectId: ID!, $shortDescription: String!, $readme: String!) {
		updateProjectV2(input: {projectId: $projectId, shortDescription: $shortDescription, readme: $readme}) {
			projectV2 { id }
		}
	}`

	var resp graphqlUpdateProjectResponse
	if err := client.Do(mutation, map[string]interface{}{
		"projectId":        projectID,
		"shortDescription": shortDescription,
		"readme":           readme,
	}, &resp); err != nil {
		return fmt.Errorf("updating project %s: %w", projectID, err)
	}
	return nil
}

// graphqlProjectFieldsResponse is the response shape for the project fields query.
type graphqlProjectFieldsResponse struct {
	Node struct {
		Fields struct {
			Nodes []struct {
				ID       string `json:"id"`
				Name     string `json:"name"`
				DataType string `json:"dataType"`
			} `json:"nodes"`
		} `json:"fields"`
	} `json:"node"`
}

// DefaultFetchProjectFields fetches the fields defined on a GitHub ProjectV2.
func DefaultFetchProjectFields(projectID string) ([]ProjectField, error) {
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return nil, fmt.Errorf("creating GraphQL client: %w", err)
	}

	query := `query($id: ID!) {
		node(id: $id) {
			... on ProjectV2 {
				fields(first: 20) {
					nodes {
						... on ProjectV2Field { id name dataType }
						... on ProjectV2SingleSelectField { id name dataType }
						... on ProjectV2IterationField { id name dataType }
					}
				}
			}
		}
	}`

	var resp graphqlProjectFieldsResponse
	if err := client.Do(query, map[string]interface{}{"id": projectID}, &resp); err != nil {
		return nil, fmt.Errorf("fetching fields for project %s: %w", projectID, err)
	}

	fields := make([]ProjectField, 0, len(resp.Node.Fields.Nodes))
	for _, n := range resp.Node.Fields.Nodes {
		fields = append(fields, ProjectField{ID: n.ID, Name: n.Name, DataType: n.DataType})
	}
	return fields, nil
}

// graphqlUpdateFieldResponse is the response shape for the updateProjectV2Field mutation.
type graphqlUpdateFieldResponse struct {
	UpdateProjectV2Field struct {
		ProjectV2Field struct {
			ID string `json:"id"`
		} `json:"projectV2Field"`
	} `json:"updateProjectV2Field"`
}

// statusOptionInput mirrors ProjectV2SingleSelectFieldOptionInput for JSON serialisation.
type statusOptionInput struct {
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
}

// DefaultUpdateStatusFieldOptions replaces the options on the project's Status single-select field.
func DefaultUpdateStatusFieldOptions(fieldID string, options []StatusOption) error {
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return fmt.Errorf("creating GraphQL client: %w", err)
	}

	mutation := `mutation($fieldId: ID!, $options: [ProjectV2SingleSelectFieldOptionInput!]) {
		updateProjectV2Field(input: {fieldId: $fieldId, singleSelectOptions: $options}) {
			projectV2Field {
				... on ProjectV2SingleSelectField { id name }
			}
		}
	}`

	opts := make([]statusOptionInput, 0, len(options))
	for _, o := range options {
		opts = append(opts, statusOptionInput{Name: o.Name, Color: o.Color, Description: o.Description})
	}

	var resp graphqlUpdateFieldResponse
	if err := client.Do(mutation, map[string]interface{}{
		"fieldId": fieldID,
		"options": opts,
	}, &resp); err != nil {
		return fmt.Errorf("updating status field options: %w", err)
	}
	return nil
}

// graphqlProjectNumberResponse is the response shape for the project number query.
type graphqlProjectNumberResponse struct {
	Node struct {
		Number int `json:"number"`
	} `json:"node"`
}

// DefaultFetchProjectNumber fetches the numeric project number for a given ProjectV2 node ID.
func DefaultFetchProjectNumber(projectID string) (int, error) {
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return 0, fmt.Errorf("creating GraphQL client: %w", err)
	}

	query := `query($id: ID!) {
		node(id: $id) {
			... on ProjectV2 { number }
		}
	}`

	var resp graphqlProjectNumberResponse
	if err := client.Do(query, map[string]interface{}{"id": projectID}, &resp); err != nil {
		return 0, fmt.Errorf("fetching project number for %s: %w", projectID, err)
	}
	if resp.Node.Number == 0 {
		return 0, fmt.Errorf("project %s not found or has no number", projectID)
	}
	return resp.Node.Number, nil
}

// ProjectView represents a view in a GitHub ProjectV2.
type ProjectView struct {
	Name string
}

// FetchProjectsForOwnerFunc lists all ProjectV2s owned by a user or organisation.
type FetchProjectsForOwnerFunc func(owner, ownerType string) ([]ProjectInfo, error)

// graphqlUserProjectsResponse is the response shape for the user projects query.
type graphqlUserProjectsResponse struct {
	Viewer struct {
		ProjectsV2 struct {
			Nodes []struct {
				ID    string `json:"id"`
				Title string `json:"title"`
			} `json:"nodes"`
		} `json:"projectsV2"`
	} `json:"viewer"`
}

// graphqlOrgProjectsResponse is the response shape for the org projects query.
type graphqlOrgProjectsResponse struct {
	Organization struct {
		ProjectsV2 struct {
			Nodes []struct {
				ID    string `json:"id"`
				Title string `json:"title"`
			} `json:"nodes"`
		} `json:"projectsV2"`
	} `json:"organization"`
}

// DefaultFetchProjectsForOwner lists all ProjectV2s for a user or organisation.
func DefaultFetchProjectsForOwner(owner, ownerType string) ([]ProjectInfo, error) {
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return nil, fmt.Errorf("creating GraphQL client: %w", err)
	}

	if ownerType == "Organization" {
		query := `query($login: String!) {
			organization(login: $login) {
				projectsV2(first: 50) {
					nodes { id title }
				}
			}
		}`
		var resp graphqlOrgProjectsResponse
		if err := client.Do(query, map[string]interface{}{"login": owner}, &resp); err != nil {
			return nil, fmt.Errorf("fetching projects for org %s: %w", owner, err)
		}
		nodes := resp.Organization.ProjectsV2.Nodes
		projects := make([]ProjectInfo, 0, len(nodes))
		for _, n := range nodes {
			projects = append(projects, ProjectInfo{ID: n.ID, Title: n.Title})
		}
		return projects, nil
	}

	// User — use viewer query (works for the authenticated user).
	query := `query {
		viewer {
			projectsV2(first: 50) {
				nodes { id title }
			}
		}
	}`
	var resp graphqlUserProjectsResponse
	if err := client.Do(query, map[string]interface{}{}, &resp); err != nil {
		return nil, fmt.Errorf("fetching projects for user %s: %w", owner, err)
	}
	nodes := resp.Viewer.ProjectsV2.Nodes
	projects := make([]ProjectInfo, 0, len(nodes))
	for _, n := range nodes {
		projects = append(projects, ProjectInfo{ID: n.ID, Title: n.Title})
	}
	return projects, nil
}

// FetchProjectViewsFunc lists the view names for a GitHub ProjectV2 by its node ID.
type FetchProjectViewsFunc func(projectID string) ([]ProjectView, error)

// graphqlProjectViewsResponse is the response shape for the project views query.
type graphqlProjectViewsResponse struct {
	Node struct {
		Views struct {
			Nodes []struct {
				Name string `json:"name"`
			} `json:"nodes"`
		} `json:"views"`
	} `json:"node"`
}

// DefaultFetchProjectViews lists the views for a GitHub ProjectV2 via GraphQL.
func DefaultFetchProjectViews(projectID string) ([]ProjectView, error) {
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return nil, fmt.Errorf("creating GraphQL client: %w", err)
	}

	query := `query($id: ID!) {
		node(id: $id) {
			... on ProjectV2 {
				views(first: 20) {
					nodes { name }
				}
			}
		}
	}`

	var resp graphqlProjectViewsResponse
	if err := client.Do(query, map[string]interface{}{"id": projectID}, &resp); err != nil {
		return nil, fmt.Errorf("fetching views for project %s: %w", projectID, err)
	}

	views := make([]ProjectView, 0, len(resp.Node.Views.Nodes))
	for _, n := range resp.Node.Views.Nodes {
		views = append(views, ProjectView{Name: n.Name})
	}
	return views, nil
}

// FetchProjectTitleFunc fetches the title of a ProjectV2 by its node ID.
type FetchProjectTitleFunc func(projectID string) (string, error)

// FetchProjectOwnerFunc fetches the owner login (user or organisation) that
// owns a GitHub ProjectV2. This is the primary disambiguator for federated-
// domain topology: if the project owner differs from the current repo's
// owner, the repo is a domain repo regardless of the linked-graph shape.
type FetchProjectOwnerFunc func(projectID string) (string, error)

// DefaultFetchProjectOwner fetches the owner login (user or organisation) of
// a GitHub ProjectV2 via GraphQL.
func DefaultFetchProjectOwner(projectID string) (string, error) {
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return "", fmt.Errorf("creating GraphQL client: %w", err)
	}

	query := `query($id: ID!) {
		node(id: $id) {
			... on ProjectV2 {
				owner {
					... on Organization { login }
					... on User { login }
				}
			}
		}
	}`

	var resp struct {
		Node struct {
			Owner struct {
				Login string `json:"login"`
			} `json:"owner"`
		} `json:"node"`
	}
	if err := client.Do(query, map[string]interface{}{"id": projectID}, &resp); err != nil {
		return "", fmt.Errorf("fetching owner for project %s: %w", projectID, err)
	}
	return resp.Node.Owner.Login, nil
}

// DefaultFetchProjectTitle fetches the title of a GitHub ProjectV2 via GraphQL.
func DefaultFetchProjectTitle(projectID string) (string, error) {
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return "", fmt.Errorf("creating GraphQL client: %w", err)
	}

	query := `query($id: ID!) {
		node(id: $id) {
			... on ProjectV2 {
				title
			}
		}
	}`

	var resp struct {
		Node struct {
			Title string `json:"title"`
		} `json:"node"`
	}
	if err := client.Do(query, map[string]interface{}{"id": projectID}, &resp); err != nil {
		return "", fmt.Errorf("fetching title for project %s: %w", projectID, err)
	}
	return resp.Node.Title, nil
}

// projectViewRequest is the REST request body for creating a project view.
type projectViewRequest struct {
	Name   string `json:"name"`
	Layout string `json:"layout"`
	Filter string `json:"filter,omitempty"`
}

// DefaultCreateProjectView creates a view in a GitHub ProjectV2 via the REST API.
// For organizations the endpoint is /orgs/{org}/projectsV2/{number}/views;
// for users it is /users/{login}/projectsV2/{number}/views.
func DefaultCreateProjectView(owner, ownerType string, projectNumber int, name, layout, filter string) error {
	client, err := api.DefaultRESTClient()
	if err != nil {
		return fmt.Errorf("creating REST client: %w", err)
	}

	var prefix string
	switch ownerType {
	case "Organization":
		prefix = fmt.Sprintf("orgs/%s", owner)
	default: // "User" or unknown
		prefix = fmt.Sprintf("users/%s", owner)
	}

	endpoint := fmt.Sprintf("%s/projectsV2/%d/views", prefix, projectNumber)

	body := projectViewRequest{
		Name:   name,
		Layout: strings.ToLower(strings.TrimSuffix(layout, "_LAYOUT")),
		Filter: filter,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshalling view request: %w", err)
	}

	var result interface{}
	if err := client.Post(endpoint, bytes.NewReader(data), &result); err != nil {
		return fmt.Errorf("creating view %q: %w", name, err)
	}
	return nil
}
