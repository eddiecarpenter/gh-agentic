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

func TestSetPipelineVariables_CorrectArguments(t *testing.T) {
	cfg := BootstrapConfig{
		Owner:         "org-name",
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
		// Expected: gh variable set <name> --body <value> --repo org-name/my-project
		if call[0] != "gh" || call[1] != "variable" || call[2] != "set" {
			t.Errorf("call %d: expected 'gh variable set', got: %v", i, call[:3])
		}
		if call[3] != exp.varName {
			t.Errorf("call %d: expected variable name %q, got %q", i, exp.varName, call[3])
		}
		if call[5] != exp.varValue {
			t.Errorf("call %d: expected variable value %q, got %q", i, exp.varValue, call[5])
		}
		if call[7] != "org-name/my-project" {
			t.Errorf("call %d: expected repo 'org-name/my-project', got %q", i, call[7])
		}
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
	run := fakeRunFail("permission denied")

	var buf bytes.Buffer
	err := SetClaudeCredentials(&buf, cfg, state, run, readFile, homeDir)
	if err != nil {
		t.Fatalf("expected nil error (non-fatal), got: %v", err)
	}

	if state.CredentialsSet {
		t.Error("expected state.CredentialsSet to be false when secret set fails")
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
