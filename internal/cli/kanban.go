package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/eddiecarpenter/gh-agentic/internal/projectstatus"
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

// kanbanMinHorizontalWidthRequirements is the minimum terminal width in
// columns required for the horizontal kanban of requirements. Below this the
// command errors cleanly rather than rendering an unreadable table.
const kanbanMinHorizontalWidthRequirements = 90

// kanbanMinHorizontalWidthFeatures is the minimum terminal width required
// for the horizontal kanban of features — wider because it has one more
// column and longer stage labels.
const kanbanMinHorizontalWidthFeatures = 120

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

// featureCards groups features by stage and renders them as cards.
func featureCards(features []projectstatus.Feature, columns []projectstatus.Stage) map[projectstatus.Stage][]kanbanCard {
	out := map[projectstatus.Stage][]kanbanCard{}
	for _, col := range columns {
		out[col] = nil
	}
	for _, f := range features {
		card := kanbanCard{Lines: []string{fmt.Sprintf("#%d %s", f.Number, f.Title)}}
		if f.Blocked != nil && f.Blocked.BlockingRef != "" {
			card.Lines = append(card.Lines, fmt.Sprintf("[blocked by %s]", f.Blocked.BlockingRef))
		}
		out[f.Stage] = append(out[f.Stage], card)
	}
	return out
}

// writeVerticalKanban renders the stage-grouped view that works at any
// terminal width: a heading, then each column as `## <stage> (N)` followed
// by its cards, or `(none)` when the column is empty.
//
// heading is the title line (e.g. "Requirements — Kanban").
func writeVerticalKanban(w io.Writer, heading string, columns []projectstatus.Stage, cards map[projectstatus.Stage][]kanbanCard) error {
	fmt.Fprintln(w, "=== "+heading+" ===")
	fmt.Fprintln(w, "")
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
// is the required terminal width; actualWidth is the detected width. The
// function returns a clean error with a fix-it message when actualWidth is
// below minWidth.
//
// unicode toggles the fancy box-drawing characters versus the ASCII fallback
// (+ - |).
func writeHorizontalKanban(w io.Writer, columns []projectstatus.Stage, cards map[projectstatus.Stage][]kanbanCard, actualWidth, minWidth int, unicode bool) error {
	if actualWidth < minWidth {
		return fmt.Errorf("--horizontal requires at least %d columns. Current terminal: %d. Try without --horizontal for vertical kanban.", minWidth, actualWidth)
	}

	// Distribute available width across columns evenly, leaving room for the
	// vertical separators between them (one character per separator).
	colCount := len(columns)
	separatorCount := colCount + 1
	contentWidth := actualWidth - separatorCount
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
			rowLine.WriteString(" " + cell + strings.Repeat(" ", cellWidth-1-len(cell)))
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
