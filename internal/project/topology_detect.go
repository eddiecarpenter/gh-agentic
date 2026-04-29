package project

import "strings"

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
func ControlPlaneRepo(linked []LinkedRepo) (LinkedRepo, bool) {
	if len(linked) == 0 {
		return LinkedRepo{}, false
	}
	return linked[0], true
}
