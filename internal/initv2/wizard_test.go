package initv2

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eddiecarpenter/gh-agentic/internal/mount"
)

func fakeCloneFunc() mount.CloneFunc {
	return func(repoURL, tag, destDir string) error {
		files := map[string]string{
			"RULEBOOK.md":            "# Rules",
			"skills/session-init.md": "# Session Init",
			"standards/go.md":        "# Go",
		}
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return err
		}
		for path, content := range files {
			full := filepath.Join(destDir, path)
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
				return err
			}
		}
		return nil
	}
}

func fakeCollectConfig(cfg *InitConfig) func(w io.Writer, repo string) (*InitConfig, error) {
	return func(w io.Writer, repo string) (*InitConfig, error) {
		return cfg, nil
	}
}

func TestRun_Success(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, ".git"), 0o755)
	var buf bytes.Buffer
	var setCalls []string

	cfg := &InitConfig{
		Version:       "v2.0.0",
		Topology:      "Single",
		Stacks:        []string{"Go"},
		AgentUser:     "goose-agent",
		RunnerLabel:   "ubuntu-latest",
		GooseProvider: "anthropic",
		GooseModel:    "claude-sonnet-4-6",
		GooseAgentPAT: "ghp_test123",
		ClaudeCreds:   "base64creds",
		ProjectID:     "PVT_123",
		RepoFullName:  "owner/repo",
		Owner:         "owner",
		RepoName:      "repo",
	}

	deps := Deps{
		Run: func(name string, args ...string) (string, error) {
			setCalls = append(setCalls, name+" "+strings.Join(args, " "))
			return "", nil
		},
		Clone:  fakeCloneFunc(),
		CollectConfig: fakeCollectConfig(cfg),
	}

	err := Run(&buf, root, false, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Verify framework mounted.
	if _, err := os.Stat(filepath.Join(root, ".ai", "RULEBOOK.md")); os.IsNotExist(err) {
		t.Error(".ai/RULEBOOK.md should exist")
	}

	// Verify variables and secrets were set.
	if !strings.Contains(output, "AGENT_USER saved") {
		t.Error("expected AGENT_USER to be configured")
	}
	if !strings.Contains(output, "GOOSE_AGENT_PAT saved") {
		t.Error("expected GOOSE_AGENT_PAT to be configured")
	}
	if !strings.Contains(output, "CLAUDE_CREDENTIALS_JSON saved") {
		t.Error("expected CLAUDE_CREDENTIALS_JSON to be configured")
	}
	if !strings.Contains(output, "AGENTIC_PROJECT_ID saved") {
		t.Error("expected AGENTIC_PROJECT_ID to be configured")
	}

	// Verify success message.
	if !strings.Contains(output, "successfully initialised") {
		t.Error("expected success message")
	}

	// Verify gh commands were called.
	if len(setCalls) == 0 {
		t.Error("expected gh commands to be called for secret/variable setup")
	}
}

func TestRun_BlockedWithoutForce(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, ".git"), 0o755)
	var buf bytes.Buffer

	// Create existing .ai/ to simulate mounted framework.
	_ = os.MkdirAll(filepath.Join(root, ".ai"), 0o755)

	deps := Deps{
		Run:          func(name string, args ...string) (string, error) { return "", nil },
		Clone: fakeCloneFunc(),
		CollectConfig: fakeCollectConfig(&InitConfig{
			Version: "v2.0.0",
		}),
	}

	err := Run(&buf, root, false, deps)
	if err == nil {
		t.Fatal("expected error when .ai-version exists without --force")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error should mention --force, got: %v", err)
	}
}

func TestRun_ProceedsWithForce(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, ".git"), 0o755)
	var buf bytes.Buffer

	// Create existing .ai/ to simulate mounted framework.
	_ = os.MkdirAll(filepath.Join(root, ".ai"), 0o755)

	cfg := &InitConfig{
		Version:      "v2.0.0",
		RepoFullName: "owner/repo",
		Owner:        "owner",
		RepoName:     "repo",
	}

	deps := Deps{
		Run:           func(name string, args ...string) (string, error) { return "", nil },
		Clone:  fakeCloneFunc(),
		CollectConfig: fakeCollectConfig(cfg),
	}

	err := Run(&buf, root, true, deps)
	if err != nil {
		t.Fatalf("expected no error with --force, got: %v", err)
	}
}

