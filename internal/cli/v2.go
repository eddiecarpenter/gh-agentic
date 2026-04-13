// Package cli defines the cobra command tree for gh-agentic.
// v2.go registers the v2 subcommand group (mount, init, auth, doctor).
package cli

import (
	"fmt"
	"io"

	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// v2DeprecatedCommands lists v1 commands that are not available in v2 mode.
var v2DeprecatedCommands = []string{
	"sync",
	"verify",
	"bootstrap",
	"inception",
}

// errV2NotAvailable returns a formatted error for commands unavailable in v2 mode.
func errV2NotAvailable(cmdName string) error {
	return fmt.Errorf("'%s' is not available in v2 mode", cmdName)
}

// isV2Mode checks whether the --v2 flag is set on the root command.
// Returns false if the flag is not registered or not set.
func isV2Mode(cmd interface{ Root() interface{ Flags() interface{ GetBool(string) (bool, error) } } }) bool {
	// This is implemented via cobra's persistent flags — see checkV2Guard.
	return false
}

// checkV2Guard returns an error if the --v2 flag is set on the root command.
// Used by v1 commands to block execution in v2 mode.
func checkV2Guard(cmdName string, v2Flag *bool) error {
	if v2Flag != nil && *v2Flag {
		return errV2NotAvailable(cmdName)
	}
	return nil
}

// deprecationNotices maps deprecated v1 commands to their v2 replacement message.
var deprecationNotices = map[string]string{
	"sync":               "Deprecated: use 'gh agentic --v2 mount' instead.",
	"bootstrap":          "Deprecated: use 'gh agentic --v2 init' instead.",
	"inception":          "Deprecated: use 'gh agentic --v2 init' instead.",
	"update-credentials": "Deprecated: use 'gh agentic --v2 auth refresh' instead.",
}

// printDeprecationNotice prints a deprecation warning to the given writer.
// The notice is styled using ui.RenderWarning for visual consistency.
// In production, pass cmd.ErrOrStderr() to print to stderr.
func printDeprecationNotice(w io.Writer, command string) {
	msg, ok := deprecationNotices[command]
	if !ok {
		return
	}
	fmt.Fprintln(w, ui.RenderWarning(msg))
	fmt.Fprintln(w)
}
