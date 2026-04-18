package doctorv2

import (
	"fmt"

	"github.com/eddiecarpenter/gh-agentic/internal/mount"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// RepairResult holds the outcome of a pipeline-side repair run.
type RepairResult struct {
	Lines      []string
	Repaired   int
	Unrepaired int
}

// workflowRepairNames is the set of CheckResult names produced by checkWorkflows
// that map to a workflow-tag mismatch (auto-fixable via UpdateWorkflowVersions).
var workflowRepairNames = map[string]bool{
	"agentic-pipeline.yml": true,
	"release.yml":          true,
}

// RepairPipeline runs all pipeline checks and attempts to auto-repair the
// failures that are mechanically fixable. Failures that require human input
// (missing variables, missing secrets) are surfaced with their remediation
// hint but counted as Unrepaired.
func RepairPipeline(deps CheckDeps, setLabel func(string)) RepairResult {
	result := RepairResult{}

	if setLabel != nil {
		setLabel("Running pipeline checks...")
	}
	report := RunAllChecksWithProgress(deps, setLabel)

	workflowRepairAttempted := false

	for _, g := range report.Groups {
		for _, r := range g.Results {
			if r.Status != Fail {
				continue
			}

			switch {
			case r.Name == "gitignore":
				if setLabel != nil {
					setLabel("Repairing: .gitignore...")
				}
				if err := mount.EnsureGitignore(deps.Root); err != nil {
					result.Lines = append(result.Lines,
						fmt.Sprintf("  %s  Could not add .ai/ to .gitignore: %v",
							ui.StatusDanger.Render("✗"), err))
					result.Unrepaired++
				} else {
					result.Lines = append(result.Lines,
						fmt.Sprintf("  %s  .ai/ added to .gitignore",
							ui.StatusOK.Render("✓")))
					result.Repaired++
				}

			case workflowRepairNames[r.Name]:
				// Multiple workflow files may all be out of date; one rewrite
				// pass fixes them all, so dedupe.
				if workflowRepairAttempted {
					continue
				}
				workflowRepairAttempted = true
				if setLabel != nil {
					setLabel("Repairing: workflow version tags...")
				}
				version, verr := mount.ReadAIVersionFromGit(deps.Root)
				if verr != nil || version == "" {
					result.Lines = append(result.Lines,
						fmt.Sprintf("  %s  Could not repair workflow versions: framework version unknown",
							ui.StatusDanger.Render("✗")))
					result.Unrepaired++
					continue
				}
				if err := mount.UpdateWorkflowVersions(deps.Root, version); err != nil {
					result.Lines = append(result.Lines,
						fmt.Sprintf("  %s  Could not update workflow versions: %v",
							ui.StatusDanger.Render("✗"), err))
					result.Unrepaired++
				} else {
					result.Lines = append(result.Lines,
						fmt.Sprintf("  %s  Workflow version tags updated to %s",
							ui.StatusOK.Render("✓"), version))
					result.Repaired++
				}

			default:
				// Not auto-repairable — surface so the user sees what is left.
				line := fmt.Sprintf("  %s  %s",
					ui.StatusDanger.Render("✗"), r.Message)
				if r.Remediation != "" {
					line += " " + ui.Muted.Render("→ "+r.Remediation)
				}
				result.Lines = append(result.Lines, line)
				result.Unrepaired++
			}
		}
	}

	return result
}
