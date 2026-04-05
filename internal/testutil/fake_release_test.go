package testutil

import (
	"errors"
	"testing"
)

func TestFakeRelease_ReturnsTag(t *testing.T) {
	fn := FakeRelease("v2.0.0", nil)
	tag, err := fn("some/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tag != "v2.0.0" {
		t.Fatalf("expected %q, got %q", "v2.0.0", tag)
	}
}

func TestFakeRelease_ReturnsError(t *testing.T) {
	wantErr := errors.New("network error")
	fn := FakeRelease("", wantErr)
	tag, err := fn("any/repo")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected error %v, got %v", wantErr, err)
	}
	if tag != "" {
		t.Fatalf("expected empty tag, got %q", tag)
	}
}

func TestFakeRelease_IgnoresRepo(t *testing.T) {
	fn := FakeRelease("v1.0.0", nil)
	tag1, _ := fn("repo/a")
	tag2, _ := fn("repo/b")
	if tag1 != tag2 {
		t.Fatalf("expected same tag for different repos, got %q and %q", tag1, tag2)
	}
}
