package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eddiecarpenter/gh-agentic/internal/project"
	"github.com/eddiecarpenter/gh-agentic/internal/projectstatus"
)

// --------------------------------------------------------------------------
// Fixture — canned federated project with two repos, three requirements,
// four features, six tasks, and one blocked item. Used across the
// integration tests to prove the full stack works end-to-end against a
// realistic-shaped project state.
// --------------------------------------------------------------------------

// fixedTime is the deterministic timestamp used everywhere so --json output
// is byte-identical across runs.
var fixedTime = time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)

// fixtureIssues returns the canned project board used by every integration
// test in this file. The shape deliberately covers: requirements and
// features across two owning repos, closed entries, a blocked feature, a
// requirement with a linked feature, and type labels on every issue.
func fixtureIssues() []projectstatus.ProjectIssue {
	return []projectstatus.ProjectIssue{
		// Requirements ------------------------------------------------------
		{Number: 447, Title: "feat: project lifecycle management", Body: "Business need.", Stage: projectstatus.StageBacklog, Type: "requirement", State: "open", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: fixedTime, LastTransitionedAt: fixedTime},
		{Number: 457, Title: "feat: single-pane pipeline status view", Body: "Closes #447", Stage: projectstatus.StageScoping, Type: "requirement", State: "open", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: fixedTime, LastTransitionedAt: fixedTime},
		{Number: 466, Title: "feat: ask-user reference", Body: "Done body.", Stage: projectstatus.StageDone, Type: "requirement", State: "closed", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: fixedTime, LastTransitionedAt: fixedTime},
		// Features ----------------------------------------------------------
		{Number: 483, Title: "feat: ask-user skill", Body: "Closes #466", Stage: projectstatus.StageDone, Type: "feature", State: "closed", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: fixedTime, LastTransitionedAt: fixedTime},
		{Number: 492, Title: "feat: status command", Body: "## User Story\nCloses #457", Stage: projectstatus.StageInDevelopment, Type: "feature", State: "open", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: fixedTime, LastTransitionedAt: fixedTime},
		{Number: 511, Title: "feat: domain-X event handler", Body: "Blocked-by: foo/domain-x#507", Stage: projectstatus.StageInDevelopment, Type: "feature", State: "open", OwningRepo: "foo/domain-x", CreatedAt: fixedTime, LastTransitionedAt: fixedTime},
		{Number: 520, Title: "feat: placeholder", Body: "", Stage: projectstatus.StageBacklog, Type: "feature", State: "open", OwningRepo: "foo/domain-x", CreatedAt: fixedTime, LastTransitionedAt: fixedTime},
		// Tasks (sub-issues of #492 scoped feature) -------------------------
		{Number: 494, Title: "Scaffold", Type: "task", State: "closed", Stage: projectstatus.StageBacklog, OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: fixedTime, LastTransitionedAt: fixedTime},
		{Number: 495, Title: "projectstatus types", Type: "task", State: "closed", Stage: projectstatus.StageBacklog, OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: fixedTime, LastTransitionedAt: fixedTime},
		{Number: 496, Title: "Requirements list", Type: "task", State: "closed", Stage: projectstatus.StageBacklog, OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: fixedTime, LastTransitionedAt: fixedTime},
		{Number: 497, Title: "Requirement detail", Type: "task", State: "closed", Stage: projectstatus.StageBacklog, OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: fixedTime, LastTransitionedAt: fixedTime},
		{Number: 498, Title: "Features list", Type: "task", State: "closed", Stage: projectstatus.StageBacklog, OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: fixedTime, LastTransitionedAt: fixedTime},
		{Number: 499, Title: "Feature detail", Type: "task", State: "open", Stage: projectstatus.StageBacklog, OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: fixedTime, LastTransitionedAt: fixedTime},
	}
}

// fixtureTasks returns the sub-issue map consumed by FetchFeature.
func fixtureTasks() map[int][]projectstatus.TaskRef {
	return map[int][]projectstatus.TaskRef{
		492: {
			{Number: 494, Title: "Scaffold", Closed: true},
			{Number: 495, Title: "projectstatus types", Closed: true},
			{Number: 496, Title: "Requirements list", Closed: true},
			{Number: 497, Title: "Requirement detail", Closed: true},
			{Number: 498, Title: "Features list", Closed: true},
			{Number: 499, Title: "Feature detail", Closed: false},
		},
	}
}

