package inception

// adapters.go contains the production implementations of all injected function
// types in the inception package. These functions bridge to real external services
// or TTY interaction and cannot be unit tested without live credentials or a terminal.
// They are excluded from SonarCloud coverage measurement via **/*_adapters.go.

import (
	"fmt"
	"io"

	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// DefaultSpinner is the production SpinnerFunc. It prints a simple
// "⠸ label..." / "✔ label" / "✖ label: error" sequence.
func DefaultSpinner(w io.Writer, label string, fn func() error) error {
	fmt.Fprintln(w, "  "+ui.Muted.Render("⠸ "+label+"..."))
	if err := fn(); err != nil {
		fmt.Fprintln(w, "  "+ui.RenderError(label+": "+err.Error()))
		return err
	}
	fmt.Fprintln(w, "  "+ui.RenderOK(label))
	return nil
}
