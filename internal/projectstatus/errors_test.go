package projectstatus

import (
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
)

// TestErrIssueNotFound_IsWithErrorsIs verifies the sentinel is detected.
func TestErrIssueNotFound_IsWithErrorsIs(t *testing.T) {
	wrapped := fmt.Errorf("wrapping: %w", ErrIssueNotFound)
	if !errors.Is(wrapped, ErrIssueNotFound) {
		t.Errorf("errors.Is did not match ErrIssueNotFound through a wrap")
	}
}

// TestErrProjectNotConfigured_IsWithErrorsIs verifies the sentinel is detected.
func TestErrProjectNotConfigured_IsWithErrorsIs(t *testing.T) {
	wrapped := fmt.Errorf("wrapping: %w", ErrProjectNotConfigured)
	if !errors.Is(wrapped, ErrProjectNotConfigured) {
		t.Errorf("errors.Is did not match ErrProjectNotConfigured through a wrap")
	}
}

// TestErrProjectUnreachable_IsWithErrorsIs verifies the sentinel is detected.
func TestErrProjectUnreachable_IsWithErrorsIs(t *testing.T) {
	wrapped := fmt.Errorf("wrapping: %w", ErrProjectUnreachable)
	if !errors.Is(wrapped, ErrProjectUnreachable) {
		t.Errorf("errors.Is did not match ErrProjectUnreachable through a wrap")
	}
}

// TestErrWrongType_AsAndIs verifies the typed error can be inspected via
// errors.As to read the Number/ActualType/WantedType fields, and detected via
// errors.Is for coarse matching.
func TestErrWrongType_AsAndIs(t *testing.T) {
	orig := &ErrWrongType{Number: 483, ActualType: "feature", WantedType: "requirement"}
	wrapped := fmt.Errorf("wrapping: %w", orig)

	var target *ErrWrongType
	if !errors.As(wrapped, &target) {
		t.Fatalf("errors.As failed to extract *ErrWrongType")
	}
	if target.Number != 483 || target.ActualType != "feature" || target.WantedType != "requirement" {
		t.Errorf("extracted fields wrong: %+v", target)
	}

	if !errors.Is(wrapped, &ErrWrongType{}) {
		t.Errorf("errors.Is(*ErrWrongType{}) did not match through wrap")
	}
}

// TestClassifyAPIError_Network covers net.Error and string-heuristic network
// failures.
func TestClassifyAPIError_Network(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "net.OpError DNS", err: &net.OpError{Op: "dial", Err: fmt.Errorf("no such host")}},
		{name: "net.DNSError", err: &net.DNSError{Err: "no such host"}},
		{name: "raw dial tcp", err: errors.New("dial tcp 1.2.3.4:443: connection refused")},
		{name: "raw i/o timeout", err: errors.New("context deadline exceeded: i/o timeout")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			class := ClassifyAPIError(tc.err)
			if class != APIErrorNetwork {
				t.Errorf("ClassifyAPIError(%q) = %q, want %q", tc.err, class, APIErrorNetwork)
			}
		})
	}
}

// TestClassifyAPIError_Auth covers 401, 403, and GraphQL unauthorized.
func TestClassifyAPIError_Auth(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{name: "http 401", err: &api.HTTPError{StatusCode: 401, Message: "Unauthorized"}},
		{name: "http 403", err: &api.HTTPError{StatusCode: 403, Message: "Forbidden"}},
		{name: "gql forbidden", err: &api.GraphQLError{Errors: []api.GraphQLErrorItem{{Message: "forbidden: token is missing required scope"}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			class := ClassifyAPIError(tc.err)
			if class != APIErrorAuth {
				t.Errorf("ClassifyAPIError(%q) = %q, want %q", tc.err, class, APIErrorAuth)
			}
		})
	}
}

// TestClassifyAPIError_RateLimit covers 429 and GraphQL rate-limit messages.
func TestClassifyAPIError_RateLimit(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{name: "http 429", err: &api.HTTPError{StatusCode: 429, Message: "API rate limit exceeded"}},
		{name: "gql rate limit message", err: &api.GraphQLError{Errors: []api.GraphQLErrorItem{{Message: "API rate limit exceeded for user"}}}},
		{name: "secondary rate limit string", err: errors.New("You have exceeded a secondary rate limit")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			class := ClassifyAPIError(tc.err)
			if class != APIErrorRateLimit {
				t.Errorf("ClassifyAPIError(%q) = %q, want %q", tc.err, class, APIErrorRateLimit)
			}
		})
	}
}

// TestClassifyAPIError_Server covers 5xx and GraphQL errors without a
// clearer signal.
func TestClassifyAPIError_Server(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{name: "http 500", err: &api.HTTPError{StatusCode: 500, Message: "Server error"}},
		{name: "http 502", err: &api.HTTPError{StatusCode: 502, Message: "Bad Gateway"}},
		{name: "http 504", err: &api.HTTPError{StatusCode: 504, Message: "Gateway Timeout"}},
		{name: "gql misc", err: &api.GraphQLError{Errors: []api.GraphQLErrorItem{{Message: "something went wrong"}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			class := ClassifyAPIError(tc.err)
			if class != APIErrorServer {
				t.Errorf("ClassifyAPIError(%q) = %q, want %q", tc.err, class, APIErrorServer)
			}
		})
	}
}

// TestClassifyAPIError_Nil returns APIErrorNone.
func TestClassifyAPIError_Nil(t *testing.T) {
	class := ClassifyAPIError(nil)
	if class != APIErrorNone {
		t.Errorf("ClassifyAPIError(nil) = %q, want %q", class, APIErrorNone)
	}
}

// TestClassifyAPIError_Unknown covers errors with no identifiable class.
func TestClassifyAPIError_Unknown(t *testing.T) {
	class := ClassifyAPIError(errors.New("something weird happened"))
	if class != APIErrorUnknown {
		t.Errorf("ClassifyAPIError(unknown) = %q, want %q", class, APIErrorUnknown)
	}
}

// netErrTimeout is a small helper type to satisfy the net.Error interface in
// tests where we want Timeout()==true.
type netErrTimeout struct{ msg string }

func (e netErrTimeout) Error() string   { return e.msg }
func (e netErrTimeout) Timeout() bool   { return true }
func (e netErrTimeout) Temporary() bool { return false }

// TestClassifyAPIError_NetErrInterface verifies a synthetic net.Error is
// correctly classified via errors.As — proves the interface check path works
// independently of string heuristics.
func TestClassifyAPIError_NetErrInterface(t *testing.T) {
	err := netErrTimeout{msg: "request failed"}
	class := ClassifyAPIError(err)
	if class != APIErrorNetwork {
		t.Errorf("ClassifyAPIError(netErr) = %q, want %q", class, APIErrorNetwork)
	}
	// Sanity check that the value is actually a net.Error — protects the
	// test against future changes that accidentally drop the interface.
	var netErr net.Error
	if !errors.As(err, &netErr) {
		t.Fatalf("test helper does not satisfy net.Error")
	}
	_ = time.Second
}
