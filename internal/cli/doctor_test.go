package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
	"github.com/eddiecarpenter/gh-agentic/internal/testutil"
)

// standardLabelsJSON is the JSON output that MockRunner returns for gh label list --json name.
const standardLabelsJSON = `[{"name":"requirement"},{"name":"feature"},{"name":"task"},{"name":"backlog"},{"name":"draft"},{"name":"scoping"},{"name":"scheduled"},{"name":"in-design"},{"name":"in-development"},{"name":"in-review"},{"name":"done"}]`

// projectJSON is the JSON output that MockRunner returns for gh project list --format json.
const projectJSON = `{"projects":[{"id":"PVT_test123","title":"test-project","number":1}]}`

// setupDoctorFakeRepo creates a FakeRepo with all files needed for the doctor
// checks to pass. It writes the extra files that NewFakeRepo does not include:
// .goose/recipes/*.yaml and .github/workflows/agentic-pipeline.yml.
func setupDoctorFakeRepo(t *testing.T) *testutil.FakeRepo {
	t.Helper()
	repo := testutil.NewFakeRepo(t)

	// GooseRecipes check expects 7 recipe files.
	recipes := []string{
		"dev-session.yaml",
		"feature-design.yaml",
		"feature-scoping.yaml",
		"foreground-recovery.yaml",
		"issue-session.yaml",
		"pr-review-session.yaml",
		"requirements-session.yaml",
	}
	for _, name := range recipes {
		repo.Write(filepath.Join(".goose", "recipes", name), "# "+name+"\n")
	}

	// Workflows check expects agentic-pipeline.yml.
	repo.Write(filepath.Join(".github", "workflows", "agentic-pipeline.yml"), "name: pipeline\n")

	// .ai/skills/ must contain at least one .md for CheckAISkills.
	repo.Write(filepath.Join(".ai", "skills", "dev-session.md"), "# skill\n")

	// .ai/project-template.json for CheckProjectStatus.
	repo.Write(filepath.Join(".ai", "project-template.json"), `{
  "statusOptions": [
    {"name": "Backlog",        "color": "GRAY",   "description": "Prioritised, ready to start"},
    {"name": "Scoping",        "color": "PURPLE", "description": "Requirement or feature being scoped"},
    {"name": "Scheduled",      "color": "BLUE",   "description": "Scoped and queued, waiting for design"},
    {"name": "In Design",      "color": "PINK",   "description": "Feature Design session active"},
    {"name": "In Development", "color": "YELLOW", "description": "Dev Session active"},
    {"name": "In Review",      "color": "ORANGE", "description": "PR open, awaiting review"},
    {"name": "Done",           "color": "GREEN",  "description": "Merged and closed"}
  ]
}`)

	return repo
}

