package doctor

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/eddiecarpenter/gh-agentic/internal/auth"
	"github.com/eddiecarpenter/gh-agentic/internal/mount"
	"github.com/eddiecarpenter/gh-agentic/internal/project"
)

// CheckDeps holds injectable dependencies for check functions.
type CheckDeps struct {
	Root                string
	RepoFullName        string
	Owner               string
	RepoName            string
	OwnerType           string
	Topology            string // "single", "federation", "" (unknown)
	ProjectID           string // value of AGENTIC_PROJECT_ID if set
	ProjectIDReadFailed bool   // true when AGENTIC_PROJECT_ID could not be read due to token permission error
	Run                 auth.RunCommandFunc
	ReadCreds           auth.ReadCredentialsFunc
	// FetchProjectTitle is used by checkProjectReachability to confirm the
	// configured AGENTIC_PROJECT_ID resolves via the GraphQL API. Tests
	// substitute a fake; production wires project.DefaultFetchProjectTitle.
	FetchProjectTitle project.FetchProjectTitleFunc
	// FetchProjectFields and UpdateStatusFieldOptions are used by
	// checkProjectStatusOptions (check) and RepairPipeline (repair) to detect
	// and fix missing project status field options. Tests substitute fakes;
	// production wires project.DefaultFetchProjectFields and
	// project.DefaultUpdateStatusFieldOptions.
	FetchProjectFields       project.FetchProjectFieldsFunc
	UpdateStatusFieldOptions project.UpdateStatusFieldOptionsFunc
	// CreateProjectField is used by RepairPipeline to create the "Target repo"
	// field (#872) when it is missing on a federation control-plane project.
	// Repair-only; the check command leaves it nil.
	CreateProjectField project.CreateProjectFieldFunc
	// FetchLinkedRepos, FetchOwnerAndRepoIDs, and LinkRepoToProject are used
	// by checkFederationProjectSync (check) and RepairPipeline (repair) to
	// detect and fix drift between FEDERATION.md and the GitHub Project's
	// linked-repo graph. Tests substitute fakes; production wires the
	// project.Default* implementations. LinkRepoToProject is repair-only; the
	// check command leaves it nil.
	FetchLinkedRepos     project.FetchLinkedReposFunc
	FetchOwnerAndRepoIDs project.FetchOwnerAndRepoIDsFunc
	LinkRepoToProject    project.LinkRepoToProjectFunc
	// FrameworkSource signals that this repo IS the gh-agentic framework
	// source itself (detected by .ai being a symlink). When true, content-
	// layer checks that inspect a mounted .agents/ tree are replaced with a
	// synthetic "skipped — framework source" group so the report is
	// honest about what was and was not examined, and configuration-layer
	// checks (variables, secrets, workflows, project reachability) run as
	// normal. See feature #619 §E.
	FrameworkSource bool
}

// checkGroupStep pairs a spinner label with a check function.
type checkGroupStep struct {
	label string
	fn    func(CheckDeps) Group
}

// LabelDef describes a required pipeline label — its canonical name, a
// six-digit hex colour (no leading #), and a short description. The list
// drives both the check (does the label exist?) and the repair (create it
// if it is missing).
type LabelDef struct {
	Name        string
	Color       string
	Description string
}

// requiredPipelineLabels is the authoritative set of lifecycle labels that
// every agentic repo must have. It covers every label that
// `agentic-pipeline.yml` or a framework skill reads, applies, or removes —
// the doctor check verifies each one exists in the repo and the doctor
// repair creates any that are missing.
//
// Sources audited:
//   - .github/workflows/agentic-pipeline.yml (every `if:` label gate,
//     every `gh issue edit --add-label / --remove-label`).
//   - skills/*/SKILL.md (every `apply-label add=[…], remove=[…]` call).
//   - skills/trigger-*/SKILL.md (the canonical lifecycle transitions).
//
// Drift between this list and the workflow / skills is caught by
// TestRequiredLabelsCoverPipelineReferences in checks_test.go — adding a
// new label trigger anywhere without updating this list fails the build.
//
// Colour conventions used by the framework:
//
//	0075ca (blue)   — completed / ready states (ready-to-implement, designed,
//	                  compliance-verified, done)
//	d93f0b (orange) — active states (in-design, in-development, in-verification,
//	                  in-review) and concurrency beacons (*-in-progress)
//	e4e669 (yellow) — human-input flags (interactive-design,
//	                  needs-interactive-design, needs-human-review,
//	                  assigned-to-agent, needs-scoping)
//	a2eeef (light)  — type labels (requirement, feature, task)
//	c5def5 (pale)   — initial / sorting states (backlog, scoping)
var requiredPipelineLabels = []LabelDef{
	// --- Type labels ---
	{
		Name:        "requirement",
		Color:       "a2eeef",
		Description: "A business need captured as a Requirement issue",
	},
	{
		Name:        "feature",
		Color:       "a2eeef",
		Description: "A scoped unit of work the pipeline can deliver as a single PR",
	},
	{
		Name:        "task",
		Color:       "a2eeef",
		Description: "An ordered Task sub-issue under a Feature — one commit per task",
	},
	// --- Requirement lifecycle ---
	{
		Name:        "backlog",
		Color:       "c5def5",
		Description: "Captured but not yet scoped — initial state for Requirements and Features",
	},
	{
		Name:        "scoping",
		Color:       "c5def5",
		Description: "Requirement is being scoped into one or more Features",
	},
	{
		Name:        "ready-to-implement",
		Color:       "0075ca",
		Description: "Requirement is scoped — its child Features are waiting for design triggers",
	},
	// --- Feature design states ---
	{
		Name:        "in-design",
		Color:       "d93f0b",
		Description: "Triggers automated Feature Design (Stage 3)",
	},
	{
		Name:        "interactive-design",
		Color:       "e4e669",
		Description: "Triggers interactive Feature Design session",
	},
	{
		Name:        "needs-interactive-design",
		Color:       "e4e669",
		Description: "Feature requires interactive design before automation can run",
	},
	{
		Name:        "designed",
		Color:       "0075ca",
		Description: "Design complete — parked awaiting trigger to implementation",
	},
	// --- Feature implementation/verification states ---
	{
		Name:        "in-development",
		Color:       "d93f0b",
		Description: "Triggers Dev Session (Stage 4) — feature is being implemented",
	},
	{
		Name:        "in-verification",
		Color:       "d93f0b",
		Description: "Triggers Compliance Verify (Stage 5) — awaiting AC check",
	},
	{
		Name:        "compliance-verified",
		Color:       "0075ca",
		Description: "All acceptance criteria verified — ready for PR",
	},
	{
		Name:        "in-review",
		Color:       "d93f0b",
		Description: "PR open and awaiting human review",
	},
	{
		Name:        "done",
		Color:       "0075ca",
		Description: "Feature merged and complete — applied by Stage 7",
	},
	// --- Concurrency beacons (in-progress markers) ---
	{
		Name:        "design-in-progress",
		Color:       "d93f0b",
		Description: "Feature Design session active — concurrency beacon (clear with foreground-recovery)",
	},
	{
		Name:        "development-in-progress",
		Color:       "d93f0b",
		Description: "Dev Session active — concurrency beacon (clear with foreground-recovery)",
	},
	{
		Name:        "issue-in-progress",
		Color:       "d93f0b",
		Description: "Issue Session active — concurrency beacon (clear with foreground-recovery)",
	},
	// --- Human-flag / escalation labels ---
	{
		Name:        "assigned-to-agent",
		Color:       "e4e669",
		Description: "Triggers Issue Session (Stage 4c) — agent handles the issue",
	},
	{
		Name:        "needs-scoping",
		Color:       "e4e669",
		Description: "Issue is too large for issue-session — redirected to scoping",
	},
	{
		Name:        "needs-human-review",
		Color:       "e4e669",
		Description: "Compliance cycle cap reached — human review required before pipeline can restart",
	},
}

