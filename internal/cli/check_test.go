package cli

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/eddiecarpenter/gh-agentic/internal/doctor"
	"github.com/eddiecarpenter/gh-agentic/internal/project"
)

// checkSectionHeadings lists the section heading names that renderCheckSections
// can emit. The title "gh agentic — check" is excluded — it is a report title,
// not a section, and does not carry the no-empty-content contract.
var checkSectionHeadings = []string{
	"Project",
	"Pipeline",
	"Repository",
	"Framework",
	"Framework source",
	"Workflows",
	"Variables & secrets",
	"Project reachability",
	"Skills",
	"Agentic project membership",
	"Agent files",
	"Shadow values",
}

// ansiEscapeRe matches ANSI SGR escape sequences produced by lipgloss styling.
var ansiEscapeRe = regexp.MustCompile(`\x1b\[[0-9;]*[mK]`)

// stripANSI removes ANSI escape sequences from s, returning plain text.
func stripANSI(s string) string {
	return ansiEscapeRe.ReplaceAllString(s, "")
}

// isDividerLine reports whether line (after ANSI stripping and trimming) is a
// divider row — a run of box-drawing dashes (─, U+2500) produced by ui.Divider.
func isDividerLine(line string) bool {
	trimmed := strings.TrimSpace(stripANSI(line))
	if trimmed == "" {
		return false
	}
	for _, r := range trimmed {
		if r != '─' {
			return false
		}
	}
	return true
}

// isKnownHeadingLine returns (headingName, true) when line (after ANSI stripping
// and trimming) exactly equals one of the known section heading names.
func isKnownHeadingLine(line string) (string, bool) {
	trimmed := strings.TrimSpace(stripANSI(line))
	for _, h := range checkSectionHeadings {
		if trimmed == h {
			return h, true
		}
	}
	return "", false
}

// assertNoEmptyHeadings is the no-empty-heading invariant verifier. It scans the
// rendered output for known section headings and fails if any heading has no body
// content (a non-empty, non-divider line) before the next heading or end-of-output.
// This is the regression contract for Feature #715 / AC-3.
func assertNoEmptyHeadings(t *testing.T, output string) {
	t.Helper()
	lines := strings.Split(output, "\n")

	currentHeading := ""
	headingLineIdx := 0
	hasBody := false

	// finalise checks whether the previous heading (if any) satisfied the invariant.
	finalise := func(nextIdx int) {
		if currentHeading == "" {
			return
		}
		if !hasBody {
			t.Errorf("no-empty-heading invariant violated: heading %q (line %d) has no body content before line %d",
				currentHeading, headingLineIdx+1, nextIdx+1)
		}
	}

	for i, line := range lines {
		if h, ok := isKnownHeadingLine(line); ok {
			finalise(i)
			currentHeading = h
			headingLineIdx = i
			hasBody = false
			continue
		}
		if currentHeading == "" {
			continue
		}
		stripped := strings.TrimSpace(stripANSI(line))
		if stripped == "" || isDividerLine(line) {
			// Empty lines and dividers do not count as body content.
			continue
		}
		hasBody = true
	}
	finalise(len(lines))
}

// makePassGroup returns a doctor.Group with a single Pass result, suitable for
// constructing representative pipeline reports in tests.
func makePassGroup(name, message string) doctor.Group {
	return doctor.Group{
		Name: name,
		Results: []doctor.CheckResult{
			{Name: "test-result", Status: doctor.Pass, Message: message},
		},
	}
}

