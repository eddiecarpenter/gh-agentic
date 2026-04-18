package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/eddiecarpenter/gh-agentic/internal/projectstatus"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// requirementKanbanColumns is the canonical left-to-right column order for
// the requirements kanban view. When --include-done is set, "done" is
// appended as the rightmost column — doneAppended() does that.
var requirementKanbanColumns = []projectstatus.Stage{
	projectstatus.StageBacklog,
	projectstatus.StageScoping,
	projectstatus.StageScheduled,
}

// featureKanbanColumns is the canonical left-to-right column order for the
// features kanban view.
var featureKanbanColumns = []projectstatus.Stage{
	projectstatus.StageBacklog,
	projectstatus.StageInDesign,
	projectstatus.StageInDevelopment,
	projectstatus.StageInReview,
}

// requirementKanbanMinWidth is the minimum terminal width in columns required
// for the horizontal kanban of requirements. Below this, the default kanban
// auto-falls-back to vertical rendering with a one-line notice. An explicit
// --horizontal flag overrides the fallback and honours the user's choice.
const requirementKanbanMinWidth = 100

// featureKanbanMinWidth is the minimum terminal width required for the
// horizontal kanban of features — wider because it has one more column and
// longer stage labels.
const featureKanbanMinWidth = 120

// Legacy constant aliases retained for backward compatibility with any
// downstream callers; the canonical names are requirementKanbanMinWidth and
// featureKanbanMinWidth above.
const (
	kanbanMinHorizontalWidthRequirements = requirementKanbanMinWidth
	kanbanMinHorizontalWidthFeatures     = featureKanbanMinWidth
)

// kanbanLayout captures the decision made by resolveKanbanLayout — whether
// to render horizontal or vertical, and an optional one-line notice to
// emit before the kanban body when the default auto-falls-back.
type kanbanLayout struct {
	horizontal bool
	notice     string
}

// resolveKanbanLayout picks the kanban layout for the current invocation.
//
// Precedence:
//  1. --horizontal and --vertical are mutually exclusive (error).
//  2. --vertical → vertical, no notice.
//  3. --horizontal → horizontal, no notice (honoured even on narrow terminals).
//  4. Neither flag: horizontal when actualWidth ≥ minWidth, otherwise
//     vertical with a fallback notice describing the widths and suggested flags.
func resolveKanbanLayout(flags statusListFlags, actualWidth, minWidth int) (kanbanLayout, error) {
	if flags.horizontal && flags.vertical {
		return kanbanLayout{}, fmt.Errorf("--horizontal and --vertical are mutually exclusive")
	}
	if flags.vertical {
		return kanbanLayout{horizontal: false}, nil
	}
	if flags.horizontal {
		return kanbanLayout{horizontal: true}, nil
	}
	if actualWidth >= minWidth {
		return kanbanLayout{horizontal: true}, nil
	}
	notice := fmt.Sprintf("(terminal %d cols — horizontal kanban needs ≥ %d. Showing vertical. Use --horizontal to override, or --vertical to make this the permanent default.)", actualWidth, minWidth)
	return kanbanLayout{horizontal: false, notice: notice}, nil
}

// kanbanCard is the pre-rendered content of a single card in a kanban
// column. The first line is typically the `#<N> <title>` summary; additional
// lines (blocked annotation, wrapped title) can follow.
type kanbanCard struct {
	Lines []string
}

// columnsForRequirements returns the column order used by the requirements
// kanban, optionally appending "done".
func columnsForRequirements(includeDone bool) []projectstatus.Stage {
	if includeDone {
		return append([]projectstatus.Stage{}, append(requirementKanbanColumns, projectstatus.StageDone)...)
	}
	return append([]projectstatus.Stage{}, requirementKanbanColumns...)
}

// columnsForFeatures returns the column order used by the features kanban,
// optionally appending "done".
func columnsForFeatures(includeDone bool) []projectstatus.Stage {
	if includeDone {
		return append([]projectstatus.Stage{}, append(featureKanbanColumns, projectstatus.StageDone)...)
	}
	return append([]projectstatus.Stage{}, featureKanbanColumns...)
}

