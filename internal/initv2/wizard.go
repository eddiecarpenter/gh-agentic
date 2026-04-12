// Package initv2 implements the v2 init wizard that replaces bootstrap.
// It collects configuration interactively and calls mount internally.
package initv2

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
	"github.com/eddiecarpenter/gh-agentic/internal/mount"
)

// InitConfig holds the collected configuration from the wizard.
type InitConfig struct {
	Version        string
	Topology       string
	Stacks         []string
	AgentUser      string
	AgentUserScope string
	RunnerLabel    string
	GooseProvider  string
	GooseModel     string
	GooseAgentPAT  string
	ClaudeCreds    string
	ProjectID      string
	RepoFullName   string
	Owner          string
	RepoName       string
	OwnerType      string
}

// RunCommandFunc is a function type for running shell commands.
type RunCommandFunc = bootstrap.RunCommandFunc

// Deps holds injectable dependencies for the init wizard.
type Deps struct {
	Run          RunCommandFunc
	FetchTarball mount.FetchTarballFunc
	// CollectConfig gathers configuration interactively (or from test injection).
	CollectConfig func(w io.Writer, repoFullName string) (*InitConfig, error)
}

// Run executes the v2 init wizard.
// It checks for existing .ai-version (blocked without --force), collects
// configuration, calls mount, and configures secrets/variables.
func Run(w io.Writer, root string, force bool, deps Deps) error {
	// Check for existing .ai-version.
	_, aiVersionErr := mount.ReadAIVersion(root)
	if aiVersionErr == nil && !force {
		return fmt.Errorf("this repository already has an .ai-version file — use --force to reinitialise")
	}

	fmt.Fprintln(w, "Initialising AI-Native Delivery Framework")
	fmt.Fprintln(w)

	// Collect configuration.
	cfg, err := deps.CollectConfig(w, "")
	if err != nil {
		return fmt.Errorf("configuration: %w", err)
	}

	// Run first-time mount.
	if err := mount.RunFirstTime(w, root, cfg.Version, deps.FetchTarball); err != nil {
		return fmt.Errorf("mount: %w", err)
	}

	// Configure secrets and variables.
	if err := configureRepo(w, cfg, deps.Run); err != nil {
		return fmt.Errorf("configuration: %w", err)
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "AI-Native Delivery Framework successfully initialised.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Next steps:")
	fmt.Fprintln(w, "  1. Review and commit the generated files")
	fmt.Fprintln(w, "  2. Run 'gh agentic -v2 doctor' to verify")

	return nil
}

// configureRepo sets up GitHub secrets, variables, and collaborator access.
func configureRepo(w io.Writer, cfg *InitConfig, run RunCommandFunc) error {
	repo := cfg.RepoFullName
	if repo == "" {
		return nil // No repo context — skip remote configuration.
	}

	// Set variables.
	variables := map[string]string{
		"AGENT_USER":     cfg.AgentUser,
		"RUNNER_LABEL":   cfg.RunnerLabel,
		"GOOSE_PROVIDER": cfg.GooseProvider,
		"GOOSE_MODEL":    cfg.GooseModel,
	}

	for name, value := range variables {
		if value == "" {
			continue
		}
		if _, err := run("gh", "variable", "set", name, "--body", value, "--repo", repo); err != nil {
			return fmt.Errorf("setting variable %s: %w", name, err)
		}
		fmt.Fprintf(w, "  ✓ %s saved as repository variable\n", name)
	}

	// Set secrets.
	if cfg.GooseAgentPAT != "" {
		if _, err := run("gh", "secret", "set", "GOOSE_AGENT_PAT", "--body", cfg.GooseAgentPAT, "--repo", repo); err != nil {
			return fmt.Errorf("setting GOOSE_AGENT_PAT: %w", err)
		}
		fmt.Fprintln(w, "  ✓ GOOSE_AGENT_PAT saved as repository secret")
	}

	if cfg.ClaudeCreds != "" {
		if _, err := run("gh", "secret", "set", "CLAUDE_CREDENTIALS_JSON", "--body", cfg.ClaudeCreds, "--repo", repo); err != nil {
			return fmt.Errorf("setting CLAUDE_CREDENTIALS_JSON: %w", err)
		}
		fmt.Fprintln(w, "  ✓ CLAUDE_CREDENTIALS_JSON saved as repository secret")
	}

	// Set project ID variable.
	if cfg.ProjectID != "" {
		if _, err := run("gh", "variable", "set", "AGENTIC_PROJECT_ID", "--body", cfg.ProjectID, "--repo", repo); err != nil {
			return fmt.Errorf("setting AGENTIC_PROJECT_ID: %w", err)
		}
		fmt.Fprintln(w, "  ✓ AGENTIC_PROJECT_ID saved as repository variable")
	}

	// Grant agent user write access.
	if cfg.AgentUser != "" {
		if _, err := run("gh", "api", "--method", "PUT",
			fmt.Sprintf("repos/%s/collaborators/%s", repo, cfg.AgentUser),
			"-f", "permission=push"); err != nil {
			fmt.Fprintf(w, "  ⚠ Could not grant %s write access (may need manual action)\n", cfg.AgentUser)
		} else {
			fmt.Fprintf(w, "  ✓ %s granted write access to %s\n", cfg.AgentUser, cfg.RepoName)
		}
	}

	return nil
}

// DetectRepoFromRemote detects the repo full name from git remote.
func DetectRepoFromRemote(run RunCommandFunc) (string, error) {
	out, err := run("git", "remote", "get-url", "origin")
	if err != nil {
		return "", fmt.Errorf("no git remote found: %w", err)
	}
	return parseRepoFromURL(out), nil
}

// parseRepoFromURL extracts owner/repo from a git remote URL.
func parseRepoFromURL(url string) string {
	url = trimString(url)
	// SSH: git@github.com:owner/repo.git
	if idx := indexOf(url, ":"); idx >= 0 && !hasPrefix(url, "http") {
		path := url[idx+1:]
		path = trimSuffix(path, ".git")
		return path
	}
	// HTTPS: https://github.com/owner/repo.git
	parts := splitURL(url)
	if len(parts) >= 2 {
		repo := parts[len(parts)-1]
		owner := parts[len(parts)-2]
		repo = trimSuffix(repo, ".git")
		return owner + "/" + repo
	}
	return ""
}

// CheckAIVersionExists returns true if .ai-version exists in root.
func CheckAIVersionExists(root string) bool {
	_, err := os.Stat(filepath.Join(root, ".ai-version"))
	return err == nil
}

// Helper string functions to avoid importing strings for simple ops.
func trimString(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r' || s[len(s)-1] == ' ') {
		s = s[:len(s)-1]
	}
	for len(s) > 0 && (s[0] == '\n' || s[0] == '\r' || s[0] == ' ') {
		s = s[1:]
	}
	return s
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func trimSuffix(s, suffix string) string {
	if len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix {
		return s[:len(s)-len(suffix)]
	}
	return s
}

func splitURL(url string) []string {
	var parts []string
	current := ""
	for _, c := range url {
		if c == '/' {
			if current != "" {
				parts = append(parts, current)
			}
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}
