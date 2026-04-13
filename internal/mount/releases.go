package mount

import (
	"fmt"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
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
type FetchReleaseFunc func(repo string) (string, error)

// FetchReleasesFunc fetches all releases for a given repo slug.
type FetchReleasesFunc func(repo string) ([]Release, error)

// DefaultFetchReleases fetches all releases from the GitHub API using pagination.
// Returns releases ordered newest-first (as returned by the API).
func DefaultFetchReleases(repo string) ([]Release, error) {
	client, err := api.DefaultRESTClient()
	if err != nil {
		return nil, fmt.Errorf("creating GitHub API client: %w", err)
	}

	var all []Release
	page := 1

	for {
		var batch []Release
		endpoint := fmt.Sprintf("repos/%s/releases?per_page=100&page=%d", repo, page)

		if err := client.Get(endpoint, &batch); err != nil {
			return nil, fmt.Errorf("fetching releases for %s (page %d): %w", repo, page, err)
		}

		if len(batch) == 0 {
			break
		}

		all = append(all, batch...)
		page++
	}

	return all, nil
}

// FindReleaseByTag searches a slice of releases for one matching the given tag.
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

// FilterReleasesSince filters releases to only those newer than the given version.
// Returns newest-first order. Tags that are not valid semver are skipped.
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

	semverSort(filtered)
	return filtered
}
