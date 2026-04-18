package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/eddiecarpenter/gh-agentic/internal/projectstatus"
)

// TestColumnsForRequirements_OrderAndDone verifies canonical ordering plus
// the --include-done suffix behaviour.
func TestColumnsForRequirements_OrderAndDone(t *testing.T) {
	got := columnsForRequirements(false)
	want := []projectstatus.Stage{projectstatus.StageBacklog, projectstatus.StageScoping, projectstatus.StageScheduled}
	if !equalStages(got, want) {
		t.Errorf("columnsForRequirements(false) = %v, want %v", got, want)
	}
	got = columnsForRequirements(true)
	want = append(want, projectstatus.StageDone)
	if !equalStages(got, want) {
		t.Errorf("columnsForRequirements(true) = %v, want %v", got, want)
	}
}

// TestColumnsForFeatures_OrderAndDone mirrors the requirements check.
func TestColumnsForFeatures_OrderAndDone(t *testing.T) {
	got := columnsForFeatures(false)
	want := []projectstatus.Stage{projectstatus.StageBacklog, projectstatus.StageInDesign, projectstatus.StageInDevelopment, projectstatus.StageInReview}
	if !equalStages(got, want) {
		t.Errorf("columnsForFeatures(false) = %v, want %v", got, want)
	}
	got = columnsForFeatures(true)
	want = append(want, projectstatus.StageDone)
	if !equalStages(got, want) {
		t.Errorf("columnsForFeatures(true) = %v, want %v", got, want)
	}
}

