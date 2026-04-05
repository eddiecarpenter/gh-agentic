package testutil

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

// expectation holds a single scripted command response.
type expectation struct {
	args   []string
	stdout string
	err    error
	called bool
}

// MockRunner captures invocations of shell commands and returns scripted
// responses. It is safe for concurrent use. Unmatched commands return ("", nil)
// by default.
type MockRunner struct {
	mu           sync.Mutex
	expectations []*expectation
	calls        [][]string
}

// Expect registers a scripted response. When RunCommand is called with
// arguments matching args (name prepended), the given stdout and err are
// returned. Expectations are matched in registration order; the first
// unconsumed match wins.
func (m *MockRunner) Expect(args []string, stdout string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expectations = append(m.expectations, &expectation{
		args:   args,
		stdout: stdout,
		err:    err,
	})
}

// RunCommand satisfies the bootstrap.RunCommandFunc signature:
//
//	func(name string, args ...string) (string, error)
//
// It records the call, matches it against expectations, and returns the
// scripted response. Unmatched calls return ("", nil).
func (m *MockRunner) RunCommand(name string, args ...string) (string, error) {
	full := append([]string{name}, args...)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.calls = append(m.calls, full)

	for _, e := range m.expectations {
		if !e.called && sliceEqual(e.args, full) {
			e.called = true
			return e.stdout, e.err
		}
	}
	return "", nil
}

// AssertExpectations fails the test if any registered expectation was not
// consumed by a matching RunCommand call.
func (m *MockRunner) AssertExpectations(t *testing.T) {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, e := range m.expectations {
		if !e.called {
			t.Errorf("MockRunner: expectation %d not met: %s", i, formatArgs(e.args))
		}
	}
}

// Calls returns a copy of all recorded invocations for inspection.
func (m *MockRunner) Calls() [][]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([][]string, len(m.calls))
	for i, c := range m.calls {
		cp := make([]string, len(c))
		copy(cp, c)
		out[i] = cp
	}
	return out
}

// sliceEqual reports whether two string slices are identical.
func sliceEqual(a, b []string) bool {
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

// formatArgs formats a string slice as a quoted command for error messages.
func formatArgs(args []string) string {
	return fmt.Sprintf("[%s]", strings.Join(args, " "))
}
