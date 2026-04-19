package project

import (
	"fmt"
	"strings"
)

// Role describes the repository's role in the agentic project topology. It is
// a derived property of Context.Topology and surfaces the intent callers care
// about ("am I the CP? a domain? standalone?") without forcing them to parse
// the canonical topology string.
const (
	// RoleStandalone — the repo is not part of a federated setup. Either
	// single topology (the CP and the code live in one repo) or the repo
	// has no project affiliation at all.
	RoleStandalone = "standalone"
	// RoleCP — the repo is the control plane of a federated setup.
	RoleCP = "cp"
	// RoleDomain — the repo is a domain repo in a federated setup. The
	// control plane lives elsewhere.
	RoleDomain = "domain"
)

// Context is the unified resolved project context returned by Resolve. It is
// the single source of truth for every piece of per-repo project information
// any gh agentic command needs. No command should read AGENTIC_* variables
// directly — everything flows through here.
//
// Consumers treat the struct as read-only; it is populated once by Resolve
// and passed by value (or pointer) into command handlers.
type Context struct {
	// --- Repository identity ---

	Owner        string
	RepoName     string
	RepoFullName string // "owner/repo"
	Root         string // local working directory

	// --- Project affiliation ---

	// ProjectID is the AGENTIC_PROJECT_ID repo variable. Empty when the
	// repo is not affiliated with a project.
	ProjectID string
	// ProjectName is the human-readable title of the project. Falls back
	// to ProjectID when the project title cannot be fetched and is not
	// confirmed deleted.
	ProjectName string
	// ProjectDeleted is true when ProjectID is set but the project node
	// no longer resolves — the affiliation is stale.
	ProjectDeleted bool

	// --- Topology ---

	// Topology is the canonical topology string: "single",
	// "federated-cp", or "federated-domain". Never empty when Resolve
	// returns a non-nil Context.
	Topology string
	// Role is the derived per-repo role: RoleStandalone, RoleCP, or
	// RoleDomain. Use this instead of string-matching Topology.
	Role string
	// ControlPlane is the linked repo that hosts the control plane. For
	// RoleCP / RoleStandalone this is the current repo; for RoleDomain it
	// is the other repo; for un-affiliated repos it is the zero value.
	ControlPlane LinkedRepo
	// LinkedRepos lists every repo linked to this project. May be empty
	// when the project is deleted or the fetch failed.
	LinkedRepos []LinkedRepo

	// --- Framework version ---

	// FrameworkVersion is the authoritative framework version resolved by
	// topology:
	//   - single / federated-cp  → AGENTIC_FRAMEWORK_VERSION on this repo
	//   - federated-domain       → AGENTIC_FRAMEWORK_VERSION on the CP
	// Empty when none is set (e.g. a single repo that has never published
	// a version, or a federated setup where the CP has not yet rolled
	// one out).
	FrameworkVersion string
	// LocalAIVersion is the framework version the local .ai/ mount
	// currently reports — read via Deps.ReadAIVersion. This is kept
	// separately from FrameworkVersion so callers can detect drift; it
	// disappears once #585 removes the file.
	LocalAIVersion string
	// VersionInSync is true when LocalAIVersion matches FrameworkVersion,
	// or when no FrameworkVersion is published (nothing to sync to).
	VersionInSync bool
}

// IsFederatedControlPlane returns true when this repo is the CP of a
// federated setup. Use this in preference to direct string comparison.
func (c *Context) IsFederatedControlPlane() bool {
	return c != nil && c.Topology == TopologyStringFederatedCP
}

// IsFederatedDomain returns true when this repo is a domain repo in a
// federated setup (the CP lives elsewhere).
func (c *Context) IsFederatedDomain() bool {
	return c != nil && c.Topology == TopologyStringFederatedDomain
}

// IsSingle returns true when this repo is a single-topology standalone
// control plane (CP and code in one repo).
func (c *Context) IsSingle() bool {
	return c != nil && c.Topology == TopologyStringSingle
}

