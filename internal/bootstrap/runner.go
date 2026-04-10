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
// spinner is injected so tests can use a plain text renderer.
func RunSteps(
	w io.Writer,
	cfg BootstrapConfig,
	workDir string,
	run RunCommandFunc,
	graphqlDo GraphQLDoFunc,
	spinner SpinnerFunc,
	fetchRelease FetchReleaseFunc,
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
			fn:    func() error { return CreateRepo(w, cfg, state, workDir, run, fetchRelease) },
		},
		{
			label: "Configuring repository",
			fn: func() error {
				if err := ConfigureRepo(w, cfg, state, run); err != nil {
					return err
				}
				return CreateProject(w, cfg, state, run, graphqlDo)
			},
		},
		{
			label: "Populating repository",
			fn:    func() error { return PopulateRepo(w, cfg, state, run) },
		},
		{
			label: "Setting agent user variable",
			fn:    func() error { return SetAgentUserVariable(w, cfg, state, run) },
		},
		{
			label: "Setting pipeline variables",
			fn:    func() error { return SetPipelineVariables(w, cfg, state, run) },
		},
		{
			label: "Configuring pipeline secrets",
			fn: func() error {
				if err := SetClaudeCredentials(w, cfg, state, run, DefaultReadFile, DefaultUserHomeDir); err != nil {
					return err
				}
				return ValidateAgentPAT(w, cfg, state, run)
			},
		},
		{
			label: "Configuring project status columns",
			fn:    func() error { return ConfigureProjectStatus(w, state, graphqlDo) },
		},
		{
			label: "Deploying sync workflows",
			fn:    func() error { return DeploySyncWorkflows(w, cfg, state, run) },
		},
	}

	for _, s := range steps {
		if err := spinner(w, s.label, s.fn); err != nil {
			return err
		}
	}

	// For existing repos: push branch and open PR as a final step.
	if state.ExistingRepo {
		if err := spinner(w, "Opening bootstrap pull request", func() error {
			return OpenBootstrapPR(cfg, state, run)
		}); err != nil {
			return err
		}
	}

	// Step 9: print summary with next-step instructions.
	return PrintSummary(w, cfg, state)
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
