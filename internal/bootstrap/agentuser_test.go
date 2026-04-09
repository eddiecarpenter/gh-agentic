package bootstrap

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestResolveAgentUser_ValidConfig_ReturnsNil(t *testing.T) {
	var buf bytes.Buffer
	cfg := &BootstrapConfig{
		Owner:          "acme-org",
		OwnerType:      OwnerTypeOrg,
		AgentUser:      "goose-agent",
		AgentUserScope: AgentUserScopeOrg,
	}
	fakeRun := func(name string, args ...string) (string, error) {
		return "", nil
	}

	err := ResolveAgentUser(&buf, cfg, fakeRun)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveAgentUser_RepoScopePersonalAccount_ReturnsNil(t *testing.T) {
	var buf bytes.Buffer
	cfg := &BootstrapConfig{
		Owner:          "alice",
		OwnerType:      OwnerTypeUser,
		AgentUser:      "goose-agent",
		AgentUserScope: AgentUserScopeRepo,
	}
	fakeRun := func(name string, args ...string) (string, error) {
		return "", nil
	}

	err := ResolveAgentUser(&buf, cfg, fakeRun)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveAgentUser_OrgScopePersonalAccount_ReturnsError(t *testing.T) {
	var buf bytes.Buffer
	cfg := &BootstrapConfig{
		Owner:          "alice",
		OwnerType:      OwnerTypeUser,
		AgentUser:      "goose-agent",
		AgentUserScope: AgentUserScopeOrg,
	}
	fakeRun := func(name string, args ...string) (string, error) {
		return "", nil
	}

	err := ResolveAgentUser(&buf, cfg, fakeRun)
	if err == nil {
		t.Fatal("expected error for org scope on personal account")
	}
	if !strings.Contains(err.Error(), "personal account") {
		t.Errorf("expected personal account error, got: %v", err)
	}
}

func TestResolveAgentUser_InvalidScope_ReturnsError(t *testing.T) {
	var buf bytes.Buffer
	cfg := &BootstrapConfig{
		Owner:          "acme-org",
		OwnerType:      OwnerTypeOrg,
		AgentUser:      "goose-agent",
		AgentUserScope: "invalid",
	}
	fakeRun := func(name string, args ...string) (string, error) {
		return "", nil
	}

	err := ResolveAgentUser(&buf, cfg, fakeRun)
	if err == nil {
		t.Fatal("expected error for invalid scope")
	}
	if !strings.Contains(err.Error(), "invalid") {
		t.Errorf("expected invalid scope error, got: %v", err)
	}
}

func TestResolveAgentUser_EmptyAgentUser_ReturnsError(t *testing.T) {
	var buf bytes.Buffer
	cfg := &BootstrapConfig{
		Owner:          "acme-org",
		OwnerType:      OwnerTypeOrg,
		AgentUser:      "",
		AgentUserScope: AgentUserScopeOrg,
	}
	fakeRun := func(name string, args ...string) (string, error) {
		return "", nil
	}

	err := ResolveAgentUser(&buf, cfg, fakeRun)
	if err == nil {
		t.Fatal("expected error for empty agent user")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("expected 'required' in error, got: %v", err)
	}
}

func TestResolveAgentUser_EmptyScope_ReturnsError(t *testing.T) {
	var buf bytes.Buffer
	cfg := &BootstrapConfig{
		Owner:          "acme-org",
		OwnerType:      OwnerTypeOrg,
		AgentUser:      "goose-agent",
		AgentUserScope: "",
	}
	fakeRun := func(name string, args ...string) (string, error) {
		return "", nil
	}

	err := ResolveAgentUser(&buf, cfg, fakeRun)
	if err == nil {
		t.Fatal("expected error for empty scope")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("expected 'required' in error, got: %v", err)
	}
}

func TestDetectOrgAgentUser_Found_ReturnsValue(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return `[{"name":"AGENT_USER","value":"goose-agent"},{"name":"OTHER","value":"x"}]`, nil
	}
	val := detectOrgAgentUser("acme-org", fakeRun)
	if val != "goose-agent" {
		t.Errorf("expected %q, got %q", "goose-agent", val)
	}
}

func TestDetectOrgAgentUser_NotFound_ReturnsEmpty(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return `[{"name":"OTHER","value":"x"}]`, nil
	}
	val := detectOrgAgentUser("acme-org", fakeRun)
	if val != "" {
		t.Errorf("expected empty, got %q", val)
	}
}

func TestDetectOrgAgentUser_Error_ReturnsEmpty(t *testing.T) {
	fakeRun := func(name string, args ...string) (string, error) {
		return "", fmt.Errorf("permission denied")
	}
	val := detectOrgAgentUser("acme-org", fakeRun)
	if val != "" {
		t.Errorf("expected empty, got %q", val)
	}
}