// checksForTopologyWithLabels returns the ordered list of labelled check steps.
func checksForTopologyWithLabels(deps CheckDeps) []checkGroupStep {
	// On the gh-agentic framework source itself, mount-layer checks do
	// not apply (.agents is a symlink → ., not a tarball clone). Replace
	// them with a synthetic "skipped" group and preserve everything
	// else. See feature #619 §E.
	if deps.FrameworkSource {
		base := []checkGroupStep{
			{"Checking repository...", checkRepository},
			{"Skipping content-layer checks...", checkFrameworkSourceSkipped},
			{"Checking workflows...", checkWorkflows},
			{"Checking variables and secrets...", checkVariablesAndSecrets},
			{"Checking pipeline labels...", checkLabels},
			{"Checking project reachability...", checkProjectReachability},
			{"Checking project status options...", checkProjectStatusOptions},
			{"Checking project target-repo field...", checkProjectTargetRepoField},
		}
		return base
	}

	// Feature #874: a pure-code domain repo carries AGENTIC_PROJECT_ID but no
	// .agents mount and no FEDERATION.md — the control plane holds the framework.
	// It runs only the minimal checks: it is registered code, not a framework
	// consumer, so mount/content/label/workflow checks do not apply.
	if isPureCodeDomainRepo(deps) {
		return []checkGroupStep{
			{"Identifying repo role...", checkDomainRepo},
			{"Checking repository...", checkRepository},
			{"Checking project reachability...", checkProjectReachability},
		}
	}

	// Feature #824: topology is now binary (single / federation). All repos
	// run the same set of checks regardless of topology variant.
	base := []checkGroupStep{
		{"Checking repository...", checkRepository},
		{"Checking framework mount...", checkFramework},
		{"Checking agent files...", checkAgentFiles},
		{"Checking skill frontmatter...", checkSkillFrontmatter},
		{"Checking workflows...", checkWorkflows},
		{"Checking variables and secrets...", checkVariablesAndSecrets},
		// Pipeline labels must exist on every repo — skills apply them via gh CLI
		// regardless of topology. Runs before project reachability so the label
		// remediation is visible alongside other pipeline-infrastructure checks.
		{"Checking pipeline labels...", checkLabels},
		// Project reachability applies to every topology — all of them consume the
		// GitHub Project board via `gh agentic status`. Added last so it lands at
		// the bottom of the report, near the "what the agent actually needs" view.
		{"Checking project reachability...", checkProjectReachability},
		// Project status options run after reachability — no point checking options
		// on a project that isn't reachable.
		{"Checking project status options...", checkProjectStatusOptions},
		// Target-repo field: required on federation control-plane projects (#872);
		// skipped for single topology.
		{"Checking project target-repo field...", checkProjectTargetRepoField},
		// Federation manifest: validates FEDERATION.md when present; passes silently
		// when absent (single topology).
		{"Checking federation manifest...", checkFederationManifest},
		// Federation project sync: checks that every manifest repo is linked to the
		// federation's GitHub Project, and flags project-linked repos not in the
		// manifest. Guard: skipped when FEDERATION.md is absent.
		{"Checking federation project sync...", checkFederationProjectSync},
		// Legacy federation config: warns when pre-#824 topology variables or .cp/
		// directory are still present so operators know to clean them up.
		{"Checking legacy federation config...", checkLegacyFederationConfig},
	}
	return base
}

// checkDomainRepo reports that the current repo is a registered pure-code domain
// (execution) repo (#874): it carries AGENTIC_PROJECT_ID but no .agents mount —
// the framework is managed on the control plane, not here. This is the correct,
// passing state for a domain repo; the mount / pipeline-infrastructure checks do
// not apply because the domain repo is not under framework control.
func checkDomainRepo(deps CheckDeps) Group {
	g := Group{Name: "Domain repo"}
	g.Results = append(g.Results, CheckResult{
		Name:    "domain-repo",
		Status:  Pass,
		Message: "pure-code domain (execution) repo — the framework is managed on the control plane; this repo carries code only and is not under framework control here",
	})
	return g
}