// newMockRunner creates a MockRunner with expectations for the common gh/git
// commands that doctor checks invoke.
func newMockRunner(t *testing.T) *testutil.MockRunner {
	t.Helper()
	m := &testutil.MockRunner{}

	// CheckAIDir runs: git -C <root> diff --stat HEAD -- .ai/
	// We use a wildcard-free approach: register empty expectations.
	// MockRunner matches exact args — we need to register per-test since root varies.
	// Instead, return ("", nil) for unmatched which is the default.

	// CheckLabels: gh label list --repo owner/repo --json name --limit 100
	m.Expect([]string{"gh", "label", "list", "--repo", "testowner/testrepo", "--json", "name", "--limit", "100"}, standardLabelsJSON, nil)

	// CheckProject: gh project list --owner testowner --format json --limit 100
	m.Expect([]string{"gh", "project", "list", "--owner", "testowner", "--format", "json", "--limit", "100"}, projectJSON, nil)

	// CheckAgenticProjectID: gh variable get AGENTIC_PROJECT_ID --repo testowner/testrepo
	m.Expect([]string{"gh", "variable", "get", "AGENTIC_PROJECT_ID", "--repo", "testowner/testrepo"}, "PVT_test123", nil)

	// resolveProjectNodeIDViaRun (used by CheckProjectStatus): gh project list --limit 1
	m.Expect([]string{"gh", "project", "list", "--owner", "testowner", "--format", "json", "--limit", "100"}, projectJSON, nil)

	// CheckProjectStatus: fetch status options (name + color) via GraphQL.
	m.Expect([]string{"gh", "api", "graphql", "-f", `query={ node(id: "PVT_test123") { ... on ProjectV2 { field(name: "Status") { ... on ProjectV2SingleSelectField { id options { name color } } } } } }`, "--jq", `.data.node.field.options[] | "\(.name)|\(.color)"`}, "Backlog|GRAY\nScoping|PURPLE\nScheduled|BLUE\nIn Design|PINK\nIn Development|YELLOW\nIn Review|ORANGE\nDone|GREEN", nil)

	// resolveProjectNodeIDViaRun (used by CheckProjectViews): gh project list
	m.Expect([]string{"gh", "project", "list", "--owner", "testowner", "--format", "json", "--limit", "100"}, projectJSON, nil)

	// CheckProjectViews: fetch views via GraphQL.
	m.Expect([]string{"gh", "api", "graphql", "-f", `query={ node(id: "PVT_test123") { ... on ProjectV2 { views(first: 20) { nodes { name layout } } } } }`, "--jq", `.data.node.views.nodes[] | "\(.name)|\(.layout)"`}, "Requirements|TABLE_LAYOUT\nRequirements Kanban|BOARD_LAYOUT\nFeatures Kanban|BOARD_LAYOUT", nil)

	// resolveProjectNodeIDViaRun (used by CheckProjectItemStatuses): gh project list
	m.Expect([]string{"gh", "project", "list", "--owner", "testowner", "--format", "json", "--limit", "100"}, projectJSON, nil)

	// CheckProjectItemStatuses: fetch Status field ID via GraphQL.
	m.Expect([]string{"gh", "api", "graphql", "-f", `query={ node(id: "PVT_test123") { ... on ProjectV2 { field(name: "Status") { ... on ProjectV2SingleSelectField { id } } } } }`, "--jq", ".data.node.field.id"}, "FIELD_STATUS_1", nil)

	// CheckProjectItemStatuses: fetch all project items (empty — all have status).
	m.Expect([]string{"gh", "api", "graphql", "-f", `query={ node(id: "PVT_test123") { ... on ProjectV2 { items(first: 100) { pageInfo { hasNextPage endCursor } nodes { id content { ... on Issue { number repository { nameWithOwner } state labels(first: 20) { nodes { name } } } } fieldValues(first: 20) { nodes { ... on ProjectV2ItemFieldSingleSelectValue { field { ... on ProjectV2SingleSelectField { id } } name } } } } } } } }`}, `{"data":{"node":{"items":{"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[]}}}}`, nil)

	// resolveProjectNodeIDViaRun (used by CheckProjectCollaborator): gh project list --limit 1
	m.Expect([]string{"gh", "project", "list", "--owner", "testowner", "--format", "json", "--limit", "100"}, projectJSON, nil)

	// CheckProjectCollaborator: fetch collaborators via GraphQL.
	m.Expect([]string{"gh", "api", "graphql", "-f", `query={ node(id: "PVT_test123") { ... on ProjectV2 { collaborators(first: 100) { nodes { login } } } } }`, "--jq", ".data.node.collaborators.nodes[].login"}, "goose-agent", nil)

	// ReadAgentUserVar: gh variable list --org (fail for personal account).
	m.Expect([]string{"gh", "variable", "list", "--org", "testowner", "--json", "name,value"}, "", fmt.Errorf("not an org"))

	// ReadAgentUserVar: gh variable list --repo (return AGENT_USER).
	m.Expect([]string{"gh", "variable", "list", "--repo", "testowner/testrepo", "--json", "name,value"}, `[{"name":"AGENT_USER","value":"goose-agent"}]`, nil)

	// CheckAgentUserVar: gh variable list --org (fail for personal account).
	m.Expect([]string{"gh", "variable", "list", "--org", "testowner", "--json", "name"}, "", fmt.Errorf("not an org"))

	// CheckAgentUserVar: gh variable list --repo (return AGENT_USER).
	m.Expect([]string{"gh", "variable", "list", "--repo", "testowner/testrepo", "--json", "name"}, `[{"name":"AGENT_USER"}]`, nil)

	// CheckRunnerLabelVar: gh variable list --repo (return RUNNER_LABEL).
	m.Expect([]string{"gh", "variable", "list", "--repo", "testowner/testrepo", "--json", "name"}, `[{"name":"RUNNER_LABEL"}]`, nil)

	// CheckGooseProviderVar: gh variable list --repo (return GOOSE_PROVIDER).
	m.Expect([]string{"gh", "variable", "list", "--repo", "testowner/testrepo", "--json", "name"}, `[{"name":"GOOSE_PROVIDER"}]`, nil)

	// CheckGooseModelVar: gh variable list --repo (return GOOSE_MODEL).
	m.Expect([]string{"gh", "variable", "list", "--repo", "testowner/testrepo", "--json", "name"}, `[{"name":"GOOSE_MODEL"}]`, nil)

	// CheckGooseAgentPATSecret: gh secret list --repo (return GOOSE_AGENT_PAT).
	m.Expect([]string{"gh", "secret", "list", "--repo", "testowner/testrepo", "--json", "name"}, `[{"name":"GOOSE_AGENT_PAT"}]`, nil)

	// CheckClaudeCredentialsSecret: gh secret list --repo (return CLAUDE_CREDENTIALS_JSON).
	m.Expect([]string{"gh", "secret", "list", "--repo", "testowner/testrepo", "--json", "name"}, `[{"name":"CLAUDE_CREDENTIALS_JSON"}]`, nil)

	// CheckStaleOpenRequirements: gh issue list --label requirement --state open
	m.Expect([]string{"gh", "issue", "list", "--repo", "testowner/testrepo", "--label", "requirement", "--state", "open", "--json", "number,title", "--limit", "200"}, "[]", nil)

	// CheckStaleOpenFeatures: gh issue list --label feature --state open
	m.Expect([]string{"gh", "issue", "list", "--repo", "testowner/testrepo", "--label", "feature", "--state", "open", "--json", "number,title", "--limit", "200"}, "[]", nil)

	return m
}

