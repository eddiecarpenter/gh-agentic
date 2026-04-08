package bootstrap

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// credentialsRelPath is the relative path from home to the Claude credentials file.
const credentialsRelPath = ".claude/.credentials.json"

// ReadFileFunc reads a file and returns its contents. Injected so tests can
// substitute a fake implementation without touching the filesystem.
type ReadFileFunc func(path string) ([]byte, error)

// DefaultReadFile is the production ReadFileFunc backed by os.ReadFile.
var DefaultReadFile ReadFileFunc = os.ReadFile

// UserHomeDirFunc returns the current user's home directory. Injected for testing.
type UserHomeDirFunc func() (string, error)

// DefaultUserHomeDir is the production UserHomeDirFunc backed by os.UserHomeDir.
var DefaultUserHomeDir UserHomeDirFunc = os.UserHomeDir

// SetPipelineVariables sets the RUNNER_LABEL, GOOSE_PROVIDER, and GOOSE_MODEL
// GitHub Actions repo variables. Each variable failure is non-fatal — a warning
// is logged and the step continues with the remaining variables.
func SetPipelineVariables(w io.Writer, cfg BootstrapConfig, state *StepState, run RunCommandFunc) error {
	vars := []struct {
		name  string
		value string
	}{
		{name: "RUNNER_LABEL", value: cfg.RunnerLabel},
		{name: "GOOSE_PROVIDER", value: cfg.GooseProvider},
		{name: "GOOSE_MODEL", value: cfg.GooseModel},
	}

	// Use --org for org-owned repos, --repo for personal repos.
	var scopeArgs []string
	var scopeLabel string
	if cfg.OwnerType == OwnerTypeOrg {
		scopeArgs = []string{"--org", cfg.Owner}
		scopeLabel = "org level"
	} else {
		fullName := cfg.Owner + "/" + state.RepoName
		scopeArgs = []string{"--repo", fullName}
		scopeLabel = "repo level"
	}

	for _, v := range vars {
		args := append([]string{"variable", "set", v.name, "--body", v.value}, scopeArgs...)
		out, err := run("gh", args...)
		if err != nil {
			fmt.Fprintln(w, "  "+ui.RenderWarning(fmt.Sprintf("Could not set %s variable: %s", v.name, strings.TrimSpace(out))))
			continue
		}
		fmt.Fprintln(w, "  "+ui.Muted.Render(fmt.Sprintf("· %s=%s set at %s", v.name, v.value, scopeLabel)))
	}

	return nil
}

// SetClaudeCredentials reads ~/.claude/.credentials.json, base64-encodes it,
// and sets it as the CLAUDE_CREDENTIALS_JSON repo secret. If the credentials
// file is not found, a warning and manual instructions are printed. All
// failures are non-fatal.
func SetClaudeCredentials(w io.Writer, cfg BootstrapConfig, state *StepState, run RunCommandFunc, readFile ReadFileFunc, homeDir UserHomeDirFunc) error {
	// Determine scope flag for gh secret set.
	var scopeFlag string
	if cfg.OwnerType == OwnerTypeOrg {
		scopeFlag = "--org " + cfg.Owner
	} else {
		scopeFlag = "--repo " + cfg.Owner + "/" + state.RepoName
	}

	home, err := homeDir()
	if err != nil {
		fmt.Fprintln(w, "  "+ui.RenderWarning("Could not determine home directory: "+err.Error()))
		printCredentialInstructions(w, scopeFlag)
		return nil
	}

	credPath := filepath.Join(home, credentialsRelPath)
	data, err := readFile(credPath)
	if err != nil {
		// File not found — try macOS Keychain as fallback.
		out, keychainErr := run("security", "find-generic-password", "-s", "Claude Code-credentials", "-w")
		out = strings.TrimSpace(out)
		if keychainErr != nil || out == "" {
			fmt.Fprintln(w, "  "+ui.RenderWarning("Claude credentials not found (tried "+credPath+" and macOS Keychain)"))
			printCredentialInstructions(w, scopeFlag)
			return nil
		}
		data = []byte(out)
	}

	// Validate Claude auth before pushing credentials.
	if authErr := ValidateClaudeAuth(run); authErr != nil {
		fmt.Fprintln(w, "  "+ui.RenderWarning("Claude authentication check failed — run 'claude auth login' to refresh your credentials"))
		fmt.Fprintln(w, "  "+ui.Muted.Render("  Skipping CLAUDE_CREDENTIALS_JSON secret — credentials may be stale"))
		return nil
	}

	encoded := base64.StdEncoding.EncodeToString(data)

	// Use --org for org-owned repos, --repo for personal repos.
	var ghArgs []string
	if cfg.OwnerType == OwnerTypeOrg {
		ghArgs = []string{"secret", "set", "CLAUDE_CREDENTIALS_JSON", "--body", encoded, "--org", cfg.Owner}
	} else {
		fullName := cfg.Owner + "/" + state.RepoName
		ghArgs = []string{"secret", "set", "CLAUDE_CREDENTIALS_JSON", "--body", encoded, "--repo", fullName}
	}

	out, runErr := run("gh", ghArgs...)
	if runErr != nil {
		fmt.Fprintln(w, "  "+ui.RenderWarning("Could not set CLAUDE_CREDENTIALS_JSON secret: "+strings.TrimSpace(out)))
		printCredentialInstructions(w, scopeFlag)
		return nil
	}

	state.CredentialsSet = true
	fmt.Fprintln(w, "  "+ui.Muted.Render("· CLAUDE_CREDENTIALS_JSON secret set"))
	fmt.Fprintln(w, "  "+ui.Muted.Render("  To renew manually:"))
	fmt.Fprintln(w, "  "+ui.Muted.Render(fmt.Sprintf("  Linux/Windows: base64 < ~/.claude/.credentials.json | gh secret set CLAUDE_CREDENTIALS_JSON --body - %s", scopeFlag)))
	fmt.Fprintln(w, "  "+ui.Muted.Render(fmt.Sprintf(`  macOS:         security find-generic-password -s "Claude Code-credentials" -w | base64 | gh secret set CLAUDE_CREDENTIALS_JSON --body - %s`, scopeFlag)))

	return nil
}

