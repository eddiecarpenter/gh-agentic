// Package init implements the interactive init wizard.
// It collects configuration interactively and calls mount internally.
package init

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"

	"github.com/eddiecarpenter/gh-agentic/internal/auth"
	"github.com/eddiecarpenter/gh-agentic/internal/mount"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// ErrAlreadyInitialised is returned when the framework is already mounted
// and --force was not passed. The error message has already been printed to w.
var ErrAlreadyInitialised = errors.New("already initialised")

// InitConfig holds the collected configuration from the wizard.
type InitConfig struct {
	Version       string
	Topology      string
	Stacks        []string
	RunnerLabel   string
	AgentProvider string
	AgentModel    string
	GooseAgentPAT string
	PipelinePAT   string
	ClaudeCreds   string
	ProjectID     string
	RepoFullName  string
	Owner         string
	RepoName      string
	OwnerType     string
}

// RunCommandFunc is a function type for running shell commands.
type RunCommandFunc = auth.RunCommandFunc

// Deps holds injectable dependencies for the init wizard.
type Deps struct {
	Run   RunCommandFunc
	Clone mount.CloneFunc
	// CollectConfig gathers configuration interactively (or from test injection).
	CollectConfig func(w io.Writer, repoFullName string) (*InitConfig, error)
}

// Run executes the init wizard.
// It requires a git repository with no existing .agents/ directory (unless --force).
func Run(w io.Writer, root string, force bool, deps Deps) error {
	// Must be inside a git repository.
	if _, err := os.Stat(filepath.Join(root, ".git")); os.IsNotExist(err) {
		return fmt.Errorf("not a git repository — run 'git init' and add a remote before running init")
	}

	// Block if .agents/ already exists (framework already mounted).
	if _, err := os.Stat(filepath.Join(root, ".agents")); err == nil && !force {
		fmt.Fprintf(w, "  %s  Framework already mounted at .agents/\n", ui.StatusWarning.Render("⚠"))
		fmt.Fprintf(w, "       → Run 'gh agentic upgrade <version>' to upgrade, or 'gh agentic init --force' to reinitialise\n\n")
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
	fmt.Fprintln(w, "  2. Run 'gh agentic check' to verify")

	return nil
}

// ConfigureRepo sets up GitHub secrets, variables, and collaborator access.
//
// All variables and secrets are written at --repo scope. The org-scope
// routing that existed under the old federated topology has been removed
// as part of Feature #824 — topology is now determined by FEDERATION.md
// presence, not by variable-level scoping.
func ConfigureRepo(w io.Writer, cfg *InitConfig, run RunCommandFunc) error {
	repo := cfg.RepoFullName
	if repo == "" {
		return nil // No repo context — skip remote configuration.
	}

	// Set variables — all at --repo scope.
	variables := map[string]string{
		"RUNNER_LABEL":   cfg.RunnerLabel,
		"AGENT_PROVIDER": cfg.AgentProvider,
		"AGENT_MODEL":    cfg.AgentModel,
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

	// Set secrets — all at --repo scope.
	if cfg.GooseAgentPAT != "" {
		if _, err := run("gh", "secret", "set", "PROJECT_PAT", "--body", cfg.GooseAgentPAT, "--repo", repo); err != nil {
			return fmt.Errorf("setting PROJECT_PAT: %w", err)
		}
		fmt.Fprintf(w, "  ✓ PROJECT_PAT saved as repository secret\n")
	}

	if cfg.PipelinePAT != "" {
		if _, err := run("gh", "secret", "set", "PIPELINE_PAT", "--body", cfg.PipelinePAT, "--repo", repo); err != nil {
			return fmt.Errorf("setting PIPELINE_PAT: %w", err)
		}
		fmt.Fprintf(w, "  ✓ PIPELINE_PAT saved as repository secret\n")
	}

	if cfg.ClaudeCreds != "" {
		if _, err := run("gh", "secret", "set", "CLAUDE_CREDENTIALS_JSON", "--body", cfg.ClaudeCreds, "--repo", repo); err != nil {
			return fmt.Errorf("setting CLAUDE_CREDENTIALS_JSON: %w", err)
		}
		fmt.Fprintf(w, "  ✓ CLAUDE_CREDENTIALS_JSON saved as repository secret\n")
	}

	// Set project ID variable at --repo scope (identity names are always repo-scoped).
	if cfg.ProjectID != "" {
		if _, err := run("gh", "variable", "set", "AGENTIC_PROJECT_ID", "--body", cfg.ProjectID, "--repo", repo); err != nil {
			return fmt.Errorf("setting AGENTIC_PROJECT_ID: %w", err)
		}
		fmt.Fprintf(w, "  ✓ AGENTIC_PROJECT_ID saved as repository variable\n")
	}

	return nil
}

// HuhConfirm is the production ConfirmFunc used by federated init. It
// wraps huh.NewConfirm so callers that want the real interactive prompt
// can wire it as cfg.Confirm without re-implementing the huh boilerplate.
// Tests pass their own ConfirmFunc and never call this.
func HuhConfirm(title, description string) (bool, error) {
	var confirmed bool
	form := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title(title).
			Description(description).
			Value(&confirmed),
	))
	if err := form.Run(); err != nil {
		return false, err
	}
	return confirmed, nil
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

// CheckAIVersionExists returns true if a framework mount is present at
// root — i.e. the .agents/ directory exists. The legacy flat .ai-version file
// was removed in #585; the presence of a mounted .agents/ is now the
// equivalent "already initialised?" signal.
func CheckAIVersionExists(root string) bool {
	info, err := os.Stat(filepath.Join(root, ".agents"))
	if err != nil {
		return false
	}
	return info.IsDir()
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