func TestRun_NoRepoContext(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, ".git"), 0o755)
	var buf bytes.Buffer

	cfg := &InitConfig{
		Version: "v2.0.0",
		// No RepoFullName — skip remote configuration.
	}

	deps := Deps{
		Run:           func(name string, args ...string) (string, error) { return "", nil },
		Clone:  fakeCloneFunc(),
		CollectConfig: fakeCollectConfig(cfg),
	}

	err := Run(&buf, root, false, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigureRepo_SetsVariables(t *testing.T) {
	var buf bytes.Buffer
	var commands []string

	cfg := &InitConfig{
		RepoFullName:  "owner/repo",
		Owner:         "owner",
		RepoName:      "repo",
		AgentUser:     "goose",
		RunnerLabel:   "ubuntu-latest",
		GooseProvider: "anthropic",
		GooseModel:    "claude-sonnet-4-6",
		GooseAgentPAT: "ghp_test",
		ClaudeCreds:   "creds",
		ProjectID:     "PVT_123",
	}

	run := func(name string, args ...string) (string, error) {
		commands = append(commands, fmt.Sprintf("%s %s", name, strings.Join(args, " ")))
		return "", nil
	}

	err := configureRepo(&buf, cfg, run)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify variables were set.
	found := map[string]bool{}
	for _, cmd := range commands {
		if strings.Contains(cmd, "AGENT_USER") {
			found["AGENT_USER"] = true
		}
		if strings.Contains(cmd, "GOOSE_AGENT_PAT") {
			found["GOOSE_AGENT_PAT"] = true
		}
		if strings.Contains(cmd, "CLAUDE_CREDENTIALS_JSON") {
			found["CLAUDE_CREDENTIALS_JSON"] = true
		}
		if strings.Contains(cmd, "AGENTIC_PROJECT_ID") {
			found["AGENTIC_PROJECT_ID"] = true
		}
	}

	for _, expected := range []string{"AGENT_USER", "GOOSE_AGENT_PAT", "CLAUDE_CREDENTIALS_JSON", "AGENTIC_PROJECT_ID"} {
		if !found[expected] {
			t.Errorf("expected %s to be configured", expected)
		}
	}
}

func TestConfigureRepo_GrantsAccess(t *testing.T) {
	var buf bytes.Buffer
	var accessGranted bool

	cfg := &InitConfig{
		RepoFullName: "owner/repo",
		Owner:        "owner",
		RepoName:     "repo",
		AgentUser:    "goose-agent",
	}

	run := func(name string, args ...string) (string, error) {
		cmd := strings.Join(args, " ")
		if strings.Contains(cmd, "collaborators/goose-agent") {
			accessGranted = true
		}
		return "", nil
	}

	err := configureRepo(&buf, cfg, run)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !accessGranted {
		t.Error("expected collaborator access to be granted")
	}
}

func TestParseRepoFromURL_SSH(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{url: "git@github.com:owner/repo.git", want: "owner/repo"},
		{url: "git@github.com:owner/repo", want: "owner/repo"},
	}

	for _, tc := range tests {
		got := parseRepoFromURL(tc.url)
		if got != tc.want {
			t.Errorf("parseRepoFromURL(%q) = %q, want %q", tc.url, got, tc.want)
		}
	}
}

func TestParseRepoFromURL_HTTPS(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{url: "https://github.com/owner/repo.git", want: "owner/repo"},
		{url: "https://github.com/owner/repo", want: "owner/repo"},
	}

	for _, tc := range tests {
		got := parseRepoFromURL(tc.url)
		if got != tc.want {
			t.Errorf("parseRepoFromURL(%q) = %q, want %q", tc.url, got, tc.want)
		}
	}
}

func TestCheckAIVersionExists(t *testing.T) {
	root := t.TempDir()
	if CheckAIVersionExists(root) {
		t.Error("should return false for non-existent .ai-version")
	}

	_ = mount.WriteAIVersion(root, "v1.0.0")
	if !CheckAIVersionExists(root) {
		t.Error("should return true for existing .ai-version")
	}
}