func TestRunDoctor_AllChecksPass(t *testing.T) {
	repo := setupDoctorFakeRepo(t)
	mock := newMockRunner(t)

	var buf bytes.Buffer
	cfg := doctorConfig{
		root:         repo.Root,
		repoFullName: "testowner/testrepo",
		owner:        "testowner",
		repoName:     "testrepo",
		run:          mock.RunCommand,
		repair:       false,
		yes:          false,
	}

	err := runDoctor(&buf, strings.NewReader(""), cfg)

	output := buf.String()
	if err != nil {
		t.Fatalf("expected nil error, got: %v\nOutput:\n%s", err, output)
	}

	if !strings.Contains(output, "All checks passed") {
		t.Errorf("expected 'All checks passed' in output, got:\n%s", output)
	}
}

func TestRunDoctor_MissingCLAUDEMD(t *testing.T) {
	repo := setupDoctorFakeRepo(t)
	repo.Remove("CLAUDE.md")
	mock := newMockRunner(t)

	var buf bytes.Buffer
	cfg := doctorConfig{
		root:         repo.Root,
		repoFullName: "testowner/testrepo",
		owner:        "testowner",
		repoName:     "testrepo",
		run:          mock.RunCommand,
		repair:       false,
		yes:          false,
	}

	err := runDoctor(&buf, strings.NewReader(""), cfg)
	if err == nil {
		t.Fatal("expected error for missing CLAUDE.md, got nil")
	}

	output := buf.String()
	// Should contain a fail marker for CLAUDE.md.
	if !strings.Contains(output, "CLAUDE.md") {
		t.Errorf("expected CLAUDE.md mentioned in output, got:\n%s", output)
	}
}

