package project

import (
	"fmt"
	"io"

	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// CheckStatus represents the result of a single check.
type CheckStatus int

const (
	CheckPass CheckStatus = iota
	CheckWarn
	CheckFail
)

// CheckResult is the outcome of a single health check.
type CheckResult struct {
	Name        string
	Status      CheckStatus
	Message     string
	Remediation string
}

// CheckReport holds all results from a project check run.
type CheckReport struct {
	Results []CheckResult
}

// FailCount returns the number of failed checks.
func (r *CheckReport) FailCount() int {
	n := 0
	for _, c := range r.Results {
		if c.Status == CheckFail {
			n++
		}
	}
	return n
}

// WarnCount returns the number of warning checks.
func (r *CheckReport) WarnCount() int {
	n := 0
	for _, c := range r.Results {
		if c.Status == CheckWarn {
			n++
		}
	}
	return n
}

// RunChecks executes all project health checks and returns a report.
func RunChecks(deps Deps) *CheckReport {
	report := &CheckReport{}

	report.Results = append(report.Results, checkProjectIDSet(deps))
	report.Results = append(report.Results, checkProjectAccessible(deps))
	report.Results = append(report.Results, checkTopologyResolvable(deps))
	report.Results = append(report.Results, checkFrameworkMounted(deps))

	return report
}

// PrintReport renders the check report to w and returns true if all checks passed.
func PrintReport(w io.Writer, report *CheckReport) bool {
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  gh agentic — project check")
	fmt.Fprintln(w, "")

	for _, r := range report.Results {
		icon := statusIcon(r.Status)
		fmt.Fprintf(w, "  %s  %s\n", icon, r.Message)
		if r.Remediation != "" {
			fmt.Fprintf(w, "       %s\n", ui.Muted.Render("→ "+r.Remediation))
		}
	}

	fmt.Fprintln(w, "")
	fails := report.FailCount()
	warns := report.WarnCount()

	if fails > 0 {
		fmt.Fprintf(w, "  %s\n\n", ui.StatusDanger.Render(fmt.Sprintf("%d check(s) failed", fails)))
		return false
	}
	if warns > 0 {
		fmt.Fprintf(w, "  %s\n\n", ui.StatusWarning.Render(fmt.Sprintf("%d warning(s)", warns)))
		return true
	}
	fmt.Fprintf(w, "  %s\n\n", ui.StatusOK.Render("All checks passed"))
	return true
}

func statusIcon(s CheckStatus) string {
	switch s {
	case CheckPass:
		return ui.StatusOK.Render("✓")
	case CheckWarn:
		return ui.StatusWarning.Render("⚠")
	default:
		return ui.StatusDanger.Render("✗")
	}
}

// --- Individual checks ---

func checkProjectIDSet(deps Deps) CheckResult {
	val, err := deps.GetRepoVariable(deps.Owner, deps.RepoName, ProjectVarName)
	if err != nil || val == "" {
		return CheckResult{
			Name:        "project-id",
			Status:      CheckFail,
			Message:     ProjectVarName + " is not set",
			Remediation: "run 'gh agentic project create' or 'gh agentic project join <project-id>'",
		}
	}
	return CheckResult{
		Name:    "project-id",
		Status:  CheckPass,
		Message: ProjectVarName + " is set (" + val + ")",
	}
}

func checkProjectAccessible(deps Deps) CheckResult {
	val, err := deps.GetRepoVariable(deps.Owner, deps.RepoName, ProjectVarName)
	if err != nil || val == "" {
		return CheckResult{
			Name:    "project-accessible",
			Status:  CheckWarn,
			Message: "project accessibility skipped — " + ProjectVarName + " not set",
		}
	}

	_, err = deps.FetchLinkedRepos(val)
	if err != nil {
		return CheckResult{
			Name:        "project-accessible",
			Status:      CheckFail,
			Message:     "cannot access project " + val,
			Remediation: "check that the project exists and gh auth has read access",
		}
	}
	return CheckResult{
		Name:    "project-accessible",
		Status:  CheckPass,
		Message: "project is accessible",
	}
}

func checkTopologyResolvable(deps Deps) CheckResult {
	val, err := deps.GetRepoVariable(deps.Owner, deps.RepoName, ProjectVarName)
	if err != nil || val == "" {
		return CheckResult{
			Name:    "topology",
			Status:  CheckWarn,
			Message: "topology check skipped — " + ProjectVarName + " not set",
		}
	}

	linked, err := deps.FetchLinkedRepos(val)
	if err != nil {
		return CheckResult{
			Name:    "topology",
			Status:  CheckWarn,
			Message: "topology check skipped — project not accessible",
		}
	}

	topo := DetectTopology(deps.RepoFullName, linked)
	switch topo {
	case TopologyUnknown:
		return CheckResult{
			Name:        "topology",
			Status:      CheckFail,
			Message:     "topology unknown — no repos linked to project",
			Remediation: "link this repo to the project in GitHub Project settings",
		}
	default:
		cp, _ := ControlPlaneRepo(linked)
		return CheckResult{
			Name:    "topology",
			Status:  CheckPass,
			Message: fmt.Sprintf("topology: %s (control plane: %s)", topo, cp.NameWithOwner),
		}
	}
}

func checkFrameworkMounted(deps Deps) CheckResult {
	version, err := deps.ReadAIVersion(deps.Root)
	if err != nil || version == "" {
		return CheckResult{
			Name:        "framework",
			Status:      CheckFail,
			Message:     "framework not mounted at .ai/",
			Remediation: "run 'gh agentic mount <version>'",
		}
	}
	return CheckResult{
		Name:    "framework",
		Status:  CheckPass,
		Message: "framework mounted at " + version,
	}
}
