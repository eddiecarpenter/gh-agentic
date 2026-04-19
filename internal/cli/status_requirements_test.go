package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/eddiecarpenter/gh-agentic/internal/project"
	"github.com/eddiecarpenter/gh-agentic/internal/projectstatus"
	"github.com/eddiecarpenter/gh-agentic/internal/testutil"
)

// fakeProjectstatusDeps builds a projectstatus.Deps populated from an
// in-memory slice of project issues — used to drive the list handler
// deterministically in unit tests.
func fakeProjectstatusDeps(issues []projectstatus.ProjectIssue) projectstatus.Deps {
	return projectstatus.Deps{
		FetchProjectIssues: func(projectID string) ([]projectstatus.ProjectIssue, error) {
			return issues, nil
		},
		FetchLinkedRepos: func(projectID string) ([]project.LinkedRepo, error) {
			return []project.LinkedRepo{{NameWithOwner: "eddiecarpenter/gh-agentic"}}, nil
		},
	}
}

// sampleRequirementIssues returns a deterministic fixture of mixed-type
// issues for requirements-list rendering tests.
func sampleRequirementIssues() []projectstatus.ProjectIssue {
	now := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	return []projectstatus.ProjectIssue{
		{Number: 467, Title: "feat: skill-publishing", Stage: projectstatus.StageBacklog, Type: "requirement", State: "open", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: now, LastTransitionedAt: now},
		{Number: 457, Title: "feat: single-pane pipeline status view", Stage: projectstatus.StageScoping, Type: "requirement", State: "open", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: now, LastTransitionedAt: now},
		{Number: 447, Title: "feat: project lifecycle management", Stage: projectstatus.StageBacklog, Type: "requirement", State: "open", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: now, LastTransitionedAt: now},
		{Number: 466, Title: "feat: ask-user", Stage: projectstatus.StageDone, Type: "requirement", State: "closed", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: now, LastTransitionedAt: now},
		// Non-requirement — must be filtered out.
		{Number: 492, Title: "feat: status command", Stage: projectstatus.StageInDevelopment, Type: "feature", State: "open", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: now, LastTransitionedAt: now},
	}
}

// fakeStatusDeps returns a statusDeps with the given issues and a deterministic
// current-repo / project ID wiring.
func fakeStatusDeps(issues []projectstatus.ProjectIssue) statusDeps {
	return statusDeps{
		currentRepo:      func() (string, error) { return "eddiecarpenter/gh-agentic", nil },
		resolveProjectID: func(string) (string, error) { return "PROJ_ID", nil },
		psDeps:           fakeProjectstatusDeps(issues),
		busy:             testutil.NoopBusy,
	}
}

// TestRunStatusRequirements_DefaultTableExcludesDone verifies default
// invocation lists only open items with fixed column headers and a totals
// line.
func TestRunStatusRequirements_DefaultTableExcludesDone(t *testing.T) {
	buf := &bytes.Buffer{}
	err := runStatusRequirements(buf, io.Discard, statusListFlags{}, fakeStatusDeps(sampleRequirementIssues()))
	if err != nil {
		t.Fatalf("runStatusRequirements: %v", err)
	}
	out := buf.String()

	for _, token := range []string{"REQUIREMENT", "STAGE", "TITLE", "#447", "#457", "#467"} {
		if !strings.Contains(out, token) {
			t.Errorf("expected output to contain %q, got:\n%s", token, out)
		}
	}
	if strings.Contains(out, "#466") {
		t.Errorf("closed requirement #466 should be excluded by default; output:\n%s", out)
	}
	if !strings.Contains(out, "3 open requirements") {
		t.Errorf("expected totals '3 open requirements', got:\n%s", out)
	}
}

