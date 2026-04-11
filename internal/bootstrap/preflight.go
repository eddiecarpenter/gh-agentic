// Package bootstrap implements the business logic for gh agentic bootstrap.
// It is independent of cobra — all functions accept explicit io.Writer and
// injected dependencies so they can be exercised in unit tests without
// spawning real processes or prompting the terminal.
package bootstrap

import (
	"fmt"
	"io"
	"strings"

	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// LookPathFunc resolves the absolute path of an executable by name.
// Injected so tests can substitute a fake resolver.
type LookPathFunc func(file string) (string, error)

// RunCommandFunc runs an external command and returns its combined output and
// any error. Injected so tests can substitute a fake runner.
type RunCommandFunc func(name string, args ...string) (string, error)

// ConfirmFunc presents a yes/no prompt and returns the user's choice.
// Returns true if the user chose yes, false otherwise.
// Injected so tests can simulate user input without a real TTY.
type ConfirmFunc func(prompt string) (bool, error)

// check describes a single preflight requirement.
type check struct {
	// name is the tool name shown in status output (e.g. "git").
	name string
	// required means a failure is a hard stop; recommended means warn and continue.
	required bool
	// installPrompt is the install-offer message shown when the tool is absent.
	// Empty string means no install offer — just print the stopURL and exit.
	installPrompt string
	// installCmd is the shell command used to install the tool (passed to bash -c).
	installCmd string
	// stopURL is the URL printed when a required tool is missing and no install
	// offer is made (or the offer is declined).
	stopURL string
	// verify is called to determine whether the check passes.
	// Returns "" on success, or a human-readable failure reason on failure.
	verify func(lookPath LookPathFunc, run RunCommandFunc) (passed bool, detail string)
}

// standardChecks returns the ordered list of preflight checks.
func standardChecks() []check {
	return []check{
		{
			name:     "git",
			required: true,
			stopURL:  "https://git-scm.com",
			verify: func(lookPath LookPathFunc, _ RunCommandFunc) (bool, string) {
				_, err := lookPath("git")
				return err == nil, ""
			},
		},
		{
			name:     "gh",
			required: true,
			stopURL:  "https://cli.github.com",
			verify: func(lookPath LookPathFunc, _ RunCommandFunc) (bool, string) {
				_, err := lookPath("gh")
				return err == nil, ""
			},
		},
		{
			name:     "gh auth",
			required: true,
			stopURL:  "run: gh auth login",
			verify: func(_ LookPathFunc, run RunCommandFunc) (bool, string) {
				out, err := run("gh", "auth", "status")
				if err != nil {
					return false, strings.TrimSpace(out)
				}
				return true, ""
			},
		},
		{
			name:          "goose",
			required:      true,
			installPrompt: "Install Goose now?",
			installCmd:    "curl -fsSL https://github.com/block/goose/releases/latest/download/install.sh | bash",
			stopURL:       "https://github.com/block/goose",
			verify: func(lookPath LookPathFunc, _ RunCommandFunc) (bool, string) {
				_, err := lookPath("goose")
				return err == nil, ""
			},
		},
		{
			name:          "claude",
			required:      false,
			installPrompt: "Install Claude Code now?",
			installCmd:    "curl -fsSL https://claude.ai/install.sh | bash",
			verify: func(lookPath LookPathFunc, _ RunCommandFunc) (bool, string) {
				_, err := lookPath("claude")
				return err == nil, ""
			},
		},
	}
}

// RunPreflight executes all preflight checks in order and writes status lines to w.
//
// lookPath resolves executables (use exec.LookPath in production).
// run executes commands and returns combined output (use defaultRunCommand in production).
// confirm presents yes/no prompts to the user (use a huh-backed implementation in production).
//
// Returns a non-nil error if any required check cannot be satisfied.
func RunPreflight(w io.Writer, lookPath LookPathFunc, run RunCommandFunc, confirm ConfirmFunc) error {
	fmt.Fprintln(w, ui.SectionHeading.Render("  Preflight checks"))
	fmt.Fprintln(w)

	for _, c := range standardChecks() {
		passed, _ := c.verify(lookPath, run)

		if passed {
			fmt.Fprintln(w, "  "+ui.RenderOK(c.name+" found"))
			continue
		}

		// Tool is missing.
		if !c.required {
			fmt.Fprintln(w, "  "+ui.RenderWarning(c.name+" not found (recommended)"))

			if c.installPrompt != "" {
				yes, err := confirm(c.installPrompt)
				if err != nil {
					return fmt.Errorf("prompt error: %w", err)
				}
				if yes {
					if installErr := runInstall(w, c, run); installErr == nil {
						// Re-verify after install.
						if ok, _ := c.verify(lookPath, run); ok {
							fmt.Fprintln(w, "  "+ui.RenderOK(c.name+" installed"))
							continue
						}
					}
				} else {
					fmt.Fprintln(w, "  "+ui.Muted.Render("· Skipping "+c.name+" — continuing without it"))
				}
			}
			continue
		}

		// Required tool is missing.
		fmt.Fprintln(w, "  "+ui.RenderError(c.name+" not found"))

		if c.installPrompt != "" {
			yes, err := confirm(c.installPrompt)
			if err != nil {
				return fmt.Errorf("prompt error: %w", err)
			}
			if yes {
				if installErr := runInstall(w, c, run); installErr == nil {
					// Re-verify after install.
					if ok, _ := c.verify(lookPath, run); ok {
						fmt.Fprintln(w, "  "+ui.RenderOK(c.name+" installed"))
						continue
					}
				}
			}
		}

		// Could not satisfy the required check.
		if c.stopURL != "" {
			fmt.Fprintln(w, "  "+ui.Muted.Render("→ "+c.stopURL))
		}
		return fmt.Errorf("required tool %q is not available", c.name)
	}

	fmt.Fprintln(w, ui.Divider(48))
	return nil
}

// runInstall runs the install command for a check using bash -c.
// Prints a spinner-style notice before running and returns any error.
func runInstall(w io.Writer, c check, run RunCommandFunc) error {
	fmt.Fprintln(w, "  "+ui.Muted.Render("Installing "+c.name+"..."))
	_, err := run("bash", "-c", c.installCmd)
	if err != nil {
		fmt.Fprintln(w, "  "+ui.RenderError("Install failed: "+err.Error()))
		return err
	}
	return nil
}

