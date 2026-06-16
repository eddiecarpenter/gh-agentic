package cli

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/eddiecarpenter/gh-agentic/internal/projectstatus"
)

func TestResolveFeatureTarget(t *testing.T) {
	cases := []struct {
		name        string
		field       string
		currentRepo string
		want        string
	}{
		{"field set — prepend CP owner", "charging-rating", "openbss/openbss", "openbss/charging-rating"},
		{"unset — single topology falls back to current repo", "", "acme/widgets", "acme/widgets"},
		{"blank field treated as unset", "   ", "acme/widgets", "acme/widgets"},
		{"field already owner/repo — used verbatim", "other/repo", "acme/cp", "other/repo"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolveFeatureTarget(c.field, c.currentRepo); got != c.want {
				t.Errorf("resolveFeatureTarget(%q, %q) = %q, want %q", c.field, c.currentRepo, got, c.want)
			}
		})
	}
}

// TestRunFeatureTarget_Raw exercises the command end-to-end against an injected
// feature with the Target repo field set, and one without (single topology).
func TestRunFeatureTarget_Raw(t *testing.T) {
	now := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name   string
		target string
		want   string
	}{
		{"federated", "charging-rating", "eddiecarpenter/charging-rating\n"},
		{"single topology", "", "eddiecarpenter/gh-agentic\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			issues := []projectstatus.ProjectIssue{
				{Number: 873, Title: "feat: x", Body: "b", Stage: projectstatus.StageInDevelopment, Type: "feature", State: "open", OwningRepo: "eddiecarpenter/gh-agentic", TargetRepo: tc.target, CreatedAt: now, LastTransitionedAt: now},
			}
			sd := featureDetailFixture(issues, nil, nil, nil, nil)

			buf := &bytes.Buffer{}
			if err := runFeatureTarget(buf, io.Discard, 873, true, sd); err != nil {
				t.Fatalf("runFeatureTarget: %v", err)
			}
			if buf.String() != tc.want {
				t.Errorf("--raw output = %q, want %q", buf.String(), tc.want)
			}
		})
	}
}

// TestRunFeatureTarget_HumanForm verifies the non-raw output names the feature
// and its target.
func TestRunFeatureTarget_HumanForm(t *testing.T) {
	now := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	issues := []projectstatus.ProjectIssue{
		{Number: 873, Title: "feat: x", Body: "b", Stage: projectstatus.StageInDevelopment, Type: "feature", State: "open", OwningRepo: "eddiecarpenter/gh-agentic", TargetRepo: "charging-rating", CreatedAt: now, LastTransitionedAt: now},
	}
	sd := featureDetailFixture(issues, nil, nil, nil, nil)

	buf := &bytes.Buffer{}
	if err := runFeatureTarget(buf, io.Discard, 873, false, sd); err != nil {
		t.Fatalf("runFeatureTarget: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "873") || !strings.Contains(out, "eddiecarpenter/charging-rating") {
		t.Errorf("human output should name the feature and target, got: %q", out)
	}
}
