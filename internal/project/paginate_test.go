package project

import (
	"errors"
	"testing"
)

func TestPaginate_ThreadsCursorAcrossPages(t *testing.T) {
	var seen []interface{}
	pages := []pageInfo{
		{HasNextPage: true, EndCursor: "C1"},
		{HasNextPage: true, EndCursor: "C2"},
		{HasNextPage: false},
	}
	i := 0

	err := paginate(func(after interface{}) (pageInfo, error) {
		seen = append(seen, after)
		p := pages[i]
		i++
		return p, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(seen) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(seen))
	}
	if seen[0] != nil {
		t.Errorf("first call should pass a nil cursor, got %v", seen[0])
	}
	if seen[1] != "C1" || seen[2] != "C2" {
		t.Errorf("cursors not threaded in order: %v", seen)
	}
}

func TestPaginate_SinglePageMakesOneCall(t *testing.T) {
	calls := 0

	err := paginate(func(after interface{}) (pageInfo, error) {
		calls++
		return pageInfo{HasNextPage: false}, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected exactly 1 call, got %d", calls)
	}
}

func TestPaginate_ErrorAbortsLoop(t *testing.T) {
	boom := errors.New("boom")
	calls := 0

	err := paginate(func(after interface{}) (pageInfo, error) {
		calls++
		if calls == 2 {
			return pageInfo{}, boom
		}
		return pageInfo{HasNextPage: true, EndCursor: "C1"}, nil
	})
	if !errors.Is(err, boom) {
		t.Fatalf("expected the page error to propagate, got %v", err)
	}
	if calls != 2 {
		t.Errorf("expected the loop to stop on the failing call, got %d calls", calls)
	}
}

// hasNextPage true with no cursor would otherwise re-request the same page
// forever.
func TestPaginate_TerminatesWhenCursorMissing(t *testing.T) {
	calls := 0

	err := paginate(func(after interface{}) (pageInfo, error) {
		calls++
		if calls > 5 {
			t.Fatal("paginate did not terminate on an empty cursor")
		}
		return pageInfo{HasNextPage: true, EndCursor: ""}, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected the loop to stop after 1 call, got %d", calls)
	}
}