// isPureCodeDomainRepo reports whether deps describes a registered pure-code
// domain repo (#874): it carries AGENTIC_PROJECT_ID but has no .agents mount,
// is not the framework source, and is not a federation requirements repo (no
// FEDERATION.md). Such a repo is validated as code, not as a framework consumer.
//
// Detection is absence-based: a domain repo carrying a stale .agents from the
// pre-pivot model is NOT distinguishable here from a single-topology repo, and
// its cleanup is handled by the #876 migration where the control-plane context
// is available.
func isPureCodeDomainRepo(deps CheckDeps) bool {
	if deps.FrameworkSource {
		return false
	}
	if strings.TrimSpace(deps.ProjectID) == "" {
		return false
	}
	if project.IsFederationRepo(deps.Root) {
		return false
	}
	if _, err := os.Stat(filepath.Join(deps.Root, ".agents")); err == nil {
		return false
	}
	return true
}

// checksForTopology returns the ordered list of check functions for the given topology.
func checksForTopology(deps CheckDeps) []func(CheckDeps) Group {
	steps := checksForTopologyWithLabels(deps)
	fns := make([]func(CheckDeps) Group, len(steps))
	for i, s := range steps {
		fns[i] = s.fn
	}
	return fns
}

// RunAllChecks runs all checks (topology-aware) and returns a grouped Report.
// Used in tests and non-streaming contexts.
func RunAllChecks(deps CheckDeps) *Report {
	return RunAllChecksWithProgress(deps, nil)
}

// RunAllChecksWithProgress runs all checks, calling setLabel before each step.
// If setLabel is nil it is not called. Returns the completed Report.
func RunAllChecksWithProgress(deps CheckDeps, setLabel func(string)) *Report {
	report := &Report{}
	for _, step := range checksForTopologyWithLabels(deps) {
		if setLabel != nil {
			setLabel(step.label)
		}
		report.Groups = append(report.Groups, step.fn(deps))
	}
	return report
}

// StreamAllChecks runs each check group and renders it immediately as it
// completes, giving the user progressive feedback rather than a single dump.
// Returns the completed Report for summary rendering.
func StreamAllChecks(w io.Writer, deps CheckDeps) *Report {
	report := &Report{}
	for _, fn := range checksForTopology(deps) {
		g := fn(deps)
		RenderGroup(w, g)
		report.Groups = append(report.Groups, g)
	}
	return report
}

// checkRepository checks basic repository setup.
func checkRepository(deps CheckDeps) Group {
	g := Group{Name: "Repository"}

	// Git repo check.
	repoMsg := fmt.Sprintf("Git repository (%s)", deps.RepoFullName)
	if deps.RepoFullName != "" {
		g.Results = append(g.Results, CheckResult{
			Name: "git-repo", Status: Pass, Message: repoMsg,
		})
	} else {
		g.Results = append(g.Results, CheckResult{
			Name: "git-repo", Status: Fail, Message: "Git repository not detected",
		})
	}

	// README.md.
	if fileExists(filepath.Join(deps.Root, "README.md")) {
		g.Results = append(g.Results, CheckResult{
			Name: "readme", Status: Pass, Message: "README.md exists",
		})
	} else {
		g.Results = append(g.Results, CheckResult{
			Name: "readme", Status: Warning, Message: "README.md not found — recommended",
		})
	}

	return g
}

// checkFrameworkSourceSkipped is substituted for checkFramework,
// checkAgentFiles, and checkSkillFrontmatter when the repo is the
// gh-agentic framework source itself. It emits one informational
// result per skipped concern so the report shape stays parallel with
// the consumer-repo flow and the user can see exactly what was not
// inspected and why.
func checkFrameworkSourceSkipped(deps CheckDeps) Group {
	reason := "framework source (.agents is a symlink) — content-layer checks do not apply"
	return Group{
		Name: "Framework source",
		Results: []CheckResult{
			{Name: "framework-mount", Status: Warning, Message: "skipped: " + reason},
			{Name: "agent-files", Status: Warning, Message: "skipped: " + reason},
			{Name: "skill-frontmatter", Status: Warning, Message: "skipped: " + reason},
		},
	}
}

// checkFramework checks the framework mount state.
func checkFramework(deps CheckDeps) Group {
	g := Group{Name: "Framework"}

	// .agents/ mounted.
	aiDir := filepath.Join(deps.Root, ".agents")
	if dirExists(aiDir) && fileExists(filepath.Join(aiDir, "RULEBOOK.md")) {
		v, err := mount.ReadAIVersionFromGit(deps.Root)
		version := "unknown"
		if err == nil {
			version = v
		}
		g.Results = append(g.Results, CheckResult{
			Name: "ai-mounted", Status: Pass, Message: fmt.Sprintf(".agents/ mounted (%s)", mount.TrimVPrefix(version)),
		})
	} else {
		g.Results = append(g.Results, CheckResult{
			Name: "ai-mounted", Status: Fail,
			Message:     ".agents/ not mounted",
			Remediation: "Run 'gh agentic upgrade <version>'",
		})
	}

	// Framework version readable from the submodule's git metadata.
	// Symlink-mode (gh-agentic itself) and submodule-mode both produce
	// a real git repo at .agents/, so this single check covers both.
	v, err := mount.ReadAIVersionFromGit(deps.Root)
	if err == nil {
		g.Results = append(g.Results, CheckResult{
			Name: "ai-version", Status: Pass, Message: fmt.Sprintf("framework version pinned (%s)", mount.TrimVPrefix(v)),
		})
	} else {
		g.Results = append(g.Results, CheckResult{
			Name: "ai-version", Status: Fail,
			Message:     ".agents/ git metadata missing — framework not installed or submodule uninitialised",
			Remediation: "Run 'gh agentic upgrade <version>' or 'git submodule update --init .agents'",
		})
	}

	// .agents/ should NOT be in .gitignore — the framework is now a tracked
	// submodule. A `.agents/` line in .gitignore is a legacy shallow-clone
	// remnant; the doctor's repair pass will strip it.
	if gitignoreContainsAI(deps.Root) {
		g.Results = append(g.Results, CheckResult{
			Name: "gitignore", Status: Fail,
			Message:     ".agents/ listed in .gitignore — legacy shallow-clone state",
			Remediation: "Remove the '.agents/' line from .gitignore (the doctor repair does this automatically)",
		})
	} else {
		g.Results = append(g.Results, CheckResult{
			Name: "gitignore", Status: Pass, Message: ".agents/ not in .gitignore",
		})
	}

	// Check key framework directories.
	for _, dir := range []string{"skills", "standards"} {
		path := filepath.Join(aiDir, dir)
		name := ".agents/" + dir + "/"
		if dirExists(path) {
			g.Results = append(g.Results, CheckResult{
				Name: name, Status: Pass, Message: name + " complete",
			})
		} else {
			g.Results = append(g.Results, CheckResult{
				Name: name, Status: Warning, Message: name + " missing",
			})
		}
	}

	return g
}

