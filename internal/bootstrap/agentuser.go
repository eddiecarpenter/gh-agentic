package bootstrap

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// PromptFunc prompts the user for a text input and returns the value.
type PromptFunc func(prompt string) (string, error)

// ResolveAgentUser resolves the agent user configuration. If both AgentUser and
// AgentUserScope are already set (from CLI flags), it validates without prompting.
// Otherwise it detects org-level variables and prompts interactively.
func ResolveAgentUser(w io.Writer, cfg *BootstrapConfig, run RunCommandFunc, prompt PromptFunc) error {
	// If both flags are provided, just validate.
	if cfg.AgentUser != "" && cfg.AgentUserScope != "" {
		if cfg.AgentUserScope != AgentUserScopeOrg && cfg.AgentUserScope != AgentUserScopeRepo {
			return fmt.Errorf("invalid agent-user-scope %q — must be %q or %q", cfg.AgentUserScope, AgentUserScopeOrg, AgentUserScopeRepo)
		}
		if cfg.AgentUserScope == AgentUserScopeOrg && cfg.OwnerType == OwnerTypeUser {
			return fmt.Errorf("cannot set org-level variable — %s is a personal account, not an organisation", cfg.Owner)
		}
		return nil
	}

	// Detect existing org-level AGENT_USER if owner is an org.
	var orgValue string
	if cfg.OwnerType == OwnerTypeOrg {
		orgValue = detectOrgAgentUser(cfg.Owner, run)
	}

	// Interactive resolution.
	if cfg.AgentUser == "" {
		if orgValue != "" {
			// Existing org-level variable found — ask to reuse.
			fmt.Fprintf(w, "  Found existing AGENT_USER at org level: %s\n", orgValue)
			answer, err := prompt(fmt.Sprintf("Reuse %q? (yes to reuse, or enter a new username)", orgValue))
			if err != nil {
				return fmt.Errorf("prompt failed: %w", err)
			}
			answer = strings.TrimSpace(answer)
			if answer == "" || strings.EqualFold(answer, "yes") || strings.EqualFold(answer, "y") {
				cfg.AgentUser = orgValue
				cfg.AgentUserScope = AgentUserScopeOrg
				return nil
			}
			// User provided a different username.
			cfg.AgentUser = answer
		} else {
			// No existing variable — prompt for username.
			answer, err := prompt("Enter agent GitHub username")
			if err != nil {
				return fmt.Errorf("prompt failed: %w", err)
			}
			cfg.AgentUser = strings.TrimSpace(answer)
			if cfg.AgentUser == "" {
				return fmt.Errorf("agent username is required")
			}
		}
	}

	// Resolve scope if not already set.
	if cfg.AgentUserScope == "" {
		if cfg.OwnerType == OwnerTypeUser {
			// Personal account — only repo scope is valid.
			cfg.AgentUserScope = AgentUserScopeRepo
		} else {
			// Organisation — prompt for scope.
			answer, err := prompt("Set AGENT_USER at org or repo level? (org/repo)")
			if err != nil {
				return fmt.Errorf("prompt failed: %w", err)
			}
			scope := strings.TrimSpace(strings.ToLower(answer))
			if scope != AgentUserScopeOrg && scope != AgentUserScopeRepo {
				return fmt.Errorf("invalid scope %q — must be %q or %q", scope, AgentUserScopeOrg, AgentUserScopeRepo)
			}
			cfg.AgentUserScope = scope
		}
	}

	// Final validation.
	if cfg.AgentUserScope == AgentUserScopeOrg && cfg.OwnerType == OwnerTypeUser {
		return fmt.Errorf("cannot set org-level variable — %s is a personal account, not an organisation", cfg.Owner)
	}

	return nil
}

// agentUserVariable is used to unmarshal JSON output from `gh variable list`.
type agentUserVariable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// detectOrgAgentUser queries org-level GitHub Actions variables and returns the
// value of AGENT_USER if set, or empty string if not found.
func detectOrgAgentUser(owner string, run RunCommandFunc) string {
	out, err := run("gh", "variable", "list", "--org", owner, "--json", "name,value", "--limit", "100")
	if err != nil {
		return ""
	}

	var vars []agentUserVariable
	if jsonErr := json.Unmarshal([]byte(strings.TrimSpace(out)), &vars); jsonErr != nil {
		return ""
	}

	for _, v := range vars {
		if v.Name == "AGENT_USER" {
			return v.Value
		}
	}
	return ""
}
