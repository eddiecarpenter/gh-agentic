package ui

import (
	"fmt"
	"io"
	"os"

	"github.com/cli/go-gh/v2/pkg/browser"
)

// OpenURLFunc is the injectable function used by OpenURL. Tests replace it
// with a recorder so the production go-gh browser launcher is never
// invoked. Callers should invoke OpenURL, not this variable directly —
// keeping the indirection in one place makes substitutions explicit.
var OpenURLFunc = openURL

// OpenURL opens the given URL in the user's default browser. The actual
// launch is delegated to OpenURLFunc which production code wires to the
// go-gh browser helper and tests replace with a capturing fake.
//
// OpenURL returns an error if the URL cannot be launched; callers on a
// headless path should check ui.IsCI before invoking OpenURL so that the
// UX degrades gracefully to "print the URL" rather than "fail opaquely".
func OpenURL(url string) error {
	if url == "" {
		return fmt.Errorf("url must be non-empty")
	}
	return OpenURLFunc(url)
}

// openURL is the production implementation — it constructs a new go-gh
// browser with stdout/stderr wired up, then asks it to launch the URL.
// The launcher respects the BROWSER environment variable via go-gh.
func openURL(url string) error {
	b := browser.New("", io.Discard, os.Stderr)
	return b.Browse(url)
}
