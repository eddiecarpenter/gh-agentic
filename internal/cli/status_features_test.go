package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/eddiecarpenter/gh-agentic/internal/project"
	"github.com/eddiecarpenter/gh-agentic/internal/projectstatus"
)

// fakeFeaturesDeps builds a statusDeps that serves the given issues through
// projectstatus with the current repo fixed to eddiecarpenter/gh-agentic.
func fakeFeaturesDeps(issues []projectstatus.ProjectIssue, linked []project.LinkedRepo) statusDeps {
	return statusDeps{
		currentRepo:      func() (string, error) { return "eddiecarpenter/gh-agentic", nil },
		resolveProjectID: func(string) (string, error) { return "PROJ_ID", nil },
		psDeps: projectstatus.Deps{
			FetchProjectIssues: func(projectID string) ([]projectstatus.ProjectIssue, error) {
				return issues, nil
			},
			FetchLinkedRepos: func(projectID string) ([]project.LinkedRepo, error) {
				return linked, nil
			},
		},
	}
}

// sampleFeatureIssues returns a deterministic fixture of feature + non-feature
// issues across two repos, used to exercise federation aggregation.
func sampleFeatureIssues() []projectstatus.ProjectIssue {
	now := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	return []projectstatus.ProjectIssue{
		{Number: 483, Title: "feat: ask-user", Stage: projectstatus.StageDone, Type: "feature", State: "closed", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: now, LastTransitionedAt: now},
		{Number: 492, Title: "feat: status", Stage: projectstatus.StageInDevelopment, Type: "feature", State: "open", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: now, LastTransitionedAt: now},
		{Number: 511, Title: "feat: domain-X event handler", Stage: projectstatus.StageInDevelopment, Type: "feature", State: "open", OwningRepo: "foo/domain-x", CreatedAt: now, LastTransitionedAt: now},
		// Requirements / tasks — must be filtered out.
		{Number: 457, Title: "requirement", Type: "requirement", State: "open", OwningRepo: "eddiecarpenter/gh-agentic", Stage: projectstatus.StageScoping, CreatedAt: now, LastTransitionedAt: now},
		{Number: 999, Title: "task", Type: "task", State: "open", OwningRepo: "eddiecarpenter/gh-agentic", Stage: projectstatus.StageBacklog, CreatedAt: now, LastTransitionedAt: now},
	}
}

// TestRunStatusFeatures_DefaultExcludesDone verifies the default invocation
// drops closed items and lists the two open features.
func TestRunStatusFeatures_DefaultExcludesDone(t *testing.T) {
	sd := fakeFeaturesDeps(sampleFeatureIssues(), nil)
	buf := &bytes.Buffer{}
	if err := runStatusFeatures(buf, statusListFlags{}, sd); err != nil {
		t.Fatalf("runStatusFeatures: %v", err)
	}
	out := buf.String()
	for _, tok := range []string{"FEATURE", "STAGE", "TITLE", "#492", "#511"} {
		if !strings.Contains(out, tok) {
			t.Errorf("missing %q; got:\n%s", tok, out)
		}
	}
	if strings.Contains(out, "#483") {
		t.Errorf("closed feature #483 should be excluded; got:\n%s", out)
	}
	if !strings.Contains(out, "2 open features") {
		t.Errorf("expected totals '2 open features'; got:\n%s", out)
	}
}

