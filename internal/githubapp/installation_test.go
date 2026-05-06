package githubapp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"
)

// fakeClient implements RESTClient for unit tests. It ignores the body and
// records the last call; callers control the outcome via the result
// function so they can exercise 200/404/500/network paths.
type fakeClient struct {
	calls  []fakeCall
	result func(method, path string, out interface{}) error
}

type fakeCall struct {
	method string
	path   string
}

func (f *fakeClient) DoWithContext(ctx context.Context, method, path string, body io.Reader, out interface{}) error {
	f.calls = append(f.calls, fakeCall{method: method, path: path})
	if f.result == nil {
		return nil
	}
	return f.result(method, path, out)
}

// jsonResult writes the given payload into out as if a 200 had returned.
func jsonResult(payload interface{}) func(string, string, interface{}) error {
	return func(_ string, _ string, out interface{}) error {
		buf, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		return json.Unmarshal(buf, out)
	}
}

// httpError returns a function that always fails with an *api.HTTPError of
// the given status code — this matches what go-gh produces on non-2xx.
func httpError(status int) func(string, string, interface{}) error {
	return func(_ string, _ string, _ interface{}) error {
		u, _ := url.Parse("https://api.github.com/test")
		return &api.HTTPError{StatusCode: status, RequestURL: u, Message: http.StatusText(status)}
	}
}

func TestCheckRepoInstallation_MatchingApp_ReturnsInstalled(t *testing.T) {
	f := &fakeClient{result: jsonResult(map[string]interface{}{
		"id":       int64(12345),
		"app_id":   int64(999),
		"app_slug": "gh-agentic-app",
	})}
	c := &Checker{Client: f, AppSlug: "gh-agentic-app"}

	installed, id, err := c.CheckRepoInstallation(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !installed {
		t.Fatalf("expected installed=true, got false")
	}
	if id != 12345 {
		t.Fatalf("expected installation id 12345, got %d", id)
	}
	if len(f.calls) != 1 || f.calls[0].method != http.MethodGet {
		t.Fatalf("expected single GET call, got %v", f.calls)
	}
	if f.calls[0].path != "repos/owner/repo/installation" {
		t.Fatalf("unexpected path %q", f.calls[0].path)
	}
}

func TestCheckRepoInstallation_WrongApp_ReturnsNotInstalled(t *testing.T) {
	f := &fakeClient{result: jsonResult(map[string]interface{}{
		"id":       int64(42),
		"app_slug": "some-other-app",
	})}
	c := &Checker{Client: f, AppSlug: "gh-agentic-app"}

	installed, id, err := c.CheckRepoInstallation(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if installed {
		t.Fatalf("expected installed=false for wrong-app response, got true")
	}
	if id != 0 {
		t.Fatalf("expected id=0 for wrong app, got %d", id)
	}
}

func TestCheckRepoInstallation_NotFound_ReturnsNotInstalled(t *testing.T) {
	f := &fakeClient{result: httpError(http.StatusNotFound)}
	c := &Checker{Client: f, AppSlug: "gh-agentic-app"}

	installed, id, err := c.CheckRepoInstallation(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if installed {
		t.Fatalf("expected installed=false on 404, got true")
	}
	if id != 0 {
		t.Fatalf("expected id=0 on 404, got %d", id)
	}
}

func TestCheckRepoInstallation_Unauthorized_ReturnsNotInstalled(t *testing.T) {
	// 401 means the endpoint requires App JWT auth; a regular user token is
	// rejected. Treat as "cannot verify" — return not-installed with no error
	// so the wizard can offer the install URL interactively.
	f := &fakeClient{result: httpError(http.StatusUnauthorized)}
	c := &Checker{Client: f, AppSlug: "gh-agentic-app"}

	installed, id, err := c.CheckRepoInstallation(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatalf("expected no error on 401, got: %v", err)
	}
	if installed || id != 0 {
		t.Fatalf("expected installed=false id=0 on 401, got installed=%v id=%d", installed, id)
	}
}

func TestCheckRepoInstallation_Forbidden_ReturnsNotInstalled(t *testing.T) {
	f := &fakeClient{result: httpError(http.StatusForbidden)}
	c := &Checker{Client: f, AppSlug: "gh-agentic-app"}

	installed, id, err := c.CheckRepoInstallation(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatalf("expected no error on 403, got: %v", err)
	}
	if installed || id != 0 {
		t.Fatalf("expected installed=false id=0 on 403, got installed=%v id=%d", installed, id)
	}
}

func TestCheckRepoInstallation_ServerError_ReturnsWrappedError(t *testing.T) {
	f := &fakeClient{result: httpError(http.StatusInternalServerError)}
	c := &Checker{Client: f, AppSlug: "gh-agentic-app"}

	installed, id, err := c.CheckRepoInstallation(context.Background(), "owner", "repo")
	if err == nil {
		t.Fatalf("expected error for 500, got nil")
	}
	if installed || id != 0 {
		t.Fatalf("expected zero values on error, got installed=%v id=%d", installed, id)
	}
	if !strings.Contains(err.Error(), "checking installation at repos/owner/repo/installation") {
		t.Fatalf("error not wrapped with endpoint context: %v", err)
	}
	var httpErr *api.HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected wrapped error to include *api.HTTPError, got %T: %v", err, err)
	}
	if httpErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected underlying status 500, got %d", httpErr.StatusCode)
	}
}

func TestCheckRepoInstallation_NetworkError_Bubbles(t *testing.T) {
	sentinel := errors.New("boom: no route to host")
	f := &fakeClient{result: func(string, string, interface{}) error { return sentinel }}
	c := &Checker{Client: f, AppSlug: "gh-agentic-app"}

	_, _, err := c.CheckRepoInstallation(context.Background(), "owner", "repo")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected wrapped error to include sentinel, got %v", err)
	}
}

