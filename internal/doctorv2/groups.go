// Package doctorv2 implements the v2 doctor health check with grouped output.
// Each check returns a CheckResult with pass/warning/fail status and optional
// remediation message.
package doctorv2

import (
	"fmt"
	"io"

	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// Status represents the outcome of a health check.
type Status int

const (
	// Pass indicates the check passed successfully.
	Pass Status = iota
	// Warning indicates a non-critical issue (exit 0).
	Warning
	// Fail indicates a critical issue (exit 1).
	Fail
)

// CheckResult holds the result of a single health check.
type CheckResult struct {
	Name        string
	Status      Status
	Message     string
	Remediation string // Optional command to fix the issue.
	// Data optionally carries structured output for downstream consumers.
	// checkShadowVars attaches the list of shadow values here so the
	// repair pipeline can consume them without re-querying. Most checks
	// leave this nil.
	Data any
}

// Group holds a named group of check results.
type Group struct {
	Name    string
	Results []CheckResult
}

// Report holds all check groups and provides summary methods.
type Report struct {
	Groups []Group
}

// HasFailures returns true if any check has Fail status.
func (r *Report) HasFailures() bool {
	for _, g := range r.Groups {
		for _, c := range g.Results {
			if c.Status == Fail {
				return true
			}
		}
	}
	return false
}

// HasWarnings returns true if any check has Warning status.
func (r *Report) HasWarnings() bool {
	for _, g := range r.Groups {
		for _, c := range g.Results {
			if c.Status == Warning {
				return true
			}
		}
	}
	return false
}

// WarningCount returns the number of warnings in the report.
func (r *Report) WarningCount() int {
	count := 0
	for _, g := range r.Groups {
		for _, c := range g.Results {
			if c.Status == Warning {
				count++
			}
		}
	}
	return count
}

// FailCount returns the number of failures in the report.
func (r *Report) FailCount() int {
	count := 0
	for _, g := range r.Groups {
		for _, c := range g.Results {
			if c.Status == Fail {
				count++
			}
		}
	}
	return count
}

// RenderHeader writes the report heading with topology context.
func RenderHeader(w io.Writer, topology string) {
	fmt.Fprintln(w, ui.SectionHeading.Render("  gh agentic — doctor"))
	fmt.Fprintln(w)

	switch topology {
	case "single":
		fmt.Fprintln(w, "  "+ui.Muted.Render("Mode: Single agentic project (control plane + code repo)"))
	case "federated-cp":
		fmt.Fprintln(w, "  "+ui.Muted.Render("Mode: Federated agentic project — control plane"))
	case "federated-domain":
		fmt.Fprintln(w, "  "+ui.Muted.Render("Mode: Federated agentic project — domain repo"))
	default:
		fmt.Fprintln(w, "  "+ui.Muted.Render("Mode: Unknown — not part of an agentic project"))
	}
	fmt.Fprintln(w)
}

// RenderGroup writes a single group's results immediately.
func RenderGroup(w io.Writer, g Group) {
	fmt.Fprintln(w, "  "+ui.SectionHeading.Render(g.Name))
	fmt.Fprintln(w, "  "+ui.Divider(48))
	for _, c := range g.Results {
		switch c.Status {
		case Pass:
			fmt.Fprintf(w, "  %s %s\n", ui.StatusOK.Render("✓"), c.Message)
		case Warning:
			fmt.Fprintf(w, "  %s %s\n", ui.StatusWarning.Render("⚠"), c.Message)
		case Fail:
			fmt.Fprintf(w, "  %s %s\n", ui.StatusDanger.Render("✗"), c.Message)
			if c.Remediation != "" {
				fmt.Fprintf(w, "    %s %s\n", ui.Muted.Render("→"), c.Remediation)
			}
		}
	}
	fmt.Fprintln(w)
}

// RenderSummary writes the failure/warning summary line.
func RenderSummary(w io.Writer, failures, warnings int) {
	if failures > 0 {
		fmt.Fprintf(w, "  %d failure(s)", failures)
		if warnings > 0 {
			fmt.Fprintf(w, ", %d warning(s)", warnings)
		}
		fmt.Fprintln(w, " — see remediation steps above.")
	} else if warnings > 0 {
		fmt.Fprintf(w, "  %d warning(s)\n", warnings)
	} else {
		fmt.Fprintf(w, "  %s\n", ui.StatusOK.Render("All checks passed"))
	}
	fmt.Fprintln(w)
}

// Render writes the full grouped report to the writer (used in tests).
func (r *Report) Render(w io.Writer) {
	RenderHeader(w, "")
	for _, g := range r.Groups {
		RenderGroup(w, g)
	}
	RenderSummary(w, r.FailCount(), r.WarningCount())
}
