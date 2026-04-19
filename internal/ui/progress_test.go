package ui

import (
	"strings"
	"testing"
)

// TestRenderProgressBar_ZeroTotalEmptyBrackets verifies the documented
// convention for a feature with no tasks: a pair of empty brackets with no
// block characters inside. The caller decides whether to add a numeric
// caption.
func TestRenderProgressBar_ZeroTotalEmptyBrackets(t *testing.T) {
	for _, unicode := range []bool{true, false} {
		got := RenderProgressBar(0, 0, unicode)
		if got != "[]" {
			t.Errorf("RenderProgressBar(0, 0, %v) = %q, want \"[]\"", unicode, got)
		}
	}
}

// TestRenderProgressBar_ZeroDoneAllEmpty verifies that a feature with no
// done tasks still shows every block, all empty.
func TestRenderProgressBar_ZeroDoneAllEmpty(t *testing.T) {
	t.Run("unicode", func(t *testing.T) {
		got := RenderProgressBar(0, 6, true)
		want := "[□□□□□□]"
		if got != want {
			t.Errorf("RenderProgressBar(0, 6, true) = %q, want %q", got, want)
		}
	})
	t.Run("ascii", func(t *testing.T) {
		got := RenderProgressBar(0, 6, false)
		want := "[      ]"
		if got != want {
			t.Errorf("RenderProgressBar(0, 6, false) = %q, want %q", got, want)
		}
	})
}

// TestRenderProgressBar_PartialCompletion exercises a selection of partial
// states to confirm the filled/empty counts are correct for total ≤ 20.
func TestRenderProgressBar_PartialCompletion(t *testing.T) {
	cases := []struct {
		done, total       int
		unicode           bool
		wantFilled, empty int
	}{
		{done: 3, total: 6, unicode: true, wantFilled: 3, empty: 3},
		{done: 1, total: 7, unicode: true, wantFilled: 1, empty: 6},
		{done: 7, total: 19, unicode: false, wantFilled: 7, empty: 12},
		{done: 5, total: 20, unicode: true, wantFilled: 5, empty: 15},
	}
	for _, tc := range cases {
		t.Run(namePartial(tc.done, tc.total, tc.unicode), func(t *testing.T) {
			got := RenderProgressBar(tc.done, tc.total, tc.unicode)
			filled, empty := countBlocks(got, tc.unicode)
			if filled != tc.wantFilled {
				t.Errorf("filled = %d, want %d (got %q)", filled, tc.wantFilled, got)
			}
			if empty != tc.empty {
				t.Errorf("empty = %d, want %d (got %q)", empty, tc.empty, got)
			}
			if !strings.HasPrefix(got, "[") || !strings.HasSuffix(got, "]") {
				t.Errorf("output must be bracketed; got %q", got)
			}
		})
	}
}

// TestRenderProgressBar_FullCompletion verifies that every block is filled
// when done == total, for both the small and max-boundary cases.
func TestRenderProgressBar_FullCompletion(t *testing.T) {
	cases := []struct{ done, total int }{
		{done: 6, total: 6},
		{done: 20, total: 20},
	}
	for _, tc := range cases {
		t.Run(namePartial(tc.done, tc.total, true), func(t *testing.T) {
			got := RenderProgressBar(tc.done, tc.total, true)
			filled, empty := countBlocks(got, true)
			if filled != tc.total {
				t.Errorf("filled = %d, want %d (got %q)", filled, tc.total, got)
			}
			if empty != 0 {
				t.Errorf("empty = %d, want 0 (got %q)", empty, got)
			}
		})
	}
}

// TestRenderProgressBar_CapAt20Blocks verifies the bar never exceeds 20
// blocks and that the filled count is proportional for total > 20.
func TestRenderProgressBar_CapAt20Blocks(t *testing.T) {
	cases := []struct {
		done, total, wantFilled int
	}{
		{done: 5, total: 25, wantFilled: 4},   // 5/25 = 20% → 4/20 filled
		{done: 20, total: 40, wantFilled: 10}, // 50% → 10/20
		{done: 40, total: 40, wantFilled: 20}, // 100% → 20/20
		{done: 0, total: 40, wantFilled: 0},   // 0% → 0/20
		{done: 1, total: 21, wantFilled: 1},   // rounding boundary
	}
	for _, tc := range cases {
		t.Run(namePartial(tc.done, tc.total, true), func(t *testing.T) {
			got := RenderProgressBar(tc.done, tc.total, true)
			filled, empty := countBlocks(got, true)
			totalBlocks := filled + empty
			if totalBlocks != ProgressBarMaxBlocks {
				t.Errorf("total blocks = %d, want %d (got %q)", totalBlocks, ProgressBarMaxBlocks, got)
			}
			if filled != tc.wantFilled {
				t.Errorf("filled = %d, want %d (got %q)", filled, tc.wantFilled, got)
			}
		})
	}
}