// TestRunStatusFeatures_FederatedAggregation verifies that when the project
// spans two repos, both repos' features appear and the REPO column is shown.
func TestRunStatusFeatures_FederatedAggregation(t *testing.T) {
	linked := []project.LinkedRepo{
		{NameWithOwner: "eddiecarpenter/gh-agentic"},
		{NameWithOwner: "foo/domain-x"},
	}
	sd := fakeFeaturesDeps(sampleFeatureIssues(), linked)
	buf := &bytes.Buffer{}
	if err := runStatusFeatures(buf, statusListFlags{}, sd); err != nil {
		t.Fatalf("runStatusFeatures: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "REPO") {
		t.Errorf("expected REPO column on federated topology; got:\n%s", out)
	}
	if !strings.Contains(out, "foo/domain-x") {
		t.Errorf("cross-repo feature #511 should list 'foo/domain-x'; got:\n%s", out)
	}
	if !strings.Contains(out, "(this repo)") {
		t.Errorf("local row should show '(this repo)'; got:\n%s", out)
	}
}

// TestRunStatusFeatures_ThisRepoNarrows verifies --this-repo drops cross-repo
// features.
func TestRunStatusFeatures_ThisRepoNarrows(t *testing.T) {
	sd := fakeFeaturesDeps(sampleFeatureIssues(), nil)
	buf := &bytes.Buffer{}
	if err := runStatusFeatures(buf, statusListFlags{thisRepo: true}, sd); err != nil {
		t.Fatalf("runStatusFeatures: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "#511") {
		t.Errorf("--this-repo should exclude cross-repo #511; got:\n%s", out)
	}
	if !strings.Contains(out, "#492") {
		t.Errorf("--this-repo should keep local #492; got:\n%s", out)
	}
}

// TestRunStatusFeatures_IncludeDone pulls in closed items.
func TestRunStatusFeatures_IncludeDone(t *testing.T) {
	sd := fakeFeaturesDeps(sampleFeatureIssues(), nil)
	buf := &bytes.Buffer{}
	if err := runStatusFeatures(buf, statusListFlags{includeDone: true}, sd); err != nil {
		t.Fatalf("runStatusFeatures: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "#483") {
		t.Errorf("closed #483 should be listed with --include-done; got:\n%s", out)
	}
	if !strings.Contains(out, "3 open features") {
		t.Errorf("expected totals '3 open features'; got:\n%s", out)
	}
}

// TestRunStatusFeatures_JSONSchema verifies --json emits the envelope with the
// documented field names on every item.
func TestRunStatusFeatures_JSONSchema(t *testing.T) {
	sd := fakeFeaturesDeps(sampleFeatureIssues(), nil)
	buf := &bytes.Buffer{}
	if err := runStatusFeatures(buf, statusListFlags{json: true}, sd); err != nil {
		t.Fatalf("runStatusFeatures: %v", err)
	}
	var parsed struct {
		Items  []map[string]interface{} `json:"items"`
		Totals struct {
			Open    int `json:"open"`
			Blocked int `json:"blocked"`
		} `json:"totals"`
	}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("json parse: %v; raw:\n%s", err, buf.String())
	}
	if parsed.Totals.Open != 2 {
		t.Errorf("totals.open = %d, want 2", parsed.Totals.Open)
	}
	requiredKeys := []string{"number", "title", "body", "stage", "created_at", "last_transitioned_at", "owning_repo", "blocked", "parent_requirement", "tasks", "branch", "pr"}
	for i, item := range parsed.Items {
		for _, k := range requiredKeys {
			if _, ok := item[k]; !ok {
				t.Errorf("item[%d] missing required key %q; keys = %v", i, k, keysOf(item))
			}
		}
	}
}

// TestRunStatusFeatures_EmptyEnvelope verifies no-results returns items: [].
func TestRunStatusFeatures_EmptyEnvelope(t *testing.T) {
	sd := fakeFeaturesDeps(nil, nil)
	buf := &bytes.Buffer{}
	if err := runStatusFeatures(buf, statusListFlags{json: true}, sd); err != nil {
		t.Fatalf("runStatusFeatures: %v", err)
	}
	if !strings.Contains(buf.String(), `"items": []`) {
		t.Errorf("expected 'items: []'; got:\n%s", buf.String())
	}
}

// TestRunStatusFeatures_BlockedAnnotation exercises the renderer directly
// with a blocked feature to verify the inline annotation and totals.
func TestRunStatusFeatures_BlockedAnnotation(t *testing.T) {
	features := []projectstatus.Feature{
		{Number: 511, Title: "feat: domain-X event handler", Stage: projectstatus.StageInDevelopment, OwningRepo: "foo/domain-x", Blocked: &projectstatus.BlockedInfo{BlockingRef: "foo/domain-x#507"}},
		{Number: 492, Title: "feat: status", Stage: projectstatus.StageInDevelopment, OwningRepo: "eddiecarpenter/gh-agentic"},
	}
	buf := &bytes.Buffer{}
	if err := writeFeaturesTable(buf, features, "eddiecarpenter/gh-agentic"); err != nil {
		t.Fatalf("writeFeaturesTable: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "[blocked by foo/domain-x#507]") {
		t.Errorf("expected blocked annotation; got:\n%s", out)
	}
	if !strings.Contains(out, "2 open features (1 blocked)") {
		t.Errorf("expected totals line; got:\n%s", out)
	}
}

// TestRunStatusFeatures_NoProjectConfigured verifies ErrProjectNotConfigured.
func TestRunStatusFeatures_NoProjectConfigured(t *testing.T) {
	sd := statusDeps{
		currentRepo:      func() (string, error) { return "eddiecarpenter/gh-agentic", nil },
		resolveProjectID: func(string) (string, error) { return "", nil },
	}
	err := runStatusFeatures(&bytes.Buffer{}, statusListFlags{}, sd)
	if !errors.Is(err, projectstatus.ErrProjectNotConfigured) {
		t.Errorf("expected ErrProjectNotConfigured; got %v", err)
	}
}

// TestRunStatusFeatures_KanbanVertical verifies --kanban renders the
// stage-grouped view for features.
func TestRunStatusFeatures_KanbanVertical(t *testing.T) {
	sd := fakeFeaturesDeps(sampleFeatureIssues(), nil)
	buf := &bytes.Buffer{}
	if err := runStatusFeatures(buf, statusListFlags{kanban: true}, sd); err != nil {
		t.Fatalf("runStatusFeatures --kanban: %v", err)
	}
	out := buf.String()
	for _, tok := range []string{
		"Features — Kanban",
		"## backlog (0)",
		"## in-design (0)",
		"## in-development (2)", // #492, #511
		"## in-review (0)",
		"#492",
		"#511",
	} {
		if !strings.Contains(out, tok) {
			t.Errorf("expected %q; got:\n%s", tok, out)
		}
	}
}

// TestFeaturesTotalsLine covers singular / plural and blocked branches.
func TestFeaturesTotalsLine(t *testing.T) {
	cases := []struct {
		n, blocked int
		expected   string
	}{
		{0, 0, "0 open features"},
		{1, 0, "1 open feature"},
		{2, 1, "2 open features (1 blocked)"},
	}
	for _, tc := range cases {
		got := featuresTotalsLine(tc.n, tc.blocked)
		if got != tc.expected {
			t.Errorf("featuresTotalsLine(%d,%d) = %q, want %q", tc.n, tc.blocked, got, tc.expected)
		}
	}
}