// fixtureBranches returns the branch map for FetchBranch.
func fixtureBranches() map[string]*projectstatus.BranchState {
	return map[string]*projectstatus.BranchState{
		"feature/483": {Name: "feature/483", Exists: true, Merged: true},
		"feature/492": {Name: "feature/492", Exists: true, Merged: false},
	}
}

// fixturePRs returns the PR map for FetchPR.
func fixturePRs() map[string]*projectstatus.PRState {
	return map[string]*projectstatus.PRState{
		"feature/483": {Number: 491, State: "merged", Reviewers: []string{"eddiecarpenter"}},
		"feature/492": {Number: 777, State: "open", Reviewers: []string{"eddie"}},
	}
}

// buildFixtureDeps assembles a statusDeps that serves the canned fixtures.
func buildFixtureDeps() statusDeps {
	return statusDeps{
		currentRepo:      func() (string, error) { return "eddiecarpenter/gh-agentic", nil },
		resolveProjectID: func(string) (string, error) { return "PVT_fixture", nil },
		psDeps: projectstatus.Deps{
			FetchProjectIssues: func(string) ([]projectstatus.ProjectIssue, error) { return fixtureIssues(), nil },
			FetchLinkedRepos: func(string) ([]project.LinkedRepo, error) {
				return []project.LinkedRepo{
					{NameWithOwner: "eddiecarpenter/gh-agentic"},
					{NameWithOwner: "foo/domain-x"},
				}, nil
			},
			FetchSubIssues: func(_, _ string, n int) ([]projectstatus.TaskRef, error) {
				return fixtureTasks()[n], nil
			},
			FetchParentIssue: func(_, _ string, n int) (*projectstatus.RequirementSummary, error) {
				return nil, nil
			},
			FetchBranch: func(_, _, name string) (*projectstatus.BranchState, error) {
				if b, ok := fixtureBranches()[name]; ok {
					return b, nil
				}
				return &projectstatus.BranchState{Name: name, Exists: false}, nil
			},
			FetchPR: func(_, _, name string) (*projectstatus.PRState, error) {
				return fixturePRs()[name], nil
			},
		},
	}
}

// --------------------------------------------------------------------------
// Lightweight schema validator — a tiny shape-matcher that reads the locked
// schema JSON file and asserts field presence and type on the payload. No
// external dependency; keeps the testdata/ files human-editable.
// --------------------------------------------------------------------------

// statusSchema is the parsed form of testdata/status_schemas/*.json.
type statusSchema struct {
	Envelope                           map[string]string `json:"envelope"`
	TotalsFields                       map[string]string `json:"totals_fields"`
	ItemFields                         map[string]string `json:"item_fields"`
	RootFields                         map[string]string `json:"root_fields"`
	LinkedFeatureFields                map[string]string `json:"linked_feature_fields"`
	TaskFields                         map[string]string `json:"task_fields"`
	BranchFieldsWhenPresent            map[string]string `json:"branch_fields_when_present"`
	PRFieldsWhenPresent                map[string]string `json:"pr_fields_when_present"`
	ParentRequirementFieldsWhenPresent map[string]string `json:"parent_requirement_fields_when_present"`
	BlockedFieldsWhenPresent           map[string]string `json:"blocked_fields_when_present"`
}

// loadSchema reads a schema JSON file from testdata.
func loadSchema(t *testing.T, name string) statusSchema {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "status_schemas", name))
	if err != nil {
		t.Fatalf("loadSchema %q: %v", name, err)
	}
	var s statusSchema
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("loadSchema %q parse: %v", name, err)
	}
	return s
}

// typeOf returns the coarse JSON-level type of v as used by assertShape.
func typeOf(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	case []interface{}:
		if len(x) > 0 {
			if _, ok := x[0].(map[string]interface{}); ok {
				return "array-of-object"
			}
		}
		return "array"
	case map[string]interface{}:
		return "object"
	default:
		return fmt.Sprintf("%T", v)
	}
}