// TestRenderProgressBar_ClampsOutOfRangeDone verifies negative and
// overflow `done` values are clamped safely rather than producing a panic
// or an inconsistent bar.
func TestRenderProgressBar_ClampsOutOfRangeDone(t *testing.T) {
	cases := []struct {
		done, total, wantFilled int
		description             string
	}{
		{done: -1, total: 5, wantFilled: 0, description: "negative done clamps to 0"},
		{done: -100, total: 10, wantFilled: 0, description: "large negative clamps to 0"},
		{done: 10, total: 5, wantFilled: 5, description: "done>total clamps to total"},
		{done: 99, total: 6, wantFilled: 6, description: "huge done clamps to total"},
	}
	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			got := RenderProgressBar(tc.done, tc.total, true)
			filled, _ := countBlocks(got, true)
			if filled != tc.wantFilled {
				t.Errorf("%s: filled = %d, want %d (got %q)", tc.description, filled, tc.wantFilled, got)
			}
		})
	}
}

// TestRenderProgressBar_ClampsNegativeTotal verifies a negative total is
// treated as zero — same output as the zero-total case.
func TestRenderProgressBar_ClampsNegativeTotal(t *testing.T) {
	got := RenderProgressBar(0, -5, true)
	if got != "[]" {
		t.Errorf("negative total should render empty brackets; got %q", got)
	}
}

// TestRenderProgressBar_UnicodeAsciiToggle verifies the two rendering modes
// use the expected glyphs and do not leak glyphs from the other mode.
func TestRenderProgressBar_UnicodeAsciiToggle(t *testing.T) {
	t.Run("unicode-uses-block-glyphs", func(t *testing.T) {
		got := RenderProgressBar(2, 4, true)
		if !strings.Contains(got, "■") {
			t.Errorf("expected filled ■ in unicode mode; got %q", got)
		}
		if !strings.Contains(got, "□") {
			t.Errorf("expected empty □ in unicode mode; got %q", got)
		}
		if strings.ContainsAny(got, "#") {
			t.Errorf("ASCII glyph leaked into unicode mode: %q", got)
		}
	})
	t.Run("ascii-uses-hash-and-space", func(t *testing.T) {
		got := RenderProgressBar(2, 4, false)
		if !strings.Contains(got, "#") {
			t.Errorf("expected filled # in ASCII mode; got %q", got)
		}
		// Two filled, two empty: "[##  ]"
		if got != "[##  ]" {
			t.Errorf("expected \"[##  ]\" for ASCII 2/4, got %q", got)
		}
		for _, forbidden := range []string{"■", "□"} {
			if strings.Contains(got, forbidden) {
				t.Errorf("unicode glyph %q leaked into ASCII mode: %q", forbidden, got)
			}
		}
	})
}

// countBlocks tallies the filled and empty glyphs in a rendered bar so the
// assertions can focus on semantic content rather than exact byte layout.
func countBlocks(rendered string, unicode bool) (filled, empty int) {
	filledGlyph := ProgressBlockFilledASCII
	emptyGlyph := ProgressBlockEmptyASCII
	if unicode {
		filledGlyph = ProgressBlockFilledUnicode
		emptyGlyph = ProgressBlockEmptyUnicode
	}
	inner := strings.TrimPrefix(strings.TrimSuffix(rendered, "]"), "[")
	filled = strings.Count(inner, filledGlyph)
	empty = strings.Count(inner, emptyGlyph)
	return filled, empty
}

// namePartial produces a compact subtest name for table-driven cases.
func namePartial(done, total int, unicode bool) string {
	mode := "ascii"
	if unicode {
		mode = "unicode"
	}
	return mode + "/" + itoa(done) + "-of-" + itoa(total)
}

// itoa avoids importing strconv just for subtest naming.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
