// Package scope is the single source of truth for routing a variable or
// secret write to the correct `gh` CLI scope (--org or --repo) based on
// topology.
//
// It lives in its own package — separate from internal/project — so that
// lower-level callers (internal/auth, internal/init) can import it
// without pulling in the broader project package, which would create an
// import cycle. The internal/project package re-exports these symbols so
// callers that already depend on project may continue to use the
// project.ScopeFor form.
//
// Under federated topology, shared variables and secrets live at the
// organisation level — the moment we write a shared value with `--repo` we
// silently shadow the org inheritance. Per-repo identity values always
// live on the individual repository.
package scope

// Scope flag constants — the string the gh CLI expects on its --org /
// --repo flags. Kept as constants so every site uses the exact same
// literal.
const (
	ScopeFlagOrg  = "--org"
	ScopeFlagRepo = "--repo"
)

// sharedNames enumerates variable/secret names that, under federated
// topology, belong at the organisation level rather than on the individual
// control-plane or domain repo. Under single topology they continue to
// live on the repo (there is no org above a single-repo deployment to pin
// them to).
var sharedNames = map[string]struct{}{
	"AGENT_USER":              {},
	"RUNNER_LABEL":            {},
	"AGENT_PROVIDER":          {},
	"AGENT_MODEL":             {},
	"AGENTIC_APP_CLIENT_ID":   {},
	"AGENTIC_APP_PRIVATE_KEY": {},
	"PROJECT_PAT":             {},
	"CLAUDE_CREDENTIALS_JSON": {},
}

// identityNames enumerates variable names that describe the repo itself —
// its project affiliation, its topology role, and the framework version it
// should mount. These are always repo-scoped regardless of topology.
var identityNames = map[string]struct{}{
	"AGENTIC_PROJECT_ID":        {},
	"AGENTIC_TOPOLOGY":          {},
	"AGENTIC_FRAMEWORK_VERSION": {},
}

// IsSharedName reports whether the given variable/secret name is a shared
// value that should live at the organisation level under federated
// topology.
func IsSharedName(name string) bool {
	_, ok := sharedNames[name]
	return ok
}

// IsIdentityName reports whether the given variable name is a per-repo
// identity value that must always be written at the repository level.
func IsIdentityName(name string) bool {
	_, ok := identityNames[name]
	return ok
}

// IsFederatedTopology reports whether the given topology string represents
// any federated variant — the bare "federated" marker written by project
// create, or the explicit "federated-cp"/"federated-domain" roles emitted
// by the repair / doctor pipeline. The match is case-sensitive so that
// unknown or mis-cased inputs default to not-federated (repo scope).
func IsFederatedTopology(topology string) bool {
	switch topology {
	case "federated", "federated-cp", "federated-domain":
		return true
	default:
		return false
	}
}

// ScopeFor returns the gh CLI scope flag ("--org" or "--repo") and the
// matching target (org login or owner/repo) for a given variable or secret
// name under a given topology.
//
// Rules:
//   - Shared names (AGENT_USER, RUNNER_LABEL, AGENT_PROVIDER, AGENT_MODEL,
//     AGENTIC_APP_CLIENT_ID, AGENTIC_APP_PRIVATE_KEY, PROJECT_PAT,
//     CLAUDE_CREDENTIALS_JSON) route to --org under any
//     federated topology variant and to --repo under single.
//   - Per-repo identity names (AGENTIC_PROJECT_ID, AGENTIC_TOPOLOGY,
//     AGENTIC_FRAMEWORK_VERSION) always route to --repo.
//   - Any other (unknown) name defaults to --repo to preserve current
//     behaviour — the router must never silently decide to push an unknown
//     value to the organisation level.
//   - Unknown topology values are treated as "not federated" — everything
//     stays at --repo so we do not widen scope by accident.
//
// Parameters:
//   - name         — the variable or secret name.
//   - topology     — one of "single", "federated", "federated-cp",
//     "federated-domain", or an empty/unknown value.
//   - owner        — the GitHub owner login (used as the --org target).
//   - repoFullName — the "owner/repo" slug (used as the --repo target).
func ScopeFor(name, topology, owner, repoFullName string) (flag, target string) {
	if IsSharedName(name) && IsFederatedTopology(topology) {
		return ScopeFlagOrg, owner
	}
	return ScopeFlagRepo, repoFullName
}
