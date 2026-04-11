package inception

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// FormRunFunc is a function that runs a huh.Form. The production implementation
// simply calls f.Run(). Tests inject a fake that sets form-bound values directly.
type FormRunFunc func(f *huh.Form) error

// DefaultFormRun is the production FormRunFunc — it delegates to huh.Form.Run().
var DefaultFormRun FormRunFunc = func(f *huh.Form) error { return f.Run() }

// ErrAborted is returned by RunForm when the user declines the final confirmation.
var ErrAborted = errors.New("aborted by user")

// repoTypeOptions are the selectable repo types.
var repoTypeOptions = []huh.Option[string]{
	huh.NewOption("Domain  — a bounded-context service", "domain"),
	huh.NewOption("Tool    — utility/testing repo", "tool"),
	huh.NewOption("Other   — custom repo type", "other"),
}

// stackOptions are the selectable stacks (same as bootstrap).
var stackOptions = []huh.Option[string]{
	huh.NewOption("Go", "Go"),
	huh.NewOption("Java — Quarkus", "Java Quarkus"),
	huh.NewOption("Java — Spring Boot", "Java Spring Boot"),
	huh.NewOption("TypeScript / Node.js", "TypeScript Node.js"),
	huh.NewOption("Python", "Python"),
	huh.NewOption("Rust", "Rust"),
	huh.NewOption("Other", "Other"),
}

// validateStackSelection returns an error if no stacks are selected.
func validateStackSelection(selected []string) error {
	if len(selected) == 0 {
		return errors.New("at least one stack must be selected")
	}
	return nil
}

// validateRepoName returns an error if s is not a valid repo name.
// Valid names are non-empty, lowercase, and contain only letters, digits, and hyphens.
func validateRepoName(s string) error {
	if s == "" {
		return errors.New("repo name cannot be empty")
	}
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
			return fmt.Errorf("repo name must be lowercase with hyphens only (got %q)", s)
		}
	}
	return nil
}

// DeriveRepoName returns the full GitHub repo name from the user-entered name
// and selected repo type.
func DeriveRepoName(name, repoType string) string {
	switch repoType {
	case "domain":
		return name + "-domain"
	case "tool":
		return name + "-tool"
	default:
		return name
	}
}

// RunForm runs the interactive huh form to collect inception configuration.
// It receives the EnvContext to pre-fill the owner field.
// Returns a populated InceptionConfig, or ErrAborted if the user declines confirmation.
func RunForm(w io.Writer, ctx EnvContext, formRun FormRunFunc) (*InceptionConfig, error) {
	cfg := &InceptionConfig{
		Owner: ctx.Owner,
	}

	// --- Group 1: Repo Type ---
	typeForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("What type of repo is this?").
				Options(repoTypeOptions...).
				Value(&cfg.RepoType),
		),
	)
	if err := formRun(typeForm); err != nil {
		return nil, fmt.Errorf("repo type form: %w", err)
	}

	// --- Group 2: Repo Details ---
	detailsForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Name (without suffix — e.g. 'charging', not 'charging-domain')").
				Value(&cfg.RepoName).
				Validate(validateRepoName),
			huh.NewInput().
				Title("Description (1-2 sentences)").
				Value(&cfg.Description),
			huh.NewMultiSelect[string]().
				Title("Stack (select all that apply)").
				Options(stackOptions...).
				Value(&cfg.Stacks).
				Validate(validateStackSelection),
		),
	)
	if err := formRun(detailsForm); err != nil {
		return nil, fmt.Errorf("repo details form: %w", err)
	}

	// --- Summary box ---
	fmt.Fprintln(w)
	fmt.Fprintln(w, ui.SectionHeading.Render("  Summary"))
	fmt.Fprintln(w)
	fmt.Fprintln(w, RenderSummaryBox(*cfg))
	fmt.Fprintln(w)

	// --- Group 3: Final Confirm ---
	var confirmed bool
	confirmForm := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Create repo?").
				Value(&confirmed),
		),
	)
	if err := formRun(confirmForm); err != nil {
		return nil, fmt.Errorf("confirm form: %w", err)
	}

	if !confirmed {
		return nil, ErrAborted
	}

	return cfg, nil
}

// RenderSummaryBox renders the lipgloss summary box for the given config.
// It is a pure function, extracted to allow unit testing without a TTY.
func RenderSummaryBox(cfg InceptionConfig) string {
	label := ui.Muted.Render
	value := ui.Value.Render

	fullName := DeriveRepoName(cfg.RepoName, cfg.RepoType)

	content := fmt.Sprintf(
		"  %s  %s\n  %s  %s\n  %s  %s\n  %s  %s\n  %s  %s\n  %s  %s",
		label("Type       "), value(cfg.RepoType),
		label("Name       "), value(cfg.RepoName),
		label("Full name  "), value(cfg.Owner+"/"+fullName),
		label("Description"), value(cfg.Description),
		label("Stack      "), value(strings.Join(cfg.Stacks, ", ")),
		label("Owner      "), value(cfg.Owner),
	)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ui.ColorPrimary)).
		Width(56).
		Padding(0, 1)

	return box.Render(content)
}