// TestVerticalKanban_EmptyColumnShowsNone verifies empty columns render the
// "(none)" marker rather than a blank section.
func TestVerticalKanban_EmptyColumnShowsNone(t *testing.T) {
	cols := []projectstatus.Stage{projectstatus.StageBacklog}
	cards := map[projectstatus.Stage][]kanbanCard{projectstatus.StageBacklog: nil}
	buf := &bytes.Buffer{}
	if err := writeVerticalKanban(buf, "Test", cols, cards); err != nil {
		t.Fatalf("writeVerticalKanban: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "(none)") {
		t.Errorf("expected '(none)' for empty column; got:\n%s", out)
	}
	if !strings.Contains(out, "## backlog (0)") {
		t.Errorf("expected '## backlog (0)'; got:\n%s", out)
	}
}

// TestVerticalKanban_BlockedCardWrapsAnnotation verifies a blocked card
// renders an indented [blocked by ...] line beneath its summary.
func TestVerticalKanban_BlockedCardWrapsAnnotation(t *testing.T) {
	cols := []projectstatus.Stage{projectstatus.StageBacklog}
	cards := map[projectstatus.Stage][]kanbanCard{
		projectstatus.StageBacklog: {
			{Lines: []string{"#10 feat: blocked", "[blocked by foo/bar#99]"}},
		},
	}
	buf := &bytes.Buffer{}
	if err := writeVerticalKanban(buf, "Test", cols, cards); err != nil {
		t.Fatalf("writeVerticalKanban: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "[blocked by foo/bar#99]") {
		t.Errorf("expected blocked annotation; got:\n%s", out)
	}
}

// TestHorizontalKanban_NarrowTerminalErrors verifies the clean error when
// the terminal is below the minimum width.
func TestHorizontalKanban_NarrowTerminalErrors(t *testing.T) {
	cols := columnsForFeatures(false)
	cards := map[projectstatus.Stage][]kanbanCard{}
	buf := &bytes.Buffer{}
	err := writeHorizontalKanban(buf, cols, cards, 80, 120, true)
	if err == nil {
		t.Fatalf("expected error for narrow terminal, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "at least 120") || !strings.Contains(msg, "80") {
		t.Errorf("expected message to name both widths; got %q", msg)
	}
}

// TestHorizontalKanban_WideTerminalRenders verifies a wide terminal produces
// a box-drawing table with column headers and at least one row.
func TestHorizontalKanban_WideTerminalRenders(t *testing.T) {
	cols := columnsForFeatures(false)
	cards := featureCards([]projectstatus.Feature{
		{Number: 492, Title: "feat: status command", Stage: projectstatus.StageInDevelopment},
	}, cols)

	buf := &bytes.Buffer{}
	if err := writeHorizontalKanban(buf, cols, cards, 160, kanbanMinHorizontalWidthFeatures, true); err != nil {
		t.Fatalf("writeHorizontalKanban: %v", err)
	}
	out := buf.String()
	// Expect unicode box-drawing chars.
	for _, tok := range []string{"┌", "┐", "└", "┘", "│", "backlog", "in-development", "#492"} {
		if !strings.Contains(out, tok) {
			t.Errorf("expected %q in horizontal output; got:\n%s", tok, out)
		}
	}
}

// TestHorizontalKanban_ASCIIFallback verifies the ASCII box-drawing
// alternative renders when unicode is false.
func TestHorizontalKanban_ASCIIFallback(t *testing.T) {
	cols := columnsForRequirements(false)
	cards := map[projectstatus.Stage][]kanbanCard{
		projectstatus.StageBacklog: {{Lines: []string{"#1 t"}}},
	}
	buf := &bytes.Buffer{}
	if err := writeHorizontalKanban(buf, cols, cards, 160, kanbanMinHorizontalWidthRequirements, false); err != nil {
		t.Fatalf("writeHorizontalKanban: %v", err)
	}
	out := buf.String()
	for _, tok := range []string{"+", "|", "-"} {
		if !strings.Contains(out, tok) {
			t.Errorf("expected ASCII box char %q; got:\n%s", tok, out)
		}
	}
	for _, tok := range []string{"┌", "┐", "│"} {
		if strings.Contains(out, tok) {
			t.Errorf("unicode leaked into ASCII output: %q in:\n%s", tok, out)
		}
	}
}

// TestRequirementCards_BlockedAnnotation verifies the blocked annotation is
// appended as a second line on the card.
func TestRequirementCards_BlockedAnnotation(t *testing.T) {
	reqs := []projectstatus.Requirement{
		{Number: 10, Title: "t", Stage: projectstatus.StageBacklog, Blocked: &projectstatus.BlockedInfo{BlockingRef: "a/b#9"}},
	}
	cards := requirementCards(reqs, []projectstatus.Stage{projectstatus.StageBacklog})
	b := cards[projectstatus.StageBacklog]
	if len(b) != 1 || len(b[0].Lines) != 2 || b[0].Lines[1] != "[blocked by a/b#9]" {
		t.Errorf("expected blocked annotation on card; got %+v", b)
	}
}

// TestFeatureCards_SortedByStage verifies all feature cards land in the
// correct stage bucket.
func TestFeatureCards_SortedByStage(t *testing.T) {
	features := []projectstatus.Feature{
		{Number: 1, Title: "a", Stage: projectstatus.StageInDesign},
		{Number: 2, Title: "b", Stage: projectstatus.StageBacklog},
		{Number: 3, Title: "c", Stage: projectstatus.StageInDesign},
	}
	cols := []projectstatus.Stage{projectstatus.StageBacklog, projectstatus.StageInDesign}
	cards := featureCards(features, cols)
	if len(cards[projectstatus.StageBacklog]) != 1 {
		t.Errorf("backlog should have 1 card; got %d", len(cards[projectstatus.StageBacklog]))
	}
	if len(cards[projectstatus.StageInDesign]) != 2 {
		t.Errorf("in-design should have 2 cards; got %d", len(cards[projectstatus.StageInDesign]))
	}
}

// TestRunStatusRequirements_JSONBeatsKanban verifies --json+--kanban yields
// the JSON envelope (no kanban decoration), matching the documented
// precedence.
func TestRunStatusRequirements_JSONBeatsKanban(t *testing.T) {
	sd := fakeStatusDeps(sampleRequirementIssues())
	buf := &bytes.Buffer{}
	err := runStatusRequirements(buf, statusListFlags{json: true, kanban: true}, sd)
	if err != nil {
		t.Fatalf("runStatusRequirements: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"items":`) {
		t.Errorf("expected JSON envelope; got:\n%s", out)
	}
	if strings.Contains(out, "Requirements — Kanban") {
		t.Errorf("kanban heading leaked into JSON output:\n%s", out)
	}
}

// TestRunStatusFeatures_KanbanHorizontalWithFakeWidth exercises the happy
// horizontal-kanban path via the command-level handler by substituting the
// terminalWidth probe.
func TestRunStatusFeatures_KanbanHorizontalWithFakeWidth(t *testing.T) {
	originalWidth := terminalWidth
	terminalWidth = func() int { return 160 }
	defer func() { terminalWidth = originalWidth }()

	sd := fakeFeaturesDeps(sampleFeatureIssues(), nil)
	buf := &bytes.Buffer{}
	err := runStatusFeatures(buf, statusListFlags{kanban: true, horizontal: true}, sd)
	if err != nil {
		t.Fatalf("runStatusFeatures kanban+horizontal: %v", err)
	}
	out := buf.String()
	// Table borders (unicode or ASCII depending on locale) must be present.
	if !strings.ContainsAny(out, "┌+") {
		t.Errorf("expected table border in output; got:\n%s", out)
	}
}

// TestRunStatusFeatures_KanbanHorizontalNarrowErrors exercises the narrow
// path via the command handler.
func TestRunStatusFeatures_KanbanHorizontalNarrowErrors(t *testing.T) {
	originalWidth := terminalWidth
	terminalWidth = func() int { return 60 }
	defer func() { terminalWidth = originalWidth }()

	sd := fakeFeaturesDeps(sampleFeatureIssues(), nil)
	err := runStatusFeatures(&bytes.Buffer{}, statusListFlags{kanban: true, horizontal: true}, sd)
	if err == nil {
		t.Fatalf("expected error from narrow terminal, got nil")
	}
	if !strings.Contains(err.Error(), "Current terminal: 60") {
		t.Errorf("error should name the current width; got %v", err)
	}
}

// TestRunStatusRequirements_KanbanIncludeDoneAddsColumn verifies the "done"
// column appears at the rightmost position only when --include-done is set.
func TestRunStatusRequirements_KanbanIncludeDoneAddsColumn(t *testing.T) {
	sd := fakeStatusDeps(sampleRequirementIssues())
	buf := &bytes.Buffer{}
	if err := runStatusRequirements(buf, statusListFlags{kanban: true, includeDone: true}, sd); err != nil {
		t.Fatalf("runStatusRequirements: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "## done") {
		t.Errorf("expected '## done' column with --include-done; got:\n%s", out)
	}

	buf = &bytes.Buffer{}
	if err := runStatusRequirements(buf, statusListFlags{kanban: true, includeDone: false}, sd); err != nil {
		t.Fatalf("runStatusRequirements: %v", err)
	}
	if strings.Contains(buf.String(), "## done") {
		t.Errorf("did not expect 'done' column without --include-done; got:\n%s", buf.String())
	}
}

// equalStages is a small helper for stage-slice comparison.
func equalStages(a, b []projectstatus.Stage) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
