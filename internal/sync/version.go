package sync

import (
	"fmt"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
)

// FetchReleaseFunc fetches the latest release tag for a given repo slug.
// Injected so tests can substitute a fake implementation without real GitHub API calls.
type FetchReleaseFunc func(repo string) (string, error)

// ghReleaseResp is the API response shape for GET /repos/{owner}/{repo}/releases/latest.
type ghReleaseResp struct {
	TagName string `json:"tag_name"`
}

// DefaultFetchRelease fetches the latest release tag using the authenticated go-gh/v2 REST client.
func DefaultFetchRelease(repo string) (string, error) {
	client, err := api.DefaultRESTClient()
	if err != nil {
		return "", fmt.Errorf("creating GitHub API client: %w", err)
	}

	var release ghReleaseResp
	endpoint := fmt.Sprintf("repos/%s/releases/latest", repo)

	if err := client.Get(endpoint, &release); err != nil {
		return "", fmt.Errorf("fetching latest release for %s: %w", repo, err)
	}

	tag := strings.TrimSpace(release.TagName)
	if tag == "" {
		return "", fmt.Errorf("no release tag found for %s", repo)
	}

	return tag, nil
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
