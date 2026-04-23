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
	"github.com/eddiecarpenter/gh-agentic/internal/scope"
)

// CheckDeps holds injectable dependencies for check functions.
type CheckDeps struct {
	Root         string
	RepoFullName string
	Owner        string
	RepoName     string
	OwnerType    string
	Topology     string // "single", "federated-cp", "federated-domain", "" (unknown)
	ProjectID    string // value of AGENTIC_PROJECT_ID if set
	Run          auth.RunCommandFunc
	ReadCreds    auth.ReadCredentialsFunc
	// FetchProjectTitle is used by checkProjectReachability to confirm the
	// configured AGENTIC_PROJECT_ID resolves via the GraphQL API. Tests
	// substitute a fake; production wires project.DefaultFetchProjectTitle.
	FetchProjectTitle project.FetchProjectTitleFunc
	// FrameworkSource signals that this repo IS the gh-agentic framework
	// source itself (detected by .ai being a symlink). When true, content-
	// layer checks that inspect a mounted .ai/ tree are replaced with a
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

// checksForTopologyWithLabels returns the ordered list of labelled check steps.
func checksForTopologyWithLabels(deps CheckDeps) []checkGroupStep {
	// On the gh-agentic framework source itself, mount-layer checks do
	// not apply (.ai is a symlink → ., not a tarball clone). Replace
	// them with a synthetic "skipped" group and preserve everything
	// else. See feature #619 §E.
	if deps.FrameworkSource {
		base := []checkGroupStep{
			{"Checking repository...", checkRepository},
			{"Skipping content-layer checks...", checkFrameworkSourceSkipped},
			{"Checking workflows...", checkWorkflows},
			{"Checking variables and secrets...", checkVariablesAndSecrets},
			{"Checking project reachability...", checkProjectReachability},
		}
		return base
	}

	base := []checkGroupStep{
		{"Checking repository...", checkRepository},
		{"Checking framework mount...", checkFramework},
		{"Checking agent files...", checkAgentFiles},
		{"Checking skill frontmatter and catalogue...", checkSkillFrontmatter},
	}
	if deps.Topology == "federated-domain" {
		base = append(base, checkGroupStep{"Checking agentic project membership...", checkProjectAffiliation})
	} else {
		base = append(base, checkGroupStep{"Checking workflows...", checkWorkflows})
		base = append(base, checkGroupStep{"Checking variables and secrets...", checkVariablesAndSecrets})
	}
	// Shadow-vars runs under every federated variant; single topology
	// gets a no-op result for uniformity (see checkShadowVars).
	if scope.IsFederatedTopology(deps.Topology) {
		base = append(base, checkGroupStep{"Checking for shadow values...", checkShadowVars})
	}
	// Project reachability applies to every topology — all of them consume the
	// GitHub Project board via `gh agentic status`. Added last so it lands at
	// the bottom of the report, near the "what the agent actually needs" view.
	base = append(base, checkGroupStep{"Checking project reachability...", checkProjectReachability})
	return base
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
	reason := "framework source (.ai is a symlink) — content-layer checks do not apply"
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

	// .ai/ mounted.
	aiDir := filepath.Join(deps.Root, ".ai")
	if dirExists(aiDir) && fileExists(filepath.Join(aiDir, "RULEBOOK.md")) {
		v, err := mount.ReadAIVersionFromGit(deps.Root)
		version := "unknown"
		if err == nil {
			version = v
		}
		g.Results = append(g.Results, CheckResult{
			Name: "ai-mounted", Status: Pass, Message: fmt.Sprintf(".ai/ mounted (%s)", version),
		})
	} else {
		g.Results = append(g.Results, CheckResult{
			Name: "ai-mounted", Status: Fail,
			Message:     ".ai/ not mounted",
			Remediation: "Run 'gh agentic mount'",
		})
	}

	// .ai-version present (from .git metadata).
	v, err := mount.ReadAIVersionFromGit(deps.Root)
	if err == nil {
		g.Results = append(g.Results, CheckResult{
			Name: "ai-version", Status: Pass, Message: fmt.Sprintf(".ai-version present (%s)", v),
		})
	} else {
		g.Results = append(g.Results, CheckResult{
			Name: "ai-version", Status: Fail,
			Message:     ".ai/ git metadata missing — framework may not be properly mounted",
			Remediation: "Run 'gh agentic mount'",
		})
	}

	// .ai/ in .gitignore.
	if gitignoreContainsAI(deps.Root) {
		g.Results = append(g.Results, CheckResult{
			Name: "gitignore", Status: Pass, Message: ".ai/ in .gitignore",
		})
	} else {
		g.Results = append(g.Results, CheckResult{
			Name: "gitignore", Status: Fail,
			Message:     ".ai/ not in .gitignore",
			Remediation: "Add '.ai/' to .gitignore",
		})
	}

	// Check key framework directories.
	for _, dir := range []string{"skills", "standards"} {
		path := filepath.Join(aiDir, dir)
		name := ".ai/" + dir + "/"
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
			Remediation: "Run 'gh agentic mount'",
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
			Remediation: "Run 'gh agentic mount'",
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
				Name: wf, Status: Warning, Message: wf + " not found",
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
				Name: wf, Status: Pass, Message: fmt.Sprintf("%s → @%s", wf, version),
			})
		default:
			g.Results = append(g.Results, CheckResult{
				Name: wf, Status: Fail,
				Message:     fmt.Sprintf("%s — version tag mismatch (expected @%s)", wf, version),
				Remediation: "Run 'gh agentic mount'",
			})
		}
	}

	return g
}

