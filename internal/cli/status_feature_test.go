package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/eddiecarpenter/gh-agentic/internal/projectstatus"
	"github.com/eddiecarpenter/gh-agentic/internal/testutil"
)

// featureDetailFixture builds a statusDeps with injectable handlers for the
// feature-detail composer (FetchProjectIssues + FetchSubIssues +
// FetchParentIssue + FetchBranch + FetchPR).
func featureDetailFixture(issues []projectstatus.ProjectIssue, subIssues map[int][]projectstatus.TaskRef, parent map[int]*projectstatus.RequirementSummary, branches map[string]*projectstatus.BranchState, prs map[string]*projectstatus.PRState) statusDeps {
	ps := projectstatus.Deps{
		FetchProjectIssues: func(string) ([]projectstatus.ProjectIssue, error) { return issues, nil },
		FetchSubIssues: func(_, _ string, n int) ([]projectstatus.TaskRef, error) {
			return subIssues[n], nil
		},
		FetchParentIssue: func(_, _ string, n int) (*projectstatus.RequirementSummary, error) {
			return parent[n], nil
		},
		FetchBranch: func(_, _, name string) (*projectstatus.BranchState, error) {
			if b, ok := branches[name]; ok {
				return b, nil
			}
			return &projectstatus.BranchState{Name: name, Exists: false}, nil
		},
		FetchPR: func(_, _, name string) (*projectstatus.PRState, error) {
			return prs[name], nil
		},
	}
	return statusDeps{
		currentRepo:      func() (string, error) { return "eddiecarpenter/gh-agentic", nil },
		resolveProjectID: func(string) (string, error) { return "PROJ_ID", nil },
		psDeps:           ps,
		busy:             testutil.NoopBusy,
	}
}

// TestRunStatusFeature_DefaultDetailOutput verifies the UX-3 layout for a
// feature with every nested resource populated.
func TestRunStatusFeature_DefaultDetailOutput(t *testing.T) {
	now := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	issues := []projectstatus.ProjectIssue{
		{Number: 492, Title: "feat: status command", Body: "## User Story\nbody", Stage: projectstatus.StageInDevelopment, Type: "feature", State: "open", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: now, LastTransitionedAt: now},
	}
	subIssues := map[int][]projectstatus.TaskRef{
		492: {
			{Number: 494, Title: "Scaffold", Closed: true},
			{Number: 495, Title: "Types", Closed: true},
			{Number: 496, Title: "Requirements list", Closed: false},
		},
	}
	parent := map[int]*projectstatus.RequirementSummary{
		492: {Number: 457, Title: "feat: single-pane view", Stage: projectstatus.StageScoping, OwningRepo: "eddiecarpenter/gh-agentic"},
	}
	branches := map[string]*projectstatus.BranchState{
		"feature/492": {Name: "feature/492", Exists: true, Merged: false},
	}
	prs := map[string]*projectstatus.PRState{
		"feature/492": {Number: 777, State: "open", Reviewers: []string{"eddie"}},
	}
	sd := featureDetailFixture(issues, subIssues, parent, branches, prs)

	buf := &bytes.Buffer{}
	if err := runStatusFeature(buf, io.Discard, 492, statusDetailFlags{}, sd); err != nil {
		t.Fatalf("runStatusFeature: %v", err)
	}
	out := buf.String()
	for _, token := range []string{
		"feat: status command",
		"Stage: in-development",
		"Created: 2026-04-18",
		"User Story",
		"---",
		"Parent requirement:  #457 [scoping]  feat: single-pane view",
		"Branch:              feature/492",
		"PR:                  #777 (open) — reviewers: eddie",
		"Tasks:               2 / 3 done",
		"#494",
		"#495",
		"#496",
	} {
		if !strings.Contains(out, token) {
			t.Errorf("expected output to contain %q; got:\n%s", token, out)
		}
	}
}

