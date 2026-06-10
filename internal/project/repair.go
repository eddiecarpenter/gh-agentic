package project

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/eddiecarpenter/gh-agentic/internal/mount"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// RepairResult holds the outcome of a repair run.
type RepairResult struct {
	Lines              []string // output lines to display
	Repaired           int
	Unrepaired         int
	FrameworkOutOfSync bool // true when framework-version-sync check failed
}

// RepairWithProgress runs all checks and repairs failures, calling setLabel
// before each step. Returns a RepairResult with buffered output lines.
func RepairWithProgress(deps Deps, setLabel func(string)) RepairResult {
	result := RepairResult{}

	if setLabel != nil {
		setLabel("Running agentic project checks...")
	}
	report := RunChecksWithProgress(deps, setLabel)

	// Surface framework-out-of-sync so the CLI can short-circuit the
	// pipeline phase — running skill / workflow checks against a stale
	// `.agents/` produces noise and blocks the real remediation.
	for _, r := range report.Results {
		if r.Name == "framework-version-sync" && r.Status == CheckFail {
			result.FrameworkOutOfSync = true
			break
		}
	}

	// Nothing to do if there are no failures and no repairable warnings.
	hasRepairable := false
	for _, r := range report.Results {
		if r.Status == CheckFail || (r.Status == CheckWarn && r.Name == "topology-vars") {
			hasRepairable = true
			break
		}
	}
	if !hasRepairable {
		result.Lines = append(result.Lines, "")
		result.Lines = append(result.Lines, "  "+ui.StatusOK.Render("✓")+"  No agentic project issues found — nothing to repair")
		result.Lines = append(result.Lines, "")
		return result
	}

	result.Lines = append(result.Lines, "")

	for _, r := range report.Results {
		// Only attempt repair on failures, or warnings that are known-repairable.
		repairable := r.Status == CheckFail ||
			(r.Status == CheckWarn && r.Name == "topology-vars")
		if !repairable {
			continue
		}
		switch r.Name {
		case "framework":
			if setLabel != nil {
				setLabel("Repairing: framework mount...")
			}
			var buf bytes.Buffer
			if err := repairFramework(&buf, deps); err != nil {
				result.Lines = append(result.Lines, fmt.Sprintf("  %s  Could not repair framework: %v", ui.StatusDanger.Render("✗"), err))
				result.Unrepaired++
			} else {
				result.Lines = append(result.Lines, strings.TrimRight(buf.String(), "\n"))
				result.Repaired++
			}
		case "views":
			if setLabel != nil {
				setLabel("Repairing: agentic project views...")
			}
			var buf bytes.Buffer
			if err := repairViews(&buf, deps); err != nil {
				result.Lines = append(result.Lines, fmt.Sprintf("  %s  Could not repair views: %v", ui.StatusDanger.Render("✗"), err))
				result.Unrepaired++
			} else {
				result.Lines = append(result.Lines, strings.TrimRight(buf.String(), "\n"))
				result.Repaired++
			}
		case "status-options":
			if setLabel != nil {
				setLabel("Repairing: project status options...")
			}
			var buf bytes.Buffer
			if err := repairStatusOptions(&buf, deps); err != nil {
				result.Lines = append(result.Lines, fmt.Sprintf("  %s  Could not repair status options: %v", ui.StatusDanger.Render("✗"), err))
				result.Unrepaired++
			} else {
				result.Lines = append(result.Lines, strings.TrimRight(buf.String(), "\n"))
				result.Repaired++
			}
		case "topology-vars":
			if setLabel != nil {
				setLabel("Repairing: topology variables...")
			}
			var buf bytes.Buffer
			if err := repairTopologyVars(&buf, deps); err != nil {
				result.Lines = append(result.Lines, fmt.Sprintf("  %s  Could not repair topology variables: %v", ui.StatusDanger.Render("✗"), err))
				result.Unrepaired++
			} else {
				result.Lines = append(result.Lines, strings.TrimRight(buf.String(), "\n"))
				result.Repaired++
			}
		case "orphan-issues":
			if setLabel != nil {
				setLabel("Repairing: adding orphan issues to project...")
			}
			var buf bytes.Buffer
			if err := repairOrphanIssues(&buf, deps); err != nil {
				result.Lines = append(result.Lines, fmt.Sprintf("  %s  Could not add orphan issues to project: %v", ui.StatusDanger.Render("✗"), err))
				result.Unrepaired++
			} else {
				result.Lines = append(result.Lines, strings.TrimRight(buf.String(), "\n"))
				result.Repaired++
			}
		default:
			result.Lines = append(result.Lines, fmt.Sprintf("  %s  %s — %s",
				ui.StatusDanger.Render("✗"),
				r.Message,
				ui.Muted.Render("cannot auto-repair: "+r.Remediation),
			))
			result.Unrepaired++
		}
	}

	result.Lines = append(result.Lines, "")
	if result.Unrepaired > 0 {
		result.Lines = append(result.Lines, fmt.Sprintf("  %s", ui.StatusWarning.Render(fmt.Sprintf("%d issue(s) repaired, %d require manual attention", result.Repaired, result.Unrepaired))))
	} else {
		result.Lines = append(result.Lines, fmt.Sprintf("  %s", ui.StatusOK.Render(fmt.Sprintf("%d issue(s) repaired", result.Repaired))))
	}
	result.Lines = append(result.Lines, "")
	return result
}

