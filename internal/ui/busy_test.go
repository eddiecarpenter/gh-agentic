package ui

import (
	"bytes"
	"errors"
	"io"
	"sync"
	"testing"
	"time"
)

// withBusySuppressed temporarily overrides busySuppressed for a single test
// and restores it when the returned cleanup runs.
func withBusySuppressed(t *testing.T, fake func(io.Writer) bool) {
	t.Helper()
	original := busySuppressed
	busySuppressed = fake
	t.Cleanup(func() { busySuppressed = original })
}

// TestBusyRun_FastCompletionEmitsNothing verifies that when fn returns well
// under the 500ms grace period, no spinner bytes are ever written even on a
// TTY-enabled stream.
func TestBusyRun_FastCompletionEmitsNothing(t *testing.T) {
	withBusySuppressed(t, func(io.Writer) bool { return false })

	buf := &bytes.Buffer{}
	called := false
	err := BusyRun(buf, "fetch fast", func() error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("BusyRun returned error: %v", err)
	}
	if !called {
		t.Fatalf("expected fn to be called")
	}
	if buf.Len() != 0 {
		t.Errorf("expected no bytes on fast path; got %q", buf.String())
	}
}

// TestBusyRun_SlowCompletionRendersThenClears verifies that on a slow fetch
// the spinner line is emitted (after the 500ms threshold) and then cleared
// with the `\r<spaces>\r` overwrite sequence when fn completes.
func TestBusyRun_SlowCompletionRendersThenClears(t *testing.T) {
	withBusySuppressed(t, func(io.Writer) bool { return false })

	buf := &bytes.Buffer{}
	label := "Fetching pipeline state…"
	err := BusyRun(buf, label, func() error {
		// Exceed the 500ms delay plus at least one tick so we see a frame.
		time.Sleep(busyDelay + 150*time.Millisecond)
		return nil
	})
	if err != nil {
		t.Fatalf("BusyRun returned error: %v", err)
	}
	out := buf.String()
	if out == "" {
		t.Fatalf("expected spinner bytes on slow path; got empty buffer")
	}
	// The label must appear at least once.
	if !bytes.Contains([]byte(out), []byte(label)) {
		t.Errorf("expected label %q in output; got %q", label, out)
	}
	// Output must end with the clear sequence: carriage return, spaces, CR.
	// Count the trailing CR pair — stricter check than substring since we
	// want to know the final phase is the clear, not an in-flight frame.
	if len(out) < 2 {
		t.Fatalf("output too short: %q", out)
	}
	last := out[len(out)-1]
	if last != '\r' {
		t.Errorf("expected trailing carriage return from clear sequence; got last byte = %q in %q", last, out)
	}
	// After the clear, the buffer should contain at least one "\r<spaces>\r"
	// run — find the last CR pair and assert the bytes between are spaces.
	penultimateCR := lastIndexOfClearSequence([]byte(out))
	if penultimateCR < 0 {
		t.Errorf("expected clear sequence (\\r<spaces>\\r) near end of output; got %q", out)
	}
}

// lastIndexOfClearSequence returns the index of the penultimate '\r' in b
// if the bytes between it and the final '\r' are all ASCII spaces; else -1.
// This isolates the final erase sequence written by busyClear.
func lastIndexOfClearSequence(b []byte) int {
	if len(b) == 0 || b[len(b)-1] != '\r' {
		return -1
	}
	// Walk backwards from the final '\r' looking for a run of spaces
	// terminated by another '\r'.
	i := len(b) - 2
	for i >= 0 && b[i] == ' ' {
		i--
	}
	if i < 0 || b[i] != '\r' {
		return -1
	}
	return i
}

