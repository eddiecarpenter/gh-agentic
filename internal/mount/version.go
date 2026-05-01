package mount

import "strings"

// TrimVPrefix strips a leading "v" from a version string for display.
// Git tags and GitHub releases carry a "v" prefix (e.g. "v2.6.2"); we
// display versions without it for consistency with the CLI binary version
// that GoReleaser injects (e.g. "2.6.2").
func TrimVPrefix(v string) string {
	return strings.TrimPrefix(v, "v")
}
