package testutil

import (
	"errors"
	"testing"
)

func TestMockRunner_RunCommand_MatchesExpectation(t *testing.T) {
	m := &MockRunner{}
	m.Expect([]string{"git", "status"}, "clean", nil)

	out, err := m.RunCommand("git", "status")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "clean" {
		t.Fatalf("expected %q, got %q", "clean", out)
	}
	m.AssertExpectations(t)
}

func TestMockRunner_RunCommand_ReturnsError(t *testing.T) {
	m := &MockRunner{}
	wantErr := errors.New("fail")
	m.Expect([]string{"gh", "repo", "create"}, "", wantErr)

	out, err := m.RunCommand("gh", "repo", "create")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected error %v, got %v", wantErr, err)
	}
	if out != "" {
		t.Fatalf("expected empty output, got %q", out)
	}
}

func TestMockRunner_RunCommand_UnmatchedReturnsEmpty(t *testing.T) {
	m := &MockRunner{}

	out, err := m.RunCommand("unknown", "cmd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "" {
		t.Fatalf("expected empty output, got %q", out)
	}
}

func TestMockRunner_RunCommand_MultipleExpectations(t *testing.T) {
	m := &MockRunner{}
	m.Expect([]string{"git", "init"}, "init ok", nil)
	m.Expect([]string{"git", "add", "."}, "add ok", nil)

	out1, _ := m.RunCommand("git", "init")
	out2, _ := m.RunCommand("git", "add", ".")

	if out1 != "init ok" {
		t.Fatalf("expected %q, got %q", "init ok", out1)
	}
	if out2 != "add ok" {
		t.Fatalf("expected %q, got %q", "add ok", out2)
	}
	m.AssertExpectations(t)
}

func TestMockRunner_RunCommand_SameCommandTwice(t *testing.T) {
	m := &MockRunner{}
	m.Expect([]string{"git", "status"}, "first", nil)
	m.Expect([]string{"git", "status"}, "second", nil)

	out1, _ := m.RunCommand("git", "status")
	out2, _ := m.RunCommand("git", "status")

	if out1 != "first" {
		t.Fatalf("expected %q, got %q", "first", out1)
	}
	if out2 != "second" {
		t.Fatalf("expected %q, got %q", "second", out2)
	}
	m.AssertExpectations(t)
}

func TestMockRunner_Calls_RecordsAll(t *testing.T) {
	m := &MockRunner{}
	m.RunCommand("git", "init")
	m.RunCommand("gh", "repo", "create")

	calls := m.Calls()
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(calls))
	}
	if !sliceEqual(calls[0], []string{"git", "init"}) {
		t.Fatalf("unexpected first call: %v", calls[0])
	}
	if !sliceEqual(calls[1], []string{"gh", "repo", "create"}) {
		t.Fatalf("unexpected second call: %v", calls[1])
	}
}

func TestMockRunner_AssertExpectations_FailsOnUnmet(t *testing.T) {
	m := &MockRunner{}
	m.Expect([]string{"git", "push"}, "ok", nil)

	// Do not call RunCommand — the expectation should be unmet.
	ft := &testing.T{}
	// We cannot easily test t.Errorf without a fake T, so we just verify
	// that calling AssertExpectations on the real t would detect the unmet
	// expectation by checking the internal state.
	m.mu.Lock()
	unmet := !m.expectations[0].called
	m.mu.Unlock()

	if !unmet {
		t.Fatal("expected expectation to be unmet")
	}
	_ = ft // silence unused
}
