package project

import (
	"fmt"
	"strings"
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
	// ProjectIDReadFailed is true when the AGENTIC_PROJECT_ID variable could
	// not be read due to a token permission error (HTTP 403). The variable may
	// be set; the token simply lacked Variables:Read (Actions:Read) permission.
	// Distinguishes "not configured" from "couldn't check" so the doctor can
	// emit Warning rather than Fail in CI contexts where PIPELINE_PAT has
	// limited scopes.
	ProjectIDReadFailed bool

	// --- Topology ---

	// Topology is the canonical topology string: "single" or "federation".
	// "federation" when FEDERATION.md is present at deps.Root; "single"
	// otherwise. Never empty when Resolve returns a non-nil Context.
	Topology string

	// --- Framework version ---

	// FrameworkVersion is the AGENTIC_FRAMEWORK_VERSION on this repo.
	// Empty when none is set.
	FrameworkVersion string
	// LocalAIVersion is the framework version the local .agents/ mount
	// currently reports — read via Deps.ReadAIVersion.
	LocalAIVersion string
	// VersionInSync is true when LocalAIVersion matches FrameworkVersion,
	// or when no FrameworkVersion is published (nothing to sync to).
	VersionInSync bool
}

// Resolve is the single canonical entry point for resolving full project
// context for a repository. Every gh agentic subcommand that needs project
// metadata calls this — no command may read AGENTIC_* variables directly.
//
// Topology is now determined by FEDERATION.md presence at deps.Root:
//   - FEDERATION.md present → "federation"
//   - FEDERATION.md absent  → "single"
//
// Error behaviour:
//   - Missing AGENTIC_PROJECT_ID is not an error — Resolve returns a Context
//     with Topology derived from FEDERATION.md and the remaining project
//     fields zero. Callers that require affiliation check ctx.ProjectID
//     explicitly and surface their own ErrProjectNotConfigured message.
func Resolve(deps Deps) (*Context, error) {
	ctx := &Context{
		Owner:        deps.Owner,
		RepoName:     deps.RepoName,
		RepoFullName: deps.RepoFullName,
		Root:         deps.Root,
	}

	// Read AGENTIC_PROJECT_ID. Empty means unaffiliated — a safe default,
	// not an error. A permission error (HTTP 403) means the token lacks
	// Variables:Read scope; record that separately so the doctor can emit
	// Warning rather than Fail.
	if deps.GetRepoVariable != nil {
		pid, err := deps.GetRepoVariable(deps.Owner, deps.RepoName, ProjectVarName)
		if err != nil {
			if isVariablePermissionError(err) {
				ctx.ProjectIDReadFailed = true
			}
			// else: genuine absence or transient error — leave ProjectID empty
		} else {
			ctx.ProjectID = strings.TrimSpace(pid)
		}
	}

	// Topology: FEDERATION.md presence at deps.Root is the sole signal.
	if IsFederationRepo(deps.Root) {
		ctx.Topology = TopologyStringFederation
	} else {
		ctx.Topology = TopologyStringSingle
	}

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

	// Framework versions — authoritative (AGENTIC_FRAMEWORK_VERSION on this
	// repo) vs local-on-disk.
	ctx.FrameworkVersion = readAuthoritativeVersion(deps)
	if deps.ReadAIVersion != nil {
		ctx.LocalAIVersion, _ = deps.ReadAIVersion(deps.Root)
	}
	ctx.VersionInSync = ctx.LocalAIVersion == ctx.FrameworkVersion || ctx.FrameworkVersion == ""

	return ctx, nil
}

// readAuthoritativeVersion returns the AGENTIC_FRAMEWORK_VERSION on the
// current repo. Returns "" when no version is published. Errors are treated
// the same as an absent value.
func readAuthoritativeVersion(deps Deps) string {
	if deps.GetRepoVariable == nil {
		return ""
	}
	out, err := deps.GetRepoVariable(deps.Owner, deps.RepoName, FrameworkVersionVarName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// ensureContextValid is an internal sanity check used by wrappers that
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

// isVariablePermissionError returns true when a GetRepoVariable error indicates
// a token-permission failure (HTTP 403) rather than a genuinely absent variable
// (HTTP 404) or a transient error. Fine-grained PATs that lack Actions:Read
// (Variables:Read) and GitHub App installation tokens without that scope both
// produce 403 errors; the doctor must not report these as "variable not
// configured" — they should be surfaced as "unable to verify".
func isVariablePermissionError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "403") ||
		strings.Contains(s, "resource not accessible") ||
		strings.Contains(s, "insufficient scopes") ||
		strings.Contains(s, "must have admin rights") ||
		strings.Contains(s, "forbidden")
}
