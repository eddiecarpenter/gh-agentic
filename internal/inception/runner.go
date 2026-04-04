package inception

import (
	"fmt"
	"io"

	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// SpinnerFunc renders a single step with a spinner while fn runs, then prints
// ✔ or ✖ based on the result. It is injected so tests can substitute a plain
// text renderer without requiring a real TTY.
type SpinnerFunc func(w io.Writer, label string, fn func() error) error

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

// RunSteps orchestrates inception steps 1-5 sequentially.
// Each step is wrapped by spinner so the user sees progress.
// On the first step failure the runner stops and returns the error immediately.
// PrintSummary is called last (outside spinner).
//
// cfg is the inception configuration from the form.
// env is the environment context from validation.
// run is injected for all CLI/git calls.
// spinner is injected so tests can use a plain text renderer.
func RunSteps(
	w io.Writer,
	cfg *InceptionConfig,
	env *EnvContext,
	run bootstrap.RunCommandFunc,
	spinner SpinnerFunc,
) error {
	fmt.Fprintln(w)
	fmt.Fprintln(w, ui.SectionHeading.Render("  Creating new repo"))
	fmt.Fprintln(w)

	state := &StepState{}

	steps := []struct {
		label string
		fn    func() error
	}{
		{
			label: "Creating repository",
			fn:    func() error { return CreateRepo(w, cfg, state, env, run) },
		},
		{
			label: "Configuring labels",
			fn:    func() error { return ConfigureLabels(w, cfg, state, run) },
		},
		{
			label: "Scaffolding " + cfg.Stack + " project",
			fn:    func() error { return ScaffoldStack(w, cfg, state, env, run) },
		},
		{
			label: "Populating repository",
			fn:    func() error { return PopulateRepo(w, cfg, state, env, run) },
		},
		{
			label: "Registering in REPOS.md",
			fn:    func() error { return RegisterInREPOS(w, cfg, state, env, run) },
		},
	}

	for _, s := range steps {
		if err := spinner(w, s.label, s.fn); err != nil {
			return err
		}
	}

	// Step 6: print summary (not wrapped in spinner).
	PrintSummary(w, cfg, state)
	return nil
}
