package bootstrap

import (
	"fmt"
)

// ResolveAgentUser validates the agent user configuration. Both AgentUser and
// AgentUserScope must already be set (from the form or CLI flags). This function
// only validates — it does not prompt interactively. All interactive input
// collection is handled by the form (RunForm).
func ResolveAgentUser(cfg *BootstrapConfig) error {
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