// TestRenderCheckSections_FullPipeline verifies that a full-pipeline render
// (non-framework-source, all groups populated, pipeline not skipped) satisfies
// the no-empty-heading invariant and does not emit a bare "Pipeline" parent
// heading above the sub-group headings.
//
// AC-1 regression: every section heading in the rendered output has at least one
// status line beneath it before the next heading or end-of-report.
func TestRenderCheckSections_FullPipeline(t *testing.T) {
	projectReport := &project.CheckReport{
		Results: []project.CheckResult{
			{Name: "project-id", Status: project.CheckPass, Message: "AGENTIC_PROJECT_ID configured"},
		},
	}
	pipelineReport := &doctor.Report{
		Groups: []doctor.Group{
			makePassGroup("Repository", "Git repository (owner/repo)"),
			makePassGroup("Workflows", "agentic-pipeline.yml present"),
			makePassGroup("Variables & secrets", "AGENT_USER configured"),
			makePassGroup("Project reachability", "Project reachable: My Project"),
		},
	}

	var buf bytes.Buffer
	renderCheckSections(&buf, false, projectReport, pipelineReport, false)
	output := buf.String()

	// The bare "Pipeline" parent heading must NOT appear in the non-skipped path.
	for _, line := range strings.Split(output, "\n") {
		if h, ok := isKnownHeadingLine(line); ok && h == "Pipeline" {
			t.Errorf("bare 'Pipeline' parent heading must not appear in full-pipeline output; found in:\n%s", output)
		}
	}

	// Every section heading that does appear must have body content beneath it.
	assertNoEmptyHeadings(t, output)

	// "Project" heading must be present (non-nil projectReport).
	if !strings.Contains(output, "Project") {
		t.Errorf("expected 'Project' section in full-pipeline output:\n%s", output)
	}
}

// TestRenderCheckSections_FrameworkSourceMode verifies that framework-source
// mode (this repo IS gh-agentic) renders sub-group headings with content, emits
// the framework-source notice, and does not emit a bare "Pipeline" parent heading.
//
// AC-2 regression: in framework-source mode the rendered output either omits
// the empty section or shows an explicit ⚠ skipped line beneath the heading.
func TestRenderCheckSections_FrameworkSourceMode(t *testing.T) {
	// projectReport is nil in framework-source mode (project-scope checks are skipped).
	pipelineReport := &doctor.Report{
		Groups: []doctor.Group{
			makePassGroup("Repository", "Git repository (owner/repo)"),
			makePassGroup("Framework source", "skipped: framework source (.ai is a symlink)"),
			makePassGroup("Workflows", "agentic-pipeline.yml present"),
			makePassGroup("Variables & secrets", "AGENT_USER configured"),
			makePassGroup("Project reachability", "AGENTIC_PROJECT_ID not configured"),
		},
	}

	var buf bytes.Buffer
	renderCheckSections(&buf, true, nil, pipelineReport, false)
	output := buf.String()

	// The bare "Pipeline" parent heading must NOT appear in the non-skipped path.
	for _, line := range strings.Split(output, "\n") {
		if h, ok := isKnownHeadingLine(line); ok && h == "Pipeline" {
			t.Errorf("bare 'Pipeline' parent heading must not appear in framework-source output; found in:\n%s", output)
		}
	}

	// Every section heading that does appear must have body content.
	assertNoEmptyHeadings(t, output)

	// Framework source notice must appear before the sub-groups.
	if !strings.Contains(output, "Framework source detected") {
		t.Errorf("expected framework source notice in output:\n%s", output)
	}
}

// TestRenderCheckSections_PipelineSkipped verifies that when the pipeline checks
// are skipped (framework out of sync), the "Pipeline" heading IS rendered and is
// followed by the ⚠ Skipped status line — so the heading is never empty.
//
// AC-3 regression contract: no heading appears with zero entries that could be
// misclassified as a pre-existing warning by a downstream autonomous session.
func TestRenderCheckSections_PipelineSkipped(t *testing.T) {
	projectReport := &project.CheckReport{
		Results: []project.CheckResult{
			// Fail status on framework-version-sync is the signal that triggers
			// pipelineSkipped=true in the real command's RunE.
			{Name: "framework-version-sync", Status: project.CheckFail, Message: "framework version out of sync"},
		},
	}

	var buf bytes.Buffer
	// pipelineReport is nil when checks are skipped.
	renderCheckSections(&buf, false, projectReport, nil, true)
	output := buf.String()

	// "Pipeline" heading MUST appear in the skipped path.
	foundPipeline := false
	for _, line := range strings.Split(output, "\n") {
		if h, ok := isKnownHeadingLine(line); ok && h == "Pipeline" {
			foundPipeline = true
		}
	}
	if !foundPipeline {
		t.Errorf("expected 'Pipeline' heading in pipeline-skipped output:\n%s", output)
	}

	// The ⚠ "Skipped" status line must follow the Pipeline heading.
	if !strings.Contains(output, "Skipped") {
		t.Errorf("expected 'Skipped' line in pipeline-skipped output:\n%s", output)
	}

	// The "Pipeline" heading must not be empty — the ⚠ line counts as body content.
	assertNoEmptyHeadings(t, output)
}