func TestCheckRepoInstallation_EmptyOwnerOrRepo_Error(t *testing.T) {
	c := &Checker{Client: &fakeClient{}, AppSlug: "gh-agentic-app"}
	if _, _, err := c.CheckRepoInstallation(context.Background(), "", "repo"); err == nil {
		t.Fatalf("expected error for empty owner")
	}
	if _, _, err := c.CheckRepoInstallation(context.Background(), "owner", ""); err == nil {
		t.Fatalf("expected error for empty repo")
	}
}

func TestCheckOrgInstallation_MatchingApp_ReturnsInstalled(t *testing.T) {
	f := &fakeClient{result: jsonResult(map[string]interface{}{
		"id":       int64(777),
		"app_slug": "gh-agentic-app",
	})}
	c := &Checker{Client: f, AppSlug: "gh-agentic-app"}

	installed, id, err := c.CheckOrgInstallation(context.Background(), "acme")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !installed || id != 777 {
		t.Fatalf("expected installed=true id=777, got installed=%v id=%d", installed, id)
	}
	if f.calls[0].path != "orgs/acme/installation" {
		t.Fatalf("unexpected path %q", f.calls[0].path)
	}
}

func TestCheckOrgInstallation_WrongApp_ReturnsNotInstalled(t *testing.T) {
	f := &fakeClient{result: jsonResult(map[string]interface{}{"id": int64(5), "app_slug": "other"})}
	c := &Checker{Client: f, AppSlug: "gh-agentic-app"}

	installed, _, err := c.CheckOrgInstallation(context.Background(), "acme")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if installed {
		t.Fatalf("expected installed=false for slug mismatch")
	}
}

func TestCheckOrgInstallation_NotFound_ReturnsNotInstalled(t *testing.T) {
	f := &fakeClient{result: httpError(http.StatusNotFound)}
	c := &Checker{Client: f, AppSlug: "gh-agentic-app"}

	installed, _, err := c.CheckOrgInstallation(context.Background(), "acme")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if installed {
		t.Fatalf("expected installed=false on 404")
	}
}

func TestCheckOrgInstallation_Unauthorized_ReturnsNotInstalled(t *testing.T) {
	f := &fakeClient{result: httpError(http.StatusUnauthorized)}
	c := &Checker{Client: f, AppSlug: "gh-agentic-app"}

	installed, id, err := c.CheckOrgInstallation(context.Background(), "acme")
	if err != nil {
		t.Fatalf("expected no error on 401, got: %v", err)
	}
	if installed || id != 0 {
		t.Fatalf("expected installed=false id=0 on 401, got installed=%v id=%d", installed, id)
	}
}

func TestCheckOrgInstallation_Forbidden_ReturnsNotInstalled(t *testing.T) {
	f := &fakeClient{result: httpError(http.StatusForbidden)}
	c := &Checker{Client: f, AppSlug: "gh-agentic-app"}

	installed, _, err := c.CheckOrgInstallation(context.Background(), "acme")
	if err != nil {
		t.Fatalf("expected no error on 403, got: %v", err)
	}
	if installed {
		t.Fatalf("expected installed=false on 403")
	}
}

func TestCheckOrgInstallation_ServerError_ReturnsWrappedError(t *testing.T) {
	f := &fakeClient{result: httpError(http.StatusBadGateway)}
	c := &Checker{Client: f, AppSlug: "gh-agentic-app"}

	_, _, err := c.CheckOrgInstallation(context.Background(), "acme")
	if err == nil {
		t.Fatalf("expected error for 502")
	}
	if !strings.Contains(err.Error(), "orgs/acme/installation") {
		t.Fatalf("error not wrapped with endpoint context: %v", err)
	}
}

func TestCheckOrgInstallation_EmptyOrg_Error(t *testing.T) {
	c := &Checker{Client: &fakeClient{}, AppSlug: "gh-agentic-app"}
	if _, _, err := c.CheckOrgInstallation(context.Background(), ""); err == nil {
		t.Fatalf("expected error for empty org")
	}
}

func TestCheck_EmptyAppSlug_AcceptsAnyInstalledApp(t *testing.T) {
	// When no slug is configured the Checker trusts any 200 response.
	// This is the "do not match on slug" mode that advanced callers opt
	// into; it must still propagate the installation id.
	f := &fakeClient{result: jsonResult(map[string]interface{}{"id": int64(11), "app_slug": "anything"})}
	c := &Checker{Client: f, AppSlug: ""}

	installed, id, err := c.CheckRepoInstallation(context.Background(), "o", "r")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !installed || id != 11 {
		t.Fatalf("expected installed=true id=11, got installed=%v id=%d", installed, id)
	}
}

func TestCheck_NilClient_ReturnsError(t *testing.T) {
	c := &Checker{Client: nil, AppSlug: "gh-agentic-app"}
	if _, _, err := c.CheckRepoInstallation(context.Background(), "o", "r"); err == nil {
		t.Fatalf("expected error when Client is nil")
	}
	if _, _, err := c.CheckOrgInstallation(context.Background(), "acme"); err == nil {
		t.Fatalf("expected error when Client is nil")
	}
}

func TestDefaultAppSlug_Constant(t *testing.T) {
	if DefaultAppSlug == "" {
		t.Fatalf("DefaultAppSlug must be a non-empty constant")
	}
}
