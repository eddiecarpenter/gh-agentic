package project

import (
	"errors"
	"strings"
	"testing"
)

// --- fake doer ---

// linkedReposPage is one scripted response from the fake doer.
type linkedReposPage struct {
	names       []string // nameWithOwner values for this page
	hasNextPage bool
	endCursor   string
	err         error // when non-nil, the doer returns this instead of a page
}

// fakeDoer serves scripted pages in order and records the `after` variable it
// was called with each time, so tests can assert cursor threading.
type fakeDoer struct {
	pages     []linkedReposPage
	callCount int
	afterSeen []interface{}
}

func (f *fakeDoer) Do(_ string, variables map[string]interface{}, response interface{}) error {
	f.afterSeen = append(f.afterSeen, variables["after"])
	if f.callCount >= len(f.pages) {
		return errors.New("fakeDoer: unexpected extra call")
	}
	page := f.pages[f.callCount]
	f.callCount++

	if page.err != nil {
		return page.err
	}

	resp, ok := response.(*graphqlLinkedReposResponse)
	if !ok {
		return errors.New("fakeDoer: unexpected response type")
	}
	for _, name := range page.names {
		resp.Node.Repositories.Nodes = append(resp.Node.Repositories.Nodes, struct {
			Name          string `json:"name"`
			NameWithOwner string `json:"nameWithOwner"`
			URL           string `json:"url"`
		}{Name: name, NameWithOwner: "owner/" + name, URL: "https://github.com/owner/" + name})
	}
	resp.Node.Repositories.PageInfo.HasNextPage = page.hasNextPage
	resp.Node.Repositories.PageInfo.EndCursor = page.endCursor
	return nil
}

func namesOf(repos []LinkedRepo) []string {
	out := make([]string, 0, len(repos))
	for _, r := range repos {
		out = append(out, r.Name)
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// --- tests ---

func TestFetchLinkedRepos_FollowsCursorAcrossPages(t *testing.T) {
	doer := &fakeDoer{pages: []linkedReposPage{
		{names: []string{"a", "b"}, hasNextPage: true, endCursor: "CURSOR1"},
		{names: []string{"c"}, hasNextPage: false},
	}}

	repos, err := fetchLinkedRepos(doer, "PVT_x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []string{"a", "b", "c"}; !equalStrings(namesOf(repos), want) {
		t.Errorf("expected %v, got %v", want, namesOf(repos))
	}
	if doer.callCount != 2 {
		t.Errorf("expected 2 calls, got %d", doer.callCount)
	}
	if doer.afterSeen[0] != nil {
		t.Errorf("first request should send after=nil, got %v", doer.afterSeen[0])
	}
	if doer.afterSeen[1] != "CURSOR1" {
		t.Errorf("second request should carry page 1's endCursor, got %v", doer.afterSeen[1])
	}
}

func TestFetchLinkedRepos_SinglePageMakesOneCall(t *testing.T) {
	doer := &fakeDoer{pages: []linkedReposPage{
		{names: []string{"only"}, hasNextPage: false},
	}}

	repos, err := fetchLinkedRepos(doer, "PVT_x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repos) != 1 {
		t.Errorf("expected 1 repo, got %d", len(repos))
	}
	if doer.callCount != 1 {
		t.Errorf("expected exactly 1 call, got %d", doer.callCount)
	}
}

func TestFetchLinkedRepos_EmptyProject(t *testing.T) {
	doer := &fakeDoer{pages: []linkedReposPage{
		{names: nil, hasNextPage: false},
	}}

	repos, err := fetchLinkedRepos(doer, "PVT_x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repos == nil {
		t.Fatal("expected an empty slice, got nil")
	}
	if len(repos) != 0 {
		t.Errorf("expected 0 repos, got %d", len(repos))
	}
	if doer.callCount != 1 {
		t.Errorf("expected exactly 1 call, got %d", doer.callCount)
	}
}

// A mid-pagination failure must not surface the pages fetched so far as if they
// were the complete set — that is the exact defect this Feature exists to fix.
func TestFetchLinkedRepos_MidPaginationErrorReturnsNoPartialSet(t *testing.T) {
	boom := errors.New("boom")
	doer := &fakeDoer{pages: []linkedReposPage{
		{names: []string{"a", "b"}, hasNextPage: true, endCursor: "CURSOR1"},
		{err: boom},
	}}

	repos, err := fetchLinkedRepos(doer, "PVT_x")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !errors.Is(err, boom) {
		t.Errorf("expected the underlying error to be wrapped, got %v", err)
	}
	if repos != nil {
		t.Errorf("expected no partial result, got %v", namesOf(repos))
	}
}

func TestFetchLinkedRepos_TerminatesWhenCursorMissing(t *testing.T) {
	// hasNextPage true but no cursor to advance with: the loop must stop
	// rather than re-requesting the same page forever.
	doer := &fakeDoer{pages: []linkedReposPage{
		{names: []string{"a"}, hasNextPage: true, endCursor: ""},
	}}

	repos, err := fetchLinkedRepos(doer, "PVT_x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repos) != 1 {
		t.Errorf("expected 1 repo, got %d", len(repos))
	}
	if doer.callCount != 1 {
		t.Errorf("expected the loop to stop after 1 call, got %d", doer.callCount)
	}
}

func TestFetchLinkedRepos_WrapsProjectIDInError(t *testing.T) {
	doer := &fakeDoer{pages: []linkedReposPage{{err: errors.New("network down")}}}

	_, err := fetchLinkedRepos(doer, "PVT_specific")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if got := err.Error(); !strings.Contains(got, "PVT_specific") {
		t.Errorf("expected the error to name the project ID, got %q", got)
	}
}