// assertShape compares a decoded payload against a map of expected types.
// Allowed expected strings: "string", "number", "boolean", "array",
// "array-of-object", "object", "object-or-null".
func assertShape(t *testing.T, context string, expected map[string]string, actual map[string]interface{}) {
	t.Helper()
	for field, wantType := range expected {
		v, ok := actual[field]
		if !ok {
			t.Errorf("%s: missing field %q", context, field)
			continue
		}
		gotType := typeOf(v)
		if wantType == "object-or-null" {
			if gotType != "object" && gotType != "null" {
				t.Errorf("%s: field %q has type %q, want object-or-null", context, field, gotType)
			}
			continue
		}
		if wantType == "array" && gotType == "array-of-object" {
			continue // array-of-object satisfies "array"
		}
		if gotType != wantType {
			t.Errorf("%s: field %q has type %q, want %q", context, field, gotType, wantType)
		}
	}
}

// --------------------------------------------------------------------------
// Integration tests — run the full stack for every sub-command.
// --------------------------------------------------------------------------

// TestIntegration_RequirementsList_JSONMatchesSchema validates
// `gh agentic status requirements --json` against the locked schema.
func TestIntegration_RequirementsList_JSONMatchesSchema(t *testing.T) {
	schema := loadSchema(t, "requirements_list.schema.json")
	buf := &bytes.Buffer{}
	if err := runStatusRequirements(buf, statusListFlags{json: true}, buildFixtureDeps()); err != nil {
		t.Fatalf("runStatusRequirements: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v\nraw:\n%s", err, buf.String())
	}
	assertShape(t, "envelope", schema.Envelope, payload)
	totals, _ := payload["totals"].(map[string]interface{})
	assertShape(t, "totals", schema.TotalsFields, totals)
	items, _ := payload["items"].([]interface{})
	if len(items) == 0 {
		t.Fatalf("expected at least 1 requirement; got 0")
	}
	for i, it := range items {
		assertShape(t, fmt.Sprintf("items[%d]", i), schema.ItemFields, it.(map[string]interface{}))
	}
}

// TestIntegration_RequirementDetail_JSONMatchesSchema validates the detail
// JSON schema including nested linked_features.
func TestIntegration_RequirementDetail_JSONMatchesSchema(t *testing.T) {
	schema := loadSchema(t, "requirement_detail.schema.json")
	buf := &bytes.Buffer{}
	if err := runStatusRequirement(buf, 457, statusDetailFlags{json: true}, buildFixtureDeps()); err != nil {
		t.Fatalf("runStatusRequirement: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v\nraw:\n%s", err, buf.String())
	}
	assertShape(t, "root", schema.RootFields, payload)
	lf, _ := payload["linked_features"].([]interface{})
	if len(lf) == 0 {
		t.Errorf("expected linked_features to contain feature #492")
	}
	for i, f := range lf {
		assertShape(t, fmt.Sprintf("linked_features[%d]", i), schema.LinkedFeatureFields, f.(map[string]interface{}))
	}
}

// TestIntegration_FeaturesList_JSONMatchesSchema validates the features list
// JSON shape.
func TestIntegration_FeaturesList_JSONMatchesSchema(t *testing.T) {
	schema := loadSchema(t, "features_list.schema.json")
	buf := &bytes.Buffer{}
	if err := runStatusFeatures(buf, statusListFlags{json: true}, buildFixtureDeps()); err != nil {
		t.Fatalf("runStatusFeatures: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v\nraw:\n%s", err, buf.String())
	}
	assertShape(t, "envelope", schema.Envelope, payload)
	items, _ := payload["items"].([]interface{})
	for i, it := range items {
		assertShape(t, fmt.Sprintf("items[%d]", i), schema.ItemFields, it.(map[string]interface{}))
	}
}

