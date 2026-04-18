package doctorv2

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/eddiecarpenter/gh-agentic/internal/auth"
	"github.com/eddiecarpenter/gh-agentic/internal/mount"
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
}

// checkGroupStep pairs a spinner label with a check function.
type checkGroupStep struct {
	label string
	fn    func(CheckDeps) Group
}

// checksForTopologyWithLabels returns the ordered list of labelled check steps.
func checksForTopologyWithLabels(deps CheckDeps) []checkGroupStep {
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

// checkFramework checks the v2 framework mount state.
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

		if version != "" && strings.Contains(string(data), "@"+version) {
			g.Results = append(g.Results, CheckResult{
				Name: wf, Status: Pass, Message: fmt.Sprintf("%s → @%s", wf, version),
			})
		} else if version != "" {
			g.Results = append(g.Results, CheckResult{
				Name: wf, Status: Fail,
				Message:     fmt.Sprintf("%s — version tag mismatch (expected @%s)", wf, version),
				Remediation: "Run 'gh agentic mount'",
			})
		} else {
			g.Results = append(g.Results, CheckResult{
				Name: wf, Status: Pass, Message: wf + " present",
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

// checkProjectAffiliation checks AGENTIC_PROJECT_ID and AGENTIC_TOPOLOGY for domain repos.
func checkProjectAffiliation(deps CheckDeps) Group {
	g := Group{Name: "Agentic project membership"}

	result := checkVariable(deps, "AGENTIC_PROJECT_ID")
	g.Results = append(g.Results, result)

	result = checkVariable(deps, "AGENTIC_TOPOLOGY")
	g.Results = append(g.Results, result)

	return g
}

// checkVariable checks if a GitHub variable exists.
func checkVariable(deps CheckDeps, name string) CheckResult {
	if deps.Run == nil {
		return CheckResult{Name: name, Status: Warning, Message: name + " — unable to check (no run func)"}
	}

	out, err := deps.Run("gh", "variable", "get", name, "--repo", deps.RepoFullName)
	if err != nil || strings.TrimSpace(out) == "" {
		return CheckResult{
			Name: name, Status: Fail,
			Message:     name + " not configured",
			Remediation: fmt.Sprintf("gh variable set %s --repo %s", name, deps.RepoFullName),
		}
	}

	return CheckResult{Name: name, Status: Pass, Message: name + " configured"}
}

// checkSecret checks if a GitHub secret exists.
func checkSecret(deps CheckDeps, name string) CheckResult {
	if deps.Run == nil {
		return CheckResult{Name: name, Status: Warning, Message: name + " — unable to check (no run func)"}
	}

	// gh secret list returns secrets — check if name is in the list.
	out, err := deps.Run("gh", "secret", "list", "--repo", deps.RepoFullName)
	if err != nil {
		return CheckResult{
			Name: name, Status: Warning,
			Message: name + " — unable to verify",
		}
	}

	if strings.Contains(out, name) {
		return CheckResult{Name: name, Status: Pass, Message: name + " configured"}
	}

	return CheckResult{
		Name: name, Status: Fail,
		Message:     name + " not configured",
		Remediation: fmt.Sprintf("gh secret set %s --repo %s", name, deps.RepoFullName),
	}
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