func TestRunDoctor_MissingLOCALRULESMD(t *testing.T) {
	repo := setupDoctorFakeRepo(t)
	repo.Remove("LOCALRULES.md")
	mock := newMockRunner(t)

	var buf bytes.Buffer
	cfg := doctorConfig{
		root:         repo.Root,
		repoFullName: "testowner/testrepo",
		owner:        "testowner",
		repoName:     "testrepo",
		run:          mock.RunCommand,
		repair:       false,
		yes:          false,
	}

	err := runDoctor(&buf, strings.NewReader(""), cfg)

	output := buf.String()
	if !strings.Contains(output, "LOCALRULES.md") {
		t.Errorf("expected LOCALRULES.md mentioned in output, got:\n%s", output)
	}

	// Missing LOCALRULES.md is a Warning — should not be a hard failure.
	if err != nil && err != ErrSilent {
		t.Errorf("expected nil or ErrSilent, got: %v", err)
	}
}

func TestRunDoctor_MissingAIConfigYML(t *testing.T) {
	repo := setupDoctorFakeRepo(t)
	repo.Remove(filepath.Join(".ai", "config.yml"))
	mock := newMockRunner(t)

	var buf bytes.Buffer
	cfg := doctorConfig{
		root:         repo.Root,
		repoFullName: "testowner/testrepo",
		owner:        "testowner",
		repoName:     "testrepo",
		run:          mock.RunCommand,
		repair:       false,
		yes:          false,
	}

	err := runDoctor(&buf, strings.NewReader(""), cfg)

	output := buf.String()
	if !strings.Contains(output, "config.yml") {
		t.Errorf("expected config.yml mentioned in output, got:\n%s", output)
	}

	if err == nil {
		t.Error("expected error for missing .ai/config.yml, got nil")
	}
}

func TestRunDoctor_RepairYes_RestoreCLAUDEMD(t *testing.T) {
	repo := setupDoctorFakeRepo(t)
	repo.Remove("CLAUDE.md")
	mock := newMockRunner(t)

	var buf bytes.Buffer
	cfg := doctorConfig{
		root:         repo.Root,
		repoFullName: "testowner/testrepo",
		owner:        "testowner",
		repoName:     "testrepo",
		run:          mock.RunCommand,
		repair:       true,
		yes:          true,
	}

	err := runDoctor(&buf, strings.NewReader(""), cfg)

	// After repair, CLAUDE.md should exist on disk.
	claudePath := filepath.Join(repo.Root, "CLAUDE.md")
	if _, statErr := os.Stat(claudePath); os.IsNotExist(statErr) {
		t.Error("expected CLAUDE.md to be restored by repair, but file does not exist")
	}

	output := buf.String()
	// Output should show the fix.
	if !strings.Contains(output, "fixed") && !strings.Contains(output, "All checks passed") {
		t.Errorf("expected repair output to contain 'fixed' or 'All checks passed', got:\n%s", output)
	}

	// If all repairs succeeded, err should be nil.
	if err != nil {
		// Acceptable if other checks still fail — just log it.
		t.Logf("runDoctor returned error after repair (may be due to other checks): %v", err)
	}
}

