package project

import (
	"fmt"
	"io"
)

// ProjectState holds the resolved state of a repo's project affiliation.
// Topology is now a plain string ("single" or "federation") derived from
// FEDERATION.md presence; the legacy Topology enum and ControlPlane /
// LinkedRepos fields have been removed as part of Feature #824.
type ProjectState struct {
	ProjectID        string
	ProjectName      string // human-readable title; falls back to ID if project was deleted
	ProjectDeleted   bool   // true if the ID is set but the project no longer exists
	Topology         string // "single" or "federation"
	AIVersion        string // local .ai-version / git-describe version
	FrameworkVersion string // AGENTIC_FRAMEWORK_VERSION on this repo (was ControlPlaneFrameworkVersion)
	VersionInSync    bool   // true if local and remote versions match
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

	return &ProjectState{
		ProjectID:        ctx.ProjectID,
		ProjectName:      ctx.ProjectName,
		ProjectDeleted:   ctx.ProjectDeleted,
		Topology:         ctx.Topology,
		AIVersion:        ctx.LocalAIVersion,
		FrameworkVersion: ctx.FrameworkVersion,
		VersionInSync:    ctx.VersionInSync,
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
		fmt.Fprintf(w, "  %-20s %s\n", "Topology:", state.Topology)
	}

	if state.AIVersion != "" {
		fmt.Fprintf(w, "  %-20s %s\n", "Framework:", state.AIVersion)
	} else {
		fmt.Fprintf(w, "  %-20s %s\n", "Framework:", "not mounted")
	}

	fmt.Fprintln(w, "")
	return nil
}
