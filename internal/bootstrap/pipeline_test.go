package bootstrap

import (
	"bytes"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

// --- SetPipelineVariables tests ---

func TestSetPipelineVariables_AllSucceed_SetsThreeVariables(t *testing.T) {
	cfg := BootstrapConfig{
		Owner:         "alice",
		RunnerLabel:   "ubuntu-latest",
		GooseProvider: "claude-code",
		GooseModel:    "default",
	}
	state := &StepState{RepoName: "my-project"}

	var calls []string
	run := func(name string, args ...string) (string, error) {
		calls = append(calls, strings.Join(append([]string{name}, args...), " "))
		return "", nil
	}

	var buf bytes.Buffer
	err := SetPipelineVariables(&buf, cfg, state, run)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all three gh variable set commands were called.
	wantVars := []string{"RUNNER_LABEL", "GOOSE_PROVIDER", "GOOSE_MODEL"}
	for _, v := range wantVars {
		found := false
		for _, call := range calls {
			if strings.Contains(call, v) && strings.Contains(call, "alice/my-project") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected gh variable set call for %s, calls: %v", v, calls)
		}
	}
}

func TestSetPipelineVariables_PartialFailure_ContinuesWithRemaining(t *testing.T) {
	cfg := BootstrapConfig{
		Owner:         "alice",
		RunnerLabel:   "ubuntu-latest",
		GooseProvider: "claude-code",
		GooseModel:    "default",
	}
	state := &StepState{RepoName: "my-project"}

	callCount := 0
	run := func(name string, args ...string) (string, error) {
		callCount++
		// Fail on the second call (GOOSE_PROVIDER).
		if callCount == 2 {
			return "permission denied", errors.New("permission denied")
		}
		return "", nil
	}

	var buf bytes.Buffer
	err := SetPipelineVariables(&buf, cfg, state, run)
	if err != nil {
		t.Fatalf("expected nil error (non-fatal), got: %v", err)
	}

	// All three calls should have been attempted.
	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}

	// Output should contain a warning for the failed variable.
	out := buf.String()
	if !strings.Contains(out, "GOOSE_PROVIDER") {
		t.Errorf("expected warning about GOOSE_PROVIDER in output, got: %s", out)
	}
}

func TestSetPipelineVariables_UserScope_CorrectArguments(t *testing.T) {
	cfg := BootstrapConfig{
		Owner:         "alice",
		OwnerType:     OwnerTypeUser,
		RunnerLabel:   "self-hosted",
		GooseProvider: "openai",
		GooseModel:    "gpt-4",
	}
	state := &StepState{RepoName: "my-project"}

	var calls [][]string
	run := func(name string, args ...string) (string, error) {
		calls = append(calls, append([]string{name}, args...))
		return "", nil
	}

	var buf bytes.Buffer
	_ = SetPipelineVariables(&buf, cfg, state, run)

	expected := []struct {
		varName  string
		varValue string
	}{
		{"RUNNER_LABEL", "self-hosted"},
		{"GOOSE_PROVIDER", "openai"},
		{"GOOSE_MODEL", "gpt-4"},
	}

	if len(calls) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(calls))
	}

	for i, exp := range expected {
		call := calls[i]
		// Expected: gh variable set <name> --body <value> --repo alice/my-project
		if call[0] != "gh" || call[1] != "variable" || call[2] != "set" {
			t.Errorf("call %d: expected 'gh variable set', got: %v", i, call[:3])
		}
		if call[3] != exp.varName {
			t.Errorf("call %d: expected variable name %q, got %q", i, exp.varName, call[3])
		}
		if call[5] != exp.varValue {
			t.Errorf("call %d: expected variable value %q, got %q", i, exp.varValue, call[5])
		}
		joined := strings.Join(call, " ")
		if !strings.Contains(joined, "--repo alice/my-project") {
			t.Errorf("call %d: expected --repo alice/my-project, got: %v", i, call)
		}
	}

	out := buf.String()
	if !strings.Contains(out, "repo level") {
		t.Errorf("expected 'repo level' in output, got: %s", out)
	}
}

