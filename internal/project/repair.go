package project

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/eddiecarpenter/gh-agentic/internal/mount"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// ErrTopologyAmbiguous is returned by repairTopologyVars when the topology
// cannot be determined automatically and requires user input.
var ErrTopologyAmbiguous = errors.New("topology ambiguous")

// RepairResult holds the outcome of a repair run.
type RepairResult struct {
	Lines               []string // output lines to display
	Repaired            int
	Unrepaired          int
	NeedsTopologyPrompt bool // true when topology couldn't be auto-determined
}

// RepairWithProgress runs all checks and repairs failures, calling setLabel
// before each step. Returns a RepairResult with buffered output lines.
func RepairWithProgress(deps Deps, setLabel func(string)) RepairResult {
	result := RepairResult{}

	if setLabel != nil {
		setLabel("Running agentic project checks...")
	}
	report := RunChecksWithProgress(deps, setLabel)

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
		case "topology-vars":
			if setLabel != nil {
				setLabel("Repairing: topology variables...")
			}
			var buf bytes.Buffer
			if err := repairTopologyVars(&buf, deps, ""); err != nil {
				if errors.Is(err, ErrTopologyAmbiguous) {
					result.NeedsTopologyPrompt = true
				} else {
					result.Lines = append(result.Lines, fmt.Sprintf("  %s  Could not repair topology variables: %v", ui.StatusDanger.Render("✗"), err))
					result.Unrepaired++
				}
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

// RepairTopologyWithChoice runs a targeted topology repair using an explicit topology
// value chosen by the user. Called after NeedsTopologyPrompt is returned by RepairWithProgress.
func RepairTopologyWithChoice(deps Deps, topology string) RepairResult {
	result := RepairResult{}
	var buf bytes.Buffer
	if err := repairTopologyVars(&buf, deps, topology); err != nil {
		result.Lines = append(result.Lines, fmt.Sprintf("  %s  Could not repair topology variables: %v", ui.StatusDanger.Render("✗"), err))
		result.Unrepaired++
	} else {
		result.Lines = append(result.Lines, strings.TrimRight(buf.String(), "\n"))
		result.Repaired++
	}
	return result
}

// repairTopologyVars ensures AGENTIC_TOPOLOGY and AGENTIC_FRAMEWORK_VERSION are set correctly.
// If topology is non-empty it is used directly (user-supplied). If empty, the function
// attempts to auto-detect; when auto-detection is ambiguous it returns ErrTopologyAmbiguous.
func repairTopologyVars(w io.Writer, deps Deps, topology string) error {
	projectID, err := deps.GetRepoVariable(deps.Owner, deps.RepoName, ProjectVarName)
	if err != nil || projectID == "" {
		return fmt.Errorf(ProjectVarName + " not set — cannot repair topology")
	}

	// Detect topology from linked repos.
	linked, err := deps.FetchLinkedRepos(projectID)
	if err != nil {
		return fmt.Errorf("fetching linked repos: %w", err)
	}
	topo := DetectTopology(deps.RepoFullName, linked)

	// Check existing variable values.
	fwVer, _ := deps.GetRepoVariable(deps.Owner, deps.RepoName, FrameworkVersionVarName)
	topoVal, _ := deps.GetRepoVariable(deps.Owner, deps.RepoName, TopologyVarName)

	// Determine the correct topology value.
	correctTopo := topology // use caller-supplied choice when provided
	if correctTopo == "" {
		if topo == TopologySingle && (fwVer != "" || len(linked) > 1) {
			// Multiple repos in project, or AGENTIC_FRAMEWORK_VERSION already set →
			// this is the federated CP (current or legacy).
			correctTopo = "federated"
		} else if topo == TopologySingle && len(linked) == 1 && fwVer == "" {
			// Cannot distinguish single from federated CP without more context.
			return ErrTopologyAmbiguous
		} else if topo == TopologySingle {
			correctTopo = "single"
		} else {
			correctTopo = "federated"
		}
	}

	if topoVal != correctTopo {
		if err := deps.SetRepoVariable(deps.Owner, deps.RepoName, TopologyVarName, correctTopo); err != nil {
			return fmt.Errorf("setting %s: %w", TopologyVarName, err)
		}
		if topoVal == "" {
			fmt.Fprintf(w, "  %s  %s set to %q\n", ui.StatusOK.Render("✓"), TopologyVarName, correctTopo)
		} else {
			fmt.Fprintf(w, "  %s  %s corrected from %q to %q\n", ui.StatusOK.Render("✓"), TopologyVarName, topoVal, correctTopo)
		}
		topoVal = correctTopo
	}

	// For federated control planes, ensure AGENTIC_FRAMEWORK_VERSION is set.
	if topoVal == "federated" && fwVer == "" {
		releases, err := deps.FetchReleases(mount.FrameworkRepo)
		if err != nil || len(releases) == 0 {
			return fmt.Errorf("fetching framework releases: %w", err)
		}
		latest := releases[0].TagName
		if err := deps.SetRepoVariable(deps.Owner, deps.RepoName, FrameworkVersionVarName, latest); err != nil {
			return fmt.Errorf("setting %s: %w", FrameworkVersionVarName, err)
		}
		fmt.Fprintf(w, "  %s  %s set to %s\n", ui.StatusOK.Render("✓"), FrameworkVersionVarName, latest)
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
