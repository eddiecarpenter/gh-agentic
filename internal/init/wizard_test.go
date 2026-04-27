package init

import (
	"bytes"
	"errors"
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
		AgentProvider: "anthropic",
		AgentModel:    "claude-sonnet-4-6",
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
		Clone:         fakeCloneFunc(),
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
	if !strings.Contains(output, "PROJECT_PAT saved") {
		t.Error("expected PROJECT_PAT to be configured")
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
		Run:   func(name string, args ...string) (string, error) { return "", nil },
		Clone: fakeCloneFunc(),
		CollectConfig: fakeCollectConfig(&InitConfig{
			Version: "v2.0.0",
		}),
	}

	err := Run(&buf, root, false, deps)
	if err == nil {
		t.Fatal("expected error when .ai/ exists without --force")
	}
	if !errors.Is(err, ErrAlreadyInitialised) {
		t.Errorf("expected ErrAlreadyInitialised, got: %v", err)
	}
	if !strings.Contains(buf.String(), "--force") {
		t.Errorf("expected --force hint in output, got: %s", buf.String())
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
		Clone:         fakeCloneFunc(),
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
		Clone:         fakeCloneFunc(),
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
		AgentProvider: "anthropic",
		AgentModel:    "claude-sonnet-4-6",
		GooseAgentPAT: "ghp_test",
		ClaudeCreds:   "creds",
		ProjectID:     "PVT_123",
	}

	run := func(name string, args ...string) (string, error) {
		commands = append(commands, fmt.Sprintf("%s %s", name, strings.Join(args, " ")))
		return "", nil
	}

	err := ConfigureRepo(&buf, cfg, run)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify variables were set.
	found := map[string]bool{}
	for _, cmd := range commands {
		if strings.Contains(cmd, "AGENT_USER") {
			found["AGENT_USER"] = true
		}
		if strings.Contains(cmd, "PROJECT_PAT") {
			found["PROJECT_PAT"] = true
		}
		if strings.Contains(cmd, "CLAUDE_CREDENTIALS_JSON") {
			found["CLAUDE_CREDENTIALS_JSON"] = true
		}
		if strings.Contains(cmd, "AGENTIC_PROJECT_ID") {
			found["AGENTIC_PROJECT_ID"] = true
		}
	}

	for _, expected := range []string{"AGENT_USER", "PROJECT_PAT", "CLAUDE_CREDENTIALS_JSON", "AGENTIC_PROJECT_ID"} {
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

	err := ConfigureRepo(&buf, cfg, run)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !accessGranted {
		t.Error("expected collaborator access to be granted")
	}
}

// --- federated init confirmation tests (task #534) ---

// recordConfirmFunc returns a ConfirmFunc that records each invocation
// and returns a prearranged yes/no answer.
func recordConfirmFunc(yes bool) (ConfirmFunc, *int) {
	count := 0
	return func(title, description string) (bool, error) {
		count++
		return yes, nil
	}, &count
}

func TestConfigureRepo_Federated_EmitsNoteAndConfirmsBeforeWriting(t *testing.T) {
	cfg := configureRepoTestConfig("Federated", "acme", "cp")
	confirm, called := recordConfirmFunc(true)
	cfg.Confirm = confirm
	run, captured := captureSetCalls()

	var buf bytes.Buffer
	if err := ConfigureRepo(&buf, cfg, run); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "Shared variables and secrets will be stored at organisation 'acme' and will be visible to any other federated control plane in the same organisation."
	if !strings.Contains(buf.String(), want) {
		t.Errorf("missing verbatim note. output:\n%s", buf.String())
	}
	if *called != 1 {
		t.Errorf("confirm called %d times, want 1", *called)
	}
	// Yes → writes proceed.
	if _, ok := (*captured)["AGENT_USER"]; !ok {
		t.Errorf("expected AGENT_USER write on Yes")
	}
}

func TestConfigureRepo_Federated_ConfirmNo_AbortsWithoutError(t *testing.T) {
	cfg := configureRepoTestConfig("Federated", "acme", "cp")
	confirm, called := recordConfirmFunc(false)
	cfg.Confirm = confirm
	run, captured := captureSetCalls()

	var buf bytes.Buffer
	if err := ConfigureRepo(&buf, cfg, run); err != nil {
		t.Fatalf("expected no error on No, got %v", err)
	}

	if *called != 1 {
		t.Errorf("confirm called %d times, want 1", *called)
	}
	if len(*captured) != 0 {
		t.Errorf("No should not trigger any writes; got: %v", *captured)
	}
	if !strings.Contains(buf.String(), "cancelled") {
		t.Errorf("expected cancellation message in output, got:\n%s", buf.String())
	}
}

func TestConfigureRepo_Single_NoNoteNoConfirm(t *testing.T) {
	cfg := configureRepoTestConfig("Single", "eddie", "repo")
	// Intentionally wire a confirm that would blow up the test if called.
	cfg.Confirm = func(title, description string) (bool, error) {
		t.Errorf("confirm must not be called under single topology")
		return false, nil
	}
	run, captured := captureSetCalls()

	var buf bytes.Buffer
	if err := ConfigureRepo(&buf, cfg, run); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Writes proceed as before.
	if _, ok := (*captured)["AGENT_USER"]; !ok {
		t.Errorf("expected AGENT_USER write under single")
	}
	// The federated note must NOT appear under single topology.
	if strings.Contains(buf.String(), "will be visible to any other federated") {
		t.Errorf("federated note must not appear under single topology; output:\n%s", buf.String())
	}
}

// configureRepoTestConfig returns a fully populated InitConfig so every
// named var/secret is written. The routing assertions below rely on this
// complete payload.
func configureRepoTestConfig(topology, owner, repoName string) *InitConfig {
	return &InitConfig{
		Version:       "v2.0.10",
		Topology:      topology,
		AgentUser:     "agent-bot",
		RunnerLabel:   "ubuntu-latest",
		AgentProvider: "claude-code",
		AgentModel:    "default",
		GooseAgentPAT: "ghp_test",
		ClaudeCreds:   "base64==",
		ProjectID:     "PVT_x",
		RepoFullName:  owner + "/" + repoName,
		Owner:         owner,
		RepoName:      repoName,
	}
}

// captureSetCalls returns a run function that records every gh set
// invocation keyed by the variable/secret name and keeps the full argument
// list so scope flags can be asserted.
func captureSetCalls() (func(string, ...string) (string, error), *map[string][]string) {
	captured := map[string][]string{}
	run := func(name string, args ...string) (string, error) {
		if name != "gh" {
			return "", nil
		}
		// gh variable set NAME ... | gh secret set NAME ...
		if len(args) >= 3 && args[0] != "variable" && args[0] != "secret" {
			return "", nil
		}
		if len(args) < 3 || args[1] != "set" {
			return "", nil
		}
		varName := args[2]
		captured[varName] = append([]string{}, args...)
		return "", nil
	}
	return run, &captured
}

// scopeFrom returns the (flag, target) pair from a recorded gh invocation,
// or ("", "") if no scope flag was found.
func scopeFrom(args []string) (string, string) {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--org" || args[i] == "--repo" {
			return args[i], args[i+1]
		}
	}
	return "", ""
}

