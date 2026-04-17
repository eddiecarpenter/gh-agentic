package project

import (
	"fmt"
	"io"
	"strings"

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

// checkStep pairs a human-readable progress label with a check function.
type checkStep struct {
	label string
	fn    func(Deps) CheckResult
}

// allCheckSteps returns the ordered list of checks with their progress labels.
func allCheckSteps() []checkStep {
	return []checkStep{
		{"Checking agentic project ID...", checkProjectIDSet},
		{"Checking agentic project accessibility...", checkProjectAccessible},
		{"Checking topology...", checkTopologyResolvable},
		{"Checking framework mount...", checkFrameworkMounted},
		{"Checking agentic project views...", checkProjectViews},
		{"Checking framework version sync...", checkFrameworkVersionSync},
		{"Checking topology variables...", checkTopologyVars},
	}
}

// RunChecks executes all project health checks and returns a report.
func RunChecks(deps Deps) *CheckReport {
	return RunChecksWithProgress(deps, nil)
}

// RunChecksWithProgress executes all checks, calling setLabel before each one
// so a spinner can display the current operation. setLabel may be nil.
func RunChecksWithProgress(deps Deps, setLabel func(string)) *CheckReport {
	report := &CheckReport{}
	for _, step := range allCheckSteps() {
		if setLabel != nil {
			setLabel(step.label)
		}
		report.Results = append(report.Results, step.fn(deps))
	}
	return report
}

// PrintReport renders the check report to w and returns true if all checks passed.
func PrintReport(w io.Writer, report *CheckReport) bool {
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  gh agentic — project check")
	fmt.Fprintln(w, "")

	for _, r := range report.Results {
		icon := StatusIcon(r.Status)
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

func StatusIcon(s CheckStatus) string {
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
			Remediation: "run 'gh agentic project init' to join or establish an agentic project",
		}
	}
	name := ProjectDisplayName(deps, val)
	return CheckResult{
		Name:    "project-id",
		Status:  CheckPass,
		Message: ProjectVarName + " is set — " + name,
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

	name := ProjectDisplayName(deps, val)

	_, err = deps.FetchLinkedRepos(val)
	if err != nil {
		// Distinguish between "deleted" (title also missing) and "inaccessible".
		if name == val {
			return CheckResult{
				Name:        "project-accessible",
				Status:      CheckFail,
				Message:     "agentic project not found — it may have been deleted (" + val + ")",
				Remediation: "run 'gh agentic project unlink' then 'gh agentic project init' to join an agentic project",
			}
		}
		return CheckResult{
			Name:        "project-accessible",
			Status:      CheckFail,
			Message:     "cannot access agentic project \"" + name + "\" — check gh auth has read access",
			Remediation: "run 'gh auth status' and verify project permissions",
		}
	}
	return CheckResult{
		Name:    "project-accessible",
		Status:  CheckPass,
		Message: "project \"" + name + "\" is accessible",
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
			Message:     "topology unknown — no repos linked to the agentic project",
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

func checkProjectViews(deps Deps) CheckResult {
	projectID, err := deps.GetRepoVariable(deps.Owner, deps.RepoName, ProjectVarName)
	if err != nil || projectID == "" {
		return CheckResult{
			Name:    "views",
			Status:  CheckWarn,
			Message: "project views check skipped — " + ProjectVarName + " not set",
		}
	}

	tpl, err := ReadProjectTemplate()
	if err != nil {
		return CheckResult{
			Name:    "views",
			Status:  CheckWarn,
			Message: "project views check skipped — could not read template",
		}
	}

	existing, err := deps.FetchProjectViews(projectID)
	if err != nil {
		return CheckResult{
			Name:    "views",
			Status:  CheckWarn,
			Message: "project views check skipped — could not fetch views: " + err.Error(),
		}
	}

	existingNames := make(map[string]bool, len(existing))
	for _, v := range existing {
		existingNames[v.Name] = true
	}

	var missing []string
	for _, v := range tpl.Views {
		if !existingNames[v.Name] {
			missing = append(missing, v.Name)
		}
	}

	if len(missing) > 0 {
		return CheckResult{
			Name:        "views",
			Status:      CheckFail,
			Message:     fmt.Sprintf("missing agentic project views: %s", strings.Join(missing, ", ")),
			Remediation: "run 'gh agentic project repair'",
		}
	}
	return CheckResult{
		Name:    "views",
		Status:  CheckPass,
		Message: fmt.Sprintf("agentic project views OK (%d views)", len(tpl.Views)),
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

func checkTopologyVars(deps Deps) CheckResult {
	projectID, err := deps.GetRepoVariable(deps.Owner, deps.RepoName, ProjectVarName)
	if err != nil || projectID == "" {
		return CheckResult{
			Name:    "topology-vars",
			Status:  CheckWarn,
			Message: "topology check skipped — " + ProjectVarName + " not set",
		}
	}

	topoVal, err := deps.GetRepoVariable(deps.Owner, deps.RepoName, TopologyVarName)
	if err != nil || topoVal == "" {
		return CheckResult{
			Name:        "topology-vars",
			Status:      CheckWarn,
			Message:     TopologyVarName + " not set — topology cannot be determined",
			Remediation: "run 'gh agentic project repair'",
		}
	}

	// Validate that AGENTIC_TOPOLOGY=single is consistent with the project state.
	// If the project has multiple linked repos, or AGENTIC_FRAMEWORK_VERSION is already
	// set, then this repo is a federated CP and the value is wrong.
	if topoVal == "single" {
		fwVer, _ := deps.GetRepoVariable(deps.Owner, deps.RepoName, FrameworkVersionVarName)
		linked, linkedErr := deps.FetchLinkedRepos(projectID)
		if linkedErr == nil && len(linked) > 1 {
			return CheckResult{
				Name:        "topology-vars",
				Status:      CheckWarn,
				Message:     TopologyVarName + "=single but project has multiple linked repos — should be federated (control plane)",
				Remediation: "run 'gh agentic project repair'",
			}
		}
		if fwVer != "" {
			return CheckResult{
				Name:        "topology-vars",
				Status:      CheckWarn,
				Message:     TopologyVarName + "=single but " + FrameworkVersionVarName + " is set — should be federated (control plane)",
				Remediation: "run 'gh agentic project repair'",
			}
		}
		return CheckResult{
			Name:    "topology-vars",
			Status:  CheckPass,
			Message: TopologyVarName + "=single",
		}
	}

	// AGENTIC_FRAMEWORK_VERSION is only required on federated control planes —
	// domain repos read it from the CP; single repos don't broadcast it at all.
	if topoVal == "federated" {
		fwVer, err := deps.GetRepoVariable(deps.Owner, deps.RepoName, FrameworkVersionVarName)
		if err != nil || fwVer == "" {
			return CheckResult{
				Name:        "topology-vars",
				Status:      CheckWarn,
				Message:     FrameworkVersionVarName + " not set on control plane",
				Remediation: "run 'gh agentic project switch version <version>' to set it",
			}
		}
		return CheckResult{
			Name:    "topology-vars",
			Status:  CheckPass,
			Message: TopologyVarName + "=" + topoVal + ", " + FrameworkVersionVarName + "=" + fwVer,
		}
	}

	// Unknown topology value.
	return CheckResult{
		Name:        "topology-vars",
		Status:      CheckWarn,
		Message:     TopologyVarName + "=" + topoVal + " — unrecognised value (expected 'single' or 'federated')",
		Remediation: "run 'gh agentic project repair'",
	}
}

func checkFrameworkVersionSync(deps Deps) CheckResult {
	// Only meaningful if project is affiliated.
	projectID, err := deps.GetRepoVariable(deps.Owner, deps.RepoName, ProjectVarName)
	if err != nil || projectID == "" {
		return CheckResult{
			Name:    "framework-version-sync",
			Status:  CheckWarn,
			Message: "framework version sync skipped — " + ProjectVarName + " not set",
		}
	}

	localVersion, err := deps.ReadAIVersion(deps.Root)
	if err != nil || localVersion == "" {
		return CheckResult{
			Name:    "framework-version-sync",
			Status:  CheckWarn,
			Message: "framework version sync skipped — .ai-version not found",
		}
	}

	// Find the control plane framework version.
	linked, err := deps.FetchLinkedRepos(projectID)
	if err != nil {
		return CheckResult{
			Name:    "framework-version-sync",
			Status:  CheckWarn,
			Message: "framework version sync skipped — cannot reach project",
		}
	}

	topo := DetectTopology(deps.RepoFullName, linked)

	if topo != TopologyFederated {
		// Single or federated CP.
		// For single topology AGENTIC_FRAMEWORK_VERSION is not used — the local
		// .ai-version file is the sole source of truth and is always "in sync".
		// For federated CP, the AGENTIC_FRAMEWORK_VERSION presence/value check is
		// handled by checkTopologyVars; here we just confirm local is mounted.
		topoVar, _ := deps.GetRepoVariable(deps.Owner, deps.RepoName, TopologyVarName)
		if topoVar != "federated" {
			return CheckResult{
				Name:    "framework-version-sync",
				Status:  CheckPass,
				Message: fmt.Sprintf("framework version OK (%s) — single topology, no remote sync required", localVersion),
			}
		}
		// Federated CP: verify local matches the broadcast variable.
		cpVersion, err := deps.GetRepoVariable(deps.Owner, deps.RepoName, FrameworkVersionVarName)
		if err != nil || cpVersion == "" {
			// Missing variable — checkTopologyVars will flag this; skip here.
			return CheckResult{
				Name:    "framework-version-sync",
				Status:  CheckWarn,
				Message: "framework version sync skipped — " + FrameworkVersionVarName + " not yet set on control plane",
			}
		}
		if localVersion != cpVersion {
			return CheckResult{
				Name:        "framework-version-sync",
				Status:      CheckFail,
				Message:     fmt.Sprintf("framework out of sync — local: %s, %s: %s", localVersion, FrameworkVersionVarName, cpVersion),
				Remediation: "run 'gh agentic project switch version " + cpVersion + "' to align local framework",
			}
		}
		return CheckResult{
			Name:    "framework-version-sync",
			Status:  CheckPass,
			Message: fmt.Sprintf("framework version in sync (%s)", localVersion),
		}
	}

	// Domain repo — compare local against control plane's AGENTIC_FRAMEWORK_VERSION.
	cp, ok := ControlPlaneRepo(linked)
	if !ok {
		return CheckResult{
			Name:    "framework-version-sync",
			Status:  CheckWarn,
			Message: "framework version sync skipped — control plane not found",
		}
	}
	parts := strings.SplitN(cp.NameWithOwner, "/", 2)
	if len(parts) != 2 {
		return CheckResult{
			Name:    "framework-version-sync",
			Status:  CheckWarn,
			Message: "framework version sync skipped — cannot parse control plane repo",
		}
	}
	cpOwner, cpRepo := parts[0], parts[1]

	cpVersion, err := deps.GetRepoVariable(cpOwner, cpRepo, FrameworkVersionVarName)
	if err != nil || cpVersion == "" {
		return CheckResult{
			Name:    "framework-version-sync",
			Status:  CheckWarn,
			Message: FrameworkVersionVarName + " not set on control plane — run 'gh agentic project switch version <version>' to set it",
		}
	}

	if localVersion != cpVersion {
		return CheckResult{
			Name:        "framework-version-sync",
			Status:      CheckFail,
			Message:     fmt.Sprintf("framework out of sync — local: %s, control plane: %s", localVersion, cpVersion),
			Remediation: "run 'gh agentic mount' to sync to the control plane version",
		}
	}

	return CheckResult{
		Name:    "framework-version-sync",
		Status:  CheckPass,
		Message: fmt.Sprintf("framework version in sync (%s)", localVersion),
	}
}