// TestRunStatusRequirements_IncludeDonePullsInClosed verifies --include-done
// surfaces closed items and updates the totals.
func TestRunStatusRequirements_IncludeDonePullsInClosed(t *testing.T) {
	buf := &bytes.Buffer{}
	err := runStatusRequirements(buf, io.Discard, statusListFlags{includeDone: true}, fakeStatusDeps(sampleRequirementIssues()))
	if err != nil {
		t.Fatalf("runStatusRequirements: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "#466") {
		t.Errorf("expected closed requirement #466 to be listed; output:\n%s", out)
	}
	if !strings.Contains(out, "4 open requirements") {
		t.Errorf("expected totals '4 open requirements', got:\n%s", out)
	}
}

// TestRunStatusRequirements_JSONEnvelopeShape verifies --json produces the
// {items, totals} envelope and that the shape is parseable and matches the
// locked field names.
func TestRunStatusRequirements_JSONEnvelopeShape(t *testing.T) {
	buf := &bytes.Buffer{}
	err := runStatusRequirements(buf, io.Discard, statusListFlags{json: true}, fakeStatusDeps(sampleRequirementIssues()))
	if err != nil {
		t.Fatalf("runStatusRequirements: %v", err)
	}

	var parsed struct {
		Items  []map[string]interface{} `json:"items"`
		Totals struct {
			Open    int `json:"open"`
			Blocked int `json:"blocked"`
		} `json:"totals"`
	}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("--json output did not parse: %v\nraw:\n%s", err, buf.String())
	}
	if parsed.Totals.Open != 3 {
		t.Errorf("totals.open = %d, want 3", parsed.Totals.Open)
	}
	if parsed.Totals.Blocked != 0 {
		t.Errorf("totals.blocked = %d, want 0", parsed.Totals.Blocked)
	}
	if len(parsed.Items) != 3 {
		t.Fatalf("len(items) = %d, want 3", len(parsed.Items))
	}
	// Field-name check — every item must declare the documented keys so the
	// schema is stable across runs.
	requiredKeys := []string{"number", "title", "body", "stage", "created_at", "last_transitioned_at", "owning_repo", "blocked", "linked_features"}
	for i, item := range parsed.Items {
		for _, k := range requiredKeys {
			if _, ok := item[k]; !ok {
				t.Errorf("item[%d] missing required key %q; keys = %v", i, k, keysOf(item))
			}
		}
	}
}

// TestRunStatusRequirements_JSONEnvelopeEmpty verifies the envelope renders
// "items": [] rather than null when no requirements match.
func TestRunStatusRequirements_JSONEnvelopeEmpty(t *testing.T) {
	buf := &bytes.Buffer{}
	err := runStatusRequirements(buf, io.Discard, statusListFlags{json: true}, fakeStatusDeps(nil))
	if err != nil {
		t.Fatalf("runStatusRequirements: %v", err)
	}
	if !strings.Contains(buf.String(), `"items": []`) {
		t.Errorf("expected empty envelope to contain \"items\": []; got:\n%s", buf.String())
	}
}

