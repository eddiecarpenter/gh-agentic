package project

import (
	"fmt"
	"strings"
)

// Pure topology-detection helpers. Extracted from api.go so they stay
// covered by the project package's test suite — the rest of api.go is
// `gh api graphql ...` shell-outs that are excluded from coverage as
// network-bound (see sonar-project.properties).
//
// These functions take plain Go values, return plain Go values, and
// have no side-effects. Their tests live in api_test.go (kept under
// that name because the test file existed before this split).

// DetectTopology compares the current repo against the project's linked repos.
// If the current repo is among the linked repos → Single (case-insensitive
// match on the owner/repo string). If the linked repo is a different repo →
// Federated. If there are no linked repos → Unknown.
func DetectTopology(currentOwnerRepo string, linked []LinkedRepo) Topology {
	if len(linked) == 0 {
		return TopologyUnknown
	}
	for _, r := range linked {
		if strings.EqualFold(r.NameWithOwner, currentOwnerRepo) {
			return TopologySingle
		}
	}
	return TopologyFederated
}

// ControlPlaneRepo returns the control plane repo from the linked repos.
// For Single topology this is the current repo; for Federated it is the
// linked repo. Returns false when there are no linked repos.
//
// NOTE: this uses a positional heuristic (first linked repo). For federation
// the authoritative rule is "the repo bearing FEDERATION.md" — see
// ResolveControlPlane, which identifies the control plane by the manifest.
func ControlPlaneRepo(linked []LinkedRepo) (LinkedRepo, bool) {
	if len(linked) == 0 {
		return LinkedRepo{}, false
	}
	return linked[0], true
}

// ResolveControlPlane identifies the control-plane repo among a project's
// linked repos as the one whose root contains FEDERATION.md. It checks each
// linked repo via hasFederationFile and returns the first match.
//
// When no linked repo carries the manifest — single topology, or a project
// with no federation requirements repo — it returns ok=false with a nil
// error, so callers fall back to single-repo behaviour. A check failure for
// any repo is surfaced as an error rather than silently skipped.
//
// Unlike ControlPlaneRepo, this does not assume the control plane is the first
// linked repo; it identifies it by the manifest, per the federation model
// (knowledge-plane.md: "the repo whose root contains FEDERATION.md").
func ResolveControlPlane(linked []LinkedRepo, hasFederationFile RepoHasFederationFileFunc) (LinkedRepo, bool, error) {
	for _, r := range linked {
		owner, repo, ok := splitOwnerRepo(r.NameWithOwner)
		if !ok {
			continue
		}
		has, err := hasFederationFile(owner, repo)
		if err != nil {
			return LinkedRepo{}, false, fmt.Errorf("resolving control plane: %w", err)
		}
		if has {
			return r, true, nil
		}
	}
	return LinkedRepo{}, false, nil
}

// splitOwnerRepo splits an "owner/repo" string into its parts. ok is false
// when the input is not in owner/repo form (empty or missing a segment).
func splitOwnerRepo(nameWithOwner string) (owner, repo string, ok bool) {
	parts := strings.SplitN(nameWithOwner, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}
