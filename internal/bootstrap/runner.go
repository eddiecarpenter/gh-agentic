package bootstrap

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// SpinnerFunc renders a single step with a spinner while fn runs, then prints
// ✔ or ✖ based on the result. It is injected so tests can substitute a plain
// text renderer without requiring a real TTY.
type SpinnerFunc func(w io.Writer, label string, fn func() error) error

// DefaultSpinner is the production SpinnerFunc. It uses a simple inline approach:
// print the step label, run fn, then overwrite with ✔ or ✖.
// A real bubbles spinner requires a Bubbletea program with a full event loop;
// for the MVP we print a simple "… label" / "✔ label" / "✖ label: error" sequence.
func DefaultSpinner(w io.Writer, label string, fn func() error) error {
	fmt.Fprintln(w, "  "+ui.Muted.Render("⠸ "+label+"..."))
	if err := fn(); err != nil {
		fmt.Fprintln(w, "  "+ui.RenderError(label+": "+err.Error()))
		return err
	}
	fmt.Fprintln(w, "  "+ui.RenderOK(label))
	return nil
}

// RunSteps orchestrates bootstrap steps 3-9 sequentially.
// Each step is wrapped by spinner so the user sees progress.
// On the first step failure the runner stops and returns the error immediately.
//
// workDir is the parent directory into which the repo will be cloned.
// run is injected for all CLI/git calls.
// graphqlDo is injected for GitHub GraphQL calls.
// launch is injected for the Goose launch at the end.
// spinner is injected so tests can use a plain text renderer.
func RunSteps(
	w io.Writer,
	cfg BootstrapConfig,
	workDir string,
	run RunCommandFunc,
	graphqlDo GraphQLDoFunc,
	launch LaunchFunc,
	spinner SpinnerFunc,
) error {
	fmt.Fprintln(w)
	fmt.Fprintln(w, ui.SectionHeading.Render("  Creating your agentic environment"))
	fmt.Fprintln(w)

	state := &StepState{}

	steps := []struct {
		label string
		fn    func() error
	}{
		{
			label: "Creating repository",
			fn:    func() error { return CreateRepo(w, cfg, state, workDir, run) },
		},
		{
			label: "Removing template files",
			fn:    func() error { return RemoveTemplateFiles(w, state, run) },
		},
		{
			label: "Scaffolding " + cfg.Stack + " project",
			fn:    func() error { return ScaffoldStack(w, cfg, state, run) },
		},
		{
			label: "Configuring labels",
			fn:    func() error { return ConfigureRepo(w, cfg, state, run) },
		},
		{
			label: "Populating repository",
			fn:    func() error { return PopulateRepo(w, cfg, state, run) },
		},
		{
			label: "Creating GitHub Project",
			fn:    func() error { return CreateProject(w, cfg, state, run, graphqlDo) },
		},
	}

	for _, s := range steps {
		if err := spinner(w, s.label, s.fn); err != nil {
			return err
		}
	}

	// Step 9: print summary and offer Goose launch.
	return PrintSummary(w, cfg, state, launch)
}

// DefaultWorkDir returns the directory in which repos will be cloned.
// It uses the current working directory.
func DefaultWorkDir() (string, error) {
	return os.Getwd()
}

// DefaultWorkDirOrHome returns the working directory, falling back to the user's
// home directory if os.Getwd() fails (e.g. in a deleted directory).
func DefaultWorkDirOrHome() string {
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, "Development")
	}
	return "."
}