// Resolve is the single canonical entry point for resolving full project
// context for a repository. Every gh agentic subcommand that needs project
// metadata calls this — no command may read AGENTIC_* variables directly.
//
// Resolve consolidates the earlier ResolveTopology (topology mode string)
// and ResolveState (ProjectState struct) into one call. Both wrappers remain
// callable during the incremental migration and delegate here internally.
//
// Error behaviour:
//   - Missing AGENTIC_PROJECT_ID is not an error — Resolve returns a Context
//     with Topology="single", Role=RoleStandalone, and the remaining
//     project fields zero. Callers that require affiliation check
//     ctx.ProjectID explicitly and surface their own ErrProjectNotConfigured
//     message (or equivalent).
//   - A GraphQL failure that denies the linked-repos fetch propagates only
//     when the topology decision genuinely depends on it. When the
//     AGENTIC_TOPOLOGY variable is authoritative, the fetch is never
//     attempted.
//   - A project whose title cannot be fetched is flagged ProjectDeleted=true
//     rather than returning an error — UX prefers "stale affiliation" over
//     "hard failure" so the user can recover via `project unlink`.
func Resolve(deps Deps) (*Context, error) {
	ctx := &Context{
		Owner:        deps.Owner,
		RepoName:     deps.RepoName,
		RepoFullName: deps.RepoFullName,
		Root:         deps.Root,
	}

	// Read AGENTIC_PROJECT_ID. Empty means unaffiliated — a safe default,
	// not an error (see doc comment).
	if deps.GetRepoVariable != nil {
		pid, _ := deps.GetRepoVariable(deps.Owner, deps.RepoName, ProjectVarName)
		ctx.ProjectID = strings.TrimSpace(pid)
	}

	// Resolve topology — the same canonical precedence used by
	// ResolveTopology: AGENTIC_TOPOLOGY override first, then the linked-
	// repos fallback. Pull the linked-repos list out so the rest of the
	// function can populate ControlPlane / LinkedRepos without a second
	// network round trip.
	var linked []LinkedRepo
	var linkedErr error
	if ctx.ProjectID != "" && deps.FetchLinkedRepos != nil {
		linked, linkedErr = deps.FetchLinkedRepos(ctx.ProjectID)
	}

	topology, topoErr := resolveTopologyWithLinked(deps, ctx.ProjectID, linked, linkedErr)
	if topoErr != nil {
		return nil, topoErr
	}
	ctx.Topology = topology
	ctx.Role = roleForTopology(topology)

	// Project title — fetched only when affiliated. A deleted/unreachable
	// project is flagged rather than raised as an error.
	if ctx.ProjectID != "" && deps.FetchProjectTitle != nil {
		title, err := deps.FetchProjectTitle(ctx.ProjectID)
		if err != nil || strings.TrimSpace(title) == "" {
			ctx.ProjectDeleted = true
			ctx.ProjectName = ctx.ProjectID
		} else {
			ctx.ProjectName = title
		}
	} else if ctx.ProjectID != "" {
		// Affiliated but no title-fetcher wired — fall back to the ID.
		ctx.ProjectName = ctx.ProjectID
	}

	// Project graph — linked repos and control plane. If linkedErr is
	// non-nil the topology resolver has already folded it into the
	// decision; expose the empty graph here rather than a spurious error.
	if linkedErr == nil {
		ctx.LinkedRepos = linked
		cp, ok := ControlPlaneRepo(linked)
		if ok {
			switch topology {
			case TopologyStringSingle, TopologyStringFederatedCP:
				// CP is the current repo.
				ctx.ControlPlane = LinkedRepo{
					Name:          deps.RepoName,
					NameWithOwner: deps.RepoFullName,
				}
			case TopologyStringFederatedDomain:
				// CP is the other linked repo. DetectTopology picks the
				// first linked repo as CP for the Federated enum; do the
				// same but never pick ourselves.
				ctx.ControlPlane = pickControlPlane(deps.RepoFullName, linked, cp)
			}
		}
	}

	// Framework versions — authoritative vs local-on-disk.
	ctx.FrameworkVersion = readAuthoritativeVersion(deps, ctx)
	if deps.ReadAIVersion != nil {
		ctx.LocalAIVersion, _ = deps.ReadAIVersion(deps.Root)
	}
	ctx.VersionInSync = ctx.LocalAIVersion == ctx.FrameworkVersion || ctx.FrameworkVersion == ""

	return ctx, nil
}

