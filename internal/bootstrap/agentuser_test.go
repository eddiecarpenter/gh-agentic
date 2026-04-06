package bootstrap

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestResolveAgentUser_FlagsProvided_SkipsPrompts(t *testing.T) {
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
	promptCalled := false
	fakePrompt := func(prompt string) (string, error) {
		promptCalled = true
		return "", nil
	}

	err := ResolveAgentUser(&buf, cfg, fakeRun, fakePrompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if promptCalled {
		t.Error("expected no prompts when both flags are provided")
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

	err := ResolveAgentUser(&buf, cfg, fakeRun, nil)
	if err == nil {
		t.Fatal("expected error for org scope on personal account")
	}
	if !strings.Contains(err.Error(), "personal account") {
		t.Errorf("expected personal account error, got: %v", err)
	}
}

func TestResolveAgentUser_OrgVarExists_Reuse(t *testing.T) {
	var buf bytes.Buffer
	cfg := &BootstrapConfig{
		Owner:     "acme-org",
		OwnerType: OwnerTypeOrg,
	}
	fakeRun := func(name string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "variable list") && strings.Contains(joined, "--org") {
			return `[{"name":"AGENT_USER","value":"existing-agent"}]`, nil
		}
		return "", nil
	}
	fakePrompt := func(prompt string) (string, error) {
		// Reuse — answer "yes"
		return "yes", nil
	}

	err := ResolveAgentUser(&buf, cfg, fakeRun, fakePrompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AgentUser != "existing-agent" {
		t.Errorf("expected AgentUser=%q, got %q", "existing-agent", cfg.AgentUser)
	}
	if cfg.AgentUserScope != AgentUserScopeOrg {
		t.Errorf("expected AgentUserScope=%q, got %q", AgentUserScopeOrg, cfg.AgentUserScope)
	}
}

func TestResolveAgentUser_OrgVarExists_Override(t *testing.T) {
	var buf bytes.Buffer
	cfg := &BootstrapConfig{
		Owner:     "acme-org",
		OwnerType: OwnerTypeOrg,
	}
	fakeRun := func(name string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "variable list") && strings.Contains(joined, "--org") {
			return `[{"name":"AGENT_USER","value":"existing-agent"}]`, nil
		}
		return "", nil
	}
	promptCount := 0
	fakePrompt := func(prompt string) (string, error) {
		promptCount++
		if strings.Contains(prompt, "Reuse") {
			return "new-agent", nil // override with new username
		}
		if strings.Contains(prompt, "org or repo") {
			return "repo", nil
		}
		return "", nil
	}

	err := ResolveAgentUser(&buf, cfg, fakeRun, fakePrompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AgentUser != "new-agent" {
		t.Errorf("expected AgentUser=%q, got %q", "new-agent", cfg.AgentUser)
	}
	if cfg.AgentUserScope != AgentUserScopeRepo {
		t.Errorf("expected AgentUserScope=%q, got %q", AgentUserScopeRepo, cfg.AgentUserScope)
	}
}

func TestResolveAgentUser_NoOrgVar_PromptsForUsernameAndScope(t *testing.T) {
	var buf bytes.Buffer
	cfg := &BootstrapConfig{
		Owner:     "acme-org",
		OwnerType: OwnerTypeOrg,
	}
	fakeRun := func(name string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "variable list") && strings.Contains(joined, "--org") {
			return `[]`, nil
		}
		return "", nil
	}
	fakePrompt := func(prompt string) (string, error) {
		if strings.Contains(prompt, "username") {
			return "my-agent", nil
		}
		if strings.Contains(prompt, "org or repo") {
			return "org", nil
		}
		return "", nil
	}

	err := ResolveAgentUser(&buf, cfg, fakeRun, fakePrompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AgentUser != "my-agent" {
		t.Errorf("expected AgentUser=%q, got %q", "my-agent", cfg.AgentUser)
	}
	if cfg.AgentUserScope != AgentUserScopeOrg {
		t.Errorf("expected AgentUserScope=%q, got %q", AgentUserScopeOrg, cfg.AgentUserScope)
	}
}

func TestResolveAgentUser_PersonalAccount_DefaultsToRepoScope(t *testing.T) {
	var buf bytes.Buffer
	cfg := &BootstrapConfig{
		Owner:     "alice",
		OwnerType: OwnerTypeUser,
	}
	fakeRun := func(name string, args ...string) (string, error) {
		return "", fmt.Errorf("not an org")
	}
	fakePrompt := func(prompt string) (string, error) {
		if strings.Contains(prompt, "username") {
			return "my-agent", nil
		}
		return "", nil
	}

	err := ResolveAgentUser(&buf, cfg, fakeRun, fakePrompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AgentUser != "my-agent" {
		t.Errorf("expected AgentUser=%q, got %q", "my-agent", cfg.AgentUser)
	}
	if cfg.AgentUserScope != AgentUserScopeRepo {
		t.Errorf("expected AgentUserScope=%q, got %q", AgentUserScopeRepo, cfg.AgentUserScope)
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

	err := ResolveAgentUser(&buf, cfg, fakeRun, nil)
	if err == nil {
		t.Fatal("expected error for invalid scope")
	}
	if !strings.Contains(err.Error(), "invalid") {
		t.Errorf("expected invalid scope error, got: %v", err)
	}
}
