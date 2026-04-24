package githubapp

import (
	"net/url"
	"strings"
	"testing"
)

func TestInstallURL_RepoTarget_HasCanonicalPrefix(t *testing.T) {
	got := InstallURL("gh-agentic-app", TargetRepo, "owner/repo")
	if !strings.HasPrefix(got, "https://github.com/apps/gh-agentic-app/installations/new") {
		t.Fatalf("expected canonical prefix in %q", got)
	}
}

func TestInstallURL_OrgTarget_HasCanonicalPrefix(t *testing.T) {
	got := InstallURL("gh-agentic-app", TargetOrg, "acme")
	if !strings.HasPrefix(got, "https://github.com/apps/gh-agentic-app/installations/new") {
		t.Fatalf("expected canonical prefix in %q", got)
	}
}

func TestInstallURL_EncodesTargetNameInState(t *testing.T) {
	got := InstallURL("gh-agentic-app", TargetRepo, "owner/repo")
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("could not parse URL %q: %v", got, err)
	}
	state := u.Query().Get("state")
	if state != "repo:owner/repo" {
		t.Fatalf("expected state=repo:owner/repo, got %q", state)
	}
}

func TestInstallURL_OrgStatePrefix(t *testing.T) {
	got := InstallURL("gh-agentic-app", TargetOrg, "acme")
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("could not parse URL %q: %v", got, err)
	}
	if state := u.Query().Get("state"); state != "org:acme" {
		t.Fatalf("expected state=org:acme, got %q", state)
	}
}

func TestInstallURL_EmptyTargetName_NoStateParam(t *testing.T) {
	got := InstallURL("gh-agentic-app", TargetRepo, "")
	if got != "https://github.com/apps/gh-agentic-app/installations/new" {
		t.Fatalf("expected bare URL without query, got %q", got)
	}
}

func TestInstallURL_EmptySlug_ReturnsEmpty(t *testing.T) {
	if got := InstallURL("", TargetRepo, "owner/repo"); got != "" {
		t.Fatalf("expected empty string for empty slug, got %q", got)
	}
}

func TestInstallURL_EscapesSlug(t *testing.T) {
	// A slug with a path-separator character must not break the URL
	// structure — the PathEscape call in InstallURL covers this. We
	// would never ship such a slug in production, but the defence-in-
	// depth is worth the test because a future misconfigured build
	// could set a hostile value.
	got := InstallURL("weird/slug", TargetOrg, "acme")
	if strings.Contains(got, "apps/weird/slug/") {
		t.Fatalf("slug not escaped in %q — risk of URL structure injection", got)
	}
	if !strings.Contains(got, "weird%2Fslug") {
		t.Fatalf("expected escaped slug in %q", got)
	}
}

func TestTargetType_String(t *testing.T) {
	cases := []struct {
		tt   TargetType
		want string
	}{
		{TargetRepo, "repo"},
		{TargetOrg, "org"},
		{TargetType(99), "unknown"},
	}
	for _, tc := range cases {
		if got := tc.tt.String(); got != tc.want {
			t.Errorf("TargetType(%d).String() = %q, want %q", tc.tt, got, tc.want)
		}
	}
}
