package testutil

import "io"

// NoopSpinner satisfies the SpinnerFunc signature used in the bootstrap, sync,
// and inception packages. It simply calls fn() and returns its result without
// any TTY rendering, making it suitable for use in tests.
func NoopSpinner(_ io.Writer, _ string, fn func() error) error {
	return fn()
}
