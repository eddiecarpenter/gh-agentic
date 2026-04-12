// Package doctorv2 implements the v2 doctor health check with grouped output.
// Each check returns a CheckResult with pass/warning/fail status and optional
// remediation message.
package doctorv2

import (
	"fmt"
	"io"
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

// Render writes the grouped report to the writer.
func (r *Report) Render(w io.Writer) {
	fmt.Fprintln(w, "AI-Native Delivery Framework — Health Check")
	fmt.Fprintln(w)

	for _, g := range r.Groups {
		fmt.Fprintf(w, "  %s\n", g.Name)
		for _, c := range g.Results {
			switch c.Status {
			case Pass:
				fmt.Fprintf(w, "  ✓ %s\n", c.Message)
			case Warning:
				fmt.Fprintf(w, "  ⚠ %s\n", c.Message)
			case Fail:
				fmt.Fprintf(w, "  ✗ %s\n", c.Message)
				if c.Remediation != "" {
					fmt.Fprintf(w, "    → %s\n", c.Remediation)
				}
			}
		}
		fmt.Fprintln(w)
	}

	// Summary.
	warnings := r.WarningCount()
	failures := r.FailCount()

	if failures > 0 {
		fmt.Fprintf(w, "%d failure(s)", failures)
		if warnings > 0 {
			fmt.Fprintf(w, ", %d warning(s)", warnings)
		}
		fmt.Fprintln(w, " — run 'gh agentic -v2 doctor --help' for remediation steps.")
	} else if warnings > 0 {
		fmt.Fprintf(w, "%d warning(s) — run 'gh agentic -v2 doctor --help' for remediation steps.\n", warnings)
	}
}