// resolveTopologyWithLinked reuses the canonical ResolveTopology precedence
// but accepts a pre-fetched linked-repos list so Resolve avoids a duplicate
// network round trip. linkedErr != nil is propagated only when the topology
// decision actually needs the list.
func resolveTopologyWithLinked(deps Deps, projectID string, linked []LinkedRepo, linkedErr error) (string, error) {
	if deps.GetRepoVariable == nil {
		return TopologyStringSingle, nil
	}

	// Precedence 1: explicit AGENTIC_TOPOLOGY wins.
	topoVal, _ := deps.GetRepoVariable(deps.Owner, deps.RepoName, TopologyVarName)
	switch strings.TrimSpace(topoVal) {
	case "federated":
		// federated-cp vs federated-domain — the presence of
		// AGENTIC_FRAMEWORK_VERSION locally identifies the CP.
		return resolveFederatedMode(deps), nil
	case "single":
		return TopologyStringSingle, nil
	}

	// Precedence 2: no project affiliation → single.
	if projectID == "" {
		return TopologyStringSingle, nil
	}

	// Precedence 3: inspect linked repos. Any error from the linked-repos
	// fetch is surfaced here because without the list the decision cannot
	// be made reliably.
	if linkedErr != nil {
		return "", linkedErr
	}

	versionVal, _ := deps.GetRepoVariable(deps.Owner, deps.RepoName, FrameworkVersionVarName)
	hasVersion := strings.TrimSpace(versionVal) != ""

	if len(linked) <= 1 && !hasVersion {
		return TopologyStringSingle, nil
	}
	return resolveFederatedMode(deps), nil
}

// resolveFederatedMode picks between federated-cp and federated-domain by
// inspecting AGENTIC_FRAMEWORK_VERSION on the current repo. Only the CP
// broadcasts that value on itself, so its presence is a reliable marker.
func resolveFederatedMode(deps Deps) string {
	if deps.GetRepoVariable == nil {
		return TopologyStringFederatedDomain
	}
	out, err := deps.GetRepoVariable(deps.Owner, deps.RepoName, FrameworkVersionVarName)
	if err == nil && strings.TrimSpace(out) != "" {
		return TopologyStringFederatedCP
	}
	return TopologyStringFederatedDomain
}

// roleForTopology maps a canonical topology string to the derived Role.
func roleForTopology(topology string) string {
	switch topology {
	case TopologyStringFederatedCP:
		return RoleCP
	case TopologyStringFederatedDomain:
		return RoleDomain
	default:
		return RoleStandalone
	}
}

// pickControlPlane selects the control-plane LinkedRepo for a federated
// domain repo. The caller has already confirmed ControlPlaneRepo reports a
// non-empty result, so at least one linked repo exists.
//
// The rule: the CP is the first linked repo whose NameWithOwner is not the
// current repo. If every linked repo is the current repo (defensive — the
// federated-domain path implies at least one other), fall back to the
// result ControlPlaneRepo returned.
func pickControlPlane(currentRepoFullName string, linked []LinkedRepo, fallback LinkedRepo) LinkedRepo {
	for _, r := range linked {
		if !strings.EqualFold(r.NameWithOwner, currentRepoFullName) {
			return r
		}
	}
	return fallback
}

// readAuthoritativeVersion returns the framework version that callers should
// trust as the source of truth, resolved per-topology:
//
//   - single / federated-cp → AGENTIC_FRAMEWORK_VERSION on the current repo
//   - federated-domain      → AGENTIC_FRAMEWORK_VERSION on the CP repo
//
// The function returns "" when no version is published. Errors from the
// underlying variable-read are treated the same as an absent value — the
// caller distinguishes "not set" from "lookup failed" only when it matters
// (the existing commands do not).
func readAuthoritativeVersion(deps Deps, ctx *Context) string {
	if deps.GetRepoVariable == nil {
		return ""
	}

	switch ctx.Topology {
	case TopologyStringSingle, TopologyStringFederatedCP:
		out, err := deps.GetRepoVariable(deps.Owner, deps.RepoName, FrameworkVersionVarName)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(out)

	case TopologyStringFederatedDomain:
		if ctx.ControlPlane.NameWithOwner == "" {
			return ""
		}
		parts := strings.SplitN(ctx.ControlPlane.NameWithOwner, "/", 2)
		if len(parts) != 2 {
			return ""
		}
		out, err := deps.GetRepoVariable(parts[0], parts[1], FrameworkVersionVarName)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(out)
	}
	return ""
}

// ensureContextValid is an internal sanity check used by the wrappers that
// surface a Context-backed legacy type; callers should not call Resolve and
// then ignore a non-nil error.
func ensureContextValid(ctx *Context, err error) (*Context, error) {
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		return nil, fmt.Errorf("project.Resolve returned nil context without an error")
	}
	return ctx, nil
}

// trim is a tiny stand-in for strings.TrimSpace kept as a file-local helper
// so the topology wrapper (which must not import strings directly — it is
// deliberately lean) can share the same whitespace-normalisation rule Resolve
// uses on variable values.
func trim(s string) string { return strings.TrimSpace(s) }
