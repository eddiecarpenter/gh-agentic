package cli

import (
	"strings"
	"testing"

	"github.com/eddiecarpenter/gh-agentic/internal/project"
)

// TestPrintInfo_NoFederation_NoFederationSection verifies that when
// FEDERATION.md is absent (no federation repos, no error), printInfo does
// not emit a "Federation" section at all.
func TestPrintInfo_NoFederation_NoFederationSection(t *testing.T) {
	data := &infoData{
		version:   "v2.0.0",
		repoLabel: "acme/repo",
		topology:  "Single",
		// federationRepos and federationError are zero-value
	}
	out := printInfoToString(data)
	if strings.Contains(out, "Federation") {
		t.Errorf("expected no 'Federation' section when FEDERATION.md absent, got:\n%s", out)
	}
}

// TestPrintInfo_FederationRepos_ShowsSection verifies that when federation
// repos are populated, printInfo renders a "Federation" section with each
// repo name and purpose listed.
func TestPrintInfo_FederationRepos_ShowsSection(t *testing.T) {
	data := &infoData{
		version:   "v2.0.0",
		repoLabel: "acme/cp",
		topology:  "Federation",
		federationRepos: []project.FederationRepo{
			{Name: "acme/domain-a", Purpose: "Primary domain"},
			{Name: "acme/domain-b", Purpose: "Analytics domain"},
		},
	}
	out := printInfoToString(data)
	if !strings.Contains(out, "Federation") {
		t.Errorf("expected 'Federation' section heading, got:\n%s", out)
	}
	if !strings.Contains(out, "acme/domain-a") {
		t.Errorf("expected first repo name 'acme/domain-a' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Primary domain") {
		t.Errorf("expected first repo purpose 'Primary domain' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "acme/domain-b") {
		t.Errorf("expected second repo name 'acme/domain-b' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Analytics domain") {
		t.Errorf("expected second repo purpose 'Analytics domain' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Target repositories") {
		t.Errorf("expected 'Target repositories' label, got:\n%s", out)
	}
}

// TestPrintInfo_FederationError_ShowsWarning verifies that when FEDERATION.md
// is present but could not be parsed, printInfo renders the warning line with
// the exact error text.
func TestPrintInfo_FederationError_ShowsWarning(t *testing.T) {
	data := &infoData{
		version:         "v2.0.0",
		repoLabel:       "acme/cp",
		topology:        "Federation",
		federationError: "FEDERATION.md: repos list is empty",
	}
	out := printInfoToString(data)
	if !strings.Contains(out, "Federation") {
		t.Errorf("expected 'Federation' section heading even on error, got:\n%s", out)
	}
	if !strings.Contains(out, "FEDERATION.md present but could not be parsed") {
		t.Errorf("expected warning phrase 'FEDERATION.md present but could not be parsed', got:\n%s", out)
	}
	if !strings.Contains(out, "repos list is empty") {
		t.Errorf("expected error text 'repos list is empty' in output, got:\n%s", out)
	}
}

// TestPrintInfo_TopologyCapitalised verifies that the topology display field
// is shown capitalised ("Single" / "Federation"), not in the raw lowercase
// internal form ("single" / "federation").
func TestPrintInfo_TopologyCapitalised(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{
		{"Single", "Single"},
		{"Federation", "Federation"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			data := &infoData{
				version:   "v2.0.0",
				repoLabel: "owner/repo",
				topology:  tc.raw,
			}
			out := printInfoToString(data)
			if !strings.Contains(out, "Topology:") {
				t.Errorf("expected 'Topology:' label, got:\n%s", out)
			}
			if !strings.Contains(out, tc.want) {
				t.Errorf("expected topology display value %q in output, got:\n%s", tc.want, out)
			}
		})
	}
}

// TestPrintInfo_NoControlPlane verifies that the old "Control plane:" row is
// no longer emitted by printInfo (Feature #824 removed it).
func TestPrintInfo_NoControlPlane(t *testing.T) {
	data := &infoData{
		version:   "v2.0.0",
		repoLabel: "acme/cp",
		topology:  "Federation",
		federationRepos: []project.FederationRepo{
			{Name: "acme/domain", Purpose: "Domain repo"},
		},
	}
	out := printInfoToString(data)
	if strings.Contains(out, "Control plane:") {
		t.Errorf("'Control plane:' row must not appear in output after Feature #824, got:\n%s", out)
	}
}