// TestIntegration_FeatureDetail_JSONMatchesSchema validates the feature
// detail JSON shape including nested tasks and branch/pr structures.
func TestIntegration_FeatureDetail_JSONMatchesSchema(t *testing.T) {
	schema := loadSchema(t, "feature_detail.schema.json")
	buf := &bytes.Buffer{}
	if err := runStatusFeature(buf, 492, statusDetailFlags{json: true}, buildFixtureDeps()); err != nil {
		t.Fatalf("runStatusFeature: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v\nraw:\n%s", err, buf.String())
	}
	assertShape(t, "root", schema.RootFields, payload)
	tasks, _ := payload["tasks"].([]interface{})
	if len(tasks) == 0 {
		t.Errorf("expected tasks to be populated for feature #492")
	}
	for i, task := range tasks {
		assertShape(t, fmt.Sprintf("tasks[%d]", i), schema.TaskFields, task.(map[string]interface{}))
	}
	if branch, ok := payload["branch"].(map[string]interface{}); ok {
		assertShape(t, "branch", schema.BranchFieldsWhenPresent, branch)
	}
	if pr, ok := payload["pr"].(map[string]interface{}); ok {
		assertShape(t, "pr", schema.PRFieldsWhenPresent, pr)
	}
}

// TestIntegration_JSONStabilityAcrossInvocations verifies byte-identical
// output across two runs of each sub-command — the hardest guarantee for
// downstream consumers.
func TestIntegration_JSONStabilityAcrossInvocations(t *testing.T) {
	runs := []struct {
		name string
		run  func(w *bytes.Buffer) error
	}{
		{"requirements list", func(w *bytes.Buffer) error {
			return runStatusRequirements(w, statusListFlags{json: true}, buildFixtureDeps())
		}},
		{"requirement detail", func(w *bytes.Buffer) error {
			return runStatusRequirement(w, 457, statusDetailFlags{json: true}, buildFixtureDeps())
		}},
		{"features list", func(w *bytes.Buffer) error {
			return runStatusFeatures(w, statusListFlags{json: true}, buildFixtureDeps())
		}},
		{"feature detail", func(w *bytes.Buffer) error {
			return runStatusFeature(w, 492, statusDetailFlags{json: true}, buildFixtureDeps())
		}},
	}
	for _, r := range runs {
		t.Run(r.name, func(t *testing.T) {
			bufA := &bytes.Buffer{}
			bufB := &bytes.Buffer{}
			if err := r.run(bufA); err != nil {
				t.Fatalf("%s run A: %v", r.name, err)
			}
			if err := r.run(bufB); err != nil {
				t.Fatalf("%s run B: %v", r.name, err)
			}
			if bufA.String() != bufB.String() {
				t.Errorf("%s: JSON output is not byte-identical across runs", r.name)
			}
		})
	}
}

// TestIntegration_HumanOutputHasFederatedRepoColumn verifies the features
// list in a federated project surfaces the REPO column with the expected
// cells.
func TestIntegration_HumanOutputHasFederatedRepoColumn(t *testing.T) {
	buf := &bytes.Buffer{}
	if err := runStatusFeatures(buf, statusListFlags{}, buildFixtureDeps()); err != nil {
		t.Fatalf("runStatusFeatures: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"REPO", "foo/domain-x", "(this repo)", "#492", "#511"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in federated human output; got:\n%s", want, out)
		}
	}
}

// TestIntegration_KanbanVerticalPerEntity verifies the vertical kanban view
// renders correctly for both entities.
func TestIntegration_KanbanVerticalPerEntity(t *testing.T) {
	// Requirements.
	reqBuf := &bytes.Buffer{}
	if err := runStatusRequirements(reqBuf, statusListFlags{kanban: true}, buildFixtureDeps()); err != nil {
		t.Fatalf("requirements kanban: %v", err)
	}
	if !strings.Contains(reqBuf.String(), "Requirements — Kanban") {
		t.Errorf("missing requirements heading; got:\n%s", reqBuf.String())
	}

	// Features.
	feaBuf := &bytes.Buffer{}
	if err := runStatusFeatures(feaBuf, statusListFlags{kanban: true}, buildFixtureDeps()); err != nil {
		t.Fatalf("features kanban: %v", err)
	}
	if !strings.Contains(feaBuf.String(), "Features — Kanban") {
		t.Errorf("missing features heading; got:\n%s", feaBuf.String())
	}
}