// Repair is the original entry point — kept for backward compatibility.
func Repair(w io.Writer, deps Deps) error {
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  gh agentic — project repair")
	fmt.Fprintln(w, "")
	result := RepairWithProgress(deps, nil)
	for _, line := range result.Lines {
		fmt.Fprintln(w, line)
	}
	return nil
}

// repairViews recreates any template views that are missing from the project.
func repairViews(w io.Writer, deps Deps) error {
	projectID, err := deps.GetRepoVariable(deps.Owner, deps.RepoName, ProjectVarName)
	if err != nil || projectID == "" {
		return fmt.Errorf("AGENTIC_PROJECT_ID not set")
	}

	tpl, err := ReadProjectTemplate()
	if err != nil {
		return fmt.Errorf("reading project template: %w", err)
	}

	existing, err := deps.FetchProjectViews(projectID)
	if err != nil {
		return fmt.Errorf("fetching existing views: %w", err)
	}
	existingNames := make(map[string]bool, len(existing))
	for _, v := range existing {
		existingNames[v.Name] = true
	}

	ownerType, err := deps.DetectOwnerType(deps.Owner)
	if err != nil {
		ownerType = "User"
	}

	projectNumber, err := deps.FetchProjectNumber(projectID)
	if err != nil {
		return fmt.Errorf("fetching project number: %w", err)
	}

	var created []string
	for _, v := range tpl.Views {
		if existingNames[v.Name] {
			continue
		}
		layout := strings.ToLower(strings.TrimSuffix(v.Layout, "_LAYOUT"))
		if err := deps.CreateProjectView(deps.Owner, ownerType, projectNumber, v.Name, layout, v.Filter); err != nil {
			fmt.Fprintf(w, "  %s  Could not create view %q: %v\n", ui.StatusWarning.Render("⚠"), v.Name, err)
		} else {
			fmt.Fprintf(w, "  %s  View %q created\n", ui.StatusOK.Render("✓"), v.Name)
			created = append(created, v.Name)
		}
	}

	if len(created) == 0 {
		return fmt.Errorf("no views could be created")
	}
	return nil
}

