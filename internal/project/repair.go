package project

import (
	"fmt"
	"io"

	"github.com/eddiecarpenter/gh-agentic/internal/mount"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// Repair runs all project health checks and interactively fixes what it can.
//
// Currently fixable:
//   - Framework not mounted → mounts the latest available version.
func Repair(w io.Writer, deps Deps) error {
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  gh agentic — project repair")
	fmt.Fprintln(w, "")

	report := RunChecks(deps)
	if report.FailCount() == 0 {
		fmt.Fprintf(w, "  %s  No issues found — nothing to repair\n\n", ui.StatusOK.Render("✓"))
		return nil
	}

	var repaired, unrepaired int

	for _, result := range report.Results {
		if result.Status != CheckFail {
			continue
		}

		switch result.Name {
		case "framework":
			fmt.Fprintf(w, "  Repairing: %s\n", result.Message)
			if err := repairFramework(w, deps); err != nil {
				fmt.Fprintf(w, "  %s  Could not repair framework: %v\n", ui.StatusDanger.Render("✗"), err)
				unrepaired++
			} else {
				repaired++
			}

		default:
			fmt.Fprintf(w, "  %s  %s — %s\n",
				ui.StatusDanger.Render("✗"),
				result.Message,
				ui.Muted.Render("cannot auto-repair: "+result.Remediation),
			)
			unrepaired++
		}
	}

	fmt.Fprintln(w, "")
	if unrepaired > 0 {
		fmt.Fprintf(w, "  %s\n\n", ui.StatusWarning.Render(fmt.Sprintf("%d issue(s) repaired, %d require manual attention", repaired, unrepaired)))
	} else {
		fmt.Fprintf(w, "  %s\n\n", ui.StatusOK.Render(fmt.Sprintf("%d issue(s) repaired", repaired)))
	}
	return nil
}

// repairFramework mounts the latest framework version into .ai/.
func repairFramework(w io.Writer, deps Deps) error {
	releases, err := deps.FetchReleases(mount.FrameworkRepo)
	if err != nil {
		return fmt.Errorf("fetching framework releases: %w", err)
	}
	if len(releases) == 0 {
		return fmt.Errorf("no framework releases available")
	}
	latest := releases[0].TagName

	fmt.Fprintf(w, "  Mounting framework %s...\n", latest)
	if err := mount.DownloadFramework(deps.Root, latest, deps.Clone); err != nil {
		return fmt.Errorf("mounting framework: %w", err)
	}
	fmt.Fprintf(w, "  %s  Framework mounted at %s\n", ui.StatusOK.Render("✓"), latest)
	return nil
}
