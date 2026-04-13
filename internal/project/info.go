package project

import (
	"fmt"
	"io"
)

// ProjectState holds the resolved state of a repo's project affiliation.
type ProjectState struct {
	ProjectID    string
	Topology     Topology
	ControlPlane LinkedRepo
	LinkedRepos  []LinkedRepo
	AIVersion    string
}

// ResolveState reads AGENTIC_PROJECT_ID, queries the API, and derives topology.
// Returns an error if the repo has no project affiliation.
func ResolveState(deps Deps) (*ProjectState, error) {
	projectID, err := deps.GetRepoVariable(deps.Owner, deps.RepoName, ProjectVarName)
	if err != nil {
		return nil, fmt.Errorf("repo is not affiliated with a project (%s not set): %w", ProjectVarName, err)
	}
	if projectID == "" {
		return nil, fmt.Errorf("repo is not affiliated with a project (%s is empty)", ProjectVarName)
	}

	linked, err := deps.FetchLinkedRepos(projectID)
	if err != nil {
		return nil, fmt.Errorf("querying project linked repos: %w", err)
	}

	topo := DetectTopology(deps.RepoFullName, linked)
	cp, _ := ControlPlaneRepo(linked)

	aiVersion, _ := deps.ReadAIVersion(deps.Root)

	return &ProjectState{
		ProjectID:    projectID,
		Topology:     topo,
		ControlPlane: cp,
		LinkedRepos:  linked,
		AIVersion:    aiVersion,
	}, nil
}

// PrintInfo writes a human-readable project info summary to w.
func PrintInfo(w io.Writer, deps Deps) error {
	state, err := ResolveState(deps)
	if err != nil {
		return err
	}

	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  gh agentic — project info")
	fmt.Fprintln(w, "")
	fmt.Fprintf(w, "  %-20s %s\n", "Project ID:", state.ProjectID)
	fmt.Fprintf(w, "  %-20s %s\n", "Topology:", string(state.Topology))

	if state.ControlPlane.NameWithOwner != "" {
		fmt.Fprintf(w, "  %-20s %s\n", "Control plane:", state.ControlPlane.NameWithOwner)
	}

	if state.AIVersion != "" {
		fmt.Fprintf(w, "  %-20s %s\n", "Framework:", state.AIVersion)
	} else {
		fmt.Fprintf(w, "  %-20s %s\n", "Framework:", "not mounted")
	}

	fmt.Fprintln(w, "")
	return nil
}
