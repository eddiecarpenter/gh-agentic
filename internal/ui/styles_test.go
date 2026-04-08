package ui

import (
	"bytes"
	"strings"
	"testing"
)

func TestColorConstants_NotEmpty(t *testing.T) {
	colors := []struct {
		name  string
		value string
	}{
		{"ColorPrimary", ColorPrimary},
		{"ColorSuccess", ColorSuccess},
		{"ColorWarning", ColorWarning},
		{"ColorDanger", ColorDanger},
		{"ColorMuted", ColorMuted},
		{"ColorWhite", ColorWhite},
	}
	for _, c := range colors {
		t.Run(c.name, func(t *testing.T) {
			if c.value == "" {
				t.Errorf("%s must not be empty", c.name)
			}
			if !strings.HasPrefix(c.value, "#") {
				t.Errorf("%s must start with '#', got %q", c.name, c.value)
			}
		})
	}
}

func TestSymbolConstants_NotEmpty(t *testing.T) {
	symbols := []struct {
		name  string
		value string
	}{
		{"SymbolOK", SymbolOK},
		{"SymbolWarning", SymbolWarning},
		{"SymbolError", SymbolError},
	}
	for _, s := range symbols {
		t.Run(s.name, func(t *testing.T) {
			if s.value == "" {
				t.Errorf("%s must not be empty", s.name)
			}
		})
	}
}

func TestDivider_ReturnsNonEmpty(t *testing.T) {
	d := Divider(48)
	if d == "" {
		t.Error("Divider(48) must return a non-empty string")
	}
}

func TestDivider_ZeroWidth_ReturnsEmpty(t *testing.T) {
	// Divider with zero width should produce empty styled string (no dashes).
	// The rendered form may contain ANSI codes but stripping them leaves nothing.
	d := Divider(0)
	// Strip any ANSI sequences by checking the raw string contains no dash character.
	if strings.Contains(d, "─") {
		t.Error("Divider(0) must not contain any dash characters")
	}
}

func TestRenderOK_ContainsSymbol(t *testing.T) {
	out := RenderOK("git found")
	if !strings.Contains(out, SymbolOK) {
		t.Errorf("RenderOK must contain %q, got: %q", SymbolOK, out)
	}
	if !strings.Contains(out, "git found") {
		t.Errorf("RenderOK must contain the message, got: %q", out)
	}
}

func TestRenderWarning_ContainsSymbol(t *testing.T) {
	out := RenderWarning("claude not found")
	if !strings.Contains(out, SymbolWarning) {
		t.Errorf("RenderWarning must contain %q, got: %q", SymbolWarning, out)
	}
	if !strings.Contains(out, "claude not found") {
		t.Errorf("RenderWarning must contain the message, got: %q", out)
	}
}

func TestRenderError_ContainsSymbol(t *testing.T) {
	out := RenderError("git not found")
	if !strings.Contains(out, SymbolError) {
		t.Errorf("RenderError must contain %q, got: %q", SymbolError, out)
	}
	if !strings.Contains(out, "git not found") {
		t.Errorf("RenderError must contain the message, got: %q", out)
	}
}

func TestClearScreen_WritesANSISequence(t *testing.T) {
	var buf bytes.Buffer
	ClearScreen(&buf)
	got := buf.String()
	want := "\033[2J\033[H"
	if got != want {
		t.Errorf("ClearScreen wrote %q, want %q", got, want)
	}
}
