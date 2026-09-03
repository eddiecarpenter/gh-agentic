package project

import (
	"errors"
	"testing"
)

// projectsPage is one scripted page of a projectsV2 connection.
type projectsPage struct {
	titles      []string
	hasNextPage bool
	endCursor   string
	err         error
}

// fakeProjectsDoer serves scripted pages into whichever of the three projectsV2
// response shapes the caller passed, so one fake covers repo, org and viewer.
type fakeProjectsDoer struct {
	pages     []projectsPage
	callCount int
	afterSeen []interface{}
}

func (f *fakeProjectsDoer) Do(_ string, variables map[string]interface{}, response interface{}) error {
	f.afterSeen = append(f.afterSeen, variables["after"])
	if f.callCount >= len(f.pages) {
		return errors.New("fakeProjectsDoer: unexpected extra call")
	}
	page := f.pages[f.callCount]
	f.callCount++

	if page.err != nil {
		return page.err
	}

	conn := projectNodes{}
	for _, tl := range page.titles {
		conn.Nodes = append(conn.Nodes, struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		}{ID: "P_" + tl, Title: tl})
	}
	conn.PageInfo = pageInfo{HasNextPage: page.hasNextPage, EndCursor: page.endCursor}

	switch r := response.(type) {
	case *graphqlProjectsForRepoResponse:
		r.Repository.ProjectsV2 = conn
	case *graphqlOrgProjectsResponse:
		r.Organization.ProjectsV2 = conn
	case *graphqlUserProjectsResponse:
		r.Viewer.ProjectsV2 = conn
	default:
		return errors.New("fakeProjectsDoer: unexpected response type")
	}
	return nil
}

func titlesOf(projects []ProjectInfo) []string {
	out := make([]string, 0, len(projects))
	for _, p := range projects {
		out = append(out, p.Title)
	}
	return out
}

// --- repo projects (was first: 10) ---

func TestFetchProjectsForRepo_FollowsCursorAcrossPages(t *testing.T) {
	doer := &fakeProjectsDoer{pages: []projectsPage{
		{titles: []string{"Roadmap", "Bugs"}, hasNextPage: true, endCursor: "C1"},
		{titles: []string{"Backlog"}, hasNextPage: false},
	}}

	got, err := fetchProjectsForRepo(doer, "acme", "widget")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 projects across both pages, got %v", titlesOf(got))
	}
	if doer.afterSeen[0] != nil || doer.afterSeen[1] != "C1" {
		t.Errorf("cursor not threaded: %v", doer.afterSeen)
	}
}

func TestFetchProjectsForRepo_MidPaginationErrorReturnsNoPartialSet(t *testing.T) {
	boom := errors.New("boom")
	doer := &fakeProjectsDoer{pages: []projectsPage{
		{titles: []string{"Roadmap"}, hasNextPage: true, endCursor: "C1"},
		{err: boom},
	}}

	got, err := fetchProjectsForRepo(doer, "acme", "widget")
	if !errors.Is(err, boom) {
		t.Fatalf("expected the page error to propagate, got %v", err)
	}
	if got != nil {
		t.Errorf("expected no partial result, got %v", titlesOf(got))
	}
}

// --- owner projects (was first: 50, org and viewer branches) ---

// AC-1: an owner with more than 50 projects must have every one returned.
func TestFetchProjectsForOwner_Org_AboveOldCap(t *testing.T) {
	first := make([]string, 50)
	for i := range first {
		first[i] = "p" + string(rune('A'+i%26)) + string(rune('0'+i/26))
	}
	doer := &fakeProjectsDoer{pages: []projectsPage{
		{titles: first, hasNextPage: true, endCursor: "C1"},
		{titles: []string{"p51", "p52"}, hasNextPage: false},
	}}

	got, err := fetchProjectsForOwner(doer, "acme", "Organization")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 52 {
		t.Errorf("expected all 52 projects past the old 50 cap, got %d", len(got))
	}
}

func TestFetchProjectsForOwner_Viewer_ThreadsCursor(t *testing.T) {
	doer := &fakeProjectsDoer{pages: []projectsPage{
		{titles: []string{"mine1"}, hasNextPage: true, endCursor: "C1"},
		{titles: []string{"mine2"}, hasNextPage: false},
	}}

	got, err := fetchProjectsForOwner(doer, "someone", "User")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 projects, got %v", titlesOf(got))
	}
	// The viewer query previously passed an empty variables map; it must now
	// carry $after or pagination silently re-requests page one.
	if doer.afterSeen[1] != "C1" {
		t.Errorf("viewer query did not thread the cursor: %v", doer.afterSeen)
	}
}

func TestFetchProjectsForOwner_EmptyOwnerReturnsEmptySlice(t *testing.T) {
	doer := &fakeProjectsDoer{pages: []projectsPage{{hasNextPage: false}}}

	got, err := fetchProjectsForOwner(doer, "acme", "Organization")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected an empty slice, got nil")
	}
	if len(got) != 0 {
		t.Errorf("expected 0 projects, got %d", len(got))
	}
}
