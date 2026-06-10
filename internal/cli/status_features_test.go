package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/eddiecarpenter/gh-agentic/internal/project"
	"github.com/eddiecarpenter/gh-agentic/internal/projectstatus"
	"github.com/eddiecarpenter/gh-agentic/internal/testutil"
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
		busy: testutil.NoopBusy,
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
	if err := runStatusFeatures(buf, io.Discard, statusListFlags{}, sd); err != nil {
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
	if err := runStatusFeatures(buf, io.Discard, statusListFlags{}, sd); err != nil {
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
	if err := runStatusFeatures(buf, io.Discard, statusListFlags{thisRepo: true}, sd); err != nil {
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
	if err := runStatusFeatures(buf, io.Discard, statusListFlags{includeDone: true}, sd); err != nil {
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
		busy:             testutil.NoopBusy,
	}
	err := runStatusFeatures(&bytes.Buffer{}, io.Discard, statusListFlags{}, sd)
	if !errors.Is(err, projectstatus.ErrProjectNotConfigured) {
		t.Errorf("expected ErrProjectNotConfigured; got %v", err)
	}
}

// TestWriteFeaturesTable_TasksColumnAllCases verifies the compact N/M
// column renders the documented format for zero-task, partial, and fully-
// done features — a single render so the column layout is asserted once.
func TestWriteFeaturesTable_TasksColumnAllCases(t *testing.T) {
	features := []projectstatus.Feature{
		{Number: 100, Title: "zero", Stage: projectstatus.StageBacklog, OwningRepo: "eddiecarpenter/gh-agentic", TasksTotal: 0, TasksDone: 0},
		{Number: 200, Title: "partial", Stage: projectstatus.StageInDevelopment, OwningRepo: "eddiecarpenter/gh-agentic", TasksTotal: 6, TasksDone: 3},
		{Number: 300, Title: "complete", Stage: projectstatus.StageInReview, OwningRepo: "eddiecarpenter/gh-agentic", TasksTotal: 6, TasksDone: 6},
	}
	buf := &bytes.Buffer{}
	if err := writeFeaturesTable(buf, features, "eddiecarpenter/gh-agentic"); err != nil {
		t.Fatalf("writeFeaturesTable: %v", err)
	}
	out := buf.String()
	for _, tok := range []string{"TASKS", "0/0", "3/6", "6/6"} {
		if !strings.Contains(out, tok) {
			t.Errorf("expected %q in table output; got:\n%s", tok, out)
		}
	}
}

// TestWriteFeaturesTable_TasksColumnWithCrossRepo verifies the TASKS
// column renders alongside the REPO column in the federated layout.
func TestWriteFeaturesTable_TasksColumnWithCrossRepo(t *testing.T) {
	features := []projectstatus.Feature{
		{Number: 492, Title: "local", Stage: projectstatus.StageInDevelopment, OwningRepo: "eddiecarpenter/gh-agentic", TasksTotal: 6, TasksDone: 5},
		{Number: 511, Title: "remote", Stage: projectstatus.StageBacklog, OwningRepo: "foo/domain-x", TasksTotal: 0, TasksDone: 0},
	}
	buf := &bytes.Buffer{}
	if err := writeFeaturesTable(buf, features, "eddiecarpenter/gh-agentic"); err != nil {
		t.Fatalf("writeFeaturesTable: %v", err)
	}
	out := buf.String()
	for _, tok := range []string{"TASKS", "REPO", "5/6", "0/0", "foo/domain-x"} {
		if !strings.Contains(out, tok) {
			t.Errorf("expected %q in federated table output; got:\n%s", tok, out)
		}
	}
}

// TestRunStatusFeatures_ListPopulatesTasksColumn verifies the handler-level
// plumbing: the FetchSubIssues dependency is consumed and the counts land
// in the TASKS column.
func TestRunStatusFeatures_ListPopulatesTasksColumn(t *testing.T) {
	sd := fakeFeaturesDeps(sampleFeatureIssues(), nil)
	sd.psDeps.FetchSubIssues = func(_, _ string, n int) ([]projectstatus.TaskRef, error) {
		if n == 492 {
			return []projectstatus.TaskRef{
				{Number: 1, Closed: true}, {Number: 2, Closed: true},
				{Number: 3, Closed: true}, {Number: 4, Closed: false},
			}, nil
		}
		return nil, nil
	}
	buf := &bytes.Buffer{}
	if err := runStatusFeatures(buf, io.Discard, statusListFlags{}, sd); err != nil {
		t.Fatalf("runStatusFeatures: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "TASKS") {
		t.Errorf("expected TASKS column header; got:\n%s", out)
	}
	if !strings.Contains(out, "3/4") {
		t.Errorf("expected '3/4' cell for feature #492; got:\n%s", out)
	}
}

// TestRunStatusFeatures_RawTSVShape exercises the `--raw` renderer end to
// end: the rendered bytes must match the golden, every line split on \t
// must have the same column count, and sparse fields must render as `-`.
func TestRunStatusFeatures_RawTSVShape(t *testing.T) {
	sd := fakeFeaturesDeps(sampleFeatureIssues(), nil)
	buf := &bytes.Buffer{}
	if err := runStatusFeatures(buf, io.Discard, statusListFlags{raw: true}, sd); err != nil {
		t.Fatalf("runStatusFeatures --raw: %v", err)
	}

	got := buf.Bytes()
	wantBytes, err := os.ReadFile("testdata/status_raw/features_list.raw")
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

// TestRunStatusFeatures_RawVerboseAppendsTimestamps verifies that
// `--raw --verbose` appends the two timestamp columns, the bytes match
// the verbose golden, and the column-count invariant still holds.
func TestRunStatusFeatures_RawVerboseAppendsTimestamps(t *testing.T) {
	sd := fakeFeaturesDeps(sampleFeatureIssues(), nil)
	buf := &bytes.Buffer{}
	if err := runStatusFeatures(buf, io.Discard, statusListFlags{raw: true, verbose: true}, sd); err != nil {
		t.Fatalf("runStatusFeatures --raw --verbose: %v", err)
	}

	got := buf.Bytes()
	wantBytes, err := os.ReadFile("testdata/status_raw/features_list_verbose.raw")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !bytes.Equal(got, wantBytes) {
		t.Errorf("--raw --verbose output does not match golden\nwant:\n%s\ngot:\n%s", string(wantBytes), string(got))
	}

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

// TestRunStatusFeatures_VerboseWithoutRawIsNoOp verifies that `--verbose`
// without `--raw` does not change the human table output.
func TestRunStatusFeatures_VerboseWithoutRawIsNoOp(t *testing.T) {
	bare := &bytes.Buffer{}
	if err := runStatusFeatures(bare, io.Discard, statusListFlags{}, fakeFeaturesDeps(sampleFeatureIssues(), nil)); err != nil {
		t.Fatalf("baseline: %v", err)
	}
	verbose := &bytes.Buffer{}
	if err := runStatusFeatures(verbose, io.Discard, statusListFlags{verbose: true}, fakeFeaturesDeps(sampleFeatureIssues(), nil)); err != nil {
		t.Fatalf("verbose: %v", err)
	}
	if !bytes.Equal(bare.Bytes(), verbose.Bytes()) {
		t.Errorf("--verbose without --raw must not change human output:\nbare:\n%s\nverbose:\n%s", bare.String(), verbose.String())
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

// TestWriteFeaturesTable_OwningRepoErrorShowsWarning verifies that when a
// feature carries a non-empty OwningRepoError (set by populateTaskCounts when
// FetchSubIssues fails), writeFeaturesTable emits a ⚠ warning line after the
// totals footer.
func TestWriteFeaturesTable_OwningRepoErrorShowsWarning(t *testing.T) {
	features := []projectstatus.Feature{
		{
			Number:          492,
			Title:           "feat: status",
			Stage:           projectstatus.StageInDevelopment,
			OwningRepo:      "eddiecarpenter/gh-agentic",
			OwningRepoError: "eddiecarpenter/gh-agentic: repository not found",
		},
	}
	buf := &bytes.Buffer{}
	if err := writeFeaturesTable(buf, features, "eddiecarpenter/gh-agentic"); err != nil {
		t.Fatalf("writeFeaturesTable: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "⚠ eddiecarpenter/gh-agentic: repository not found") {
		t.Errorf("expected warning line; got:\n%s", out)
	}
}

// TestRunStatusFeatures_OwningRepoErrorRendersWarning is an integration test:
// when FetchSubIssues returns an error for a feature, FetchFeatures populates
// OwningRepoError and the table renderer emits the ⚠ warning line.
func TestRunStatusFeatures_OwningRepoErrorRendersWarning(t *testing.T) {
	sd := fakeFeaturesDeps(sampleFeatureIssues(), nil)
	sd.psDeps.FetchSubIssues = func(_, repo string, _ int) ([]projectstatus.TaskRef, error) {
		return nil, errors.New("repository not found")
	}
	buf := &bytes.Buffer{}
	if err := runStatusFeatures(buf, io.Discard, statusListFlags{}, sd); err != nil {
		t.Fatalf("runStatusFeatures: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "⚠") {
		t.Errorf("expected at least one ⚠ warning line; got:\n%s", out)
	}
}
