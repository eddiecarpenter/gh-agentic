package project

import "strings"

// Canonical topology strings returned by ResolveTopology and consumed by
// downstream checks/repairs.
const (
	TopologyStringSingle          = "single"
	TopologyStringFederatedCP     = "federated-cp"
	TopologyStringFederatedDomain = "federated-domain"
)

// ResolveTopologyDeps holds the injectable dependencies required by
// ResolveTopology. Callers fill these in from production defaults;
// tests substitute fakes so no network or real gh CLI is touched.
type ResolveTopologyDeps struct {
	// Owner is the GitHub owner login for the current repo.
	Owner string
	// Repo is the repository name (no owner prefix).
	Repo string
	// ProjectID is the GitHub ProjectV2 node ID linked to this repo
	// (sourced from the AGENTIC_PROJECT_ID variable). Empty when the
	// repo has no project affiliation — see the fallback rule below.
	ProjectID string
	// GetRepoVariable reads a repo-level Actions variable. Used to
	// inspect AGENTIC_TOPOLOGY and AGENTIC_FRAMEWORK_VERSION on the
	// current repo.
	GetRepoVariable GetRepoVariableFunc
	// FetchLinkedRepos returns the repositories linked to a ProjectV2
	// by its node ID. Called at most once per ResolveTopology call.
	FetchLinkedRepos FetchLinkedReposFunc
}

// ResolveTopology is the single source of truth for determining pipeline
// topology for the current repository. Both `gh agentic check` and
// `gh agentic repair` invoke it — no other code path should decide
// topology locally.
//
// Precedence:
//
//  1. If AGENTIC_TOPOLOGY is set on the local repo, honour it:
//     - "federated" → delegate to resolveTopologyMode (inspects
//     AGENTIC_FRAMEWORK_VERSION to decide federated-cp vs
//     federated-domain). The control plane is the only repo that sets
//     AGENTIC_FRAMEWORK_VERSION on itself.
//     - "single"    → return "single".
//
//  2. Otherwise, when ProjectID is known, inspect the project's linked
//     repos:
//     - 0 or 1 linked repos AND no local AGENTIC_FRAMEWORK_VERSION →
//     "single" (the repo stands alone).
//     - More than 1 linked repo → federated. Delegate to
//     resolveTopologyMode.
//
//  3. No project affiliation (empty ProjectID) → "single". This
//     preserves existing behaviour for un-affiliated repositories.
//
// The prior `federated-domain → single` downgrade is intentionally gone.
// A federated domain repo that happens to lack AGENTIC_TOPOLOGY locally
// (because only the control plane broadcasts shared values) must now be
// detected correctly as "federated-domain".
//
// FetchLinkedRepos is called at most once per invocation — its result
// (or error) is captured once and reused.
//
// Errors are propagated only when FetchLinkedRepos itself fails.
// Missing/unset variables are treated as "not set" rather than errors,
// because the gh CLI returns a non-zero exit code for a missing
// variable and that is the common case, not an exceptional one.
func ResolveTopology(deps ResolveTopologyDeps) (string, error) {
	if deps.GetRepoVariable == nil {
		// Without the ability to read variables, we cannot inspect the
		// local signals at all — fall back to the safe default.
		return TopologyStringSingle, nil
	}

	topoVal, _ := deps.GetRepoVariable(deps.Owner, deps.Repo, TopologyVarName)
	switch strings.TrimSpace(topoVal) {
	case "federated":
		return resolveTopologyMode(deps), nil
	case "single":
		return TopologyStringSingle, nil
	}

	// AGENTIC_TOPOLOGY is unset — fall back to the project-linked-repos
	// signal, which is how a federated domain repo is correctly
	// identified even when it never sets the variable locally.
	if deps.ProjectID == "" {
		return TopologyStringSingle, nil
	}
	if deps.FetchLinkedRepos == nil {
		return TopologyStringSingle, nil
	}

	linked, err := deps.FetchLinkedRepos(deps.ProjectID)
	if err != nil {
		return "", err
	}

	versionVal, _ := deps.GetRepoVariable(deps.Owner, deps.Repo, FrameworkVersionVarName)
	hasVersion := strings.TrimSpace(versionVal) != ""

	if len(linked) <= 1 && !hasVersion {
		return TopologyStringSingle, nil
	}
	return resolveTopologyMode(deps), nil
}

// resolveTopologyMode decides between "federated-cp" and
// "federated-domain" by inspecting AGENTIC_FRAMEWORK_VERSION on the
// current repo. Only the control plane sets that variable on itself,
// so its presence is a reliable CP marker.
//
// This helper is internal to the resolver — callers must go through
// ResolveTopology so that all the precedence rules (unset variable,
// no project affiliation, linked-repo inspection) are applied.
func resolveTopologyMode(deps ResolveTopologyDeps) string {
	if deps.GetRepoVariable == nil {
		return TopologyStringFederatedDomain
	}
	out, err := deps.GetRepoVariable(deps.Owner, deps.Repo, FrameworkVersionVarName)
	if err == nil && strings.TrimSpace(out) != "" {
		return TopologyStringFederatedCP
	}
	return TopologyStringFederatedDomain
}