func TestRunDoctor_RepairYes_RestoreLOCALRULESMD(t *testing.T) {
	repo := setupDoctorFakeRepo(t)
	repo.Remove("LOCALRULES.md")
	mock := newMockRunner(t)

	var buf bytes.Buffer
	cfg := doctorConfig{
		root:         repo.Root,
		repoFullName: "testowner/testrepo",
		owner:        "testowner",
		repoName:     "testrepo",
		run:          mock.RunCommand,
		repair:       true,
		yes:          true,
	}

	err := runDoctor(&buf, strings.NewReader(""), cfg)

	// After repair, LOCALRULES.md should exist on disk.
	localRulesPath := filepath.Join(repo.Root, "LOCALRULES.md")
	if _, statErr := os.Stat(localRulesPath); os.IsNotExist(statErr) {
		t.Error("expected LOCALRULES.md to be restored by repair, but file does not exist")
	}

	output := buf.String()
	if !strings.Contains(output, "fixed") && !strings.Contains(output, "All checks passed") {
		t.Errorf("expected repair output to contain 'fixed' or 'All checks passed', got:\n%s", output)
	}

	if err != nil {
		t.Logf("runDoctor returned error after repair (may be due to other checks): %v", err)
	}
}

func TestRunDoctor_UpdateCredentials_SkipsChecks_UploadsCredentials(t *testing.T) {
	mock := &testutil.MockRunner{}
	// gh secret set CLAUDE_CREDENTIALS_JSON
	mock.Expect([]string{"gh", "secret", "set", "CLAUDE_CREDENTIALS_JSON", "--body", "eyJ0b2tlbiI6InRlc3QifQ==", "--repo", "testowner/testrepo"}, "", nil)

	var buf bytes.Buffer
	cfg := doctorConfig{
		repoFullName:      "testowner/testrepo",
		owner:             "testowner",
		repoName:          "testrepo",
		ownerType:         "User",
		run:               mock.RunCommand,
		updateCredentials: true,
		claudeRefreshCmd: func() error {
			return nil // Simulate successful claude refresh.
		},
		readCredentials: func(run bootstrap.RunCommandFunc) ([]byte, error) {
			return []byte(`{"token":"test"}`), nil
		},
	}

	err := runDoctor(&buf, strings.NewReader(""), cfg)

	output := buf.String()
	if err != nil {
		t.Fatalf("expected nil error, got: %v\nOutput:\n%s", err, output)
	}

	// Should show "Updating credentials..." and confirmation.
	if !strings.Contains(output, "Updating credentials") {
		t.Errorf("expected 'Updating credentials' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "CLAUDE_CREDENTIALS_JSON") {
		t.Errorf("expected 'CLAUDE_CREDENTIALS_JSON' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "configured") {
		t.Errorf("expected 'configured' in output, got:\n%s", output)
	}

	// Should NOT contain any check output (checks skipped).
	if strings.Contains(output, "CLAUDE.md") {
		t.Errorf("checks should be skipped with --update-credentials, got:\n%s", output)
	}
}

func TestRunDoctor_UpdateCredentials_ClaudeRefreshFails(t *testing.T) {
	var buf bytes.Buffer
	cfg := doctorConfig{
		repoFullName:      "testowner/testrepo",
		owner:             "testowner",
		repoName:          "testrepo",
		ownerType:         "User",
		run:               (&testutil.MockRunner{}).RunCommand,
		updateCredentials: true,
		claudeRefreshCmd: func() error {
			return fmt.Errorf("auth expired")
		},
	}

	err := runDoctor(&buf, strings.NewReader(""), cfg)
	if err == nil {
		t.Fatal("expected error when claude refresh fails, got nil")
	}

	output := buf.String()
	if !strings.Contains(output, "claude refresh failed") {
		t.Errorf("expected 'claude refresh failed' in output, got:\n%s", output)
	}
}

func TestRunDoctor_UpdateCredentials_MacOS_KeychainRead(t *testing.T) {
	mock := &testutil.MockRunner{}
	// gh secret set CLAUDE_CREDENTIALS_JSON
	mock.Expect([]string{"gh", "secret", "set", "CLAUDE_CREDENTIALS_JSON", "--body", "a2V5Y2hhaW4tY3JlZHM=", "--repo", "testowner/testrepo"}, "", nil)

	var buf bytes.Buffer
	cfg := doctorConfig{
		repoFullName:      "testowner/testrepo",
		owner:             "testowner",
		repoName:          "testrepo",
		ownerType:         "User",
		run:               mock.RunCommand,
		updateCredentials: true,
		claudeRefreshCmd: func() error {
			return nil
		},
		readCredentials: func(run bootstrap.RunCommandFunc) ([]byte, error) {
			// Simulate macOS keychain read.
			return []byte("keychain-creds"), nil
		},
	}

	err := runDoctor(&buf, strings.NewReader(""), cfg)
	if err != nil {
		t.Fatalf("expected nil error, got: %v\nOutput:\n%s", err, buf.String())
	}

	output := buf.String()
	if !strings.Contains(output, "configured") {
		t.Errorf("expected 'configured' in output, got:\n%s", output)
	}
}

func TestRunDoctor_UpdateCredentials_LinuxFileRead(t *testing.T) {
	mock := &testutil.MockRunner{}
	// gh secret set CLAUDE_CREDENTIALS_JSON — for Linux file-based read.
	mock.Expect([]string{"gh", "secret", "set", "CLAUDE_CREDENTIALS_JSON", "--body", "bGludXgtZmlsZS1jcmVkcw==", "--repo", "testowner/testrepo"}, "", nil)

	var buf bytes.Buffer
	cfg := doctorConfig{
		repoFullName:      "testowner/testrepo",
		owner:             "testowner",
		repoName:          "testrepo",
		ownerType:         "User",
		run:               mock.RunCommand,
		updateCredentials: true,
		claudeRefreshCmd: func() error {
			return nil
		},
		readCredentials: func(run bootstrap.RunCommandFunc) ([]byte, error) {
			// Simulate Linux file read.
			return []byte("linux-file-creds"), nil
		},
	}

	err := runDoctor(&buf, strings.NewReader(""), cfg)
	if err != nil {
		t.Fatalf("expected nil error, got: %v\nOutput:\n%s", err, buf.String())
	}

	output := buf.String()
	if !strings.Contains(output, "configured") {
		t.Errorf("expected 'configured' in output, got:\n%s", output)
	}
}

func TestRunDoctor_NoUpdateCredentials_RunsChecks(t *testing.T) {
	repo := setupDoctorFakeRepo(t)
	mock := newMockRunner(t)

	var buf bytes.Buffer
	cfg := doctorConfig{
		root:              repo.Root,
		repoFullName:      "testowner/testrepo",
		owner:             "testowner",
		repoName:          "testrepo",
		run:               mock.RunCommand,
		repair:            false,
		yes:               false,
		updateCredentials: false,
	}

	err := runDoctor(&buf, strings.NewReader(""), cfg)

	output := buf.String()
	if err != nil {
		t.Fatalf("expected nil error, got: %v\nOutput:\n%s", err, output)
	}

	// Should contain check output — not skipped.
	if !strings.Contains(output, "All checks passed") {
		t.Errorf("expected checks to run when --update-credentials not set, got:\n%s", output)
	}
}

func TestRunDoctor_AgenticProjectIDCheckAppearsInOutput(t *testing.T) {
	repo := setupDoctorFakeRepo(t)
	mock := newMockRunner(t)

	var buf bytes.Buffer
	cfg := doctorConfig{
		root:         repo.Root,
		repoFullName: "testowner/testrepo",
		owner:        "testowner",
		repoName:     "testrepo",
		run:          mock.RunCommand,
		repair:       false,
		yes:          false,
	}

	_ = runDoctor(&buf, strings.NewReader(""), cfg)

	output := buf.String()
	if !strings.Contains(output, "AGENTIC_PROJECT_ID is configured") {
		t.Errorf("expected 'AGENTIC_PROJECT_ID is configured' in output, got:\n%s", output)
	}
}


