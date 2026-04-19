package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/eddiecarpenter/gh-agentic/internal/auth"
	"github.com/eddiecarpenter/gh-agentic/internal/scope"
)

// FederatedRequiresOrgMessage is the verbatim user-facing message emitted when
// a federated-topology operation is attempted against a user-owned repo.
// Kept as a package-level format string so every call site speaks with one
// voice and tests can assert the exact wording.
const FederatedRequiresOrgMessage = "Federated topology requires a GitHub Organization. The owner '%s' is a user account, which cannot host org-scoped variables and secrets. Either move this repo under an organisation, or use `--topology single`."

// EnsureFederatedOwnerIsOrg returns an error if a federated topology variant
// has been chosen against a user-owned repo. A user account cannot host
// org-scoped variables and secrets, so federated topology is refused before
// any state-changing side effect is performed.
//
// Parameters:
//   - topology  — one of "single", "federated", "federated-cp",
//     "federated-domain", or empty. Only federated variants are guarded.
//   - owner     — the GitHub owner login, interpolated into the error
//     message so operators see which account tripped the guard.
//   - ownerType — the string returned by auth.DetectOwnerType; one of
//     auth.OwnerTypeUser ("User") or auth.OwnerTypeOrg ("Organization").
//
// Returns nil when topology is not federated or when ownerType is not a user
// account. Returns an error with the verbatim FederatedRequiresOrgMessage
// otherwise.
func EnsureFederatedOwnerIsOrg(topology, owner, ownerType string) error {
	// Tolerate capitalised topology strings (the initv2 form emits
	// "Single"/"Federated"). The stricter scope.IsFederatedTopology helper
	// used by ScopeFor stays case-sensitive to avoid accidental scope
	// widening.
	normalised := strings.ToLower(topology)
	if !scope.IsFederatedTopology(normalised) {
		return nil
	}
	if ownerType != auth.OwnerTypeUser {
		return nil
	}
	return fmt.Errorf(FederatedRequiresOrgMessage, owner)
}

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