// requirementCards groups the slice by stage and renders each requirement
// as a single-line card (with a wrapped blocked annotation when present).
func requirementCards(reqs []projectstatus.Requirement, columns []projectstatus.Stage) map[projectstatus.Stage][]kanbanCard {
	out := map[projectstatus.Stage][]kanbanCard{}
	for _, col := range columns {
		out[col] = nil
	}
	for _, r := range reqs {
		card := kanbanCard{Lines: []string{fmt.Sprintf("#%d %s", r.Number, r.Title)}}
		if r.Blocked != nil && r.Blocked.BlockingRef != "" {
			card.Lines = append(card.Lines, fmt.Sprintf("[blocked by %s]", r.Blocked.BlockingRef))
		}
		out[r.Stage] = append(out[r.Stage], card)
	}
	return out
}

// featureCards groups features by stage and renders them as cards. Every
// feature card carries a progress line combining the block-bar glyph
// (via ui.RenderProgressBar) and the numeric `N/M tasks` caption so both
// list-context views — horizontal and vertical kanban — communicate
// per-feature progress at a glance.
//
// unicode selects between the Unicode block glyphs and the ASCII fallback;
// callers thread in ui.TerminalSupportsUTF8() — the same value used to
// choose box-drawing glyphs — so a terminal gets a consistent look across
// the whole view.
func featureCards(features []projectstatus.Feature, columns []projectstatus.Stage, unicode bool) map[projectstatus.Stage][]kanbanCard {
	out := map[projectstatus.Stage][]kanbanCard{}
	for _, col := range columns {
		out[col] = nil
	}
	for _, f := range features {
		card := kanbanCard{Lines: []string{fmt.Sprintf("#%d %s", f.Number, f.Title)}}
		card.Lines = append(card.Lines, featureProgressLine(f, unicode))
		if f.Blocked != nil && f.Blocked.BlockingRef != "" {
			card.Lines = append(card.Lines, fmt.Sprintf("[blocked by %s]", f.Blocked.BlockingRef))
		}
		out[f.Stage] = append(out[f.Stage], card)
	}
	return out
}

// featureProgressLine renders the progress indicator shown on every
// feature kanban card — a block-bar followed by the exact N/M numeric.
// Zero-total features still emit a `0/0 tasks` caption so the line
// position is consistent across cards; the block-bar renders as `[]` in
// that case (see ui.RenderProgressBar).
func featureProgressLine(f projectstatus.Feature, unicode bool) string {
	bar := ui.RenderProgressBar(f.TasksDone, f.TasksTotal, unicode)
	return fmt.Sprintf("%s %d/%d tasks", bar, f.TasksDone, f.TasksTotal)
}

// writeVerticalKanban renders the stage-grouped view that works at any
// terminal width: a heading, then each column as `## <stage> (N)` followed
// by its cards, or `(none)` when the column is empty.
//
// heading is the title line (e.g. "Requirements — Kanban"). notice, if
// non-empty, is printed on its own line between the heading and the first
// column — used by the auto-fallback path to explain the layout choice.
func writeVerticalKanban(w io.Writer, heading string, columns []projectstatus.Stage, cards map[projectstatus.Stage][]kanbanCard, notice ...string) error {
	fmt.Fprintln(w, "=== "+heading+" ===")
	fmt.Fprintln(w, "")
	for _, n := range notice {
		if n != "" {
			fmt.Fprintln(w, n)
			fmt.Fprintln(w, "")
		}
	}
	for _, col := range columns {
		colCards := cards[col]
		fmt.Fprintf(w, "## %s (%d)\n", stageDisplay(col), len(colCards))
		if len(colCards) == 0 {
			fmt.Fprintln(w, "  (none)")
			fmt.Fprintln(w, "")
			continue
		}
		for _, c := range colCards {
			for i, line := range c.Lines {
				if i == 0 {
					fmt.Fprintf(w, "  %s\n", line)
				} else {
					fmt.Fprintf(w, "    %s\n", line)
				}
			}
		}
		fmt.Fprintln(w, "")
	}
	return nil
}

