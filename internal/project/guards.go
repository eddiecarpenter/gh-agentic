package project

import (
	"os"
	"path/filepath"
)

// JoinGuard describes the outcome of the guard evaluation for join/unlink operations.
type JoinGuard int

const (
	// JoinGuardClear means no impediment — proceed.
	JoinGuardClear JoinGuard = iota
	// JoinGuardSameProject means the repo is already affiliated with the requested project.
	JoinGuardSameProject
	// JoinGuardWarnConfirm means the operation should warn and ask for confirmation.
	JoinGuardWarnConfirm
	// JoinGuardBlocked means the operation is blocked and must not proceed.
	JoinGuardBlocked
)

// GuardResult is the evaluated outcome of a join or unlink guard check.
type GuardResult struct {
	Guard   JoinGuard
	Message string
}

// HasDocsContent returns true if docs/ at root contains at least one file.
func HasDocsContent(root string) bool {
	docsDir := filepath.Join(root, "docs")
	entries, err := os.ReadDir(docsDir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() {
			return true
		}
		// Recursively check sub-directories.
		sub := filepath.Join(docsDir, e.Name())
		subEntries, err := os.ReadDir(sub)
		if err == nil && len(subEntries) > 0 {
			return true
		}
	}
	return false
}

// EvalJoinGuard evaluates the guard matrix for a project join operation.
// Returns JoinGuardClear when the operation can proceed without further action.
// Returns JoinGuardSameProject when already affiliated with newProjectID.
// Returns JoinGuardWarnConfirm when a confirmation is required before proceeding.
// Returns JoinGuardBlocked when the operation must not proceed.
func EvalJoinGuard(deps Deps, newProjectID string) (GuardResult, error) {
	currentProjectID, err := deps.GetRepoVariable(deps.Owner, deps.RepoName, ProjectVarName)
	if err != nil || currentProjectID == "" {
		// Not currently affiliated — clear to join.
		return GuardResult{Guard: JoinGuardClear}, nil
	}

	currentName := ProjectDisplayName(deps, currentProjectID)

	if currentProjectID == newProjectID {
		return GuardResult{
			Guard:   JoinGuardSameProject,
			Message: "this repo is already part of agentic project \"" + currentName + "\"",
		}, nil
	}

	newName := ProjectDisplayName(deps, newProjectID)

	// Already affiliated with a different project — check topology to decide severity.
	linked, err := deps.FetchLinkedRepos(currentProjectID)
	if err != nil {
		// Cannot determine topology — treat as warn+confirm.
		return GuardResult{
			Guard:   JoinGuardWarnConfirm,
			Message: "repo is currently part of agentic project \"" + currentName + "\"; re-affiliating will change agentic project membership",
		}, nil
	}

	topo := DetectTopology(deps.RepoFullName, linked)
	if topo == TopologySingle {
		// This repo is the current control plane.
		if HasDocsContent(deps.Root) {
			return GuardResult{
				Guard:   JoinGuardBlocked,
				Message: "this repo is the agentic project control plane and docs/ has content — migrate to the new agentic project control plane first",
			}, nil
		}
		return GuardResult{
			Guard:   JoinGuardWarnConfirm,
			Message: "this repo is the agentic project control plane; moving it to a different agentic project will remove it as control plane",
		}, nil
	}

	// Federated member re-affiliating.
	return GuardResult{
		Guard:   JoinGuardWarnConfirm,
		Message: "repo is currently part of agentic project \"" + currentName + "\"; re-affiliating to \"" + newName + "\"",
	}, nil
}

// EvalPreJoinWarning checks current repo state to surface any warning before a project
// selection UI is shown. It is independent of the target project ID.
// Returns JoinGuardBlocked if the operation must not proceed at all.
// Returns JoinGuardWarnConfirm with a message if a heads-up should be shown.
// Returns JoinGuardClear if no warning is needed.
func EvalPreJoinWarning(deps Deps) (GuardResult, error) {
	currentProjectID, err := deps.GetRepoVariable(deps.Owner, deps.RepoName, ProjectVarName)
	if err != nil || currentProjectID == "" {
		return GuardResult{Guard: JoinGuardClear}, nil
	}

	linked, err := deps.FetchLinkedRepos(currentProjectID)
	if err != nil {
		// Cannot determine topology — no pre-warning, let EvalJoinGuard handle it post-selection.
		return GuardResult{Guard: JoinGuardClear}, nil
	}

	topo := DetectTopology(deps.RepoFullName, linked)
	if topo == TopologySingle {
		if HasDocsContent(deps.Root) {
			return GuardResult{
				Guard:   JoinGuardBlocked,
				Message: "this repo is the agentic project control plane and docs/ has content — migrate to the new agentic project control plane first",
			}, nil
		}
		return GuardResult{
			Guard:   JoinGuardWarnConfirm,
			Message: "this repo is the agentic project control plane; moving it to a different agentic project will remove it as control plane",
		}, nil
	}

	return GuardResult{Guard: JoinGuardClear}, nil
}

// EvalUnlinkGuard evaluates the guard for an unlink operation.
func EvalUnlinkGuard(deps Deps) (GuardResult, error) {
	currentProjectID, err := deps.GetRepoVariable(deps.Owner, deps.RepoName, ProjectVarName)
	if err != nil || currentProjectID == "" {
		return GuardResult{
			Guard:   JoinGuardClear,
			Message: "nothing to unlink — this repo is not part of an agentic project",
		}, nil
	}

	// Check if this is the control plane.
	linked, err := deps.FetchLinkedRepos(currentProjectID)
	if err != nil {
		// Cannot determine topology — warn+confirm is safe.
		return GuardResult{
			Guard:   JoinGuardWarnConfirm,
			Message: "remove this repo from the agentic project?",
		}, nil
	}

	topo := DetectTopology(deps.RepoFullName, linked)
	if topo == TopologySingle {
		if HasDocsContent(deps.Root) {
			return GuardResult{
				Guard:   JoinGuardBlocked,
				Message: "this repo is the agentic project control plane and docs/ has content — migrate to the new agentic project control plane first before unlinking",
			}, nil
		}
		return GuardResult{
			Guard:   JoinGuardWarnConfirm,
			Message: "this repo is the agentic project control plane — unlinking will remove it from the agentic project",
		}, nil
	}

	return GuardResult{
		Guard:   JoinGuardWarnConfirm,
		Message: "remove this repo from the agentic project?",
	}, nil
}