// checkAgentFiles checks CLAUDE.md, AGENTS.md, LOCALRULES.md, skills/.
func checkAgentFiles(deps CheckDeps) Group {
	g := Group{Name: "Agent files"}

	// CLAUDE.md.
	if fileExists(filepath.Join(deps.Root, "CLAUDE.md")) {
		g.Results = append(g.Results, CheckResult{
			Name: "claude-md", Status: Pass, Message: "CLAUDE.md exists",
		})
	} else {
		g.Results = append(g.Results, CheckResult{
			Name: "claude-md", Status: Fail,
			Message:     "CLAUDE.md not found",
			Remediation: "Run 'gh agentic repair'",
		})
	}

	// AGENTS.md.
	if fileExists(filepath.Join(deps.Root, "AGENTS.md")) {
		g.Results = append(g.Results, CheckResult{
			Name: "agents-md", Status: Pass, Message: "AGENTS.md exists",
		})
	} else {
		g.Results = append(g.Results, CheckResult{
			Name: "agents-md", Status: Fail,
			Message:     "AGENTS.md not found",
			Remediation: "Run 'gh agentic repair'",
		})
	}

	// LOCALRULES.md — optional.
	if fileExists(filepath.Join(deps.Root, "LOCALRULES.md")) {
		g.Results = append(g.Results, CheckResult{
			Name: "localrules", Status: Pass, Message: "LOCALRULES.md exists",
		})
	} else {
		g.Results = append(g.Results, CheckResult{
			Name: "localrules", Status: Warning, Message: "LOCALRULES.md not found — recommended for project-specific rules",
		})
	}

	// skills/ — optional.
	if dirExists(filepath.Join(deps.Root, "skills")) {
		g.Results = append(g.Results, CheckResult{
			Name: "skills", Status: Pass, Message: "skills/ exists",
		})
	} else {
		g.Results = append(g.Results, CheckResult{
			Name: "skills", Status: Warning, Message: "skills/ not found — recommended for local skill overrides",
		})
	}

	return g
}

// checkWorkflows verifies wrapper workflows reference correct version.
func checkWorkflows(deps CheckDeps) Group {
	g := Group{Name: "Workflows"}

	version, _ := mount.ReadAIVersionFromGit(deps.Root)
	workflowsDir := filepath.Join(deps.Root, ".github", "workflows")

	workflows := []string{"agentic-pipeline.yml", "release.yml"}
	for _, wf := range workflows {
		path := filepath.Join(workflowsDir, wf)
		if !fileExists(path) {
			g.Results = append(g.Results, CheckResult{
				Name:        wf,
				Status:      Fail,
				Message:     wf + " — not found",
				Remediation: "Run 'gh agentic repair'",
			})
			continue
		}

		data, err := os.ReadFile(path)
		if err != nil {
			g.Results = append(g.Results, CheckResult{
				Name: wf, Status: Fail, Message: fmt.Sprintf("%s unreadable: %v", wf, err),
			})
			continue
		}

		// Only enforce the framework version tag if the workflow actually
		// references gh-agentic via a 'uses:' line. Inlined workflows (the
		// current framework template) don't reference gh-agentic at all and
		// don't need a version tag.
		referencesFramework := strings.Contains(string(data), "eddiecarpenter/gh-agentic")
		switch {
		case version == "" || !referencesFramework:
			g.Results = append(g.Results, CheckResult{
				Name: wf, Status: Pass, Message: wf + " present",
			})
		case strings.Contains(string(data), "@"+version):
			g.Results = append(g.Results, CheckResult{
				Name: wf, Status: Pass, Message: fmt.Sprintf("%s → @%s", wf, mount.TrimVPrefix(version)),
			})
		default:
			g.Results = append(g.Results, CheckResult{
				Name: wf, Status: Fail,
				Message:     fmt.Sprintf("%s — version tag mismatch (expected @%s)", wf, mount.TrimVPrefix(version)),
				Remediation: "Run 'gh agentic repair' to update workflow versions",
			})
		}
	}

	return g
}

// checkVariablesAndSecrets checks GitHub variables and secrets.
func checkVariablesAndSecrets(deps CheckDeps) Group {
	g := Group{Name: "Variables & secrets"}

	// Check variables via gh CLI.
	variables := []string{
		"RUNNER_LABEL",
		"AGENT_PROVIDER", "AGENT_MODEL",
	}
	for _, v := range variables {
		result := checkVariable(deps, v)
		g.Results = append(g.Results, result)
	}

	// Check secrets.
	secrets := []string{"PROJECT_PAT", "PIPELINE_PAT"}
	for _, s := range secrets {
		result := checkSecret(deps, s)
		g.Results = append(g.Results, result)
	}

	// Credential check (delegates to auth.Check logic).
	if deps.ReadCreds != nil {
		data, err := deps.ReadCreds(deps.Run)
		if err != nil {
			g.Results = append(g.Results, CheckResult{
				Name: "claude-creds", Status: Fail,
				Message:     "CLAUDE_CREDENTIALS_JSON — not configured",
				Remediation: "Run 'gh agentic auth login'",
			})
		} else {
			_ = data
			g.Results = append(g.Results, CheckResult{
				Name: "claude-creds", Status: Pass,
				Message: "CLAUDE_CREDENTIALS_JSON configured",
			})
		}
	}

	return g
}

