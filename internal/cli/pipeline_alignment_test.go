package cli

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/eddiecarpenter/gh-agentic/internal/projectstatus"
)

// TestHorizontalPipeline_MultiByteRuneRowsStayAligned is the regression
// guard for feature #562's rune-aware cell-alignment fix. A card title
// containing a multi-byte rune like `→` (3 bytes, 1 visual column) used
// to shift every `│` separator left of the card by two columns because
// the renderer computed widths with len() (byte count) rather than the
// rune count.
//
// The test mirrors issue #467's real-world title `promote local →
// framework` and invokes writeHorizontalPipeline on a 252-col terminal
// (the width that originally surfaced the bug). Two properties hold
// post-fix:
//
//  1. Every non-empty output line has the same rune count — the top
//     border, each content row, and the bottom border all measure the
//     same number of columns.
//
//  2. The vertical boundary glyphs line up down the table. Top border
//     uses `┌`, `┬`, `┐`; content rows use `│`; bottom border uses
//     `└`, `┴`, `┘`. They must all sit at the same rune positions.
//
// Either property must fail against the pre-fix code (which leaked byte
// counts into the width arithmetic) and must hold after the fix.
func TestHorizontalPipeline_MultiByteRuneRowsStayAligned(t *testing.T) {
	cols := columnsForRequirements(false) // 3 columns

	// Card whose title contains a multi-byte rune. `→` is 3 bytes in
	// UTF-8 but one visual column; with len()-based width arithmetic the
	// row lands 2 columns short.
	cards := map[projectstatus.Stage][]pipelineCard{
		projectstatus.StageBacklog: {
			{Lines: []string{"#467 promote local → framework"}},
		},
	}

	buf := &bytes.Buffer{}
	if err := writeHorizontalPipeline(buf, cols, cards, 252, requirementPipelineMinWidth, true); err != nil {
		t.Fatalf("writeHorizontalPipeline: %v", err)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least top border + one row + bottom border; got %d lines:\n%s", len(lines), buf.String())
	}

	// Property 1: every non-empty line has the same rune count.
	wantRunes := utf8.RuneCountInString(lines[0])
	for i, line := range lines {
		if line == "" {
			continue
		}
		got := utf8.RuneCountInString(line)
		if got != wantRunes {
			t.Errorf("line %d rune count = %d, want %d; line=%q", i, got, wantRunes, line)
		}
	}

	// Property 2: vertical boundary glyphs must sit at the same rune
	// positions on every line. Each row uses its own set of boundary
	// glyphs: the top border has `┌`, `┬`, `┐`; content rows have `│`;
	// the bottom border has `└`, `┴`, `┘`. The positions of these
	// markers collectively define the column boundaries — in a correctly
	// aligned table they match across every line.
	boundaryGlyphs := map[rune]bool{
		'┌': true, '┬': true, '┐': true, // top border
		'│': true,                       // content rows
		'└': true, '┴': true, '┘': true, // bottom border
	}
	want := runeBoundaryPositions(lines[0], boundaryGlyphs)
	if len(want) == 0 {
		t.Fatalf("top border has no boundary glyphs; cannot anchor alignment check. line=%q", lines[0])
	}
	for i, line := range lines {
		if line == "" {
			continue
		}
		got := runeBoundaryPositions(line, boundaryGlyphs)
		if !intSlicesEqual(got, want) {
			t.Errorf("line %d boundary positions = %v, want %v; line=%q", i, got, want, line)
		}
	}
}

// TestTruncateString_IsRuneSafe verifies the helper never produces
// invalid UTF-8 when asked to clip a string containing multi-byte runes.
// The byte-slice form used to cut mid-rune; the rune-slice form cannot.
func TestTruncateString_IsRuneSafe(t *testing.T) {
	// 10 runes, each 3 bytes in UTF-8 (BMP CJK range encoded as 3-byte
	// sequences) — a byte-based truncation would cut in the middle of a
	// rune for any n where n*3 is not a multiple of 3.
	input := "→→→→→→→→→→" // 10 × `→`

	cases := []struct {
		n    int
		want string
	}{
		{n: 0, want: input}, // non-positive is a no-op
		{n: 1, want: "→"},   // single rune fits
		{n: 5, want: "→→→→…"},
		{n: 10, want: input}, // equal rune count is a no-op
		{n: 15, want: input}, // more than available is a no-op
	}
	for _, tc := range cases {
		got := truncateString(input, tc.n)
		if got != tc.want {
			t.Errorf("truncateString(%q, %d) = %q, want %q", input, tc.n, got, tc.want)
		}
		if !utf8.ValidString(got) {
			t.Errorf("truncateString(%q, %d) produced invalid UTF-8: %q", input, tc.n, got)
		}
	}
}

// runeBoundaryPositions returns the rune indexes at which any of the
// given boundary glyphs appear in s. Positions are in rune space (not
// byte space) so the returned slice can be compared directly across
// strings that contain multi-byte runes.
func runeBoundaryPositions(s string, boundary map[rune]bool) []int {
	out := make([]int, 0, 4)
	for i, r := range []rune(s) {
		if boundary[r] {
			out = append(out, i)
		}
	}
	return out
}

// intSlicesEqual reports whether two int slices have the same length and
// element-wise-equal values.
func intSlicesEqual(a, b []int) bool {
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
