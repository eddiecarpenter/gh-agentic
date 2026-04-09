package bootstrap

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// ResolveAgentUser validates the agent user configuration. Both AgentUser and
// AgentUserScope must already be set (from the form or CLI flags). This function
// only validates — it does not prompt interactively. All interactive input
// collection is handled by the form (RunForm).
func ResolveAgentUser(w io.Writer, cfg *BootstrapConfig, run RunCommandFunc) error {
	// Validate that agent user is set.
	if cfg.AgentUser == "" {
		return fmt.Errorf("agent username is required")
	}

	// Validate scope.
	if cfg.AgentUserScope == "" {
		return fmt.Errorf("agent user scope is required")
	}
	if cfg.AgentUserScope != AgentUserScopeOrg && cfg.AgentUserScope != AgentUserScopeRepo {
		return fmt.Errorf("invalid agent-user-scope %q — must be %q or %q", cfg.AgentUserScope, AgentUserScopeOrg, AgentUserScopeRepo)
	}

	// Validate org scope is only used with org accounts.
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