// checkProjectReachability verifies AGENTIC_PROJECT_ID is set and the
// configured ProjectV2 node ID resolves via GraphQL. This catches two common
// misconfigurations that otherwise surface as confusing runtime errors from
// `gh agentic status`: the variable missing entirely, and the variable set
// to a node ID that the authenticated user cannot access (revoked token,
// deleted project, moved to a different org).
//
// ProjectID is supplied by the caller via deps.ProjectID — populated from
// project.Resolve in the CLI layer. The doctor does not re-read AGENTIC_*
// variables directly; the resolver is the single canonical source.
//
// Outcomes:
//   - Variable missing → Fail with a 'gh agentic project join' remediation.
//   - Variable set but GraphQL errors → Fail with an auth-aware remediation.
//   - Variable set and query returns a title → Pass (includes the title so
//     the human can confirm they're pointed at the project they expect).
func checkProjectReachability(deps CheckDeps) Group {
	g := Group{Name: "Project reachability"}

	// Step 1 — verify AGENTIC_PROJECT_ID is present (trusted from the
	// resolver; no fallback read here).
	projectID := strings.TrimSpace(deps.ProjectID)
	if projectID == "" {
		remediation := "Run 'gh agentic project join' to affiliate this repo, " +
			"or 'gh agentic project create' to start a new project"
		g.Results = append(g.Results, CheckResult{
			Name: "project-id", Status: Fail,
			Message:     "AGENTIC_PROJECT_ID not configured",
			Remediation: remediation,
		})
		return g
	}

	// Step 2 — verify the node ID resolves.
	if deps.FetchProjectTitle == nil {
		g.Results = append(g.Results, CheckResult{
			Name: "project-reachability", Status: Warning,
			Message: "AGENTIC_PROJECT_ID set — skipping reachability (no GraphQL client)",
		})
		return g
	}

	title, err := deps.FetchProjectTitle(projectID)
	if err != nil {
		g.Results = append(g.Results, CheckResult{
			Name: "project-reachability", Status: Fail,
			Message:     fmt.Sprintf("AGENTIC_PROJECT_ID is set but GraphQL lookup failed: %v", err),
			Remediation: "Run 'gh auth status' to verify credentials; confirm the project exists and is accessible",
		})
		return g
	}

	g.Results = append(g.Results, CheckResult{
		Name: "project-reachability", Status: Pass,
		Message: fmt.Sprintf("Project reachable: %s", title),
	})
	return g
}

// checkProjectStatusOptions verifies that every status option defined in the
// project template exists on the project board's Status single-select field.
// A missing option (e.g. "In Verification" after a pipeline stage addition)
// causes workflow steps that set project status to fail at runtime.
func checkProjectStatusOptions(deps CheckDeps) Group {
	g := Group{Name: "Project status options"}

	projectID := strings.TrimSpace(deps.ProjectID)
	if projectID == "" {
		g.Results = append(g.Results, CheckResult{
			Name: "status-options", Status: Warning,
			Message: "project status options check skipped — AGENTIC_PROJECT_ID not configured",
		})
		return g
	}

	if deps.FetchProjectFields == nil {
		g.Results = append(g.Results, CheckResult{
			Name: "status-options", Status: Warning,
			Message: "project status options check skipped — no GraphQL client",
		})
		return g
	}

	tpl, err := project.ReadProjectTemplate()
	if err != nil {
		g.Results = append(g.Results, CheckResult{
			Name: "status-options", Status: Warning,
			Message: "project status options check skipped — could not read template: " + err.Error(),
		})
		return g
	}

	fields, err := deps.FetchProjectFields(projectID)
	if err != nil {
		g.Results = append(g.Results, CheckResult{
			Name: "status-options", Status: Warning,
			Message: "project status options check skipped — could not fetch fields: " + err.Error(),
		})
		return g
	}

	var existing []string
	for _, f := range fields {
		if f.Name == "Status" {
			for _, o := range f.Options {
				existing = append(existing, o.Name)
			}
			break
		}
	}

	existingSet := make(map[string]bool, len(existing))
	for _, name := range existing {
		existingSet[name] = true
	}

	var missing []string
	for _, o := range tpl.StatusField.Options {
		if !existingSet[o.Name] {
			missing = append(missing, o.Name)
		}
	}

	if len(missing) > 0 {
		g.Results = append(g.Results, CheckResult{
			Name: "status-options", Status: Fail,
			Message:     fmt.Sprintf("missing project status options: %s", strings.Join(missing, ", ")),
			Remediation: "run 'gh agentic repair'",
		})
		return g
	}

	g.Results = append(g.Results, CheckResult{
		Name: "status-options", Status: Pass,
		Message: fmt.Sprintf("project status options OK (%d options)", len(existing)),
	})
	return g
}

// checkProjectTargetRepoField verifies the project board carries the
// "Target repo" field (#872) that control-plane Feature issues use to record
// their target domain repo. A missing field is repairable.
func checkProjectTargetRepoField(deps CheckDeps) Group {
	g := Group{Name: "Project target-repo field"}

	// The Target repo field is a federation concern — single-topology projects
	// have no separate target repo, so the field is not required there.
	if !project.IsFederationRepo(deps.Root) {
		g.Results = append(g.Results, CheckResult{
			Name: "target-repo-field", Status: Pass,
			Message: "target-repo field not required — single topology",
		})
		return g
	}

	projectID := strings.TrimSpace(deps.ProjectID)
	if projectID == "" {
		g.Results = append(g.Results, CheckResult{
			Name: "target-repo-field", Status: Warning,
			Message: "target-repo field check skipped — AGENTIC_PROJECT_ID not configured",
		})
		return g
	}
	if deps.FetchProjectFields == nil {
		g.Results = append(g.Results, CheckResult{
			Name: "target-repo-field", Status: Warning,
			Message: "target-repo field check skipped — no GraphQL client",
		})
		return g
	}

	fields, err := deps.FetchProjectFields(projectID)
	if err != nil {
		g.Results = append(g.Results, CheckResult{
			Name: "target-repo-field", Status: Warning,
			Message: "target-repo field check skipped — could not fetch fields: " + err.Error(),
		})
		return g
	}

	if _, ok := project.FindField(fields, project.TargetRepoFieldName); !ok {
		g.Results = append(g.Results, CheckResult{
			Name: "target-repo-field", Status: Fail,
			Message:     fmt.Sprintf("project %q field is missing", project.TargetRepoFieldName),
			Remediation: "run 'gh agentic repair'",
		})
		return g
	}

	g.Results = append(g.Results, CheckResult{
		Name: "target-repo-field", Status: Pass,
		Message: fmt.Sprintf("project %q field present", project.TargetRepoFieldName),
	})
	return g
}

