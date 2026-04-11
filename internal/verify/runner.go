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
	return RunVerifyWithPrompt(w, checks, repairFn, nil)
}

// RunVerifyWithPrompt is the full-featured entry point that supports an optional
// interactive prompt when no --repair flag is set but issues are found.
func RunVerifyWithPrompt(w io.Writer, checks []CheckFunc, repairFn RepairFunc, promptFn PromptFunc) error {
	if repairFn != nil {
		return runInlineRepair(w, checks, repairFn)
	}

	// ── No repair function: run all checks, print results ─────────────────
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

	// If a prompt function is available, offer to fix.
	if promptFn != nil {
		fmt.Fprintln(w)
		accepted, err := promptFn(fmt.Sprintf("Fix %d issue(s) now?", issues))
		if err != nil {
			return fmt.Errorf("prompt error: %w", err)
		}
		if accepted {
			return runSelectiveRepair(w, results, nil)
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
// the prompt-to-fix flow). It prints only the issue lines with suffixes.
// If defaultRepairFn is nil, it returns an error — caller must provide a
// repair function via the results' repair context or a separate mechanism.
func runSelectiveRepair(w io.Writer, results []CheckResult, repairFn RepairFunc) error {
	// Placeholder for Task #414 — prompt-to-fix flow.
	// This will be fully implemented when prompt support is added.
	return fmt.Errorf("selective repair not yet implemented")
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