func TestConfigureRepo_Federated_SharedNames_RouteToOrg(t *testing.T) {
	// NOTE: topology "Federated" is the capitalised wizard value — it is
	// normalised to lowercase inside ConfigureRepo before scope routing.
	cfg := configureRepoTestConfig("Federated", "acme", "cp")
	run, captured := captureSetCalls()

	var buf bytes.Buffer
	if err := ConfigureRepo(&buf, cfg, run); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	shared := []string{"AGENT_USER", "RUNNER_LABEL", "AGENT_PROVIDER", "AGENT_MODEL", "PROJECT_PAT", "CLAUDE_CREDENTIALS_JSON"}
	for _, name := range shared {
		args, ok := (*captured)[name]
		if !ok {
			t.Errorf("%s was not written", name)
			continue
		}
		flag, target := scopeFrom(args)
		if flag != "--org" || target != "acme" {
			t.Errorf("%s routed to (%q, %q), want (%q, %q)", name, flag, target, "--org", "acme")
		}
	}
}

func TestConfigureRepo_Federated_IdentityName_StaysAtRepo(t *testing.T) {
	cfg := configureRepoTestConfig("Federated", "acme", "cp")
	run, captured := captureSetCalls()

	var buf bytes.Buffer
	if err := ConfigureRepo(&buf, cfg, run); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only AGENTIC_PROJECT_ID is written by ConfigureRepo today. The other
	// identity names are written elsewhere (project.Create for topology,
	// etc.). Verify this one does not get the org treatment.
	args, ok := (*captured)["AGENTIC_PROJECT_ID"]
	if !ok {
		t.Fatal("AGENTIC_PROJECT_ID was not written")
	}
	flag, target := scopeFrom(args)
	if flag != "--repo" || target != "acme/cp" {
		t.Errorf("AGENTIC_PROJECT_ID routed to (%q, %q), want (%q, %q)", flag, target, "--repo", "acme/cp")
	}
}

func TestConfigureRepo_Single_AllNames_StayAtRepo(t *testing.T) {
	cfg := configureRepoTestConfig("Single", "eddie", "repo")
	run, captured := captureSetCalls()

	var buf bytes.Buffer
	if err := ConfigureRepo(&buf, cfg, run); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	all := []string{
		"AGENT_USER", "RUNNER_LABEL", "AGENT_PROVIDER", "AGENT_MODEL",
		"PROJECT_PAT", "CLAUDE_CREDENTIALS_JSON",
		"AGENTIC_PROJECT_ID",
	}
	for _, name := range all {
		args, ok := (*captured)[name]
		if !ok {
			t.Errorf("%s was not written", name)
			continue
		}
		flag, target := scopeFrom(args)
		if flag != "--repo" || target != "eddie/repo" {
			t.Errorf("%s routed to (%q, %q), want (%q, %q)", name, flag, target, "--repo", "eddie/repo")
		}
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
		t.Error("should return false when .ai/ is not present")
	}

	// Create a .ai/ directory to simulate a mounted framework — the
	// .ai-version flat file was removed in #585, so the init wizard
	// uses directory presence as the "already initialised" signal.
	if err := os.MkdirAll(filepath.Join(root, ".ai"), 0o755); err != nil {
		t.Fatalf("creating .ai/: %v", err)
	}
	if !CheckAIVersionExists(root) {
		t.Error("should return true when .ai/ directory is present")
	}
}
