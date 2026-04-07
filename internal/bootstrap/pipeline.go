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
	fullName := cfg.Owner + "/" + state.RepoName

	vars := []struct {
		name  string
		value string
	}{
		{name: "RUNNER_LABEL", value: cfg.RunnerLabel},
		{name: "GOOSE_PROVIDER", value: cfg.GooseProvider},
		{name: "GOOSE_MODEL", value: cfg.GooseModel},
	}

	for _, v := range vars {
		out, err := run("gh", "variable", "set", v.name, "--body", v.value, "--repo", fullName)
		if err != nil {
			fmt.Fprintln(w, "  "+ui.RenderWarning(fmt.Sprintf("Could not set %s variable: %s", v.name, strings.TrimSpace(out))))
			continue
		}
		fmt.Fprintln(w, "  "+ui.Muted.Render(fmt.Sprintf("· %s=%s set at repo level", v.name, v.value)))
	}

	return nil
}

// SetClaudeCredentials reads ~/.claude/.credentials.json, base64-encodes it,
// and sets it as the CLAUDE_CREDENTIALS_JSON repo secret. If the credentials
// file is not found, a warning and manual instructions are printed. All
// failures are non-fatal.
func SetClaudeCredentials(w io.Writer, cfg BootstrapConfig, state *StepState, run RunCommandFunc, readFile ReadFileFunc, homeDir UserHomeDirFunc) error {
	fullName := cfg.Owner + "/" + state.RepoName

	home, err := homeDir()
	if err != nil {
		fmt.Fprintln(w, "  "+ui.RenderWarning("Could not determine home directory: "+err.Error()))
		printCredentialInstructions(w, fullName)
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
			printCredentialInstructions(w, fullName)
			return nil
		}
		data = []byte(out)
	}

	encoded := base64.StdEncoding.EncodeToString(data)

	out, runErr := run("gh", "secret", "set", "CLAUDE_CREDENTIALS_JSON", "--body", encoded, "--repo", fullName)
	if runErr != nil {
		fmt.Fprintln(w, "  "+ui.RenderWarning("Could not set CLAUDE_CREDENTIALS_JSON secret: "+strings.TrimSpace(out)))
		printCredentialInstructions(w, fullName)
		return nil
	}

	state.CredentialsSet = true
	fmt.Fprintln(w, "  "+ui.Muted.Render("· CLAUDE_CREDENTIALS_JSON secret set"))
	fmt.Fprintln(w, "  "+ui.Muted.Render("  To renew manually:"))
	fmt.Fprintln(w, "  "+ui.Muted.Render(fmt.Sprintf("  Linux/Windows: base64 < ~/.claude/.credentials.json | gh secret set CLAUDE_CREDENTIALS_JSON --body - --repo %s", fullName)))
	fmt.Fprintln(w, "  "+ui.Muted.Render(fmt.Sprintf(`  macOS:         security find-generic-password -s "Claude Code-credentials" -w | base64 | gh secret set CLAUDE_CREDENTIALS_JSON --body - --repo %s`, fullName)))

	return nil
}

// printCredentialInstructions prints manual instructions for setting the
// CLAUDE_CREDENTIALS_JSON secret for both Linux/Windows and macOS.
func printCredentialInstructions(w io.Writer, fullName string) {
	fmt.Fprintln(w, "  "+ui.Muted.Render("  To set manually:"))
	fmt.Fprintln(w, "  "+ui.Muted.Render(fmt.Sprintf("  Linux/Windows: base64 < ~/.claude/.credentials.json | gh secret set CLAUDE_CREDENTIALS_JSON --body - --repo %s", fullName)))
	fmt.Fprintln(w, "  "+ui.Muted.Render(fmt.Sprintf(`  macOS:         security find-generic-password -s "Claude Code-credentials" -w | base64 | gh secret set CLAUDE_CREDENTIALS_JSON --body - --repo %s`, fullName)))
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