// TestRunStatusFeature_EmptyResourcesRenderCleanly verifies a feature with
// no parent, no branch, no PR, and zero tasks renders without "nil" strings
// or panics.
func TestRunStatusFeature_EmptyResourcesRenderCleanly(t *testing.T) {
	now := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	issues := []projectstatus.ProjectIssue{
		{Number: 10, Title: "feat: naked", Stage: projectstatus.StageBacklog, Type: "feature", State: "open", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: now, LastTransitionedAt: now},
	}
	sd := featureDetailFixture(issues, nil, nil, nil, nil)
	buf := &bytes.Buffer{}
	if err := runStatusFeature(buf, io.Discard, 10, statusDetailFlags{}, sd); err != nil {
		t.Fatalf("runStatusFeature: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "nil") || strings.Contains(out, "<nil>") {
		t.Errorf("detail output should not render 'nil'; got:\n%s", out)
	}
	for _, expected := range []string{
		"Parent requirement:  (none)",
		"Branch:              (no branch yet)",
		"PR:                  (no PR opened)",
		"Tasks:               0 / 0 done",
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("expected %q; got:\n%s", expected, out)
		}
	}
}

// TestRunStatusFeature_BlockedLineRenders verifies the Blocked line appears
// when Blocked is non-nil.
func TestRunStatusFeature_BlockedLineRenders(t *testing.T) {
	f := &projectstatus.Feature{
		Number:  511,
		Title:   "feat: blocked",
		Stage:   projectstatus.StageInDevelopment,
		Blocked: &projectstatus.BlockedInfo{BlockingRef: "a/b#99", Reason: "waiting"},
	}
	buf := &bytes.Buffer{}
	if err := writeFeatureDetail(buf, f, true); err != nil {
		t.Fatalf("writeFeatureDetail: %v", err)
	}
	if !strings.Contains(buf.String(), "Blocked: waiting (a/b#99)") {
		t.Errorf("expected blocked line; got:\n%s", buf.String())
	}
}

