package testutil

import "io"

// NoopSpinner satisfies the SpinnerFunc signature used in the bootstrap, sync,
// and inception packages. It simply calls fn() and returns its result without
// any TTY rendering, making it suitable for use in tests.
func NoopSpinner(_ io.Writer, _ string, fn func() error) error {
	return fn()
}

// NoopDynamicSpinner satisfies the DynamicSpinnerFunc signature. It calls fn
// with a no-op setLabel callback, making it suitable for use in tests.
func NoopDynamicSpinner(_ io.Writer, _ string, fn func(setLabel func(string)) error) error {
	return fn(func(string) {})
}

// NoopBusy satisfies the ui.BusyFunc signature used by the status commands
// to wrap long-running fetches in a busy indicator. It calls fn() and
// returns its result without writing any spinner bytes, making it suitable
// for use in tests that need a deterministic, silent wrapper.
func NoopBusy(_ io.Writer, _ string, fn func() error) error {
	return fn()
}
