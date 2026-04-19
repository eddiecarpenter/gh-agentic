package project

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

// ResolveTopology is retained as a thin wrapper over Resolve for call sites
// that only need the topology string and do not yet carry a full project.Deps
// bundle. New code should call Resolve directly.
//
// The canonical precedence — AGENTIC_TOPOLOGY override, then project-linked-
// repos inspection, then the no-affiliation default — is defined in Resolve;
// this wrapper forwards to the same internal implementation used there so
// the two entry points never drift.
//
// Preserves the historic behaviour of this function:
//   - FetchLinkedRepos is called at most once per invocation.
//   - Missing/unset variables are treated as "not set" rather than errors.
//   - Only a genuine FetchLinkedRepos failure propagates as an error.
func ResolveTopology(deps ResolveTopologyDeps) (string, error) {
	// Adapt the thin-slice ResolveTopologyDeps to the full project.Deps
	// shape used by the shared helpers. Only the fields the topology
	// decision reads are required; everything else is left at its zero
	// value.
	fullDeps := Deps{
		Owner:            deps.Owner,
		RepoName:         deps.Repo,
		RepoFullName:     deps.Owner + "/" + deps.Repo,
		GetRepoVariable:  deps.GetRepoVariable,
		FetchLinkedRepos: deps.FetchLinkedRepos,
	}

	// The historic wrapper contract: FetchLinkedRepos must not be called
	// on the variable-set path. Short-circuit here before any network
	// call happens so that behaviour is preserved.
	if fullDeps.GetRepoVariable == nil {
		return TopologyStringSingle, nil
	}
	topoVal, _ := fullDeps.GetRepoVariable(deps.Owner, deps.Repo, TopologyVarName)
	switch trim(topoVal) {
	case "federated":
		return resolveFederatedMode(fullDeps), nil
	case "single":
		return TopologyStringSingle, nil
	}
	if deps.ProjectID == "" || deps.FetchLinkedRepos == nil {
		return TopologyStringSingle, nil
	}
	linked, err := deps.FetchLinkedRepos(deps.ProjectID)
	if err != nil {
		return "", err
	}
	return resolveTopologyWithLinked(fullDeps, deps.ProjectID, linked, nil)
}

