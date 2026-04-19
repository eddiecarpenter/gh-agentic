package ui

import "strings"

// ProgressBarMaxBlocks is the maximum number of block glyphs rendered by
// RenderProgressBar. Features with more than this many tasks still render
// exactly this many blocks — each block represents more than one task —
// keeping the bar width bounded for readability.
const ProgressBarMaxBlocks = 20

// Block glyphs used by the Unicode and ASCII rendering modes. Keeping them
// as exported constants lets tests assert precise rune output without
// duplicating string literals.
const (
	ProgressBlockFilledUnicode = "■"
	ProgressBlockEmptyUnicode  = "□"
	ProgressBlockFilledASCII   = "#"
	ProgressBlockEmptyASCII    = " "
)

// RenderProgressBar returns a bracketed block-bar visualising (done / total)
// completion, ready to place inline next to a numeric "N/M tasks" caption.
// The caller chooses whether to render Unicode block glyphs or the ASCII
// fallback — terminal-capability detection (e.g. ui.TerminalSupportsUTF8)
// is intentionally the caller's concern so unit tests can pin the mode
// deterministically.
//
// Rendering rules:
//
//   - Unicode mode uses `■` for filled and `□` for empty blocks.
//   - ASCII mode uses `#` for filled and a single space for empty blocks.
//   - For total ≤ ProgressBarMaxBlocks (20), each block represents exactly
//     one task; filled == done.
//   - For total > ProgressBarMaxBlocks, the bar is capped at
//     ProgressBarMaxBlocks blocks and the filled count is proportional
//     (round(done * 20 / total)), preserving the visual ratio. The
//     numeric caption carried by the caller still shows the exact values.
//   - A total of zero renders as `[]` (empty brackets, zero blocks). The
//     caller decides whether to show a caption alongside.
//   - Out-of-range `done` values are clamped: negative → 0, greater than
//     total → total. No panics, no negative repeat counts.
func RenderProgressBar(done, total int, unicode bool) string {
	if total < 0 {
		total = 0
	}
	if done < 0 {
		done = 0
	}
	if done > total {
		done = total
	}

	filledChar := ProgressBlockFilledASCII
	emptyChar := ProgressBlockEmptyASCII
	if unicode {
		filledChar = ProgressBlockFilledUnicode
		emptyChar = ProgressBlockEmptyUnicode
	}

	var blocks, filled int
	switch {
	case total == 0:
		// No tasks — render empty brackets. The caller adds any caption.
		return "[]"
	case total <= ProgressBarMaxBlocks:
		blocks = total
		filled = done
	default:
		blocks = ProgressBarMaxBlocks
		// Integer-rounded proportion — equivalent to math.Round(done*20/total)
		// without pulling in the math package for one division.
		filled = (done*ProgressBarMaxBlocks + total/2) / total
		if filled > blocks {
			filled = blocks
		}
		if filled < 0 {
			filled = 0
		}
	}
	empty := blocks - filled

	var b strings.Builder
	b.Grow(1 + blocks*len(filledChar) + 1)
	b.WriteByte('[')
	b.WriteString(strings.Repeat(filledChar, filled))
	b.WriteString(strings.Repeat(emptyChar, empty))
	b.WriteByte(']')
	return b.String()
}
