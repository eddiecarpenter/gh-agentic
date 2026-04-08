// Package ui defines the shared colour palette and lipgloss styles for gh-agentic.
// All terminal styling is centralised here; no other package defines styles inline.
package ui

import (
	"io"

	"github.com/charmbracelet/lipgloss"
)

// GitHub colour palette — from docs/TUI_DESIGN.md.
const (
	ColorPrimary = "#0969DA" // Headings, prompts, cursors, spinners, URLs
	ColorSuccess = "#1A7F37" // ✔ check marks, final summary border
	ColorWarning = "#9A6700" // ⚠ warnings
	ColorDanger  = "#CF222E" // ✖ errors, failures
	ColorMuted   = "#656D76" // Labels, dividers, secondary text
	ColorWhite   = "#FFFFFF" // Values, selected items
)

// Status symbols used in preflight and execution output.
const (
	SymbolOK      = "✔"
	SymbolWarning = "⚠"
	SymbolError   = "✖"
	SymbolInfo    = "ℹ"
)

// Styles — pre-built lipgloss renderers for common UI elements.
var (
	// SectionHeading renders a bold heading in Primary blue.
	SectionHeading = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorPrimary))

	// StatusOK renders the ✔ symbol in Success green.
	StatusOK = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorSuccess))

	// StatusWarning renders the ⚠ symbol in Warning amber.
	StatusWarning = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorWarning))

	// StatusDanger renders the ✖ symbol in Danger red.
	StatusDanger = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorDanger))

	// Muted renders secondary text in Muted grey.
	Muted = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorMuted))

	// URL renders a URL in Primary blue.
	URL = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorPrimary))

	// Value renders a value in White.
	Value = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorWhite))
)

// Divider returns a muted horizontal rule of the given width.
// Typically 48 dashes, matching the TUI_DESIGN.md specification.
func Divider(width int) string {
	line := ""
	for i := 0; i < width; i++ {
		line += "─"
	}
	return Muted.Render(line)
}

// RenderOK returns a formatted "✔ <msg>" line in Success green.
func RenderOK(msg string) string {
	return StatusOK.Render(SymbolOK) + " " + msg
}

// RenderWarning returns a formatted "⚠ <msg>" line in Warning amber.
func RenderWarning(msg string) string {
	return StatusWarning.Render(SymbolWarning) + " " + msg
}

// RenderError returns a formatted "✖ <msg>" line in Danger red.
func RenderError(msg string) string {
	return StatusDanger.Render(SymbolError) + " " + msg
}

// RenderInfo returns a formatted "ℹ <msg>" line in Primary blue.
// Used for manual-action items that are not failures.
func RenderInfo(msg string) string {
	return URL.Render(SymbolInfo) + " " + msg
}

// ClearScreen writes the ANSI escape sequence to clear the terminal and move
// the cursor to the top-left corner. Used between stages in interactive flows.
func ClearScreen(w io.Writer) {
	_, _ = io.WriteString(w, "\033[2J\033[H")
}
