package project

import (
	"errors"
	"testing"
)

type viewsPage struct {
	names       []string
	hasNextPage bool
	endCursor   string
	err         error
}

type fakeViewsDoer struct {
	pages     []viewsPage
	callCount int
	afterSeen []interface{}
}

func (f *fakeViewsDoer) Do(_ string, variables map[string]interface{}, response interface{}) error {
	f.afterSeen = append(f.afterSeen, variables["after"])
	if f.callCount >= len(f.pages) {
		return errors.New("fakeViewsDoer: unexpected extra call")
	}
	page := f.pages[f.callCount]
	f.callCount++

	if page.err != nil {
		return page.err
	}
	resp, ok := response.(*graphqlProjectViewsResponse)
	if !ok {
		return errors.New("fakeViewsDoer: unexpected response type")
	}
	for _, n := range page.names {
		resp.Node.Views.Nodes = append(resp.Node.Views.Nodes, struct {
			Name string `json:"name"`
		}{Name: n})
	}
	resp.Node.Views.PageInfo = pageInfo{HasNextPage: page.hasNextPage, EndCursor: page.endCursor}
	return nil
}

func TestFetchProjectViews_FollowsCursorAcrossPages(t *testing.T) {
	doer := &fakeViewsDoer{pages: []viewsPage{
		{names: []string{"Board", "Table"}, hasNextPage: true, endCursor: "C1"},
		{names: []string{"Roadmap"}, hasNextPage: false},
	}}

	got, err := fetchProjectViews(doer, "PVT_x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 views across both pages, got %d", len(got))
	}
	if doer.afterSeen[0] != nil || doer.afterSeen[1] != "C1" {
		t.Errorf("cursor not threaded: %v", doer.afterSeen)
	}
}

func TestFetchProjectViews_SinglePage(t *testing.T) {
	doer := &fakeViewsDoer{pages: []viewsPage{
		{names: []string{"Board"}, hasNextPage: false},
	}}

	got, err := fetchProjectViews(doer, "PVT_x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Board" {
		t.Errorf("expected the Board view, got %+v", got)
	}
	if doer.callCount != 1 {
		t.Errorf("expected exactly 1 call, got %d", doer.callCount)
	}
}

func TestFetchProjectViews_MidPaginationErrorReturnsNoPartialSet(t *testing.T) {
	boom := errors.New("boom")
	doer := &fakeViewsDoer{pages: []viewsPage{
		{names: []string{"Board"}, hasNextPage: true, endCursor: "C1"},
		{err: boom},
	}}

	got, err := fetchProjectViews(doer, "PVT_x")
	if !errors.Is(err, boom) {
		t.Fatalf("expected the page error to propagate, got %v", err)
	}
	if got != nil {
		t.Errorf("expected no partial result, got %d views", len(got))
	}
}