// checkFederationManifest validates the FEDERATION.md manifest when it is
// present at deps.Root. When absent, the repo is single-topology and the
// check passes unconditionally. When present but invalid, each error from
// project.ReadFederation is surfaced as a Fail result with the exact error
// text so the operator knows exactly what to fix.
func checkFederationManifest(deps CheckDeps) Group {
	g := Group{Name: "Federation manifest"}

	if !project.IsFederationRepo(deps.Root) {
		g.Results = append(g.Results, CheckResult{
			Name:    "federation-manifest",
			Status:  Pass,
			Message: "FEDERATION.md not present (single topology)",
		})
		return g
	}

	fed, err := project.ReadFederation(deps.Root)
	if err != nil {
		g.Results = append(g.Results, CheckResult{
			Name:    "federation-manifest",
			Status:  Fail,
			Message: err.Error(),
		})
		return g
	}

	g.Results = append(g.Results, CheckResult{
		Name:    "federation-manifest",
		Status:  Pass,
		Message: "FEDERATION.md is valid",
	})

	// Soft check (#871): each domain's documentation lives at
	// docs/domains/<domain>/. A missing folder is a Warning, never a Fail —
	// domain docs are authored incrementally and must not block the manifest
	// from validating.
	for _, d := range fed.Domains {
		dir := filepath.Join(deps.Root, "docs", "domains", d.Name)
		if info, statErr := os.Stat(dir); statErr != nil || !info.IsDir() {
			g.Results = append(g.Results, CheckResult{
				Name:        fmt.Sprintf("federation-manifest:domain-docs:%s", d.Name),
				Status:      Warning,
				Message:     fmt.Sprintf("domain %q has no docs/domains/%s/ directory yet", d.Name, d.Name),
				Remediation: fmt.Sprintf("add docs/domains/%s/ on the control plane when domain docs are ready", d.Name),
			})
		}
	}

	return g
}

// checkFederationProjectSync validates that every repo listed in FEDERATION.md
// is linked to the federation's GitHub Project, and flags project-linked repos
// that are absent from the manifest. It is active only when FEDERATION.md is
// present; single-topology repos receive a single Pass result (guard AC-4).
//
// Result name conventions:
//   - "federation-sync"                      — guards (single Pass or Warning)
//   - "federation-sync:not-linked:<owner/repo>" — AC-1: linked to project missing
//   - "federation-sync:inaccessible:<owner/repo>" — AC-3: repo not reachable
//   - "federation-sync:unlisted:<owner/repo>"    — AC-2: linked but not in manifest
//   - "federation-sync:linked:<owner/repo>"      — AC-1 pass: linked OK
//
// The "not-linked" result stores the repo node ID in Data (string) so the
// repair pass can link the repo without a redundant re-query.
func checkFederationProjectSync(deps CheckDeps) Group {
	g := Group{Name: "Federation project sync"}

	// Guard AC-4: no FEDERATION.md → skip silently (single topology).
	if !project.IsFederationRepo(deps.Root) {
		g.Results = append(g.Results, CheckResult{
			Name:    "federation-sync",
			Status:  Pass,
			Message: "FEDERATION.md not present — federation sync check skipped",
		})
		return g
	}

	// Guard: invalid manifest → skip; checkFederationManifest owns parse errors.
	fed, err := project.ReadFederation(deps.Root)
	if err != nil {
		g.Results = append(g.Results, CheckResult{
			Name:    "federation-sync",
			Status:  Warning,
			Message: "federation project sync skipped — manifest invalid (see Federation manifest check)",
		})
		return g
	}

	// Guard: project not configured.
	projectID := strings.TrimSpace(deps.ProjectID)
	if projectID == "" {
		g.Results = append(g.Results, CheckResult{
			Name:    "federation-sync",
			Status:  Warning,
			Message: "federation project sync skipped — AGENTIC_PROJECT_ID not configured",
		})
		return g
	}

	// Guard: no GraphQL client.
	if deps.FetchLinkedRepos == nil || deps.FetchOwnerAndRepoIDs == nil {
		g.Results = append(g.Results, CheckResult{
			Name:    "federation-sync",
			Status:  Warning,
			Message: "federation project sync skipped — no GraphQL client",
		})
		return g
	}

	// Fetch repos currently linked to the project.
	linked, err := deps.FetchLinkedRepos(projectID)
	if err != nil {
		g.Results = append(g.Results, CheckResult{
			Name:    "federation-sync",
			Status:  Warning,
			Message: fmt.Sprintf("federation project sync skipped — could not fetch linked repos: %v", err),
		})
		return g
	}

	// Build case-insensitive lookup sets.
	linkedSet := make(map[string]bool, len(linked))
	for _, r := range linked {
		linkedSet[strings.ToLower(r.NameWithOwner)] = true
	}
	manifestRepos := fed.AllRepos()
	manifestSet := make(map[string]bool, len(manifestRepos))
	for _, r := range manifestRepos {
		manifestSet[strings.ToLower(r.Name)] = true
	}

	// Per-manifest-repo checks: reachability (AC-3) then link status (AC-1).
	for _, repo := range manifestRepos {
		parts := strings.SplitN(repo.Name, "/", 2)
		if len(parts) != 2 {
			continue // validated by ReadFederation; skip malformed entry defensively
		}
		owner, repoName := parts[0], parts[1]

		_, repoID, fetchErr := deps.FetchOwnerAndRepoIDs(owner, repoName)
		if fetchErr != nil {
			// AC-3: repo is not accessible via the API.
			g.Results = append(g.Results, CheckResult{
				Name:    fmt.Sprintf("federation-sync:inaccessible:%s", repo.Name),
				Status:  Fail,
				Message: fmt.Sprintf("manifest repo %s is not accessible: %v", repo.Name, fetchErr),
			})
			continue
		}

		if !linkedSet[strings.ToLower(repo.Name)] {
			// AC-1: accessible but not linked to the project.
			g.Results = append(g.Results, CheckResult{
				Name:        fmt.Sprintf("federation-sync:not-linked:%s", repo.Name),
				Status:      Fail,
				Message:     fmt.Sprintf("manifest repo %s is not linked to the federation project", repo.Name),
				Data:        repoID,
				Remediation: "run 'gh agentic repair'",
			})
		} else {
			g.Results = append(g.Results, CheckResult{
				Name:    fmt.Sprintf("federation-sync:linked:%s", repo.Name),
				Status:  Pass,
				Message: fmt.Sprintf("manifest repo %s is linked", repo.Name),
			})
		}
	}

	// AC-2: project-linked repos absent from the manifest.
	for _, r := range linked {
		if !manifestSet[strings.ToLower(r.NameWithOwner)] {
			g.Results = append(g.Results, CheckResult{
				Name:    fmt.Sprintf("federation-sync:unlisted:%s", r.NameWithOwner),
				Status:  Warning,
				Message: fmt.Sprintf("project-linked repo %s is not in FEDERATION.md — add it to the manifest or unlink it from the project", r.NameWithOwner),
			})
		}
	}

	return g
}

