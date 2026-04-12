package doctorv2

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/eddiecarpenter/gh-agentic/internal/auth"
	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
	"github.com/eddiecarpenter/gh-agentic/internal/mount"
)

// CheckDeps holds injectable dependencies for check functions.
type CheckDeps struct {
	Root         string
	RepoFullName string
	Owner        string
	RepoName     string
	OwnerType    string
	Run          bootstrap.RunCommandFunc
	ReadCreds    auth.ReadCredentialsFunc
}

// RunAllChecks runs all v2 checks and returns a grouped Report.
func RunAllChecks(deps CheckDeps) *Report {
	report := &Report{}

	report.Groups = append(report.Groups, checkRepository(deps))
	report.Groups = append(report.Groups, checkFramework(deps))
	report.Groups = append(report.Groups, checkAgentFiles(deps))
	report.Groups = append(report.Groups, checkWorkflows(deps))
	report.Groups = append(report.Groups, checkVariablesAndSecrets(deps))

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
		v, err := mount.ReadAIVersion(deps.Root)
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
			Remediation: "Run 'gh agentic -v2 mount <version>'",
		})
	}

	// .ai-version present.
	v, err := mount.ReadAIVersion(deps.Root)
	if err == nil {
		g.Results = append(g.Results, CheckResult{
			Name: "ai-version", Status: Pass, Message: fmt.Sprintf(".ai-version present (%s)", v),
		})
	} else {
		g.Results = append(g.Results, CheckResult{
			Name: "ai-version", Status: Fail,
			Message:     ".ai-version not found",
			Remediation: "Run 'gh agentic -v2 mount <version>'",
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
			Remediation: "Run 'gh agentic -v2 mount <version>'",
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
			Remediation: "Run 'gh agentic -v2 mount <version>'",
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

	version, _ := mount.ReadAIVersion(deps.Root)
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
				Remediation: fmt.Sprintf("Run 'gh agentic -v2 mount %s'", version),
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
				Remediation: "Run 'gh agentic -v2 auth login'",
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
