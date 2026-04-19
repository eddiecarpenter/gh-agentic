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

// TestVerticalPipeline_EmptyColumnShowsNone verifies empty columns render
// the "(none)" marker rather than a blank section.
func TestVerticalPipeline_EmptyColumnShowsNone(t *testing.T) {
	cols := []projectstatus.Stage{projectstatus.StageBacklog}
	cards := map[projectstatus.Stage][]pipelineCard{projectstatus.StageBacklog: nil}
	buf := &bytes.Buffer{}
	if err := writeVerticalPipeline(buf, "Test", cols, cards); err != nil {
		t.Fatalf("writeVerticalPipeline: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "(none)") {
		t.Errorf("expected '(none)' for empty column; got:\n%s", out)
	}
	if !strings.Contains(out, "## backlog (0)") {
		t.Errorf("expected '## backlog (0)'; got:\n%s", out)
	}
}

// TestVerticalPipeline_BlockedCardWrapsAnnotation verifies a blocked card
// renders an indented [blocked by ...] line beneath its summary.
func TestVerticalPipeline_BlockedCardWrapsAnnotation(t *testing.T) {
	cols := []projectstatus.Stage{projectstatus.StageBacklog}
	cards := map[projectstatus.Stage][]pipelineCard{
		projectstatus.StageBacklog: {
			{Lines: []string{"#10 feat: blocked", "[blocked by foo/bar#99]"}},
		},
	}
	buf := &bytes.Buffer{}
	if err := writeVerticalPipeline(buf, "Test", cols, cards); err != nil {
		t.Fatalf("writeVerticalPipeline: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "[blocked by foo/bar#99]") {
		t.Errorf("expected blocked annotation; got:\n%s", out)
	}
}

// TestHorizontalPipeline_NarrowTerminalStillRenders verifies that
// writeHorizontalPipeline renders (without error) even when the detected
// terminal width is below the readability threshold — the layout is computed
// against minWidth and callers accept the overflow as the price of forcing
// horizontal on a narrow terminal.
func TestHorizontalPipeline_NarrowTerminalStillRenders(t *testing.T) {
	cols := columnsForFeatures(false)
	cards := featureCards([]projectstatus.Feature{
		{Number: 1, Title: "t", Stage: projectstatus.StageBacklog},
	}, cols, true)
	buf := &bytes.Buffer{}
	err := writeHorizontalPipeline(buf, cols, cards, 80, 120, true)
	if err != nil {
		t.Fatalf("writeHorizontalPipeline on narrow terminal returned error: %v", err)
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

// TestHorizontalPipeline_WideTerminalRenders verifies a wide terminal
// produces a box-drawing table with column headers and at least one row.
func TestHorizontalPipeline_WideTerminalRenders(t *testing.T) {
	cols := columnsForFeatures(false)
	cards := featureCards([]projectstatus.Feature{
		{Number: 492, Title: "feat: status command", Stage: projectstatus.StageInDevelopment},
	}, cols, true)

	buf := &bytes.Buffer{}
	if err := writeHorizontalPipeline(buf, cols, cards, 160, pipelineMinHorizontalWidthFeatures, true); err != nil {
		t.Fatalf("writeHorizontalPipeline: %v", err)
	}
	out := buf.String()
	// Expect unicode box-drawing chars.
	for _, tok := range []string{"┌", "┐", "└", "┘", "│", "backlog", "in-development", "#492"} {
		if !strings.Contains(out, tok) {
			t.Errorf("expected %q in horizontal output; got:\n%s", tok, out)
		}
	}
}

// TestHorizontalPipeline_ASCIIFallback verifies the ASCII box-drawing
// alternative renders when unicode is false.
func TestHorizontalPipeline_ASCIIFallback(t *testing.T) {
	cols := columnsForRequirements(false)
	cards := map[projectstatus.Stage][]pipelineCard{
		projectstatus.StageBacklog: {{Lines: []string{"#1 t"}}},
	}
	buf := &bytes.Buffer{}
	if err := writeHorizontalPipeline(buf, cols, cards, 160, pipelineMinHorizontalWidthRequirements, false); err != nil {
		t.Fatalf("writeHorizontalPipeline: %v", err)
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

// TestHorizontalPipeline_TopBorderHasLeadingDashBeforeLabel verifies the
// fix for issue #516 defect 1 — the top border emits one horiz glyph
// before each column label, producing `┌─ backlog ───┬─ scoping ───┐`
// rather than `┌ backlog ───┬ scoping ───┐`.
func TestHorizontalPipeline_TopBorderHasLeadingDashBeforeLabel(t *testing.T) {
	cols := columnsForRequirements(false)
	cards := map[projectstatus.Stage][]pipelineCard{
		projectstatus.StageBacklog: {{Lines: []string{"#1 t"}}},
	}
	buf := &bytes.Buffer{}
	if err := writeHorizontalPipeline(buf, cols, cards, 160, pipelineMinHorizontalWidthRequirements, true); err != nil {
		t.Fatalf("writeHorizontalPipeline: %v", err)
	}
	out := buf.String()
	topLine := strings.SplitN(out, "\n", 2)[0]
	// Each column header must be preceded by a horiz glyph: `┌─ backlog`
	// for the first column, `┬─ scoping` and `┬─ scheduled` for the joins.
	for _, want := range []string{"┌─ backlog", "┬─ scoping", "┬─ scheduled"} {
		if !strings.Contains(topLine, want) {
			t.Errorf("expected top border to contain %q; got:\n%s", want, topLine)
		}
	}
	// And it must NOT contain the broken form (label directly after the
	// corner / join with no leading dash).
	for _, bad := range []string{"┌ backlog", "┬ scoping", "┬ scheduled"} {
		if strings.Contains(topLine, bad) {
			t.Errorf("unexpected unspaced form %q in top border:\n%s", bad, topLine)
		}
	}
}

// TestHorizontalPipeline_TopBorderHasLeadingDashASCII verifies the leading
// dash also appears in the ASCII fallback rendering.
func TestHorizontalPipeline_TopBorderHasLeadingDashASCII(t *testing.T) {
	cols := columnsForRequirements(false)
	cards := map[projectstatus.Stage][]pipelineCard{
		projectstatus.StageBacklog: {{Lines: []string{"#1 t"}}},
	}
	buf := &bytes.Buffer{}
	if err := writeHorizontalPipeline(buf, cols, cards, 160, pipelineMinHorizontalWidthRequirements, false); err != nil {
		t.Fatalf("writeHorizontalPipeline: %v", err)
	}
	topLine := strings.SplitN(buf.String(), "\n", 2)[0]
	for _, want := range []string{"+- backlog", "+- scoping", "+- scheduled"} {
		if !strings.Contains(topLine, want) {
			t.Errorf("expected ASCII top border to contain %q; got:\n%s", want, topLine)
		}
	}
}

// TestHorizontalPipeline_CellWidthCappedOnWideTerminal verifies the fix
// for issue #516 defect 2 — column cell width is capped (currently at 50
// chars) so the pipeline does not stretch to fill very wide terminals.
// The bottom border is the cleanest place to measure cell width because
// it is pure box glyphs with no inline labels.
func TestHorizontalPipeline_CellWidthCappedOnWideTerminal(t *testing.T) {
	cols := columnsForRequirements(false) // 3 columns
	cards := map[projectstatus.Stage][]pipelineCard{
		projectstatus.StageBacklog: {{Lines: []string{"#1 t"}}},
	}
	buf := &bytes.Buffer{}
	// 252-col terminal — without the cap, cellWidth = (252-4)/3 = 82.
	if err := writeHorizontalPipeline(buf, cols, cards, 252, pipelineMinHorizontalWidthRequirements, true); err != nil {
		t.Fatalf("writeHorizontalPipeline: %v", err)
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	// Last line is the bottom border: `└<dashes>┴<dashes>┴<dashes>┘`.
	bottom := lines[len(lines)-1]
	// Each segment between corner/join glyphs is the run of horiz glyphs
	// for one column. With the 50-char cap, every segment must contain
	// exactly 50 horiz glyphs.
	dashRuns := strings.FieldsFunc(bottom, func(r rune) bool {
		return r == '└' || r == '┘' || r == '┴'
	})
	if len(dashRuns) != 3 {
		t.Fatalf("expected 3 column segments in bottom border; got %d in %q", len(dashRuns), bottom)
	}
	for i, run := range dashRuns {
		count := strings.Count(run, "─")
		if count != 50 {
			t.Errorf("column %d width = %d, expected cap of 50; bottom=%q", i, count, bottom)
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
	cards := featureCards(features, cols, true)
	if len(cards[projectstatus.StageBacklog]) != 1 {
		t.Errorf("backlog should have 1 card; got %d", len(cards[projectstatus.StageBacklog]))
	}
	if len(cards[projectstatus.StageInDesign]) != 2 {
		t.Errorf("in-design should have 2 cards; got %d", len(cards[projectstatus.StageInDesign]))
	}
}

// TestResolvePipelineLayout_DefaultWideIsHorizontal verifies that with no
// layout flag and a wide-enough terminal the resolver picks horizontal
// without a notice — the new default behaviour.
func TestResolvePipelineLayout_DefaultWideIsHorizontal(t *testing.T) {
	layout, err := resolvePipelineLayout(statusListFlags{kanban: true}, 150, featurePipelineMinWidth)
	if err != nil {
		t.Fatalf("resolvePipelineLayout: %v", err)
	}
	if !layout.horizontal {
		t.Errorf("expected horizontal layout on wide terminal; got vertical")
	}
	if layout.notice != "" {
		t.Errorf("expected no notice on wide default; got %q", layout.notice)
	}
}

// TestResolvePipelineLayout_DefaultNarrowFallsBack verifies the
// auto-fallback: narrow terminal + no flag → vertical + notice line.
func TestResolvePipelineLayout_DefaultNarrowFallsBack(t *testing.T) {
	layout, err := resolvePipelineLayout(statusListFlags{kanban: true}, 80, featurePipelineMinWidth)
	if err != nil {
		t.Fatalf("resolvePipelineLayout: %v", err)
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

// TestResolvePipelineLayout_VerticalForcesVertical verifies --vertical picks
// vertical on any width without a notice line.
func TestResolvePipelineLayout_VerticalForcesVertical(t *testing.T) {
	for _, width := range []int{60, 120, 200} {
		layout, err := resolvePipelineLayout(statusListFlags{kanban: true, vertical: true}, width, featurePipelineMinWidth)
		if err != nil {
			t.Fatalf("width=%d: resolvePipelineLayout: %v", width, err)
		}
		if layout.horizontal {
			t.Errorf("width=%d: expected vertical; got horizontal", width)
		}
		if layout.notice != "" {
			t.Errorf("width=%d: expected no notice with --vertical; got %q", width, layout.notice)
		}
	}
}

// TestResolvePipelineLayout_HorizontalForcesHorizontal verifies --horizontal
// picks horizontal on any width without a notice line.
func TestResolvePipelineLayout_HorizontalForcesHorizontal(t *testing.T) {
	for _, width := range []int{40, 120, 200} {
		layout, err := resolvePipelineLayout(statusListFlags{kanban: true, horizontal: true}, width, featurePipelineMinWidth)
		if err != nil {
			t.Fatalf("width=%d: resolvePipelineLayout: %v", width, err)
		}
		if !layout.horizontal {
			t.Errorf("width=%d: expected horizontal; got vertical", width)
		}
		if layout.notice != "" {
			t.Errorf("width=%d: expected no notice with --horizontal; got %q", width, layout.notice)
		}
	}
}

// TestResolvePipelineLayout_BothFlagsErrors verifies that passing both
// --horizontal and --vertical is a clean user-facing error.
func TestResolvePipelineLayout_BothFlagsErrors(t *testing.T) {
	_, err := resolvePipelineLayout(statusListFlags{kanban: true, horizontal: true, vertical: true}, 150, featurePipelineMinWidth)
	if err == nil {
		t.Fatalf("expected error for mutually-exclusive flags, got nil")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected message to mention 'mutually exclusive'; got %q", err.Error())
	}
}

// TestFeatureCards_ProgressLineZeroTasks verifies a feature with zero
// sub-issues renders the documented zero-total form on its progress line
// — `[] 0/0 tasks` — so the cards maintain a consistent structure.
func TestFeatureCards_ProgressLineZeroTasks(t *testing.T) {
	features := []projectstatus.Feature{
		{Number: 7, Title: "zero-task", Stage: projectstatus.StageBacklog, TasksTotal: 0, TasksDone: 0},
	}
	cols := []projectstatus.Stage{projectstatus.StageBacklog}
	cards := featureCards(features, cols, true)
	if len(cards[projectstatus.StageBacklog]) != 1 {
		t.Fatalf("expected 1 card; got %d", len(cards[projectstatus.StageBacklog]))
	}
	card := cards[projectstatus.StageBacklog][0]
	if len(card.Lines) < 2 {
		t.Fatalf("expected progress line on card; got lines %v", card.Lines)
	}
	got := card.Lines[1]
	if got != "[] 0/0 tasks" {
		t.Errorf("zero-task progress line = %q, want %q", got, "[] 0/0 tasks")
	}
}

// TestFeatureCards_ProgressLinePartial verifies a partially-complete
// feature renders the expected filled/empty block mix plus numeric.
func TestFeatureCards_ProgressLinePartial(t *testing.T) {
	features := []projectstatus.Feature{
		{Number: 1, Title: "partial", Stage: projectstatus.StageInDevelopment, TasksTotal: 6, TasksDone: 3},
	}
	cols := []projectstatus.Stage{projectstatus.StageInDevelopment}
	cards := featureCards(features, cols, true)
	card := cards[projectstatus.StageInDevelopment][0]
	if len(card.Lines) < 2 {
		t.Fatalf("expected progress line; got %v", card.Lines)
	}
	got := card.Lines[1]
	if got != "[■■■□□□] 3/6 tasks" {
		t.Errorf("progress line = %q, want %q", got, "[■■■□□□] 3/6 tasks")
	}
}

// TestFeatureCards_ProgressLineComplete verifies a fully-complete feature
// renders all filled blocks.
func TestFeatureCards_ProgressLineComplete(t *testing.T) {
	features := []projectstatus.Feature{
		{Number: 1, Title: "done", Stage: projectstatus.StageInReview, TasksTotal: 6, TasksDone: 6},
	}
	cards := featureCards(features, []projectstatus.Stage{projectstatus.StageInReview}, true)
	got := cards[projectstatus.StageInReview][0].Lines[1]
	if got != "[■■■■■■] 6/6 tasks" {
		t.Errorf("progress line = %q, want %q", got, "[■■■■■■] 6/6 tasks")
	}
}

// TestFeatureCards_ProgressLineBeyondCap verifies the > 20-task case: 20
// blocks rendered, numeric carries the exact N/M.
func TestFeatureCards_ProgressLineBeyondCap(t *testing.T) {
	features := []projectstatus.Feature{
		{Number: 1, Title: "big", Stage: projectstatus.StageInDevelopment, TasksTotal: 40, TasksDone: 23},
	}
	cards := featureCards(features, []projectstatus.Stage{projectstatus.StageInDevelopment}, true)
	got := cards[projectstatus.StageInDevelopment][0].Lines[1]
	// 23/40 = 57.5% → round(0.575 * 20) = 12 filled blocks
	if !strings.HasSuffix(got, " 23/40 tasks") {
		t.Errorf("expected numeric '23/40 tasks'; got %q", got)
	}
	totalBlocks := strings.Count(got, "■") + strings.Count(got, "□")
	if totalBlocks != 20 {
		t.Errorf("expected 20 blocks (cap); got %d in %q", totalBlocks, got)
	}
}

// TestFeatureCards_ProgressLineASCIIFallback verifies the non-Unicode
// rendering mode produces the documented ASCII characters.
func TestFeatureCards_ProgressLineASCIIFallback(t *testing.T) {
	features := []projectstatus.Feature{
		{Number: 1, Title: "ascii", Stage: projectstatus.StageInDevelopment, TasksTotal: 4, TasksDone: 2},
	}
	cards := featureCards(features, []projectstatus.Stage{projectstatus.StageInDevelopment}, false)
	got := cards[projectstatus.StageInDevelopment][0].Lines[1]
	if got != "[##  ] 2/4 tasks" {
		t.Errorf("ASCII progress line = %q, want %q", got, "[##  ] 2/4 tasks")
	}
	if strings.ContainsAny(got, "■□") {
		t.Errorf("unicode glyph leaked into ASCII output: %q", got)
	}
}

// TestRequirementCards_NoProgressLine verifies AC-9: requirement cards
// carry no progress-bar line and no `tasks` suffix.
func TestRequirementCards_NoProgressLine(t *testing.T) {
	reqs := []projectstatus.Requirement{
		{Number: 1, Title: "r", Stage: projectstatus.StageBacklog},
		{Number: 2, Title: "r2", Stage: projectstatus.StageBacklog, Blocked: &projectstatus.BlockedInfo{BlockingRef: "x/y#3"}},
	}
	cards := requirementCards(reqs, []projectstatus.Stage{projectstatus.StageBacklog})
	for _, c := range cards[projectstatus.StageBacklog] {
		for _, line := range c.Lines {
			// A requirement card must never contain the progress-bar glyphs
			// or the "N/M tasks" suffix — even the bracketed zero form.
			if strings.Contains(line, "■") || strings.Contains(line, "□") {
				t.Errorf("requirement card should not contain block glyphs; got %q", line)
			}
			if strings.Contains(line, " tasks") {
				t.Errorf("requirement card should not contain 'tasks' suffix; got %q", line)
			}
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