// checkLegacyFederationConfig detects remnants of the pre-#824 topology model.
// It runs unconditionally regardless of topology so repos that still carry
// the old variables or directory layout are flagged even when they have
// already adopted FEDERATION.md. Each legacy artefact becomes a Warning with
// a remediation command.
func checkLegacyFederationConfig(deps CheckDeps) Group {
	g := Group{Name: "Legacy federation config"}
	found := 0

	// AGENTIC_TOPOLOGY variable — was the old topology marker; replaced by
	// FEDERATION.md presence detection in Feature #824.
	if deps.Run != nil {
		if hit, _, _ := getRepoVariable(deps, "AGENTIC_TOPOLOGY"); hit {
			g.Results = append(g.Results, CheckResult{
				Name:        "legacy-topology",
				Status:      Warning,
				Message:     "AGENTIC_TOPOLOGY is set — no longer used; topology is now inferred from FEDERATION.md presence",
				Remediation: "gh variable delete AGENTIC_TOPOLOGY --repo " + deps.RepoFullName,
			})
			found++
		}
	}

	// AGENTIC_CONTROL_PLANE variable — was written by the old federated init
	// flow; the FEDERATION.md manifest supersedes it.
	if deps.Run != nil {
		if hit, _, _ := getRepoVariable(deps, "AGENTIC_CONTROL_PLANE"); hit {
			g.Results = append(g.Results, CheckResult{
				Name:        "legacy-control-plane",
				Status:      Warning,
				Message:     "AGENTIC_CONTROL_PLANE is set — no longer used; remove it to keep the repo config clean",
				Remediation: "gh variable delete AGENTIC_CONTROL_PLANE --repo " + deps.RepoFullName,
			})
			found++
		}
	}

	// .cp/ directory — was created by the old federated domain init; no longer
	// needed and should be removed.
	if _, err := os.Stat(filepath.Join(deps.Root, ".cp")); err == nil {
		g.Results = append(g.Results, CheckResult{
			Name:        "legacy-cp-dir",
			Status:      Warning,
			Message:     ".cp/ directory found — no longer used by the framework",
			Remediation: "rm -rf .cp/ && git rm -r --cached .cp/ 2>/dev/null || true",
		})
		found++
	}

	if found == 0 {
		g.Results = append(g.Results, CheckResult{
			Name:    "legacy-config",
			Status:  Pass,
			Message: "No legacy federation config found",
		})
	}

	return g
}

// getRepoVariable reads a repo-scoped Actions variable via `gh api`. It
// returns found=true when the variable exists, along with the raw gh output
// and error for the caller's failure-mode analysis.
//
// `gh api` is used instead of `gh variable get` deliberately (#844): the
// `gh variable` command group only shipped in gh 2.31, and distro-packaged
// gh on self-hosted runners can predate it. On such versions `gh variable
// get` fails with "unknown command" — an error that must not be mistaken
// for "variable missing". The api command exists in every gh version.
func getRepoVariable(deps CheckDeps, name string) (bool, string, error) {
	out, err := deps.Run("gh", "api", "repos/"+deps.RepoFullName+"/actions/variables/"+name)
	return err == nil && strings.TrimSpace(out) != "", out, err
}

// checkVariable checks if a GitHub variable exists at --repo scope.
//
// Feature #824: all variables are --repo scoped. The old org-scope fallback
// for shared names under federated topology has been removed.
//
// Failure-mode discipline (#844): only a clean HTTP 404 proves the variable
// is missing. A permission error or any other failure (network, unexpected
// gh error) is inconclusive and reported as "unable to check" — never as
// "not configured".
func checkVariable(deps CheckDeps, name string) CheckResult {
	if deps.Run == nil {
		return CheckResult{Name: name, Status: Warning, Message: name + " — unable to check (no run func)"}
	}

	hit, out, err := getRepoVariable(deps, name)

	// Distinguish an auth/permission failure from a genuine missing variable.
	// GitHub App tokens used in CI often lack the Actions:Read permission
	// required to read variables, producing a 403. Treat that as "unable to
	// check" rather than "not configured" so the check doesn't emit false
	// positives when the variable is set but the token can't read it.
	if !hit && err != nil && isPermissionError(out) {
		return CheckResult{
			Name: name, Status: Warning,
			Message: name + " — unable to check (token lacks variable-read permission)",
		}
	}

	// Any other non-404 failure is equally inconclusive.
	if !hit && err != nil && !isNotFoundError(out) {
		return CheckResult{
			Name: name, Status: Warning,
			Message: name + " — unable to check (gh api call failed)",
		}
	}

	if hit {
		return CheckResult{Name: name, Status: Pass, Message: name + " configured"}
	}

	// Variables with a known sensible default are not hard failures — the
	// workflows fall back to the same default, so the pipeline is still
	// runnable. Surface as a Warning so the human sees the suggestion but
	// `check` exits 0. Repair still offers an interactive prompt.
	if meta, ok := pendingDescriptions[name]; ok && meta.Default != "" {
		return CheckResult{
			Name: name, Status: Warning,
			Message:     fmt.Sprintf("%s not set — using default %q", name, meta.Default),
			Remediation: remediationSet("variable", name, deps),
		}
	}

	return CheckResult{
		Name: name, Status: Fail,
		Message:     name + " not configured",
		Remediation: remediationSet("variable", name, deps),
	}
}

