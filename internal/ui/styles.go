// Package ui defines the shared colour palette and lipgloss styles for gh-agentic.
// All terminal styling is centralised here; no other package defines styles inline.
package ui

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

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

// SpinnerFunc is the signature for a function that runs fn() while displaying
// an animated spinner labelled with label. Tests use testutil.NoopSpinner.
type SpinnerFunc func(w io.Writer, label string, fn func() error) error

// spinnerFrames are the braille dot animation frames used by RunWithSpinner.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// RunWithSpinner runs fn() in a background goroutine while animating a spinner
// on w. When fn returns the spinner is erased and the error (if any) is returned.
// It is safe to call on a non-TTY writer — the spinner simply overwrites itself
// using carriage returns.
func RunWithSpinner(w io.Writer, label string, fn func() error) error {
	done := make(chan error, 1)
	go func() { done <- fn() }()

	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()

	i := 0
	for {
		select {
		case err := <-done:
			// Erase the spinner line.
			fmt.Fprintf(w, "\r%s\r", strings.Repeat(" ", len(label)+6))
			return err
		case <-ticker.C:
			frame := URL.Render(spinnerFrames[i%len(spinnerFrames)])
			fmt.Fprintf(w, "\r  %s  %s", frame, label)
			i++
		}
	}
}

// DynamicSpinnerFunc is the signature for a spinner whose label can be updated
// mid-flight. fn receives a setLabel callback it can call at any time.
// Tests use testutil.NoopDynamicSpinner.
type DynamicSpinnerFunc func(w io.Writer, initialLabel string, fn func(setLabel func(string)) error) error

// RunWithDynamicSpinner is like RunWithSpinner but the label can be updated at
// any time by calling setLabel from within fn.
//
// setLabel writes directly to the terminal in the calling goroutine (holding a
// mutex shared with the animation goroutine), so each label change is visible
// immediately — even when many fast steps complete well under the tick interval.
// The animation goroutine only drives the spinner frames; it never races with a
// setLabel call because both hold mu before writing.
func RunWithDynamicSpinner(w io.Writer, initialLabel string, fn func(setLabel func(string)) error) error {
	var mu sync.Mutex
	if initialLabel == "" {
		initialLabel = "Working..."
	}
	currentLabel := initialLabel
	maxLen := len(initialLabel)
	frame := 0

	// write renders the current label at frame f. Caller must hold mu.
	write := func(f int) {
		fmt.Fprintf(w, "\r  %s  %-*s", URL.Render(spinnerFrames[f%len(spinnerFrames)]), maxLen, currentLabel)
	}

	// Write initial label synchronously before any goroutines start.
	mu.Lock()
	write(frame)
	frame++
	mu.Unlock()

	// setLabel writes directly to the terminal under mu so each label change
	// is immediately visible regardless of the animation goroutine's schedule.
	setLabel := func(s string) {
		mu.Lock()
		currentLabel = s
		if len(s) > maxLen {
			maxLen = len(s)
		}
		frame = 0
		write(frame)
		frame++
		mu.Unlock()
	}

	// Animation goroutine — advances the spinner braille frame on the current
	// label. Shares mu with setLabel so writes never interleave.
	stop := make(chan struct{})
	animDone := make(chan struct{})
	go func() {
		defer close(animDone)
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				mu.Lock()
				write(frame)
				frame++
				mu.Unlock()
			}
		}
	}()

	err := fn(setLabel)

	close(stop)
	<-animDone

	// Erase the spinner line.
	mu.Lock()
	fmt.Fprintf(w, "\r%s\r", strings.Repeat(" ", maxLen+6))
	mu.Unlock()

	return err
}