func TestSetPipelineVariables_OrgScope_UsesOrgFlag(t *testing.T) {
	cfg := BootstrapConfig{
		Owner:         "acme-org",
		OwnerType:     OwnerTypeOrg,
		RunnerLabel:   "ubuntu-latest",
		GooseProvider: "claude-code",
		GooseModel:    "default",
	}
	state := &StepState{RepoName: "my-project"}

	var calls [][]string
	run := func(name string, args ...string) (string, error) {
		calls = append(calls, append([]string{name}, args...))
		return "", nil
	}

	var buf bytes.Buffer
	_ = SetPipelineVariables(&buf, cfg, state, run)

	if len(calls) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(calls))
	}

	for i, call := range calls {
		joined := strings.Join(call, " ")
		if !strings.Contains(joined, "--org acme-org") {
			t.Errorf("call %d: expected --org acme-org, got: %v", i, call)
		}
		if strings.Contains(joined, "--repo") {
			t.Errorf("call %d: expected no --repo flag for org scope, got: %v", i, call)
		}
	}

	out := buf.String()
	if !strings.Contains(out, "org level") {
		t.Errorf("expected 'org level' in output, got: %s", out)
	}
}

// --- SetClaudeCredentials tests ---

func TestSetClaudeCredentials_FileExists_SetsSecret(t *testing.T) {
	cfg := BootstrapConfig{Owner: "alice"}
	state := &StepState{RepoName: "my-project"}

	credContent := []byte(`{"key":"test-value"}`)
	readFile := func(path string) ([]byte, error) {
		return credContent, nil
	}
	homeDir := func() (string, error) { return "/home/test", nil }

	var secretBody string
	run := func(name string, args ...string) (string, error) {
		// Capture the body argument for gh secret set.
		for i, a := range args {
			if a == "--body" && i+1 < len(args) {
				secretBody = args[i+1]
			}
		}
		return "", nil
	}

	var buf bytes.Buffer
	err := SetClaudeCredentials(&buf, cfg, state, run, readFile, homeDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !state.CredentialsSet {
		t.Error("expected state.CredentialsSet to be true")
	}

	expectedEncoded := base64.StdEncoding.EncodeToString(credContent)
	if secretBody != expectedEncoded {
		t.Errorf("expected base64-encoded body %q, got %q", expectedEncoded, secretBody)
	}
}

func TestSetClaudeCredentials_FileMissing_WarnsAndContinues(t *testing.T) {
	cfg := BootstrapConfig{Owner: "alice"}
	state := &StepState{RepoName: "my-project"}

	readFile := func(path string) ([]byte, error) {
		return nil, errors.New("file not found")
	}
	homeDir := func() (string, error) { return "/home/test", nil }
	// Keychain returns empty — triggers warning path.
	run := fakeRunOK("")

	var buf bytes.Buffer
	err := SetClaudeCredentials(&buf, cfg, state, run, readFile, homeDir)
	if err != nil {
		t.Fatalf("expected nil error (non-fatal), got: %v", err)
	}

	if state.CredentialsSet {
		t.Error("expected state.CredentialsSet to be false when file is missing")
	}

	out := buf.String()
	if !strings.Contains(out, "credentials") {
		t.Errorf("expected credentials warning in output, got: %s", out)
	}
	if !strings.Contains(out, "gh secret set") {
		t.Errorf("expected manual instructions in output, got: %s", out)
	}
}

func TestSetClaudeCredentials_FileMissing_KeychainPresent_SetsSecret(t *testing.T) {
	cfg := BootstrapConfig{Owner: "alice"}
	state := &StepState{RepoName: "my-project"}

	readFile := func(path string) ([]byte, error) {
		return nil, errors.New("file not found")
	}
	homeDir := func() (string, error) { return "/home/test", nil }

	credContent := `{"token":"keychain-value"}`
	var secretBody string
	run := func(name string, args ...string) (string, error) {
		if name == "security" {
			return credContent, nil
		}
		for i, a := range args {
			if a == "--body" && i+1 < len(args) {
				secretBody = args[i+1]
			}
		}
		return "", nil
	}

	var buf bytes.Buffer
	err := SetClaudeCredentials(&buf, cfg, state, run, readFile, homeDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !state.CredentialsSet {
		t.Error("expected state.CredentialsSet to be true when keychain has credentials")
	}

	expectedEncoded := base64.StdEncoding.EncodeToString([]byte(credContent))
	if secretBody != expectedEncoded {
		t.Errorf("expected base64-encoded keychain content, got %q", secretBody)
	}
}

func TestSetClaudeCredentials_SecretSetFails_WarnsAndContinues(t *testing.T) {
	cfg := BootstrapConfig{Owner: "alice"}
	state := &StepState{RepoName: "my-project"}

	readFile := func(path string) ([]byte, error) {
		return []byte(`{"key":"value"}`), nil
	}
	homeDir := func() (string, error) { return "/home/test", nil }
	// Auth succeeds but gh secret set fails.
	run := func(name string, args ...string) (string, error) {
		if name == "claude" {
			return "Hello!", nil
		}
		return "permission denied", errors.New("permission denied")
	}

	var buf bytes.Buffer
	err := SetClaudeCredentials(&buf, cfg, state, run, readFile, homeDir)
	if err != nil {
		t.Fatalf("expected nil error (non-fatal), got: %v", err)
	}

	if state.CredentialsSet {
		t.Error("expected state.CredentialsSet to be false when secret set fails")
	}
}

func TestSetClaudeCredentials_AuthFails_SkipsSecretSet(t *testing.T) {
	cfg := BootstrapConfig{Owner: "alice"}
	state := &StepState{RepoName: "my-project"}

	readFile := func(path string) ([]byte, error) {
		return []byte(`{"key":"value"}`), nil
	}
	homeDir := func() (string, error) { return "/home/test", nil }

	var secretSetCalled bool
	run := func(name string, args ...string) (string, error) {
		if name == "claude" {
			return "", errors.New("auth required")
		}
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "secret set CLAUDE_CREDENTIALS_JSON") {
			secretSetCalled = true
		}
		return "", nil
	}

	var buf bytes.Buffer
	err := SetClaudeCredentials(&buf, cfg, state, run, readFile, homeDir)
	if err != nil {
		t.Fatalf("expected nil error (non-fatal), got: %v", err)
	}

	if state.CredentialsSet {
		t.Error("expected state.CredentialsSet to be false when auth fails")
	}
	if secretSetCalled {
		t.Error("expected gh secret set NOT to be called when auth fails")
	}

	out := buf.String()
	if !strings.Contains(out, "claude auth login") {
		t.Errorf("expected 'claude auth login' instruction in output, got: %s", out)
	}
}

func TestSetClaudeCredentials_AuthSucceeds_SetsSecret(t *testing.T) {
	cfg := BootstrapConfig{Owner: "alice"}
	state := &StepState{RepoName: "my-project"}

	credContent := []byte(`{"key":"test-value"}`)
	readFile := func(path string) ([]byte, error) {
		return credContent, nil
	}
	homeDir := func() (string, error) { return "/home/test", nil }

	var secretBody string
	run := func(name string, args ...string) (string, error) {
		if name == "claude" {
			return "Hello!", nil
		}
		for i, a := range args {
			if a == "--body" && i+1 < len(args) {
				secretBody = args[i+1]
			}
		}
		return "", nil
	}

	var buf bytes.Buffer
	err := SetClaudeCredentials(&buf, cfg, state, run, readFile, homeDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !state.CredentialsSet {
		t.Error("expected state.CredentialsSet to be true when auth succeeds")
	}

	expectedEncoded := base64.StdEncoding.EncodeToString(credContent)
	if secretBody != expectedEncoded {
		t.Errorf("expected base64-encoded body %q, got %q", expectedEncoded, secretBody)
	}
}

func TestSetClaudeCredentials_OrgScope_UsesOrgFlag(t *testing.T) {
	cfg := BootstrapConfig{Owner: "acme-org", OwnerType: OwnerTypeOrg}
	state := &StepState{RepoName: "my-project"}

	credContent := []byte(`{"key":"org-value"}`)
	readFile := func(path string) ([]byte, error) {
		return credContent, nil
	}
	homeDir := func() (string, error) { return "/home/test", nil }

	var ghArgs []string
	run := func(name string, args ...string) (string, error) {
		if name == "gh" {
			ghArgs = args
		}
		return "", nil
	}

	var buf bytes.Buffer
	err := SetClaudeCredentials(&buf, cfg, state, run, readFile, homeDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !state.CredentialsSet {
		t.Error("expected state.CredentialsSet to be true")
	}

	// Verify --org flag is used.
	joined := strings.Join(ghArgs, " ")
	if !strings.Contains(joined, "--org acme-org") {
		t.Errorf("expected --org acme-org in gh args, got: %v", ghArgs)
	}
	if strings.Contains(joined, "--repo") {
		t.Errorf("expected no --repo flag for org scope, got: %v", ghArgs)
	}

	// Verify renewal instructions use --org.
	out := buf.String()
	if !strings.Contains(out, "--org acme-org") {
		t.Errorf("expected --org acme-org in renewal instructions, got: %s", out)
	}
}

func TestSetClaudeCredentials_UserScope_UsesRepoFlag(t *testing.T) {
	cfg := BootstrapConfig{Owner: "alice", OwnerType: OwnerTypeUser}
	state := &StepState{RepoName: "my-project"}

	credContent := []byte(`{"key":"user-value"}`)
	readFile := func(path string) ([]byte, error) {
		return credContent, nil
	}
	homeDir := func() (string, error) { return "/home/test", nil }

	var ghArgs []string
	run := func(name string, args ...string) (string, error) {
		if name == "gh" {
			ghArgs = args
		}
		return "", nil
	}

	var buf bytes.Buffer
	err := SetClaudeCredentials(&buf, cfg, state, run, readFile, homeDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify --repo flag is used with owner/repo.
	joined := strings.Join(ghArgs, " ")
	if !strings.Contains(joined, "--repo alice/my-project") {
		t.Errorf("expected --repo alice/my-project in gh args, got: %v", ghArgs)
	}
	if strings.Contains(joined, "--org") {
		t.Errorf("expected no --org flag for user scope, got: %v", ghArgs)
	}

	// Verify renewal instructions use --repo.
	out := buf.String()
	if !strings.Contains(out, "--repo alice/my-project") {
		t.Errorf("expected --repo alice/my-project in renewal instructions, got: %s", out)
	}
}

func TestSetClaudeCredentials_OrgScope_ManualInstructions_UseOrg(t *testing.T) {
	cfg := BootstrapConfig{Owner: "acme-org", OwnerType: OwnerTypeOrg}
	state := &StepState{RepoName: "my-project"}

	readFile := func(path string) ([]byte, error) {
		return nil, errors.New("file not found")
	}
	homeDir := func() (string, error) { return "/home/test", nil }
	// Keychain returns empty — triggers manual instruction path.
	run := fakeRunOK("")

	var buf bytes.Buffer
	err := SetClaudeCredentials(&buf, cfg, state, run, readFile, homeDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "--org acme-org") {
		t.Errorf("expected --org acme-org in manual instructions, got: %s", out)
	}
	if strings.Contains(out, "--repo") {
		t.Errorf("expected no --repo in manual instructions for org scope, got: %s", out)
	}
}

// --- ValidateClaudeAuth tests ---

func TestValidateClaudeAuth_Success_ReturnsNil(t *testing.T) {
	run := fakeRunOK("Hello!")
	err := ValidateClaudeAuth(run)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestValidateClaudeAuth_Failure_ReturnsErrorWithInstruction(t *testing.T) {
	run := fakeRunFail("auth required")
	err := ValidateClaudeAuth(run)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "claude auth login") {
		t.Errorf("expected error to contain 'claude auth login', got: %s", err.Error())
	}
}

// --- ValidateAgentPAT tests ---

func TestValidateAgentPAT_PATPresent_SetsAgentPATFound(t *testing.T) {
	cfg := BootstrapConfig{Owner: "alice"}
	state := &StepState{RepoName: "my-project"}

	run := fakeRunOK(`[{"name":"GOOSE_AGENT_PAT"},{"name":"OTHER_SECRET"}]`)

	var buf bytes.Buffer
	err := ValidateAgentPAT(&buf, cfg, state, run)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !state.AgentPATFound {
		t.Error("expected state.AgentPATFound to be true")
	}
}

func TestValidateAgentPAT_PATMissing_WarnsWithURL(t *testing.T) {
	cfg := BootstrapConfig{Owner: "alice"}
	state := &StepState{RepoName: "my-project"}

	run := fakeRunOK(`[{"name":"OTHER_SECRET"}]`)

	var buf bytes.Buffer
	err := ValidateAgentPAT(&buf, cfg, state, run)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if state.AgentPATFound {
		t.Error("expected state.AgentPATFound to be false")
	}

	out := buf.String()
	if !strings.Contains(out, "GOOSE_AGENT_PAT") {
		t.Errorf("expected PAT warning in output, got: %s", out)
	}
	if !strings.Contains(out, "settings/secrets/actions") {
		t.Errorf("expected GitHub settings URL in output, got: %s", out)
	}
}

func TestValidateAgentPAT_ListFails_WarnsAndContinues(t *testing.T) {
	cfg := BootstrapConfig{Owner: "alice"}
	state := &StepState{RepoName: "my-project"}

	run := fakeRunFail("not authorized")

	var buf bytes.Buffer
	err := ValidateAgentPAT(&buf, cfg, state, run)
	if err != nil {
		t.Fatalf("expected nil error (non-fatal), got: %v", err)
	}

	if state.AgentPATFound {
		t.Error("expected state.AgentPATFound to be false on failure")
	}
}

func TestValidateAgentPAT_OrgScope_UsesOrgFlag(t *testing.T) {
	cfg := BootstrapConfig{Owner: "acme-org", OwnerType: OwnerTypeOrg}
	state := &StepState{RepoName: "my-project"}

	var capturedArgs []string
	run := func(name string, args ...string) (string, error) {
		capturedArgs = append([]string{name}, args...)
		return `[{"name":"GOOSE_AGENT_PAT"}]`, nil
	}

	var buf bytes.Buffer
	err := ValidateAgentPAT(&buf, cfg, state, run)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !state.AgentPATFound {
		t.Error("expected state.AgentPATFound to be true")
	}

	joined := strings.Join(capturedArgs, " ")
	if !strings.Contains(joined, "--org acme-org") {
		t.Errorf("expected --org acme-org in args, got: %v", capturedArgs)
	}
	if strings.Contains(joined, "--repo") {
		t.Errorf("expected no --repo flag for org scope, got: %v", capturedArgs)
	}
}

func TestValidateAgentPAT_UserScope_UsesRepoFlag(t *testing.T) {
	cfg := BootstrapConfig{Owner: "alice", OwnerType: OwnerTypeUser}
	state := &StepState{RepoName: "my-project"}

	var capturedArgs []string
	run := func(name string, args ...string) (string, error) {
		capturedArgs = append([]string{name}, args...)
		return `[{"name":"GOOSE_AGENT_PAT"}]`, nil
	}

	var buf bytes.Buffer
	err := ValidateAgentPAT(&buf, cfg, state, run)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	joined := strings.Join(capturedArgs, " ")
	if !strings.Contains(joined, "--repo alice/my-project") {
		t.Errorf("expected --repo alice/my-project in args, got: %v", capturedArgs)
	}
	if strings.Contains(joined, "--org") {
		t.Errorf("expected no --org flag for user scope, got: %v", capturedArgs)
	}
}

func TestValidateAgentPAT_OrgScope_PATMissing_ShowsOrgSettingsURL(t *testing.T) {
	cfg := BootstrapConfig{Owner: "acme-org", OwnerType: OwnerTypeOrg}
	state := &StepState{RepoName: "my-project"}

	run := fakeRunOK(`[{"name":"OTHER_SECRET"}]`)

	var buf bytes.Buffer
	err := ValidateAgentPAT(&buf, cfg, state, run)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "organizations/acme-org/settings/secrets/actions") {
		t.Errorf("expected org settings URL in output, got: %s", out)
	}
}

func TestValidateAgentPAT_UserScope_PATMissing_ShowsRepoSettingsURL(t *testing.T) {
	cfg := BootstrapConfig{Owner: "alice", OwnerType: OwnerTypeUser}
	state := &StepState{RepoName: "my-project"}

	run := fakeRunOK(`[{"name":"OTHER_SECRET"}]`)

	var buf bytes.Buffer
	err := ValidateAgentPAT(&buf, cfg, state, run)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "alice/my-project/settings/secrets/actions") {
		t.Errorf("expected repo settings URL in output, got: %s", out)
	}
}