// checkSecret checks if a GitHub secret exists at --repo scope.
//
// Feature #824: all secrets are --repo scoped. The old org-scope fallback
// for shared names under federated topology has been removed.
func checkSecret(deps CheckDeps, name string) CheckResult {
	if deps.Run == nil {
		return CheckResult{Name: name, Status: Warning, Message: name + " — unable to check (no run func)"}
	}

	out, err := deps.Run("gh", "secret", "list", "--repo", deps.RepoFullName)
	hit := err == nil && containsSecretName(out, name)

	if hit {
		return CheckResult{Name: name, Status: Pass, Message: name + " configured"}
	}

	// Listing error — cannot tell whether the secret is configured.
	if err != nil {
		return CheckResult{
			Name: name, Status: Warning,
			Message: name + " — unable to verify",
		}
	}

	return CheckResult{
		Name: name, Status: Fail,
		Message:     name + " not configured",
		Remediation: remediationSet("secret", name, deps),
	}
}

// isPermissionError returns true when gh CLI output indicates an auth/permission
// failure (HTTP 403) rather than a genuine missing resource (HTTP 404). Used by
// checkVariable to avoid false-positive "not configured" results when the token
// lacks the Actions:Read permission required to read repo variables (common with
// GitHub App installation tokens in CI).
func isPermissionError(out string) bool {
	lower := strings.ToLower(out)
	return strings.Contains(lower, "403") ||
		strings.Contains(lower, "resource not accessible") ||
		strings.Contains(lower, "insufficient scopes") ||
		strings.Contains(lower, "must have admin rights") ||
		strings.Contains(lower, "forbidden")
}

// isNotFoundError returns true when gh CLI output indicates HTTP 404 — the
// only failure mode that proves a queried resource genuinely does not exist.
func isNotFoundError(out string) bool {
	lower := strings.ToLower(out)
	return strings.Contains(lower, "404") || strings.Contains(lower, "not found")
}

// containsVariableName returns true if the gh output from
// `gh variable list` contains a row for the given variable name. Each row
// starts with the variable name followed by whitespace, so a naive
// strings.Contains would false-positive on prefix collisions (e.g. AGENT
// vs AGENT_USER). Match on the first token of each line instead.
func containsVariableName(out, name string) bool {
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == name {
			return true
		}
	}
	return false
}

// containsSecretName returns true if the gh output from
// `gh secret list` contains a row for the given secret name. Same
// first-token match semantics as containsVariableName.
func containsSecretName(out, name string) bool {
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == name {
			return true
		}
	}
	return false
}

// remediationSet returns the `gh variable set` / `gh secret set` hint at
// --repo scope. Feature #824: all variables and secrets are repo-scoped.
// The kind argument is "variable" or "secret".
func remediationSet(kind, name string, deps CheckDeps) string {
	return fmt.Sprintf("gh %s set %s --repo %s", kind, name, deps.RepoFullName)
}

// checkLabels verifies that every label in requiredPipelineLabels exists in
// the repo. It uses `gh label list` via deps.Run so the check is purely
// network-driven (no local file state) and works identically across all
// topology variants.
//
// For each missing label, a Fail result is emitted whose Remediation is the
// exact `gh label create` command, so RepairPipeline can execute it
// automatically without user interaction.
func checkLabels(deps CheckDeps) Group {
	g := Group{Name: "Pipeline labels"}

	if deps.Run == nil {
		g.Results = append(g.Results, CheckResult{
			Name:    "pipeline-labels",
			Status:  Warning,
			Message: "pipeline labels — unable to check (no run func)",
		})
		return g
	}
	if deps.RepoFullName == "" {
		g.Results = append(g.Results, CheckResult{
			Name:    "pipeline-labels",
			Status:  Warning,
			Message: "pipeline labels — unable to check (repo not resolved)",
		})
		return g
	}

	out, err := deps.Run("gh", "label", "list", "--repo", deps.RepoFullName, "--limit", "200")
	if err != nil {
		g.Results = append(g.Results, CheckResult{
			Name:    "pipeline-labels",
			Status:  Warning,
			Message: fmt.Sprintf("pipeline labels — unable to list: %v", err),
		})
		return g
	}

	for _, lbl := range requiredPipelineLabels {
		if containsLabelName(out, lbl.Name) {
			g.Results = append(g.Results, CheckResult{
				Name:    "label:" + lbl.Name,
				Status:  Pass,
				Message: fmt.Sprintf("label %q present", lbl.Name),
			})
		} else {
			g.Results = append(g.Results, CheckResult{
				Name:    "label:" + lbl.Name,
				Status:  Fail,
				Message: fmt.Sprintf("label %q missing", lbl.Name),
				Remediation: fmt.Sprintf(
					"gh label create %q --repo %s --color %s --description %q",
					lbl.Name, deps.RepoFullName, lbl.Color, lbl.Description,
				),
			})
		}
	}

	return g
}

// containsLabelName returns true if the gh label list output contains a row
// whose first whitespace-delimited token matches name. The gh label list
// output has the label name as the first column, possibly followed by
// description and colour columns. Using the first-token match avoids
// false-positives when one label name is a prefix of another.
func containsLabelName(out, name string) bool {
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == name {
			return true
		}
	}
	return false
}

// fileExists returns true if the path exists and is a regular file.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// dirExists returns true if the path exists and is a directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// gitignoreContainsAI checks if .gitignore contains a .agents/ entry.
func gitignoreContainsAI(root string) bool {
	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == ".agents/" {
			return true
		}
	}
	return false
}
