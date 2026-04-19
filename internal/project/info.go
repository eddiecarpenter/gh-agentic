package project

import (
	"fmt"
	"io"
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

// ResolveState is retained as a thin wrapper over Resolve for call sites that
// still consume the legacy ProjectState shape. New code should call Resolve
// directly and consume the richer Context. ResolveState preserves the
// historic contract that an unaffiliated repo is an error — Resolve
// intentionally relaxes this so callers can inspect ctx.ProjectID
// themselves.
func ResolveState(deps Deps) (*ProjectState, error) {
	ctx, err := Resolve(deps)
	if err != nil {
		return nil, err
	}
	if ctx.ProjectID == "" {
		return nil, fmt.Errorf("this repo is not part of an agentic project (%s is empty)", ProjectVarName)
	}

	// The legacy Topology enum answers "is the current repo the single
	// linked repo on this project, or is the control plane elsewhere?"
	// which is a graph question, not the canonical-string question the
	// new Context.Topology answers. Compute it from the linked graph so
	// existing consumers (info.PrintInfo, info_test.go) see the same
	// result they always have.
	legacyTopology := DetectTopology(deps.RepoFullName, ctx.LinkedRepos)
	legacyCP, _ := ControlPlaneRepo(ctx.LinkedRepos)

	// ResolveState exposes the local AIVersion (from .ai-version) rather
	// than the authoritative FrameworkVersion — preserving the historic
	// shape until .ai-version is removed in #585.
	aiVersion := ctx.LocalAIVersion

	return &ProjectState{
		ProjectID:                    ctx.ProjectID,
		ProjectName:                  ctx.ProjectName,
		ProjectDeleted:               ctx.ProjectDeleted,
		Topology:                     legacyTopology,
		ControlPlane:                 legacyCP,
		LinkedRepos:                  ctx.LinkedRepos,
		AIVersion:                    aiVersion,
		ControlPlaneFrameworkVersion: ctx.FrameworkVersion,
		VersionInSync:                aiVersion == ctx.FrameworkVersion || ctx.FrameworkVersion == "",
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
