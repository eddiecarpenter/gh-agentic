package testutil

import (
	"bytes"
	"errors"
	"testing"
)

func TestNoopSpinner_ExecutesFn(t *testing.T) {
	called := false
	err := NoopSpinner(&bytes.Buffer{}, "test", func() error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected fn to be called")
	}
}

func TestNoopSpinner_PropagatesError(t *testing.T) {
	wantErr := errors.New("boom")
	err := NoopSpinner(&bytes.Buffer{}, "test", func() error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected error %v, got %v", wantErr, err)
	}
}

func TestNoopSpinner_NilWriter(t *testing.T) {
	err := NoopSpinner(nil, "test", func() error {
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
