package projectstatus

import (
	"fmt"
	"regexp"
	"strconv"
)

// blockedByConventionPattern matches the structured convention used as a
// fallback when native GitHub issue dependencies are not populated:
//
//	Blocked-by: owner/repo#N
//	Blocked-by: #N
//
// The pattern is deliberately narrow — it must anchor to the start of a
// line and require the exact `Blocked-by:` prefix so free-form prose in
// issue bodies is never mistaken for a dependency declaration.
var blockedByConventionPattern = regexp.MustCompile(`(?mi)^Blocked-by:\s+(?:([\w.-]+/[\w.-]+))?#(\d+)\s*$`)

// FetchBlockerFunc returns the native blocker (via GitHub's trackedIssues /
// issue-dependency GraphQL relationship) for the given issue. A nil result
// with a nil error means the issue has no native blocker — the caller then
// falls back to the structured convention in the issue body.
type FetchBlockerFunc func(owner, repo string, number int) (*BlockedInfo, error)

// FetchBlocker resolves the blocker of an issue using the project's chosen
// dependency mechanism. The resolution order is:
//
//  1. Native GitHub dependency (`trackedIssues` field) — preferred when the
//     repository has issue-dependency tracking enabled.
//  2. Structured convention line `Blocked-by: owner/repo#N` in the issue
//     body — fallback when native dependencies are absent.
//  3. Neither — return (nil, nil). Items without a reported blocker render
//     with no annotation; the UI never guesses.
//
// Free-form prose parsing is explicitly *not* performed — only the
// regex-anchored convention above is accepted.
//
// body is passed in so callers that already have it (e.g. composers reading
// from project items) avoid a second round trip; an empty body skips the
// convention step.
func FetchBlocker(deps Deps, owner, repo string, number int, body string) (*BlockedInfo, error) {
	// Step 1 — native mechanism.
	if deps.FetchBlocker != nil {
		info, err := deps.FetchBlocker(owner, repo, number)
		if err != nil {
			return nil, fmt.Errorf("querying native blocker for #%d: %w", number, err)
		}
		if info != nil {
			return info, nil
		}
	}

	// Step 2 — structured convention.
	if info := parseBlockedByConvention(body, owner, repo); info != nil {
		return info, nil
	}

	// Step 3 — absent.
	return nil, nil
}

// parseBlockedByConvention returns the first `Blocked-by:` line in body, or
// nil when the body carries no matching marker. The BlockingRef in the
// returned BlockedInfo is always normalised to "owner/repo#N".
func parseBlockedByConvention(body, defaultOwner, defaultRepo string) *BlockedInfo {
	m := blockedByConventionPattern.FindStringSubmatch(body)
	if len(m) == 0 {
		return nil
	}
	n, err := strconv.Atoi(m[2])
	if err != nil || n <= 0 {
		return nil
	}
	ref := ""
	if m[1] != "" {
		ref = fmt.Sprintf("%s#%d", m[1], n)
	} else {
		if defaultOwner == "" || defaultRepo == "" {
			ref = fmt.Sprintf("#%d", n)
		} else {
			ref = fmt.Sprintf("%s/%s#%d", defaultOwner, defaultRepo, n)
		}
	}
	return &BlockedInfo{BlockingRef: ref}
}