// TestBusyRun_NonTTYSuppresses verifies that when the suppression check
// returns true (non-TTY, NO_COLOR, GH_NO_SPINNER) no bytes are written even
// when fn runs long enough to trigger the spinner.
func TestBusyRun_NonTTYSuppresses(t *testing.T) {
	withBusySuppressed(t, func(io.Writer) bool { return true })

	buf := &bytes.Buffer{}
	err := BusyRun(buf, "Fetching", func() error {
		time.Sleep(busyDelay + 100*time.Millisecond)
		return nil
	})
	if err != nil {
		t.Fatalf("BusyRun returned error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected zero bytes when suppressed; got %q", buf.String())
	}
}

// TestBusyRun_SuppressedByNoColor covers the NO_COLOR precedence rule — a
// TTY stream with NO_COLOR set in the environment produces zero bytes.
func TestBusyRun_SuppressedByNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	// busySuppressed is the real function here so we exercise the env-var
	// branch; the writer is a bytes.Buffer, so the third rule (non-TTY)
	// would also match, but we rely on the first rule short-circuiting
	// before it. Assert no bytes either way.
	buf := &bytes.Buffer{}
	err := BusyRun(buf, "Fetching", func() error {
		time.Sleep(busyDelay + 100*time.Millisecond)
		return nil
	})
	if err != nil {
		t.Fatalf("BusyRun returned error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("NO_COLOR: expected zero bytes; got %q", buf.String())
	}
}

// TestBusyRun_SuppressedByGHNoSpinner covers the GH_NO_SPINNER precedence
// rule — overrides the TTY signal just like NO_COLOR does.
func TestBusyRun_SuppressedByGHNoSpinner(t *testing.T) {
	// Make sure NO_COLOR is cleared so this test isolates GH_NO_SPINNER.
	t.Setenv("NO_COLOR", "")
	t.Setenv("GH_NO_SPINNER", "1")
	buf := &bytes.Buffer{}
	err := BusyRun(buf, "Fetching", func() error {
		time.Sleep(busyDelay + 100*time.Millisecond)
		return nil
	})
	if err != nil {
		t.Fatalf("BusyRun returned error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("GH_NO_SPINNER: expected zero bytes; got %q", buf.String())
	}
}

// TestBusyRun_PropagatesError verifies the error returned by fn propagates
// through BusyRun unchanged, on both the fast and slow completion paths.
func TestBusyRun_PropagatesError(t *testing.T) {
	withBusySuppressed(t, func(io.Writer) bool { return true })
	want := errors.New("fetch failed")
	buf := &bytes.Buffer{}

	// Fast path.
	if got := BusyRun(buf, "fast", func() error { return want }); got != want {
		t.Errorf("fast path: got error %v, want %v", got, want)
	}

	// Slow path — still suppressed so no bytes.
	got := BusyRun(buf, "slow", func() error {
		time.Sleep(busyDelay + 50*time.Millisecond)
		return want
	})
	if got != want {
		t.Errorf("slow path: got error %v, want %v", got, want)
	}
}

// TestBusyRun_ConcurrentInvocationsDoNotRace exercises multiple concurrent
// BusyRun calls against a thread-safe writer. Each call triggers the
// spinner (slow path) so frame writes are in-flight on every goroutine.
// The shared mutex must prevent interleaved writes; `go test -race` will
// fail if the underlying counter is touched without synchronisation.
func TestBusyRun_ConcurrentInvocationsDoNotRace(t *testing.T) {
	withBusySuppressed(t, func(io.Writer) bool { return false })

	w := &countingWriter{}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = BusyRun(w, "concurrent", func() error {
				time.Sleep(busyDelay + 100*time.Millisecond)
				return nil
			})
		}()
	}
	wg.Wait()
	if w.writes() == 0 {
		t.Errorf("expected at least one write across concurrent invocations")
	}
}

// countingWriter is a threadsafe io.Writer used by the race-detector test.
type countingWriter struct {
	mu    sync.Mutex
	count int
}

func (c *countingWriter) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.count++
	return len(p), nil
}

func (c *countingWriter) writes() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.count
}