// TestIntegration_KanbanHorizontalWide verifies horizontal kanban succeeds
// on a wide terminal.
func TestIntegration_KanbanHorizontalWide(t *testing.T) {
	originalWidth := terminalWidth
	terminalWidth = func() int { return 200 }
	defer func() { terminalWidth = originalWidth }()

	buf := &bytes.Buffer{}
	if err := runStatusFeatures(buf, statusListFlags{kanban: true, horizontal: true}, buildFixtureDeps()); err != nil {
		t.Fatalf("wide horizontal kanban: %v", err)
	}
	if !strings.ContainsAny(buf.String(), "┌+") {
		t.Errorf("expected box-drawing border; got:\n%s", buf.String())
	}
}

// TestIntegration_KanbanHorizontalNarrowStillRenders verifies that an
// explicit --horizontal on a narrow terminal renders horizontal without
// error — the user's choice is honoured even if the table overflows.
func TestIntegration_KanbanHorizontalNarrowStillRenders(t *testing.T) {
	originalWidth := terminalWidth
	terminalWidth = func() int { return 80 }
	defer func() { terminalWidth = originalWidth }()

	buf := &bytes.Buffer{}
	if err := runStatusFeatures(buf, statusListFlags{kanban: true, horizontal: true}, buildFixtureDeps()); err != nil {
		t.Fatalf("--horizontal on narrow terminal should not error: %v", err)
	}
	out := buf.String()
	if !strings.ContainsAny(out, "┌+") {
		t.Errorf("expected horizontal borders; got:\n%s", out)
	}
	if strings.Contains(out, "horizontal kanban needs ≥") {
		t.Errorf("--horizontal must not emit the fallback notice; got:\n%s", out)
	}
}