// printCredentialInstructions prints manual instructions for setting the
// CLAUDE_CREDENTIALS_JSON secret for both Linux/Windows and macOS.
// scopeFlag is the pre-formatted scope argument, e.g. "--org acme" or "--repo alice/repo".
func printCredentialInstructions(w io.Writer, scopeFlag string) {
	fmt.Fprintln(w, "  "+ui.Muted.Render("  To set manually:"))
	fmt.Fprintln(w, "  "+ui.Muted.Render(fmt.Sprintf("  Linux/Windows: base64 < ~/.claude/.credentials.json | gh secret set CLAUDE_CREDENTIALS_JSON --body - %s", scopeFlag)))
	fmt.Fprintln(w, "  "+ui.Muted.Render(fmt.Sprintf(`  macOS:         security find-generic-password -s "Claude Code-credentials" -w | base64 | gh secret set CLAUDE_CREDENTIALS_JSON --body - %s`, scopeFlag)))
}

// ValidateClaudeAuth verifies that the local Claude CLI can authenticate by
// running `claude -p "hi"`. Returns nil if auth succeeds, or a descriptive
// error instructing the user to run `claude auth login` if it fails.
func ValidateClaudeAuth(run RunCommandFunc) error {
	_, err := run("claude", "-p", "hi")
	if err != nil {
		return fmt.Errorf("Claude authentication failed — run 'claude auth login' to refresh your credentials: %w", err)
	}
	return nil
}

// ValidateAgentPAT checks whether the GOOSE_AGENT_PAT secret exists on the repo.
// If missing, a warning is printed with the URL to add it. This is purely
// informational — the function always returns nil.
func ValidateAgentPAT(w io.Writer, cfg BootstrapConfig, state *StepState, run RunCommandFunc) error {
	fullName := cfg.Owner + "/" + state.RepoName

	out, err := run("gh", "secret", "list", "--repo", fullName, "--json", "name")
	if err != nil {
		fmt.Fprintln(w, "  "+ui.RenderWarning("Could not list repo secrets: "+strings.TrimSpace(out)))
		return nil
	}

	var secrets []struct {
		Name string `json:"name"`
	}
	if jsonErr := json.Unmarshal([]byte(strings.TrimSpace(out)), &secrets); jsonErr != nil {
		fmt.Fprintln(w, "  "+ui.RenderWarning("Could not parse secret list"))
		return nil
	}

	for _, s := range secrets {
		if s.Name == "GOOSE_AGENT_PAT" {
			state.AgentPATFound = true
			fmt.Fprintln(w, "  "+ui.Muted.Render("· GOOSE_AGENT_PAT secret found"))
			return nil
		}
	}

	settingsURL := fmt.Sprintf("https://github.com/%s/settings/secrets/actions", fullName)
	fmt.Fprintln(w, "  "+ui.RenderWarning("GOOSE_AGENT_PAT secret not found — pipeline will not run until added."))
	fmt.Fprintln(w, "  "+ui.Muted.Render("  Add it at: "+settingsURL))

	return nil
}
