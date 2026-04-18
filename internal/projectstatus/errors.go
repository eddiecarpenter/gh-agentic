package projectstatus

import (
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
)

// ErrProjectNotConfigured is returned when the repository has no
// AGENTIC_PROJECT_ID variable set. The CLI layer turns this into a clear
// "set AGENTIC_PROJECT_ID" message; see task #503 for the error renderer.
var ErrProjectNotConfigured = errors.New("AGENTIC_PROJECT_ID is not set for this repository")

// ErrProjectUnreachable is returned when the project node cannot be reached
// via GraphQL — the typical causes are a missing node ID, a permission error,
// or a network failure against GitHub.
var ErrProjectUnreachable = errors.New("agentic project is unreachable")

// ErrIssueNotFound is returned by detail queries when the referenced issue
// does not exist in any of the repos associated with the project.
var ErrIssueNotFound = errors.New("issue not found")

// ErrWrongType is returned by a detail query when the caller asked for one
// entity type (e.g. requirement) but the referenced issue carries a different
// type label (e.g. feature). The CLI layer uses ActualType and WantedType to
// render a helpful "did you mean `status feature <N>`?" message.
type ErrWrongType struct {
	Number     int
	ActualType string
	WantedType string
}

// Error implements error. The message names the mismatch plainly.
func (e *ErrWrongType) Error() string {
	return fmt.Sprintf("#%d is a %s, not a %s", e.Number, e.ActualType, e.WantedType)
}

// Is reports whether target is also an *ErrWrongType. This lets callers use
// errors.Is(err, &ErrWrongType{}) to detect the error class without caring
// about the specific values — errors.As remains available for the full
// struct.
func (e *ErrWrongType) Is(target error) bool {
	_, ok := target.(*ErrWrongType)
	return ok
}

// APIErrorClass is a coarse classification of a GitHub API error. It lets the
// CLI layer print a message that names the failure class, matching the
// acceptance criterion that distinguishes network / auth / rate-limit /
// server errors.
type APIErrorClass string

// Classification values returned by ClassifyAPIError.
const (
	APIErrorNone      APIErrorClass = ""
	APIErrorNetwork   APIErrorClass = "network"
	APIErrorAuth      APIErrorClass = "auth"
	APIErrorRateLimit APIErrorClass = "rate-limit"
	APIErrorServer    APIErrorClass = "server"
	APIErrorUnknown   APIErrorClass = "unknown"
)

// ClassifyAPIError inspects an error returned from the GitHub REST or GraphQL
// client and returns its coarse class. The logic is:
//
//   - nil → APIErrorNone
//   - net.Error (dial, timeout, DNS) → APIErrorNetwork
//   - *api.HTTPError with status 401 or 403 → APIErrorAuth
//   - *api.HTTPError with status 429, or any error message mentioning
//     "rate limit" → APIErrorRateLimit
//   - *api.HTTPError with status >= 500 → APIErrorServer
//   - *api.GraphQLError → APIErrorServer (unless the message indicates
//     rate-limit or auth, caught above)
//   - everything else → APIErrorUnknown
//
// The caller is expected to log / render the raw error separately for
// debuggability; this function only classifies.
func ClassifyAPIError(err error) APIErrorClass {
	if err == nil {
		return APIErrorNone
	}

	// Rate-limit detection via message string — works for both HTTPError and
	// GraphQL messages like "API rate limit exceeded".
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "rate limit") || strings.Contains(lower, "secondary rate") {
		return APIErrorRateLimit
	}

	var httpErr *api.HTTPError
	if errors.As(err, &httpErr) {
		switch {
		case httpErr.StatusCode == 401 || httpErr.StatusCode == 403:
			return APIErrorAuth
		case httpErr.StatusCode == 429:
			return APIErrorRateLimit
		case httpErr.StatusCode >= 500:
			return APIErrorServer
		}
	}

	var gqlErr *api.GraphQLError
	if errors.As(err, &gqlErr) {
		// GraphQL errors without a clearer signal are classified as server —
		// the GraphQL endpoint rejected the request for a reason the caller
		// cannot fix locally.
		for _, item := range gqlErr.Errors {
			lowerItem := strings.ToLower(item.Message)
			if strings.Contains(lowerItem, "unauthorized") || strings.Contains(lowerItem, "forbidden") {
				return APIErrorAuth
			}
			if strings.Contains(lowerItem, "not found") || strings.Contains(lowerItem, "could not resolve") {
				// A "not found" GraphQL error is a server-reported class; the
				// higher-level query converts it into ErrIssueNotFound as
				// appropriate. Here we just classify it.
				return APIErrorServer
			}
		}
		return APIErrorServer
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return APIErrorNetwork
	}

	// Fall back to string heuristics on common network failure modes. go-gh
	// sometimes wraps net errors in a way that defeats errors.As — this is
	// the safety net.
	if strings.Contains(lower, "no such host") ||
		strings.Contains(lower, "dial tcp") ||
		strings.Contains(lower, "connection refused") ||
		strings.Contains(lower, "i/o timeout") ||
		strings.Contains(lower, "network is unreachable") {
		return APIErrorNetwork
	}

	return APIErrorUnknown
}
