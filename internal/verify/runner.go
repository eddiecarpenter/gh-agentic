package verify

import (
	"fmt"
	"io"

	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// PromptFunc is a callback that asks the user a yes/no question and returns
// the answer. It allows the runner to prompt without importing CLI dependencies.
type PromptFunc func(prompt string) (bool, error)

// RunVerify runs all checks and, when repairFn is set, performs inline repair
// in a single pass — each check line is printed once with a coloured suffix
// showing the outcome. When repairFn is nil and issues are found, promptFn
// (if non-nil) is called to offer interactive repair.
//
// Returns an error if any unresolved warnings or failures remain after repair.
// ManualAction results are displayed with instructions but do not cause failure.
func RunVerify(w io.Writer, checks []CheckFunc, repairFn RepairFunc) error {
	return RunVerifyWithPrompt(w, checks, VerifyOptions{RepairFn: repairFn})
}

// VerifyOptions configures the RunVerifyWithPrompt behaviour.
type VerifyOptions struct {
	// RepairFn is the repair function used for inline repair (--repair mode).
	// When set, checks and repairs happen in a single pass with suffixes.
	RepairFn RepairFunc

	// PromptFn is called when no RepairFn is set and issues are found.
	// It asks the user whether to fix issues interactively.
	PromptFn PromptFunc

	// PromptRepairFn is the repair function used when the user accepts the
	// prompt. It is separate from RepairFn to allow different wiring:
	// --repair uses RepairFn (inline), prompt uses PromptRepairFn (selective).
	PromptRepairFn RepairFunc
}

// RunVerifyWithPrompt is the full-featured entry point that supports inline
// repair (--repair) and interactive prompt-to-fix (no --repair, issues found).
func RunVerifyWithPrompt(w io.Writer, checks []CheckFunc, opts VerifyOptions) error {
	if opts.RepairFn != nil {
		return runInlineRepair(w, checks, opts.RepairFn)
	}

	// ── No inline repair: run all checks, print results ──────────────────
	results := make([]CheckResult, len(checks))
	for i, fn := range checks {
		results[i] = fn()
		printResult(w, results[i])
	}

	// Count actionable issues.
	issues := countIssues(results)

	if issues == 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "  "+ui.RenderOK("All checks passed"))
		return nil
	}

	// If a prompt function and repair function are available, offer to fix.
	if opts.PromptFn != nil && opts.PromptRepairFn != nil {
		fmt.Fprintln(w)
		accepted, err := opts.PromptFn(fmt.Sprintf("Fix %d issue(s) now?", issues))
		if err != nil {
			return fmt.Errorf("prompt error: %w", err)
		}
		if accepted {
			return runSelectiveRepair(w, results, opts.PromptRepairFn)
		}
	}

	// No repair — print summary and return error.
	fmt.Fprintln(w)
	printSummary(w, results)
	return fmt.Errorf("%d issue(s) remain", issues)
}

// runInlineRepair implements the single-pass check-and-repair flow used when
// --repair is set. Each check line is printed exactly once with a suffix.
func runInlineRepair(w io.Writer, checks []CheckFunc, repairFn RepairFunc) error {
	var okCount, fixedCount, failingCount, actionCount int

	results := make([]CheckResult, len(checks))
	for i, fn := range checks {
		r := fn()
		results[i] = r

		switch r.Status {
		case Pass:
			printResultWithSuffix(w, r, ui.Muted.Render("→  ok"))
			okCount++
		case ManualAction:
			printResultWithSuffix(w, r, ui.StatusWarning.Render("→  action needed"))
			actionCount++
		case Warning, Fail:
			// Attempt repair.
			updated := repairFn(r)
			if updated != nil && updated.Status == Pass {
				printResultWithSuffix(w, r, ui.StatusOK.Render("→  fixed"))
				results[i] = *updated
				fixedCount++
			} else if updated != nil && updated.Status == ManualAction {
				printResultWithSuffix(w, r, ui.StatusWarning.Render("→  action needed"))
				results[i] = *updated
				actionCount++
			} else {
				printResultWithSuffix(w, r, ui.StatusDanger.Render("→  still failing"))
				if updated != nil {
					results[i] = *updated
				}
				failingCount++
			}
		}
	}

	fmt.Fprintln(w)

	// If everything is ok (possibly with fixes), report success.
	if failingCount == 0 && actionCount == 0 {
		if fixedCount == 0 {
			fmt.Fprintln(w, "  "+ui.RenderOK("All checks passed"))
			return nil
		}
	}

	// Print inline summary.
	printInlineSummary(w, okCount, fixedCount, failingCount, actionCount)

	if failingCount > 0 {
		return fmt.Errorf("%d issue(s) still failing", failingCount)
	}
	return nil
}

