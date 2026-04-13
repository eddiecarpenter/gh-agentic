package initv2

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/huh"

	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
)

// fakeFormRun returns a FormRunFunc that sets bound values directly without
// rendering a terminal. The setValues callback is called once per form group
// to populate the form fields before submission.
func fakeFormRun(callCount *int, setValues func(callIndex int)) FormRunFunc {
	return func(f *huh.Form) error {
		idx := *callCount
		*callCount++
		if setValues != nil {
			setValues(idx)
		}
		return nil
	}
}

func TestCollectConfigInteractive_Success(t *testing.T) {
	var buf bytes.Buffer
	callCount := 0

	cfg := &InitConfig{}

	deps := FormDeps{
		RunForm: func(f *huh.Form) error {
			idx := callCount
			callCount++

			switch idx {
			case 0: // Phase 1: version + topology
				cfg.Version = "v2.0.0"
				cfg.Topology = "Single"
			case 1: // Phase 2: stacks + agent
				cfg.Stacks = []string{"Go"}
				cfg.AgentUser = "goose-agent"
			case 2: // Phase 3: pipeline config
				cfg.RunnerLabel = "ubuntu-latest"
				cfg.GooseProvider = "claude-code"
				cfg.GooseModel = "default"
			case 3: // Phase 4: credentials + project
				cfg.GooseAgentPAT = "ghp_test123"
				cfg.ClaudeCreds = "base64creds"
				cfg.ProjectID = "PVT_123"
			}
			return nil
		},
		RunCommand: func(name string, args ...string) (string, error) {
			if name == "git" && len(args) > 0 && args[0] == "remote" {
				return "git@github.com:owner/repo.git", nil
			}
			return "", nil
		},
		DetectOwnerType: func(owner string) (string, error) {
			return bootstrap.OwnerTypeUser, nil
		},
	}

	result, err := CollectConfigInteractive(&buf, "", deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The CollectConfigInteractive function populates cfg fields via form binding.
	// Since our fake sets values on the shared cfg, verify the returned result
	// has the auto-detected fields set.
	if result.RepoFullName != "owner/repo" {
		t.Errorf("expected RepoFullName 'owner/repo', got %q", result.RepoFullName)
	}
	if result.Owner != "owner" {
		t.Errorf("expected Owner 'owner', got %q", result.Owner)
	}
	if result.RepoName != "repo" {
		t.Errorf("expected RepoName 'repo', got %q", result.RepoName)
	}
	if result.OwnerType != bootstrap.OwnerTypeUser {
		t.Errorf("expected OwnerType %q, got %q", bootstrap.OwnerTypeUser, result.OwnerType)
	}

	output := buf.String()
	if !strings.Contains(output, "Detected repo") {
		t.Error("expected repo detection message in output")
	}
}

func TestCollectConfigInteractive_WithExplicitRepo(t *testing.T) {
	var buf bytes.Buffer
	callCount := 0

	cfg := &InitConfig{}

	deps := FormDeps{
		RunForm: func(f *huh.Form) error {
			idx := callCount
			callCount++
			switch idx {
			case 0:
				cfg.Version = "v2.0.0"
				cfg.Topology = "Single"
			case 1:
				cfg.Stacks = []string{"Go"}
				cfg.AgentUser = "agent"
			case 2:
				// defaults
			case 3:
				// empty creds
			}
			return nil
		},
		DetectOwnerType: func(owner string) (string, error) {
			return bootstrap.OwnerTypeOrg, nil
		},
	}

	result, err := CollectConfigInteractive(&buf, "acme/my-repo", deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Owner != "acme" {
		t.Errorf("expected Owner 'acme', got %q", result.Owner)
	}
	if result.RepoName != "my-repo" {
		t.Errorf("expected RepoName 'my-repo', got %q", result.RepoName)
	}
	if result.OwnerType != bootstrap.OwnerTypeOrg {
		t.Errorf("expected OwnerType %q, got %q", bootstrap.OwnerTypeOrg, result.OwnerType)
	}
}

func TestCollectConfigInteractive_OrgSetsAgentUserScope(t *testing.T) {
	var buf bytes.Buffer
	callCount := 0

	cfg := &InitConfig{}

	deps := FormDeps{
		RunForm: func(f *huh.Form) error {
			idx := callCount
			callCount++
			switch idx {
			case 0:
				cfg.Version = "v2.0.0"
				cfg.Topology = "Federated"
			case 1:
				cfg.Stacks = []string{"Go"}
				cfg.AgentUser = "agent"
				cfg.AgentUserScope = bootstrap.AgentUserScopeOrg
			case 2:
				// pipeline defaults
			case 3:
				// empty creds
			}
			return nil
		},
		DetectOwnerType: func(owner string) (string, error) {
			return bootstrap.OwnerTypeOrg, nil
		},
	}

	result, err := CollectConfigInteractive(&buf, "acme/repo", deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.AgentUserScope != bootstrap.AgentUserScopeOrg {
		t.Errorf("expected AgentUserScope %q for org, got %q", bootstrap.AgentUserScopeOrg, result.AgentUserScope)
	}
}

func TestCollectConfigInteractive_UserSetsAgentUserScopeToRepo(t *testing.T) {
	var buf bytes.Buffer
	callCount := 0

	cfg := &InitConfig{}

	deps := FormDeps{
		RunForm: func(f *huh.Form) error {
			callCount++
			switch callCount {
			case 1:
				cfg.Version = "v2.0.0"
				cfg.Topology = "Single"
			case 2:
				cfg.Stacks = []string{"Go"}
				cfg.AgentUser = "agent"
			case 3:
				// pipeline defaults
			case 4:
				// creds
			}
			return nil
		},
		DetectOwnerType: func(owner string) (string, error) {
			return bootstrap.OwnerTypeUser, nil
		},
	}

	result, err := CollectConfigInteractive(&buf, "alice/repo", deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.AgentUserScope != bootstrap.AgentUserScopeRepo {
		t.Errorf("expected AgentUserScope %q for user, got %q", bootstrap.AgentUserScopeRepo, result.AgentUserScope)
	}
}

func TestCollectConfigInteractive_NoRepoContext(t *testing.T) {
	var buf bytes.Buffer
	callCount := 0

	cfg := &InitConfig{}

	deps := FormDeps{
		RunForm: func(f *huh.Form) error {
			callCount++
			switch callCount {
			case 1:
				cfg.Version = "v2.0.0"
				cfg.Topology = "Single"
			case 2:
				cfg.Stacks = []string{"Go"}
				cfg.AgentUser = "agent"
			case 3:
				// pipeline defaults
			case 4:
				// creds
			}
			return nil
		},
		RunCommand: func(name string, args ...string) (string, error) {
			return "", fmt.Errorf("no remote")
		},
	}

	result, err := CollectConfigInteractive(&buf, "", deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.RepoFullName != "" {
		t.Errorf("expected empty RepoFullName, got %q", result.RepoFullName)
	}
	if result.OwnerType != bootstrap.OwnerTypeUser {
		t.Errorf("expected default OwnerType %q, got %q", bootstrap.OwnerTypeUser, result.OwnerType)
	}
}

func TestCollectConfigInteractive_FormError(t *testing.T) {
	var buf bytes.Buffer

	deps := FormDeps{
		RunForm: func(f *huh.Form) error {
			return fmt.Errorf("terminal not available")
		},
	}

	_, err := CollectConfigInteractive(&buf, "owner/repo", deps)
	if err == nil {
		t.Fatal("expected error when form fails")
	}
	if !strings.Contains(err.Error(), "terminal not available") {
		t.Errorf("expected form error, got: %v", err)
	}
}

func TestCollectConfigInteractive_PipelineDefaults(t *testing.T) {
	var buf bytes.Buffer
	callCount := 0

	cfg := &InitConfig{}

	deps := FormDeps{
		RunForm: func(f *huh.Form) error {
			callCount++
			switch callCount {
			case 1:
				cfg.Version = "v2.0.0"
				cfg.Topology = "Single"
			case 2:
				cfg.Stacks = []string{"Go"}
				cfg.AgentUser = "agent"
			case 3:
				// Don't set — verify defaults are pre-filled
			case 4:
				// creds
			}
			return nil
		},
		DetectOwnerType: func(owner string) (string, error) {
			return bootstrap.OwnerTypeUser, nil
		},
	}

	result, err := CollectConfigInteractive(&buf, "owner/repo", deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.RunnerLabel != bootstrap.DefaultRunnerLabel {
		t.Errorf("expected default runner label %q, got %q", bootstrap.DefaultRunnerLabel, result.RunnerLabel)
	}
	if result.GooseProvider != bootstrap.DefaultGooseProvider {
		t.Errorf("expected default provider %q, got %q", bootstrap.DefaultGooseProvider, result.GooseProvider)
	}
	if result.GooseModel != bootstrap.DefaultGooseModel {
		t.Errorf("expected default model %q, got %q", bootstrap.DefaultGooseModel, result.GooseModel)
	}
}

// --- Validation unit tests ---

func TestValidateVersion(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		{name: "valid version", input: "v2.0.0", wantErr: false},
		{name: "valid prerelease", input: "v2.0.0-rc1", wantErr: false},
		{name: "empty string", input: "", wantErr: true, errMsg: "required"},
		{name: "whitespace only", input: "   ", wantErr: true, errMsg: "required"},
		{name: "missing v prefix", input: "2.0.0", wantErr: true, errMsg: "must start with 'v'"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateVersion(tc.input)
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tc.wantErr && err != nil && tc.errMsg != "" {
				if !strings.Contains(err.Error(), tc.errMsg) {
					t.Errorf("expected error containing %q, got: %v", tc.errMsg, err)
				}
			}
		})
	}
}

func TestValidateStackSelection_Form(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		wantErr bool
	}{
		{name: "one stack", input: []string{"Go"}, wantErr: false},
		{name: "multiple stacks", input: []string{"Go", "Python"}, wantErr: false},
		{name: "empty selection", input: []string{}, wantErr: true},
		{name: "nil selection", input: nil, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateStackSelection(tc.input)
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateRequired(t *testing.T) {
	fn := validateRequired("test field")

	if err := fn("hello"); err != nil {
		t.Errorf("unexpected error for non-empty: %v", err)
	}
	if err := fn(""); err == nil {
		t.Error("expected error for empty string")
	}
	if err := fn("   "); err == nil {
		t.Error("expected error for whitespace-only string")
	}

	err := fn("")
	if err != nil && !strings.Contains(err.Error(), "test field") {
		t.Errorf("expected field name in error, got: %v", err)
	}
}

func TestSplitOwnerRepo(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantOwner string
		wantRepo  string
	}{
		{name: "normal", input: "owner/repo", wantOwner: "owner", wantRepo: "repo"},
		{name: "with dots", input: "org/my-repo.go", wantOwner: "org", wantRepo: "my-repo.go"},
		{name: "no slash", input: "owner", wantOwner: "owner", wantRepo: ""},
		{name: "empty", input: "", wantOwner: "", wantRepo: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := splitOwnerRepo(tc.input)
			if result[0] != tc.wantOwner {
				t.Errorf("owner: got %q, want %q", result[0], tc.wantOwner)
			}
			if result[1] != tc.wantRepo {
				t.Errorf("repo: got %q, want %q", result[1], tc.wantRepo)
			}
		})
	}
}

func TestStackOptions(t *testing.T) {
	opts := stackOptions()
	if len(opts) == 0 {
		t.Fatal("expected non-empty stack options")
	}

	// Verify Go is present.
	found := false
	for _, opt := range opts {
		if opt.Value == "Go" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'Go' to be in stack options")
	}
}

func TestCollectConfigInteractive_OwnerTypeDetectionFailure(t *testing.T) {
	var buf bytes.Buffer
	callCount := 0

	cfg := &InitConfig{}

	deps := FormDeps{
		RunForm: func(f *huh.Form) error {
			callCount++
			switch callCount {
			case 1:
				cfg.Version = "v2.0.0"
				cfg.Topology = "Single"
			case 2:
				cfg.Stacks = []string{"Go"}
				cfg.AgentUser = "agent"
			case 3:
				// defaults
			case 4:
				// creds
			}
			return nil
		},
		DetectOwnerType: func(owner string) (string, error) {
			return "", fmt.Errorf("API error")
		},
	}

	result, err := CollectConfigInteractive(&buf, "owner/repo", deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should fall back to User type.
	if result.OwnerType != bootstrap.OwnerTypeUser {
		t.Errorf("expected fallback to %q, got %q", bootstrap.OwnerTypeUser, result.OwnerType)
	}

	output := buf.String()
	if !strings.Contains(output, "Could not detect") {
		t.Error("expected warning about owner type detection failure")
	}
}
