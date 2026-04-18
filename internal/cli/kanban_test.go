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

// TestHorizontalKanban_NarrowTerminalStillRenders verifies that
// writeHorizontalKanban renders (without error) even when the detected
// terminal width is below the readability threshold — the layout is computed
// against minWidth and callers accept the overflow as the price of forcing
// horizontal on a narrow terminal.
func TestHorizontalKanban_NarrowTerminalStillRenders(t *testing.T) {
	cols := columnsForFeatures(false)
	cards := featureCards([]projectstatus.Feature{
		{Number: 1, Title: "t", Stage: projectstatus.StageBacklog},
	}, cols)
	buf := &bytes.Buffer{}
	err := writeHorizontalKanban(buf, cols, cards, 80, 120, true)
	if err != nil {
		t.Fatalf("writeHorizontalKanban on narrow terminal returned error: %v", err)
	}
	out := buf.String()
	// Layout should have been computed against minWidth (120), so the box
	// must include border characters and at least one column header.
	for _, tok := range []string{"┌", "┐", "backlog"} {
		if !strings.Contains(out, tok) {
			t.Errorf("expected %q in narrow-terminal output; got:\n%s", tok, out)
		}
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

// TestRunStatusFeatures_KanbanHorizontalOnNarrowHonoursChoice verifies that
// an explicit --horizontal on a narrow terminal still renders horizontal
// (the user's choice is honoured) and does not error, in contrast to the
// auto-fallback path which picks vertical silently with a notice.
func TestRunStatusFeatures_KanbanHorizontalOnNarrowHonoursChoice(t *testing.T) {
	originalWidth := terminalWidth
	terminalWidth = func() int { return 60 }
	defer func() { terminalWidth = originalWidth }()

	sd := fakeFeaturesDeps(sampleFeatureIssues(), nil)
	buf := &bytes.Buffer{}
	err := runStatusFeatures(buf, statusListFlags{kanban: true, horizontal: true}, sd)
	if err != nil {
		t.Fatalf("--horizontal on narrow terminal returned error: %v", err)
	}
	out := buf.String()
	if !strings.ContainsAny(out, "┌+") {
		t.Errorf("expected horizontal table borders even on narrow terminal; got:\n%s", out)
	}
	if strings.Contains(out, "horizontal kanban needs ≥") {
		t.Errorf("--horizontal must not emit the fallback notice; got:\n%s", out)
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

// TestResolveKanbanLayout_DefaultWideIsHorizontal verifies that with no layout
// flag and a wide-enough terminal the resolver picks horizontal without a
// notice — the new default behaviour.
func TestResolveKanbanLayout_DefaultWideIsHorizontal(t *testing.T) {
	layout, err := resolveKanbanLayout(statusListFlags{kanban: true}, 150, featureKanbanMinWidth)
	if err != nil {
		t.Fatalf("resolveKanbanLayout: %v", err)
	}
	if !layout.horizontal {
		t.Errorf("expected horizontal layout on wide terminal; got vertical")
	}
	if layout.notice != "" {
		t.Errorf("expected no notice on wide default; got %q", layout.notice)
	}
}

// TestResolveKanbanLayout_DefaultNarrowFallsBack verifies the auto-fallback:
// narrow terminal + no flag → vertical + notice line.
func TestResolveKanbanLayout_DefaultNarrowFallsBack(t *testing.T) {
	layout, err := resolveKanbanLayout(statusListFlags{kanban: true}, 80, featureKanbanMinWidth)
	if err != nil {
		t.Fatalf("resolveKanbanLayout: %v", err)
	}
	if layout.horizontal {
		t.Errorf("expected vertical fallback on narrow terminal; got horizontal")
	}
	if layout.notice == "" {
		t.Errorf("expected fallback notice on narrow default; got empty")
	}
	// The notice must name the current width, the required width, and point
	// at both opt-in flags.
	for _, tok := range []string{"80", "120", "--horizontal", "--vertical"} {
		if !strings.Contains(layout.notice, tok) {
			t.Errorf("expected notice to name %q; got %q", tok, layout.notice)
		}
	}
}

// TestResolveKanbanLayout_VerticalForcesVertical verifies --vertical picks
// vertical on any width without a notice line.
func TestResolveKanbanLayout_VerticalForcesVertical(t *testing.T) {
	for _, width := range []int{60, 120, 200} {
		layout, err := resolveKanbanLayout(statusListFlags{kanban: true, vertical: true}, width, featureKanbanMinWidth)
		if err != nil {
			t.Fatalf("width=%d: resolveKanbanLayout: %v", width, err)
		}
		if layout.horizontal {
			t.Errorf("width=%d: expected vertical; got horizontal", width)
		}
		if layout.notice != "" {
			t.Errorf("width=%d: expected no notice with --vertical; got %q", width, layout.notice)
		}
	}
}

// TestResolveKanbanLayout_HorizontalForcesHorizontal verifies --horizontal
// picks horizontal on any width without a notice line.
func TestResolveKanbanLayout_HorizontalForcesHorizontal(t *testing.T) {
	for _, width := range []int{40, 120, 200} {
		layout, err := resolveKanbanLayout(statusListFlags{kanban: true, horizontal: true}, width, featureKanbanMinWidth)
		if err != nil {
			t.Fatalf("width=%d: resolveKanbanLayout: %v", width, err)
		}
		if !layout.horizontal {
			t.Errorf("width=%d: expected horizontal; got vertical", width)
		}
		if layout.notice != "" {
			t.Errorf("width=%d: expected no notice with --horizontal; got %q", width, layout.notice)
		}
	}
}

// TestResolveKanbanLayout_BothFlagsErrors verifies that passing both
// --horizontal and --vertical is a clean user-facing error.
func TestResolveKanbanLayout_BothFlagsErrors(t *testing.T) {
	_, err := resolveKanbanLayout(statusListFlags{kanban: true, horizontal: true, vertical: true}, 150, featureKanbanMinWidth)
	if err == nil {
		t.Fatalf("expected error for mutually-exclusive flags, got nil")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected message to mention 'mutually exclusive'; got %q", err.Error())
	}
}

// TestRunStatusFeatures_KanbanDefaultWideIsHorizontal verifies the features
// kanban defaults to horizontal on a wide terminal and emits no notice.
func TestRunStatusFeatures_KanbanDefaultWideIsHorizontal(t *testing.T) {
	originalWidth := terminalWidth
	terminalWidth = func() int { return 160 }
	defer func() { terminalWidth = originalWidth }()

	sd := fakeFeaturesDeps(sampleFeatureIssues(), nil)
	buf := &bytes.Buffer{}
	if err := runStatusFeatures(buf, statusListFlags{kanban: true}, sd); err != nil {
		t.Fatalf("runStatusFeatures: %v", err)
	}
	out := buf.String()
	if !strings.ContainsAny(out, "┌+") {
		t.Errorf("expected horizontal borders; got:\n%s", out)
	}
	if strings.Contains(out, "horizontal kanban needs ≥") {
		t.Errorf("no fallback notice expected on wide terminal; got:\n%s", out)
	}
	// No '## stage' vertical section headings should appear.
	if strings.Contains(out, "## backlog") {
		t.Errorf("vertical section heading leaked into horizontal default; got:\n%s", out)
	}
}

// TestRunStatusFeatures_KanbanDefaultNarrowAutoFallsBack verifies a narrow
// terminal falls back to vertical, exits 0, and prints the notice line.
func TestRunStatusFeatures_KanbanDefaultNarrowAutoFallsBack(t *testing.T) {
	originalWidth := terminalWidth
	terminalWidth = func() int { return 80 }
	defer func() { terminalWidth = originalWidth }()

	sd := fakeFeaturesDeps(sampleFeatureIssues(), nil)
	buf := &bytes.Buffer{}
	if err := runStatusFeatures(buf, statusListFlags{kanban: true}, sd); err != nil {
		t.Fatalf("runStatusFeatures auto-fallback should not error: %v", err)
	}
	out := buf.String()
	for _, tok := range []string{
		"## backlog",
		"## in-development",
		"terminal 80 cols",
		"needs ≥ 120",
		"--horizontal",
		"--vertical",
	} {
		if !strings.Contains(out, tok) {
			t.Errorf("expected %q in auto-fallback output; got:\n%s", tok, out)
		}
	}
}

// TestRunStatusFeatures_KanbanVerticalForced verifies --vertical forces
// vertical on wide terminals without emitting the notice line.
func TestRunStatusFeatures_KanbanVerticalForced(t *testing.T) {
	originalWidth := terminalWidth
	terminalWidth = func() int { return 200 }
	defer func() { terminalWidth = originalWidth }()

	sd := fakeFeaturesDeps(sampleFeatureIssues(), nil)
	buf := &bytes.Buffer{}
	if err := runStatusFeatures(buf, statusListFlags{kanban: true, vertical: true}, sd); err != nil {
		t.Fatalf("runStatusFeatures --vertical: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "## backlog") {
		t.Errorf("expected vertical section headings; got:\n%s", out)
	}
	if strings.Contains(out, "horizontal kanban needs ≥") {
		t.Errorf("--vertical must not emit fallback notice; got:\n%s", out)
	}
}

// TestRunStatusFeatures_KanbanBothFlagsError verifies passing --horizontal
// and --vertical together yields a clean mutually-exclusive error.
func TestRunStatusFeatures_KanbanBothFlagsError(t *testing.T) {
	sd := fakeFeaturesDeps(sampleFeatureIssues(), nil)
	err := runStatusFeatures(&bytes.Buffer{}, statusListFlags{kanban: true, horizontal: true, vertical: true}, sd)
	if err == nil {
		t.Fatalf("expected mutually-exclusive error, got nil")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected message to mention 'mutually exclusive'; got %q", err.Error())
	}
}

// TestRunStatusRequirements_KanbanDefaultWideIsHorizontal verifies the
// requirements kanban defaults to horizontal on a wide terminal.
func TestRunStatusRequirements_KanbanDefaultWideIsHorizontal(t *testing.T) {
	originalWidth := terminalWidth
	terminalWidth = func() int { return 160 }
	defer func() { terminalWidth = originalWidth }()

	sd := fakeStatusDeps(sampleRequirementIssues())
	buf := &bytes.Buffer{}
	if err := runStatusRequirements(buf, statusListFlags{kanban: true}, sd); err != nil {
		t.Fatalf("runStatusRequirements: %v", err)
	}
	out := buf.String()
	if !strings.ContainsAny(out, "┌+") {
		t.Errorf("expected horizontal borders; got:\n%s", out)
	}
	if strings.Contains(out, "## backlog (") {
		t.Errorf("vertical section heading leaked into horizontal default; got:\n%s", out)
	}
}

// TestRunStatusRequirements_KanbanDefaultNarrowAutoFallsBack verifies the
// requirements kanban auto-falls-back to vertical on a narrow terminal.
func TestRunStatusRequirements_KanbanDefaultNarrowAutoFallsBack(t *testing.T) {
	originalWidth := terminalWidth
	terminalWidth = func() int { return 60 }
	defer func() { terminalWidth = originalWidth }()

	sd := fakeStatusDeps(sampleRequirementIssues())
	buf := &bytes.Buffer{}
	if err := runStatusRequirements(buf, statusListFlags{kanban: true}, sd); err != nil {
		t.Fatalf("runStatusRequirements auto-fallback should not error: %v", err)
	}
	out := buf.String()
	for _, tok := range []string{
		"## backlog",
		"terminal 60 cols",
		"needs ≥ 100",
		"--horizontal",
		"--vertical",
	} {
		if !strings.Contains(out, tok) {
			t.Errorf("expected %q in auto-fallback output; got:\n%s", tok, out)
		}
	}
}

// TestRunStatusRequirements_KanbanBothFlagsError verifies passing
// --horizontal and --vertical together yields a clean error.
func TestRunStatusRequirements_KanbanBothFlagsError(t *testing.T) {
	sd := fakeStatusDeps(sampleRequirementIssues())
	err := runStatusRequirements(&bytes.Buffer{}, statusListFlags{kanban: true, horizontal: true, vertical: true}, sd)
	if err == nil {
		t.Fatalf("expected mutually-exclusive error, got nil")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected message to mention 'mutually exclusive'; got %q", err.Error())
	}
}

// TestStatusCmd_VerticalFlagRegistered verifies the new --vertical flag is
// declared on both list sub-commands.
func TestStatusCmd_VerticalFlagRegistered(t *testing.T) {
	for _, parent := range []string{"requirements", "features"} {
		cmd := newStatusCmd()
		child := findChild(cmd, parent)
		if child == nil {
			t.Fatalf("status: sub-command %q not found", parent)
		}
		if child.Flags().Lookup("vertical") == nil {
			t.Errorf("status %s: expected --vertical flag; not registered", parent)
		}
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
