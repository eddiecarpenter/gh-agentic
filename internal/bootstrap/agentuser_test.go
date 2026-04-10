package bootstrap

import (
	"strings"
	"testing"
)

func TestResolveAgentUser_ValidConfig_ReturnsNil(t *testing.T) {
	cfg := &BootstrapConfig{
		Owner:          "acme-org",
		OwnerType:      OwnerTypeOrg,
		AgentUser:      "goose-agent",
		AgentUserScope: AgentUserScopeOrg,
	}

	err := ResolveAgentUser(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveAgentUser_RepoScopePersonalAccount_ReturnsNil(t *testing.T) {
	cfg := &BootstrapConfig{
		Owner:          "alice",
		OwnerType:      OwnerTypeUser,
		AgentUser:      "goose-agent",
		AgentUserScope: AgentUserScopeRepo,
	}

	err := ResolveAgentUser(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveAgentUser_OrgScopePersonalAccount_ReturnsError(t *testing.T) {
	cfg := &BootstrapConfig{
		Owner:          "alice",
		OwnerType:      OwnerTypeUser,
		AgentUser:      "goose-agent",
		AgentUserScope: AgentUserScopeOrg,
	}

	err := ResolveAgentUser(cfg)
	if err == nil {
		t.Fatal("expected error for org scope on personal account")
	}
	if !strings.Contains(err.Error(), "personal account") {
		t.Errorf("expected personal account error, got: %v", err)
	}
}

func TestResolveAgentUser_InvalidScope_ReturnsError(t *testing.T) {
	cfg := &BootstrapConfig{
		Owner:          "acme-org",
		OwnerType:      OwnerTypeOrg,
		AgentUser:      "goose-agent",
		AgentUserScope: "invalid",
	}

	err := ResolveAgentUser(cfg)
	if err == nil {
		t.Fatal("expected error for invalid scope")
	}
	if !strings.Contains(err.Error(), "invalid") {
		t.Errorf("expected invalid scope error, got: %v", err)
	}
}

func TestResolveAgentUser_EmptyAgentUser_ReturnsError(t *testing.T) {
	cfg := &BootstrapConfig{
		Owner:          "acme-org",
		OwnerType:      OwnerTypeOrg,
		AgentUser:      "",
		AgentUserScope: AgentUserScopeOrg,
	}

	err := ResolveAgentUser(cfg)
	if err == nil {
		t.Fatal("expected error for empty agent user")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("expected 'required' in error, got: %v", err)
	}
}

func TestResolveAgentUser_EmptyScope_ReturnsError(t *testing.T) {
	cfg := &BootstrapConfig{
		Owner:          "acme-org",
		OwnerType:      OwnerTypeOrg,
		AgentUser:      "goose-agent",
		AgentUserScope: "",
	}

	err := ResolveAgentUser(cfg)
	if err == nil {
		t.Fatal("expected error for empty scope")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("expected 'required' in error, got: %v", err)
	}
}
