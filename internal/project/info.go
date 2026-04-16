package project

import (
	"fmt"
	"io"
	"strings"
)

// ProjectState holds the resolved state of a repo's project affiliation.
type ProjectState struct {
	ProjectID      string
	ProjectName    string // human-readable title; empty if project was deleted
	ProjectDeleted bool   // true if the ID is set but the project no longer exists
	Topology       Topology
	ControlPlane   LinkedRepo
	LinkedRepos    []LinkedRepo
	AIVersion      string
	ControlPlaneFrameworkVersion string // AGENTIC_FRAMEWORK_VERSION from the control plane repo
	VersionInSync                bool   // true if local and control plane versions match
}

// ResolveState reads AGENTIC_PROJECT_ID, queries the API, and derives topology.
// Returns an error if the repo has no project affiliation.
func ResolveState(deps Deps) (*ProjectState, error) {
	projectID, err := deps.GetRepoVariable(deps.Owner, deps.RepoName, ProjectVarName)
	if err != nil {
		return nil, fmt.Errorf("this repo is not part of an agentic project (%s not set): %w", ProjectVarName, err)
	}
	if projectID == "" {
		return nil, fmt.Errorf("this repo is not part of an agentic project (%s is empty)", ProjectVarName)
	}

	// Resolve project title and detect deletion.
	// If FetchProjectTitle is wired and returns empty/error, the project node no
	// longer exists — flag it as deleted regardless of other API results.
	projectName := projectID // safe fallback
	projectDeleted := false
	if deps.FetchProjectTitle != nil {
		if title, err := deps.FetchProjectTitle(projectID); err != nil || title == "" {
			projectDeleted = true
		} else {
			projectName = title
		}
	}

	linked, err := deps.FetchLinkedRepos(projectID)
	if err != nil || projectDeleted {
		// Project is inaccessible or confirmed deleted.
		aiVersion, _ := deps.ReadAIVersion(deps.Root)
		return &ProjectState{
			ProjectID:      projectID,
			ProjectName:    projectName,
			ProjectDeleted: true,
			AIVersion:      aiVersion,
		}, nil
	}

	topo := DetectTopology(deps.RepoFullName, linked)
	cp, _ := ControlPlaneRepo(linked)
	aiVersion, _ := deps.ReadAIVersion(deps.Root)

	// Fetch control plane framework version.
	var cpFrameworkVersion string
	if len(linked) > 0 {
		var cpOwner, cpRepo string
		if topo == TopologyFederated {
			if cp.NameWithOwner != "" {
				parts := strings.SplitN(cp.NameWithOwner, "/", 2)
				if len(parts) == 2 {
					cpOwner, cpRepo = parts[0], parts[1]
				}
			}
		} else {
			cpOwner, cpRepo = deps.Owner, deps.RepoName
		}
		if cpOwner != "" {
			cpFrameworkVersion, _ = deps.GetRepoVariable(cpOwner, cpRepo, FrameworkVersionVarName)
		}
	}

	return &ProjectState{
		ProjectID:                    projectID,
		ProjectName:                  projectName,
		ProjectDeleted:               projectDeleted,
		Topology:                     topo,
		ControlPlane:                 cp,
		LinkedRepos:                  linked,
		AIVersion:                    aiVersion,
		ControlPlaneFrameworkVersion: cpFrameworkVersion,
		VersionInSync:                aiVersion == cpFrameworkVersion || cpFrameworkVersion == "",
	}, nil
}

// PrintInfo writes a human-readable project info summary to w.
func PrintInfo(w io.Writer, deps Deps) error {
	state, err := ResolveState(deps)
	if err != nil {
		return err
	}

	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  gh agentic — agentic project info")
	fmt.Fprintln(w, "")

	if state.ProjectDeleted {
		fmt.Fprintf(w, "  %-20s %s (%s)\n", "Agentic project:", "⚠ agentic project not found — may have been deleted", state.ProjectID)
		fmt.Fprintf(w, "  %-20s %s\n", "", "→ run 'gh agentic project unlink' or 'gh agentic project init'")
	} else {
		fmt.Fprintf(w, "  %-20s %s (%s)\n", "Agentic project:", state.ProjectName, state.ProjectID)
		fmt.Fprintf(w, "  %-20s %s\n", "Topology:", string(state.Topology))
		if state.ControlPlane.NameWithOwner != "" {
			fmt.Fprintf(w, "  %-20s %s\n", "Control plane:", state.ControlPlane.NameWithOwner)
		}
	}

	if state.AIVersion != "" {
		fmt.Fprintf(w, "  %-20s %s\n", "Framework:", state.AIVersion)
	} else {
		fmt.Fprintf(w, "  %-20s %s\n", "Framework:", "not mounted")
	}

	fmt.Fprintln(w, "")
	return nil
}
