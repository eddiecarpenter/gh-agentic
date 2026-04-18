package cli

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/projectstatus"
)

// newCmdForErrorCapture builds a throwaway cobra.Command whose stderr is
// captured so each test can assert on the rendered message.
func newCmdForErrorCapture() (*cobra.Command, *bytes.Buffer) {
	cmd := &cobra.Command{}
	buf := &bytes.Buffer{}
	cmd.SetErr(buf)
	cmd.SetOut(&bytes.Buffer{})
	return cmd, buf
}

// TestRenderStatusError_NilInputReturnsNil verifies a nil err passes through.
func TestRenderStatusError_NilInputReturnsNil(t *testing.T) {
	cmd, _ := newCmdForErrorCapture()
	if err := renderStatusError(cmd, nil); err != nil {
		t.Errorf("renderStatusError(nil) = %v, want nil", err)
	}
}

// TestRenderStatusError_AllClasses is the table-driven guard: every error
// class must produce a concrete message with no stack-trace leak, and must
// return ErrSilent so main.go exits non-zero without printing the raw error.
func TestRenderStatusError_AllClasses(t *testing.T) {
	cases := []struct {
		name      string
		err       error
		mustMatch []string
	}{
		{
			name:      "project not configured",
			err:       projectstatus.ErrProjectNotConfigured,
			mustMatch: []string{"AGENTIC_PROJECT_ID is not set", "gh variable set AGENTIC_PROJECT_ID"},
		},
		{
			name:      "project unreachable with node id",
			err:       fmt.Errorf("failed: %w (id=PVT_kwABC)", projectstatus.ErrProjectUnreachable),
			mustMatch: []string{"is not reachable", "PVT_kwABC", "gh auth status"},
		},
		{
			name:      "issue not found",
			err:       fmt.Errorf("issue #9999 not found in eddiecarpenter/gh-agentic: %w", projectstatus.ErrIssueNotFound),
			mustMatch: []string{"issue #9999 not found in eddiecarpenter/gh-agentic"},
		},
		{
			name:      "wrong type feature->requirement",
			err:       &projectstatus.ErrWrongType{Number: 492, ActualType: "feature", WantedType: "requirement"},
			mustMatch: []string{"#492 is a feature, not a requirement", "Try: gh agentic status feature 492"},
		},
		{
			name:      "wrong type requirement->feature",
			err:       &projectstatus.ErrWrongType{Number: 457, ActualType: "requirement", WantedType: "feature"},
			mustMatch: []string{"#457 is a requirement, not a feature", "Try: gh agentic status requirement 457"},
		},
		{
			name:      "network error",
			err:       errors.New("dial tcp 1.2.3.4:443: connection refused"),
			mustMatch: []string{"cannot reach GitHub", "network connection"},
		},
		{
			name:      "auth 401",
			err:       &api.HTTPError{StatusCode: 401, Message: "Unauthorized"},
			mustMatch: []string{"authentication failed", "gh auth status"},
		},
		{
			name:      "rate limit",
			err:       &api.HTTPError{StatusCode: 429, Message: "API rate limit exceeded"},
			mustMatch: []string{"rate limit exceeded"},
		},
		{
			name:      "server 500",
			err:       &api.HTTPError{StatusCode: 500, Message: "Server Error"},
			mustMatch: []string{"server error", "retry in a moment"},
		},
		{
			name:      "horizontal narrow",
			err:       errors.New("--horizontal requires at least 120 columns. Current terminal: 80. Try without --horizontal for vertical kanban."),
			mustMatch: []string{"--horizontal requires at least 120 columns", "Current terminal: 80"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd, buf := newCmdForErrorCapture()
			got := renderStatusError(cmd, tc.err)
			if !errors.Is(got, ErrSilent) {
				t.Errorf("expected ErrSilent; got %v", got)
			}
			for _, token := range tc.mustMatch {
				if !strings.Contains(buf.String(), token) {
					t.Errorf("output missing %q; got:\n%s", token, buf.String())
				}
			}
			// No stack-trace leakage — these strings are common signals of a
			// Go panic in error output; they must not appear.
			for _, forbidden := range []string{"goroutine ", "runtime.gopark", ".go:"} {
				if strings.Contains(buf.String(), forbidden) {
					t.Errorf("stack-trace leakage (%q); output:\n%s", forbidden, buf.String())
				}
			}
		})
	}
}

// TestExtractProjectIDFromErr covers the node-ID extraction helper.
func TestExtractProjectIDFromErr(t *testing.T) {
	cases := []struct {
		in   error
		want string
	}{
		{errors.New("failed for PVT_kwABC: unreachable"), "PVT_kwABC"},
		{errors.New("no id here"), ""},
		{errors.New("PVT_xyz123 then stuff"), "PVT_xyz123"},
	}
	for _, tc := range cases {
		got := extractProjectIDFromErr(tc.in)
		if got != tc.want {
			t.Errorf("extractProjectIDFromErr(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestRenderStatusError_IntegratesWithRequirementsHandler verifies the full
// path from a handler error to rendered stderr. Uses ErrProjectNotConfigured
// via a statusDeps that returns an empty project ID.
func TestRenderStatusError_IntegratesWithRequirementsHandler(t *testing.T) {
	sd := statusDeps{
		currentRepo:      func() (string, error) { return "owner/repo", nil },
		resolveProjectID: func(string) (string, error) { return "", nil },
		psDeps:           projectstatus.Deps{},
	}
	cmd := newStatusRequirementsCmdWithDeps(sd)
	buf := &bytes.Buffer{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(buf)
	err := cmd.Execute()
	if !errors.Is(err, ErrSilent) {
		t.Errorf("expected ErrSilent; got %v", err)
	}
	if !strings.Contains(buf.String(), "AGENTIC_PROJECT_ID is not set") {
		t.Errorf("expected configured-error message; got:\n%s", buf.String())
	}
}

// TestRenderStatusError_UnclassifiedFalloverPreservesMessage verifies an
// error that does not match any typed class still prints with "Error:" and
// returns ErrSilent (no stack trace, no bare message prefix).
func TestRenderStatusError_UnclassifiedFallover(t *testing.T) {
	cmd, buf := newCmdForErrorCapture()
	got := renderStatusError(cmd, errors.New("something weird"))
	if !errors.Is(got, ErrSilent) {
		t.Errorf("expected ErrSilent; got %v", got)
	}
	if !strings.HasPrefix(strings.TrimSpace(buf.String()), "Error: something weird") {
		t.Errorf("expected 'Error: ...' prefix; got:\n%s", buf.String())
	}
}