// TestRunStatusRequirements_ThisRepoNarrows verifies --this-repo drops
// items from other repos.
func TestRunStatusRequirements_ThisRepoNarrows(t *testing.T) {
	mixed := []projectstatus.ProjectIssue{
		{Number: 457, Title: "local", Type: "requirement", Stage: projectstatus.StageScoping, State: "open", OwningRepo: "eddiecarpenter/gh-agentic"},
		{Number: 600, Title: "other", Type: "requirement", Stage: projectstatus.StageBacklog, State: "open", OwningRepo: "foo/bar"},
	}
	buf := &bytes.Buffer{}
	err := runStatusRequirements(buf, io.Discard, statusListFlags{thisRepo: true}, fakeStatusDeps(mixed))
	if err != nil {
		t.Fatalf("runStatusRequirements: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "#600") {
		t.Errorf("foreign repo #600 should be filtered by --this-repo; output:\n%s", out)
	}
	if !strings.Contains(out, "#457") {
		t.Errorf("local #457 should be listed by --this-repo; output:\n%s", out)
	}
}

// TestRunStatusRequirements_BlockedAnnotation verifies the inline
// [blocked by owner/repo#N] annotation renders in both table and JSON paths,
// and that the blocked totals count is incremented.
func TestRunStatusRequirements_BlockedAnnotation(t *testing.T) {
	issues := []projectstatus.ProjectIssue{
		{Number: 111, Title: "blocked-req", Type: "requirement", Stage: projectstatus.StageScoping, State: "open", OwningRepo: "eddiecarpenter/gh-agentic"},
	}
	// Inject a blocked annotation via a custom projectstatus.Deps that
	// annotates the fetched requirement. We do it by wrapping the fake.
	deps := fakeProjectstatusDeps(issues)
	originalFetch := deps.FetchProjectIssues
	deps.FetchProjectIssues = func(projectID string) ([]projectstatus.ProjectIssue, error) {
		items, err := originalFetch(projectID)
		return items, err
	}
	// We cannot write Blocked onto a ProjectIssue (the field is on Requirement
	// itself). Instead, write a blocked Requirement via a post-fetch hook:
	// swap in a projectstatus.Deps with a patched FetchProjectIssues.
	// Since the fetcher returns ProjectIssue (no Blocked field), the
	// production wiring uses a separate blocked-by layer (task #501). For
	// this test we exercise the renderer directly by building a Requirement
	// slice.
	sd := statusDeps{
		currentRepo:      func() (string, error) { return "eddiecarpenter/gh-agentic", nil },
		resolveProjectID: func(string) (string, error) { return "PROJ_ID", nil },
		psDeps:           deps,
		busy:             testutil.NoopBusy,
	}

	buf := &bytes.Buffer{}
	if err := runStatusRequirements(buf, io.Discard, statusListFlags{}, sd); err != nil {
		t.Fatalf("runStatusRequirements: %v", err)
	}

	// Direct renderer test — construct a blocked Requirement and render it.
	reqs := []projectstatus.Requirement{{
		Number:     222,
		Title:      "needs dep",
		Stage:      projectstatus.StageBacklog,
		OwningRepo: "eddiecarpenter/gh-agentic",
		Blocked:    &projectstatus.BlockedInfo{BlockingRef: "foo/bar#99"},
	}}
	out := &bytes.Buffer{}
	if err := writeRequirementsTable(out, reqs, "eddiecarpenter/gh-agentic"); err != nil {
		t.Fatalf("writeRequirementsTable: %v", err)
	}
	if !strings.Contains(out.String(), "[blocked by foo/bar#99]") {
		t.Errorf("expected blocked annotation; got:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "1 open requirement (1 blocked)") {
		t.Errorf("expected blocked totals; got:\n%s", out.String())
	}

	// JSON path — confirm the blocked totals increment and the item carries
	// the blocked payload.
	jsonBuf := &bytes.Buffer{}
	if err := writeRequirementsJSON(jsonBuf, reqs); err != nil {
		t.Fatalf("writeRequirementsJSON: %v", err)
	}
	var parsed struct {
		Items []struct {
			Blocked *projectstatus.BlockedInfo `json:"blocked"`
		} `json:"items"`
		Totals projectstatus.ListTotals `json:"totals"`
	}
	if err := json.Unmarshal(jsonBuf.Bytes(), &parsed); err != nil {
		t.Fatalf("json decode: %v\nraw: %s", err, jsonBuf.String())
	}
	if parsed.Totals.Blocked != 1 {
		t.Errorf("totals.blocked = %d, want 1", parsed.Totals.Blocked)
	}
	if parsed.Items[0].Blocked == nil || parsed.Items[0].Blocked.BlockingRef != "foo/bar#99" {
		t.Errorf("blocked payload wrong: %+v", parsed.Items[0].Blocked)
	}
}

// TestRunStatusRequirements_NoProjectConfigured verifies a clean error path
// when AGENTIC_PROJECT_ID is not set.
func TestRunStatusRequirements_NoProjectConfigured(t *testing.T) {
	sd := statusDeps{
		currentRepo:      func() (string, error) { return "eddiecarpenter/gh-agentic", nil },
		resolveProjectID: func(string) (string, error) { return "", nil },
		psDeps:           projectstatus.Deps{},
		busy:             testutil.NoopBusy,
	}
	err := runStatusRequirements(&bytes.Buffer{}, io.Discard, statusListFlags{}, sd)
	if !errors.Is(err, projectstatus.ErrProjectNotConfigured) {
		t.Errorf("expected ErrProjectNotConfigured, got %v", err)
	}
}

// TestRunStatusRequirements_ShowsRepoColumnWhenFederated verifies that the
// REPO column appears only when some row is cross-repo, and "(this repo)" is
// rendered for local rows.
func TestRunStatusRequirements_ShowsRepoColumnWhenFederated(t *testing.T) {
	issues := []projectstatus.ProjectIssue{
		{Number: 111, Title: "local", Type: "requirement", Stage: projectstatus.StageBacklog, State: "open", OwningRepo: "eddiecarpenter/gh-agentic"},
		{Number: 222, Title: "remote", Type: "requirement", Stage: projectstatus.StageScoping, State: "open", OwningRepo: "foo/other"},
	}
	buf := &bytes.Buffer{}
	if err := runStatusRequirements(buf, io.Discard, statusListFlags{}, fakeStatusDeps(issues)); err != nil {
		t.Fatalf("runStatusRequirements: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "REPO") {
		t.Errorf("expected REPO column when federated; got:\n%s", out)
	}
	if !strings.Contains(out, "(this repo)") {
		t.Errorf("expected '(this repo)' for local row; got:\n%s", out)
	}
	if !strings.Contains(out, "foo/other") {
		t.Errorf("expected 'foo/other' for cross-repo row; got:\n%s", out)
	}
}

// TestRequirementsTotalsLine covers singular / plural and blocked branches.
func TestRequirementsTotalsLine(t *testing.T) {
	cases := []struct {
		n, blocked int
		expected   string
	}{
		{n: 0, blocked: 0, expected: "0 open requirements"},
		{n: 1, blocked: 0, expected: "1 open requirement"},
		{n: 3, blocked: 1, expected: "3 open requirements (1 blocked)"},
	}
	for _, tc := range cases {
		got := requirementsTotalsLine(tc.n, tc.blocked)
		if got != tc.expected {
			t.Errorf("requirementsTotalsLine(%d, %d) = %q, want %q", tc.n, tc.blocked, got, tc.expected)
		}
	}
}

// TestRunStatusRequirements_CurrentRepoErrorPropagates verifies the handler
// surfaces a failure resolving the current repo rather than silently
// continuing with an empty owner/name.
func TestRunStatusRequirements_CurrentRepoErrorPropagates(t *testing.T) {
	sd := statusDeps{
		currentRepo:      func() (string, error) { return "", fmt.Errorf("no remote") },
		resolveProjectID: func(string) (string, error) { return "PROJ_ID", nil },
		psDeps:           fakeProjectstatusDeps(nil),
		busy:             testutil.NoopBusy,
	}
	err := runStatusRequirements(&bytes.Buffer{}, io.Discard, statusListFlags{}, sd)
	if err == nil || !strings.Contains(err.Error(), "resolving current repository") {
		t.Errorf("expected 'resolving current repository' error; got %v", err)
	}
}

// TestRunStatusRequirements_RawTSVShape exercises the `--raw` renderer end
// to end: column count must match the header row on every line, sparse
// fields must render as `-`, and the rendered bytes must match the golden
// fixture for the fixed integration sample.
func TestRunStatusRequirements_RawTSVShape(t *testing.T) {
	buf := &bytes.Buffer{}
	if err := runStatusRequirements(buf, io.Discard, statusListFlags{raw: true}, fakeStatusDeps(sampleRequirementIssues())); err != nil {
		t.Fatalf("runStatusRequirements --raw: %v", err)
	}

	got := buf.Bytes()
	wantBytes, err := os.ReadFile("testdata/status_raw/requirements_list.raw")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !bytes.Equal(got, wantBytes) {
		t.Errorf("--raw output does not match golden\nwant:\n%s\ngot:\n%s", string(wantBytes), string(got))
	}

	// AC-3: every line, when split on \t, must have the same column count
	// as the header row.
	rawLines := strings.Split(strings.TrimRight(string(got), "\n"), "\n")
	if len(rawLines) == 0 {
		t.Fatal("expected at least the header row in --raw output")
	}
	headerCols := len(strings.Split(rawLines[0], "\t"))
	if headerCols != 5 {
		t.Errorf("header column count = %d, want 5", headerCols)
	}
	for i, line := range rawLines {
		cols := strings.Split(line, "\t")
		if len(cols) != headerCols {
			t.Errorf("line %d column count = %d, want %d (line: %q)", i, len(cols), headerCols, line)
		}
	}

	// Sparse blocked_by must render as the absent sentinel `-`, not empty.
	for i, line := range rawLines[1:] {
		cols := strings.Split(line, "\t")
		if cols[3] == "" {
			t.Errorf("data row %d blocked_by cell empty; expected sentinel %q", i, rawAbsentValue)
		}
	}
}

// TestRunStatusRequirements_RawVerboseAppendsTimestamps verifies that
// `--raw --verbose` appends `created_at` and `last_transitioned_at`
// columns to the header and to every data row, while preserving the TSV
// column-count invariant.
func TestRunStatusRequirements_RawVerboseAppendsTimestamps(t *testing.T) {
	buf := &bytes.Buffer{}
	if err := runStatusRequirements(buf, io.Discard, statusListFlags{raw: true, verbose: true}, fakeStatusDeps(sampleRequirementIssues())); err != nil {
		t.Fatalf("runStatusRequirements --raw --verbose: %v", err)
	}

	got := buf.Bytes()
	wantBytes, err := os.ReadFile("testdata/status_raw/requirements_list_verbose.raw")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !bytes.Equal(got, wantBytes) {
		t.Errorf("--raw --verbose output does not match golden\nwant:\n%s\ngot:\n%s", string(wantBytes), string(got))
	}

	// Column-count invariant: 7 columns (5 base + 2 timestamps).
	rawLines := strings.Split(strings.TrimRight(string(got), "\n"), "\n")
	headerCols := len(strings.Split(rawLines[0], "\t"))
	if headerCols != 7 {
		t.Errorf("verbose header column count = %d, want 7", headerCols)
	}
	for i, line := range rawLines {
		cols := strings.Split(line, "\t")
		if len(cols) != headerCols {
			t.Errorf("verbose line %d column count = %d, want %d", i, len(cols), headerCols)
		}
	}
}

// TestRunStatusRequirements_VerboseWithoutRawIsNoOp verifies that
// `--verbose` without `--raw` does not alter the human table — the bytes
// are identical to a default invocation.
func TestRunStatusRequirements_VerboseWithoutRawIsNoOp(t *testing.T) {
	bare := &bytes.Buffer{}
	if err := runStatusRequirements(bare, io.Discard, statusListFlags{}, fakeStatusDeps(sampleRequirementIssues())); err != nil {
		t.Fatalf("baseline: %v", err)
	}
	verbose := &bytes.Buffer{}
	if err := runStatusRequirements(verbose, io.Discard, statusListFlags{verbose: true}, fakeStatusDeps(sampleRequirementIssues())); err != nil {
		t.Fatalf("verbose: %v", err)
	}
	if !bytes.Equal(bare.Bytes(), verbose.Bytes()) {
		t.Errorf("--verbose without --raw must not change human output:\nbare:\n%s\nverbose:\n%s", bare.String(), verbose.String())
	}
}

// keysOf returns the keys of a map — used for diagnostics in test failures.
func keysOf(m map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