// checkVariablesAndSecrets checks GitHub variables and secrets.
func checkVariablesAndSecrets(deps CheckDeps) Group {
	g := Group{Name: "Variables & secrets"}

	// Check variables via gh CLI.
	variables := []string{"AGENT_USER", "RUNNER_LABEL", "GOOSE_PROVIDER", "GOOSE_MODEL"}
	for _, v := range variables {
		result := checkVariable(deps, v)
		g.Results = append(g.Results, result)
	}

	// Federated control plane also needs topology and framework version variables.
	if deps.Topology == "federated-cp" {
		for _, v := range []string{"AGENTIC_TOPOLOGY", "AGENTIC_FRAMEWORK_VERSION"} {
			result := checkVariable(deps, v)
			g.Results = append(g.Results, result)
		}
	}

	// Check secrets.
	secrets := []string{"GOOSE_AGENT_PAT"}
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

// checkProjectAffiliation reports whether the federated-domain repo has its
// agentic project membership configured. Values are trusted from the
// resolver (deps.ProjectID and deps.Topology) — the doctor does not re-read
// AGENTIC_* variables directly. The check's purpose is to surface a clear
// diagnostic when the domain repo is missing affiliation; the resolver
// already validated the variables on the way in.
//
// After feature #571 / task #585, AGENTIC_TOPOLOGY is no longer written
// automatically. Its presence is optional — the resolver infers topology
// from the project-linked-repo graph. If the variable IS set but agrees
// with what the resolver would compute, this check emits a Warning
// pointing to its now-redundant status so operators can remove the
// stopgap cleanly (the #571 non-goal for domain-repo cleanup).
func checkProjectAffiliation(deps CheckDeps) Group {
	g := Group{Name: "Agentic project membership"}

	if strings.TrimSpace(deps.ProjectID) == "" {
		g.Results = append(g.Results, CheckResult{
			Name: "AGENTIC_PROJECT_ID", Status: Fail,
			Message:     "AGENTIC_PROJECT_ID not configured",
			Remediation: remediationSet("variable", "AGENTIC_PROJECT_ID", deps),
		})
	} else {
		g.Results = append(g.Results, CheckResult{
			Name: "AGENTIC_PROJECT_ID", Status: Pass,
			Message: "AGENTIC_PROJECT_ID configured",
		})
	}

	// AGENTIC_TOPOLOGY is optional after #585 — a federated-domain repo
	// resolves correctly without it via the linked-repo signal. When it is
	// set but the resolver's output matches, flag it as a redundant
	// stopgap that can be deleted.
	g.Results = append(g.Results, checkTopologyStopgap(deps))

	return g
}

// checkTopologyStopgap returns a CheckResult describing the current state of
// the AGENTIC_TOPOLOGY variable relative to what the resolver would infer.
// Three outcomes:
//   - Variable absent: Pass ("AGENTIC_TOPOLOGY not set — resolver infers").
//   - Variable set, matches the resolver's inferred value: Warning ("redundant
//     stopgap — safe to delete").
//   - Variable set, disagrees with the resolver: Pass with an informational
//     message ("explicit override — honoured").
//
// deps.Run is consulted once to read the variable; if the run func is absent
// the check falls back to a Warning that the stopgap status cannot be
// determined. This keeps the scanner honest under fully-fake test harnesses.
func checkTopologyStopgap(deps CheckDeps) CheckResult {
	if deps.Run == nil {
		return CheckResult{
			Name:    "AGENTIC_TOPOLOGY",
			Status:  Warning,
			Message: "AGENTIC_TOPOLOGY stopgap status — unable to check (no run func)",
		}
	}
	out, err := deps.Run("gh", "variable", "get", "AGENTIC_TOPOLOGY", "--repo", deps.RepoFullName)
	val := strings.TrimSpace(out)
	if err != nil || val == "" {
		return CheckResult{
			Name:    "AGENTIC_TOPOLOGY",
			Status:  Pass,
			Message: "AGENTIC_TOPOLOGY not set — resolver infers topology from project graph",
		}
	}

	// Map the canonical deps.Topology ("single" / "federated-cp" /
	// "federated-domain") back to the variable's two legal values
	// ("single" / "federated"). If the variable agrees, it is a redundant
	// stopgap; otherwise treat it as an explicit override.
	inferred := ""
	switch deps.Topology {
	case "single":
		inferred = "single"
	case "federated-cp", "federated-domain":
		inferred = "federated"
	}
	if inferred != "" && val == inferred {
		return CheckResult{
			Name:    "AGENTIC_TOPOLOGY",
			Status:  Warning,
			Message: "AGENTIC_TOPOLOGY=" + val + " is redundant — the resolver infers the same value; safe to delete",
			Remediation: "gh variable delete AGENTIC_TOPOLOGY --repo " + deps.RepoFullName,
		}
	}
	return CheckResult{
		Name:    "AGENTIC_TOPOLOGY",
		Status:  Pass,
		Message: "AGENTIC_TOPOLOGY=" + val + " — explicit override honoured",
	}
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

// checkVariable checks if a GitHub variable exists.
//
// Under federated topology the shared names (AGENT_USER, RUNNER_LABEL,
// GOOSE_PROVIDER, GOOSE_MODEL) live at the organisation level and are not
// visible via `gh variable list --repo OWNER/REPO`. The check therefore
// consults the org scope for shared names under federated topology, treating
// a hit at either scope as "configured". Identity names (AGENTIC_*) are
// repo-scoped only under any topology.
//
// The remediation message references the authoritative target scope so the
// human knows where the missing value should actually be set.
func checkVariable(deps CheckDeps, name string) CheckResult {
	if deps.Run == nil {
		return CheckResult{Name: name, Status: Warning, Message: name + " — unable to check (no run func)"}
	}

	// Repo scope — always consulted.
	repoOut, repoErr := deps.Run("gh", "variable", "get", name, "--repo", deps.RepoFullName)
	repoHit := repoErr == nil && strings.TrimSpace(repoOut) != ""

	// Org scope — only consulted for shared names under federated topology.
	// Identity names stay repo-only even under federated.
	orgHit := false
	if shouldConsultOrg(name, deps.Topology) {
		orgOut, orgErr := deps.Run("gh", "variable", "list", "--org", deps.Owner)
		if orgErr == nil && containsVariableName(orgOut, name) {
			orgHit = true
		}
	}

	if repoHit || orgHit {
		return CheckResult{Name: name, Status: Pass, Message: name + " configured"}
	}

	return CheckResult{
		Name: name, Status: Fail,
		Message:     name + " not configured",
		Remediation: remediationSet("variable", name, deps),
	}
}

// checkSecret checks if a GitHub secret exists.
//
// Under federated topology the shared secret names (GOOSE_AGENT_PAT,
// CLAUDE_CREDENTIALS_JSON) live at the organisation level and are not
// visible via `gh secret list --repo OWNER/REPO`. The check therefore
// consults the org scope for shared names under federated topology,
// treating a hit at either scope as "configured". Identity names are never
// looked up at org scope.
func checkSecret(deps CheckDeps, name string) CheckResult {
	if deps.Run == nil {
		return CheckResult{Name: name, Status: Warning, Message: name + " — unable to check (no run func)"}
	}

	// Repo scope — always consulted.
	repoOut, repoErr := deps.Run("gh", "secret", "list", "--repo", deps.RepoFullName)
	// A repo listing error is treated as inconclusive unless the org lookup
	// can confirm the secret for us below.
	repoHit := repoErr == nil && containsSecretName(repoOut, name)

	// Org scope — only consulted for shared names under federated topology.
	orgHit := false
	if shouldConsultOrg(name, deps.Topology) {
		orgOut, orgErr := deps.Run("gh", "secret", "list", "--org", deps.Owner)
		if orgErr == nil && containsSecretName(orgOut, name) {
			orgHit = true
		}
	}

	if repoHit || orgHit {
		return CheckResult{Name: name, Status: Pass, Message: name + " configured"}
	}

	// Neither scope returned a hit. If the repo listing errored AND the org
	// fallback did not confirm, surface the original soft-warning so the
	// human can distinguish "not configured" from "could not verify".
	if repoErr != nil && !orgHit {
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

// shouldConsultOrg reports whether the org scope should be queried for the
// given variable/secret name under the given topology. Shared names under
// any federated topology variant go to the org; everything else stays at
// the repo (and the caller must not waste an API call on the org list).
func shouldConsultOrg(name, topology string) bool {
	return scope.IsSharedName(name) && scope.IsFederatedTopology(topology)
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

// remediationSet returns the `gh variable set` / `gh secret set` hint
// pointing at the authoritative scope (org for shared names under
// federated, repo otherwise). The kind argument is "variable" or "secret".
func remediationSet(kind, name string, deps CheckDeps) string {
	flag, target := scope.ScopeFor(name, deps.Topology, deps.Owner, deps.RepoFullName)
	return fmt.Sprintf("gh %s set %s %s %s", kind, name, flag, target)
}

// --- shadow-vars check ---

// ShadowValue describes a shared variable or secret that is present at
// both org and repo scope on a federated repo — the write at --repo
// silently overrides the org-level value, which is the root cause of the
// federated-org-scoped-vars feature. Emitted by FindShadowValues and
// consumed by checkShadowVars' CheckResults and the repair pipeline.
type ShadowValue struct {
	// Name is the variable or secret name.
	Name string
	// Kind is "variable" or "secret".
	Kind string
	// DeleteCommand is the exact gh command that removes the shadow.
	DeleteCommand string
}

// shadowGhListQuery is a fake-friendly seam: the code path always calls
// deps.Run but the test harness can assert which list queries happened.

// FindShadowValues returns every shared name that currently exists at
// both `--org <owner>` and `--repo <owner/repo>` scope. Under single
// topology (or any topology that is not a federated variant) no
// consultation happens and the returned slice is nil — shadows are
// meaningless when everything lives at the repo.
//
// This helper is used both by checkShadowVars (to populate CheckResults)
// and by the repair pipeline (to drive the batch-delete confirmation
// prompt). Callers that need both the check output and the repair data
// may call this once and reuse the slice.
func FindShadowValues(deps CheckDeps) []ShadowValue {
	if !scope.IsFederatedTopology(deps.Topology) {
		return nil
	}
	if deps.Run == nil {
		return nil
	}

	// Query all four lists exactly once. On error we treat the listing as
	// empty — if the org listing errors we cannot prove a shadow exists
	// (which is the safer default than false-positive Fail).
	varRepoOut, _ := deps.Run("gh", "variable", "list", "--repo", deps.RepoFullName)
	varOrgOut, _ := deps.Run("gh", "variable", "list", "--org", deps.Owner)
	secRepoOut, _ := deps.Run("gh", "secret", "list", "--repo", deps.RepoFullName)
	secOrgOut, _ := deps.Run("gh", "secret", "list", "--org", deps.Owner)

	// Ordered iteration so tests get deterministic output without relying
	// on map iteration order. The names themselves come from scope's
	// canonical shared set; we iterate a stable slice here.
	shared := []string{
		"AGENT_USER",
		"RUNNER_LABEL",
		"GOOSE_PROVIDER",
		"GOOSE_MODEL",
		"GOOSE_AGENT_PAT",
		"CLAUDE_CREDENTIALS_JSON",
	}

	var shadows []ShadowValue
	for _, name := range shared {
		// Defensive: if scope stops treating a name as shared, skip it
		// here too. Keeps this helper honest if the canonical list drifts.
		if !scope.IsSharedName(name) {
			continue
		}
		// Variables are usually ambient; secrets occasionally carry the
		// same name under a different kind. The feature's contract is
		// "shared name under federated lives at org", so we check each
		// kind independently.
		varAtRepo := containsVariableName(varRepoOut, name)
		varAtOrg := containsVariableName(varOrgOut, name)
		if varAtRepo && varAtOrg {
			shadows = append(shadows, ShadowValue{
				Name:          name,
				Kind:          "variable",
				DeleteCommand: fmt.Sprintf("gh variable delete --repo %s %s", deps.RepoFullName, name),
			})
		}
		secAtRepo := containsSecretName(secRepoOut, name)
		secAtOrg := containsSecretName(secOrgOut, name)
		if secAtRepo && secAtOrg {
			shadows = append(shadows, ShadowValue{
				Name:          name,
				Kind:          "secret",
				DeleteCommand: fmt.Sprintf("gh secret delete --repo %s %s", deps.RepoFullName, name),
			})
		}
	}
	return shadows
}

// checkShadowVars reports whether any shared variable/secret exists at
// both org and repo scope under federated topology. Under single topology
// it returns an empty group — the concept does not apply.
//
// When shadows are present the group contains one Fail CheckResult per
// shadow, each with the exact delete command as its Remediation. The
// first such result also carries the structured []ShadowValue on
// CheckResult.Data so the repair pipeline can consume the list without
// re-querying.
func checkShadowVars(deps CheckDeps) Group {
	g := Group{Name: "Shadow values"}

	if !scope.IsFederatedTopology(deps.Topology) {
		// Not applicable under single or unknown topology — return a
		// benign informational result that matches the pattern used by
		// the other topology-conditional checks.
		g.Results = append(g.Results, CheckResult{
			Name:    "shadow-vars",
			Status:  Pass,
			Message: "shadow-vars — not applicable under single topology",
		})
		return g
	}

	shadows := FindShadowValues(deps)
	if len(shadows) == 0 {
		g.Results = append(g.Results, CheckResult{
			Name:    "shadow-vars",
			Status:  Pass,
			Message: "No repo-scoped shadow values found",
		})
		return g
	}

	// First result carries the structured data so repair can consume the
	// slice directly.
	g.Results = append(g.Results, CheckResult{
		Name:    "shadow-vars",
		Status:  Fail,
		Message: fmt.Sprintf("%d shadow value(s) detected — repo-scoped values override org inheritance", len(shadows)),
		Data:    shadows,
	})
	for _, s := range shadows {
		g.Results = append(g.Results, CheckResult{
			Name:        fmt.Sprintf("shadow-vars:%s:%s", s.Kind, s.Name),
			Status:      Fail,
			Message:     fmt.Sprintf("%s %q shadows the org-level value", s.Kind, s.Name),
			Remediation: s.DeleteCommand,
		})
	}
	return g
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

// gitignoreContainsAI checks if .gitignore contains a .ai/ entry.
func gitignoreContainsAI(root string) bool {
	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == ".ai/" {
			return true
		}
	}
	return false
}