// TestIntegration_KanbanDefaultNarrowAutoFallsBack verifies the features
// kanban default on a narrow terminal auto-falls-back to vertical with a
// notice and exits without error.
func TestIntegration_KanbanDefaultNarrowAutoFallsBack(t *testing.T) {
	originalWidth := terminalWidth
	terminalWidth = func() int { return 80 }
	defer func() { terminalWidth = originalWidth }()

	buf := &bytes.Buffer{}
	if err := runStatusFeatures(buf, statusListFlags{kanban: true}, buildFixtureDeps()); err != nil {
		t.Fatalf("default narrow kanban should not error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "## backlog") {
		t.Errorf("expected vertical fallback section headings; got:\n%s", out)
	}
	if !strings.Contains(out, "terminal 80 cols") {
		t.Errorf("expected fallback notice naming current width; got:\n%s", out)
	}
}

// TestIntegration_ThisRepoNarrowing verifies --this-repo filters to the
// current repo on both list commands.
func TestIntegration_ThisRepoNarrowing(t *testing.T) {
	buf := &bytes.Buffer{}
	if err := runStatusFeatures(buf, statusListFlags{thisRepo: true}, buildFixtureDeps()); err != nil {
		t.Fatalf("runStatusFeatures: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "#511") {
		t.Errorf("--this-repo should exclude foreign feature #511; got:\n%s", out)
	}
	if !strings.Contains(out, "#492") {
		t.Errorf("--this-repo should include local feature #492; got:\n%s", out)
	}
}

// TestIntegration_IncludeDoneAddsDoneColumn verifies --include-done surfaces
// the done column in the kanban view and closed items in the list view.
func TestIntegration_IncludeDoneAddsDoneColumn(t *testing.T) {
	buf := &bytes.Buffer{}
	if err := runStatusRequirements(buf, statusListFlags{kanban: true, includeDone: true}, buildFixtureDeps()); err != nil {
		t.Fatalf("runStatusRequirements: %v", err)
	}
	if !strings.Contains(buf.String(), "## done") {
		t.Errorf("expected '## done' column with --include-done; got:\n%s", buf.String())
	}
}

// TestIntegration_BlockedAnnotationEndToEnd verifies the blocked annotation
// appears in list views and the Blocked line appears in detail.
func TestIntegration_BlockedAnnotationEndToEnd(t *testing.T) {
	buf := &bytes.Buffer{}
	if err := runStatusFeatures(buf, statusListFlags{}, buildFixtureDeps()); err != nil {
		t.Fatalf("runStatusFeatures: %v", err)
	}
	if !strings.Contains(buf.String(), "[blocked by foo/domain-x#507]") {
		t.Errorf("expected blocked annotation on #511; got:\n%s", buf.String())
	}

	detailBuf := &bytes.Buffer{}
	if err := runStatusFeature(detailBuf, 511, statusDetailFlags{}, buildFixtureDeps()); err != nil {
		t.Fatalf("runStatusFeature: %v", err)
	}
	if !strings.Contains(detailBuf.String(), "Blocked: foo/domain-x#507") {
		t.Errorf("expected Blocked: line in detail; got:\n%s", detailBuf.String())
	}
}

// TestIntegration_ErrorPaths exercises the classified error scenarios through
// the command handlers to confirm each produces the documented message.
func TestIntegration_ErrorPaths(t *testing.T) {
	t.Run("missing project id", func(t *testing.T) {
		sd := statusDeps{
			currentRepo:      func() (string, error) { return "owner/repo", nil },
			resolveProjectID: func(string) (string, error) { return "", nil },
			psDeps:           projectstatus.Deps{},
		}
		err := runStatusRequirements(&bytes.Buffer{}, statusListFlags{}, sd)
		if !errors.Is(err, projectstatus.ErrProjectNotConfigured) {
			t.Errorf("expected ErrProjectNotConfigured; got %v", err)
		}
	})

	t.Run("unknown issue number", func(t *testing.T) {
		err := runStatusRequirement(&bytes.Buffer{}, 9999, statusDetailFlags{}, buildFixtureDeps())
		if !errors.Is(err, projectstatus.ErrIssueNotFound) {
			t.Errorf("expected ErrIssueNotFound; got %v", err)
		}
	})

	t.Run("wrong type requirement->feature", func(t *testing.T) {
		err := runStatusFeature(&bytes.Buffer{}, 457, statusDetailFlags{}, buildFixtureDeps())
		var wt *projectstatus.ErrWrongType
		if !errors.As(err, &wt) {
			t.Fatalf("expected ErrWrongType; got %v", err)
		}
		if wt.ActualType != "requirement" || wt.WantedType != "feature" {
			t.Errorf("unexpected wrong-type fields: %+v", wt)
		}
	})

	t.Run("wrong type feature->requirement", func(t *testing.T) {
		err := runStatusRequirement(&bytes.Buffer{}, 492, statusDetailFlags{}, buildFixtureDeps())
		var wt *projectstatus.ErrWrongType
		if !errors.As(err, &wt) {
			t.Fatalf("expected ErrWrongType; got %v", err)
		}
	})
}

// TestIntegration_JSONKanbanPrecedence verifies --kanban with --json returns
// JSON, not a kanban layout — the documented precedence.
func TestIntegration_JSONKanbanPrecedence(t *testing.T) {
	buf := &bytes.Buffer{}
	if err := runStatusRequirements(buf, statusListFlags{kanban: true, json: true}, buildFixtureDeps()); err != nil {
		t.Fatalf("runStatusRequirements: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"items":`) {
		t.Errorf("expected JSON envelope; got:\n%s", out)
	}
	if strings.Contains(out, "## backlog") || strings.Contains(out, "Kanban") {
		t.Errorf("kanban text leaked into JSON output:\n%s", out)
	}
}

// TestIntegration_BareStatusPrintsHelp verifies `status` with no sub-command
// prints help with all four leaf sub-commands listed.
func TestIntegration_BareStatusPrintsHelp(t *testing.T) {
	cmd := newStatusCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("bare status: %v", err)
	}
	for _, tok := range []string{"requirements", "requirement", "features", "feature"} {
		if !strings.Contains(buf.String(), tok) {
			t.Errorf("help missing %q; got:\n%s", tok, buf.String())
		}
	}
}
