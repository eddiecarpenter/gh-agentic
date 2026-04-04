package verify

import (
	"fmt"
	"io"

	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// RunVerify executes all check functions, renders ✔/⚠/✖ per check, applies
// the repair function to non-passing checks, and prints a summary line.
// Returns an error if any unresolved warnings or failures remain after repair.
func RunVerify(w io.Writer, checks []CheckFunc, repairFn RepairFunc) error {
	var passed, warnings, repaired, failures int

	for _, check := range checks {
		result := check()

		switch result.Status {
		case Pass:
			fmt.Fprintln(w, "  "+ui.RenderOK(result.Name))
			passed++

		case Warning:
			fmt.Fprintln(w, "  "+ui.RenderWarning(result.Name+": "+result.Message))
			if repairFn != nil {
				updated := repairFn(result)
				if updated != nil && updated.Status == Pass {
					fmt.Fprintln(w, "    "+ui.RenderOK("repaired"))
					repaired++
				} else {
					warnings++
				}
			} else {
				warnings++
			}

		case Fail:
			fmt.Fprintln(w, "  "+ui.RenderError(result.Name+": "+result.Message))
			if repairFn != nil {
				updated := repairFn(result)
				if updated != nil && updated.Status == Pass {
					fmt.Fprintln(w, "    "+ui.RenderOK("repaired"))
					repaired++
				} else {
					failures++
				}
			} else {
				failures++
			}
		}
	}

	// Print summary.
	fmt.Fprintln(w)
	if warnings == 0 && failures == 0 {
		fmt.Fprintln(w, "  "+ui.RenderOK("All checks passed"))
		return nil
	}

	summary := fmt.Sprintf("  %d passed", passed)
	if warnings > 0 {
		summary += fmt.Sprintf(", %d warnings", warnings)
	}
	if repairFn != nil && repaired > 0 {
		summary += fmt.Sprintf(", %d repaired", repaired)
	}
	if failures > 0 {
		summary += fmt.Sprintf(", %d failed", failures)
	}
	fmt.Fprintln(w, summary)

	return fmt.Errorf("%d warnings, %d failures remain", warnings, failures)
}
