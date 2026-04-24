// Package githubapp provides helpers for detecting and guiding installation
// of the agentic GitHub App on a target repository or organisation.
//
// The detection API wraps the GitHub REST endpoints:
//
//	GET /repos/{owner}/{repo}/installation
//	GET /orgs/{org}/installation
//
// A 200 response means an App is installed; a 404 means none is installed.
// Consumers decide whether the installed App matches the agentic App by
// comparing the returned `app_slug` against a configured expected slug.
package githubapp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/cli/go-gh/v2/pkg/api"
)

// DefaultAppSlug is the slug of the agentic GitHub App. This is the stable
// value used by the canonical App listed on GitHub and is the default value
// Checker instances are constructed with. Tests may override it via
// NewChecker to exercise the "wrong app installed" path.
const DefaultAppSlug = "gh-agentic-app"

// RESTClient is the minimal surface the installation checks need from a
// GitHub REST client. It is satisfied by *api.RESTClient from
// github.com/cli/go-gh/v2/pkg/api. Tests substitute a fake implementation
// so they never touch the network.
type RESTClient interface {
	DoWithContext(ctx context.Context, method string, path string, body io.Reader, response interface{}) error
}

// installationResponse is the subset of the GitHub installation JSON this
// package cares about. Additional fields are intentionally ignored — the
// App-installation endpoint returns a richer payload, but only id and
// app_slug are needed for detection.
type installationResponse struct {
	ID      int64  `json:"id"`
	AppID   int64  `json:"app_id"`
	AppSlug string `json:"app_slug"`
}

// Checker detects whether the agentic GitHub App is installed on a given
// target (repo or org). A Checker is cheap to construct and safe for
// concurrent use across goroutines — the underlying REST client is
// goroutine-safe.
type Checker struct {
	// Client is the REST client used for all installation lookups. When
	// nil, Check* methods return an error. Use NewChecker to construct a
	// ready-to-use Checker.
	Client RESTClient

	// AppSlug is the expected app slug. When set, Check* methods return
	// Installed=false for responses whose app_slug does not match. When
	// empty, any 200 response is treated as installed regardless of slug.
	AppSlug string
}

// NewChecker constructs a Checker wired to the default gh REST client and
// the supplied app slug. Pass DefaultAppSlug for production callers; tests
// and advanced consumers may pass a different slug (or the empty string to
// disable slug matching entirely).
func NewChecker(appSlug string) (*Checker, error) {
	client, err := api.DefaultRESTClient()
	if err != nil {
		return nil, fmt.Errorf("creating REST client: %w", err)
	}
	return &Checker{Client: client, AppSlug: appSlug}, nil
}

// CheckRepoInstallation reports whether the agentic GitHub App is installed
// on the given repository. It returns Installed=true and the installation
// ID on a 200 response whose app_slug matches the Checker's configured
// AppSlug. It returns Installed=false on a 404 or on a 200 response with a
// non-matching app_slug. All other errors (5xx, transport failures, etc.)
// are wrapped and returned.
func (c *Checker) CheckRepoInstallation(ctx context.Context, owner, repo string) (bool, int64, error) {
	if owner == "" || repo == "" {
		return false, 0, fmt.Errorf("owner and repo must be non-empty")
	}
	return c.check(ctx, fmt.Sprintf("repos/%s/%s/installation", owner, repo))
}

// CheckOrgInstallation reports whether the agentic GitHub App is installed
// on the given organisation. Semantics match CheckRepoInstallation — 404 is
// not-installed, slug mismatch is not-installed, 5xx is an error.
func (c *Checker) CheckOrgInstallation(ctx context.Context, org string) (bool, int64, error) {
	if org == "" {
		return false, 0, fmt.Errorf("org must be non-empty")
	}
	return c.check(ctx, fmt.Sprintf("orgs/%s/installation", org))
}

// check performs the installation lookup against the given endpoint and
// applies the uniform 404/slug-mismatch/other-error semantics. It is the
// single source of truth for how Checker interprets the installation
// endpoints — the exported Check* methods differ only in the endpoint path
// they hand to it.
func (c *Checker) check(ctx context.Context, endpoint string) (bool, int64, error) {
	if c.Client == nil {
		return false, 0, fmt.Errorf("githubapp: REST client is not configured")
	}

	var resp installationResponse
	if err := c.Client.DoWithContext(ctx, http.MethodGet, endpoint, nil, &resp); err != nil {
		var httpErr *api.HTTPError
		if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusNotFound {
			return false, 0, nil
		}
		return false, 0, fmt.Errorf("checking installation at %s: %w", endpoint, err)
	}

	if c.AppSlug != "" && resp.AppSlug != c.AppSlug {
		return false, 0, nil
	}
	return true, resp.ID, nil
}
