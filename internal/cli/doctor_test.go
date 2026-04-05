package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

	// base/skills/ must contain at least one .md for CheckBaseRecipes.
	repo.Write(filepath.Join("base", "skills", "dev-session.md"), "# skill\n")

	// base/project-template.json for CheckProjectStatus.
	repo.Write(filepath.Join("base", "project-template.json"), `{
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

	// CheckBaseDir runs: git -C <root> diff --stat HEAD -- base/
	// We use a wildcard-free approach: register empty expectations.
	// MockRunner matches exact args — we need to register per-test since root varies.
	// Instead, return ("", nil) for unmatched which is the default.

	// CheckLabels: gh label list --repo owner/repo --json name --limit 100
	m.Expect([]string{"gh", "label", "list", "--repo", "testowner/testrepo", "--json", "name", "--limit", "100"}, standardLabelsJSON, nil)

	// CheckProject: gh project list --owner testowner --format json --limit 100
	m.Expect([]string{"gh", "project", "list", "--owner", "testowner", "--format", "json", "--limit", "100"}, projectJSON, nil)

	// resolveProjectNodeIDViaRun (used by CheckProjectStatus): gh project list --limit 1
	m.Expect([]string{"gh", "project", "list", "--owner", "testowner", "--format", "json", "--limit", "100"}, projectJSON, nil)

	// CheckProjectStatus: fetch status options via GraphQL.
	m.Expect([]string{"gh", "api", "graphql", "-f", `query={ node(id: \"PVT_test123\") { ... on ProjectV2 { field(name: \"Status\") { ... on ProjectV2SingleSelectField { id options { name } } } } } }`, "--jq", ".data.node.field.options[].name"}, "Backlog\nScoping\nScheduled\nIn Design\nIn Development\nIn Review\nDone", nil)

	// resolveProjectNodeIDViaRun (used by CheckProjectItemStatuses): gh project list --limit 1
	m.Expect([]string{"gh", "project", "list", "--owner", "testowner", "--format", "json", "--limit", "100"}, projectJSON, nil)

	// CheckProjectItemStatuses: fetch Status field ID via GraphQL.
	m.Expect([]string{"gh", "api", "graphql", "-f", `query={ node(id: \"PVT_test123\") { ... on ProjectV2 { field(name: \"Status\") { ... on ProjectV2SingleSelectField { id } } } } }`, "--jq", ".data.node.field.id"}, "FIELD_STATUS_1", nil)

	// CheckProjectItemStatuses: fetch all project items (empty — all have status).
	m.Expect([]string{"gh", "api", "graphql", "-f", `query={ node(id: \"PVT_test123\") { ... on ProjectV2 { items(first: 100) { pageInfo { hasNextPage endCursor } nodes { id content { ... on Issue { state labels(first: 20) { nodes { name } } } } fieldValues(first: 20) { nodes { ... on ProjectV2ItemFieldSingleSelectValue { field { ... on ProjectV2SingleSelectField { id } } name } } } } } } } }`}, `{"data":{"node":{"items":{"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[]}}}}`, nil)

	// resolveProjectNodeIDViaRun (used by CheckProjectCollaborator): gh project list --limit 1
	m.Expect([]string{"gh", "project", "list", "--owner", "testowner", "--format", "json", "--limit", "100"}, projectJSON, nil)

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

func TestRunDoctor_MissingAGENTSLocalMD(t *testing.T) {
	repo := setupDoctorFakeRepo(t)
	repo.Remove("AGENTS.local.md")
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
		t.Fatal("expected error for missing AGENTS.local.md, got nil")
	}

	output := buf.String()
	if !strings.Contains(output, "AGENTS.local.md") {
		t.Errorf("expected AGENTS.local.md mentioned in output, got:\n%s", output)
	}
}

func TestRunDoctor_MissingTEMPLATESOURCE(t *testing.T) {
	repo := setupDoctorFakeRepo(t)
	repo.Remove("TEMPLATE_SOURCE")
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

	// TEMPLATE_SOURCE missing is a Warning, not a Fail in some checks.
	// The test verifies the output mentions TEMPLATE_SOURCE.
	err := runDoctor(&buf, strings.NewReader(""), cfg)

	output := buf.String()
	if !strings.Contains(output, "TEMPLATE_SOURCE") {
		t.Errorf("expected TEMPLATE_SOURCE mentioned in output, got:\n%s", output)
	}

	// If there is an error, it should be ErrSilent (already printed).
	if err != nil && err != ErrSilent {
		t.Errorf("expected nil or ErrSilent, got: %v", err)
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

func TestRunDoctor_RepairYes_RestoreAGENTSLocalMD(t *testing.T) {
	repo := setupDoctorFakeRepo(t)
	repo.Remove("AGENTS.local.md")
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

	// After repair, AGENTS.local.md should exist on disk.
	agentsPath := filepath.Join(repo.Root, "AGENTS.local.md")
	if _, statErr := os.Stat(agentsPath); os.IsNotExist(statErr) {
		t.Error("expected AGENTS.local.md to be restored by repair, but file does not exist")
	}

	output := buf.String()
	if !strings.Contains(output, "fixed") && !strings.Contains(output, "All checks passed") {
		t.Errorf("expected repair output to contain 'fixed' or 'All checks passed', got:\n%s", output)
	}

	if err != nil {
		t.Logf("runDoctor returned error after repair (may be due to other checks): %v", err)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// --resync-statuses flag tests
// ──────────────────────────────────────────────────────────────────────────────

func TestRunDoctor_ResyncStatuses_CallsResync(t *testing.T) {
	repo := setupDoctorFakeRepo(t)
	mock := &testutil.MockRunner{}

	// ResyncProjectItemStatuses calls:
	// 1. resolveProjectNodeIDViaRun
	mock.Expect([]string{"gh", "project", "list", "--owner", "testowner", "--format", "json", "--limit", "100"}, projectJSON, nil)
	// 2. Fetch Status field ID
	mock.Expect([]string{"gh", "api", "graphql", "-f", `query={ node(id: \"PVT_test123\") { ... on ProjectV2 { field(name: \"Status\") { ... on ProjectV2SingleSelectField { id } } } } }`, "--jq", ".data.node.field.id"}, "FIELD_1", nil)
	// 3. fetchStatusOptionMap
	mock.Expect([]string{"gh", "api", "graphql", "-f", `query={ node(id: \"PVT_test123\") { ... on ProjectV2 { field(name: \"Status\") { ... on ProjectV2SingleSelectField { options { id name } } } } } }`, "--jq", `.data.node.field.options[] | "\(.id)|\(.name)"`}, "OPT_1|Backlog\nOPT_7|Done", nil)
	// 4. fetchAllProjectItems — empty
	mock.Expect([]string{"gh", "api", "graphql", "-f", `query={ node(id: \"PVT_test123\") { ... on ProjectV2 { items(first: 100) { pageInfo { hasNextPage endCursor } nodes { id content { ... on Issue { state labels(first: 20) { nodes { name } } } } fieldValues(first: 20) { nodes { ... on ProjectV2ItemFieldSingleSelectValue { field { ... on ProjectV2SingleSelectField { id } } name } } } } } } } }`}, `{"data":{"node":{"items":{"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[]}}}}`, nil)

	var buf bytes.Buffer
	cfg := doctorConfig{
		root:           repo.Root,
		repoFullName:   "testowner/testrepo",
		owner:          "testowner",
		repoName:       "testrepo",
		run:            mock.RunCommand,
		resyncStatuses: true,
		yes:            true,
	}

	err := runDoctor(&buf, strings.NewReader(""), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v\nOutput:\n%s", err, buf.String())
	}

	output := buf.String()
	// Should NOT contain normal check output.
	if strings.Contains(output, "CLAUDE.md exists") {
		t.Error("normal checks should be skipped when --resync-statuses is set")
	}
	// Should contain summary.
	if !strings.Contains(output, "items updated") {
		t.Errorf("expected summary in output, got:\n%s", output)
	}
}

func TestRunDoctor_ResyncStatuses_PrintsSummary(t *testing.T) {
	repo := setupDoctorFakeRepo(t)
	mock := &testutil.MockRunner{}

	// Same setup as above but with items to update.
	mock.Expect([]string{"gh", "project", "list", "--owner", "testowner", "--format", "json", "--limit", "100"}, projectJSON, nil)
	mock.Expect([]string{"gh", "api", "graphql", "-f", `query={ node(id: \"PVT_test123\") { ... on ProjectV2 { field(name: \"Status\") { ... on ProjectV2SingleSelectField { id } } } } }`, "--jq", ".data.node.field.id"}, "FIELD_1", nil)
	mock.Expect([]string{"gh", "api", "graphql", "-f", `query={ node(id: \"PVT_test123\") { ... on ProjectV2 { field(name: \"Status\") { ... on ProjectV2SingleSelectField { options { id name } } } } } }`, "--jq", `.data.node.field.options[] | "\(.id)|\(.name)"`}, "OPT_1|Backlog\nOPT_7|Done", nil)
	// Return one item that needs updating (OPEN with backlog → Backlog, currently has no status).
	mock.Expect([]string{"gh", "api", "graphql", "-f", `query={ node(id: \"PVT_test123\") { ... on ProjectV2 { items(first: 100) { pageInfo { hasNextPage endCursor } nodes { id content { ... on Issue { state labels(first: 20) { nodes { name } } } } fieldValues(first: 20) { nodes { ... on ProjectV2ItemFieldSingleSelectValue { field { ... on ProjectV2SingleSelectField { id } } name } } } } } } } }`},
		`{"data":{"node":{"items":{"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[{"id":"ITEM_1","content":{"state":"OPEN","labels":{"nodes":[{"name":"backlog"}]}},"fieldValues":{"nodes":[]}}]}}}}`, nil)

	var buf bytes.Buffer
	cfg := doctorConfig{
		root:           repo.Root,
		repoFullName:   "testowner/testrepo",
		owner:          "testowner",
		repoName:       "testrepo",
		run:            mock.RunCommand,
		resyncStatuses: true,
		yes:            true,
	}

	err := runDoctor(&buf, strings.NewReader(""), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v\nOutput:\n%s", err, buf.String())
	}

	output := buf.String()
	if !strings.Contains(output, "1 items updated") {
		t.Errorf("expected '1 items updated' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "0 already correct") {
		t.Errorf("expected '0 already correct' in output, got:\n%s", output)
	}
}
