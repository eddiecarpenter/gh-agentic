package bootstrap

import (
	"fmt"

	"github.com/cli/go-gh/v2/pkg/api"
)

// OwnerType constants identify whether a GitHub owner is a personal account or an organisation.
const (
	OwnerTypeUser = "User"
	OwnerTypeOrg  = "Organization"
)

// DetectOwnerTypeFunc detects whether a GitHub owner is a personal account or an organisation.
// Injected so tests can substitute a fake implementation without real gh auth.
type DetectOwnerTypeFunc func(owner string) (string, error)

// ghOwnerTypeResp is the API response shape for GET /users/<owner>.
type ghOwnerTypeResp struct {
	Type string `json:"type"`
}

// DefaultDetectOwnerType calls GET users/<owner> via the authenticated go-gh/v2 REST client
// and returns OwnerTypeUser or OwnerTypeOrg based on the response "type" field.
func DefaultDetectOwnerType(owner string) (string, error) {
	client, err := api.DefaultRESTClient()
	if err != nil {
		return "", fmt.Errorf("creating GitHub API client: %w", err)
	}

	var resp ghOwnerTypeResp
	if err := client.Get(fmt.Sprintf("users/%s", owner), &resp); err != nil {
		return "", fmt.Errorf("fetching owner type for %q: %w", owner, err)
	}

	switch resp.Type {
	case OwnerTypeUser:
		return OwnerTypeUser, nil
	case OwnerTypeOrg:
		return OwnerTypeOrg, nil
	default:
		return "", fmt.Errorf("unexpected owner type %q for %q", resp.Type, owner)
	}
}
