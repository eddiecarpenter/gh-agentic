package ui

import (
	"io"
	"os"

	"golang.org/x/term"
)

// IsInteractive reports whether w is a terminal — specifically an *os.File
// whose file descriptor is attached to a TTY. It returns false for pipes,
// buffers, and any non-file writer.
//
// IsInteractive does NOT consult NO_COLOR or GH_NO_SPINNER — those are
// UI-suppression concerns orthogonal to TTY detection. Callers that care
// about colour or spinner opt-outs must check those environment variables
// themselves; see busySuppressed for the composed check used by BusyRun.
func IsInteractive(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// IsCI reports whether the process is running inside a CI environment.
// The detection is deliberately narrow — only GITHUB_ACTIONS=true and the
// widely-honoured CI=true are recognised — to avoid false positives from
// tools that set unrelated variables. Callers use IsCI to pick the
// headless install-flow path that prints a URL instead of opening a
// browser.
func IsCI() bool {
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		return true
	}
	if os.Getenv("CI") == "true" {
		return true
	}
	return false
}