// runSelectiveRepair runs repair only on failing/warning results (used by
// the prompt-to-fix flow). Only issue lines are printed with coloured suffixes;
// passing lines are hidden. A filtered summary is printed after repair.
func runSelectiveRepair(w io.Writer, results []CheckResult, repairFn RepairFunc) error {
	var fixedCount, failingCount, actionCount int

	fmt.Fprintln(w)
	for _, r := range results {
		if r.Status == Pass {
			continue // Hide passing lines.
		}
		if r.Status == ManualAction {
			printResultWithSuffix(w, r, ui.StatusWarning.Render("→  action needed"))
			actionCount++
			continue
		}

		// Warning or Fail — attempt repair.
		updated := repairFn(r)
		if updated != nil && updated.Status == Pass {
			printResultWithSuffix(w, r, ui.StatusOK.Render("→  fixed"))
			fixedCount++
		} else if updated != nil && updated.Status == ManualAction {
			printResultWithSuffix(w, r, ui.StatusWarning.Render("→  action needed"))
			actionCount++
		} else {
			printResultWithSuffix(w, r, ui.StatusDanger.Render("→  still failing"))
			failingCount++
		}
	}

	fmt.Fprintln(w)

	// Print filtered summary (only repaired/failing counts).
	printInlineSummary(w, 0, fixedCount, failingCount, actionCount)

	if failingCount > 0 {
		return fmt.Errorf("%d issue(s) still failing", failingCount)
	}
	return nil
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

// printResultWithSuffix renders a check line with a right-aligned coloured suffix.
func printResultWithSuffix(w io.Writer, r CheckResult, suffix string) {
	var base string
	switch r.Status {
	case Pass:
		base = "  " + ui.RenderOK(r.Name)
	case Warning:
		base = "  " + ui.RenderWarning(r.Name+": "+r.Message)
	case Fail:
		base = "  " + ui.RenderError(r.Name+": "+r.Message)
	case ManualAction:
		base = "  " + ui.RenderInfo(r.Name+": "+r.Message)
	}
	fmt.Fprintf(w, "%s  %s\n", base, suffix)
}

// printResults renders the ✔/⚠/✖/ℹ line for each result.
func printResults(w io.Writer, results []CheckResult) {
	for _, r := range results {
		printResult(w, r)
	}
}

// printSummary prints the count line in the legacy format (no-repair mode).
func printSummary(w io.Writer, results []CheckResult) {
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
	if manual > 0 {
		summary += fmt.Sprintf(", %d action(s) needed", manual)
	}
	if failures > 0 {
		summary += fmt.Sprintf(", %d failed", failures)
	}
	fmt.Fprintln(w, summary)
}

// printInlineSummary prints the new-format summary for inline repair mode.
func printInlineSummary(w io.Writer, ok, fixed, failing, action int) {
	summary := fmt.Sprintf("  %d ok", ok)
	if fixed > 0 {
		summary += fmt.Sprintf(", %d fixed", fixed)
	}
	if failing > 0 {
		summary += fmt.Sprintf(", %d still failing", failing)
	}
	if action > 0 {
		summary += fmt.Sprintf(", %d action needed", action)
	}
	fmt.Fprintln(w, summary)
}

// countIssues returns the number of warnings and failures in the results.
func countIssues(results []CheckResult) int {
	var count int
	for _, r := range results {
		if r.Status == Warning || r.Status == Fail {
			count++
		}
	}
	return count
}
