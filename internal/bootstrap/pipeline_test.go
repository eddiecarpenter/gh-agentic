package bootstrap

import (
	"bytes"
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