// writeHorizontalKanban renders the side-by-side box-drawing view. minWidth
// is the minimum terminal width needed for a readable layout; actualWidth is
// the detected width. When actualWidth is below minWidth the function still
// renders — honouring an explicit --horizontal opt-in — by using minWidth for
// cell-size calculations so cards remain legible even if the table overflows
// the terminal.
//
// unicode toggles the fancy box-drawing characters versus the ASCII fallback
// (+ - |).
func writeHorizontalKanban(w io.Writer, columns []projectstatus.Stage, cards map[projectstatus.Stage][]kanbanCard, actualWidth, minWidth int, unicode bool) error {
	// Use minWidth for layout when the terminal is narrower than the
	// readability threshold — the caller has chosen to force horizontal and
	// any overflow is their deliberate trade-off.
	layoutWidth := actualWidth
	if layoutWidth < minWidth {
		layoutWidth = minWidth
	}

	// Distribute available width across columns evenly, leaving room for the
	// vertical separators between them (one character per separator).
	colCount := len(columns)
	separatorCount := colCount + 1
	contentWidth := layoutWidth - separatorCount
	if contentWidth < colCount {
		contentWidth = colCount // minimum 1 cell per column
	}
	cellWidth := contentWidth / colCount

	var topLeft, topRight, bottomLeft, bottomRight, topJoin, bottomJoin, horiz, vert string
	if unicode {
		topLeft, topRight = "┌", "┐"
		bottomLeft, bottomRight = "└", "┘"
		topJoin, bottomJoin = "┬", "┴"
		horiz = "─"
		vert = "│"
	} else {
		topLeft, topRight = "+", "+"
		bottomLeft, bottomRight = "+", "+"
		topJoin, bottomJoin = "+", "+"
		horiz = "-"
		vert = "|"
	}

	// Top border with column headers inline: ┌─ name ─┬─ name ─┐
	var topLine strings.Builder
	topLine.WriteString(topLeft)
	for i, col := range columns {
		label := " " + stageDisplay(col) + " "
		if len(label) > cellWidth {
			label = truncateString(label, cellWidth)
		}
		dashes := cellWidth - len(label)
		if dashes < 0 {
			dashes = 0
		}
		topLine.WriteString(label)
		topLine.WriteString(strings.Repeat(horiz, dashes))
		if i < len(columns)-1 {
			topLine.WriteString(topJoin)
		}
	}
	topLine.WriteString(topRight)
	fmt.Fprintln(w, topLine.String())

	// Card rows — determine the maximum card height across all columns; pad
	// shorter columns with blank cells.
	maxHeight := 0
	for _, col := range columns {
		h := 0
		for _, c := range cards[col] {
			h += len(c.Lines)
		}
		if h > maxHeight {
			maxHeight = h
		}
	}
	if maxHeight == 0 {
		maxHeight = 1
	}

	// Build per-column line slices so we can render row-by-row.
	perColumn := make([][]string, colCount)
	for i, col := range columns {
		var lines []string
		for _, c := range cards[col] {
			lines = append(lines, c.Lines...)
		}
		for len(lines) < maxHeight {
			lines = append(lines, "")
		}
		perColumn[i] = lines
	}

	for row := 0; row < maxHeight; row++ {
		var rowLine strings.Builder
		rowLine.WriteString(vert)
		for i := 0; i < colCount; i++ {
			cell := perColumn[i][row]
			if len(cell) > cellWidth-1 {
				cell = truncateString(cell, cellWidth-1)
			}
			// truncateString can produce a byte-length slightly larger than
			// cellWidth-1 when it appends a multi-byte ellipsis glyph. Guard
			// against a negative pad count so the renderer tolerates narrow
			// cells without panicking.
			pad := cellWidth - 1 - len(cell)
			if pad < 0 {
				pad = 0
			}
			rowLine.WriteString(" " + cell + strings.Repeat(" ", pad))
			rowLine.WriteString(vert)
		}
		fmt.Fprintln(w, rowLine.String())
	}

	// Bottom border
	var bottomLine strings.Builder
	bottomLine.WriteString(bottomLeft)
	for i := 0; i < colCount; i++ {
		bottomLine.WriteString(strings.Repeat(horiz, cellWidth))
		if i < colCount-1 {
			bottomLine.WriteString(bottomJoin)
		}
	}
	bottomLine.WriteString(bottomRight)
	fmt.Fprintln(w, bottomLine.String())
	return nil
}

// truncateString clips s to at most n characters. Callers always pass a
// positive n; defensively, the function is a no-op for non-positive n.
func truncateString(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}
