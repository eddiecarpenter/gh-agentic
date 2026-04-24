package githubapp

import (
	"fmt"
	"net/url"
)

// TargetType describes the kind of install target an InstallURL is built
// for. The distinction matters because repo-scoped installs surface a
// different picker in the GitHub UI than org-scoped ones — even though the
// canonical entry URL is the same.
type TargetType int

const (
	// TargetRepo identifies a repo-scoped install. The target name is the
	// `owner/repo` slug; GitHub's UI prompts the user to pick an owner
	// (user or org) and which of their repos to grant access to.
	TargetRepo TargetType = iota

	// TargetOrg identifies an org-scoped install. The target name is the
	// organisation login; GitHub's UI prompts the user to pick the org
	// (if they are a member of several) and confirm all-repos vs selected.
	TargetOrg
)

// String returns a lowercase human-readable form of the TargetType — used
// in error messages and log lines. Keep it stable; callers may compare
// against it in tests.
func (t TargetType) String() string {
	switch t {
	case TargetRepo:
		return "repo"
	case TargetOrg:
		return "org"
	default:
		return "unknown"
	}
}

// InstallURL returns the URL the user should visit to install the given
// GitHub App on the target. The returned URL is always the canonical
// GitHub install page — `https://github.com/apps/<slug>/installations/new`
// — optionally annotated with a `state` parameter that carries the target
// name for diagnostics; GitHub's install flow shows a target picker on
// that page so full URL pre-selection is not supported for every path.
//
// The slug is URL-path-escaped so a malformed value cannot inject into
// the URL structure. An empty slug returns an empty string — callers
// should treat "" as an unrecoverable programming error.
func InstallURL(slug string, targetType TargetType, targetName string) string {
	if slug == "" {
		return ""
	}
	base := fmt.Sprintf("https://github.com/apps/%s/installations/new", url.PathEscape(slug))

	// The target name is informational — GitHub presents a picker on the
	// install page and makes the final choice. We encode it as a `state`
	// query parameter so diagnostic tooling (and the `state` callback
	// after install) can correlate the flow with the target. Dropping
	// the parameter entirely when targetName is empty keeps the URL
	// identical to what a user would paste by hand, which matters for
	// tests and for operators who compare URLs.
	if targetName == "" {
		return base
	}

	q := url.Values{}
	q.Set("state", fmt.Sprintf("%s:%s", targetType, targetName))
	return base + "?" + q.Encode()
}
