package cli

import (
	"fmt"

	ghAPI "github.com/cli/go-gh/v2/pkg/api"
)

// defaultDetectOwnerType detects whether a GitHub owner is a user or org via the API.
func defaultDetectOwnerType(owner string) (string, error) {
	client, err := ghAPI.DefaultRESTClient()
	if err != nil {
		return "", fmt.Errorf("creating GitHub API client: %w", err)
	}

	var resp struct {
		Type string `json:"type"`
	}
	if err := client.Get(fmt.Sprintf("users/%s", owner), &resp); err != nil {
		return "", fmt.Errorf("detecting owner type for %q: %w", owner, err)
	}
	return resp.Type, nil
}