// repairTopologyVars ensures topology-related variables are correct on the
// current repo, deducing topology via the canonical project.Resolve. The
// rule is: each repo's repair fixes its own state only. No cross-repo
// writes; when a federated-domain repo detects broken control-plane
// state, it terminates with a pointed "run repair on the CP" error.
//
// Per-topology behaviour:
//
//   - single          → no topology-var writes required
//   - federated-cp    → write AGENTIC_FRAMEWORK_VERSION to the latest
//     release when missing
//   - federated-domain → no local writes; if the CP's
//     AGENTIC_FRAMEWORK_VERSION is missing, return a
//     hard-stop error pointing at the CP repo
func repairTopologyVars(w io.Writer, deps Deps) error {
	ctx, err := Resolve(deps)
	if err != nil {
		return fmt.Errorf("resolving topology: %w", err)
	}
	if ctx.ProjectID == "" {
		return fmt.Errorf(ProjectVarName + " not set — cannot repair topology")
	}

	switch ctx.Topology {
	case TopologyStringSingle:
		// Single repo: topology is derived from FEDERATION.md absence,
		// so no variable writes are required. Report cleanly.
		fmt.Fprintf(w, "  %s  topology=single — no variable writes required\n", ui.StatusOK.Render("✓"))
		return nil

	case TopologyStringFederation:
		// Federation topology is declared by FEDERATION.md presence; no
		// AGENTIC_TOPOLOGY variable writes are required.
		fmt.Fprintf(w, "  %s  topology=federation — FEDERATION.md present, no variable writes required\n", ui.StatusOK.Render("✓"))
		return nil
	}

	return fmt.Errorf("unrecognised topology %q", ctx.Topology)
}

// repairStatusOptions syncs the project's Status single-select field options
// to match the full set defined in the project template. Uses a replace-all
// approach so that both missing options and stale ordering are corrected in
// a single mutation.
func repairStatusOptions(w io.Writer, deps Deps) error {
	projectID, err := deps.GetRepoVariable(deps.Owner, deps.RepoName, ProjectVarName)
	if err != nil || projectID == "" {
		return fmt.Errorf(ProjectVarName + " not set")
	}

	tpl, err := ReadProjectTemplate()
	if err != nil {
		return fmt.Errorf("reading project template: %w", err)
	}

	fields, err := deps.FetchProjectFields(projectID)
	if err != nil {
		return fmt.Errorf("fetching project fields: %w", err)
	}

	var statusFieldID string
	for _, f := range fields {
		if f.Name == "Status" {
			statusFieldID = f.ID
			break
		}
	}
	if statusFieldID == "" {
		return fmt.Errorf("Status field not found on project")
	}

	if err := deps.UpdateStatusFieldOptions(statusFieldID, tpl.StatusField.Options); err != nil {
		return fmt.Errorf("updating status field options: %w", err)
	}

	fmt.Fprintf(w, "  %s  Project status options synced (%d options)\n",
		ui.StatusOK.Render("✓"), len(tpl.StatusField.Options))
	return nil
}

// repairFramework mounts the latest framework version into .agents/.
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

// repairOrphanIssues adds every open issue carrying a type label
// (feature / requirement / task) that is missing from the agentic
// project. Idempotent — the `addProjectV2ItemById` mutation returns
// the existing item ID for an already-member issue.
func repairOrphanIssues(w io.Writer, deps Deps) error {
	projectID, err := deps.GetRepoVariable(deps.Owner, deps.RepoName, ProjectVarName)
	if err != nil || projectID == "" {
		return fmt.Errorf("AGENTIC_PROJECT_ID not set")
	}
	if deps.FetchOrphanIssues == nil {
		return fmt.Errorf("no FetchOrphanIssues function configured")
	}
	if deps.AddIssueToProject == nil {
		return fmt.Errorf("no AddIssueToProject function configured")
	}

	orphans, err := deps.FetchOrphanIssues(deps.Owner, deps.RepoName, projectID)
	if err != nil {
		return fmt.Errorf("fetching orphan issues: %w", err)
	}
	if len(orphans) == 0 {
		fmt.Fprintf(w, "  %s  No orphan issues to add\n", ui.StatusOK.Render("✓"))
		return nil
	}

	var failures []string
	added := 0
	for _, o := range orphans {
		if err := deps.AddIssueToProject(projectID, o.NodeID); err != nil {
			failures = append(failures, fmt.Sprintf("#%d (%v)", o.Number, err))
			continue
		}
		fmt.Fprintf(w, "  %s  Added #%d to project: %s\n",
			ui.StatusOK.Render("✓"), o.Number, o.Title)
		added++
	}
	if len(failures) > 0 {
		return fmt.Errorf("added %d, failed %d: %s",
			added, len(failures), strings.Join(failures, "; "))
	}
	fmt.Fprintf(w, "  %s  %d orphan issue(s) added to project\n",
		ui.StatusOK.Render("✓"), added)
	return nil
}
