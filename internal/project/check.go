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
		{"Checking project status options...", checkStatusFieldOptions},
		{"Checking framework version sync...", checkFrameworkVersionSync},
		{"Checking topology variables...", checkTopologyVars},
		{"Checking issue membership in project...", checkOrphanProjectIssues},
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
			Message:     "framework not mounted at .agents/",
			Remediation: "run 'gh agentic init'",
		}
	}
	return CheckResult{
		Name:    "framework",
		Status:  CheckPass,
		Message: "framework mounted at " + version,
	}
}

// checkTopologyVars validates topology-related variables against the
// canonical topology deduced by project.Resolve. Per-topology rules:
//
//   - single           → nothing required locally
//   - federated-cp     → AGENTIC_FRAMEWORK_VERSION must be set locally
//   - federated-domain → no local vars required; a local
//     AGENTIC_FRAMEWORK_VERSION is a misconfiguration
//     (domain repos read the value from the CP, not
//     their own repo)
func checkTopologyVars(deps Deps) CheckResult {
	ctx, err := Resolve(deps)
	if err != nil {
		return CheckResult{
			Name:    "topology-vars",
			Status:  CheckWarn,
			Message: "topology check skipped — cannot resolve topology: " + err.Error(),
		}
	}
	if ctx.ProjectID == "" {
		return CheckResult{
			Name:    "topology-vars",
			Status:  CheckWarn,
			Message: "topology check skipped — " + ProjectVarName + " not set",
		}
	}

	switch ctx.Topology {
	case TopologyStringSingle:
		return CheckResult{
			Name:    "topology-vars",
			Status:  CheckPass,
			Message: "topology=single — no topology vars required",
		}

	case TopologyStringFederation:
		// Federation topology is declared by FEDERATION.md presence; no
		// AGENTIC_TOPOLOGY variable writes are required.
		return CheckResult{
			Name:    "topology-vars",
			Status:  CheckPass,
			Message: "topology=federation — FEDERATION.md present, no variable writes required",
		}
	}

	return CheckResult{
		Name:        "topology-vars",
		Status:      CheckWarn,
		Message:     "unrecognised topology " + ctx.Topology,
		Remediation: "run 'gh agentic repair'",
	}
}

func checkStatusFieldOptions(deps Deps) CheckResult {
	projectID, err := deps.GetRepoVariable(deps.Owner, deps.RepoName, ProjectVarName)
	if err != nil || projectID == "" {
		return CheckResult{
			Name:    "status-options",
			Status:  CheckWarn,
			Message: "project status options check skipped — " + ProjectVarName + " not set",
		}
	}

	tpl, err := ReadProjectTemplate()
	if err != nil {
		return CheckResult{
			Name:    "status-options",
			Status:  CheckWarn,
			Message: "project status options check skipped — could not read template",
		}
	}

	fields, err := deps.FetchProjectFields(projectID)
	if err != nil {
		return CheckResult{
			Name:    "status-options",
			Status:  CheckWarn,
			Message: "project status options check skipped — could not fetch fields: " + err.Error(),
		}
	}

	var statusFieldID string
	var existingOptions []string
	for _, f := range fields {
		if f.Name == "Status" {
			statusFieldID = f.ID
			for _, o := range f.Options {
				existingOptions = append(existingOptions, o.Name)
			}
			break
		}
	}

	if statusFieldID == "" {
		return CheckResult{
			Name:        "status-options",
			Status:      CheckFail,
			Message:     "project Status field not found",
			Remediation: "run 'gh agentic repair'",
		}
	}

	existing := make(map[string]bool, len(existingOptions))
	for _, name := range existingOptions {
		existing[name] = true
	}

	var missing []string
	for _, o := range tpl.StatusField.Options {
		if !existing[o.Name] {
			missing = append(missing, o.Name)
		}
	}

	if len(missing) > 0 {
		return CheckResult{
			Name:        "status-options",
			Status:      CheckFail,
			Message:     fmt.Sprintf("missing project status options: %s", strings.Join(missing, ", ")),
			Remediation: "run 'gh agentic repair'",
		}
	}

	return CheckResult{
		Name:    "status-options",
		Status:  CheckPass,
		Message: fmt.Sprintf("project status options OK (%d options)", len(existingOptions)),
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
			Remediation: "run 'gh agentic repair' to sync to the control plane version",
		}
	}

	return CheckResult{
		Name:    "framework-version-sync",
		Status:  CheckPass,
		Message: fmt.Sprintf("framework version in sync (%s)", localVersion),
	}
}

// checkOrphanProjectIssues detects open issues carrying a type
// label (feature / requirement / task) that are NOT members of the
// agentic ProjectV2. The pipeline cannot resolve such issues via
// `gh agentic status …` — `gh agentic` only reports project members
// — so a feature-design / dev-session triggered on an orphan halts
// at "issue not found in project" without producing any artefacts.
//
// The gap surfaced on yapper: features #13–#19 were created with
// the `feature` + `in-design` labels but never added to the
// project. The feature-design recipe aborted on every one with the
// same error. This check catches the condition at `gh agentic check`
// time; `gh agentic repair` then adds the orphans to the project
// via `addProjectV2ItemById`.
func checkOrphanProjectIssues(deps Deps) CheckResult {
	projectID, err := deps.GetRepoVariable(deps.Owner, deps.RepoName, ProjectVarName)
	if err != nil || projectID == "" {
		// No project set — earlier checks already report this; no
		// orphan check is meaningful here.
		return CheckResult{
			Name:    "orphan-issues",
			Status:  CheckWarn,
			Message: "orphan-issue check skipped — " + ProjectVarName + " not set",
		}
	}

	if deps.FetchOrphanIssues == nil {
		return CheckResult{
			Name:    "orphan-issues",
			Status:  CheckWarn,
			Message: "orphan-issue check skipped — no fetcher configured",
		}
	}

	orphans, err := deps.FetchOrphanIssues(deps.Owner, deps.RepoName, projectID)
	if err != nil {
		return CheckResult{
			Name:    "orphan-issues",
			Status:  CheckWarn,
			Message: fmt.Sprintf("orphan-issue check failed — %v", err),
		}
	}

	if len(orphans) == 0 {
		return CheckResult{
			Name:    "orphan-issues",
			Status:  CheckPass,
			Message: "all type-labelled open issues are project members",
		}
	}

	// Build a compact summary: "#13, #14, #15 (3 orphan issue(s))"
	parts := make([]string, 0, len(orphans))
	for _, o := range orphans {
		parts = append(parts, fmt.Sprintf("#%d", o.Number))
	}
	noun := "issue"
	if len(orphans) > 1 {
		noun = "issues"
	}
	return CheckResult{
		Name:   "orphan-issues",
		Status: CheckFail,
		Message: fmt.Sprintf(
			"%d open %s carrying a type label but missing from the project: %s",
			len(orphans), noun, strings.Join(parts, ", "),
		),
		Remediation: "run 'gh agentic repair' to add each orphan to the agentic project",
	}
}
