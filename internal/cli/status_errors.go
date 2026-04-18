package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/projectstatus"
)

// renderStatusError is the centralised error-to-message renderer for every
// `gh agentic status` sub-command. It inspects the error type via the typed
// sentinels / struct errors defined in the projectstatus package and prints
// a concrete, user-facing message on the command's stderr.
//
// It always returns ErrSilent so main.go exits non-zero without printing the
// raw error on top of the formatted message. No stack traces leak.
//
// The suggestCommand argument is used for ErrWrongType to tell the user
// which command to try instead — e.g. "gh agentic status feature". The
// function builds the full suggested invocation using the error's Number.
func renderStatusError(cmd *cobra.Command, err error) error {
	if err == nil {
		return nil
	}
	w := cmd.ErrOrStderr()

	if errors.Is(err, projectstatus.ErrProjectNotConfigured) {
		renderProjectNotConfigured(w)
		return ErrSilent
	}
	if errors.Is(err, projectstatus.ErrProjectUnreachable) {
		renderProjectUnreachable(w, err)
		return ErrSilent
	}
	if errors.Is(err, projectstatus.ErrIssueNotFound) {
		// The wrap from annotateDetailError already carries the repo/number
		// text verbatim ("issue #N not found in owner/repo"); surface it
		// as-is with the Error: prefix.
		fmt.Fprintln(w, "Error: "+stripIssueNotFoundWrap(err))
		return ErrSilent
	}

	var wt *projectstatus.ErrWrongType
	if errors.As(err, &wt) {
		renderWrongType(w, wt)
		return ErrSilent
	}

	// Classify network / auth / rate-limit / 5xx via the shared helper.
	switch projectstatus.ClassifyAPIError(err) {
	case projectstatus.APIErrorNetwork:
		fmt.Fprintln(w, "Error: cannot reach GitHub — check your network connection.")
		return ErrSilent
	case projectstatus.APIErrorAuth:
		fmt.Fprintln(w, "Error: GitHub authentication failed. Run: gh auth status")
		return ErrSilent
	case projectstatus.APIErrorRateLimit:
		fmt.Fprintln(w, "Error: GitHub API rate limit exceeded. Retry in a few minutes.")
		return ErrSilent
	case projectstatus.APIErrorServer:
		fmt.Fprintln(w, "Error: GitHub returned a server error (5xx). This is likely transient — retry in a moment.")
		return ErrSilent
	}

	// --horizontal narrow terminal — the kanban renderer raises a plain
	// fmt.Errorf; recognise its prefix and pass through the message
	// unchanged so the user sees the concrete width mismatch.
	if strings.HasPrefix(err.Error(), "--horizontal requires at least") {
		fmt.Fprintln(w, "Error: "+err.Error())
		return ErrSilent
	}

	// Fallback — unclassified error. Still surface as "Error: <msg>" with no
	// stack trace; the underlying message is informational.
	fmt.Fprintln(w, "Error: "+err.Error())
	return ErrSilent
}

// renderProjectNotConfigured writes the "AGENTIC_PROJECT_ID not set" block.
func renderProjectNotConfigured(w io.Writer) {
	fmt.Fprintln(w, "Error: AGENTIC_PROJECT_ID is not set for this repository.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "To fix:")
	fmt.Fprintln(w, "  gh agentic info        # show current configuration")
	fmt.Fprintln(w, "  gh variable set AGENTIC_PROJECT_ID --body <project-node-id>")
}

// renderProjectUnreachable writes the "project not reachable" block.
// err is surfaced inline for context; the message is formatted so the user
// sees concrete causes to investigate.
func renderProjectUnreachable(w io.Writer, err error) {
	id := extractProjectIDFromErr(err)
	if id != "" {
		fmt.Fprintf(w, "Error: agentic project %s is not reachable.\n", id)
	} else {
		fmt.Fprintln(w, "Error: agentic project is not reachable.")
	}
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Possible causes:")
	fmt.Fprintln(w, "  - project was deleted")
	fmt.Fprintln(w, "  - your token has lost access")
	fmt.Fprintln(w, "  - network is unreachable")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Check: gh auth status")
}

// renderWrongType writes the "#N is a <actual>, not a <wanted>" line and a
// "Try: gh agentic status <other> <N>" suggestion.
func renderWrongType(w io.Writer, wt *projectstatus.ErrWrongType) {
	fmt.Fprintf(w, "Error: #%d is a %s, not a %s.\n", wt.Number, wt.ActualType, wt.WantedType)
	fmt.Fprintf(w, "Try: gh agentic status %s %d\n", wt.ActualType, wt.Number)
}

// stripIssueNotFoundWrap removes the trailing ": issue not found" that
// errors.Is appends when annotateDetailError wraps with %w — the caller has
// already printed the concrete message up front and we do not want the
// sentinel suffix leaking into user-facing text.
func stripIssueNotFoundWrap(err error) string {
	msg := err.Error()
	const suffix = ": issue not found"
	if strings.HasSuffix(msg, suffix) {
		return msg[:len(msg)-len(suffix)]
	}
	return msg
}

// extractProjectIDFromErr looks for a PVT_* node ID substring in the error
// message — the underlying queries embed the node ID in their wrapped error
// text. Best-effort only; when absent, the renderer uses a generic message.
func extractProjectIDFromErr(err error) string {
	msg := err.Error()
	idx := strings.Index(msg, "PVT_")
	if idx == -1 {
		return ""
	}
	out := strings.Builder{}
	for _, r := range msg[idx:] {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			out.WriteRune(r)
			continue
		}
		break
	}
	return out.String()
}