// TestRunStatusFeature_ASCIIFallback verifies ASCII glyphs are used when
// utf8 support is flagged as false.
func TestRunStatusFeature_ASCIIFallback(t *testing.T) {
	f := &projectstatus.Feature{
		Number: 10,
		Title:  "t",
		Tasks: []projectstatus.TaskRef{
			{Number: 1, Title: "done", Closed: true},
			{Number: 2, Title: "open", Closed: false},
		},
	}
	buf := &bytes.Buffer{}
	if err := writeFeatureDetail(buf, f, false); err != nil {
		t.Fatalf("writeFeatureDetail: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "[x]") || !strings.Contains(out, "[ ]") {
		t.Errorf("expected ASCII fallback glyphs; got:\n%s", out)
	}
	if strings.Contains(out, "✓") || strings.Contains(out, "☐") {
		t.Errorf("unicode glyphs leaked into ASCII output:\n%s", out)
	}
}

// TestRunStatusFeature_UnicodeGlyphs verifies utf8 mode uses ✓ and ☐.
func TestRunStatusFeature_UnicodeGlyphs(t *testing.T) {
	f := &projectstatus.Feature{
		Number: 10,
		Title:  "t",
		Tasks: []projectstatus.TaskRef{
			{Number: 1, Title: "done", Closed: true},
			{Number: 2, Title: "open", Closed: false},
		},
	}
	buf := &bytes.Buffer{}
	if err := writeFeatureDetail(buf, f, true); err != nil {
		t.Fatalf("writeFeatureDetail: %v", err)
	}
	if !strings.Contains(buf.String(), "✓") || !strings.Contains(buf.String(), "☐") {
		t.Errorf("expected unicode glyphs; got:\n%s", buf.String())
	}
}

// TestRunStatusFeature_JSONSchema verifies --json emits a single object with
// every nested resource present (or null when absent).
func TestRunStatusFeature_JSONSchema(t *testing.T) {
	now := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	issues := []projectstatus.ProjectIssue{
		{Number: 492, Title: "feat: status command", Body: "b", Stage: projectstatus.StageInDevelopment, Type: "feature", State: "open", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: now, LastTransitionedAt: now},
	}
	sd := featureDetailFixture(issues, nil, nil, nil, nil)

	buf := &bytes.Buffer{}
	if err := runStatusFeature(buf, io.Discard, 492, statusDetailFlags{json: true}, sd); err != nil {
		t.Fatalf("runStatusFeature: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("json parse: %v; raw:\n%s", err, buf.String())
	}
	for _, key := range []string{"number", "title", "body", "stage", "created_at", "last_transitioned_at", "owning_repo", "blocked", "parent_requirement", "tasks", "branch", "pr"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("JSON missing key %q; keys = %v", key, keysOf(parsed))
		}
	}
	if parsed["parent_requirement"] != nil {
		t.Errorf("parent_requirement = %v, want null", parsed["parent_requirement"])
	}
	if parsed["pr"] != nil {
		t.Errorf("pr = %v, want null", parsed["pr"])
	}
	if _, ok := parsed["tasks"].([]interface{}); !ok {
		t.Errorf("tasks should be [] (non-null); got %v", parsed["tasks"])
	}
}

// TestRunStatusFeature_NotFound verifies ErrIssueNotFound is annotated with
// repo / number.
func TestRunStatusFeature_NotFound(t *testing.T) {
	sd := featureDetailFixture(nil, nil, nil, nil, nil)
	err := runStatusFeature(&bytes.Buffer{}, io.Discard, 9999, statusDetailFlags{}, sd)
	if !errors.Is(err, projectstatus.ErrIssueNotFound) {
		t.Fatalf("expected ErrIssueNotFound; got %v", err)
	}
	if !strings.Contains(err.Error(), "#9999") {
		t.Errorf("error should mention #9999; got %v", err)
	}
}

// TestRunStatusFeature_WrongType verifies a requirement number passed to the
// feature detail returns *ErrWrongType with fields set correctly.
func TestRunStatusFeature_WrongType(t *testing.T) {
	now := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	issues := []projectstatus.ProjectIssue{
		{Number: 457, Title: "requirement", Type: "requirement", Stage: projectstatus.StageScoping, State: "open", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: now, LastTransitionedAt: now},
	}
	sd := featureDetailFixture(issues, nil, nil, nil, nil)
	err := runStatusFeature(&bytes.Buffer{}, io.Discard, 457, statusDetailFlags{}, sd)

	var wt *projectstatus.ErrWrongType
	if !errors.As(err, &wt) {
		t.Fatalf("expected *ErrWrongType; got %v", err)
	}
	if wt.ActualType != "requirement" || wt.WantedType != "feature" {
		t.Errorf("wrong-type fields: %+v", wt)
	}
}

// TestRunStatusFeature_RawVerbatimBody verifies the `--raw` renderer emits
// the feature-specific frontmatter header, the literal `---` separator, and
// the body verbatim — markdown headings, fenced code, and a `→` character
// all survive without escaping. The rendered bytes must match the golden.
func TestRunStatusFeature_RawVerbatimBody(t *testing.T) {
	now := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	body := "## User Story\n\n" +
		"As an agent, I want a status command so I can answer \"where are we?\".\n\n" +
		"```go\nstatus := \"in-development\"\n```\n\n" +
		"Flow: backlog → scoping → in-development."
	issues := []projectstatus.ProjectIssue{
		{Number: 492, Title: "feat: status command", Body: body, Stage: projectstatus.StageInDevelopment, Type: "feature", State: "open", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: now, LastTransitionedAt: now},
	}
	subIssues := map[int][]projectstatus.TaskRef{
		492: {
			{Number: 494, Title: "Scaffold", Closed: true},
			{Number: 495, Title: "Types", Closed: true},
			{Number: 496, Title: "Requirements list", Closed: false},
		},
	}
	parent := map[int]*projectstatus.RequirementSummary{
		492: {Number: 457, Title: "feat: single-pane view", Stage: projectstatus.StageScoping, OwningRepo: "eddiecarpenter/gh-agentic"},
	}
	branches := map[string]*projectstatus.BranchState{
		"feature/492": {Name: "feature/492", Exists: true, Merged: true},
	}
	prs := map[string]*projectstatus.PRState{
		"feature/492": {Number: 777, State: "open", Reviewers: []string{"eddie"}},
	}
	sd := featureDetailFixture(issues, subIssues, parent, branches, prs)

	buf := &bytes.Buffer{}
	if err := runStatusFeature(buf, io.Discard, 492, statusDetailFlags{raw: true}, sd); err != nil {
		t.Fatalf("runStatusFeature --raw: %v", err)
	}
	got := buf.Bytes()

	wantBytes, err := os.ReadFile("testdata/status_raw/feature_detail.raw")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !bytes.Equal(got, wantBytes) {
		t.Errorf("--raw output does not match golden\nwant:\n%s\ngot:\n%s", string(wantBytes), string(got))
	}

	// AC-2 reinforcement on the feature side: body region must be verbatim.
	parts := strings.SplitN(string(got), "\n---\n", 2)
	if len(parts) != 2 {
		t.Fatalf("expected '---' separator on its own line; got:\n%s", string(got))
	}
	bodyOut := parts[1]
	for _, marker := range []string{"## User Story", "```go", "→"} {
		if !strings.Contains(bodyOut, marker) {
			t.Errorf("body should contain %q verbatim; got:\n%s", marker, bodyOut)
		}
	}
	for _, escape := range []string{`\n`, "\\`", `\u2192`} {
		if strings.Contains(bodyOut, escape) {
			t.Errorf("body should not contain escape sequence %q; got:\n%s", escape, bodyOut)
		}
	}
}

// TestRunStatusFeature_RawSeparatorAlwaysPresent verifies the `---`
// separator is emitted even when the issue body is empty.
func TestRunStatusFeature_RawSeparatorAlwaysPresent(t *testing.T) {
	now := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	issues := []projectstatus.ProjectIssue{
		{Number: 700, Title: "feat: empty", Body: "", Stage: projectstatus.StageBacklog, Type: "feature", State: "open", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: now, LastTransitionedAt: now},
	}
	sd := featureDetailFixture(issues, nil, nil, nil, nil)
	buf := &bytes.Buffer{}
	if err := runStatusFeature(buf, io.Discard, 700, statusDetailFlags{raw: true}, sd); err != nil {
		t.Fatalf("runStatusFeature --raw: %v", err)
	}
	if !strings.Contains(buf.String(), "\n---\n") {
		t.Errorf("expected '---' separator even for empty body; got:\n%s", buf.String())
	}
}

// TestParentRequirementOneLiner covers nil and normal rendering paths.
func TestParentRequirementOneLiner(t *testing.T) {
	if got := parentRequirementOneLiner(nil); got != "" {
		t.Errorf("nil parent should render empty string; got %q", got)
	}
	got := parentRequirementOneLiner(&projectstatus.RequirementSummary{Number: 1, Stage: projectstatus.StageDone, Title: "t"})
	if got != "#1 [done]  t" {
		t.Errorf("unexpected one-liner %q", got)
	}
}

// TestRenderFeatureBranchOneLiner covers merged / non-merged / absent paths.
func TestRenderFeatureBranchOneLiner(t *testing.T) {
	if got := renderFeatureBranchOneLiner(nil); got != "" {
		t.Errorf("nil branch should render empty; got %q", got)
	}
	if got := renderFeatureBranchOneLiner(&projectstatus.BranchState{Name: "x", Exists: false}); got != "" {
		t.Errorf("absent branch should render empty; got %q", got)
	}
	if got := renderFeatureBranchOneLiner(&projectstatus.BranchState{Name: "x", Exists: true, Merged: false}); got != "x" {
		t.Errorf("non-merged branch; got %q", got)
	}
	if got := renderFeatureBranchOneLiner(&projectstatus.BranchState{Name: "x", Exists: true, Merged: true}); got != "x (merged)" {
		t.Errorf("merged branch; got %q", got)
	}
}
