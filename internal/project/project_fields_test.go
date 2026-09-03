package project

import (
	"errors"
	"testing"
)

// fieldsPage is one scripted response from fakeFieldsDoer.
type fieldsPage struct {
	names       []string
	hasNextPage bool
	endCursor   string
	err         error
}

type fakeFieldsDoer struct {
	pages     []fieldsPage
	callCount int
	afterSeen []interface{}
}

func (f *fakeFieldsDoer) Do(_ string, variables map[string]interface{}, response interface{}) error {
	f.afterSeen = append(f.afterSeen, variables["after"])
	if f.callCount >= len(f.pages) {
		return errors.New("fakeFieldsDoer: unexpected extra call")
	}
	page := f.pages[f.callCount]
	f.callCount++

	if page.err != nil {
		return page.err
	}
	resp, ok := response.(*graphqlProjectFieldsResponse)
	if !ok {
		return errors.New("fakeFieldsDoer: unexpected response type")
	}
	for _, n := range page.names {
		resp.Node.Fields.Nodes = append(resp.Node.Fields.Nodes, struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			DataType string `json:"dataType"`
			Options  []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"options"`
		}{ID: "F_" + n, Name: n, DataType: "TEXT"})
	}
	resp.Node.Fields.PageInfo.HasNextPage = page.hasNextPage
	resp.Node.Fields.PageInfo.EndCursor = page.endCursor
	return nil
}

func TestFetchProjectFields_FollowsCursorAcrossPages(t *testing.T) {
	doer := &fakeFieldsDoer{pages: []fieldsPage{
		{names: []string{"Title", "Status"}, hasNextPage: true, endCursor: "C1"},
		{names: []string{"Target repo"}, hasNextPage: false},
	}}

	fields, err := fetchProjectFields(doer, "PVT_x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 3 {
		t.Fatalf("expected 3 fields across both pages, got %d", len(fields))
	}
	if doer.afterSeen[0] != nil {
		t.Errorf("first request should send after=nil, got %v", doer.afterSeen[0])
	}
	if doer.afterSeen[1] != "C1" {
		t.Errorf("second request should carry page 1's endCursor, got %v", doer.afterSeen[1])
	}
}

// The regression that matters: a field beyond the first page must be findable,
// because EnsureTargetRepoField creates the field when FindField reports absent.
func TestFetchProjectFields_FindsFieldBeyondFirstPage(t *testing.T) {
	doer := &fakeFieldsDoer{pages: []fieldsPage{
		{names: []string{"Title", "Status", "Assignees"}, hasNextPage: true, endCursor: "C1"},
		{names: []string{TargetRepoFieldName}, hasNextPage: false},
	}}

	fields, err := fetchProjectFields(doer, "PVT_x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := FindField(fields, TargetRepoFieldName); !ok {
		t.Errorf("%q sits on page 2 and must be found; a miss here makes repair create a duplicate field", TargetRepoFieldName)
	}
}

func TestFetchProjectFields_SinglePage(t *testing.T) {
	doer := &fakeFieldsDoer{pages: []fieldsPage{
		{names: []string{"Title"}, hasNextPage: false},
	}}

	fields, err := fetchProjectFields(doer, "PVT_x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 1 {
		t.Errorf("expected 1 field, got %d", len(fields))
	}
	if doer.callCount != 1 {
		t.Errorf("expected exactly 1 call, got %d", doer.callCount)
	}
}

func TestFetchProjectFields_MidPaginationErrorReturnsNoPartialSet(t *testing.T) {
	boom := errors.New("boom")
	doer := &fakeFieldsDoer{pages: []fieldsPage{
		{names: []string{"Title", "Status"}, hasNextPage: true, endCursor: "C1"},
		{err: boom},
	}}

	fields, err := fetchProjectFields(doer, "PVT_x")
	if !errors.Is(err, boom) {
		t.Fatalf("expected the page error to propagate, got %v", err)
	}
	if fields != nil {
		t.Errorf("a partial field list must not be returned — a caller would treat the missing fields as absent and create duplicates; got %d fields", len(fields))
	}
}

func TestFetchProjectFields_PreservesSingleSelectOptions(t *testing.T) {
	doer := &fakeFieldsDoer{pages: []fieldsPage{
		{names: []string{"Status"}, hasNextPage: false},
	}}

	fields, err := fetchProjectFields(doer, "PVT_x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 1 || fields[0].Name != "Status" {
		t.Fatalf("expected the Status field, got %+v", fields)
	}
}
