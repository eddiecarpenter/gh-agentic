package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
)

// busyDelay is the grace period before the busy indicator first appears.
// Operations that complete under this threshold never flash the spinner.
const busyDelay = 500 * time.Millisecond

// busyTickInterval drives the spinner frame animation once the indicator
// has been shown. Matches the cadence of RunWithSpinner so the visual feel
// is consistent across the two helpers.
const busyTickInterval = 80 * time.Millisecond

// BusyFunc is the signature for a function that runs fn() while displaying
// a busy indicator labelled with label on w. Tests may substitute a noop
// implementation via testutil.NoopBusy.
type BusyFunc func(w io.Writer, label string, fn func() error) error

// busySuppressed reports whether the busy indicator should be suppressed
// entirely for the given writer. The check is a package-level var so tests
// can substitute a deterministic fake.
//
// Suppression precedence (first matching rule wins):
//  1. NO_COLOR is set in the environment — industry convention; overrides
//     any TTY signal.
//  2. GH_NO_SPINNER is set in the environment — our dedicated opt-out so a
//     user can disable only the spinner without losing colour output.
//  3. w is not an *os.File, or is an *os.File whose file descriptor is not
//     attached to a terminal (piped, redirected, captured in a buffer).
//
// Callers treat a suppressed indicator as "run the function silently" —
// the busy glyph and label are never written.
var busySuppressed = func(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return true
	}
	if os.Getenv("GH_NO_SPINNER") != "" {
		return true
	}
	f, ok := w.(*os.File)
	if !ok {
		return true
	}
	return !term.IsTerminal(int(f.Fd()))
}

// BusyRun runs fn() and, when running on a TTY, displays a single-line
// busy indicator on w after a 500ms grace period. When fn completes the
// indicator line is erased (`\r<spaces>\r`) so subsequent writes start at
// column 0 with no leftover glyphs.
//
// BusyRun is safe for concurrent use — concurrent invocations serialise
// on a shared mutex so their writes never interleave. (Current call sites
// are sequential; the lock is for robustness only.)
//
// Suppression precedence is documented on busySuppressed: NO_COLOR,
// GH_NO_SPINNER, and non-TTY writers all disable the indicator entirely.
// In those modes BusyRun reduces to calling fn and returning its error,
// writing zero bytes to w.
//
// The label should describe the in-flight operation, e.g.
// "Fetching requirements…". Keep it short — the line is not wrapped.
func BusyRun(w io.Writer, label string, fn func() error) error {
	if busySuppressed(w) {
		return fn()
	}

	// Shared mutex guards all writes to w for the lifetime of this call
	// and across concurrent invocations (busyMu is a package-level lock).
	// This keeps the spinner line, re-draws, and the final erase from
	// interleaving with any other BusyRun goroutine targeting the same
	// stream.
	done := make(chan error, 1)
	go func() { done <- fn() }()

	// Phase 1 — wait up to busyDelay for fn to complete. If it finishes
	// first, suppress the spinner entirely (fast path).
	select {
	case err := <-done:
		return err
	case <-time.After(busyDelay):
		// fall through — fn is still running, show the indicator.
	}

	// Phase 2 — render the spinner and tick frames until fn returns.
	ticker := time.NewTicker(busyTickInterval)
	defer ticker.Stop()

	// Print the first frame immediately on entering phase 2 so the user
	// sees the label without waiting another tick interval.
	frame := 0
	busyMu.Lock()
	busyWriteFrame(w, label, frame)
	frame++
	busyMu.Unlock()

	for {
		select {
		case err := <-done:
			busyMu.Lock()
			busyClear(w, label)
			busyMu.Unlock()
			return err
		case <-ticker.C:
			busyMu.Lock()
			busyWriteFrame(w, label, frame)
			frame++
			busyMu.Unlock()
		}
	}
}

// busyMu serialises writes to the same stream across concurrent BusyRun
// invocations so their frame redraws and clears never interleave.
var busyMu sync.Mutex

// busyWriteFrame prints one frame of the spinner at column 0. Caller must
// hold busyMu.
func busyWriteFrame(w io.Writer, label string, frame int) {
	glyph := spinnerFrames[frame%len(spinnerFrames)]
	fmt.Fprintf(w, "\r%s %s", URL.Render(glyph), label)
}

// busyClear overwrites the spinner line with whitespace and returns the
// cursor to column 0. Caller must hold busyMu.
func busyClear(w io.Writer, label string) {
	// +4 accounts for the glyph, the trailing space, and a small safety
	// margin so stray escape sequences left by the styled glyph are fully
	// overwritten.
	width := len(label) + 4
	fmt.Fprintf(w, "\r%s\r", strings.Repeat(" ", width))
}
