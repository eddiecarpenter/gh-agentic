// Package init implements the interactive init wizard.
// It collects configuration interactively and calls mount internally.
package init

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"

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
	AgentProvider  string
	AgentModel     string
	GooseAgentPAT  string
	ClaudeCreds    string
	ProjectID      string
	RepoFullName   string
	Owner          string
	RepoName       string
	OwnerType      string
	// Confirm optionally gates the federated-visibility confirmation in
	// ConfigureRepo. When set and topology is any federated variant,
	// ConfigureRepo emits the org-visibility note and calls Confirm before
	// writing anything. On a No answer, the call returns nil and no
	// variable or secret is written. When nil, ConfigureRepo proceeds
	// without prompting — used by single-topology callers and tests that
	// do not exercise the federated-confirm path.
	Confirm ConfirmFunc
}

// ConfirmFunc prompts the user for a yes/no confirmation. Title is the
// primary question, message is the supplementary body shown beneath it.
// Returns true on Yes, false otherwise. Implementations may return an
// error for IO / cancellation failures — ConfigureRepo treats an error
// the same as a No (abort without writing).
type ConfirmFunc func(title, message string) (bool, error)

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
	fmt.Fprintln(w, "  2. Run 'gh agentic check' to verify")

	return nil
}

// ConfigureRepo sets up GitHub secrets, variables, and collaborator access.
//
// Under federated topology the shared names (AGENT_USER, RUNNER_LABEL,
// AGENT_PROVIDER, AGENT_MODEL, GOOSE_AGENT_PAT, CLAUDE_CREDENTIALS_JSON)
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

	// Under federated topology the shared values will land at the
	// organisation level — visible to every other federated control plane
	// in the same org. That is load-bearing information; surface it before
	// any write, and gate ConfigureRepo on an explicit yes when the caller
	// has wired a ConfirmFunc. Single topology is unchanged: no note, no
	// prompt, behaviour identical to pre-feature.
	if scope.IsFederatedTopology(topology) {
		fmt.Fprintf(w, "\n  %s\n\n", federatedOrgVisibilityNote(owner))
		if cfg.Confirm != nil {
			ok, err := cfg.Confirm(
				"Proceed with federated init?",
				"Shared variables and secrets will be written at organisation scope.",
			)
			if err != nil {
				fmt.Fprintf(w, "  init cancelled by user\n")
				return nil
			}
			if !ok {
				fmt.Fprintf(w, "  init cancelled by user\n")
				return nil
			}
		}
	}

	// Set variables.
	variables := map[string]string{
		"AGENT_USER":     cfg.AgentUser,
		"RUNNER_LABEL":   cfg.RunnerLabel,
		"AGENT_PROVIDER": cfg.AgentProvider,
		"AGENT_MODEL":    cfg.AgentModel,
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

// federatedOrgVisibilityNote returns the exact verbatim message that
// ConfigureRepo prints before writing anything under federated topology.
// The wording is fixed by the acceptance criteria for this feature —
// tests assert the full sentence, so any change here must update the
// corresponding test.
func federatedOrgVisibilityNote(org string) string {
	return fmt.Sprintf(
		"Shared variables and secrets will be stored at organisation '%s' and will be visible to any other federated control plane in the same organisation.",
		org,
	)
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
// root — i.e. the .ai/ directory exists. The legacy flat .ai-version file
// was removed in #585; the presence of a mounted .ai/ is now the
// equivalent "already initialised?" signal.
func CheckAIVersionExists(root string) bool {
	info, err := os.Stat(filepath.Join(root, ".ai"))
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
