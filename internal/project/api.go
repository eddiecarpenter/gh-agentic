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

// --- Topology detection ---

// DetectTopology compares the current repo against the project's linked repos.
// If the current repo is the single linked repo → Single.
// If the linked repo is a different repo → Federated.
// If there are no linked repos → Unknown.
func DetectTopology(currentOwnerRepo string, linked []LinkedRepo) Topology {
	if len(linked) == 0 {
		return TopologyUnknown
	}
	for _, r := range linked {
		if strings.EqualFold(r.NameWithOwner, currentOwnerRepo) {
			return TopologySingle
		}
	}
	return TopologyFederated
}

// ControlPlaneRepo returns the control plane repo from the linked repos.
// For Single topology this is the current repo; for Federated it is the linked repo.
func ControlPlaneRepo(linked []LinkedRepo) (LinkedRepo, bool) {
	if len(linked) == 0 {
		return LinkedRepo{}, false
	}
	return linked[0], true
}

// --- Production implementations ---

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
