package ui

import (
	"errors"
	"testing"
)

// withOpenURL installs the given fake OpenURLFunc for the duration of the
// test, restoring the production pointer on cleanup. Keeping the fake
// wiring in one helper prevents each test from having to remember to
// restore OpenURLFunc and risking cross-test pollution.
func withOpenURL(t *testing.T, fake func(string) error) {
	t.Helper()
	orig := OpenURLFunc
	OpenURLFunc = fake
	t.Cleanup(func() { OpenURLFunc = orig })
}

func TestOpenURL_InvokesInjectedFunc(t *testing.T) {
	var got string
	withOpenURL(t, func(u string) error {
		got = u
		return nil
	})

	if err := OpenURL("https://example.test/page"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "https://example.test/page" {
		t.Fatalf("expected injected func to receive the URL; got %q", got)
	}
}

func TestOpenURL_PropagatesError(t *testing.T) {
	sentinel := errors.New("browser failed")
	withOpenURL(t, func(string) error { return sentinel })

	err := OpenURL("https://x.test")
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected error %v, got %v", sentinel, err)
	}
}

func TestOpenURL_EmptyURL_ReturnsError(t *testing.T) {
	// The injected func must not be called when the URL is empty.
	called := false
	withOpenURL(t, func(string) error { called = true; return nil })

	if err := OpenURL(""); err == nil {
		t.Fatalf("expected error for empty URL")
	}
	if called {
		t.Fatalf("expected injected func not to be called for empty URL")
	}
}
