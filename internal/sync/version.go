package sync

import (
	"fmt"
	"strings"

	"golang.org/x/mod/semver"
)

// Release represents a single GitHub release with its tag, name, body, and tarball URL.
type Release struct {
	TagName    string `json:"tag_name"`
	Name       string `json:"name"`
	Body       string `json:"body"`
	TarballURL string `json:"tarball_url"`
}

// FetchReleaseFunc fetches the latest release tag for a given repo slug.
// Injected so tests can substitute a fake implementation without real GitHub API calls.
type FetchReleaseFunc func(repo string) (string, error)

// FetchReleasesFunc fetches all releases for a given repo slug.
// Injected so tests can substitute a fake implementation without real GitHub API calls.
type FetchReleasesFunc func(repo string) ([]Release, error)

// FilterReleasesSince filters a slice of releases to only those with a TagName
// newer than the given version (using semver comparison). Returns newest-first order.
// Tags that are not valid semver are skipped.
func FilterReleasesSince(releases []Release, since string) []Release {
	sinceCanonical := canonicalSemver(since)
	if !semver.IsValid(sinceCanonical) {
		return nil
	}

	var filtered []Release
	for _, r := range releases {
		tagCanonical := canonicalSemver(r.TagName)
		if !semver.IsValid(tagCanonical) {
			continue
		}
		if semver.Compare(tagCanonical, sinceCanonical) > 0 {
			filtered = append(filtered, r)
		}
	}

	// Sort newest-first using semver comparison.
	semverSort(filtered)
	return filtered
}

// FindReleaseByTag searches a slice of releases for one matching the given tag.
// Returns the release and true if found, or a zero-value Release and false if not.
func FindReleaseByTag(releases []Release, tag string) (Release, bool) {
	for _, r := range releases {
		if r.TagName == tag {
			return r, true
		}
	}
	return Release{}, false
}

// canonicalSemver ensures a version string has a "v" prefix for semver comparison.
func canonicalSemver(v string) string {
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	return v
}

// semverSort sorts releases newest-first by semver tag.
func semverSort(releases []Release) {
	for i := 1; i < len(releases); i++ {
		for j := i; j > 0; j-- {
			a := canonicalSemver(releases[j].TagName)
			b := canonicalSemver(releases[j-1].TagName)
			if semver.Compare(a, b) > 0 {
				releases[j], releases[j-1] = releases[j-1], releases[j]
			} else {
				break
			}
		}
	}
}

// FetchLatestRelease retrieves the latest release tag from the upstream template repo
// using the provided fetch function.
func FetchLatestRelease(repo string, fetchFunc FetchReleaseFunc) (string, error) {
	if repo == "" {
		return "", fmt.Errorf("template repo cannot be empty")
	}

	tag, err := fetchFunc(repo)
	if err != nil {
		return "", err
	}

	return tag, nil
}

// IsUpToDate returns true if the current version matches the latest version.
func IsUpToDate(current, latest string) bool {
	return strings.TrimSpace(current) == strings.TrimSpace(latest)
}
