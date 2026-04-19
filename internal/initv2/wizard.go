// Package initv2 implements the v2 init wizard that replaces bootstrap.
// It collects configuration interactively and calls mount internally.
package initv2

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/eddiecarpenter/gh-agentic/internal/auth"
	"github.com/eddiecarpenter/gh-agentic/internal/mount"
	"github.com/eddiecarpenter/gh-agentic/internal/scope"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// ErrAlreadyInitialised is returned when the framework is already mounted
// and --force was not passed. The error message has already been printed to w.
var ErrAlreadyInitialised = errors.New("already initialised")

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
type RunCommandFunc = auth.RunCommandFunc

// Deps holds injectable dependencies for the init wizard.
type Deps struct {
	Run          RunCommandFunc
	Clone mount.CloneFunc
	// CollectConfig gathers configuration interactively (or from test injection).
	CollectConfig func(w io.Writer, repoFullName string) (*InitConfig, error)
}

// Run executes the v2 init wizard.
// It requires a git repository with no existing .ai/ directory (unless --force).
func Run(w io.Writer, root string, force bool, deps Deps) error {
	// Must be inside a git repository.
	if _, err := os.Stat(filepath.Join(root, ".git")); os.IsNotExist(err) {
		return fmt.Errorf("not a git repository — run 'git init' and add a remote before running init")
	}

	// Block if .ai/ already exists (framework already mounted).
	if _, err := os.Stat(filepath.Join(root, ".ai")); err == nil && !force {
		fmt.Fprintf(w, "  %s  Framework already mounted at .ai/\n", ui.StatusWarning.Render("⚠"))
		fmt.Fprintf(w, "       → Run 'gh agentic mount <version>' to upgrade, or 'gh agentic init --force' to reinitialise\n\n")
		return ErrAlreadyInitialised
	}

	fmt.Fprintln(w, "Initialising AI-Native Delivery Framework")
	fmt.Fprintln(w)

	// Collect configuration.
	cfg, err := deps.CollectConfig(w, "")
	if err != nil {
		return fmt.Errorf("configuration: %w", err)
	}

	// Run first-time mount.
	if err := mount.RunFirstTime(w, root, cfg.Version, deps.Clone); err != nil {
		return fmt.Errorf("mount: %w", err)
	}

	// Configure secrets and variables.
	if err := ConfigureRepo(w, cfg, deps.Run); err != nil {
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

// ConfigureRepo sets up GitHub secrets, variables, and collaborator access.
//
// Under federated topology the shared names (AGENT_USER, RUNNER_LABEL,
// GOOSE_PROVIDER, GOOSE_MODEL, GOOSE_AGENT_PAT, CLAUDE_CREDENTIALS_JSON)
// are routed to the organisation level via `scope.ScopeFor`. Per-repo
// identity names (AGENTIC_PROJECT_ID, AGENTIC_TOPOLOGY, and so on) stay at
// `--repo`. Under single topology everything stays at `--repo` — the
// routing is identical to the pre-scope behaviour.
func ConfigureRepo(w io.Writer, cfg *InitConfig, run RunCommandFunc) error {
	repo := cfg.RepoFullName
	if repo == "" {
		return nil // No repo context — skip remote configuration.
	}

	// cfg.Topology carries the wizard's capitalised value ("Single" /
	// "Federated"); scope.ScopeFor expects the lowercase form.
	topology := strings.ToLower(cfg.Topology)
	owner := cfg.Owner

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
		flag, target := scope.ScopeFor(name, topology, owner, repo)
		if _, err := run("gh", "variable", "set", name, "--body", value, flag, target); err != nil {
			return fmt.Errorf("setting variable %s: %w", name, err)
		}
		fmt.Fprintf(w, "  ✓ %s saved as %s\n", name, describeScope(flag, "variable"))
	}

	// Set secrets.
	if cfg.GooseAgentPAT != "" {
		flag, target := scope.ScopeFor("GOOSE_AGENT_PAT", topology, owner, repo)
		if _, err := run("gh", "secret", "set", "GOOSE_AGENT_PAT", "--body", cfg.GooseAgentPAT, flag, target); err != nil {
			return fmt.Errorf("setting GOOSE_AGENT_PAT: %w", err)
		}
		fmt.Fprintf(w, "  ✓ GOOSE_AGENT_PAT saved as %s\n", describeScope(flag, "secret"))
	}

	if cfg.ClaudeCreds != "" {
		flag, target := scope.ScopeFor("CLAUDE_CREDENTIALS_JSON", topology, owner, repo)
		if _, err := run("gh", "secret", "set", "CLAUDE_CREDENTIALS_JSON", "--body", cfg.ClaudeCreds, flag, target); err != nil {
			return fmt.Errorf("setting CLAUDE_CREDENTIALS_JSON: %w", err)
		}
		fmt.Fprintf(w, "  ✓ CLAUDE_CREDENTIALS_JSON saved as %s\n", describeScope(flag, "secret"))
	}

	// Set project ID variable. Identity names always route to --repo
	// regardless of topology.
	if cfg.ProjectID != "" {
		flag, target := scope.ScopeFor("AGENTIC_PROJECT_ID", topology, owner, repo)
		if _, err := run("gh", "variable", "set", "AGENTIC_PROJECT_ID", "--body", cfg.ProjectID, flag, target); err != nil {
			return fmt.Errorf("setting AGENTIC_PROJECT_ID: %w", err)
		}
		fmt.Fprintf(w, "  ✓ AGENTIC_PROJECT_ID saved as %s\n", describeScope(flag, "variable"))
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

// describeScope turns the gh scope flag into a human-readable phrase for
// the "saved as ..." output line — preserving the pre-ScopeFor messaging
// where org-scoped values are clearly labelled as such.
func describeScope(flag, kind string) string {
	if flag == scope.ScopeFlagOrg {
		return "organisation " + kind
	}
	return "repository " + kind
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
