package verify

import (
	"fmt"
	"io"

	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// RunVerify runs all checks, prints the full result list, then (if repairFn is
// set) runs repairs with clear before/after output and reprints the final state.
// Returns an error if any unresolved warnings or failures remain after repair.
// ManualAction results are displayed with instructions but do not cause failure.
func RunVerify(w io.Writer, checks []CheckFunc, repairFn RepairFunc) error {
	// ── Phase 1: run all checks, print each result immediately ──────────────
	results := make([]CheckResult, len(checks))
	for i, fn := range checks {
		results[i] = fn()
		printResult(w, results[i])
	}

	// Count actionable issues (not ManualAction — those need human steps, not repair).
	var warnings, failures int
	for _, r := range results {
		switch r.Status {
		case Warning:
			warnings++
		case Fail:
			failures++
		case Pass, ManualAction:
			// not counted as actionable issues
		}
	}

	// If nothing to repair, print summary and return.
	if repairFn == nil || (warnings == 0 && failures == 0) {
		fmt.Fprintln(w)
		if warnings == 0 && failures == 0 {
			fmt.Fprintln(w, "  "+ui.RenderOK("All checks passed"))
			return nil
		}
		printSummary(w, results, 0)
		return fmt.Errorf("%d warnings, %d failures remain", warnings, failures)
	}

	// ── Phase 2: repair ───────────────────────────────────────────────────────
	issueCount := warnings + failures
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %s\n", ui.Muted.Render(fmt.Sprintf("Repairing %d issue(s)...", issueCount)))

	repaired := 0
	for i, r := range results {
		if r.Status == Pass {
			continue
		}
		fmt.Fprintln(w)
		fmt.Fprintln(w, "  "+ui.Muted.Render("↻ "+r.Name))

		updated := repairFn(r)
		if updated != nil && updated.Status == Pass {
			fmt.Fprintln(w, "  "+ui.RenderOK("fixed"))
			results[i] = *updated
			repaired++
		} else if updated != nil && updated.Status == ManualAction {
			fmt.Fprintln(w, "  "+ui.RenderInfo("action needed: "+updated.Message))
			results[i] = *updated
		} else {
			msg := r.Message
			if updated != nil && updated.Message != "" {
				msg = updated.Message
			}
			fmt.Fprintln(w, "  "+ui.RenderError("still failing: "+msg))
		}
	}

	// ── Final state ───────────────────────────────────────────────────────────
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  "+ui.Muted.Render("─── Final state ───"))
	fmt.Fprintln(w)
	printResults(w, results)
	fmt.Fprintln(w)

	// Recount after repairs — ManualAction is not a failure.
	warnings, failures = 0, 0
	for _, r := range results {
		switch r.Status {
		case Warning:
			warnings++
		case Fail:
			failures++
		case Pass, ManualAction:
			// not counted as actionable issues
		}
	}

	if warnings == 0 && failures == 0 {
		fmt.Fprintln(w, "  "+ui.RenderOK("All checks passed"))
		return nil
	}

	printSummary(w, results, repaired)
	return fmt.Errorf("%d warnings, %d failures remain", warnings, failures)
}

// printResult renders a single ✔/⚠/✖/ℹ line for one result.
func printResult(w io.Writer, r CheckResult) {
	switch r.Status {
	case Pass:
		fmt.Fprintln(w, "  "+ui.RenderOK(r.Name))
	case Warning:
		fmt.Fprintln(w, "  "+ui.RenderWarning(r.Name+": "+r.Message))
	case Fail:
		fmt.Fprintln(w, "  "+ui.RenderError(r.Name+": "+r.Message))
	case ManualAction:
		fmt.Fprintln(w, "  "+ui.RenderInfo(r.Name+": "+r.Message))
	}
}

// printResults renders the ✔/⚠/✖/ℹ line for each result.
func printResults(w io.Writer, results []CheckResult) {
	for _, r := range results {
		printResult(w, r)
	}
}

// printSummary prints the count line.
func printSummary(w io.Writer, results []CheckResult, repaired int) {
	var passed, warnings, failures, manual int
	for _, r := range results {
		switch r.Status {
		case Pass:
			passed++
		case Warning:
			warnings++
		case Fail:
			failures++
		case ManualAction:
			manual++
		}
	}

	summary := fmt.Sprintf("  %d passed", passed)
	if warnings > 0 {
		summary += fmt.Sprintf(", %d warnings", warnings)
	}
	if repaired > 0 {
		summary += fmt.Sprintf(", %d repaired", repaired)
	}
	if manual > 0 {
		summary += fmt.Sprintf(", %d action(s) needed", manual)
	}
	if failures > 0 {
		summary += fmt.Sprintf(", %d failed", failures)
	}
	fmt.Fprintln(w, summary)
}
